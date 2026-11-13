package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
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

func run() error {
	productSlug := flag.String("product", "cafe", "product source set to seed: cafe or bank")
	flag.Parse()

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

	transaction, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin %s seed transaction: %w", seed.name, err)
	}
	defer transaction.Rollback(ctx) //nolint:errcheck

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
