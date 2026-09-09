package file

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"time"

	"app/test/harness"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

var pdfContent = []byte("%PDF-1.4\n1 0 obj\n<<\n/Type /Catalog\n>>\nendobj\n")

func (s *StatefulSuite) TestUploadFile() {
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
		hypothesisID, userID, "File test hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	body, contentType := buildUploadRequest("test.pdf", "application/pdf", pdfContent, "other")

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetHeader("Content-Type", contentType).
		SetBody(body.Bytes()).
		Post("/api/v1/hypothesis/" + hypothesisID + "/files")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusCreated, resp.StatusCode())

	var uploaded struct {
		ID        string `json:"id"`
		CreatedAt string `json:"created_at"`
	}
	require.NoError(s.T(), json.Unmarshal(resp.Body(), &uploaded))
	require.NotEmpty(s.T(), uploaded.ID)
	require.JSONEq(s.T(), fmt.Sprintf(`{
		"id": %q,
		"hypothesis_id": %q,
		"type": "other",
		"filename": "test.pdf",
		"content_type": "application/pdf",
		"size": %d,
		"created_at": %q
	}`, uploaded.ID, hypothesisID, len(pdfContent), uploaded.CreatedAt), resp.String())

	row := harness.RowAsStrings(ctx, s.T(), pool, "file", uploaded.ID)
	require.Equal(s.T(), map[string]string{
		"id":           uploaded.ID,
		"hypothesis_id": hypothesisID,
		"type":         "other",
		"filename":     "test.pdf",
		"content_type": "application/pdf",
		"size":         strconv.Itoa(len(pdfContent)),
		"s3_key":       row["s3_key"],
		"created_at":   uploaded.CreatedAt,
	}, row)

	object, err := env.S3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(env.S3Bucket()),
		Key:    aws.String(row["s3_key"]),
	})
	require.NoError(s.T(), err)

	objectBody, err := io.ReadAll(object.Body)
	require.NoError(s.T(), err)
	require.NoError(s.T(), object.Body.Close())
	require.Equal(s.T(), pdfContent, objectBody)
	require.Equal(s.T(), "application/pdf", aws.ToString(object.ContentType))
}

func (s *StatefulSuite) TestListFiles() {
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
		hypothesisID, userID, "File test hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	createdAt1 := time.Now().UTC().Truncate(time.Microsecond)
	fileID1 := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO file (id, hypothesis_id, type, filename, content_type, size, s3_key, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		fileID1, hypothesisID, "other", "test.pdf", "application/pdf", 11, "files/"+fileID1, createdAt1,
	)
	require.NoError(s.T(), err)

	createdAt2 := createdAt1.Add(time.Second)
	fileID2 := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO file (id, hypothesis_id, type, filename, content_type, size, s3_key, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		fileID2, hypothesisID, "brandbook", "brand.pdf", "application/pdf", 256, "files/"+fileID2, createdAt2,
	)
	require.NoError(s.T(), err)

	resp, err := env.Client.R().
		SetAuthToken(token).
		Get("/api/v1/hypothesis/" + hypothesisID + "/files")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusOK, resp.StatusCode())
	require.JSONEq(s.T(), fmt.Sprintf(`[{
		"id": %q,
		"hypothesis_id": %q,
		"type": "other",
		"filename": "test.pdf",
		"content_type": "application/pdf",
		"size": 11,
		"created_at": %q
	},{
		"id": %q,
		"hypothesis_id": %q,
		"type": "brandbook",
		"filename": "brand.pdf",
		"content_type": "application/pdf",
		"size": 256,
		"created_at": %q
	}]`, fileID1, hypothesisID, createdAt1.Format(time.RFC3339Nano),
		fileID2, hypothesisID, createdAt2.Format(time.RFC3339Nano)), resp.String())
}

func (s *StatefulSuite) TestDeleteFile() {
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
		hypothesisID, userID, "File delete hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	body, contentType := buildUploadRequest("delete.pdf", "application/pdf", pdfContent, "other")

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetHeader("Content-Type", contentType).
		SetBody(body.Bytes()).
		Post("/api/v1/hypothesis/" + hypothesisID + "/files")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusCreated, resp.StatusCode())

	var uploaded struct {
		ID string `json:"id"`
	}
	require.NoError(s.T(), json.Unmarshal(resp.Body(), &uploaded))

	var s3Key string
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT s3_key FROM file WHERE id = $1", uploaded.ID).Scan(&s3Key))

	resp, err = env.Client.R().
		SetAuthToken(token).
		Delete("/api/v1/files/" + uploaded.ID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusNoContent, resp.StatusCode())

	_, err = env.S3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(env.S3Bucket()),
		Key:    aws.String(s3Key),
	})
	var noSuchKey *types.NoSuchKey
	require.ErrorAs(s.T(), err, &noSuchKey)
}

