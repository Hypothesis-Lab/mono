package adcampaign

import (
	"context"
	"errors"
	"fmt"

	"app/internal/db"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrNotFound = errors.New("ad campaign not found")

	squirrelPSQL = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
)

const ProviderYandex = "yandex"

type Campaign struct {
	ProductID  string
	Provider   string
	ExternalID int64
}

type Repository struct {
	db *db.DB
}

func NewRepository(db *db.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, campaign Campaign) error {
	sql, args, err := squirrelPSQL.
		Insert("ad_campaign").
		Columns("id", "product_id", "provider", "external_id").
		Values(uuid.NewString(), campaign.ProductID, campaign.Provider, campaign.ExternalID).
		ToSql()
	if err != nil {
		return fmt.Errorf("build create ad campaign query: %w", err)
	}

	if _, err := r.db.Pool().Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("create ad campaign: %w", err)
	}

	return nil
}

func (r *Repository) GetByProductID(ctx context.Context, productID, provider string) (*Campaign, error) {
	sql, args, err := squirrelPSQL.
		Select("product_id", "provider", "external_id").
		From("ad_campaign").
		Where(squirrel.Eq{"product_id": productID, "provider": provider}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get ad campaign query: %w", err)
	}

	campaign := &Campaign{}
	if err := r.db.Pool().QueryRow(ctx, sql, args...).Scan(&campaign.ProductID, &campaign.Provider, &campaign.ExternalID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query ad campaign for product %s: %w", productID, err)
	}

	return campaign, nil
}

func (r *Repository) Delete(ctx context.Context, productID, provider string) error {
	sql, args, err := squirrelPSQL.
		Delete("ad_campaign").
		Where(squirrel.Eq{"product_id": productID, "provider": provider}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build delete ad campaign query: %w", err)
	}

	result, err := r.db.Pool().Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("delete ad campaign for product %s: %w", productID, err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}
