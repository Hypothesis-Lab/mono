DROP TABLE temporal_stop_outbox;

ALTER TABLE hypothesis
    DROP CONSTRAINT hypothesis_status_check,
    ADD CONSTRAINT hypothesis_status_check CHECK (status IN ('idle', 'launching', 'in_progress'));

ALTER TABLE hypothesis
    DROP COLUMN active_stop_id;

DROP TABLE hypothesis_stop;
