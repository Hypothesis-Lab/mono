package hypothesis

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

func (s *StatefulSuite) TestCreateHypothesis() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]string{"description": "test hypothesis"}).
		Post("/api/v1/hypothesis")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusCreated, resp.StatusCode())

	var created struct {
		ID        string `json:"id"`
		CreatedAt string `json:"created_at"`
	}
	require.NoError(s.T(), json.Unmarshal(resp.Body(), &created))
	require.NotEmpty(s.T(), created.ID)
	require.NotEmpty(s.T(), created.CreatedAt)

	require.JSONEq(s.T(), fmt.Sprintf(`{
		"id": %q,
		"description": "test hypothesis",
		"status": "idle",
		"created_at": %q
	}`, created.ID, created.CreatedAt), string(resp.Body()))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	var userID string
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

	got := harness.RowAsStrings(ctx, s.T(), pool, "hypothesis", created.ID)
	require.Equal(s.T(), map[string]string{
		"id":          created.ID,
		"user_id":     userID,
		"description": "test hypothesis",
		"status":      "idle",
		"created_at":  created.CreatedAt,
	}, got)
}

func (s *StatefulSuite) TestCreateHypothesisDescriptionRequired() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]string{"description": ""}).
		Post("/api/v1/hypothesis")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusBadRequest, resp.StatusCode())

	require.JSONEq(s.T(), `{"error": {"code": 400, "description": "description is required"}}`, string(resp.Body()))
}

func (s *StatefulSuite) TestGetHypothesis() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	var userID string
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

	id := uuid.NewString()
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status, created_at) VALUES ($1, $2, $3, $4, $5)",
		id, userID, "test hypothesis", "idle", createdAt,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		Get("/api/v1/hypothesis/" + id)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusOK, resp.StatusCode())

	var got struct {
		CreatedAt string `json:"created_at"`
	}
	require.NoError(s.T(), json.Unmarshal(resp.Body(), &got))

	require.JSONEq(s.T(), fmt.Sprintf(`{
		"id": %q,
		"description": "test hypothesis",
		"status": "idle",
		"created_at": %q
	}`, id, got.CreatedAt), string(resp.Body()))
}

