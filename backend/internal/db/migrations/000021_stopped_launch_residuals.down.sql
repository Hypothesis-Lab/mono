DROP TRIGGER preserve_stopped_launch_history ON hypothesis_launch;
DROP FUNCTION preserve_stopped_launch_history();

UPDATE hypothesis_launch
SET status = 'reconciliation_required'
WHERE status = 'failed_with_residuals';

ALTER TABLE hypothesis_launch
    DROP CONSTRAINT hypothesis_launch_status_check,
    ADD CONSTRAINT hypothesis_launch_status_check CHECK (
        status IN ('queued', 'running', 'rolling_back', 'succeeded', 'failed',
                   'reconciliation_required')
    );
