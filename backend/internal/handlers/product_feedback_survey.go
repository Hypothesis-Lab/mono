package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"app/internal/api"

	"go.uber.org/zap"
)

func (s *Server) RunProductFeedbackSurvey(ctx context.Context, _ api.RunProductFeedbackSurveyRequestObject) (api.RunProductFeedbackSurveyResponseObject, error) {
	count, err := s.productFeedbackSurvey.Run(ctx)
	if err != nil {
		s.logger.Error("run product feedback survey", zap.Error(err))

		return api.RunProductFeedbackSurvey500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.RunProductFeedbackSurvey200JSONResponse(api.ProductFeedbackSurveyRunResponse{Count: count}), nil
}

func (s *Server) ListProductFeedbackSurveyResults(ctx context.Context, _ api.ListProductFeedbackSurveyResultsRequestObject) (api.ListProductFeedbackSurveyResultsResponseObject, error) {
	results, err := s.productFeedbackSurvey.List(ctx)
	if err != nil {
		s.logger.Error("list product feedback survey results", zap.Error(err))

		return api.ListProductFeedbackSurveyResults500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	response := make([]api.ProductFeedbackSurveyResultResponse, 0, len(results))
	for _, result := range results {
		var decodedResponse any
		if err := json.Unmarshal(result.Response, &decodedResponse); err != nil {
			s.logger.Error("decode product feedback survey result", zap.String("persona_id", result.PersonaID), zap.Error(err))

			return api.ListProductFeedbackSurveyResults500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
		}

		response = append(response, api.ProductFeedbackSurveyResultResponse{
			PersonaId:   result.PersonaID,
			PersonaName: result.PersonaName,
			Response:    decodedResponse,
		})
	}

	return api.ListProductFeedbackSurveyResults200JSONResponse(response), nil
}
