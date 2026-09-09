package audience

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"app/test/harness"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (s *StatefulSuite) TestCreateProductAudience() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	var userID string
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

	hypothesisID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, userID, "test hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	productID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
		productID, "test product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]string{"description": "test audience"}).
		Post("/api/v1/product/" + productID + "/audiences")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusCreated, resp.StatusCode())

	var created struct {
		ID        string `json:"id"`
		CreatedAt string `json:"created_at"`
	}
	require.NoError(s.T(), json.Unmarshal(resp.Body(), &created))
	require.NotEmpty(s.T(), created.ID)

	require.JSONEq(s.T(), fmt.Sprintf(`{
		"id": %q,
		"product_id": %q,
		"description": "test audience",
		"created_at": %q
	}`, created.ID, productID, created.CreatedAt), string(resp.Body()))

	row := harness.RowAsStrings(ctx, s.T(), pool, "target_audience", created.ID)
	require.Equal(s.T(), map[string]string{
		"id":                 created.ID,
		"product_id":         productID,
		"description":        "test audience",
		"age_from":           "<NULL>",
		"age_to":             "<NULL>",
		"gender":             "<NULL>",
		"interests":          "<NULL>",
		"geography":          "<NULL>",
		"pain_points":        "<NULL>",
		"desires_motivation": "<NULL>",
		"created_at":         created.CreatedAt,
	}, row)
}

func (s *StatefulSuite) TestCreateProductAudienceWithAllFields() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	var userID string
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

	hypothesisID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, userID, "test hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	productID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
		productID, "test product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]any{
			"description":        "full audience",
			"age_from":           18,
			"age_to":             35,
			"gender":             "all",
			"interests":          "sports",
			"geography":          "US",
			"pain_points":        "no time",
			"desires_motivation": "stay fit",
		}).
		Post("/api/v1/product/" + productID + "/audiences")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusCreated, resp.StatusCode())

	var created struct {
		ID        string `json:"id"`
		CreatedAt string `json:"created_at"`
	}
	require.NoError(s.T(), json.Unmarshal(resp.Body(), &created))
	require.NotEmpty(s.T(), created.ID)

	require.JSONEq(s.T(), fmt.Sprintf(`{
		"id": %q,
		"product_id": %q,
		"description": "full audience",
		"age_from": 18,
		"age_to": 35,
		"gender": "all",
		"interests": "sports",
		"geography": "US",
		"pain_points": "no time",
		"desires_motivation": "stay fit",
		"created_at": %q
	}`, created.ID, productID, created.CreatedAt), string(resp.Body()))

	row := harness.RowAsStrings(ctx, s.T(), pool, "target_audience", created.ID)
	require.Equal(s.T(), map[string]string{
		"id":                 created.ID,
		"product_id":         productID,
		"description":        "full audience",
		"age_from":           "18",
		"age_to":             "35",
		"gender":             "all",
		"interests":          "sports",
		"geography":          "US",
		"pain_points":        "no time",
		"desires_motivation": "stay fit",
		"created_at":         created.CreatedAt,
	}, row)
}

