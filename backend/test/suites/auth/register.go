package auth

import (
	"encoding/json"
	"net/http"

	"app/test/harness"

	"github.com/stretchr/testify/require"
)

// Register endpoint test cases
//
// Optional cross-cutting check
//   - Token works on a protected route: after registering, use the returned token
//     to call GET /api/v1/hypothesis and expect 200 (or at least not 401).

func (s *StatefulSuite) TestRegisterValid() {
	env := harness.CurrentEnv()

	resp, err := env.Client.R().
		SetBody(map[string]string{
			"email":    "user@test.com",
			"password": "password123",
		}).
		Post("/api/v1/auth/register")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusCreated, resp.StatusCode())

	var body map[string]any
	require.NoError(s.T(), json.Unmarshal(resp.Body(), &body))

	token, ok := body["token"].(string)
	require.True(s.T(), ok)
	require.NotEmpty(s.T(), token)

	delete(body, "token")
	require.Empty(s.T(), body)
}

func (s *StatefulSuite) TestRegisterValidation() {
	env := harness.CurrentEnv()

	tests := []struct {
		name         string
		req          string
		expectedResp string
	}{
		{
			name:         "missing body",
			req:          "",
			expectedResp: `{"error": {"code": 400, "description": "request body is required"}}`,
		},
		{
			name:         "missing email",
			req:          `{"password": "password123"}`,
			expectedResp: `{"error": {"code": 400, "description": "email is required"}}`,
		},
		{
			name:         "missing password",
			req:          `{"email": "user@test.com"}`,
			expectedResp: `{"error": {"code": 400, "description": "password is required"}}`,
		},
		{
			name:         "email without @",
			req:          `{"email": "not-an-email", "password": "password123"}`,
			expectedResp: `{"error": {"code": 400, "description": "email must be a valid email address"}}`,
		},
		{
			name:         "invalid email user@",
			req:          `{"email": "user@", "password": "password123"}`,
			expectedResp: `{"error": {"code": 400, "description": "email must be a valid email address"}}`,
		},
		{
			name:         "invalid email user@.com",
			req:          `{"email": "user@.com", "password": "password123"}`,
			expectedResp: `{"error": {"code": 400, "description": "email must be a valid email address"}}`,
		},
		{
			name:         "password too short",
			req:          `{"email": "user@test.com", "password": "12345"}`,
			expectedResp: `{"error": {"code": 400, "description": "password must be at least 6 characters long"}}`,
		},
		{
			name:         "empty password",
			req:          `{"email": "user@test.com", "password": ""}`,
			expectedResp: `{"error": {"code": 400, "description": "password must be at least 6 characters long"}}`,
		},
		{
			name:         "empty email",
			req:          `{"email": "", "password": "password123"}`,
			expectedResp: `{"error": {"code": 400, "description": "email must be a valid email address"}}`,
		},
		{
			name:         "malformed JSON",
			req:          `{"email": "user@test.com", "pass`,
			expectedResp: `{"error": {"code": 400, "description": "malformed request body"}}`,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			r := env.Client.R().
				SetHeader("Content-Type", "application/json")
			if tt.req != "" {
				r.SetBody(tt.req)
			}

			resp, err := r.Post("/api/v1/auth/register")
			require.NoError(s.T(), err)
			require.Equal(s.T(), http.StatusBadRequest, resp.StatusCode())

			require.JSONEq(s.T(), tt.expectedResp, string(resp.Body()))
		})
	}
}

func (s *StatefulSuite) TestRegisterWrongContentType() {
	env := harness.CurrentEnv()

	resp, err := env.Client.R().
		SetHeader("Content-Type", "text/plain").
		SetBody(`{"email": "user@test.com", "password": "password123"}`).
		Post("/api/v1/auth/register")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusBadRequest, resp.StatusCode())

	require.JSONEq(s.T(), `{"error": {"code": 400, "description": "unsupported content type"}}`, string(resp.Body()))
}

func (s *StatefulSuite) TestRegisterDuplicateEmail() {
	env := harness.CurrentEnv()

	body := map[string]string{
		"email":    "user@test.com",
		"password": "password123",
	}

	resp, err := env.Client.R().
		SetBody(body).
		Post("/api/v1/auth/register")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusCreated, resp.StatusCode())

	resp, err = env.Client.R().
		SetBody(body).
		Post("/api/v1/auth/register")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusConflict, resp.StatusCode())

	require.JSONEq(s.T(), `{"error": {"code": 409, "description": "email already registered"}}`, string(resp.Body()))
}

func (s *StatefulSuite) TestRegisterTokenWorks() {
	env := harness.CurrentEnv()

	token := harness.RegisterUser(s.T(), env, "user@test.com", "password123")

	resp, err := env.Client.R().
		SetAuthToken(token).
		Get("/api/v1/hypothesis")
	require.NoError(s.T(), err)
	require.Equal(s.T(), http.StatusOK, resp.StatusCode())

	require.JSONEq(s.T(), `[]`, string(resp.Body()))
}
