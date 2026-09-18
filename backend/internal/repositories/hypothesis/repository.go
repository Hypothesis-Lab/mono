package hypothesis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"app/internal/db"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	StatusIdle       Status = "idle"
	StatusInProgress Status = "in_progress"
)

var (
	ErrNotFound = errors.New("hypothesis not found")

	squirrelPSQL = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
)

type Status string

type Hypothesis struct {
	ID          string
	UserID      string
	Description string
	Status      Status
	CreatedAt   time.Time
}

type CreateInput struct {
	UserID      string
	Description string
}

type UpdateInput struct {
	Description *string
}

type Repository struct {
	db *db.DB
}

func NewRepository(db *db.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context, userID string) ([]*Hypothesis, error) {
	sql, args, err := squirrelPSQL.
		Select("id", "user_id", "description", "status", "created_at").
		From("hypothesis").
		Where(squirrel.Eq{"user_id": userID}).
		OrderBy("id").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list hypotheses query: %w", err)
	}

	rows, err := r.db.Pool().Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query hypotheses: %w", err)
	}
	defer rows.Close()

	var result []*Hypothesis
	for rows.Next() {
		h := &Hypothesis{}
		if err := rows.Scan(&h.ID, &h.UserID, &h.Description, &h.Status, &h.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan hypothesis: %w", err)
		}

		result = append(result, h)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate hypotheses: %w", err)
	}

	return result, nil
}

func (r *Repository) Create(ctx context.Context, in CreateInput) (*Hypothesis, error) {
	sql, args, err := squirrelPSQL.
		Insert("hypothesis").
		Columns("id", "user_id", "description", "status").
		Values(uuid.NewString(), in.UserID, in.Description, StatusIdle).
		Suffix("RETURNING id, user_id, description, status, created_at").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build create hypothesis query: %w", err)
	}

	h := &Hypothesis{}
	if err := r.db.Pool().QueryRow(ctx, sql, args...).
		Scan(&h.ID, &h.UserID, &h.Description, &h.Status, &h.CreatedAt); err != nil {
		return nil, fmt.Errorf("create hypothesis: %w", err)
	}

	return h, nil
}

func (r *Repository) Get(ctx context.Context, userID, id string) (*Hypothesis, error) {
	sql, args, err := squirrelPSQL.
		Select("id", "user_id", "description", "status", "created_at").
		From("hypothesis").
		Where(squirrel.Eq{"id": id, "user_id": userID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get hypothesis query: %w", err)
	}

	h := &Hypothesis{}
	if err := r.db.Pool().QueryRow(ctx, sql, args...).
		Scan(&h.ID, &h.UserID, &h.Description, &h.Status, &h.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query hypothesis %s: %w", id, err)
	}

	return h, nil
}

func (r *Repository) Update(ctx context.Context, userID, id string, in UpdateInput) (*Hypothesis, error) {
	if in.Description == nil {
		return r.Get(ctx, userID, id)
	}

	builder := squirrelPSQL.
		Update("hypothesis").
		Where(squirrel.Eq{"id": id, "user_id": userID}).
		Suffix("RETURNING id, user_id, description, status, created_at")

	if in.Description != nil {
		builder = builder.Set("description", *in.Description)
	}

	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build update hypothesis query: %w", err)
	}

	h := &Hypothesis{}
	if err := r.db.Pool().QueryRow(ctx, sql, args...).
		Scan(&h.ID, &h.UserID, &h.Description, &h.Status, &h.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("update hypothesis %s: %w", id, err)
	}

	return h, nil
}

func (r *Repository) Delete(ctx context.Context, userID, id string) error {
	sql, args, err := squirrelPSQL.
		Delete("hypothesis").
		Where(squirrel.Eq{"id": id, "user_id": userID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build delete hypothesis query: %w", err)
	}

	cmd, err := r.db.Pool().Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("delete hypothesis %s: %w", id, err)
	}

	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *Repository) SetStatus(ctx context.Context, userID, id string, status Status) (*Hypothesis, error) {
	sql, args, err := squirrelPSQL.
		Update("hypothesis").
		Set("status", status).
		Where(squirrel.Eq{"id": id, "user_id": userID}).
		Suffix("RETURNING id, user_id, description, status, created_at").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build set status hypothesis query: %w", err)
	}

	h := &Hypothesis{}
	if err := r.db.Pool().QueryRow(ctx, sql, args...).
		Scan(&h.ID, &h.UserID, &h.Description, &h.Status, &h.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("set status hypothesis %s: %w", id, err)
	}

	return h, nil
}
