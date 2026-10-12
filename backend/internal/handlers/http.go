package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"app/internal/api"

	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"
)

func errBody(code int, description string) api.ErrBody {
	return api.ErrBody{Error: api.ErrDetail{Code: code, Description: description}}
}

func errBodyWithCode(code int, description, errorCode string) api.ErrBody {
	return api.ErrBody{Error: api.ErrDetail{Code: code, Description: description, ErrorCode: &errorCode}}
}

func strictOptions(logger *zap.Logger) api.StrictHTTPServerOptions {
	return api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			writeErr(w, logger, http.StatusBadRequest, decodeMessage(err))
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			logger.Error("unexpected handler error", zap.Error(err))
			writeErr(w, logger, http.StatusInternalServerError, "internal server error")
		},
	}
}

// decodeMessage maps request body decode failures to user-facing messages.
// The spec types (e.g. openapi_types.Email) reject invalid values at decode
// time, before any handler runs; this gives those failures concrete messages.
func decodeMessage(err error) string {
	var syntaxErr *json.SyntaxError

	switch {
	case errors.Is(err, openapi_types.ErrValidationEmail):
		return "email must be a valid email address"
	case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
		return "malformed request body"
	default:
		return "invalid request body"
	}
}

func writeErr(w http.ResponseWriter, logger *zap.Logger, status int, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(errBody(status, description)); err != nil {
		logger.Error("encode error response", zap.Int("status", status), zap.Error(err))
	}
}
