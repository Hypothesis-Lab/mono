package productfeedbacksurvey

import (
	"context"
	"encoding/json"
	"fmt"

	"app/internal/db"
)

type Result struct {
	PersonaID   string
	PersonaName string
	Response    json.RawMessage
}

type Repository struct {
	db *db.DB
}

func NewRepository(db *db.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) DeleteAll(ctx context.Context) error {
	if _, err := r.db.Pool().Exec(ctx, "DELETE FROM product_feedback_survey_result"); err != nil {
		return fmt.Errorf("delete product feedback survey results: %w", err)
	}

	return nil
}

func (r *Repository) Save(ctx context.Context, results []Result) error {
	tx, err := r.db.Pool().Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin save product feedback survey results transaction: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	for _, result := range results {
		if _, err := tx.Exec(ctx, `
			INSERT INTO product_feedback_survey_result (persona_id, persona_name, response)
			VALUES ($1, $2, $3)
			ON CONFLICT (persona_id) DO UPDATE SET
				persona_name = EXCLUDED.persona_name,
				response = EXCLUDED.response,
				updated_at = NOW()
		`, result.PersonaID, result.PersonaName, result.Response); err != nil {
			return fmt.Errorf("save product feedback survey result %s: %w", result.PersonaID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit product feedback survey results: %w", err)
	}

	return nil
}

func (r *Repository) List(ctx context.Context) ([]*Result, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT persona_id, persona_name, response
		FROM product_feedback_survey_result
		ORDER BY persona_id
	`)
	if err != nil {
		return nil, fmt.Errorf("query product feedback survey results: %w", err)
	}
	defer rows.Close()

	var results []*Result
	for rows.Next() {
		result := &Result{}
		if err := rows.Scan(&result.PersonaID, &result.PersonaName, &result.Response); err != nil {
			return nil, fmt.Errorf("scan product feedback survey result: %w", err)
		}

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate product feedback survey results: %w", err)
	}

	return results, nil
}
