package yandexdirectconnection

import (
	"context"
	"errors"
	"fmt"
	"time"

	"app/internal/db"

	"github.com/jackc/pgx/v5"
)

var ErrNotFound = errors.New("yandex direct connection not found")

type Connection struct {
	UserID                 string
	AccessTokenCiphertext  []byte
	RefreshTokenCiphertext []byte
	TokenExpiresAt         *time.Time
	Status                 string
	ConnectedAt            *time.Time
	DisconnectedAt         *time.Time
}

type Repository struct {
	db *db.DB
}

func NewRepository(database *db.DB) *Repository {
	return &Repository{db: database}
}

func (r *Repository) Get(ctx context.Context, userID string) (*Connection, error) {
	connection := &Connection{}
	err := r.db.Pool().QueryRow(ctx, `
		SELECT user_id, access_token_ciphertext, refresh_token_ciphertext,
		       token_expires_at, status, connected_at, disconnected_at
		FROM yandex_direct_connection
		WHERE user_id = $1`, userID).Scan(
		&connection.UserID,
		&connection.AccessTokenCiphertext,
		&connection.RefreshTokenCiphertext,
		&connection.TokenExpiresAt,
		&connection.Status,
		&connection.ConnectedAt,
		&connection.DisconnectedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get yandex direct connection: %w", err)
	}

	return connection, nil
}

func (r *Repository) SaveConnected(ctx context.Context, userID string, accessToken, refreshToken []byte, expiresAt time.Time) error {
	_, err := r.db.Pool().Exec(ctx, `
		INSERT INTO yandex_direct_connection (
			user_id, access_token_ciphertext, refresh_token_ciphertext,
			token_expires_at, status, connected_at, disconnected_at
		) VALUES ($1, $2, $3, $4, 'connected', NOW(), NULL)
		ON CONFLICT (user_id) DO UPDATE SET
			access_token_ciphertext = EXCLUDED.access_token_ciphertext,
			refresh_token_ciphertext = EXCLUDED.refresh_token_ciphertext,
			token_expires_at = EXCLUDED.token_expires_at,
			status = 'connected',
			connected_at = NOW(),
			disconnected_at = NULL,
			updated_at = NOW()`, userID, accessToken, refreshToken, expiresAt)
	if err != nil {
		return fmt.Errorf("save connected yandex direct connection: %w", err)
	}

	return nil
}

func (r *Repository) Disconnect(ctx context.Context, userID string) error {
	_, err := r.db.Pool().Exec(ctx, `
		INSERT INTO yandex_direct_connection (
			user_id, access_token_ciphertext, refresh_token_ciphertext,
			token_expires_at, status, connected_at, disconnected_at
		) VALUES ($1, NULL, NULL, NULL, 'disconnected', NULL, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			access_token_ciphertext = NULL,
			refresh_token_ciphertext = NULL,
			token_expires_at = NULL,
			status = 'disconnected',
			disconnected_at = COALESCE(yandex_direct_connection.disconnected_at, NOW()),
			updated_at = NOW()`, userID)
	if err != nil {
		return fmt.Errorf("disconnect yandex direct connection: %w", err)
	}

	return nil
}

func (r *Repository) MarkRevoked(ctx context.Context, userID string) error {
	result, err := r.db.Pool().Exec(ctx, `
		UPDATE yandex_direct_connection
		SET access_token_ciphertext = NULL,
		    refresh_token_ciphertext = NULL,
		    token_expires_at = NULL,
		    status = 'revoked',
		    updated_at = NOW()
		WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("mark yandex direct connection revoked: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}
