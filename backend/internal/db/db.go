package db

import (
	"context"
	"embed"
	"errors"
	"fmt"

	gomigrate "github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"app/internal/config"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

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

			if err := database.applyMigrations(); err != nil {
				return err
			}

			database.logger.Info("database migrations applied")

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

func (db *DB) applyMigrations() error {
	migrationConfig, err := pgx.ParseConfig(db.cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("parse migration database URL: %w", err)
	}
	migrationDB := stdlib.OpenDB(*migrationConfig)
	defer migrationDB.Close()

	source, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("load embedded migrations: %w", err)
	}

	driver, err := pgxmigrate.WithInstance(migrationDB, &pgxmigrate.Config{
		MigrationsTable: "schema_migrations",
		SchemaName:      "public",
	})
	if err != nil {
		return fmt.Errorf("create migration database driver: %w", err)
	}

	migrator, err := gomigrate.NewWithInstance("embedded", source, "pgx", driver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer migrator.Close()

	if err := migrator.Up(); err != nil && !errors.Is(err, gomigrate.ErrNoChange) {
		return fmt.Errorf("apply database migrations: %w", err)
	}

	return nil
}
