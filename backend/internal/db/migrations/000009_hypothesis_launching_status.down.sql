UPDATE hypothesis SET status = 'idle' WHERE status = 'launching';

ALTER TABLE hypothesis
    DROP CONSTRAINT hypothesis_status_check,
    ADD CONSTRAINT hypothesis_status_check CHECK (status IN ('idle', 'in_progress'));
