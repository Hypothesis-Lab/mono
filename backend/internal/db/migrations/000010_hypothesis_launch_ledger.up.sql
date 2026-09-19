CREATE TABLE hypothesis_launch (
    id            TEXT PRIMARY KEY,
    hypothesis_id TEXT NOT NULL REFERENCES hypothesis(id) ON DELETE CASCADE,
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    product_id    TEXT NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    workflow_id   TEXT NOT NULL UNIQUE,
    status        TEXT NOT NULL CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'reconciliation_required')),
    last_error    TEXT,
    created_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    completed_at  TIMESTAMP
);

ALTER TABLE hypothesis
    ADD COLUMN active_launch_id TEXT REFERENCES hypothesis_launch(id) ON DELETE SET NULL;

CREATE TABLE hypothesis_launch_resource (
    id          TEXT PRIMARY KEY,
    launch_id   TEXT NOT NULL REFERENCES hypothesis_launch(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL CHECK (kind IN ('campaign', 'retargeting_list', 'ad_group', 'audience_target')),
    logical_key TEXT NOT NULL,
    position    INTEGER NOT NULL,
    name        TEXT,
    gender      TEXT,
    age         INTEGER,
    start_date  TEXT,
    external_id BIGINT,
    status      TEXT NOT NULL CHECK (status IN ('pending', 'creating', 'created', 'deleted')),
    UNIQUE (launch_id, kind, logical_key)
);

CREATE TABLE temporal_outbox (
    id           TEXT PRIMARY KEY,
    launch_id    TEXT NOT NULL UNIQUE REFERENCES hypothesis_launch(id) ON DELETE CASCADE,
    status       TEXT NOT NULL CHECK (status IN ('pending', 'starting', 'dispatched')),
    attempts     INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMP NOT NULL DEFAULT NOW(),
    last_error   TEXT
);

CREATE INDEX temporal_outbox_pending_idx ON temporal_outbox (available_at) WHERE status = 'pending';
