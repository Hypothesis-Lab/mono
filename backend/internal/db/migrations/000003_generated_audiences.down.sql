DROP TABLE IF EXISTS audience;

CREATE TABLE target_audience (
    id                  TEXT PRIMARY KEY,
    product_id          TEXT NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    description         TEXT NOT NULL,
    age_from            INTEGER CHECK (age_from >= 0 AND age_from <= 100),
    age_to              INTEGER CHECK (age_to >= 0 AND age_to <= 100),
    gender              TEXT CHECK (gender IN ('men', 'women', 'all')),
    interests           TEXT CHECK (length(interests) <= 1000),
    geography           VARCHAR(255),
    pain_points         TEXT,
    desires_motivation  TEXT,
    created_at          TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX target_audience_product_id_idx ON target_audience (product_id);
