package db

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"app/internal/config"
)

//go:embed schema.sql
var schemaSQL string

type DB struct {
	cfg    *config.Config
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewDB(lc fx.Lifecycle, cfg *config.Config, logger *zap.Logger) *DB {
	database := &DB{
		cfg:    cfg,
		logger: logger,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			pool, err := pgxpool.New(ctx, database.cfg.DatabaseURL)
			if err != nil {
				return fmt.Errorf("connect to database: %w", err)
			}

			database.pool = pool

			database.logger.Warn("dropping database tables")

			if err := database.applySchema(ctx); err != nil {
				return err
			}

			database.logger.Info("schema applied")

			return nil
		},
		OnStop: func(_ context.Context) error {
			if database.pool != nil {
				database.pool.Close()
			}

			return nil
		},
	})

	return database
}

func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}

func (db *DB) applySchema(ctx context.Context) error {
	if _, err := db.pool.Exec(ctx, schemaSQL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}

	return nil
}