func (s *StatefulSuite) TestGetHypothesisNotFound() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	resp, err := env.Client.R().
		SetAuthToken(token).
		Get("/api/v1/hypothesis/00000000-0000-0000-0000-000000000000")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())

	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "hypothesis not found"}}`, string(resp.Body()))
}

func (s *StatefulSuite) TestGetHypothesisBadUUID() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	resp, err := env.Client.R().
		SetAuthToken(token).
		Get("/api/v1/hypothesis/not-a-uuid")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNotFound, resp.StatusCode())

	require.JSONEq(s.T(), `{"error": {"code": 404, "description": "hypothesis not found"}}`, string(resp.Body()))
}

func (s *StatefulSuite) TestListHypotheses() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	var userID string
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

	id1 := uuid.NewString()
	id2 := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		id1, userID, "first hypothesis", "idle",
	)
	require.NoError(s.T(), err)
	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		id2, userID, "second hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	row1 := harness.RowAsStrings(ctx, s.T(), pool, "hypothesis", id1)
	row2 := harness.RowAsStrings(ctx, s.T(), pool, "hypothesis", id2)

	entries := []struct{ id, desc, status, created string }{
		{id: id1, desc: "first hypothesis", status: row1["status"], created: row1["created_at"]},
		{id: id2, desc: "second hypothesis", status: row2["status"], created: row2["created_at"]},
	}
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].id < entries[j-1].id; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}

	resp, err := env.Client.R().
		SetAuthToken(token).
		Get("/api/v1/hypothesis")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusOK, resp.StatusCode())

	require.JSONEq(s.T(), fmt.Sprintf(`[%s,%s]`,
		fmt.Sprintf(`{"id":%q,"description":%q,"status":%q,"created_at":%q}`, entries[0].id, entries[0].desc, entries[0].status, entries[0].created),
		fmt.Sprintf(`{"id":%q,"description":%q,"status":%q,"created_at":%q}`, entries[1].id, entries[1].desc, entries[1].status, entries[1].created),
	), resp.String())
}

func (s *StatefulSuite) TestUpdateHypothesis() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	var userID string
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

	id := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		id, userID, "original description", "idle",
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]string{"description": "updated description"}).
		Patch("/api/v1/hypothesis/" + id)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusOK, resp.StatusCode())

	var got struct {
		CreatedAt string `json:"created_at"`
	}
	require.NoError(s.T(), json.Unmarshal(resp.Body(), &got))

	require.JSONEq(s.T(), fmt.Sprintf(`{
		"id": %q,
		"description": "updated description",
		"status": "idle",
		"created_at": %q
	}`, id, got.CreatedAt), resp.String())

	row := harness.RowAsStrings(ctx, s.T(), pool, "hypothesis", id)
	require.Equal(s.T(), map[string]string{
		"id":          id,
		"user_id":     userID,
		"description": "updated description",
		"status":      "idle",
		"created_at":  got.CreatedAt,
	}, row)
}

func (s *StatefulSuite) TestDeleteHypothesis() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	var userID string
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

	id := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		id, userID, "to delete", "idle",
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
		fileID, id, "other", "test.pdf", "application/pdf", len(fileContent), s3Key,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		Delete("/api/v1/hypothesis/" + id)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNoContent, resp.StatusCode())
	require.Empty(s.T(), resp.Body())

	var count int
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT count(*) FROM hypothesis WHERE id = $1", id).Scan(&count))
	require.Equal(s.T(), 0, count)

	var fileCount int
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT count(*) FROM file WHERE id = $1", fileID).Scan(&fileCount))
	require.Equal(s.T(), 0, fileCount)

	_, err = env.S3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(env.S3Bucket()),
		Key:    aws.String(s3Key),
	})
	var noSuchKey *types.NoSuchKey
	require.ErrorAs(s.T(), err, &noSuchKey)
}

func (s *StatefulSuite) TestLaunchHypothesis() {
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
		hypothesisID, userID, "to launch", "idle",
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
		Post("/api/v1/hypothesis/" + hypothesisID + "/launch")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusOK, resp.StatusCode())

	var got struct {
		CreatedAt string `json:"created_at"`
	}
	require.NoError(s.T(), json.Unmarshal(resp.Body(), &got))

	require.JSONEq(s.T(), fmt.Sprintf(`{
		"id": %q,
		"description": "to launch",
		"status": "in_progress",
		"created_at": %q
	}`, hypothesisID, got.CreatedAt), resp.String())

	row := harness.RowAsStrings(ctx, s.T(), pool, "hypothesis", hypothesisID)
	require.Equal(s.T(), map[string]string{
		"id":          hypothesisID,
		"user_id":     userID,
		"description": "to launch",
		"status":      "in_progress",
		"created_at":  got.CreatedAt,
	}, row)
}

func (s *StatefulSuite) TestStopHypothesis() {
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
		hypothesisID, userID, "to stop", "in_progress",
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		Post("/api/v1/hypothesis/" + hypothesisID + "/stop")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusOK, resp.StatusCode())

	var got struct {
		CreatedAt string `json:"created_at"`
	}
	require.NoError(s.T(), json.Unmarshal(resp.Body(), &got))

	require.JSONEq(s.T(), fmt.Sprintf(`{
		"id": %q,
		"description": "to stop",
		"status": "idle",
		"created_at": %q
	}`, hypothesisID, got.CreatedAt), resp.String())

	row := harness.RowAsStrings(ctx, s.T(), pool, "hypothesis", hypothesisID)
	require.Equal(s.T(), map[string]string{
		"id":          hypothesisID,
		"user_id":     userID,
		"description": "to stop",
		"status":      "idle",
		"created_at":  got.CreatedAt,
	}, row)
}

func (s *StatefulSuite) TestGetHypothesisProduct() {
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
		hypothesisID, userID, "with product", "idle",
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
		Get("/api/v1/hypothesis/" + hypothesisID + "/product")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusOK, resp.StatusCode())

	require.JSONEq(s.T(), fmt.Sprintf(`{
		"id": %q,
		"name": "test product",
		"category": "electronics",
		"keywords": [],
		"hypothesis_id": %q
	}`, productID, hypothesisID), resp.String())
}

func (s *StatefulSuite) TestUpdateHypothesisInProgress() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	var userID string
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

	id := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		id, userID, "running hypothesis", "in_progress",
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetBody(map[string]string{"description": "hijacked"}).
		Patch("/api/v1/hypothesis/" + id)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusBadRequest, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 400, "description": "hypothesis in progress"}}`, resp.String())

	row := harness.RowAsStrings(ctx, s.T(), pool, "hypothesis", id)
	require.Equal(s.T(), map[string]string{
		"id":          id,
		"user_id":     userID,
		"description": "running hypothesis",
		"status":      "in_progress",
		"created_at":  row["created_at"],
	}, row)
}

