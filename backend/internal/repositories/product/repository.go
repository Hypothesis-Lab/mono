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
	ErrCustomCategoryNotAllowed = errors.New("custom_category is only allowed when category is other")
	ErrCustomCategoryRequired   = errors.New("custom_category is required when category is other")
	ErrNotFound                 = errors.New("product not found")

	squirrelPSQL = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
)

type Product struct {
	ID                    string
	Name                  string
	Description           string
	Category              string
	CustomCategory        *string
	Brand                 string
	Country               string
	Town                  *string
	ExperimentGoal        string
	GoalOrigin            string
	TargetAction          string
	PrimaryMetric         string
	MetricDirection       string
	MetricNumerator       string
	MetricDenominator     string
	SuccessCriterion      *string
	Facts                 json.RawMessage
	KeyFactIDs            []string
	Price                 json.RawMessage
	FactsCheckedAt        *string
	EligibilityRules      json.RawMessage
	AllowedChannels       []string
	ContactablePersonaIDs []string
	CampaignNote          string
	ForbiddenClaims       []string
	KnownConstraints      []string
	Keywords              []string
	Unknowns              []string
	Link                  *string
	HypothesisID          string
}

type CreateInput struct {
	Name                  string
	Description           string
	Category              string
	CustomCategory        *string
	Brand                 string
	Country               string
	Town                  *string
	ExperimentGoal        string
	GoalOrigin            string
	TargetAction          string
	PrimaryMetric         string
	MetricDirection       string
	MetricNumerator       string
	MetricDenominator     string
	SuccessCriterion      *string
	Facts                 json.RawMessage
	KeyFactIDs            []string
	Price                 json.RawMessage
	FactsCheckedAt        *string
	EligibilityRules      json.RawMessage
	AllowedChannels       []string
	ContactablePersonaIDs []string
	CampaignNote          string
	ForbiddenClaims       []string
	KnownConstraints      []string
	Keywords              []string
	Unknowns              []string
	Link                  *string
	HypothesisID          string
}

type UpdateInput struct {
	Name                  *string
	Description           *string
	Category              *string
	CustomCategory        *string
	ClearCustomCategory   bool
	Brand                 *string
	Country               *string
	Town                  *string
	ExperimentGoal        *string
	GoalOrigin            *string
	TargetAction          *string
	PrimaryMetric         *string
	MetricDirection       *string
	MetricNumerator       *string
	MetricDenominator     *string
	SuccessCriterion      *string
	Facts                 *json.RawMessage
	KeyFactIDs            *[]string
	Price                 *json.RawMessage
	FactsCheckedAt        *string
	EligibilityRules      *json.RawMessage
	AllowedChannels       *[]string
	ContactablePersonaIDs *[]string
	CampaignNote          *string
	ForbiddenClaims       *[]string
	KnownConstraints      *[]string
	Keywords              *[]string
	Unknowns              *[]string
	Link                  *string
}

type Repository struct {
	db *db.DB
}

type updateField struct {
	column string
	value  any
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
		Columns("id", "name", "description", "category", "custom_category", "brand", "country", "town", "experiment_goal", "goal_origin", "target_action", "primary_metric", "metric_direction", "metric_numerator", "metric_denominator", "success_criterion", "facts", "key_fact_ids", "price", "facts_checked_at", "eligibility_rules", "allowed_channels", "contactable_persona_ids", "campaign_note", "forbidden_claims", "known_constraints", "keywords", "unknowns", "link", "hypothesis_id").
		Values(uuid.NewString(), in.Name, in.Description, in.Category, in.CustomCategory, in.Brand, in.Country, in.Town, in.ExperimentGoal, in.GoalOrigin, in.TargetAction, in.PrimaryMetric, in.MetricDirection, in.MetricNumerator, in.MetricDenominator, in.SuccessCriterion, in.Facts, in.KeyFactIDs, in.Price, in.FactsCheckedAt, in.EligibilityRules, in.AllowedChannels, in.ContactablePersonaIDs, in.CampaignNote, in.ForbiddenClaims, in.KnownConstraints, in.Keywords, in.Unknowns, in.Link, in.HypothesisID).
		Suffix("RETURNING id, name, description, category, custom_category, brand, country, town, experiment_goal, goal_origin, target_action, primary_metric, metric_direction, metric_numerator, metric_denominator, success_criterion, facts, key_fact_ids, price, facts_checked_at, eligibility_rules, allowed_channels, contactable_persona_ids, campaign_note, forbidden_claims, known_constraints, keywords, unknowns, link, hypothesis_id").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build create product query: %w", err)
	}

	p := &Product{}
	if err := r.db.Pool().QueryRow(ctx, sql, args...).
		Scan(&p.ID, &p.Name, &p.Description, &p.Category, &p.CustomCategory, &p.Brand, &p.Country, &p.Town, &p.ExperimentGoal, &p.GoalOrigin, &p.TargetAction, &p.PrimaryMetric, &p.MetricDirection, &p.MetricNumerator, &p.MetricDenominator, &p.SuccessCriterion, &p.Facts, &p.KeyFactIDs, &p.Price, &p.FactsCheckedAt, &p.EligibilityRules, &p.AllowedChannels, &p.ContactablePersonaIDs, &p.CampaignNote, &p.ForbiddenClaims, &p.KnownConstraints, &p.Keywords, &p.Unknowns, &p.Link, &p.HypothesisID); err != nil {
		return nil, fmt.Errorf("create product: %w", err)
	}

	return p, nil
}

