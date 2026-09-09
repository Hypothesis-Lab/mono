DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS target_audience CASCADE;
DROP TABLE IF EXISTS file CASCADE;
DROP TABLE IF EXISTS product CASCADE;
DROP TABLE IF EXISTS hypothesis CASCADE;

CREATE TABLE users (
    id             TEXT PRIMARY KEY,
    email          TEXT NOT NULL UNIQUE,
    password_hash  TEXT NOT NULL,
    created_at     TIMESTAMP NOT NULL DEFAULT NOW()
);

DROP DOMAIN IF EXISTS keyword CASCADE;

CREATE DOMAIN keyword AS TEXT CHECK (length(VALUE) <= 64);

CREATE TABLE hypothesis (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    description VARCHAR(1024) NOT NULL,
    status      TEXT NOT NULL DEFAULT 'idle' CHECK (status IN ('idle', 'in_progress')),
    created_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX hypothesis_user_id_idx ON hypothesis (user_id);

CREATE TABLE product (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    description   TEXT,
    category      TEXT NOT NULL CHECK (category IN ('electronics', 'clothing', 'food', 'furniture', 'beauty', 'sports', 'toys', 'books', 'automotive', 'health', 'home', 'office', 'garden', 'pets', 'music', 'other')),
    keywords      keyword[] NOT NULL DEFAULT '{}',
    price         VARCHAR(128),
    link          VARCHAR(128),
    hypothesis_id TEXT NOT NULL UNIQUE REFERENCES hypothesis(id) ON DELETE CASCADE
);

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

CREATE TABLE file (
    id            TEXT PRIMARY KEY,
    hypothesis_id TEXT NOT NULL REFERENCES hypothesis(id) ON DELETE CASCADE,
    type          TEXT NOT NULL DEFAULT 'other' CHECK (type IN ('product', 'brandbook', 'reference', 'other')),
    filename      TEXT NOT NULL,
    content_type  TEXT NOT NULL,
    size          BIGINT NOT NULL,
    s3_key        TEXT NOT NULL UNIQUE,
    created_at    TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX file_hypothesis_id_idx ON file (hypothesis_id);
CREATE INDEX target_audience_product_id_idx ON target_audience (product_id);
CREATE UNIQUE INDEX file_one_brandbook_per_hypothesis_idx
    ON file (hypothesis_id) WHERE type = 'brandbook';
