package product

import (
	"context"
	"net/http"
	"time"

	"app/test/harness"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func (s *StatefulSuite) TestCreateProductOtherUserHypothesis() {
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
		hypothesisID, otherID, "other hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]string{
			"name":          "test product",
			"category":      "electronics",
			"hypothesis_id": hypothesisID,
		}).
		Post("/api/v1/product")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "hypothesis not found"}}`, string(resp.Body()))

	var count int
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT count(*) FROM product WHERE hypothesis_id = $1", hypothesisID).Scan(&count))
	require.Equal(s.T(), 0, count)
}

func (s *StatefulSuite) TestUpdateProductOtherUser() {
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
		hypothesisID, otherID, "other hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	productID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
		productID, "other product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]string{"name": "hijacked"}).
		Patch("/api/v1/product/" + productID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "product not found"}}`, string(resp.Body()))

	row := harness.RowAsStrings(ctx, s.T(), pool, "product", productID)
	require.Equal(s.T(), map[string]string{
		"id":            productID,
		"name":          "other product",
		"description":   "<NULL>",
		"category":      "electronics",
		"keywords":      "{}",
		"price":         "<NULL>",
		"link":          "<NULL>",
		"hypothesis_id": hypothesisID,
	}, row)
}

func (s *StatefulSuite) TestDeleteProductOtherUser() {
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
		hypothesisID, otherID, "other hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	productID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
		productID, "other product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		Delete("/api/v1/product/" + productID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "product not found"}}`, string(resp.Body()))

	var count int
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT count(*) FROM product WHERE id = $1", productID).Scan(&count))
	require.Equal(s.T(), 1, count)
}