func (s *StatefulSuite) TestCreateProductAudienceWithInvalidGender() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	var userID string
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

	hypothesisID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, userID, "test hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	productID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
		productID, "test product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetHeader("Content-Type", "application/json").
		SetBody(`{"description": "invalid gender audience", "gender": "unknown"}`).
		Post("/api/v1/product/" + productID + "/audiences")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusBadRequest, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 400, "description": "gender must be one of men, women, all"}}`, string(resp.Body()))

	var count int
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT count(*) FROM target_audience WHERE product_id = $1", productID).Scan(&count))
	require.Equal(s.T(), 0, count)
}

func (s *StatefulSuite) TestCreateProductAudienceWithNullFields() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	var userID string
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

	hypothesisID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, userID, "test hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	productID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
		productID, "test product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetHeader("Content-Type", "application/json").
		SetBody(`{
			"description":        "null audience",
			"age_from":           null,
			"age_to":             null,
			"gender":             "all",
			"interests":          null,
			"geography":          null,
			"pain_points":        null,
			"desires_motivation": null
		}`).
		Post("/api/v1/product/" + productID + "/audiences")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusCreated, resp.StatusCode())

	var created struct {
		ID        string `json:"id"`
		CreatedAt string `json:"created_at"`
	}
	require.NoError(s.T(), json.Unmarshal(resp.Body(), &created))
	require.NotEmpty(s.T(), created.ID)

	require.JSONEq(s.T(), fmt.Sprintf(`{
		"id": %q,
		"product_id": %q,
		"description": "null audience",
		"gender": "all",
		"created_at": %q
	}`, created.ID, productID, created.CreatedAt), string(resp.Body()))

	row := harness.RowAsStrings(ctx, s.T(), pool, "target_audience", created.ID)
	require.Equal(s.T(), map[string]string{
		"id":                 created.ID,
		"product_id":         productID,
		"description":        "null audience",
		"age_from":           "<NULL>",
		"age_to":             "<NULL>",
		"gender":             "all",
		"interests":          "<NULL>",
		"geography":          "<NULL>",
		"pain_points":        "<NULL>",
		"desires_motivation": "<NULL>",
		"created_at":         created.CreatedAt,
	}, row)
}

func (s *StatefulSuite) TestCreateProductAudienceGenderValidation() {
	env := harness.CurrentEnv()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	seed := func() (string, string) {
		token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

		var userID string
		require.NoError(s.T(), pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

		hypothesisID := uuid.NewString()
		_, err = pool.Exec(ctx,
			"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
			hypothesisID, userID, "test hypothesis", "idle",
		)
		require.NoError(s.T(), err)

		productID := uuid.NewString()
		_, err = pool.Exec(ctx,
			"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
			productID, "test product", "electronics", hypothesisID,
		)
		require.NoError(s.T(), err)

		return token, productID
	}

	cases := []struct {
		name         string
		body         string
		expectedBody string
	}{
		{
			name:         "null gender",
			body:         `{"description": "null gender audience", "gender": null}`,
			expectedBody: `{"error": {"code": 400, "description": "gender must be one of men, women, all"}}`,
		},
		{
			name:         "invalid gender",
			body:         `{"description": "invalid gender audience", "gender": "unknown"}`,
			expectedBody: `{"error": {"code": 400, "description": "gender must be one of men, women, all"}}`,
		},
	}

	for _, tt := range cases {
		s.Run(tt.name, func() {
			token, productID := seed()

			resp, err := env.Client.R().
				SetAuthToken(token).
				SetHeader("Content-Type", "application/json").
				SetBody(tt.body).
				Post("/api/v1/product/" + productID + "/audiences")
			require.NoError(s.T(), err)
			assert.Equal(s.T(), http.StatusBadRequest, resp.StatusCode())

			require.JSONEq(s.T(), tt.expectedBody, string(resp.Body()))

			var count int
			require.NoError(s.T(), pool.QueryRow(ctx, "SELECT count(*) FROM target_audience WHERE product_id = $1", productID).Scan(&count))
			require.Equal(s.T(), 0, count)
		})
	}
}

func (s *StatefulSuite) TestListProductAudiences() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	var userID string
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

	hypothesisID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, userID, "test hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	productID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
		productID, "test product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	createdAt1 := time.Now().UTC().Truncate(time.Microsecond)
	createdAt2 := createdAt1.Add(time.Second)

	id1 := uuid.NewString()
	id2 := uuid.NewString()

	_, err = pool.Exec(ctx,
		`INSERT INTO target_audience (
			id, product_id, description, age_from, age_to, gender,
			interests, geography, pain_points, desires_motivation, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		id1, productID, "first audience", 18, 25, "men",
		"tech", "US", "cost", "quality", createdAt1,
	)
	require.NoError(s.T(), err)

	_, err = pool.Exec(ctx,
		`INSERT INTO target_audience (
			id, product_id, description, age_from, age_to, gender,
			interests, geography, pain_points, desires_motivation, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		id2, productID, "second audience", 26, 35, "women",
		"fitness", "CA", "time", "health", createdAt2,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		Get("/api/v1/product/" + productID + "/audiences")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusOK, resp.StatusCode())

	expected := fmt.Sprintf(`[
		{
			"id": %q,
			"product_id": %q,
			"description": "first audience",
			"age_from": 18,
			"age_to": 25,
			"gender": "men",
			"interests": "tech",
			"geography": "US",
			"pain_points": "cost",
			"desires_motivation": "quality",
			"created_at": %q
		},
		{
			"id": %q,
			"product_id": %q,
			"description": "second audience",
			"age_from": 26,
			"age_to": 35,
			"gender": "women",
			"interests": "fitness",
			"geography": "CA",
			"pain_points": "time",
			"desires_motivation": "health",
			"created_at": %q
		}
	]`, id1, productID, createdAt1.Format(time.RFC3339Nano),
		id2, productID, createdAt2.Format(time.RFC3339Nano))

	require.JSONEq(s.T(), expected, string(resp.Body()))
}

func (s *StatefulSuite) TestGetAudience() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	var userID string
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

	hypothesisID := uuid.NewString()
	productID := uuid.NewString()
	audienceID := uuid.NewString()

	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, userID, "test hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	_, err = pool.Exec(ctx,
		"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
		productID, "test product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	_, err = pool.Exec(ctx,
		`INSERT INTO target_audience (
			id, product_id, description, age_from, age_to, gender,
			interests, geography, pain_points, desires_motivation, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		audienceID, productID, "test audience", 18, 35, "all",
		"sports", "US", "no time", "stay fit", createdAt,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		Get("/api/v1/audiences/" + audienceID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusOK, resp.StatusCode())

	var got struct {
		CreatedAt string `json:"created_at"`
	}
	require.NoError(s.T(), json.Unmarshal(resp.Body(), &got))

	require.JSONEq(s.T(), fmt.Sprintf(`{
		"id": %q,
		"product_id": %q,
		"description": "test audience",
		"age_from": 18,
		"age_to": 35,
		"gender": "all",
		"interests": "sports",
		"geography": "US",
		"pain_points": "no time",
		"desires_motivation": "stay fit",
		"created_at": %q
	}`, audienceID, productID, got.CreatedAt), string(resp.Body()))
}

func (s *StatefulSuite) TestUpdateAudience() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	var userID string
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

	hypothesisID := uuid.NewString()
	productID := uuid.NewString()
	audienceID := uuid.NewString()

	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, userID, "test hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	_, err = pool.Exec(ctx,
		"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
		productID, "test product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	_, err = pool.Exec(ctx,
		`INSERT INTO target_audience (
			id, product_id, description, age_from, age_to, gender,
			interests, geography, pain_points, desires_motivation, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		audienceID, productID, "old audience", 18, 35, "all",
		"sports", "US", "no time", "stay fit", createdAt,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]any{
			"description":        "updated audience",
			"age_from":           25,
			"age_to":             45,
			"gender":             "men",
			"interests":          "tech",
			"geography":          "UK",
			"pain_points":        "slow",
			"desires_motivation": "speed",
		}).
		Patch("/api/v1/audiences/" + audienceID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusOK, resp.StatusCode())

	var got struct {
		CreatedAt string `json:"created_at"`
	}
	require.NoError(s.T(), json.Unmarshal(resp.Body(), &got))

	require.JSONEq(s.T(), fmt.Sprintf(`{
		"id": %q,
		"product_id": %q,
		"description": "updated audience",
		"age_from": 25,
		"age_to": 45,
		"gender": "men",
		"interests": "tech",
		"geography": "UK",
		"pain_points": "slow",
		"desires_motivation": "speed",
		"created_at": %q
	}`, audienceID, productID, got.CreatedAt), string(resp.Body()))

	row := harness.RowAsStrings(ctx, s.T(), pool, "target_audience", audienceID)
	require.Equal(s.T(), map[string]string{
		"id":                 audienceID,
		"product_id":         productID,
		"description":        "updated audience",
		"age_from":           "25",
		"age_to":             "45",
		"gender":             "men",
		"interests":          "tech",
		"geography":          "UK",
		"pain_points":        "slow",
		"desires_motivation": "speed",
		"created_at":         got.CreatedAt,
	}, row)
}

func (s *StatefulSuite) TestUpdateAudienceNoFields() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	var userID string
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

	hypothesisID := uuid.NewString()
	productID := uuid.NewString()
	audienceID := uuid.NewString()

	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, userID, "test hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	_, err = pool.Exec(ctx,
		"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
		productID, "test product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	_, err = pool.Exec(ctx,
		`INSERT INTO target_audience (
			id, product_id, description, age_from, age_to, gender,
			interests, geography, pain_points, desires_motivation, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		audienceID, productID, "test audience", 18, 35, "all",
		"sports", "US", "no time", "stay fit", createdAt,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]any{}).
		Patch("/api/v1/audiences/" + audienceID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusOK, resp.StatusCode())

	require.JSONEq(s.T(), fmt.Sprintf(`{
		"id": %q,
		"product_id": %q,
		"description": "test audience",
		"age_from": 18,
		"age_to": 35,
		"gender": "all",
		"interests": "sports",
		"geography": "US",
		"pain_points": "no time",
		"desires_motivation": "stay fit",
		"created_at": %q
	}`, audienceID, productID, createdAt.Format(time.RFC3339Nano)), string(resp.Body()))

	row := harness.RowAsStrings(ctx, s.T(), pool, "target_audience", audienceID)
	require.Equal(s.T(), map[string]string{
		"id":                 audienceID,
		"product_id":         productID,
		"description":        "test audience",
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

func (s *StatefulSuite) TestDeleteAudience() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	var userID string
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

	hypothesisID := uuid.NewString()
	productID := uuid.NewString()
	audienceID := uuid.NewString()

	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, userID, "test hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	_, err = pool.Exec(ctx,
		"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
		productID, "test product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	_, err = pool.Exec(ctx,
		`INSERT INTO target_audience (
			id, product_id, description, age_from, age_to, gender,
			interests, geography, pain_points, desires_motivation, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		audienceID, productID, "test audience", 18, 35, "all",
		"sports", "US", "no time", "stay fit", createdAt,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		Delete("/api/v1/audiences/" + audienceID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNoContent, resp.StatusCode())
	require.Empty(s.T(), resp.Body())

	var count int
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT count(*) FROM target_audience WHERE id = $1", audienceID).Scan(&count))
	require.Equal(s.T(), 0, count)
}

func (s *StatefulSuite) TestCreateProductAudienceInvalidAgeOrder() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	var userID string
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

	hypothesisID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, userID, "test hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	productID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
		productID, "test product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]any{"description": "test audience", "age_from": 35, "age_to": 18}).
		Post("/api/v1/product/" + productID + "/audiences")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusBadRequest, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 400, "description": "age_from must be less than or equal to age_to"}}`, string(resp.Body()))

	var count int
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT count(*) FROM target_audience WHERE product_id = $1", productID).Scan(&count))
	require.Equal(s.T(), 0, count)
}

