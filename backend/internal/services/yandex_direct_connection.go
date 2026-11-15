package services

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"time"

	"app/internal/clients/yandex_direct"
	"app/internal/clients/yandex_oauth"
	"app/internal/config"
	"app/internal/repositories/yandexdirectconnection"
)

var ErrYandexDirectNotConnected = errors.New("yandex direct connection is not connected")

type YandexDirectConnectionStatus struct {
	Status         string
	ConnectedAt    *time.Time
	DisconnectedAt *time.Time
	TokenExpiresAt *time.Time
}

type YandexDirectConnectionService struct {
	connections  *yandexdirectconnection.Repository
	oauth        *yandexoauth.Client
	cipher       cipher.AEAD
	mockEndpoint string
}

func NewYandexDirectConnectionService(connections *yandexdirectconnection.Repository, cfg *config.Config) (*YandexDirectConnectionService, error) {
	mockEndpoint := os.Getenv("YANDEX_DIRECT_MOCK_ENDPOINT")
	if mockEndpoint != "" {
		parsed, err := url.Parse(mockEndpoint)
		if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "/json/v5" || net.ParseIP(parsed.Hostname()) == nil || !net.ParseIP(parsed.Hostname()).IsLoopback() || parsed.Port() == "" {
			return nil, errors.New("YANDEX_DIRECT_MOCK_ENDPOINT must be a loopback HTTP URL with port and path /json/v5")
		}
	}

	block, err := aes.NewCipher(cfg.YandexDirectTokenEncryptionKey[:])
	if err != nil {
		return nil, fmt.Errorf("create yandex direct token cipher: %w", err)
	}

	cipher, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create yandex direct token gcm: %w", err)
	}

	return &YandexDirectConnectionService{
		connections:  connections,
		oauth:        yandexoauth.New(cfg.YandexDirectClientID, cfg.YandexDirectClientSecret),
		cipher:       cipher,
		mockEndpoint: mockEndpoint,
	}, nil
}

func (s *YandexDirectConnectionService) AuthorizationURL() string {
	return s.oauth.AuthorizationURL()
}

func (s *YandexDirectConnectionService) Status(ctx context.Context, userID string) (*YandexDirectConnectionStatus, error) {
	connection, err := s.connections.Get(ctx, userID)
	if err != nil {
		if errors.Is(err, yandexdirectconnection.ErrNotFound) {
			return &YandexDirectConnectionStatus{Status: "disconnected"}, nil
		}

		return nil, fmt.Errorf("get yandex direct connection status: %w", err)
	}

	return &YandexDirectConnectionStatus{
		Status:         connection.Status,
		ConnectedAt:    connection.ConnectedAt,
		DisconnectedAt: connection.DisconnectedAt,
		TokenExpiresAt: connection.TokenExpiresAt,
	}, nil
}

func (s *YandexDirectConnectionService) EnsureConnected(ctx context.Context, userID string) error {
	connection, err := s.connections.Get(ctx, userID)
	if errors.Is(err, yandexdirectconnection.ErrNotFound) {
		return ErrYandexDirectNotConnected
	}

	if err != nil {
		return fmt.Errorf("check yandex direct connection: %w", err)
	}

	if connection.Status != "connected" {
		return fmt.Errorf("%w: status=%s", ErrYandexDirectNotConnected, connection.Status)
	}

	return nil
}

func (s *YandexDirectConnectionService) Connect(ctx context.Context, userID, code string) (*YandexDirectConnectionStatus, error) {
	token, err := s.oauth.ExchangeCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange yandex direct confirmation code: %w", err)
	}

	accessToken, err := s.encrypt([]byte(token.AccessToken))
	if err != nil {
		return nil, fmt.Errorf("encrypt yandex direct access token: %w", err)
	}

	refreshToken, err := s.encrypt([]byte(token.RefreshToken))
	if err != nil {
		return nil, fmt.Errorf("encrypt yandex direct refresh token: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	if err := s.connections.SaveConnected(ctx, userID, accessToken, refreshToken, expiresAt); err != nil {
		return nil, fmt.Errorf("store yandex direct connection: %w", err)
	}

	return s.Status(ctx, userID)
}

func (s *YandexDirectConnectionService) Disconnect(ctx context.Context, userID string) error {
	if err := s.connections.Disconnect(ctx, userID); err != nil {
		return fmt.Errorf("remove yandex direct connection: %w", err)
	}

	return nil
}

func (s *YandexDirectConnectionService) MarkRevoked(ctx context.Context, userID string) error {
	if err := s.connections.MarkRevoked(ctx, userID); err != nil {
		return fmt.Errorf("mark yandex direct connection revoked: %w", err)
	}

	return nil
}

func (s *YandexDirectConnectionService) NewClient(ctx context.Context, userID string) (*yandexdirect.Client, error) {
	if s.mockEndpoint != "" {
		return yandexdirect.NewLocalMock(s.mockEndpoint), nil
	}

	connection, err := s.connections.Get(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get yandex direct credentials: %w", err)
	}

	if connection.Status != "connected" {
		return nil, fmt.Errorf("yandex direct connection is not connected: status=%s", connection.Status)
	}

	accessToken, err := s.decrypt(connection.AccessTokenCiphertext)
	if err != nil {
		return nil, fmt.Errorf("decrypt yandex direct access token: %w", err)
	}

	return yandexdirect.NewWithAuthenticationFailure(string(accessToken), func(ctx context.Context) error {
		return s.MarkRevoked(ctx, userID)
	}), nil
}

func (s *YandexDirectConnectionService) encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, s.cipher.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate token encryption nonce: %w", err)
	}

	return s.cipher.Seal(nonce, nonce, plaintext, nil), nil
}

func (s *YandexDirectConnectionService) decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < s.cipher.NonceSize() {
		return nil, errors.New("encrypted token is shorter than nonce")
	}

	plaintext, err := s.cipher.Open(nil, ciphertext[:s.cipher.NonceSize()], ciphertext[s.cipher.NonceSize():], nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt encrypted token: %w", err)
	}

	return plaintext, nil
}
