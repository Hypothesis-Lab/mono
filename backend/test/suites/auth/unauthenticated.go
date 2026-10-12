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
	{"GET", "/api/v1/hypothesis/{id}/launch-status"},
	{"POST", "/api/v1/hypothesis/{id}/stop"},
	{"GET", "/api/v1/hypothesis/{id}/product"},

	{"POST", "/api/v1/product"},
	{"PATCH", "/api/v1/product/{id}"},
	{"DELETE", "/api/v1/product/{id}"},
	{"GET", "/api/v1/product/{id}/audience-generation"},
	{"POST", "/api/v1/product/{id}/audience-generation"},
	{"POST", "/api/v1/product/{id}/audience-generation/retry"},
	{"GET", "/api/v1/hypothesis/{id}/files"},
	{"POST", "/api/v1/hypothesis/{id}/files"},

	{"DELETE", "/api/v1/files/{id}"},

	{"GET", "/api/v1/yandex-direct/authorization-url"},
	{"GET", "/api/v1/yandex-direct/connection"},
	{"POST", "/api/v1/yandex-direct/connection"},
	{"DELETE", "/api/v1/yandex-direct/connection"},
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
