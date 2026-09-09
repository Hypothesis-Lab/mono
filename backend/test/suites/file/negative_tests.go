package file

import (
	"context"
	"net/http"
	"time"

	"app/test/harness"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func (s *StatefulSuite) TestUploadProductFileOtherUser() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	otherID := harness.UserIDFromToken(s.T(), harness.RegisterUser(s.T(), env, "other@test.com", "password123"))

	hypothesisID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, otherID, "owner hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	body, contentType := buildUploadRequest("test.pdf", "application/pdf", pdfContent, "other")

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetHeader("Content-Type", contentType).
		SetBody(body.Bytes()).
		Post("/api/v1/hypothesis/" + hypothesisID + "/files")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "hypothesis not found"}}`, resp.String())

	var count int
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT count(*) FROM file WHERE hypothesis_id = $1", hypothesisID).Scan(&count))
	require.Equal(s.T(), 0, count)
}

func (s *StatefulSuite) TestListProductFilesOtherUser() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	otherID := harness.UserIDFromToken(s.T(), harness.RegisterUser(s.T(), env, "other@test.com", "password123"))

	hypothesisID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, otherID, "owner hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	fileID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO file (id, hypothesis_id, type, filename, content_type, size, s3_key, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		fileID, hypothesisID, "other", "test.pdf", "application/pdf", 11, "files/"+fileID, time.Now().UTC().Truncate(time.Microsecond),
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		Get("/api/v1/hypothesis/" + hypothesisID + "/files")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "hypothesis not found"}}`, resp.String())
}

func (s *StatefulSuite) TestDeleteProductFileOtherUser() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	otherID := harness.UserIDFromToken(s.T(), harness.RegisterUser(s.T(), env, "other@test.com", "password123"))

	hypothesisID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, otherID, "owner hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	fileID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO file (id, hypothesis_id, type, filename, content_type, size, s3_key, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		fileID, hypothesisID, "other", "test.pdf", "application/pdf", 11, "files/"+fileID, time.Now().UTC().Truncate(time.Microsecond),
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		Delete("/api/v1/files/" + fileID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "file not found"}}`, resp.String())

	var count int
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT count(*) FROM file WHERE id = $1", fileID).Scan(&count))
	require.Equal(s.T(), 1, count)
}
