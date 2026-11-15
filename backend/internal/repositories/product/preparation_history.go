package product

import (
	"context"
	"fmt"
)

func (r *Repository) HasUnresolvedPreparation(ctx context.Context, userID, hypothesisID string) (bool, error) {
	var exists bool
	err := r.db.Pool().QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM hypothesis_launch
			WHERE user_id = $1 AND hypothesis_id = $2
			  AND status IN ('queued', 'running', 'rolling_back', 'reconciliation_required', 'failed_with_residuals')
		)
	`, userID, hypothesisID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check unresolved preparation before deletion: %w", err)
	}

	return exists, nil
}
