package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"app/internal/api"
	"app/internal/clients"

	"go.uber.org/zap"
)

func (s *Server) RunProductFeedbackSurvey(ctx context.Context, _ api.RunProductFeedbackSurveyRequestObject) (api.RunProductFeedbackSurveyResponseObject, error) {
	count, err := s.productFeedbackSurvey.Run(context.WithoutCancel(ctx))
	if err != nil {
		s.logger.Error("run product feedback survey", zap.Error(err))

		return api.RunProductFeedbackSurvey500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.RunProductFeedbackSurvey200JSONResponse(api.ProductFeedbackSurveyRunResponse{
		Count:         count,
		Questionnaire: productFeedbackSurveyQuestionnaire(s.productFeedbackSurvey.Questionnaire()),
	}), nil
}

func (s *Server) ListProductFeedbackSurveyResults(ctx context.Context, _ api.ListProductFeedbackSurveyResultsRequestObject) (api.ListProductFeedbackSurveyResultsResponseObject, error) {
	results, err := s.productFeedbackSurvey.List(ctx)
	if err != nil {
		s.logger.Error("list product feedback survey results", zap.Error(err))

		return api.ListProductFeedbackSurveyResults500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	response := make([]api.ProductFeedbackSurveyResultResponse, 0, len(results))
	for _, surveyResult := range results {
		result := surveyResult.Result
		var decodedResponse any
		if err := json.Unmarshal(result.Response, &decodedResponse); err != nil {
			s.logger.Error("decode product feedback survey result", zap.String("persona_id", result.PersonaID), zap.Error(err))

			return api.ListProductFeedbackSurveyResults500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
		}

		persona := api.ProductFeedbackSurveyPersona{
			Id:          result.PersonaID,
			DisplayName: result.PersonaName,
		}
		if surveyResult.Persona != nil {
			version := surveyResult.Persona.Version
			source := surveyResult.Persona.Source
			dimensions := surveyResult.Persona.Dimensions
			persona = api.ProductFeedbackSurveyPersona{
				Id:          surveyResult.Persona.ID,
				Version:     &version,
				Source:      &source,
				DisplayName: surveyResult.Persona.DisplayName,
				Dimensions:  &dimensions,
			}
		}

		response = append(response, api.ProductFeedbackSurveyResultResponse{
			PersonaId:   result.PersonaID,
			PersonaName: result.PersonaName,
			Persona:     persona,
			Response:    decodedResponse,
		})
	}

	return api.ListProductFeedbackSurveyResults200JSONResponse(api.ProductFeedbackSurveyResultsResponse{
		Questionnaire: productFeedbackSurveyQuestionnaire(s.productFeedbackSurvey.Questionnaire()),
		Results:       response,
	}), nil
}

func productFeedbackSurveyQuestionnaire(questions map[string]clients.JevDecisionQuestion) api.ProductFeedbackSurveyQuestionnaire {
	questionnaire := make(api.ProductFeedbackSurveyQuestionnaire, len(questions))
	for key, question := range questions {
		questionnaire[key] = api.ProductFeedbackSurveyQuestion{
			Type:         question.Type,
			Instructions: question.Instructions,
			Criteria:     question.Criteria,
		}
	}

	return questionnaire
}
