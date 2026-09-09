package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"app/internal/config"
	"app/internal/repositories/user"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	MinPasswordLength = 6
	tokenDuration     = 24 * time.Hour
)

var (
	ErrInvalidEmail       = errors.New("invalid email")
	ErrPasswordTooShort   = errors.New("password too short")
	ErrEmailExists        = errors.New("email already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type UserAuthService struct {
	users     *user.Repository
	jwtSecret string
}

func NewUserAuthService(users *user.Repository, cfg *config.Config) *UserAuthService {
	return &UserAuthService{users: users, jwtSecret: cfg.JWTSecret}
}

func (s *UserAuthService) Register(ctx context.Context, email, password string) (string, error) {
	if email == "" || !strings.Contains(email, "@") {
		return "", ErrInvalidEmail
	}

	if len(password) < MinPasswordLength {
		return "", ErrPasswordTooShort
	}

	hash, err := hashPassword(password)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	u, err := s.users.Create(ctx, user.CreateInput{
		Email:        email,
		PasswordHash: hash,
	})
	if err != nil {
		if errors.Is(err, user.ErrEmailExists) {
			return "", ErrEmailExists
		}

		return "", fmt.Errorf("creating user: %w", err)
	}

	token, err := generateToken(u.ID, s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	return token, nil
}

func (s *UserAuthService) Login(ctx context.Context, email, password string) (string, error) {
	if email == "" || password == "" {
		return "", ErrInvalidCredentials
	}

	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return "", ErrInvalidCredentials
		}

		return "", fmt.Errorf("getting user by email: %w", err)
	}

	if !checkPassword(u.PasswordHash, password) {
		return "", ErrInvalidCredentials
	}

	token, err := generateToken(u.ID, s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	return token, nil
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	return string(hash), nil
}

func checkPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func generateToken(id, secret string) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   id,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(tokenDuration)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return signed, nil
}
