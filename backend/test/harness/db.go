package harness

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type testDB struct {
	pool *pgxpool.Pool
}

func resetDatabase(databaseURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	db := &testDB{pool: pool}

	if _, err := db.pool.Exec(ctx, "TRUNCATE TABLE users, hypothesis, product, audience, file, ad_campaign, hypothesis_launch, hypothesis_launch_resource, temporal_outbox, hypothesis_stop, temporal_stop_outbox, product_feedback_survey_result CASCADE"); err != nil {
		return fmt.Errorf("truncate tables: %w", err)
	}

	return nil
}
