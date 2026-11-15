package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultDatabaseURL = "postgres://postgres:postgres@localhost:5433/app?sslmode=disable" //nolint:gosec // Credentials are the local Compose defaults.
	userEmail          = "a@a"
	s3Bucket           = "my-bucket"
	s3Endpoint         = "http://localhost:8333"
	s3AccessKeyID      = "admin"
	s3SecretAccessKey  = "secret"
	s3Region           = "us-east-1"
)

type attachment struct {
	filename string
	path     string
	fileType string
}

type sourceFile struct {
	kind, filename, contentType, key string
	size                             int64
}

type productSeed struct {
	slug              string
	s3KeyPrefix       string
	hypothesis        string
	name              string
	description       string
	category          string
	keywords          []string
	price             string
	experimentGoal    string
	targetAction      string
	allowedChannels   []string
	forbiddenClaims   []string
	knownConstraints  []string
	primaryMetric     string
	metricNumerator   string
	metricDenominator string
	unknowns          []string
}

func productSeedFor(slug string) (productSeed, bool) {
	seeds := map[string]productSeed{
		"cafe": {
			slug:              "cafe",
			s3KeyPrefix:       "tempo-test-pair",
			hypothesis:        "Владельцы и управляющие небольших кафе в России, которые сейчас составляют графики смен вручную, захотят попробовать Tempo для создания и публикации расписания команды.",
			name:              "Tempo",
			description:       "Сервис для создания и распространения рабочих расписаний небольших кафе.",
			category:          "food",
			keywords:          []string{"кафе", "расписание смен"},
			price:             `{"status":"unknown","description":"Цена не указана."}`,
			experimentGoal:    "Проверить интерес владельцев и управляющих небольших кафе к пробному использованию Tempo для планирования смен.",
			targetAction:      "Запросить демонстрацию или начать пробное использование Tempo.",
			allowedChannels:   []string{},
			forbiddenClaims:   []string{"Не утверждать, что Tempo автоматически планирует смены.", "Не утверждать, что Tempo прогнозирует спрос."},
			knownConstraints:  []string{"Спрос и готовность платить не подтверждены."},
			primaryMetric:     "tempo_trial_interest",
			metricNumerator:   "Уникальные владельцы или управляющие, запросившие демонстрацию или начавшие пробное использование",
			metricDenominator: "Уникальные подходящие участники, которым было предложено попробовать Tempo",
			unknowns:          []string{"Фактическая цена и условия пробного периода не указаны.", "Наличие интереса и готовность платить не подтверждены."},
		},
		"bank": {
			slug:              "bank",
			s3KeyPrefix:       "lumen-test-pair",
			hypothesis:        "Действующие держатели дебетовой или зарплатной карты Lumen банка с неравномерными расходами захотят подать заявление на подключение кредитного лимита через мобильное приложение.",
			name:              "Lumen",
			description:       "Банковская карта, объединяющая собственные средства и подключаемый кредитный лимит.",
			category:          "banking_card",
			keywords:          []string{"Lumen", "банк", "кредитный лимит"},
			price:             `{"status":"unknown","description":"Итоговые условия и решение по лимиту зависят от банка."}`,
			experimentGoal:    "Проверить долю действующих держателей карты Lumen банка, подающих заявление на подключение кредитного лимита после знакомства с предложением.",
			targetAction:      "Подать заявление на подключение кредитного лимита в мобильном приложении Lumen банка.",
			allowedChannels:   []string{"in_app_banner", "push", "email", "landing", "sms"},
			forbiddenClaims:   []string{"Не обещать одобрение каждому заявителю.", "Не выдавать желаемый лимит за решение банка.", "Не обещать отсутствие обязательных платежей или переплаты при любых условиях."},
			knownConstraints:  []string{"Окончательное решение по кредитному лимиту принимает банк.", "Наличие карты не означает одобрение или гарантию подключения лимита."},
			primaryMetric:     "application_submit_rate",
			metricNumerator:   "Число уникальных клиентов группы, отправивших заявление в заданное окно наблюдения",
			metricDenominator: "Число уникальных клиентов, рандомизированных в эту группу",
			unknowns:          []string{"Фактическая доступность предложения и согласие на коммуникацию проверяются отдельно.", "Результат кредитного решения и готовность клиента использовать лимит не подтверждены."},
		},
	}

	seed, ok := seeds[slug]
	return seed, ok
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}

}

