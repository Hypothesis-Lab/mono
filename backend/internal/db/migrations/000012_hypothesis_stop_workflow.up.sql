CREATE TABLE hypothesis_stop (
    id               TEXT PRIMARY KEY,
    hypothesis_id    TEXT NOT NULL REFERENCES hypothesis(id) ON DELETE CASCADE,
    user_id          TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_launch_id TEXT NOT NULL REFERENCES hypothesis_launch(id) ON DELETE CASCADE,
    workflow_id      TEXT NOT NULL UNIQUE,
    status           TEXT NOT NULL CHECK (status IN ('queued', 'running', 'succeeded', 'requires_manual_intervention')),
    last_error       TEXT,
    created_at       TIMESTAMP NOT NULL DEFAULT NOW(),
    completed_at     TIMESTAMP
);

ALTER TABLE hypothesis
    ADD COLUMN active_stop_id TEXT REFERENCES hypothesis_stop(id) ON DELETE SET NULL;

ALTER TABLE hypothesis
    DROP CONSTRAINT hypothesis_status_check,
    ADD CONSTRAINT hypothesis_status_check CHECK (status IN ('idle', 'launching', 'in_progress', 'stopping', 'requires_manual_intervention'));

CREATE TABLE temporal_stop_outbox (
    id           TEXT PRIMARY KEY,
    stop_id      TEXT NOT NULL UNIQUE REFERENCES hypothesis_stop(id) ON DELETE CASCADE,
    status       TEXT NOT NULL CHECK (status IN ('pending', 'starting', 'dispatched')),
    attempts     INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMP NOT NULL DEFAULT NOW(),
    last_error   TEXT
);

CREATE INDEX temporal_stop_outbox_pending_idx ON temporal_stop_outbox (available_at) WHERE status = 'pending';
