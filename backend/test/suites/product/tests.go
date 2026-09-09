package product

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"app/test/harness"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func (s *StatefulSuite) TestCreateProduct() {
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

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]string{
			"name":          "test product",
			"category":      "electronics",
			"hypothesis_id": hypothesisID,
		}).
		Post("/api/v1/product")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusCreated, resp.StatusCode())

	var created struct {
		ID string `json:"id"`
	}
	require.NoError(s.T(), json.Unmarshal(resp.Body(), &created))
	require.NotEmpty(s.T(), created.ID)

	require.JSONEq(s.T(), fmt.Sprintf(`{
		"id": %q,
		"name": "test product",
		"category": "electronics",
		"keywords": [],
		"hypothesis_id": %q
	}`, created.ID, hypothesisID), string(resp.Body()))

	row := harness.RowAsStrings(ctx, s.T(), pool, "product", created.ID)
	require.Equal(s.T(), map[string]string{
		"id":            created.ID,
		"name":          "test product",
		"description":   "<NULL>",
		"category":      "electronics",
		"keywords":      "{}",
		"price":         "<NULL>",
		"link":          "<NULL>",
		"hypothesis_id": hypothesisID,
	}, row)
}

func (s *StatefulSuite) TestCreateProductWithAllFields() {
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

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]any{
			"name":          "full product",
			"description":   "a description",
			"category":      "books",
			"keywords":      []string{"kw1", "kw2"},
			"price":         "19.99",
			"link":          "https://example.com",
			"hypothesis_id": hypothesisID,
		}).
		Post("/api/v1/product")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusCreated, resp.StatusCode())

	var created struct {
		ID string `json:"id"`
	}
	require.NoError(s.T(), json.Unmarshal(resp.Body(), &created))
	require.NotEmpty(s.T(), created.ID)

	require.JSONEq(s.T(), fmt.Sprintf(`{
		"id": %q,
		"name": "full product",
		"description": "a description",
		"category": "books",
		"keywords": ["kw1", "kw2"],
		"price": "19.99",
		"link": "https://example.com",
		"hypothesis_id": %q
	}`, created.ID, hypothesisID), string(resp.Body()))

	row := harness.RowAsStrings(ctx, s.T(), pool, "product", created.ID)
	require.Equal(s.T(), map[string]string{
		"id":            created.ID,
		"name":          "full product",
		"description":   "a description",
		"category":      "books",
		"keywords":      "{kw1,kw2}",
		"price":         "19.99",
		"link":          "https://example.com",
		"hypothesis_id": hypothesisID,
	}, row)
}

func (s *StatefulSuite) TestUpdateProduct() {
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
		productID, "old product", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]any{
			"name":        "updated product",
			"description": "updated description",
			"category":    "books",
			"keywords":    []string{"new"},
			"price":       "9.99",
			"link":        "https://updated.example.com",
		}).
		Patch("/api/v1/product/" + productID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusOK, resp.StatusCode())

	require.JSONEq(s.T(), fmt.Sprintf(`{
		"id": %q,
		"name": "updated product",
		"description": "updated description",
		"category": "books",
		"keywords": ["new"],
		"price": "9.99",
		"link": "https://updated.example.com",
		"hypothesis_id": %q
	}`, productID, hypothesisID), string(resp.Body()))

	row := harness.RowAsStrings(ctx, s.T(), pool, "product", productID)
	require.Equal(s.T(), map[string]string{
		"id":            productID,
		"name":          "updated product",
		"description":   "updated description",
		"category":      "books",
		"keywords":      "{new}",
		"price":         "9.99",
		"link":          "https://updated.example.com",
		"hypothesis_id": hypothesisID,
	}, row)
}

func (s *StatefulSuite) TestDeleteProduct() {
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
		productID, "to delete", "electronics", hypothesisID,
	)
	require.NoError(s.T(), err)

	fileContent := []byte("%PDF-1.4 test content")
	s3Key := uuid.NewString() + "/test.pdf"

	_, err = env.S3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(env.S3Bucket()),
		Key:           aws.String(s3Key),
		Body:          bytes.NewReader(fileContent),
		ContentType:   aws.String("application/pdf"),
		ContentLength: aws.Int64(int64(len(fileContent))),
	})
	require.NoError(s.T(), err)

	fileID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO file (id, hypothesis_id, type, filename, content_type, size, s3_key) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		fileID, hypothesisID, "other", "test.pdf", "application/pdf", len(fileContent), s3Key,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		Delete("/api/v1/product/" + productID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNoContent, resp.StatusCode())
	require.Empty(s.T(), resp.Body())

	var count int
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT count(*) FROM product WHERE id = $1", productID).Scan(&count))
	require.Equal(s.T(), 0, count)

	var fileCount int
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT count(*) FROM file WHERE id = $1", fileID).Scan(&fileCount))
	require.Equal(s.T(), 0, fileCount)

	var hypothesisCount int
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT count(*) FROM hypothesis WHERE id = $1", hypothesisID).Scan(&hypothesisCount))
	require.Equal(s.T(), 1, hypothesisCount)

	_, err = env.S3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(env.S3Bucket()),
		Key:    aws.String(s3Key),
	})
	var noSuchKey *types.NoSuchKey
	require.ErrorAs(s.T(), err, &noSuchKey)
}

