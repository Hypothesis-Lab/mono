package handlers

import (
	"context"

	"app/internal/api"
)

func (s *Server) Healthz(_ context.Context, _ api.HealthzRequestObject) (api.HealthzResponseObject, error) {
	return api.Healthz200Response{}, nil
}