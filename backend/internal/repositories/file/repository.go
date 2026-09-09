package file

import (
	"context"
	"errors"
	"fmt"
	"time"

	"app/internal/db"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNotFound = errors.New("file not found")

	squirrelPSQL = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
)

type Type string

const (
	TypeOther     Type = "other"
	TypeBrandbook Type = "brandbook"
	TypeProduct   Type = "product"
	TypeReference Type = "reference"
)

type File struct {
	ID           string
	HypothesisID string
	Type         Type
	Filename     string
	ContentType  string
	Size         int64
	S3Key        string
	CreatedAt    time.Time
}

type CreateInput struct {
	HypothesisID string
	Type         Type
	Filename     string
	ContentType  string
	Size         int64
	S3Key        string
}

type Repository struct {
	db *db.DB
}

func NewRepository(db *db.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) DeleteByHypothesisID(ctx context.Context, hypothesisID string) error {
	sql, args, err := squirrelPSQL.
		Delete("file").
		Where(squirrel.Eq{"hypothesis_id": hypothesisID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build delete files by hypothesis query: %w", err)
	}

	if _, err := r.db.Pool().Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("delete files by hypothesis %s: %w", hypothesisID, err)
	}

	return nil
}

func (r *Repository) ListByHypothesisID(ctx context.Context, hypothesisID string) ([]*File, error) {
	sql, args, err := squirrelPSQL.
		Select("id", "hypothesis_id", "type", "filename", "content_type", "size", "s3_key", "created_at").
		From("file").
		Where(squirrel.Eq{"hypothesis_id": hypothesisID}).
		OrderBy("created_at").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list files query: %w", err)
	}

	rows, err := r.db.Pool().Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query files: %w", err)
	}
	defer rows.Close()

	var result []*File
	for rows.Next() {
		f := &File{}
		if err := rows.Scan(&f.ID, &f.HypothesisID, &f.Type, &f.Filename, &f.ContentType, &f.Size, &f.S3Key, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan file: %w", err)
		}

		result = append(result, f)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate files: %w", err)
	}

	return result, nil
}

func (r *Repository) Create(ctx context.Context, in CreateInput) (*File, error) {
	sql, args, err := squirrelPSQL.
		Insert("file").
		Columns("id", "hypothesis_id", "type", "filename", "content_type", "size", "s3_key").
		Values(uuid.NewString(), in.HypothesisID, in.Type, in.Filename, in.ContentType, in.Size, in.S3Key).
		Suffix("RETURNING id, hypothesis_id, type, filename, content_type, size, s3_key, created_at").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build create file query: %w", err)
	}

	f := &File{}
	if err := r.db.Pool().QueryRow(ctx, sql, args...).
		Scan(&f.ID, &f.HypothesisID, &f.Type, &f.Filename, &f.ContentType, &f.Size, &f.S3Key, &f.CreatedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrBrandbookAlreadyExists
		}

		return nil, fmt.Errorf("create file: %w", err)
	}

	return f, nil
}

func (r *Repository) Get(ctx context.Context, id string) (*File, error) {
	sql, args, err := squirrelPSQL.
		Select("id", "hypothesis_id", "type", "filename", "content_type", "size", "s3_key", "created_at").
		From("file").
		Where(squirrel.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get file query: %w", err)
	}

	f := &File{}
	if err := r.db.Pool().QueryRow(ctx, sql, args...).
		Scan(&f.ID, &f.HypothesisID, &f.Type, &f.Filename, &f.ContentType, &f.Size, &f.S3Key, &f.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query file %s: %w", id, err)
	}

	return f, nil
}

func (r *Repository) GetByUserID(ctx context.Context, userID, id string) (*File, error) {
	sql, args, err := squirrelPSQL.
		Select("f.id", "f.hypothesis_id", "f.type", "f.filename", "f.content_type", "f.size", "f.s3_key", "f.created_at").
		From("file f").
		Join("hypothesis h ON h.id = f.hypothesis_id").
		Where(squirrel.Eq{"f.id": id, "h.user_id": userID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get file by user query: %w", err)
	}

	f := &File{}
	if err := r.db.Pool().QueryRow(ctx, sql, args...).
		Scan(&f.ID, &f.HypothesisID, &f.Type, &f.Filename, &f.ContentType, &f.Size, &f.S3Key, &f.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query file %s for user %s: %w", id, userID, err)
	}

	return f, nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	sql, args, err := squirrelPSQL.
		Delete("file").
		Where(squirrel.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build delete file query: %w", err)
	}

	cmd, err := r.db.Pool().Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("delete file %s: %w", id, err)
	}

	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

var ErrBrandbookAlreadyExists = errors.New("brandbook already exists for this hypothesis")
