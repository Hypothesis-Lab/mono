package audience

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

var (
	ErrNotFound = errors.New("audience not found")

	squirrelPSQL = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
)

type Audience struct {
	ID                string
	ProductID         string
	Description       string
	AgeFrom           *int32
	AgeTo             *int32
	Gender            *string
	Interests         *string
	Geography         *string
	PainPoints        *string
	DesiresMotivation *string
	CreatedAt         time.Time
}

type CreateInput struct {
	ProductID         string
	Description       string
	AgeFrom           *int32
	AgeTo             *int32
	Gender            *string
	Interests         *string
	Geography         *string
	PainPoints        *string
	DesiresMotivation *string
}

type UpdateInput struct {
	Description       *string
	AgeFrom           *int32
	AgeTo             *int32
	Gender            *string
	Interests         *string
	Geography         *string
	PainPoints        *string
	DesiresMotivation *string
}

type Repository struct {
	db *db.DB
}

func NewRepository(db *db.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListByProductID(ctx context.Context, productID string) ([]*Audience, error) {
	sql, args, err := squirrelPSQL.
		Select("id", "product_id", "description", "age_from", "age_to", "gender", "interests", "geography", "pain_points", "desires_motivation", "created_at").
		From("target_audience").
		Where(squirrel.Eq{"product_id": productID}).
		OrderBy("created_at").
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
		if err := rows.Scan(&a.ID, &a.ProductID, &a.Description, &a.AgeFrom, &a.AgeTo, &a.Gender, &a.Interests, &a.Geography, &a.PainPoints, &a.DesiresMotivation, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan audience: %w", err)
		}

		result = append(result, a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audiences: %w", err)
	}

	return result, nil
}

func (r *Repository) Create(ctx context.Context, in CreateInput) (*Audience, error) {
	sql, args, err := squirrelPSQL.
		Insert("target_audience").
		Columns("id", "product_id", "description", "age_from", "age_to", "gender", "interests", "geography", "pain_points", "desires_motivation").
		Values(uuid.NewString(), in.ProductID, in.Description, in.AgeFrom, in.AgeTo, in.Gender, in.Interests, in.Geography, in.PainPoints, in.DesiresMotivation).
		Suffix("RETURNING id, product_id, description, age_from, age_to, gender, interests, geography, pain_points, desires_motivation, created_at").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build create audience query: %w", err)
	}

	a := &Audience{}
	if err := r.db.Pool().QueryRow(ctx, sql, args...).
		Scan(&a.ID, &a.ProductID, &a.Description, &a.AgeFrom, &a.AgeTo, &a.Gender, &a.Interests, &a.Geography, &a.PainPoints, &a.DesiresMotivation, &a.CreatedAt); err != nil {
		return nil, fmt.Errorf("create audience: %w", err)
	}

	return a, nil
}

func (r *Repository) Get(ctx context.Context, userID, id string) (*Audience, error) {
	sql, args, err := squirrelPSQL.
		Select("id", "product_id", "description", "age_from", "age_to", "gender", "interests", "geography", "pain_points", "desires_motivation", "created_at").
		From("target_audience").
		Where(squirrel.Eq{"id": id}).
		Where("EXISTS (SELECT 1 FROM product JOIN hypothesis ON hypothesis.id = product.hypothesis_id WHERE product.id = target_audience.product_id AND hypothesis.user_id = ?)", userID).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get audience query: %w", err)
	}

	a := &Audience{}
	if err := r.db.Pool().QueryRow(ctx, sql, args...).
		Scan(&a.ID, &a.ProductID, &a.Description, &a.AgeFrom, &a.AgeTo, &a.Gender, &a.Interests, &a.Geography, &a.PainPoints, &a.DesiresMotivation, &a.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query audience %s: %w", id, err)
	}

	return a, nil
}

func (r *Repository) Update(ctx context.Context, userID, id string, in UpdateInput) (*Audience, error) {
	if in.Description == nil && in.AgeFrom == nil && in.AgeTo == nil && in.Gender == nil && in.Interests == nil && in.Geography == nil && in.PainPoints == nil && in.DesiresMotivation == nil {
		return r.Get(ctx, userID, id)
	}

	builder := squirrelPSQL.
		Update("target_audience").
		Where(squirrel.Eq{"id": id}).
		Where("EXISTS (SELECT 1 FROM product JOIN hypothesis ON hypothesis.id = product.hypothesis_id WHERE product.id = target_audience.product_id AND hypothesis.user_id = ?)", userID).
		Suffix("RETURNING id, product_id, description, age_from, age_to, gender, interests, geography, pain_points, desires_motivation, created_at")

	if in.Description != nil {
		builder = builder.Set("description", *in.Description)
	}

	if in.AgeFrom != nil {
		builder = builder.Set("age_from", *in.AgeFrom)
	}

	if in.AgeTo != nil {
		builder = builder.Set("age_to", *in.AgeTo)
	}

	if in.Gender != nil {
		builder = builder.Set("gender", *in.Gender)
	}

	if in.Interests != nil {
		builder = builder.Set("interests", *in.Interests)
	}

	if in.Geography != nil {
		builder = builder.Set("geography", *in.Geography)
	}

	if in.PainPoints != nil {
		builder = builder.Set("pain_points", *in.PainPoints)
	}

	if in.DesiresMotivation != nil {
		builder = builder.Set("desires_motivation", *in.DesiresMotivation)
	}

	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build update audience query: %w", err)
	}

	a := &Audience{}
	if err := r.db.Pool().QueryRow(ctx, sql, args...).
		Scan(&a.ID, &a.ProductID, &a.Description, &a.AgeFrom, &a.AgeTo, &a.Gender, &a.Interests, &a.Geography, &a.PainPoints, &a.DesiresMotivation, &a.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("update audience %s: %w", id, err)
	}

	return a, nil
}

func (r *Repository) Delete(ctx context.Context, userID, id string) error {
	sql, args, err := squirrelPSQL.
		Delete("target_audience").
		Where(squirrel.Eq{"id": id}).
		Where("EXISTS (SELECT 1 FROM product JOIN hypothesis ON hypothesis.id = product.hypothesis_id WHERE product.id = target_audience.product_id AND hypothesis.user_id = ?)", userID).
		ToSql()
	if err != nil {
		return fmt.Errorf("build delete audience query: %w", err)
	}

	cmd, err := r.db.Pool().Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("delete audience %s: %w", id, err)
	}

	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}
