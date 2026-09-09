package harness

import (
	"encoding/json"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func RegisterUser(t *testing.T, env *Env, email, password string) string {
	t.Helper()

	resp, err := env.Client.R().
		SetBody(map[string]string{"email": email, "password": password}).
		Post("/api/v1/auth/register")
	require.NoError(t, err)
	require.Equal(t, 201, resp.StatusCode())

	var result struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(resp.Body(), &result))
	require.NotEmpty(t, result.Token)

	return result.Token
}

func UserIDFromToken(t *testing.T, token string) string {
	t.Helper()

	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(_ *jwt.Token) (any, error) {
		return []byte(testJWTSecret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	require.NoError(t, err)

	return claims.Subject
}