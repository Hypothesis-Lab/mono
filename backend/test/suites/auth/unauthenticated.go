package auth

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"app/test/harness"

	"github.com/stretchr/testify/require"
)

var protectedRoutes = []struct {
	method string
	path   string
}{
	{"GET", "/api/v1/hypothesis"},
	{"POST", "/api/v1/hypothesis"},
	{"GET", "/api/v1/hypothesis/{id}"},
	{"PATCH", "/api/v1/hypothesis/{id}"},
	{"DELETE", "/api/v1/hypothesis/{id}"},
	{"POST", "/api/v1/hypothesis/{id}/launch"},
	{"POST", "/api/v1/hypothesis/{id}/stop"},
	{"GET", "/api/v1/hypothesis/{id}/product"},

	{"POST", "/api/v1/product"},
	{"PATCH", "/api/v1/product/{id}"},
	{"DELETE", "/api/v1/product/{id}"},
	{"GET", "/api/v1/hypothesis/{id}/files"},
	{"POST", "/api/v1/hypothesis/{id}/files"},

	{"GET", "/api/v1/product/{id}/audiences"},
	{"POST", "/api/v1/product/{id}/audiences"},
	{"GET", "/api/v1/audiences/{id}"},
	{"PATCH", "/api/v1/audiences/{id}"},
	{"DELETE", "/api/v1/audiences/{id}"},

	{"DELETE", "/api/v1/files/{id}"},
}

func (s *StatelessSuite) TestEndpointsRequireAuth() {
	env := harness.CurrentEnv()
	t := s.T()
	t.Parallel()

	for _, route := range protectedRoutes {
		t.Run(fmt.Sprintf("%s_%s", route.method, route.path), func(t *testing.T) {
			t.Parallel()

			path := strings.ReplaceAll(route.path, "{id}", "00000000-0000-0000-0000-000000000000")

			resp, err := env.Client.R().Execute(route.method, path)
			require.NoError(t, err)
			require.Equal(t, http.StatusUnauthorized, resp.StatusCode())
		})
	}
}
