ALTER TABLE hypothesis_launch
    DROP CONSTRAINT hypothesis_launch_status_check,
    ADD CONSTRAINT hypothesis_launch_status_check CHECK (
        status IN ('queued', 'running', 'rolling_back', 'succeeded', 'failed',
                   'reconciliation_required', 'failed_with_residuals')
    );

CREATE FUNCTION preserve_stopped_launch_history() RETURNS trigger AS $$
BEGIN
    IF OLD.status IN ('queued', 'running', 'rolling_back', 'reconciliation_required', 'failed_with_residuals') THEN
        RAISE EXCEPTION 'cannot delete active or unresolved preparation history'
            USING ERRCODE = '23503';
    END IF;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER preserve_stopped_launch_history
    BEFORE DELETE ON hypothesis_launch
    FOR EACH ROW EXECUTE FUNCTION preserve_stopped_launch_history();
