DROP TABLE IF EXISTS target_audience;

CREATE TABLE audience (
    id               TEXT PRIMARY KEY,
    product_id       TEXT NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    generator_job_id TEXT NOT NULL,
    persona_id       TEXT NOT NULL,
    persona          JSONB NOT NULL,
    research         JSONB NOT NULL,
    metrics          JSONB NOT NULL,
    hypotheses       JSONB NOT NULL,
    rank             INTEGER NOT NULL CHECK (rank BETWEEN 1 AND 3),
    score            DOUBLE PRECISION NOT NULL,
    created_at       TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (generator_job_id, persona_id)
);

CREATE INDEX audience_product_id_rank_idx ON audience (product_id, rank);
