package logger

import (
	"app/internal/config"

	"go.uber.org/zap"
)

func New(cfg *config.Config) (*zap.Logger, error) {
	if cfg.IsDevLogger {
		return zap.NewDevelopment()
	}

	return zap.NewProduction()
}
