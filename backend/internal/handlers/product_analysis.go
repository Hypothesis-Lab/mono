package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"app/internal/api"
	"app/internal/http/middlewares"
	"app/internal/repositories/product"
	"app/internal/repositories/productanalysis"
	"app/internal/services"

	"go.uber.org/zap"
)

func (s *Server) StartProductAnalysis(ctx context.Context, req api.StartProductAnalysisRequestObject) (api.StartProductAnalysisResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id for product analysis", zap.String("product_id", req.Id))
		return api.StartProductAnalysis500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	run, err := s.productAnalysis.Start(ctx, userID, req.Id)
	if err != nil {
		s.logger.Error("start product analysis", zap.String("product_id", req.Id), zap.Error(err))
		switch {
		case errors.Is(err, product.ErrNotFound):
			return api.StartProductAnalysis404JSONResponse(errBody(http.StatusNotFound, "product not found")), nil
		case errors.Is(err, productanalysis.ErrAlreadyActive):
			return api.StartProductAnalysis409JSONResponse(errBody(http.StatusConflict, "product analysis is already running")), nil
		case errors.Is(err, productanalysis.ErrNotFound):
			return api.StartProductAnalysis409JSONResponse(errBody(http.StatusConflict, "no prepared audience dataset available")), nil
		default:
			return api.StartProductAnalysis500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
		}
	}

	response, err := productAnalysisResponse(run)
	if err != nil {
		s.logger.Error("build product analysis response", zap.String("product_id", req.Id), zap.Error(err))
		return api.StartProductAnalysis500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.StartProductAnalysis201JSONResponse(response), nil
}

func (s *Server) GetProductAnalysis(ctx context.Context, req api.GetProductAnalysisRequestObject) (api.GetProductAnalysisResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id for product analysis status", zap.String("product_id", req.Id))
		return api.GetProductAnalysis500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	run, err := s.productAnalysis.Latest(ctx, userID, req.Id)
	if err != nil {
		s.logger.Error("get product analysis", zap.String("product_id", req.Id), zap.Error(err))
		if errors.Is(err, product.ErrNotFound) || errors.Is(err, productanalysis.ErrNotFound) {
			return api.GetProductAnalysis404JSONResponse(errBody(http.StatusNotFound, "product or analysis not found")), nil
		}

		return api.GetProductAnalysis500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	response, err := productAnalysisResponse(run)
	if err != nil {
		s.logger.Error("build product analysis status", zap.String("product_id", req.Id), zap.Error(err))
		return api.GetProductAnalysis500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.GetProductAnalysis200JSONResponse(response), nil
}

func (s *Server) GetProductAnalysisStatus(ctx context.Context, req api.GetProductAnalysisStatusRequestObject) (api.GetProductAnalysisStatusResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id for product analysis status summary", zap.String("product_id", req.Id))
		return api.GetProductAnalysisStatus500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	status, err := s.productAnalysis.LatestStatus(ctx, userID, req.Id)
	if err != nil {
		s.logger.Error("get product analysis status summary", zap.String("product_id", req.Id), zap.Error(err))
		if errors.Is(err, product.ErrNotFound) || errors.Is(err, productanalysis.ErrNotFound) {
			return api.GetProductAnalysisStatus404JSONResponse(errBody(http.StatusNotFound, "product or analysis not found")), nil
		}

		return api.GetProductAnalysisStatus500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.GetProductAnalysisStatus200JSONResponse(api.ProductAnalysisStatusResponse{
		Id:                        status.ID,
		Status:                    api.ProductAnalysisStatusResponseStatus(status.Status),
		CurrentStage:              api.ProductAnalysisStatusResponseCurrentStage(status.CurrentStage),
		ProductResultAvailable:    status.ProductResultAvailable,
		AudienceResultAvailable:   status.AudienceResultAvailable,
		HypothesisResultAvailable: status.HypothesisResultAvailable,
		AudienceReviewAvailable:   status.AudienceReviewAvailable,
		HypothesisReviewAvailable: status.HypothesisReviewAvailable,
	}), nil
}

func (s *Server) SaveProductAnalysisReview(ctx context.Context, req api.SaveProductAnalysisReviewRequestObject) (api.SaveProductAnalysisReviewResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id for product analysis review", zap.String("product_id", req.Id))
		return api.SaveProductAnalysisReview500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	if req.Body == nil {
		s.logger.Error("missing product analysis review body", zap.String("product_id", req.Id))
		return api.SaveProductAnalysisReview400JSONResponse(errBody(http.StatusBadRequest, "review body is required")), nil
	}

	review, err := s.productAnalysis.SaveReview(ctx, userID, req.Id, string(req.Body.Stage), req.Body.SelectedCandidateIds, req.Body.CustomCandidates, hypothesisRatings(req.Body.Ratings))
	if err != nil {
		s.logger.Error("save product analysis review", zap.String("product_id", req.Id), zap.String("stage", string(req.Body.Stage)), zap.Error(err))
		switch {
		case errors.Is(err, product.ErrNotFound), errors.Is(err, productanalysis.ErrNotFound):
			return api.SaveProductAnalysisReview404JSONResponse(errBody(http.StatusNotFound, "product or analysis not found")), nil
		case errors.Is(err, services.ErrInvalidAnalysisReview):
			return api.SaveProductAnalysisReview400JSONResponse(errBody(http.StatusBadRequest, "invalid analysis review")), nil
		case errors.Is(err, services.ErrAnalysisReviewNotReady):
			return api.SaveProductAnalysisReview409JSONResponse(errBody(http.StatusConflict, "analysis is not waiting for this review")), nil
		default:
			return api.SaveProductAnalysisReview500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
		}
	}

	response, err := newProductAnalysisReviewResponse(review)
	if err != nil {
		s.logger.Error("build saved product analysis review response", zap.String("product_id", req.Id), zap.Error(err))
		return api.SaveProductAnalysisReview500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.SaveProductAnalysisReview200JSONResponse(response), nil
}

func (s *Server) ContinueProductAnalysis(ctx context.Context, req api.ContinueProductAnalysisRequestObject) (api.ContinueProductAnalysisResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id to continue product analysis", zap.String("product_id", req.Id))
		return api.ContinueProductAnalysis500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	run, err := s.productAnalysis.Latest(ctx, userID, req.Id)
	if err != nil {
		s.logger.Error("get product analysis to continue", zap.String("product_id", req.Id), zap.Error(err))
		if errors.Is(err, product.ErrNotFound) || errors.Is(err, productanalysis.ErrNotFound) {
			return api.ContinueProductAnalysis404JSONResponse(errBody(http.StatusNotFound, "product or analysis not found")), nil
		}

		return api.ContinueProductAnalysis500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	if run.Status != "waiting_for_user_input" || run.CurrentStage != "audience" || run.AudienceReview == nil {
		s.logger.Warn("product analysis is not ready to continue", zap.String("product_id", req.Id), zap.String("run_id", run.ID), zap.String("status", run.Status), zap.String("stage", run.CurrentStage))
		return api.ContinueProductAnalysis409JSONResponse(errBody(http.StatusConflict, "analysis is not ready to continue")), nil
	}

	if err := s.productAnalysisStarter.Continue(ctx, run.ID, "hypotheses"); err != nil {
		s.logger.Error("continue product analysis workflow", zap.String("product_id", req.Id), zap.String("run_id", run.ID), zap.Error(err))
		return api.ContinueProductAnalysis500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	response, err := productAnalysisResponse(run)
	if err != nil {
		s.logger.Error("build continued product analysis response", zap.String("product_id", req.Id), zap.Error(err))
		return api.ContinueProductAnalysis500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.ContinueProductAnalysis200JSONResponse(response), nil
}

func productAnalysisResponse(run *productanalysis.Run) (api.ProductAnalysisResponse, error) {
	var audienceReview interface{}
	if run.AudienceReview != nil {
		mappedReview, err := newProductAnalysisReviewResponse(run.AudienceReview)
		if err != nil {
			return api.ProductAnalysisResponse{}, fmt.Errorf("map audience analysis review: %w", err)
		}

		audienceReview = mappedReview
	}

	var hypothesisReview interface{}
	if run.HypothesisReview != nil {
		mappedReview, err := newProductAnalysisReviewResponse(run.HypothesisReview)
		if err != nil {
			return api.ProductAnalysisResponse{}, fmt.Errorf("map hypothesis analysis review: %w", err)
		}

		hypothesisReview = mappedReview
	}

	response := api.ProductAnalysisResponse{
		Id:               run.ID,
		Status:           api.ProductAnalysisResponseStatus(run.Status),
		CurrentStage:     api.ProductAnalysisResponseCurrentStage(run.CurrentStage),
		AudienceReview:   audienceReview,
		HypothesisReview: hypothesisReview,
	}
	for _, item := range []struct {
		name   string
		raw    json.RawMessage
		target *interface{}
	}{
		{name: "product", raw: run.ProductResult, target: &response.ProductResult},
		{name: "audience", raw: run.AudienceResult, target: &response.AudienceResult},
		{name: "hypothesis", raw: run.HypothesisResult, target: &response.HypothesisResult},
	} {
		if len(item.raw) == 0 {
			continue
		}

		if err := json.Unmarshal(item.raw, item.target); err != nil {
			return api.ProductAnalysisResponse{}, fmt.Errorf("decode %s analysis result: %w", item.name, err)
		}
	}

	return response, nil
}

func newProductAnalysisReviewResponse(review *productanalysis.StageReview) (api.ProductAnalysisReviewResponse, error) {
	customCandidates := []map[string]interface{}{}
	if len(review.CustomCandidates) > 0 {
		if err := json.Unmarshal(review.CustomCandidates, &customCandidates); err != nil {
			return api.ProductAnalysisReviewResponse{}, fmt.Errorf("decode custom analysis review candidates: %w", err)
		}
	}

	ratings := map[string]api.HypothesisRating{}
	if len(review.Ratings) > 0 {
		if err := json.Unmarshal(review.Ratings, &ratings); err != nil {
			return api.ProductAnalysisReviewResponse{}, fmt.Errorf("decode hypothesis ratings: %w", err)
		}
	}

	return api.ProductAnalysisReviewResponse{
		Stage:                api.ProductAnalysisReviewStage(review.Stage),
		SelectedCandidateIds: review.SelectedCandidateIDs,
		CustomCandidates:     customCandidates,
		Ratings:              ratings,
		UpdatedAt:            review.UpdatedAt,
	}, nil
}

func hypothesisRatings(ratings *map[string]api.HypothesisRating) map[string]string {
	result := map[string]string{}
	if ratings == nil {
		return result
	}

	for id, rating := range *ratings {
		result[id] = string(rating)
	}

	return result
}
