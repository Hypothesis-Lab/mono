DROP TABLE temporal_outbox;
DROP TABLE hypothesis_launch_resource;

ALTER TABLE hypothesis
    DROP COLUMN active_launch_id;

DROP TABLE hypothesis_launch;
