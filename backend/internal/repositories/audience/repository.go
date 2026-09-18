package audience

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"app/internal/db"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
)

var squirrelPSQL = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

type Audience struct {
	ID             string
	ProductID      string
	GeneratorJobID string
	PersonaID      string
	Persona        json.RawMessage
	Research       json.RawMessage
	Metrics        json.RawMessage
	Hypotheses     json.RawMessage
	Rank           int
	Score          float64
	CreatedAt      time.Time
}

type GeneratedInput struct {
	PersonaID  string
	Persona    json.RawMessage
	Research   json.RawMessage
	Metrics    json.RawMessage
	Hypotheses json.RawMessage
	Rank       int
	Score      float64
}

type Repository struct {
	db *db.DB
}

func NewRepository(db *db.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListByProductID(ctx context.Context, productID string) ([]*Audience, error) {
	sql, args, err := squirrelPSQL.
		Select("id", "product_id", "generator_job_id", "persona_id", "persona", "research", "metrics", "hypotheses", "rank", "score", "created_at").
		From("audience").
		Where(squirrel.Eq{"product_id": productID}).
		OrderBy("rank").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list audiences query: %w", err)
	}

	rows, err := r.db.Pool().Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query audiences: %w", err)
	}
	defer rows.Close()

	var result []*Audience
	for rows.Next() {
		a := &Audience{}
		if err := rows.Scan(&a.ID, &a.ProductID, &a.GeneratorJobID, &a.PersonaID, &a.Persona, &a.Research, &a.Metrics, &a.Hypotheses, &a.Rank, &a.Score, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan audience: %w", err)
		}

		result = append(result, a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audiences: %w", err)
	}

	return result, nil
}

func (r *Repository) PersistGenerated(ctx context.Context, generatorJobID, productID string, inputs []GeneratedInput) error {
	tx, err := r.db.Pool().Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin persist generated audiences transaction: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	for _, in := range inputs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO audience (id, product_id, generator_job_id, persona_id, persona, research, metrics, hypotheses, rank, score)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (generator_job_id, persona_id) DO NOTHING
		`, uuid.NewString(), productID, generatorJobID, in.PersonaID, in.Persona, in.Research, in.Metrics, in.Hypotheses, in.Rank, in.Score); err != nil {
			return fmt.Errorf("insert generated audience %s: %w", in.PersonaID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit persist generated audiences transaction: %w", err)
	}

	return nil
}