func (s *StatefulSuite) TestProductFileTooLarge() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	var userID string
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "user@test.com").Scan(&userID))

	hypothesisID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, userID, "File too large hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(s.T(), writer.SetBoundary("test-boundary"))

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename="big.pdf"`)
	header.Set("Content-Type", "application/pdf")

	part, err := writer.CreatePart(header)
	require.NoError(s.T(), err)

	_, err = part.Write(bytes.Repeat([]byte("a"), 50*1024*1024+1))
	require.NoError(s.T(), err)

	require.NoError(s.T(), writer.WriteField("type", "other"))
	require.NoError(s.T(), writer.Close())

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetHeader("Content-Type", writer.FormDataContentType()).
		SetBody(body.Bytes()).
		Post("/api/v1/hypothesis/" + hypothesisID + "/files")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusRequestEntityTooLarge, resp.StatusCode())
	require.JSONEq(s.T(), `{
		"error": {
			"code": 413,
			"description": "file exceeds 50 MB limit"
		}
	}`, resp.String())
}

func (s *StatefulSuite) TestBrandbookConflict() {
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
		hypothesisID, userID, "Brandbook hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	body, contentType := buildUploadRequest("brandbook.pdf", "application/pdf", pdfContent, "brandbook")

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetHeader("Content-Type", contentType).
		SetBody(body.Bytes()).
		Post("/api/v1/hypothesis/" + hypothesisID + "/files")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusCreated, resp.StatusCode())

	body, contentType = buildUploadRequest("brandbook2.pdf", "application/pdf", pdfContent, "brandbook")

	resp, err = env.Client.R().
		SetAuthToken(token).
		SetHeader("Content-Type", contentType).
		SetBody(body.Bytes()).
		Post("/api/v1/hypothesis/" + hypothesisID + "/files")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusConflict, resp.StatusCode())
	require.JSONEq(s.T(), `{
		"error": {
			"code": 409,
			"description": "brandbook already exists for this hypothesis"
		}
	}`, resp.String())
}

func (s *StatefulSuite) TestUploadProductFileInvalidType() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, env.DatabaseURL())
	require.NoError(s.T(), err)
	defer pool.Close()

	userID := harness.UserIDFromToken(s.T(), token)

	hypothesisID := uuid.NewString()
	_, err = pool.Exec(ctx,
		"INSERT INTO hypothesis (id, user_id, description, status) VALUES ($1, $2, $3, $4)",
		hypothesisID, userID, "invalid type hypothesis", "idle",
	)
	require.NoError(s.T(), err)

	body, contentType := buildUploadRequest("test.pdf", "application/pdf", pdfContent, "invalidtype")

	resp, err := env.Client.R().
		SetAuthToken(token).
		SetHeader("Content-Type", contentType).
		SetBody(body.Bytes()).
		Post("/api/v1/hypothesis/" + hypothesisID + "/files")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusBadRequest, resp.StatusCode())
	require.JSONEq(s.T(), `{"error": {"code": 400, "description": "type must be one of product, brandbook, reference, other"}}`, resp.String())

	var count int
	require.NoError(s.T(), pool.QueryRow(ctx, "SELECT count(*) FROM file WHERE hypothesis_id = $1", hypothesisID).Scan(&count))
	require.Equal(s.T(), 0, count)
}

func buildUploadRequest(filename, contentType string, content []byte, fileType string) (*bytes.Buffer, string) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.SetBoundary("test-boundary"); err != nil {
		panic(err)
	}

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, filename))
	header.Set("Content-Type", contentType)

	part, err := writer.CreatePart(header)
	if err != nil {
		panic(err)
	}

	_, err = part.Write(content)
	if err != nil {
		panic(err)
	}

	_ = writer.WriteField("type", fileType)
	_ = writer.Close()

	return &body, writer.FormDataContentType()
}
