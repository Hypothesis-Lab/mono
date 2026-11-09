package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
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
	s3KeyPrefix        = "tempo-test-pair"
	s3Bucket           = "my-bucket"
	s3Endpoint         = "http://localhost:8333"
	s3AccessKeyID      = "admin"
	s3SecretAccessKey  = "secret"
	s3Region           = "us-east-1"
)

var attachments = []attachment{
	{filename: "api-reference.md", path: "api-reference.md", fileType: "reference"},
	{filename: "brand-book.md", path: "brand-book.md", fileType: "brandbook"},
	{filename: "features-and-pricing.md", path: "features-and-pricing.md", fileType: "product"},
	{filename: "glossary.md", path: "glossary.md", fileType: "reference"},
	{filename: "product-brief.md", path: "product-brief.md", fileType: "product"},
	{filename: "policies.md", path: "policies.md", fileType: "reference"},
	{filename: "support-cases.md", path: "support-cases.md", fileType: "reference"},
	{filename: "website-copy.md", path: "website-copy.md", fileType: "reference"},
}

type attachment struct {
	filename string
	path     string
	fileType string
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
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
		return fmt.Errorf("begin Tempo seed transaction: %w", err)
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
		"Владельцы и управляющие небольших кафе в России, которые сейчас составляют графики смен вручную, захотят попробовать Tempo для создания и публикации расписания команды.",
	)
	if err != nil {
		return fmt.Errorf("insert Tempo test hypothesis: %w", err)
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
			$1, 'Tempo',
			'Сервис для создания и распространения рабочих расписаний небольших кафе.',
			'food', ARRAY['кафе', 'расписание смен'],
			'{"status":"unknown","description":"Цена не указана."}'::jsonb,
			$2, 'Tempo', 'Россия', NULL,
			'Проверить интерес владельцев и управляющих небольших кафе к пробному использованию Tempo для планирования смен.',
			'Запросить демонстрацию или начать пробное использование Tempo.',
			'{}',
			ARRAY['Не утверждать, что Tempo автоматически планирует смены.','Не утверждать, что Tempo прогнозирует спрос.'],
			ARRAY['Спрос и готовность платить не подтверждены.'],
			'user_provided', 'tempo_trial_interest', 'increase',
			'Уникальные владельцы или управляющие, запросившие демонстрацию или начавшие пробное использование',
			'Уникальные подходящие участники, которым было предложено попробовать Tempo',
			ARRAY['Фактическая цена и условия пробного периода не указаны.','Наличие интереса и готовность платить не подтверждены.']
		)
		ON CONFLICT (hypothesis_id) DO NOTHING`, productID, hypothesisID)
	if err != nil {
		return fmt.Errorf("insert Tempo test product: %w", err)
	}

	if err := attachProductFiles(ctx, transaction, s3Client, hypothesisID); err != nil {
		return err
	}

	var storedProductID string
	if err := transaction.QueryRow(ctx, "SELECT id FROM product WHERE hypothesis_id = $1", hypothesisID).Scan(&storedProductID); err != nil {
		return fmt.Errorf("verify Tempo test product: %w", err)
	}

	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit Tempo seed transaction: %w", err)
	}

	fmt.Printf("Tempo test pair is ready for %s (hypothesis %s, product %s)\n", userEmail, hypothesisID, storedProductID)
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

func attachProductFiles(ctx context.Context, transaction pgx.Tx, client *s3.Client, hypothesisID string) error {
	cafeDirectory, err := locateCafeDirectory()
	if err != nil {
		return err
	}

	for _, item := range attachments {
		data, err := os.ReadFile(filepath.Join(cafeDirectory, item.path))
		if err != nil {
			return fmt.Errorf("read Tempo attachment %s: %w", item.filename, err)
		}

		key := s3KeyPrefix + "/" + hypothesisID + "/" + item.filename
		contentType := "text/markdown; charset=utf-8"
		if _, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:        aws.String(envOrDefault("S3_BUCKET", s3Bucket)),
			Key:           aws.String(key),
			Body:          bytes.NewReader(data),
			ContentType:   aws.String(contentType),
			ContentLength: aws.Int64(int64(len(data))),
		}); err != nil {
			return fmt.Errorf("upload Tempo attachment %s: %w", item.filename, err)
		}

		textKey := key + ".extracted.txt"
		if _, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:        aws.String(envOrDefault("S3_BUCKET", s3Bucket)),
			Key:           aws.String(textKey),
			Body:          bytes.NewReader(data),
			ContentType:   aws.String("text/plain; charset=utf-8"),
			ContentLength: aws.Int64(int64(len(data))),
		}); err != nil {
			return fmt.Errorf("upload extracted Tempo attachment %s: %w", item.filename, err)
		}

		_, err = transaction.Exec(ctx, `
			INSERT INTO file (id, hypothesis_id, type, filename, content_type, size, s3_key)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (s3_key) DO NOTHING`,
			uuid.NewString(), hypothesisID, item.fileType, item.filename, contentType, len(data), key,
		)
		if err != nil {
			return fmt.Errorf("record Tempo attachment %s: %w", item.filename, err)
		}
	}

	return nil
}

func locateCafeDirectory() (string, error) {
	for _, path := range []string{"../n8n/product/cafe", "n8n/product/cafe"} {
		if info, err := os.Stat(filepath.Join(path, "product-brief.md")); err == nil && !info.IsDir() {
			return path, nil
		}
	}

	return "", errors.New("locate Tempo source documents: run from the repository root or backend directory")
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
