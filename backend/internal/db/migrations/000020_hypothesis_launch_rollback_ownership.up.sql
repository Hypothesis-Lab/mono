ALTER TABLE hypothesis_launch_resource
    DROP CONSTRAINT hypothesis_launch_resource_status_check,
    ADD COLUMN owned_by_launch BOOLEAN,
    ADD CONSTRAINT hypothesis_launch_resource_status_check
        CHECK (status IN ('pending', 'creating', 'created', 'deleted', 'compensated', 'retained'));

ALTER TABLE hypothesis_launch
    DROP CONSTRAINT hypothesis_launch_status_check,
    ADD CONSTRAINT hypothesis_launch_status_check
        CHECK (status IN ('queued', 'running', 'rolling_back', 'succeeded', 'failed', 'reconciliation_required'));
