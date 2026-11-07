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

func productAnalysisResponse(run *productanalysis.Run) (api.ProductAnalysisResponse, error) {
	response := api.ProductAnalysisResponse{
		Id:           run.ID,
		Status:       api.ProductAnalysisResponseStatus(run.Status),
		CurrentStage: api.ProductAnalysisResponseCurrentStage(run.CurrentStage),
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
