package productanalysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"app/internal/db"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrNotFound = errors.New("product analysis run not found")
var ErrAlreadyActive = errors.New("product analysis run is already active")
var squirrelPSQL = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

type Run struct {
	ID               string
	ProductID        string
	InputSnapshot    json.RawMessage
	SegmentSetID     string
	Status           string
	CurrentStage     string
	ProductJobID     *string
	AudienceJobID    *string
	HypothesisJobID  *string
	ProductResult    json.RawMessage
	AudienceResult   json.RawMessage
	HypothesisResult json.RawMessage
	AudienceReview   *StageReview
	HypothesisReview *StageReview
	CreatedAt        time.Time
	FinishedAt       *time.Time
}

type Status struct {
	ID                        string
	Status                    string
	CurrentStage              string
	ProductResultAvailable    bool
	AudienceResultAvailable   bool
	HypothesisResultAvailable bool
	AudienceReviewAvailable   bool
	HypothesisReviewAvailable bool
}

type StageReview struct {
	Stage                string
	SelectedCandidateIDs []string
	CustomCandidates     json.RawMessage
	Ratings              json.RawMessage
	UpdatedAt            time.Time
}

type Repository struct {
	db *db.DB
}

type SegmentSet struct {
	ID       string
	Metadata json.RawMessage
	Segments []json.RawMessage
}

func NewRepository(db *db.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, id, productID, segmentSetID string, snapshot json.RawMessage) error {
	_, err := r.db.Pool().Exec(ctx, `
		INSERT INTO product_analysis_run (id, product_id, input_snapshot, segment_set_id, status, current_stage)
		VALUES ($1, $2, $3, $4, 'queued', 'product')`, id, productID, snapshot, segmentSetID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrAlreadyActive
		}

		return fmt.Errorf("create product analysis run %s: %w", id, err)
	}

	return nil
}

func (r *Repository) LatestID(ctx context.Context, productID string) (string, error) {
	var id string
	err := r.db.Pool().QueryRow(ctx, `SELECT id FROM product_analysis_run
		WHERE product_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1`, productID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}

	if err != nil {
		return "", fmt.Errorf("get latest product analysis run for product %s: %w", productID, err)
	}

	return id, nil
}

func (r *Repository) LatestStatus(ctx context.Context, productID string) (*Status, error) {
	status := &Status{}
	err := r.db.Pool().QueryRow(ctx, `
		SELECT r.id, r.status, r.current_stage,
			r.product_result IS NOT NULL,
			r.audience_result IS NOT NULL,
			r.hypothesis_result IS NOT NULL,
			EXISTS (SELECT 1 FROM product_analysis_review review WHERE review.run_id = r.id AND review.stage = 'audience'),
			EXISTS (SELECT 1 FROM product_analysis_review review WHERE review.run_id = r.id AND review.stage = 'hypotheses')
		FROM product_analysis_run r
		WHERE r.product_id = $1
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT 1`, productID).Scan(
		&status.ID,
		&status.Status,
		&status.CurrentStage,
		&status.ProductResultAvailable,
		&status.AudienceResultAvailable,
		&status.HypothesisResultAvailable,
		&status.AudienceReviewAvailable,
		&status.HypothesisReviewAvailable,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get latest product analysis status for product %s: %w", productID, err)
	}

	return status, nil
}

