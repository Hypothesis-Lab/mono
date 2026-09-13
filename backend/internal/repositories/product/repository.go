package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"app/internal/db"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrNotFound = errors.New("product not found")

	squirrelPSQL = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
)

type Product struct {
	ID               string
	Name             string
	Description      string
	Category         string
	Brand            string
	Country          string
	Town             *string
	ExperimentGoal   string
	TargetAction     string
	Facts            json.RawMessage
	EligibilityRules json.RawMessage
	AllowedChannels  []string
	ForbiddenClaims  []string
	KnownConstraints []string
	Keywords         []string
	Price            *string
	Link             *string
	HypothesisID     string
}

type CreateInput struct {
	Name             string
	Description      string
	Category         string
	Brand            string
	Country          string
	Town             *string
	ExperimentGoal   string
	TargetAction     string
	Facts            json.RawMessage
	EligibilityRules json.RawMessage
	AllowedChannels  []string
	ForbiddenClaims  []string
	KnownConstraints []string
	Keywords         []string
	Price            *string
	Link             *string
	HypothesisID     string
}

type UpdateInput struct {
	Name             *string
	Description      *string
	Category         *string
	Brand            *string
	Country          *string
	Town             *string
	ExperimentGoal   *string
	TargetAction     *string
	Facts            *json.RawMessage
	EligibilityRules *json.RawMessage
	AllowedChannels  *[]string
	ForbiddenClaims  *[]string
	KnownConstraints *[]string
	Keywords         *[]string
	Price            *string
	Link             *string
}

type Repository struct {
	db *db.DB
}

func NewRepository(db *db.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, in CreateInput) (*Product, error) {
	if in.Keywords == nil {
		in.Keywords = []string{}
	}

	sql, args, err := squirrelPSQL.
		Insert("product").
		Columns("id", "name", "description", "category", "brand", "country", "town", "experiment_goal", "target_action", "facts", "eligibility_rules", "allowed_channels", "forbidden_claims", "known_constraints", "keywords", "price", "link", "hypothesis_id").
		Values(uuid.NewString(), in.Name, in.Description, in.Category, in.Brand, in.Country, in.Town, in.ExperimentGoal, in.TargetAction, in.Facts, in.EligibilityRules, in.AllowedChannels, in.ForbiddenClaims, in.KnownConstraints, in.Keywords, in.Price, in.Link, in.HypothesisID).
		Suffix("RETURNING id, name, description, category, brand, country, town, experiment_goal, target_action, facts, eligibility_rules, allowed_channels, forbidden_claims, known_constraints, keywords, price, link, hypothesis_id").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build create product query: %w", err)
	}

	p := &Product{}
	if err := r.db.Pool().QueryRow(ctx, sql, args...).
		Scan(&p.ID, &p.Name, &p.Description, &p.Category, &p.Brand, &p.Country, &p.Town, &p.ExperimentGoal, &p.TargetAction, &p.Facts, &p.EligibilityRules, &p.AllowedChannels, &p.ForbiddenClaims, &p.KnownConstraints, &p.Keywords, &p.Price, &p.Link, &p.HypothesisID); err != nil {
		return nil, fmt.Errorf("create product: %w", err)
	}

	return p, nil
}

