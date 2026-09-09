package hypothesis

import (
	"context"
	"net/http"
	"time"

	"app/test/harness"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func (s *StatefulSuite) TestGetHypothesisOtherUser() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	otherID := harness.UserIDFromToken(s.T(), harness.RegisterUser(s.T(), env, "other@test.com", "password123"))

	id := uuid.NewString()
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status, created_at) VALUES ($1, $2, $3, $4, $5)",
		id, otherID, "owner hypothesis", "idle", createdAt,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		Get("/api/v1/hypothesis/" + id)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "hypothesis not found"}}`, resp.String())

	row := harness.RowAsStrings(ctx, s.T(), pool, "hypothesis", id)
	require.Equal(s.T(), map[string]string{
		"id":          id,
		"user_id":     otherID,
		"description": "owner hypothesis",
		"status":      "idle",
		"created_at":  createdAt.Format(time.RFC3339Nano),
	}, row)
}

func (s *StatefulSuite) TestUpdateHypothesisOtherUser() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	otherID := harness.UserIDFromToken(s.T(), harness.RegisterUser(s.T(), env, "other@test.com", "password123"))

	id := uuid.NewString()
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status, created_at) VALUES ($1, $2, $3, $4, $5)",
		id, otherID, "owner hypothesis", "idle", createdAt,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]string{"description": "hijacked"}).
		Patch("/api/v1/hypothesis/" + id)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "hypothesis not found"}}`, resp.String())

	row := harness.RowAsStrings(ctx, s.T(), pool, "hypothesis", id)
	require.Equal(s.T(), map[string]string{
		"id":          id,
		"user_id":     otherID,
		"description": "owner hypothesis",
		"status":      "idle",
		"created_at":  createdAt.Format(time.RFC3339Nano),
	}, row)
}

func (s *StatefulSuite) TestDeleteHypothesisOtherUser() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	otherID := harness.UserIDFromToken(s.T(), harness.RegisterUser(s.T(), env, "other@test.com", "password123"))

	id := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		id, otherID, "owner hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		Delete("/api/v1/hypothesis/" + id)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "hypothesis not found"}}`, resp.String())

	var count int
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT count(*) FROM hypothesis WHERE id = $1", id).Scan(&count))
	require.Equal(s.T(), 1, count)
}

func (s *StatefulSuite) TestLaunchHypothesisOtherUser() {
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

	productID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
		productID, "owner product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		Post("/api/v1/hypothesis/" + hypothesisID + "/launch")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "hypothesis not found"}}`, resp.String())

	row := harness.RowAsStrings(ctx, s.T(), pool, "hypothesis", hypothesisID)
	require.Equal(s.T(), "idle", row["status"])
}

func (s *StatefulSuite) TestStopHypothesisOtherUser() {
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
		hypothesisID, otherID, "owner hypothesis", "in_progress",
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		Post("/api/v1/hypothesis/" + hypothesisID + "/stop")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "hypothesis not found"}}`, resp.String())

	row := harness.RowAsStrings(ctx, s.T(), pool, "hypothesis", hypothesisID)
	require.Equal(s.T(), "in_progress", row["status"])
}

func (s *StatefulSuite) TestGetHypothesisProductOtherUser() {
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

	productID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
		productID, "owner product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		Get("/api/v1/hypothesis/" + hypothesisID + "/product")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "hypothesis not found"}}`, resp.String())

	var productCount int
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT count(*) FROM product WHERE id = $1", productID).Scan(&productCount))
	require.Equal(s.T(), 1, productCount)
}

func (s *StatefulSuite) TestListHypothesesOtherUser() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	otherID := harness.UserIDFromToken(s.T(), harness.RegisterUser(s.T(), env, "other@test.com", "password123"))

	id := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		id, otherID, "owner hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		Get("/api/v1/hypothesis")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusOK, resp.StatusCode())
	require.JSONEq(s.T(), `[]`, resp.String())

	var count int
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT count(*) FROM hypothesis WHERE id = $1", id).Scan(&count))
	require.Equal(s.T(), 1, count)
}

