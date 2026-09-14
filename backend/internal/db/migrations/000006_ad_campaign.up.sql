CREATE TABLE ad_campaign (
    id            TEXT PRIMARY KEY,
    hypothesis_id TEXT NOT NULL REFERENCES hypothesis(id) ON DELETE CASCADE,
    provider      TEXT NOT NULL CHECK (provider IN ('yandex')),
    external_id   BIGINT NOT NULL,
    created_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (hypothesis_id, provider)
);