func (r *Repository) Get(ctx context.Context, userID, id string) (*Product, error) {
	sql, args, err := squirrelPSQL.
		Select("p.id", "p.name", "p.description", "p.category", "p.custom_category", "p.brand", "p.country", "p.town", "p.experiment_goal", "p.goal_origin", "p.target_action", "p.primary_metric", "p.metric_direction", "p.metric_numerator", "p.metric_denominator", "p.success_criterion", "p.facts", "p.key_fact_ids", "p.price", "p.facts_checked_at", "p.eligibility_rules", "p.allowed_channels", "p.contactable_persona_ids", "p.campaign_note", "p.forbidden_claims", "p.known_constraints", "p.keywords", "p.unknowns", "p.link", "p.hypothesis_id").
		From("product p").
		Join("hypothesis h ON h.id = p.hypothesis_id").
		Where(squirrel.Eq{"p.id": id, "h.user_id": userID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get product query: %w", err)
	}

	p := &Product{}
	if err := r.db.Pool().QueryRow(ctx, sql, args...).Scan(&p.ID, &p.Name, &p.Description, &p.Category, &p.CustomCategory, &p.Brand, &p.Country, &p.Town, &p.ExperimentGoal, &p.GoalOrigin, &p.TargetAction, &p.PrimaryMetric, &p.MetricDirection, &p.MetricNumerator, &p.MetricDenominator, &p.SuccessCriterion, &p.Facts, &p.KeyFactIDs, &p.Price, &p.FactsCheckedAt, &p.EligibilityRules, &p.AllowedChannels, &p.ContactablePersonaIDs, &p.CampaignNote, &p.ForbiddenClaims, &p.KnownConstraints, &p.Keywords, &p.Unknowns, &p.Link, &p.HypothesisID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query product %s: %w", id, err)
	}

	return p, nil
}

func (r *Repository) GetByHypothesisID(ctx context.Context, hypothesisID string) (*Product, error) {
	sql, args, err := squirrelPSQL.
		Select("id", "name", "description", "category", "custom_category", "brand", "country", "town", "experiment_goal", "goal_origin", "target_action", "primary_metric", "metric_direction", "metric_numerator", "metric_denominator", "success_criterion", "facts", "key_fact_ids", "price", "facts_checked_at", "eligibility_rules", "allowed_channels", "contactable_persona_ids", "campaign_note", "forbidden_claims", "known_constraints", "keywords", "unknowns", "link", "hypothesis_id").
		From("product").
		Where(squirrel.Eq{"hypothesis_id": hypothesisID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get product by hypothesis query: %w", err)
	}

	p := &Product{}
	if err := r.db.Pool().QueryRow(ctx, sql, args...).Scan(&p.ID, &p.Name, &p.Description, &p.Category, &p.CustomCategory, &p.Brand, &p.Country, &p.Town, &p.ExperimentGoal, &p.GoalOrigin, &p.TargetAction, &p.PrimaryMetric, &p.MetricDirection, &p.MetricNumerator, &p.MetricDenominator, &p.SuccessCriterion, &p.Facts, &p.KeyFactIDs, &p.Price, &p.FactsCheckedAt, &p.EligibilityRules, &p.AllowedChannels, &p.ContactablePersonaIDs, &p.CampaignNote, &p.ForbiddenClaims, &p.KnownConstraints, &p.Keywords, &p.Unknowns, &p.Link, &p.HypothesisID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query product by hypothesis %s: %w", hypothesisID, err)
	}

	return p, nil
}

func (r *Repository) Update(ctx context.Context, userID, id string, in UpdateInput) (*Product, error) {
	builder := squirrelPSQL.
		Update("product").
		Where(squirrel.Eq{"product.id": id}).
		Where("EXISTS (SELECT 1 FROM hypothesis WHERE hypothesis.id = product.hypothesis_id AND hypothesis.user_id = ?)", userID).
		Suffix("RETURNING id, name, description, category, custom_category, brand, country, town, experiment_goal, goal_origin, target_action, primary_metric, metric_direction, metric_numerator, metric_denominator, success_criterion, facts, key_fact_ids, price, facts_checked_at, eligibility_rules, allowed_channels, contactable_persona_ids, campaign_note, forbidden_claims, known_constraints, keywords, unknowns, link, hypothesis_id")

	builder, updates := applyUpdateFields(builder, in)
	if updates == 0 {
		return r.Get(ctx, userID, id)
	}

	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build update product query: %w", err)
	}

	p := &Product{}
	if err := r.db.Pool().QueryRow(ctx, sql, args...).
		Scan(&p.ID, &p.Name, &p.Description, &p.Category, &p.CustomCategory, &p.Brand, &p.Country, &p.Town, &p.ExperimentGoal, &p.GoalOrigin, &p.TargetAction, &p.PrimaryMetric, &p.MetricDirection, &p.MetricNumerator, &p.MetricDenominator, &p.SuccessCriterion, &p.Facts, &p.KeyFactIDs, &p.Price, &p.FactsCheckedAt, &p.EligibilityRules, &p.AllowedChannels, &p.ContactablePersonaIDs, &p.CampaignNote, &p.ForbiddenClaims, &p.KnownConstraints, &p.Keywords, &p.Unknowns, &p.Link, &p.HypothesisID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("update product %s: %w", id, err)
	}

	return p, nil
}

func applyUpdateFields(builder squirrel.UpdateBuilder, in UpdateInput) (squirrel.UpdateBuilder, int) {
	fields := []updateField{
		{"name", in.Name},
		{"description", in.Description},
		{"category", in.Category},
		{"custom_category", in.CustomCategory},
		{"brand", in.Brand},
		{"country", in.Country},
		{"town", in.Town},
		{"experiment_goal", in.ExperimentGoal},
		{"goal_origin", in.GoalOrigin},
		{"target_action", in.TargetAction},
		{"primary_metric", in.PrimaryMetric},
		{"metric_direction", in.MetricDirection},
		{"metric_numerator", in.MetricNumerator},
		{"metric_denominator", in.MetricDenominator},
		{"success_criterion", in.SuccessCriterion},
		{"facts", in.Facts},
		{"key_fact_ids", in.KeyFactIDs},
		{"price", in.Price},
		{"facts_checked_at", in.FactsCheckedAt},
		{"eligibility_rules", in.EligibilityRules},
		{"allowed_channels", in.AllowedChannels},
		{"contactable_persona_ids", in.ContactablePersonaIDs},
		{"campaign_note", in.CampaignNote},
		{"forbidden_claims", in.ForbiddenClaims},
		{"known_constraints", in.KnownConstraints},
		{"keywords", in.Keywords},
		{"unknowns", in.Unknowns},
		{"link", in.Link},
	}

	updates := 0
	for _, field := range fields {
		var updated bool
		builder, updated = setUpdateField(builder, field)
		if updated {
			updates++
		}
	}
	if in.ClearCustomCategory {
		builder = builder.Set("custom_category", nil)
		updates++
	}

	return builder, updates
}

func setUpdateField(builder squirrel.UpdateBuilder, field updateField) (squirrel.UpdateBuilder, bool) {
	switch value := field.value.(type) {
	case *string:
		if value != nil {
			return builder.Set(field.column, *value), true
		}
	case *json.RawMessage:
		if value != nil {
			return builder.Set(field.column, *value), true
		}
	case *[]string:
		if value != nil {
			return builder.Set(field.column, *value), true
		}
	}

	return builder, false
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