//nolint:cyclop // Existing seed setup plus explicit clone-mode validation.
func run() error {
	productSlug := flag.String("product", "cafe", "product source set to seed: cafe or bank")
	cloneSource := flag.String("clone-source", "", "source hypothesis ID to clone into an A/B pair")
	abPair := flag.Bool("ab-pair", false, "create two independent copies of --clone-source")
	pilotAudience := flag.Bool("pilot-audience", false, "attach three fixed synthetic audiences to both clones if source has none")
	draftLandingURL := flag.String("draft-landing-url", "", "add nine synthetic Tempo draft creatives using this landing URL (pair mode only)")
	flag.Parse()
	if *abPair && *cloneSource == "" {
		return errors.New("--ab-pair requires --clone-source")
	}

	if *cloneSource != "" && !*abPair {
		return errors.New("--clone-source requires --ab-pair")
	}

	if *pilotAudience && !*abPair {
		return errors.New("--pilot-audience requires --ab-pair and --clone-source")
	}

	if *abPair && os.Getenv("DATABASE_URL") == "" {
		return errors.New("A/B clone mode requires an explicit DATABASE_URL")
	}

	if *draftLandingURL != "" && !*abPair {
		return errors.New("--draft-landing-url requires --ab-pair and --clone-source")
	}

	seed, ok := productSeedFor(*productSlug)
	if !ok {
		return fmt.Errorf("unsupported product %q; choose cafe or bank", *productSlug)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = defaultDatabaseURL
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	defer pool.Close()

	s3Client, err := newS3Client(ctx)
	if err != nil {
		return err
	}

	var transaction pgx.Tx
	if *abPair {
		transaction, err = pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	} else {
		transaction, err = pool.Begin(ctx)
	}

	if err != nil {
		return fmt.Errorf("begin %s seed transaction: %w", seed.name, err)
	}

	defer transaction.Rollback(ctx) //nolint:errcheck
	if *abPair {
		return clonePair(ctx, transaction, s3Client, *cloneSource, *pilotAudience, *draftLandingURL)
	}

	var userID string
	if err := transaction.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", userEmail).Scan(&userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("find seed user %q: account does not exist", userEmail)
		}

		return fmt.Errorf("find seed user %q: %w", userEmail, err)
	}

	hypothesisID := uuid.NewString()
	productID := uuid.NewString()

	_, err = transaction.Exec(ctx, `
		INSERT INTO hypothesis (id, user_id, description, status)
		VALUES ($1, $2, $3, 'idle')
		ON CONFLICT (id) DO NOTHING`,
		hypothesisID,
		userID,
		seed.hypothesis,
	)
	if err != nil {
		return fmt.Errorf("insert %s test hypothesis: %w", seed.name, err)
	}

	_, err = transaction.Exec(ctx, `
		INSERT INTO product (
			id, name, description, category, keywords, price, hypothesis_id,
			brand, country, town, experiment_goal, target_action,
			allowed_channels, forbidden_claims, known_constraints,
			goal_origin, primary_metric, metric_direction, metric_numerator,
			metric_denominator, unknowns
		)
		VALUES (
			$1, $2, $3, $4, $5, $6::jsonb, $7,
			$8, 'Россия', NULL, $9, $10,
			$11, $12, $13,
			'user_provided', $14, 'increase', $15,
			$16, $17
		)
		ON CONFLICT (hypothesis_id) DO NOTHING`,
		productID,
		seed.name,
		seed.description,
		seed.category,
		seed.keywords,
		seed.price,
		hypothesisID,
		seed.name,
		seed.experimentGoal,
		seed.targetAction,
		seed.allowedChannels,
		seed.forbiddenClaims,
		seed.knownConstraints,
		seed.primaryMetric,
		seed.metricNumerator,
		seed.metricDenominator,
		seed.unknowns,
	)
	if err != nil {
		return fmt.Errorf("insert %s test product: %w", seed.name, err)
	}

	sourceDirectory, err := locateProductDirectory(seed.slug)
	if err != nil {
		return err
	}

	attachments, err := loadAttachments(sourceDirectory)
	if err != nil {
		return err
	}

	if err := attachProductFiles(ctx, transaction, s3Client, hypothesisID, seed, sourceDirectory, attachments); err != nil {
		return err
	}

	var storedProductID string
	if err := transaction.QueryRow(ctx, "SELECT id FROM product WHERE hypothesis_id = $1", hypothesisID).Scan(&storedProductID); err != nil {
		return fmt.Errorf("verify %s test product: %w", seed.name, err)
	}

	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit Tempo seed transaction: %w", err)
	}

	fmt.Printf("%s test pair is ready for %s (hypothesis %s, product %s)\n", seed.name, userEmail, hypothesisID, storedProductID)
	fmt.Printf("Attached %d product files\n", len(attachments))
	return nil
}

