UPDATE hypothesis_launch_resource
SET status = CASE status
    WHEN 'compensated' THEN 'deleted'
    WHEN 'retained' THEN 'created'
    ELSE status
END
WHERE status IN ('compensated', 'retained');

UPDATE hypothesis_launch SET status = 'reconciliation_required' WHERE status = 'rolling_back';
ALTER TABLE hypothesis_launch
    DROP CONSTRAINT hypothesis_launch_status_check,
    ADD CONSTRAINT hypothesis_launch_status_check
        CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'reconciliation_required'));

ALTER TABLE hypothesis_launch_resource
    DROP CONSTRAINT hypothesis_launch_resource_status_check,
    DROP COLUMN owned_by_launch,
    ADD CONSTRAINT hypothesis_launch_resource_status_check
        CHECK (status IN ('pending', 'creating', 'created', 'deleted'));
