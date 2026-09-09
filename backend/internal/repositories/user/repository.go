package user

import (
	"context"
	"errors"
	"fmt"

	"app/internal/db"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const uniqueViolationCode = "23505"

var (
	ErrNotFound   = errors.New("user not found")
	ErrEmailExists = errors.New("email already exists")

	squirrelPSQL = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
)

type User struct {
	ID           string
	Email        string
	PasswordHash string
}

type CreateInput struct {
	Email        string
	PasswordHash string
}

type Repository struct {
	db *db.DB
}

func NewRepository(db *db.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, in CreateInput) (*User, error) {
	sql, args, err := squirrelPSQL.
		Insert("users").
		Columns("id", "email", "password_hash").
		Values(uuid.NewString(), in.Email, in.PasswordHash).
		Suffix("RETURNING id, email, password_hash").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build create user query: %w", err)
	}

	u := &User{}
	if err := r.db.Pool().QueryRow(ctx, sql, args...).
		Scan(&u.ID, &u.Email, &u.PasswordHash); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
			return nil, ErrEmailExists
		}

		return nil, fmt.Errorf("create user: %w", err)
	}

	return u, nil
}

func (r *Repository) GetByEmail(ctx context.Context, email string) (*User, error) {
	sql, args, err := squirrelPSQL.
		Select("id", "email", "password_hash").
		From("users").
		Where(squirrel.Eq{"email": email}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get user by email query: %w", err)
	}

	u := &User{}
	if err := r.db.Pool().QueryRow(ctx, sql, args...).
		Scan(&u.ID, &u.Email, &u.PasswordHash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query user by email: %w", err)
	}

	return u, nil
}