func newS3Client(ctx context.Context) (*s3.Client, error) {
	endpoint := envOrDefault("S3_ENDPOINT", s3Endpoint)
	region := envOrDefault("S3_REGION", s3Region)
	accessKeyID := envOrDefault("S3_ACCESS_KEY_ID", s3AccessKeyID)
	secretAccessKey := envOrDefault("S3_SECRET_ACCESS_KEY", s3SecretAccessKey)

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load S3 configuration: %w", err)
	}

	return s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = true
	}), nil
}

//nolint:cyclop // Keeps the paired clone transaction and its safety checks together.
func clonePair(ctx context.Context, tx pgx.Tx, client *s3.Client, sourceID string, pilotAudience bool, draftLandingURL string) error {
	var userID, description, sourceProductID string
	if err := tx.QueryRow(ctx, `SELECT h.user_id, h.description, p.id FROM hypothesis h JOIN product p ON p.hypothesis_id=h.id WHERE h.id=$1`, sourceID).Scan(&userID, &description, &sourceProductID); err != nil {
		return fmt.Errorf("load source hypothesis and product %s: %w", sourceID, err)
	}

	var audienceCount int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM audience WHERE product_id=$1", sourceProductID).Scan(&audienceCount); err != nil {
		return fmt.Errorf("count source audiences: %w", err)
	}

	if audienceCount == 0 && !pilotAudience {
		return errors.New("source has no audience snapshot; use --pilot-audience to explicitly attach synthetic pilot data")
	}

	draftConfig, err := draftPilotConfig(ctx, tx, sourceProductID, audienceCount, draftLandingURL)
	if err != nil {
		return err
	}

	fileRows, err := tx.Query(ctx, "SELECT type, filename, content_type, size, s3_key FROM file WHERE hypothesis_id=$1 ORDER BY filename", sourceID)
	if err != nil {
		return fmt.Errorf("list source files: %w", err)
	}

	var files []sourceFile
	for fileRows.Next() {
		var file sourceFile
		if err := fileRows.Scan(&file.kind, &file.filename, &file.contentType, &file.size, &file.key); err != nil {
			fileRows.Close()
			return fmt.Errorf("scan source file: %w", err)
		}

		files = append(files, file)
	}

	if err := fileRows.Err(); err != nil {
		fileRows.Close()
		return fmt.Errorf("iterate source files: %w", err)
	}

	fileRows.Close()

	var receipts []string
	for _, arm := range []string{"A", "B"} {
		hypothesisID, productID := uuid.NewString(), uuid.NewString()
		if _, err := tx.Exec(ctx, `INSERT INTO hypothesis (id,user_id,description,status) VALUES ($1,$2,$3,'idle')`, hypothesisID, userID, description); err != nil {
			return fmt.Errorf("create %s hypothesis clone: %w", arm, err)
		}

		if _, err := tx.Exec(ctx, `INSERT INTO product SELECT (jsonb_populate_record(NULL::product, jsonb_set(jsonb_set(to_jsonb(p),'{id}',to_jsonb($1::text)),'{hypothesis_id}',to_jsonb($2::text)))).* FROM product p WHERE p.id=$3`, productID, hypothesisID, sourceProductID); err != nil {
			return fmt.Errorf("create %s product clone: %w", arm, err)
		}

		if len(draftConfig) > 0 {
			if _, err := tx.Exec(ctx, "UPDATE product SET preparation_config=$2::jsonb WHERE id=$1", productID, string(draftConfig)); err != nil {
				return fmt.Errorf("store %s draft pilot creatives: %w", arm, err)
			}
		}

		if err := cloneFiles(ctx, tx, client, hypothesisID, files); err != nil {
			return fmt.Errorf("clone %s files: %w", arm, err)
		}

		if audienceCount > 0 {
			if _, err := tx.Exec(ctx, `INSERT INTO audience (id,product_id,generator_job_id,persona_id,persona,research,metrics,hypotheses,rank,score) SELECT gen_random_uuid()::text,$1,gen_random_uuid()::text,persona_id,persona,research,metrics,hypotheses,rank,score FROM audience WHERE product_id=$2`, productID, sourceProductID); err != nil {
				return fmt.Errorf("copy %s audience snapshot: %w", arm, err)
			}

		} else if pilotAudience {
			if err := insertPilotAudiences(ctx, tx, productID); err != nil {
				return fmt.Errorf("insert %s synthetic pilot audiences: %w", arm, err)
			}

		}

		receipts = append(receipts, fmt.Sprintf("%s hypothesis=%s product=%s", arm, hypothesisID, productID))
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit cloned A/B pair: %w", err)
	}

	for _, receipt := range receipts {
		fmt.Println(receipt)
	}

	return nil
}

