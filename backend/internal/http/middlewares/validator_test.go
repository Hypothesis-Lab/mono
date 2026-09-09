package middlewares

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"app/internal/api"

	"github.com/getkin/kin-openapi/openapi3filter"
	middleware "github.com/oapi-codegen/nethttp-middleware"
	"github.com/stretchr/testify/require"
)

// TestValidationMessage exercises validationMessage against the embedded
// spec across every constraint branch it handles. It guards the
// array-item field derivation (the pointer for a violating array item
// is ["keywords","0"], where the trailing segment is a numeric index)
// and catches drift if spec constraints or the message switch change.
func TestValidationMessage(t *testing.T) {
	spec, err := api.GetSpec()
	require.NoError(t, err)

	validator := middleware.OapiRequestValidatorWithOptions(spec, &middleware.Options{
		Options: openapi3filter.Options{
			AuthenticationFunc: func(context.Context, *openapi3filter.AuthenticationInput) error {
				return nil
			},
		},
		ErrorHandlerWithOpts: func(_ context.Context, err error, w http.ResponseWriter, _ *http.Request, opts middleware.ErrorHandlerOpts) {
			status := opts.StatusCode
			if status == 0 {
				status = http.StatusBadRequest
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = io.WriteString(w, validationMessage(err, status))
		},
	})

	mux := http.NewServeMux()
	mux.Handle("/", validator(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	const productReq = `{"name":"p","category":"electronics","hypothesis_id":"00000000-0000-0000-0000-000000000000"`
	const audReq = `{"description":"a"`

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		want   string
	}{
		{"array item maxLength yields parent field, not index", "POST", "/api/v1/product", productReq + `,"keywords":["` + strings.Repeat("k", 65) + `"]}`, "keywords must be at most 64 characters long"},
		{"string maxLength", "POST", "/api/v1/product", productReq + `,"link":"` + strings.Repeat("l", 129) + `"}`, "link must be at most 128 characters long"},
		{"nullable string maxLength", "POST", "/api/v1/product", productReq + `,"price":"` + strings.Repeat("p", 129) + `"}`, "price must be at most 128 characters long"},
		{"hypothesis description maxLength", "POST", "/api/v1/hypothesis", `{"description":"` + strings.Repeat("d", 1025) + `"}`, "description must be at most 1024 characters long"},
		{"integer maximum", "POST", "/api/v1/product/00000000-0000-0000-0000-000000000000/audiences", audReq + `,"age_from":150}`, "age_from must be less than or equal to 100"},
		{"integer minimum", "POST", "/api/v1/product/00000000-0000-0000-0000-000000000000/audiences", audReq + `,"age_to":-1}`, "age_to must be greater than or equal to 0"},
		{"audience interests maxLength", "POST", "/api/v1/product/00000000-0000-0000-0000-000000000000/audiences", audReq + `,"interests":"` + strings.Repeat("x", 1001) + `"}`, "interests must be at most 1000 characters long"},
		{"required field", "POST", "/api/v1/hypothesis", `{}`, "description is required"},
		{"enum field", "POST", "/api/v1/product", `{"name":"p","category":"nope","hypothesis_id":"00000000-0000-0000-0000-000000000000"}`, "category must be one of electronics, clothing, food, furniture, beauty, sports, toys, books, automotive, health, home, office, garden, pets, music, other"},
		{"malformed body", "POST", "/api/v1/hypothesis", `{not json`, "malformed request body"},
		{"missing body", "POST", "/api/v1/hypothesis", ``, "request body is required"},
		{"wrong type", "POST", "/api/v1/hypothesis", `{"description":123}`, "description has invalid type"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, srv.URL+tc.path, strings.NewReader(tc.body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer x")

			httpResp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer httpResp.Body.Close()
			body, _ := io.ReadAll(httpResp.Body)

			require.Equal(t, http.StatusBadRequest, httpResp.StatusCode, "body: %s", string(body))
			require.Equal(t, tc.want, string(body))
		})
	}
}