func (s *StatefulSuite) TestDeleteHypothesisInProgress() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	var userID string
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

	id := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		id, userID, "running hypothesis", "in_progress",
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		Delete("/api/v1/hypothesis/" + id)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusBadRequest, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 400, "description": "hypothesis in progress"}}`, resp.String())

	row := harness.RowAsStrings(ctx, s.T(), pool, "hypothesis", id)
	require.Equal(s.T(), map[string]string{
		"id":          id,
		"user_id":     userID,
		"description": "running hypothesis",
		"status":      "in_progress",
		"created_at":  row["created_at"],
	}, row)
}

func (s *StatefulSuite) TestCreateHypothesisSpecConstraints() {
	env := harness.CurrentEnv()

	tooLong := strings.Repeat("x", 1025)

	s.Run("description exceeds maxLength", func() {
		t := s.T()
		token := harness.RegisterUser(t, env, "user@test.com", "password123")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		pool, err := pgxpool.New(ctx, env.DatabaseURL())
		require.NoError(t, err)
		defer pool.Close()

		resp, err := env.Client.R().
			SetAuthToken(token).
			SetBody(map[string]string{"description": tooLong}).
			Post("/api/v1/hypothesis")
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode())
		require.JSONEq(t, `{"error": {"code": 400, "description": "description must be at most 1024 characters long"}}`, string(resp.Body()))

		var count int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM hypothesis").Scan(&count))
		require.Equal(t, 0, count, "no hypothesis should be persisted when validation fails")
	})
}

func (s *StatefulSuite) TestUpdateHypothesisSpecConstraints() {
	env := harness.CurrentEnv()

	tooLong := strings.Repeat("x", 1025)

	s.Run("description exceeds maxLength", func() {
		t := s.T()
		token := harness.RegisterUser(t, env, "user@test.com", "password123")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		pool, err := pgxpool.New(ctx, env.DatabaseURL())
		require.NoError(t, err)
		defer pool.Close()

		var userID string
		require.NoError(t, pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

		id := uuid.NewString()
		_, err = pool.Exec(ctx,
			"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
			id, userID, "original description", "idle",
		)
		require.NoError(t, err)

		resp, err := env.Client.R().
			SetAuthToken(token).
			SetBody(map[string]string{"description": tooLong}).
			Patch("/api/v1/hypothesis/" + id)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode())
		require.JSONEq(t, `{"error": {"code": 400, "description": "description must be at most 1024 characters long"}}`, string(resp.Body()))

		row := harness.RowAsStrings(ctx, t, pool, "hypothesis", id)
		require.Equal(t, map[string]string{
			"id":          id,
			"user_id":     userID,
			"description": "original description",
			"status":      "idle",
			"created_at":  row["created_at"],
		}, row)
	})
}
