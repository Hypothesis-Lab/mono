package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                           int
	DatabaseURL                    string
	JWTSecret                      string
	S3Endpoint                     string
	S3Region                       string
	S3Bucket                       string
	S3AccessKeyID                  string
	S3SecretAccessKey              string
	YandexDirectClientID           string
	YandexDirectClientSecret       string
	YandexDirectTokenEncryptionKey [32]byte
	GeneratorURL                   string
	OpenRouterAPIKey               string
	TemporalAddress                string
	TemporalNamespace              string
	IsDevLogger                    bool

	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

//nolint:cyclop // Config must validate every required environment variable.
func New() (*Config, error) {
	portStr := os.Getenv("PORT")
	if portStr == "" {
		return nil, errors.New("PORT is required")
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, errors.New("JWT_SECRET is required")
	}

	s3Endpoint := os.Getenv("S3_ENDPOINT")
	if s3Endpoint == "" {
		return nil, errors.New("S3_ENDPOINT is required")
	}

	s3Region := os.Getenv("S3_REGION")
	if s3Region == "" {
		return nil, errors.New("S3_REGION is required")
	}

	s3Bucket := os.Getenv("S3_BUCKET")
	if s3Bucket == "" {
		return nil, errors.New("S3_BUCKET is required")
	}

	s3AccessKeyID := os.Getenv("S3_ACCESS_KEY_ID")
	if s3AccessKeyID == "" {
		return nil, errors.New("S3_ACCESS_KEY_ID is required")
	}

	s3SecretAccessKey := os.Getenv("S3_SECRET_ACCESS_KEY")
	if s3SecretAccessKey == "" {
		return nil, errors.New("S3_SECRET_ACCESS_KEY is required")
	}

	yandexDirectClientID := os.Getenv("YANDEX_DIRECT_CLIENT_ID")
	if yandexDirectClientID == "" {
		return nil, errors.New("YANDEX_DIRECT_CLIENT_ID is required")
	}

	yandexDirectClientSecret := os.Getenv("YANDEX_DIRECT_CLIENT_SECRET")
	if yandexDirectClientSecret == "" {
		return nil, errors.New("YANDEX_DIRECT_CLIENT_SECRET is required")
	}

	yandexDirectTokenEncryptionKey, err := tokenEncryptionKey(os.Getenv("YANDEX_DIRECT_TOKEN_ENCRYPTION_KEY"))
	if err != nil {
		return nil, err
	}

	generatorURL := os.Getenv("GENERATOR_URL")
	if generatorURL == "" {
		return nil, errors.New("GENERATOR_URL is required")
	}

	openRouterAPIKey := os.Getenv("OPENROUTER_API_KEY")
	if openRouterAPIKey == "" {
		return nil, errors.New("OPENROUTER_API_KEY is required")
	}

	temporalAddress := os.Getenv("TEMPORAL_ADDRESS")
	if temporalAddress == "" {
		return nil, errors.New("TEMPORAL_ADDRESS is required")
	}

	devLogger := os.Getenv("DEV_LOGGER")

	return &Config{
		Port:                           port,
		DatabaseURL:                    databaseURL,
		JWTSecret:                      jwtSecret,
		S3Endpoint:                     s3Endpoint,
		S3Region:                       s3Region,
		S3Bucket:                       s3Bucket,
		S3AccessKeyID:                  s3AccessKeyID,
		S3SecretAccessKey:              s3SecretAccessKey,
		YandexDirectClientID:           yandexDirectClientID,
		YandexDirectClientSecret:       yandexDirectClientSecret,
		YandexDirectTokenEncryptionKey: yandexDirectTokenEncryptionKey,
		GeneratorURL:                   generatorURL,
		OpenRouterAPIKey:               openRouterAPIKey,
		TemporalAddress:                temporalAddress,
		TemporalNamespace:              os.Getenv("TEMPORAL_NAMESPACE"),
		IsDevLogger:                    devLogger == "true",
		ReadHeaderTimeout:              envDuration("READ_HEADER_TIMEOUT", 10*time.Second),
		ReadTimeout:                    envDuration("READ_TIMEOUT", 30*time.Second),
		WriteTimeout:                   envDuration("WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:                    envDuration("IDLE_TIMEOUT", 120*time.Second),
	}, nil
}

func tokenEncryptionKey(encoded string) ([32]byte, error) {
	if encoded == "" {
		return [32]byte{}, errors.New("YANDEX_DIRECT_TOKEN_ENCRYPTION_KEY is required")
	}

	decoded, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil {
		return [32]byte{}, fmt.Errorf("YANDEX_DIRECT_TOKEN_ENCRYPTION_KEY must be valid base64: %w", err)
	}

	if len(decoded) != 32 {
		return [32]byte{}, fmt.Errorf("YANDEX_DIRECT_TOKEN_ENCRYPTION_KEY must decode to exactly 32 bytes, got %d", len(decoded))
	}

	var key [32]byte
	copy(key[:], decoded)

	return key, nil
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}

	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}

	return d
}
