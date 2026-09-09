package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"app/internal/api"
	"app/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// testing that this https://github.com/go-chi/chi/issues/843 does not happen in current router
func TestUnknownRoute_Returns404(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}

	router, err := NewRouter(api.Unimplemented{}, cfg, zap.NewNop())
	require.NoError(t, err)

	srv := httptest.NewServer(router)
	defer srv.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(srv.URL + "/definitely-not-a-route-abc123")
	require.NoError(t, err, "request to unknown route should not hang or time out")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