func (s *StatefulSuite) TestCreateProductSpecConstraints() {
	env := harness.CurrentEnv()

	cases := []struct {
		name string
		body map[string]any
		want string
	}{
		{
			name: "keyword exceeds maxLength",
			body: map[string]any{
				"name":          "test product",
				"category":      "electronics",
				"hypothesis_id": "placeholder",
				"keywords":      []string{strings.Repeat("x", 65)},
			},
			want: "keywords must be at most 64 characters long",
		},
		{
			name: "link exceeds maxLength",
			body: map[string]any{
				"name":          "test product",
				"category":      "electronics",
				"hypothesis_id": "placeholder",
				"link":          strings.Repeat("x", 129),
			},
			want: "link must be at most 128 characters long",
		},
		{
			name: "price exceeds maxLength",
			body: map[string]any{
				"name":          "test product",
				"category":      "electronics",
				"hypothesis_id": "placeholder",
				"price":         strings.Repeat("x", 129),
			},
			want: "price must be at most 128 characters long",
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

			tc.body["hypothesis_id"] = hypothesisID

			resp, err := env.Client.R().
				SetAuthToken(token).
				SetBody(tc.body).
				Post("/api/v1/product")
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, resp.StatusCode())
			require.JSONEq(t, fmt.Sprintf(`{"error": {"code": 400, "description": %q}}`, tc.want), string(resp.Body()))

			var count int
			require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM product WHERE hypothesis_id = $1", hypothesisID).Scan(&count))
			require.Equal(t, 0, count, "no product should be persisted when validation fails")
		})
	}
}

func (s *StatefulSuite) TestUpdateProductSpecConstraints() {
	env := harness.CurrentEnv()

	cases := []struct {
		name string
		body map[string]any
		want string
	}{
		{
			name: "keyword exceeds maxLength",
			body: map[string]any{"keywords": []string{strings.Repeat("x", 65)}},
			want: "keywords must be at most 64 characters long",
		},
		{
			name: "link exceeds maxLength",
			body: map[string]any{"link": strings.Repeat("x", 129)},
			want: "link must be at most 128 characters long",
		},
		{
			name: "price exceeds maxLength",
			body: map[string]any{"price": strings.Repeat("x", 129)},
			want: "price must be at most 128 characters long",
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
				Patch("/api/v1/product/" + productID)
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, resp.StatusCode())
			require.JSONEq(t, fmt.Sprintf(`{"error": {"code": 400, "description": %q}}`, tc.want), string(resp.Body()))

			row := harness.RowAsStrings(ctx, t, pool, "product", productID)
			require.Equal(t, map[string]string{
				"id":            productID,
				"name":          "test product",
				"description":   "<NULL>",
				"category":      "electronics",
				"keywords":      "{}",
				"price":         "<NULL>",
				"link":          "<NULL>",
				"hypothesis_id": hypothesisID,
			}, row)
		})
	}
}

func (s *StatefulSuite) TestCreateProductNameRequired() {
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

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]string{
			"category":      "electronics",
			"hypothesis_id": hypothesisID,
		}).
		Post("/api/v1/product")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusBadRequest, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 400, "description": "name is required"}}`, string(resp.Body()))
}

func (s *StatefulSuite) TestCreateProductCategoryRequired() {
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

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]string{
			"name":          "test product",
			"hypothesis_id": hypothesisID,
		}).
		Post("/api/v1/product")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusBadRequest, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 400, "description": "category is required"}}`, string(resp.Body()))
}

func (s *StatefulSuite) TestCreateProductInvalidCategory() {
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

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]string{
			"name":          "test product",
			"category":      "invalid",
			"hypothesis_id": hypothesisID,
		}).
		Post("/api/v1/product")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusBadRequest, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 400, "description": "category must be one of electronics, clothing, food, furniture, beauty, sports, toys, books, automotive, health, home, office, garden, pets, music, other"}}`, string(resp.Body()))
}

func (s *StatefulSuite) TestCreateProductHypothesisNotFound() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]string{
			"name":          "test product",
			"category":      "electronics",
			"hypothesis_id": "00000000-0000-0000-0000-000000000000",
		}).
		Post("/api/v1/product")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "hypothesis not found"}}`, string(resp.Body()))
}

func (s *StatefulSuite) TestUpdateProductNotFound() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]string{"name": "updated"}).
		Patch("/api/v1/product/00000000-0000-0000-0000-000000000000")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "product not found"}}`, string(resp.Body()))
}

func (s *StatefulSuite) TestDeleteProductNotFound() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	resp, err := env.Client.R().
		SetAuthToken(token).
		Delete("/api/v1/product/00000000-0000-0000-0000-000000000000")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "product not found"}}`, string(resp.Body()))
}

func (s *StatefulSuite) TestUpdateProductInvalidCategory() {
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
		SetBody(map[string]string{"category": "invalid"}).
		Patch("/api/v1/product/" + productID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusBadRequest, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 400, "description": "category must be one of electronics, clothing, food, furniture, beauty, sports, toys, books, automotive, health, home, office, garden, pets, music, other"}}`, string(resp.Body()))

	row := harness.RowAsStrings(ctx, s.T(), pool, "product", productID)
	require.Equal(s.T(), map[string]string{
		"id":            productID,
		"name":          "test product",
		"description":   "<NULL>",
		"category":      "electronics",
		"keywords":      "{}",
		"price":         "<NULL>",
		"link":          "<NULL>",
		"hypothesis_id": hypothesisID,
	}, row)
}
