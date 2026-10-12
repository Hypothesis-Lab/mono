package hypothesislaunch

import (
	"context"
	"errors"
	"fmt"

	"app/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	ResourceCampaign        = "campaign"
	ResourceRetargetingList = "retargeting_list"
	ResourceAdGroup         = "ad_group"
	ResourceAudienceTarget  = "audience_target"
)

var (
	ErrNotFound          = errors.New("hypothesis launch not found")
	ErrHypothesisGone    = errors.New("hypothesis not found")
	ErrAlreadyActive     = errors.New("hypothesis launch already active")
	ErrNoProduct         = errors.New("hypothesis has no product")
	ErrNoAudiences       = errors.New("product has no generated audiences")
	ErrResourceNotFound  = errors.New("hypothesis launch resource not found")
	ErrOutboxEmpty       = errors.New("temporal outbox is empty")
	ErrStopNotActive     = errors.New("hypothesis is not in progress")
	ErrStopActive        = errors.New("hypothesis stop already active")
	ErrStopManual        = errors.New("hypothesis stop requires manual intervention")
	ErrNoSucceededLaunch = errors.New("hypothesis has no successful launch to stop")
	ErrStopOutboxEmpty   = errors.New("temporal stop outbox is empty")
)

type Launch struct {
	ID           string
	HypothesisID string
	UserID       string
	ProductID    string
	WorkflowID   string
	Status       string
}

type Resource struct {
	ID         string
	LaunchID   string
	Kind       string
	LogicalKey string
	Name       *string
	Gender     *string
	Age        *int
	StartDate  *string
	ExternalID *int64
	Status     string
}

type OutboxItem struct {
	ID           string
	LaunchID     string
	HypothesisID string
	UserID       string
	WorkflowID   string
}

type Stop struct {
	ID         string
	WorkflowID string
	Status     string
}

type StopOutboxItem struct {
	ID             string
	StopID         string
	HypothesisID   string
	UserID         string
	SourceLaunchID string
	WorkflowID     string
}

type WorkflowResource struct {
	ID         string  `json:"resource_id"`
	Kind       string  `json:"kind"`
	LogicalKey string  `json:"logical_key"`
	Name       *string `json:"name,omitempty"`
	Gender     *string `json:"gender,omitempty"`
	Age        *int    `json:"age,omitempty"`
	StartDate  *string `json:"start_date,omitempty"`
}

type Repository struct {
	db *db.DB
}

func NewRepository(db *db.DB) *Repository {
	return &Repository{db: db}
}