func (s *StatefulSuite) TestUpdateAudienceInvalidAgeRange() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	var userID string
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

	hypothesisID := uuid.NewString()
	productID := uuid.NewString()
	audienceID := uuid.NewString()

	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, userID, "test hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	_, err = pool.Exec(ctx,
		"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
		productID, "test product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	_, err = pool.Exec(ctx,
		`INSERT INTO target_audience (
			id, product_id, description, age_from, age_to, gender,
			interests, geography, pain_points, desires_motivation, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		audienceID, productID, "test audience", 18, 35, "all",
		"sports", "US", "no time", "stay fit", createdAt,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]any{"age_from": 200}).
		Patch("/api/v1/audiences/" + audienceID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusBadRequest, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 400, "description": "age_from must be less than or equal to 100"}}`, string(resp.Body()))

	row := harness.RowAsStrings(ctx, s.T(), pool, "target_audience", audienceID)
	require.Equal(s.T(), map[string]string{
		"id":                 audienceID,
		"product_id":         productID,
		"description":        "test audience",
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

func (s *StatefulSuite) TestUpdateAudienceInvalidAgeOrder() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	var userID string
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

	hypothesisID := uuid.NewString()
	productID := uuid.NewString()
	audienceID := uuid.NewString()

	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, userID, "test hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	_, err = pool.Exec(ctx,
		"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
		productID, "test product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	_, err = pool.Exec(ctx,
		`INSERT INTO target_audience (
			id, product_id, description, age_from, age_to, gender,
			interests, geography, pain_points, desires_motivation, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		audienceID, productID, "test audience", 18, 35, "all",
		"sports", "US", "no time", "stay fit", createdAt,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]any{"age_from": 35, "age_to": 18}).
		Patch("/api/v1/audiences/" + audienceID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusBadRequest, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 400, "description": "age_from must be less than or equal to age_to"}}`, string(resp.Body()))

	row := harness.RowAsStrings(ctx, s.T(), pool, "target_audience", audienceID)
	require.Equal(s.T(), map[string]string{
		"id":                 audienceID,
		"product_id":         productID,
		"description":        "test audience",
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

func (s *StatefulSuite) TestCreateProductAudienceSpecConstraints() {
	env := harness.CurrentEnv()

	cases := []struct {
		name string
		body map[string]any
		want string
	}{
		{
			name: "age_from above maximum",
			body: map[string]any{"description": "a", "age_from": 150},
			want: "age_from must be less than or equal to 100",
		},
		{
			name: "age_to below minimum",
			body: map[string]any{"description": "a", "age_to": -1},
			want: "age_to must be greater than or equal to 0",
		},
		{
			name: "interests exceeds maxLength",
			body: map[string]any{"description": "a", "interests": strings.Repeat("x", 1001)},
			want: "interests must be at most 1000 characters long",
		},
		{
			name: "geography exceeds maxLength",
			body: map[string]any{"description": "a", "geography": strings.Repeat("x", 256)},
			want: "geography must be at most 255 characters long",
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			t := s.T()
			token := harness.RegisterUser(t, env, "user@test.com", "password123")

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			pool, err := pgxpool.New(ctx, env.DatabaseURL())
			require.NoError(t, err)
			defer pool.Close()

			var userID string
			require.NoError(t, pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

			hypothesisID := uuid.NewString()
			_, err = pool.Exec(ctx,
				"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
				hypothesisID, userID, "test hypothesis", "idle",
			)
			require.NoError(t, err)

			productID := uuid.NewString()
			_, err = pool.Exec(ctx,
				"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
				productID, "test product", "electronics", hypothesisID,
			)
			require.NoError(t, err)

			resp, err := env.Client.R().
				SetAuthToken(token).
				SetBody(tc.body).
				Post("/api/v1/product/" + productID + "/audiences")
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, resp.StatusCode())
			require.JSONEq(t, fmt.Sprintf(`{"error": {"code": 400, "description": %q}}`, tc.want), string(resp.Body()))

var count int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM target_audience WHERE product_id = $1", productID).Scan(&count))
		require.Equal(t, 0, count, "no audience should be persisted when validation fails")
		})
	}
}

func (s *StatefulSuite) TestCreateProductAudienceDescriptionRequired() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	var userID string
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

	hypothesisID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, userID, "test hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	productID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
		productID, "test product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]string{"description": ""}).
		Post("/api/v1/product/" + productID + "/audiences")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusBadRequest, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 400, "description": "description is required"}}`, string(resp.Body()))
}

func (s *StatefulSuite) TestUpdateAudienceInvalidGender() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	var userID string
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

	hypothesisID := uuid.NewString()
	productID := uuid.NewString()
	audienceID := uuid.NewString()

	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, userID, "test hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	_, err = pool.Exec(ctx,
		"INSERT INTO product (id, name, category, hypothesis_id) VALUES ($1, $2, $3, $4)",
		productID, "test product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	_, err = pool.Exec(ctx,
		`INSERT INTO target_audience (
			id, product_id, description, age_from, age_to, gender,
			interests, geography, pain_points, desires_motivation, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		audienceID, productID, "test audience", 18, 35, "all",
		"sports", "US", "no time", "stay fit", createdAt,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]string{"gender": "invalid"}).
		Patch("/api/v1/audiences/" + audienceID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusBadRequest, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 400, "description": "gender must be one of men, women, all"}}`, string(resp.Body()))

	row := harness.RowAsStrings(ctx, s.T(), pool, "target_audience", audienceID)
	require.Equal(s.T(), map[string]string{
		"id":                 audienceID,
		"product_id":         productID,
		"description":        "test audience",
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

func (s *StatefulSuite) TestGetAudienceNotFound() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	resp, err := env.Client.R().
		SetAuthToken(token).
		Get("/api/v1/audiences/00000000-0000-0000-0000-000000000000")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())

	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "audience not found"}}`, string(resp.Body()))
}

