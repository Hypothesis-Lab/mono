CREATE TABLE yandex_direct_connection (
    user_id                  TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    access_token_ciphertext  BYTEA,
    refresh_token_ciphertext BYTEA,
    token_expires_at         TIMESTAMP,
    status                   TEXT NOT NULL CHECK (status IN ('connected', 'disconnected', 'revoked')),
    connected_at             TIMESTAMP,
    disconnected_at          TIMESTAMP,
    updated_at               TIMESTAMP NOT NULL DEFAULT NOW(),
    CHECK (
        (status = 'connected' AND access_token_ciphertext IS NOT NULL AND refresh_token_ciphertext IS NOT NULL AND connected_at IS NOT NULL)
        OR
        (status IN ('disconnected', 'revoked') AND access_token_ciphertext IS NULL AND refresh_token_ciphertext IS NULL)
    )
);
