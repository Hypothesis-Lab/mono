package http

import (
	"net/http"
	"testing"

	"app/test/harness"

	"github.com/stretchr/testify/require"
)

func (s *StatelessSuite) TestNonExistentRouteReturns404() {
	env := harness.CurrentEnv()
	t := s.T()
	t.Parallel()

	routes := []string{
		"/",
		"/non-existent",
		"/api/non-existent",
		"/api/v1/non-existent",
	}

	for _, route := range routes {
		t.Run(route, func(t *testing.T) {
			t.Parallel()

			resp, err := env.Client.R().Get(route)
			require.NoError(t, err)
			require.Equal(t, http.StatusNotFound, resp.StatusCode())

			require.JSONEq(t, `{"error": {"code": 404, "description": "Not Found"}}`, string(resp.Body()))
		})
	}
}
