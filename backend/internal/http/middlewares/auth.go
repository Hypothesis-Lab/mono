package middlewares

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"app/internal/api"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/golang-jwt/jwt/v5"
	middleware "github.com/oapi-codegen/nethttp-middleware"
	"go.uber.org/zap"
)

const authorizationScheme = "bearerAuth"

type userIDContextKey struct{}

func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDContextKey{}, userID)
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDContextKey{}).(string)
	return userID, ok
}

// AuthMiddleware returns a middleware that enforces the security
// requirements declared in the OpenAPI spec (global bearerAuth, with
// `security: []` opting out per-endpoint).
func AuthMiddleware(logger *zap.Logger) (func(http.Handler) http.Handler, error) {
	spec, err := api.GetSpec()
	if err != nil {
		return nil, fmt.Errorf("load embedded spec: %w", err)
	}

	validator := middleware.OapiRequestValidatorWithOptions(spec, &middleware.Options{
		Options: openapi3filter.Options{
			AuthenticationFunc: func(ctx context.Context, input *openapi3filter.AuthenticationInput) error {
				return authenticate(ctx, input, logger)
			},
		},
		ErrorHandlerWithOpts: func(ctx context.Context, err error, w http.ResponseWriter, r *http.Request, opts middleware.ErrorHandlerOpts) {
			authErrorHandler(ctx, err, w, r, opts, logger)
		},
	})

	return validator, nil
}

func UserContextMiddleware(secret string, logger *zap.Logger) func(http.Handler) http.Handler {
	secretBytes := []byte(secret)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r.Header.Get("Authorization"))
			if !ok {
				logger.Error("missing or invalid bearer token")

				next.ServeHTTP(w, r)
				return
			}

			userID, err := validateToken(token, secretBytes)
			if err != nil {
				logger.Error("validate token", zap.Error(err))

				next.ServeHTTP(w, r)
				return
			}

			next.ServeHTTP(w, r.WithContext(ContextWithUserID(r.Context(), userID)))
		})
	}
}

// authenticate is the openapi3filter.AuthenticationFunc that authorizes
// requests whose security requirement references bearerAuth. It returns
// an error when the request context carries no authenticated user ID.
func authenticate(ctx context.Context, input *openapi3filter.AuthenticationInput, logger *zap.Logger) error {
	if input.SecuritySchemeName != authorizationScheme {
		logger.Error("unexpected security scheme", zap.String("scheme", input.SecuritySchemeName))

		return fmt.Errorf("security scheme %q != %q", input.SecuritySchemeName, authorizationScheme)
	}

	if _, ok := UserIDFromContext(ctx); !ok {
		logger.Error("authenticate: missing or invalid authorization")

		return errors.New("missing or invalid authorization")
	}

	return nil
}

func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}

	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return "", false
	}

	return token, true
}

func validateToken(tokenString string, secret []byte) (string, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&jwt.RegisteredClaims{},
		func(_ *jwt.Token) (any, error) { return secret, nil },
		jwt.WithValidMethods([]string{"HS256"}),
	)
	if err != nil {
		return "", fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid {
		return "", errors.New("invalid token")
	}

	return claims.Subject, nil
}

// authErrorHandler renders a validation failure (auth, malformed route,
// or bad request body) into the API's standard ErrBody JSON envelope,
// using opts.StatusCode as the response status (defaulting to 400).
func authErrorHandler(_ context.Context, err error, w http.ResponseWriter, _ *http.Request, opts middleware.ErrorHandlerOpts, logger *zap.Logger) {
	logger.Error("auth handler error", zap.Error(err), zap.Int("status", opts.StatusCode))

	w.Header().Set("Content-Type", "application/json")

	status := opts.StatusCode
	if status == 0 {
		status = http.StatusBadRequest
	}
	w.WriteHeader(status)

	if encodeErr := json.NewEncoder(w).Encode(api.ErrBody{Error: api.ErrDetail{Code: status, Description: validationMessage(err, status)}}); encodeErr != nil {
		logger.Error("encode auth error response", zap.Error(encodeErr))
	}
}