func insertPilotAudiences(ctx context.Context, tx pgx.Tx, productID string) error {
	profiles := []struct {
		id, gender string
		age, rank  int
	}{
		{id: "synthetic-pilot-male-30", gender: "мужчина", age: 30, rank: 1},
		{id: "synthetic-pilot-female-30", gender: "женщина", age: 30, rank: 2},
		{id: "synthetic-pilot-male-40", gender: "мужчина", age: 40, rank: 3},
	}
	for _, profile := range profiles {
		persona := fmt.Sprintf(`{"gender":%q,"age":%d,"name":"Синтетический пилотный сегмент"}`, profile.gender, profile.age)
		research := `{"provenance":"synthetic_pilot","note":"Engineering pilot only; not generated research."}`
		metrics := `{"provenance":"synthetic_pilot"}`
		hypotheses := `{"provenance":"synthetic_pilot"}`
		if _, err := tx.Exec(ctx, `INSERT INTO audience (id,product_id,generator_job_id,persona_id,persona,research,metrics,hypotheses,rank,score) VALUES ($1,$2,$3,$4,$5::jsonb,$6::jsonb,$7::jsonb,$8::jsonb,$9,0)`, uuid.NewString(), productID, uuid.NewString(), profile.id, persona, research, metrics, hypotheses, profile.rank); err != nil {
			return fmt.Errorf("insert synthetic persona %s: %w", profile.id, err)
		}
	}

	return nil
}

