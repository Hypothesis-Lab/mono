package audience

import (
	"context"
	"net/http"
	"time"

	"app/test/harness"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func (s *StatefulSuite) TestGetAudienceOtherUser() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	otherID := harness.UserIDFromToken(s.T(), harness.RegisterUser(s.T(), env, "other@test.com", "password123"))

	hypothesisID := uuid.NewString()
	productID := uuid.NewString()
	audienceID := uuid.NewString()

	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, otherID, "other hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	_, err = pool.Exec(ctx,
		"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
		productID, "other product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	_, err = pool.Exec(ctx,
		`INSERT INTO target_audience (
			id, product_id, description, age_from, age_to, gender,
			interests, geography, pain_points, desires_motivation, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		audienceID, productID, "other audience", 18, 35, "all",
		"sports", "US", "no time", "stay fit", createdAt,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		Get("/api/v1/audiences/" + audienceID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "audience not found"}}`, string(resp.Body()))
}

func (s *StatefulSuite) TestCreateProductAudienceOtherUserProduct() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	otherID := harness.UserIDFromToken(s.T(), harness.RegisterUser(s.T(), env, "other@test.com", "password123"))

	hypothesisID := uuid.NewString()
	productID := uuid.NewString()

	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, otherID, "other hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	_, err = pool.Exec(ctx,
		"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
		productID, "other product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]string{"description": "test audience"}).
		Post("/api/v1/product/" + productID + "/audiences")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "product not found"}}`, string(resp.Body()))

	var count int
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT count(*) FROM target_audience WHERE product_id = $1", productID).Scan(&count))
	require.Equal(s.T(), 0, count)
}

func (s *StatefulSuite) TestListProductAudiencesOtherUserProduct() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	otherID := harness.UserIDFromToken(s.T(), harness.RegisterUser(s.T(), env, "other@test.com", "password123"))

	hypothesisID := uuid.NewString()
	productID := uuid.NewString()
	audienceID := uuid.NewString()

	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, otherID, "other hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	_, err = pool.Exec(ctx,
		"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
		productID, "other product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	_, err = pool.Exec(ctx,
		`INSERT INTO target_audience (
			id, product_id, description, age_from, age_to, gender,
			interests, geography, pain_points, desires_motivation, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		audienceID, productID, "other audience", 18, 35, "all",
		"sports", "US", "no time", "stay fit", createdAt,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		Get("/api/v1/product/" + productID + "/audiences")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "product not found"}}`, string(resp.Body()))
}

func (s *StatefulSuite) TestUpdateAudienceOtherUser() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	otherID := harness.UserIDFromToken(s.T(), harness.RegisterUser(s.T(), env, "other@test.com", "password123"))

	hypothesisID := uuid.NewString()
	productID := uuid.NewString()
	audienceID := uuid.NewString()

	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, otherID, "other hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	_, err = pool.Exec(ctx,
		"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
		productID, "other product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	_, err = pool.Exec(ctx,
		`INSERT INTO target_audience (
			id, product_id, description, age_from, age_to, gender,
			interests, geography, pain_points, desires_motivation, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		audienceID, productID, "other audience", 18, 35, "all",
		"sports", "US", "no time", "stay fit", createdAt,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]string{"description": "hijacked"}).
		Patch("/api/v1/audiences/" + audienceID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "audience not found"}}`, string(resp.Body()))

	row := harness.RowAsStrings(ctx, s.T(), pool, "target_audience", audienceID)
	require.Equal(s.T(), map[string]string{
		"id":                 audienceID,
		"product_id":         productID,
		"description":        "other audience",
		"age_from":           "18",
		"age_to":             "35",
		"gender":             "all",
		"interests":          "sports",
		"geography":          "US",
		"pain_points":        "no time",
		"desires_motivation": "stay fit",
		"created_at":         createdAt.Format(time.RFC3339Nano),
	}, row)
}

func (s *StatefulSuite) TestDeleteAudienceOtherUser() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	otherID := harness.UserIDFromToken(s.T(), harness.RegisterUser(s.T(), env, "other@test.com", "password123"))

	hypothesisID := uuid.NewString()
	productID := uuid.NewString()
	audienceID := uuid.NewString()

	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, otherID, "other hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	_, err = pool.Exec(ctx,
		"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
		productID, "other product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	_, err = pool.Exec(ctx,
		`INSERT INTO target_audience (
			id, product_id, description, age_from, age_to, gender,
			interests, geography, pain_points, desires_motivation, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		audienceID, productID, "other audience", 18, 35, "all",
		"sports", "US", "no time", "stay fit", createdAt,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		Delete("/api/v1/audiences/" + audienceID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "audience not found"}}`, string(resp.Body()))

	var count int
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT count(*) FROM target_audience WHERE id = $1", audienceID).Scan(&count))
	require.Equal(s.T(), 1, count)
}