func (s *StatefulSuite) TestGetAudienceBadUUID() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	resp, err := env.Client.R().
		SetAuthToken(token).
		Get("/api/v1/audiences/not-a-uuid")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())

	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "audience not found"}}`, string(resp.Body()))
}

func (s *StatefulSuite) TestCreateProductAudienceProductNotFound() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]string{"description": "test audience"}).
		Post("/api/v1/product/00000000-0000-0000-0000-000000000000/audiences")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())

	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "product not found"}}`, string(resp.Body()))
}

func (s *StatefulSuite) TestListProductAudiencesProductNotFound() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	resp, err := env.Client.R().
		SetAuthToken(token).
		Get("/api/v1/product/00000000-0000-0000-0000-000000000000/audiences")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())

	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "product not found"}}`, string(resp.Body()))
}

func (s *StatefulSuite) TestUpdateAudienceNotFound() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]string{"description": "updated"}).
		Patch("/api/v1/audiences/00000000-0000-0000-0000-000000000000")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())

	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "audience not found"}}`, string(resp.Body()))
}

func (s *StatefulSuite) TestDeleteAudienceNotFound() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	resp, err := env.Client.R().
		SetAuthToken(token).
		Delete("/api/v1/audiences/00000000-0000-0000-0000-000000000000")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())

	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "audience not found"}}`, string(resp.Body()))
}