func cloneFiles(ctx context.Context, tx pgx.Tx, client *s3.Client, targetID string, files []sourceFile) error {
	bucket := envOrDefault("S3_BUCKET", s3Bucket)
	for _, file := range files {
		fileID := uuid.NewString()
		key := "tempo-ab-clone/" + targetID + "/" + fileID + "/" + filepath.Base(file.key)
		if _, err := client.CopyObject(ctx, &s3.CopyObjectInput{Bucket: aws.String(bucket), Key: aws.String(key), CopySource: aws.String(url.PathEscape(bucket + "/" + file.key))}); err != nil {
			return fmt.Errorf("copy object %s: %w", file.key, err)
		}

		if _, err := tx.Exec(ctx, `INSERT INTO file (id,hypothesis_id,type,filename,content_type,size,s3_key) VALUES ($1,$2,$3,$4,$5,$6,$7)`, fileID, targetID, file.kind, file.filename, file.contentType, file.size, key); err != nil {
			return fmt.Errorf("record cloned file %s: %w", file.filename, err)
		}

		if _, err := client.CopyObject(ctx, &s3.CopyObjectInput{Bucket: aws.String(bucket), Key: aws.String(key + ".extracted.txt"), CopySource: aws.String(url.PathEscape(bucket + "/" + file.key + ".extracted.txt"))}); err != nil {
			return fmt.Errorf("copy extracted object %s: %w", file.key, err)
		}

	}

	return nil
}

func attachProductFiles(ctx context.Context, transaction pgx.Tx, client *s3.Client, hypothesisID string, seed productSeed, sourceDirectory string, attachments []attachment) error {
	for _, item := range attachments {
		data, err := os.ReadFile(filepath.Join(sourceDirectory, item.path))
		if err != nil {
			return fmt.Errorf("read %s attachment %s: %w", seed.name, item.filename, err)
		}

		key := seed.s3KeyPrefix + "/" + hypothesisID + "/" + item.filename
		contentType := "text/markdown; charset=utf-8"
		if _, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:        aws.String(envOrDefault("S3_BUCKET", s3Bucket)),
			Key:           aws.String(key),
			Body:          bytes.NewReader(data),
			ContentType:   aws.String(contentType),
			ContentLength: aws.Int64(int64(len(data))),
		}); err != nil {
			return fmt.Errorf("upload %s attachment %s: %w", seed.name, item.filename, err)
		}

		textKey := key + ".extracted.txt"
		if _, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:        aws.String(envOrDefault("S3_BUCKET", s3Bucket)),
			Key:           aws.String(textKey),
			Body:          bytes.NewReader(data),
			ContentType:   aws.String("text/plain; charset=utf-8"),
			ContentLength: aws.Int64(int64(len(data))),
		}); err != nil {
			return fmt.Errorf("upload extracted %s attachment %s: %w", seed.name, item.filename, err)
		}

		_, err = transaction.Exec(ctx, `
			INSERT INTO file (id, hypothesis_id, type, filename, content_type, size, s3_key)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (s3_key) DO NOTHING`,
			uuid.NewString(), hypothesisID, item.fileType, item.filename, contentType, len(data), key,
		)
		if err != nil {
			return fmt.Errorf("record %s attachment %s: %w", seed.name, item.filename, err)
		}

	}

	return nil
}

func locateProductDirectory(productSlug string) (string, error) {
	for _, path := range []string{"../n8n/product/" + productSlug, "n8n/product/" + productSlug} {
		if info, err := os.Stat(filepath.Join(path, "product-brief.md")); err == nil && !info.IsDir() {
			return path, nil
		}

	}

	return "", fmt.Errorf("locate %s source documents: run from the repository root or backend directory", productSlug)
}

func loadAttachments(sourceDirectory string) ([]attachment, error) {
	entries, err := os.ReadDir(sourceDirectory)
	if err != nil {
		return nil, fmt.Errorf("list product source documents: %w", err)
	}

	attachments := make([]attachment, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.EqualFold(entry.Name(), "README.md") || filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		attachments = append(attachments, attachment{
			filename: entry.Name(),
			path:     entry.Name(),
			fileType: attachmentType(entry.Name()),
		})
	}

	if len(attachments) == 0 {
		return nil, fmt.Errorf("no Markdown source documents found in %s", sourceDirectory)
	}

	sort.Slice(attachments, func(i, j int) bool {
		return attachments[i].filename < attachments[j].filename
	})
	return attachments, nil
}

func attachmentType(filename string) string {
	switch filename {
	case "brand-book.md":
		return "brandbook"
	case "features-and-pricing.md", "product-brief.md":
		return "product"
	default:
		return "reference"
	}

}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
