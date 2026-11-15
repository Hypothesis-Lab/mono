package hypothesislaunch

import (
	"context"
	"fmt"
)

// StopWithResiduals closes automatic execution without claiming that resources are absent.
func (r *Repository) StopWithResiduals(ctx context.Context, launchID, reason string) (string, error) {
	tx, err := r.db.Pool().Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin stop launch with residuals: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	if err := tx.QueryRow(ctx, "SELECT status FROM hypothesis_launch WHERE id = $1 FOR UPDATE", launchID).Scan(&status); err != nil {
		return "", fmt.Errorf("lock launch before stopping with residuals: %w", err)
	}

	switch status {
	case "succeeded", "failed", "failed_with_residuals":
		return status, nil
	}

	if _, err := tx.Exec(ctx, `
		UPDATE hypothesis_launch
		SET status = 'failed_with_residuals', completed_at = NOW(),
		    last_error = CASE WHEN last_error IS NULL OR last_error = $2 THEN $2
		                      ELSE last_error || E'\nStopped: ' || $2 END
		WHERE id = $1
	`, launchID, reason); err != nil {
		return "", fmt.Errorf("record stopped launch with residuals: %w", err)
	}

	// Dispatched is also the consumed state: no future starter may re-enqueue this launch.
	if _, err := tx.Exec(ctx, `
		UPDATE temporal_outbox SET status = 'dispatched', updated_at = NOW(), last_error = $2
		WHERE launch_id = $1
	`, launchID, reason); err != nil {
		return "", fmt.Errorf("close stopped launch outbox: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE hypothesis SET status = 'idle', active_launch_id = NULL
		WHERE active_launch_id = $1 AND status = 'launching'
	`, launchID); err != nil {
		return "", fmt.Errorf("release stopped launch hypothesis claim: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit stopped launch with residuals: %w", err)
	}

	return "failed_with_residuals", nil
}

func (r *Repository) EnsureCreatingAllowed(ctx context.Context, launchID string) error {
	var allowed bool
	err := r.db.Pool().QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM hypothesis_launch l JOIN hypothesis h ON h.active_launch_id = l.id
			WHERE l.id = $1 AND l.status = 'running' AND h.status = 'launching'
		)
	`, launchID).Scan(&allowed)
	if err != nil {
		return fmt.Errorf("check launch creation phase: %w", err)
	}

	if !allowed {
		return fmt.Errorf("launch %s no longer permits resource creation", launchID)
	}

	return nil
}

func (r *Repository) EnsureRollbackAllowed(ctx context.Context, launchID string) error {
	var allowed bool
	err := r.db.Pool().QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM hypothesis_launch l JOIN hypothesis h ON h.active_launch_id = l.id
			WHERE l.id = $1 AND l.status = 'rolling_back' AND h.status = 'launching'
		)
	`, launchID).Scan(&allowed)
	if err != nil {
		return fmt.Errorf("check launch rollback phase: %w", err)
	}

	if !allowed {
		return fmt.Errorf("launch %s no longer permits rollback", launchID)
	}

	return nil
}
