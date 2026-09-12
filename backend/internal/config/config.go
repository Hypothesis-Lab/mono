package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                     int
	DatabaseURL              string
	JWTSecret                string
	S3Endpoint               string
	S3Region                 string
	S3Bucket                 string
	S3AccessKeyID            string
	S3SecretAccessKey        string
	YandexDirectClientID     string
	YandexDirectClientSecret string
	GeneratorURL             string
	IsDevLogger              bool
	DevSeedDB                bool

	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

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

	generatorURL := os.Getenv("GENERATOR_URL")
	if generatorURL == "" {
		return nil, errors.New("GENERATOR_URL is required")
	}

	devLogger := os.Getenv("DEV_LOGGER")
	devSeedDB := os.Getenv("DEV_SEED_DB")

	return &Config{
		Port:                     port,
		DatabaseURL:              databaseURL,
		JWTSecret:                jwtSecret,
		S3Endpoint:               s3Endpoint,
		S3Region:                 s3Region,
		S3Bucket:                 s3Bucket,
		S3AccessKeyID:            s3AccessKeyID,
		S3SecretAccessKey:        s3SecretAccessKey,
		YandexDirectClientID:     yandexDirectClientID,
		YandexDirectClientSecret: yandexDirectClientSecret,
		GeneratorURL:             generatorURL,
		IsDevLogger:              devLogger == "true",
		DevSeedDB:                devSeedDB == "true",
		ReadHeaderTimeout:        envDuration("READ_HEADER_TIMEOUT", 10*time.Second),
		ReadTimeout:              envDuration("READ_TIMEOUT", 30*time.Second),
		WriteTimeout:             envDuration("WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:              envDuration("IDLE_TIMEOUT", 120*time.Second),
	}, nil
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