func (r *Repository) QueuedIDs(ctx context.Context) ([]string, error) {
	rows, err := r.db.Pool().Query(ctx, `SELECT id FROM product_analysis_run WHERE status = 'queued' ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("query queued product analysis runs: %w", err)
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan queued product analysis run: %w", err)
		}

		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate queued product analysis runs: %w", err)
	}

	return ids, nil
}

func (r *Repository) Get(ctx context.Context, id string) (*Run, error) {
	run := &Run{}
	err := r.db.Pool().QueryRow(ctx, `
		SELECT id, product_id, input_snapshot, segment_set_id::text, status, current_stage,
		       product_job_id, audience_job_id, hypothesis_job_id, product_result, audience_result,
		       hypothesis_result, created_at, finished_at
		FROM product_analysis_run WHERE id = $1`, id).Scan(
		&run.ID, &run.ProductID, &run.InputSnapshot, &run.SegmentSetID, &run.Status, &run.CurrentStage,
		&run.ProductJobID, &run.AudienceJobID, &run.HypothesisJobID, &run.ProductResult, &run.AudienceResult,
		&run.HypothesisResult, &run.CreatedAt, &run.FinishedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get product analysis run %s: %w", id, err)
	}

	if err := r.loadReviews(ctx, run); err != nil {
		return nil, fmt.Errorf("load product analysis reviews for run %s: %w", id, err)
	}

	return run, nil
}

func (r *Repository) LatestSegmentSet(ctx context.Context) (*SegmentSet, error) {
	set := &SegmentSet{}
	err := r.db.Pool().QueryRow(ctx, `
		SELECT id::text, metadata FROM generator.segment_set ORDER BY created_at DESC, id DESC LIMIT 1`).
		Scan(&set.ID, &set.Metadata)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get latest audience segment set: %w", err)
	}

	return r.loadSegments(ctx, set)
}

func (r *Repository) GetSegmentSet(ctx context.Context, id string) (*SegmentSet, error) {
	set := &SegmentSet{}
	err := r.db.Pool().QueryRow(ctx, `SELECT id::text, metadata FROM generator.segment_set WHERE id = $1`, id).
		Scan(&set.ID, &set.Metadata)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get audience segment set %s: %w", id, err)
	}

	return r.loadSegments(ctx, set)
}

func (r *Repository) SetJob(ctx context.Context, runID, stage, jobID string) error {
	var column string
	switch stage {
	case "product":
		column = "product_job_id"
	case "audience":
		column = "audience_job_id"
	case "hypotheses":
		column = "hypothesis_job_id"
	default:
		return fmt.Errorf("invalid product analysis stage %q", stage)
	}

	query, args, err := squirrelPSQL.
		Update("product_analysis_run").
		Set(column, jobID).
		Set("status", squirrel.Expr(
			"CASE WHEN status = 'queued' OR (status = 'waiting_for_user_input' AND ? = 'hypotheses') THEN 'running' ELSE status END",
			stage,
		)).
		Set("current_stage", stage).
		Where(squirrel.Eq{"id": runID}).
		Where(squirrel.Or{squirrel.Eq{column: nil}, squirrel.Eq{column: jobID}}).
		Where(squirrel.Or{
			squirrel.Eq{"status": []string{"queued", "running"}},
			squirrel.Eq{"current_stage": stage},
			squirrel.And{
				squirrel.Eq{"status": "waiting_for_user_input"},
				squirrel.Eq{"current_stage": "audience"},
				squirrel.Expr("? = 'hypotheses'", stage),
			},
		}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build set %s job query for product analysis run %s: %w", stage, runID, err)
	}

	result, err := r.db.Pool().Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("set %s job for product analysis run %s: %w", stage, runID, err)
	}

	if result.RowsAffected() != 1 {
		return fmt.Errorf("set %s job for product analysis run %s: run missing or job changed", stage, runID)
	}

	return nil
}

func (r *Repository) SaveResult(ctx context.Context, runID, stage string, result json.RawMessage, status string) error {
	var column string
	switch stage {
	case "product":
		column = "product_result"
	case "audience":
		column = "audience_result"
	case "hypotheses":
		column = "hypothesis_result"
	default:
		return fmt.Errorf("invalid result stage %q", stage)
	}

	if status != "running" && status != "waiting_for_user_input" && status != "blocked" && status != "completed" {
		return fmt.Errorf("invalid product analysis result status %q", status)
	}

	query := fmt.Sprintf(`UPDATE product_analysis_run SET %s = $2, status = $3,
		finished_at = CASE WHEN $3 IN ('blocked', 'completed') THEN COALESCE(finished_at, NOW()) ELSE NULL END
		WHERE id = $1 AND current_stage = $4 AND (status IN ('queued', 'running') OR status = $3)`, column)
	updated, err := r.db.Pool().Exec(ctx, query, runID, result, status, stage)
	if err != nil {
		return fmt.Errorf("save %s result for product analysis run %s: %w", stage, runID, err)
	}

	if updated.RowsAffected() != 1 {
		return fmt.Errorf("save %s result for product analysis run %s: %w", stage, runID, ErrNotFound)
	}

	return nil
}

func (r *Repository) SaveReview(ctx context.Context, runID, stage string, selectedCandidateIDs []string, customCandidates, ratings json.RawMessage) (*StageReview, error) {
	if stage != "audience" && stage != "hypotheses" {
		return nil, fmt.Errorf("invalid product analysis review stage %q", stage)
	}

	review := &StageReview{}
	err := r.db.Pool().QueryRow(ctx, `
		INSERT INTO product_analysis_review (run_id, stage, selected_candidate_ids, custom_candidates, ratings)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (run_id, stage) DO UPDATE SET
			selected_candidate_ids = EXCLUDED.selected_candidate_ids,
			custom_candidates = EXCLUDED.custom_candidates,
			ratings = EXCLUDED.ratings,
			updated_at = NOW()
		RETURNING stage, selected_candidate_ids, custom_candidates, ratings, updated_at`,
		runID,
		stage,
		selectedCandidateIDs,
		customCandidates,
		ratings,
	).Scan(
		&review.Stage,
		&review.SelectedCandidateIDs,
		&review.CustomCandidates,
		&review.Ratings,
		&review.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("save product analysis %s review for run %s: %w", stage, runID, err)
	}

	return review, nil
}

func (r *Repository) Fail(ctx context.Context, runID string) error {
	result, err := r.db.Pool().Exec(ctx, `UPDATE product_analysis_run SET status = 'failed', finished_at = NOW()
		WHERE id = $1 AND status IN ('queued', 'running', 'failed')`, runID)
	if err != nil {
		return fmt.Errorf("fail product analysis run %s: %w", runID, err)
	}

	if result.RowsAffected() != 1 {
		return fmt.Errorf("fail product analysis run %s: run missing or already finished", runID)
	}

	return nil
}

func (r *Repository) loadReviews(ctx context.Context, run *Run) error {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT stage, selected_candidate_ids, custom_candidates, ratings, updated_at
		FROM product_analysis_review
		WHERE run_id = $1`, run.ID)
	if err != nil {
		return fmt.Errorf("query reviews: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		review := &StageReview{}
		if err := rows.Scan(
			&review.Stage,
			&review.SelectedCandidateIDs,
			&review.CustomCandidates,
			&review.Ratings,
			&review.UpdatedAt,
		); err != nil {
			return fmt.Errorf("scan review: %w", err)
		}

		switch review.Stage {
		case "audience":
			run.AudienceReview = review
		case "hypotheses":
			run.HypothesisReview = review
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate reviews: %w", err)
	}

	return nil
}

func (r *Repository) loadSegments(ctx context.Context, set *SegmentSet) (*SegmentSet, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT candidate FROM generator.segment WHERE segment_set_id = $1 ORDER BY segment_id`, set.ID)
	if err != nil {
		return nil, fmt.Errorf("query segments for set %s: %w", set.ID, err)
	}
	defer rows.Close()
	for rows.Next() {
		var candidate json.RawMessage
		if err := rows.Scan(&candidate); err != nil {
			return nil, fmt.Errorf("scan segment for set %s: %w", set.ID, err)
		}

		set.Segments = append(set.Segments, candidate)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate segments for set %s: %w", set.ID, err)
	}

	return set, nil
}