func (r *Repository) Get(ctx context.Context, userID, id string) (*Product, error) {
	sql, args, err := squirrelPSQL.
		Select("p.id", "p.name", "p.description", "p.category", "p.brand", "p.country", "p.town", "p.experiment_goal", "p.target_action", "p.facts", "p.eligibility_rules", "p.allowed_channels", "p.forbidden_claims", "p.known_constraints", "p.keywords", "p.price", "p.link", "p.hypothesis_id").
		From("product p").
		Join("hypothesis h ON h.id = p.hypothesis_id").
		Where(squirrel.Eq{"p.id": id, "h.user_id": userID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get product query: %w", err)
	}

	p := &Product{}
	if err := r.db.Pool().QueryRow(ctx, sql, args...).Scan(&p.ID, &p.Name, &p.Description, &p.Category, &p.Brand, &p.Country, &p.Town, &p.ExperimentGoal, &p.TargetAction, &p.Facts, &p.EligibilityRules, &p.AllowedChannels, &p.ForbiddenClaims, &p.KnownConstraints, &p.Keywords, &p.Price, &p.Link, &p.HypothesisID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query product %s: %w", id, err)
	}

	return p, nil
}

func (r *Repository) GetByHypothesisID(ctx context.Context, hypothesisID string) (*Product, error) {
	sql, args, err := squirrelPSQL.
		Select("id", "name", "description", "category", "brand", "country", "town", "experiment_goal", "target_action", "facts", "eligibility_rules", "allowed_channels", "forbidden_claims", "known_constraints", "keywords", "price", "link", "hypothesis_id").
		From("product").
		Where(squirrel.Eq{"hypothesis_id": hypothesisID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get product by hypothesis query: %w", err)
	}

	p := &Product{}
	if err := r.db.Pool().QueryRow(ctx, sql, args...).Scan(&p.ID, &p.Name, &p.Description, &p.Category, &p.Brand, &p.Country, &p.Town, &p.ExperimentGoal, &p.TargetAction, &p.Facts, &p.EligibilityRules, &p.AllowedChannels, &p.ForbiddenClaims, &p.KnownConstraints, &p.Keywords, &p.Price, &p.Link, &p.HypothesisID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query product by hypothesis %s: %w", hypothesisID, err)
	}

	return p, nil
}

func (r *Repository) Update(ctx context.Context, userID, id string, in UpdateInput) (*Product, error) {
	if in.Name == nil && in.Description == nil && in.Category == nil && in.Brand == nil && in.Country == nil && in.Town == nil && in.ExperimentGoal == nil && in.TargetAction == nil && in.Facts == nil && in.EligibilityRules == nil && in.AllowedChannels == nil && in.ForbiddenClaims == nil && in.KnownConstraints == nil && in.Keywords == nil && in.Price == nil && in.Link == nil {
		return r.Get(ctx, userID, id)
	}

	builder := squirrelPSQL.
		Update("product").
		Where(squirrel.Eq{"product.id": id}).
		Where("EXISTS (SELECT 1 FROM hypothesis WHERE hypothesis.id = product.hypothesis_id AND hypothesis.user_id = ?)", userID).
		Suffix("RETURNING id, name, description, category, brand, country, town, experiment_goal, target_action, facts, eligibility_rules, allowed_channels, forbidden_claims, known_constraints, keywords, price, link, hypothesis_id")

	if in.Name != nil {
		builder = builder.Set("name", *in.Name)
	}

	if in.Description != nil {
		builder = builder.Set("description", *in.Description)
	}

	if in.Category != nil {
		builder = builder.Set("category", *in.Category)
	}

	if in.Brand != nil {
		builder = builder.Set("brand", *in.Brand)
	}

	if in.Country != nil {
		builder = builder.Set("country", *in.Country)
	}

	if in.Town != nil {
		builder = builder.Set("town", *in.Town)
	}

	if in.ExperimentGoal != nil {
		builder = builder.Set("experiment_goal", *in.ExperimentGoal)
	}

	if in.TargetAction != nil {
		builder = builder.Set("target_action", *in.TargetAction)
	}

	if in.Facts != nil {
		builder = builder.Set("facts", *in.Facts)
	}

	if in.EligibilityRules != nil {
		builder = builder.Set("eligibility_rules", *in.EligibilityRules)
	}

	if in.AllowedChannels != nil {
		builder = builder.Set("allowed_channels", *in.AllowedChannels)
	}

	if in.ForbiddenClaims != nil {
		builder = builder.Set("forbidden_claims", *in.ForbiddenClaims)
	}

	if in.KnownConstraints != nil {
		builder = builder.Set("known_constraints", *in.KnownConstraints)
	}

	if in.Keywords != nil {
		builder = builder.Set("keywords", *in.Keywords)
	}

	if in.Price != nil {
		builder = builder.Set("price", *in.Price)
	}

	if in.Link != nil {
		builder = builder.Set("link", *in.Link)
	}

	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build update product query: %w", err)
	}

	p := &Product{}
	if err := r.db.Pool().QueryRow(ctx, sql, args...).
		Scan(&p.ID, &p.Name, &p.Description, &p.Category, &p.Brand, &p.Country, &p.Town, &p.ExperimentGoal, &p.TargetAction, &p.Facts, &p.EligibilityRules, &p.AllowedChannels, &p.ForbiddenClaims, &p.KnownConstraints, &p.Keywords, &p.Price, &p.Link, &p.HypothesisID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("update product %s: %w", id, err)
	}

	return p, nil
}

func (r *Repository) Delete(ctx context.Context, userID, id string) error {
	sql, args, err := squirrelPSQL.
		Delete("product").
		Where(squirrel.Eq{"product.id": id}).
		Where("EXISTS (SELECT 1 FROM hypothesis WHERE hypothesis.id = product.hypothesis_id AND hypothesis.user_id = ?)", userID).
		ToSql()
	if err != nil {
		return fmt.Errorf("build delete product query: %w", err)
	}

	cmd, err := r.db.Pool().Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("delete product %s: %w", id, err)
	}

	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}
