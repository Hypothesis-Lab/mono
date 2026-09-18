ALTER TABLE hypothesis
    DROP CONSTRAINT hypothesis_status_check,
    ADD CONSTRAINT hypothesis_status_check CHECK (status IN ('idle', 'launching', 'in_progress'));
