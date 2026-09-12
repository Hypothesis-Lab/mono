package storage

import (
	"context"
	"errors"
	"fmt"
	"io"

	"app/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type S3 struct {
	client *s3.Client
	bucket string
	logger *zap.Logger
}

func NewS3(lc fx.Lifecycle, cfg *config.Config, logger *zap.Logger) (*S3, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(cfg.S3Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.S3AccessKeyID, cfg.S3SecretAccessKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.S3Endpoint)
		o.UsePathStyle = true
	})

	storage := &S3{
		client: client,
		bucket: cfg.S3Bucket,
		logger: logger,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			storage.logger.Info("ensuring s3 bucket", zap.String("bucket", storage.bucket))

			if _, err := storage.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(storage.bucket)}); err != nil {
				var apiErr smithy.APIError
				if !errors.As(err, &apiErr) || (apiErr.ErrorCode() != "BucketAlreadyExists" && apiErr.ErrorCode() != "BucketAlreadyOwnedByYou") {
					return fmt.Errorf("create bucket: %w", err)
				}
			}

			return nil
		},
	})

	return storage, nil
}

func (s *S3) Upload(ctx context.Context, key string, contentType string, size int64, body io.Reader) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		return fmt.Errorf("put object %s: %w", key, err)
	}

	return nil
}

func (s *S3) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete object %s: %w", key, err)
	}

	return nil
}
