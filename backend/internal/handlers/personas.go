package handlers

import (
	"context"
	"errors"
	"net/http"

	"app/internal/api"
	"app/internal/repositories/persona1m"

	"go.uber.org/zap"
)

func (s *Server) QueryPersonas(ctx context.Context, request api.QueryPersonasRequestObject) (api.QueryPersonasResponseObject, error) {
	if request.Body == nil {
		s.logger.Error("query personas: missing request body")

		return api.QueryPersonas400JSONResponse(errBody(http.StatusBadRequest, "request body is required")), nil
	}

	query := persona1m.Query{
		SampleSize: request.Body.SampleSize,
		Seed:       42,
	}
	if request.Body.Seed != nil {
		query.Seed = *request.Body.Seed
	}

	if request.Body.Sources != nil {
		query.Sources = *request.Body.Sources
	}

	if request.Body.DimensionFilters != nil {
		query.DimensionFilters = *request.Body.DimensionFilters
	}

	result, err := s.personas1m.Query(ctx, query)
	if err != nil {
		switch {
		case errors.Is(err, persona1m.ErrInvalidQuery):
			s.logger.Error("query personas: invalid query", zap.Error(err))

			return api.QueryPersonas400JSONResponse(errBody(http.StatusBadRequest, "invalid persona query")), nil
		case errors.Is(err, persona1m.ErrUnavailable):
			s.logger.Error("query personas: dataset unavailable", zap.Error(err))

			return api.QueryPersonas503JSONResponse(errBody(http.StatusServiceUnavailable, "persona dataset unavailable")), nil
		default:
			s.logger.Error("query personas", zap.Error(err))

			return api.QueryPersonas500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
		}
	}

	personas := make([]api.PersonaQueryPersona, 0, len(result.Personas))
	for _, persona := range result.Personas {
		personas = append(personas, api.PersonaQueryPersona{
			Id:             persona.ID,
			Source:         persona.Source,
			SourceRowIndex: persona.SourceRowIndex,
			Dimensions:     persona.Dimensions,
		})
	}

	return api.QueryPersonas200JSONResponse(api.PersonaQueryResponse{
		MatchedCount: result.MatchedCount,
		Personas:     personas,
	}), nil
}