//nolint:cyclop // The transaction deliberately contains the full launch snapshot.
func (r *Repository) Create(ctx context.Context, userID, hypothesisID, startDate string) (*Launch, error) {
	tx, err := r.db.Pool().Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create hypothesis launch transaction: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	var currentStatus string
	if err := tx.QueryRow(ctx, "SELECT status FROM hypothesis WHERE id = $1 AND user_id = $2", hypothesisID, userID).Scan(&currentStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrHypothesisGone
		}

		return nil, fmt.Errorf("get hypothesis before launch: %w", err)
	}

	if currentStatus != "idle" {
		return nil, ErrAlreadyActive
	}

	var productID string
	if err := tx.QueryRow(ctx, "SELECT id FROM product WHERE hypothesis_id = $1", hypothesisID).Scan(&productID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoProduct
		}

		return nil, fmt.Errorf("get product before launch: %w", err)
	}

	type audience struct {
		id     string
		gender string
		age    int
	}
	rows, err := tx.Query(ctx, `
		SELECT id, persona->>'gender', (persona->>'age')::integer
		FROM audience
		WHERE product_id = $1
		ORDER BY rank, id
	`, productID)
	if err != nil {
		return nil, fmt.Errorf("list audiences before launch: %w", err)
	}
	defer rows.Close()

	var audiences []audience
	for rows.Next() {
		var item audience
		if err := rows.Scan(&item.id, &item.gender, &item.age); err != nil {
			return nil, fmt.Errorf("scan audience before launch: %w", err)
		}

		if item.gender == "" || item.age == 0 {
			return nil, fmt.Errorf("audience %s has incomplete demographic data", item.id)
		}

		audiences = append(audiences, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audiences before launch: %w", err)
	}

	if len(audiences) == 0 {
		return nil, ErrNoAudiences
	}

	launchID := uuid.NewString()
	launch := &Launch{
		ID:           launchID,
		HypothesisID: hypothesisID,
		UserID:       userID,
		ProductID:    productID,
		WorkflowID:   "hypothesis-launch-" + launchID,
		Status:       "queued",
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO hypothesis_launch (id, hypothesis_id, user_id, product_id, workflow_id, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, launch.ID, launch.HypothesisID, launch.UserID, launch.ProductID, launch.WorkflowID, launch.Status); err != nil {
		return nil, fmt.Errorf("insert hypothesis launch: %w", err)
	}

	command, err := tx.Exec(ctx, `
		UPDATE hypothesis
		SET status = 'launching', active_launch_id = $1
		WHERE id = $2 AND user_id = $3 AND status = 'idle'
	`, launch.ID, hypothesisID, userID)
	if err != nil {
		return nil, fmt.Errorf("claim hypothesis launch: %w", err)
	}

	if command.RowsAffected() != 1 {
		return nil, ErrAlreadyActive
	}

	campaignName := "hyp-" + hypothesisID + "-launch-" + launchID
	if _, err := tx.Exec(ctx, `
		INSERT INTO hypothesis_launch_resource (id, launch_id, kind, logical_key, position, name, start_date, status)
		VALUES ($1, $2, $3, 'campaign', 0, $4, $5, 'pending')
	`, uuid.NewString(), launch.ID, ResourceCampaign, campaignName, startDate); err != nil {
		return nil, fmt.Errorf("insert campaign launch resource: %w", err)
	}

	for index, audience := range audiences {
		position := 1 + index*3
		name := campaignName + "-aud-" + audience.id
		if _, err := tx.Exec(ctx, `
			INSERT INTO hypothesis_launch_resource (id, launch_id, kind, logical_key, position, name, gender, age, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending')
		`, uuid.NewString(), launch.ID, ResourceRetargetingList, audience.id, position, name, audience.gender, audience.age); err != nil {
			return nil, fmt.Errorf("insert retargeting list launch resource: %w", err)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO hypothesis_launch_resource (id, launch_id, kind, logical_key, position, name, status)
			VALUES ($1, $2, $3, $4, $5, $6, 'pending')
		`, uuid.NewString(), launch.ID, ResourceAdGroup, audience.id, position+1, name); err != nil {
			return nil, fmt.Errorf("insert ad group launch resource: %w", err)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO hypothesis_launch_resource (id, launch_id, kind, logical_key, position, status)
			VALUES ($1, $2, $3, $4, $5, 'pending')
		`, uuid.NewString(), launch.ID, ResourceAudienceTarget, audience.id, position+2); err != nil {
			return nil, fmt.Errorf("insert audience target launch resource: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, "INSERT INTO temporal_outbox (id, launch_id, status) VALUES ($1, $2, 'pending')", uuid.NewString(), launch.ID); err != nil {
		return nil, fmt.Errorf("insert temporal launch outbox item: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create hypothesis launch: %w", err)
	}

	return launch, nil
}

func (r *Repository) GetLatest(ctx context.Context, userID, hypothesisID string) (*Launch, error) {
	launch := &Launch{}
	err := r.db.Pool().QueryRow(ctx, `
		SELECT id, hypothesis_id, user_id, product_id, workflow_id, status
		FROM hypothesis_launch
		WHERE hypothesis_id = $1 AND user_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, hypothesisID, userID).Scan(&launch.ID, &launch.HypothesisID, &launch.UserID, &launch.ProductID, &launch.WorkflowID, &launch.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get latest hypothesis launch: %w", err)
	}

	return launch, nil
}

func (r *Repository) CreateStop(ctx context.Context, userID, hypothesisID string) (*Stop, error) {
	tx, err := r.db.Pool().Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create hypothesis stop transaction: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	var currentStatus string
	if err := tx.QueryRow(ctx, "SELECT status FROM hypothesis WHERE id = $1 AND user_id = $2 FOR UPDATE", hypothesisID, userID).Scan(&currentStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrHypothesisGone
		}

		return nil, fmt.Errorf("get hypothesis before stop: %w", err)
	}

	switch currentStatus {
	case "in_progress":
	case "stopping":
		return nil, ErrStopActive
	case "requires_manual_intervention":
		return nil, ErrStopManual
	default:
		return nil, ErrStopNotActive
	}

	var sourceLaunchID string
	if err := tx.QueryRow(ctx, `
		SELECT id
		FROM hypothesis_launch
		WHERE hypothesis_id = $1 AND user_id = $2 AND status = 'succeeded'
		ORDER BY completed_at DESC, created_at DESC
		LIMIT 1
	`, hypothesisID, userID).Scan(&sourceLaunchID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoSucceededLaunch
		}

		return nil, fmt.Errorf("get successful hypothesis launch before stop: %w", err)
	}

	stop := &Stop{ID: uuid.NewString(), Status: "queued"}
	stop.WorkflowID = "hypothesis-stop-" + stop.ID
	if _, err := tx.Exec(ctx, `
		INSERT INTO hypothesis_stop (id, hypothesis_id, user_id, source_launch_id, workflow_id, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, stop.ID, hypothesisID, userID, sourceLaunchID, stop.WorkflowID, stop.Status); err != nil {
		return nil, fmt.Errorf("insert hypothesis stop: %w", err)
	}

	command, err := tx.Exec(ctx, `
		UPDATE hypothesis
		SET status = 'stopping', active_stop_id = $1
		WHERE id = $2 AND user_id = $3 AND status = 'in_progress'
	`, stop.ID, hypothesisID, userID)
	if err != nil {
		return nil, fmt.Errorf("claim hypothesis stop: %w", err)
	}

	if command.RowsAffected() != 1 {
		return nil, ErrStopNotActive
	}

	if _, err := tx.Exec(ctx, "INSERT INTO temporal_stop_outbox (id, stop_id, status) VALUES ($1, $2, 'pending')", uuid.NewString(), stop.ID); err != nil {
		return nil, fmt.Errorf("insert temporal stop outbox item: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create hypothesis stop: %w", err)
	}

	return stop, nil
}

func (r *Repository) ClaimOutbox(ctx context.Context) (*OutboxItem, error) {
	tx, err := r.db.Pool().Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin claim temporal outbox transaction: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	item := &OutboxItem{}
	err = tx.QueryRow(ctx, `
		WITH candidate AS (
			SELECT id
			FROM temporal_outbox
			WHERE (status = 'pending' AND available_at <= NOW())
				OR (status = 'starting' AND updated_at <= NOW() - INTERVAL '1 minute')
			ORDER BY available_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE temporal_outbox AS outbox
		SET status = 'starting', attempts = attempts + 1, updated_at = NOW()
		FROM candidate, hypothesis_launch AS launch
		WHERE outbox.id = candidate.id AND launch.id = outbox.launch_id
		RETURNING outbox.id, outbox.launch_id, launch.hypothesis_id, launch.user_id, launch.workflow_id
	`).Scan(&item.ID, &item.LaunchID, &item.HypothesisID, &item.UserID, &item.WorkflowID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOutboxEmpty
	}

	if err != nil {
		return nil, fmt.Errorf("claim temporal outbox item: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit claim temporal outbox item: %w", err)
	}

	return item, nil
}

func (r *Repository) ClaimStopOutbox(ctx context.Context) (*StopOutboxItem, error) {
	tx, err := r.db.Pool().Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin claim temporal stop outbox transaction: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	item := &StopOutboxItem{}
	err = tx.QueryRow(ctx, `
		WITH candidate AS (
			SELECT id
			FROM temporal_stop_outbox
			WHERE (status = 'pending' AND available_at <= NOW())
				OR (status = 'starting' AND updated_at <= NOW() - INTERVAL '1 minute')
			ORDER BY available_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE temporal_stop_outbox AS outbox
		SET status = 'starting', attempts = attempts + 1, updated_at = NOW()
		FROM candidate, hypothesis_stop AS stop
		WHERE outbox.id = candidate.id AND stop.id = outbox.stop_id
		RETURNING outbox.id, outbox.stop_id, stop.hypothesis_id, stop.user_id, stop.source_launch_id, stop.workflow_id
	`).Scan(&item.ID, &item.StopID, &item.HypothesisID, &item.UserID, &item.SourceLaunchID, &item.WorkflowID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrStopOutboxEmpty
	}

	if err != nil {
		return nil, fmt.Errorf("claim temporal stop outbox item: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit claim temporal stop outbox item: %w", err)
	}

	return item, nil
}

func (r *Repository) UserIDForLaunch(ctx context.Context, launchID string) (string, error) {
	var userID string
	if err := r.db.Pool().QueryRow(ctx, "SELECT user_id FROM hypothesis_launch WHERE id = $1", launchID).Scan(&userID); errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	} else if err != nil {
		return "", fmt.Errorf("get hypothesis launch user: %w", err)
	}

	return userID, nil
}

func (r *Repository) MarkOutboxDispatched(ctx context.Context, outboxID string) error {
	if _, err := r.db.Pool().Exec(ctx, "UPDATE temporal_outbox SET status = 'dispatched', updated_at = NOW(), last_error = NULL WHERE id = $1", outboxID); err != nil {
		return fmt.Errorf("mark temporal outbox dispatched: %w", err)
	}

	return nil
}

func (r *Repository) ReleaseOutbox(ctx context.Context, outboxID string, cause error) error {
	if _, err := r.db.Pool().Exec(ctx, `
		UPDATE temporal_outbox
		SET status = 'pending', available_at = NOW() + INTERVAL '5 seconds', updated_at = NOW(), last_error = $2
		WHERE id = $1
	`, outboxID, cause.Error()); err != nil {
		return fmt.Errorf("release temporal outbox item: %w", err)
	}

	return nil
}

func (r *Repository) MarkStopOutboxDispatched(ctx context.Context, outboxID string) error {
	if _, err := r.db.Pool().Exec(ctx, "UPDATE temporal_stop_outbox SET status = 'dispatched', updated_at = NOW(), last_error = NULL WHERE id = $1", outboxID); err != nil {
		return fmt.Errorf("mark temporal stop outbox dispatched: %w", err)
	}

	return nil
}

func (r *Repository) ReleaseStopOutbox(ctx context.Context, outboxID string, cause error) error {
	if _, err := r.db.Pool().Exec(ctx, `
		UPDATE temporal_stop_outbox
		SET status = 'pending', available_at = NOW() + INTERVAL '5 seconds', updated_at = NOW(), last_error = $2
		WHERE id = $1
	`, outboxID, cause.Error()); err != nil {
		return fmt.Errorf("release temporal stop outbox item: %w", err)
	}

	return nil
}

func (r *Repository) WorkflowResources(ctx context.Context, launchID string) ([]WorkflowResource, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, kind, logical_key, name, gender, age, start_date
		FROM hypothesis_launch_resource
		WHERE launch_id = $1
		ORDER BY position
	`, launchID)
	if err != nil {
		return nil, fmt.Errorf("list hypothesis workflow resources: %w", err)
	}
	defer rows.Close()

	var resources []WorkflowResource
	for rows.Next() {
		var resource WorkflowResource
		if err := rows.Scan(&resource.ID, &resource.Kind, &resource.LogicalKey, &resource.Name, &resource.Gender, &resource.Age, &resource.StartDate); err != nil {
			return nil, fmt.Errorf("scan hypothesis launch workflow resource: %w", err)
		}

		resources = append(resources, resource)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate hypothesis launch workflow resources: %w", err)
	}

	return resources, nil
}

func (r *Repository) StopWorkflowResources(ctx context.Context, stopID string) ([]WorkflowResource, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT resource.id, resource.kind, resource.logical_key, resource.name, resource.gender, resource.age, resource.start_date
		FROM hypothesis_launch_resource AS resource
		JOIN hypothesis_stop AS stop ON stop.source_launch_id = resource.launch_id
		WHERE stop.id = $1
		ORDER BY resource.position DESC
	`, stopID)
	if err != nil {
		return nil, fmt.Errorf("list hypothesis stop workflow resources: %w", err)
	}
	defer rows.Close()

	var resources []WorkflowResource
	for rows.Next() {
		var resource WorkflowResource
		if err := rows.Scan(&resource.ID, &resource.Kind, &resource.LogicalKey, &resource.Name, &resource.Gender, &resource.Age, &resource.StartDate); err != nil {
			return nil, fmt.Errorf("scan hypothesis stop workflow resource: %w", err)
		}

		resources = append(resources, resource)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate hypothesis stop workflow resources: %w", err)
	}

	return resources, nil
}

func (r *Repository) GetResource(ctx context.Context, resourceID string) (*Resource, error) {
	resource := &Resource{}
	err := r.db.Pool().QueryRow(ctx, `
		SELECT id, launch_id, kind, logical_key, name, gender, age, start_date, external_id, status
		FROM hypothesis_launch_resource
		WHERE id = $1
	`, resourceID).Scan(
		&resource.ID, &resource.LaunchID, &resource.Kind, &resource.LogicalKey, &resource.Name,
		&resource.Gender, &resource.Age, &resource.StartDate, &resource.ExternalID, &resource.Status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrResourceNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get hypothesis launch resource: %w", err)
	}

	return resource, nil
}

func (r *Repository) DependencyExternalID(ctx context.Context, launchID, kind, logicalKey string) (int64, error) {
	var externalID *int64
	err := r.db.Pool().QueryRow(ctx, `
		SELECT external_id
		FROM hypothesis_launch_resource
		WHERE launch_id = $1 AND kind = $2 AND logical_key = $3
	`, launchID, kind, logicalKey).Scan(&externalID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrResourceNotFound
	}

	if err != nil {
		return 0, fmt.Errorf("get hypothesis launch dependency: %w", err)
	}

	if externalID == nil {
		return 0, fmt.Errorf("hypothesis launch dependency %s/%s has no external id", kind, logicalKey)
	}

	return *externalID, nil
}

func (r *Repository) RecordResourceCreated(ctx context.Context, resourceID string, externalID int64) error {
	command, err := r.db.Pool().Exec(ctx, `
		UPDATE hypothesis_launch_resource
		SET external_id = $2, status = 'created'
		WHERE id = $1 AND (external_id IS NULL OR external_id = $2)
	`, resourceID, externalID)
	if err != nil {
		return fmt.Errorf("record hypothesis launch resource: %w", err)
	}

	if command.RowsAffected() != 1 {
		return fmt.Errorf("record hypothesis launch resource %s conflicted", resourceID)
	}

	return nil
}

func (r *Repository) MarkResourceCreating(ctx context.Context, resourceID string) error {
	command, err := r.db.Pool().Exec(ctx, "UPDATE hypothesis_launch_resource SET status = 'creating' WHERE id = $1 AND status = 'pending'", resourceID)
	if err != nil {
		return fmt.Errorf("mark hypothesis launch resource creating: %w", err)
	}

	if command.RowsAffected() != 1 {
		return fmt.Errorf("mark hypothesis launch resource %s creating conflicted", resourceID)
	}

	return nil
}

func (r *Repository) MarkResourceDeleted(ctx context.Context, resourceID string) error {
	if _, err := r.db.Pool().Exec(ctx, "UPDATE hypothesis_launch_resource SET status = 'deleted' WHERE id = $1", resourceID); err != nil {
		return fmt.Errorf("mark hypothesis launch resource deleted: %w", err)
	}

	return nil
}

func (r *Repository) MarkRunning(ctx context.Context, launchID string) error {
	if _, err := r.db.Pool().Exec(ctx, "UPDATE hypothesis_launch SET status = 'running' WHERE id = $1 AND status IN ('queued', 'reconciliation_required')", launchID); err != nil {
		return fmt.Errorf("mark hypothesis launch running: %w", err)
	}

	return nil
}

func (r *Repository) MarkStopRunning(ctx context.Context, stopID string) error {
	if _, err := r.db.Pool().Exec(ctx, "UPDATE hypothesis_stop SET status = 'running' WHERE id = $1 AND status = 'queued'", stopID); err != nil {
		return fmt.Errorf("mark hypothesis stop running: %w", err)
	}

	return nil
}

func (r *Repository) Complete(ctx context.Context, launchID string) error {
	tx, err := r.db.Pool().Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin complete hypothesis launch transaction: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	var productID string
	var campaignID int64
	if err := tx.QueryRow(ctx, `
		SELECT launch.product_id, resource.external_id
		FROM hypothesis_launch AS launch
		JOIN hypothesis_launch_resource AS resource ON resource.launch_id = launch.id
		WHERE launch.id = $1 AND resource.kind = 'campaign' AND resource.logical_key = 'campaign' AND resource.status = 'created'
	`, launchID).Scan(&productID, &campaignID); err != nil {
		return fmt.Errorf("get completed launch campaign: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO ad_campaign (id, product_id, provider, external_id)
		VALUES ($1, $2, 'yandex', $3)
	`, uuid.NewString(), productID, campaignID); err != nil {
		return fmt.Errorf("store completed launch campaign: %w", err)
	}

	command, err := tx.Exec(ctx, `
		UPDATE hypothesis
		SET status = 'in_progress', active_launch_id = NULL
		WHERE active_launch_id = $1 AND status = 'launching'
	`, launchID)
	if err != nil {
		return fmt.Errorf("mark hypothesis in progress after launch: %w", err)
	}

	if command.RowsAffected() != 1 {
		return fmt.Errorf("complete hypothesis launch %s lost its hypothesis claim", launchID)
	}

	if _, err := tx.Exec(ctx, "UPDATE hypothesis_launch SET status = 'succeeded', completed_at = NOW(), last_error = NULL WHERE id = $1", launchID); err != nil {
		return fmt.Errorf("mark hypothesis launch succeeded: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit complete hypothesis launch: %w", err)
	}

	return nil
}

func (r *Repository) CompleteStop(ctx context.Context, stopID string) error {
	tx, err := r.db.Pool().Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin complete hypothesis stop transaction: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	var productID, stopStatus string
	if err := tx.QueryRow(ctx, `
		SELECT launch.product_id, stop.status
		FROM hypothesis_stop AS stop
		JOIN hypothesis_launch AS launch ON launch.id = stop.source_launch_id
		WHERE stop.id = $1
	`, stopID).Scan(&productID, &stopStatus); err != nil {
		return fmt.Errorf("get completed stop product: %w", err)
	}

	if stopStatus == "succeeded" {
		return nil
	}

	if _, err := tx.Exec(ctx, "DELETE FROM ad_campaign WHERE product_id = $1 AND provider = 'yandex'", productID); err != nil {
		return fmt.Errorf("delete local campaign after stop: %w", err)
	}

	if _, err := tx.Exec(ctx, "UPDATE hypothesis_stop SET status = 'succeeded', completed_at = NOW(), last_error = NULL WHERE id = $1", stopID); err != nil {
		return fmt.Errorf("mark hypothesis stop succeeded: %w", err)
	}

	command, err := tx.Exec(ctx, `
		UPDATE hypothesis
		SET status = 'idle', active_stop_id = NULL
		WHERE active_stop_id = $1 AND status = 'stopping'
	`, stopID)
	if err != nil {
		return fmt.Errorf("mark hypothesis idle after stop: %w", err)
	}

	if command.RowsAffected() != 1 {
		return fmt.Errorf("complete hypothesis stop %s lost its hypothesis claim", stopID)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit complete hypothesis stop: %w", err)
	}

	return nil
}

func (r *Repository) Fail(ctx context.Context, launchID, reason string) error {
	tx, err := r.db.Pool().Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin fail hypothesis launch transaction: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "UPDATE hypothesis_launch SET status = 'failed', last_error = $2, completed_at = NOW() WHERE id = $1", launchID, reason); err != nil {
		return fmt.Errorf("mark hypothesis launch failed: %w", err)
	}

	if _, err := tx.Exec(ctx, "UPDATE hypothesis SET status = 'idle', active_launch_id = NULL WHERE active_launch_id = $1 AND status = 'launching'", launchID); err != nil {
		return fmt.Errorf("reset hypothesis after failed launch: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit fail hypothesis launch: %w", err)
	}

	return nil
}

func (r *Repository) RequireReconciliation(ctx context.Context, launchID, reason string, retry bool) error {
	tx, err := r.db.Pool().Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin require hypothesis launch reconciliation transaction: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "UPDATE hypothesis_launch SET status = 'reconciliation_required', last_error = $2 WHERE id = $1", launchID, reason); err != nil {
		return fmt.Errorf("mark hypothesis launch reconciliation required: %w", err)
	}

	if retry {
		if _, err := tx.Exec(ctx, `
			UPDATE temporal_outbox
			SET status = 'pending', available_at = NOW() + INTERVAL '10 seconds', updated_at = NOW(), last_error = $2
			WHERE launch_id = $1
		`, launchID, reason); err != nil {
			return fmt.Errorf("requeue hypothesis launch reconciliation: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit require hypothesis launch reconciliation: %w", err)
	}

	return nil
}

func (r *Repository) RequireStopManualIntervention(ctx context.Context, stopID, reason string) error {
	tx, err := r.db.Pool().Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin require hypothesis stop manual intervention transaction: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	var stopStatus string
	if err := tx.QueryRow(ctx, "SELECT status FROM hypothesis_stop WHERE id = $1", stopID).Scan(&stopStatus); err != nil {
		return fmt.Errorf("get hypothesis stop before requiring manual intervention: %w", err)
	}

	if stopStatus == "requires_manual_intervention" {
		return nil
	}

	if _, err := tx.Exec(ctx, "UPDATE hypothesis_stop SET status = 'requires_manual_intervention', last_error = $2, completed_at = NOW() WHERE id = $1", stopID, reason); err != nil {
		return fmt.Errorf("mark hypothesis stop manual intervention required: %w", err)
	}

	command, err := tx.Exec(ctx, `
		UPDATE hypothesis
		SET status = 'requires_manual_intervention', active_stop_id = NULL
		WHERE active_stop_id = $1 AND status = 'stopping'
	`, stopID)
	if err != nil {
		return fmt.Errorf("mark hypothesis manual intervention required after stop: %w", err)
	}

	if command.RowsAffected() != 1 {
		return fmt.Errorf("require manual intervention for hypothesis stop %s lost its hypothesis claim", stopID)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit require hypothesis stop manual intervention: %w", err)
	}

	return nil
}
