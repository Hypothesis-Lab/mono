package handlers

import (
	"context"
	"errors"
	"net/http"

	"app/internal/api"
	"app/internal/http/middlewares"
	"app/internal/repositories/hypothesis"
	"app/internal/repositories/product"
	"app/internal/services"

	"go.uber.org/zap"
)

func (s *Server) CreateHypothesis(ctx context.Context, req api.CreateHypothesisRequestObject) (api.CreateHypothesisResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id in context")

		return api.CreateHypothesis500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	if req.Body.Description == "" {
		return api.CreateHypothesis400JSONResponse(errBody(http.StatusBadRequest, "description is required")), nil
	}

	created, err := s.hypothesisService.Create(ctx, userID, req.Body.Description)
	if err != nil {
		s.logger.Error("create hypothesis", zap.Error(err))

		return api.CreateHypothesis500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.CreateHypothesis201JSONResponse(newHypothesisResponse(created)), nil
}

func (s *Server) ListHypotheses(ctx context.Context, _ api.ListHypothesesRequestObject) (api.ListHypothesesResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id in context")

		return api.ListHypotheses500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	items, err := s.hypothesisService.List(ctx, userID)
	if err != nil {
		s.logger.Error("list hypotheses", zap.Error(err))

		return api.ListHypotheses500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	resp := make(api.ListHypotheses200JSONResponse, 0, len(items))
	for _, h := range items {
		resp = append(resp, newHypothesisResponse(h))
	}

	return resp, nil
}

func (s *Server) GetHypothesis(ctx context.Context, req api.GetHypothesisRequestObject) (api.GetHypothesisResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id in context")

		return api.GetHypothesis500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	h, err := s.hypothesisService.Get(ctx, userID, req.Id)
	if err != nil {
		if errors.Is(err, hypothesis.ErrNotFound) {
			s.logger.Error("get hypothesis", zap.String("id", req.Id), zap.Error(err))

			return api.GetHypothesis404JSONResponse(errBody(http.StatusNotFound, "hypothesis not found")), nil
		}

		s.logger.Error("get hypothesis", zap.String("id", req.Id), zap.Error(err))

		return api.GetHypothesis500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.GetHypothesis200JSONResponse(newHypothesisResponse(h)), nil
}

func (s *Server) GetHypothesisProduct(ctx context.Context, req api.GetHypothesisProductRequestObject) (api.GetHypothesisProductResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id in context")

		return api.GetHypothesisProduct500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	p, err := s.hypothesisService.GetProduct(ctx, userID, req.Id)
	if err != nil {
		switch {
		case errors.Is(err, hypothesis.ErrNotFound):
			s.logger.Error("get hypothesis product", zap.String("id", req.Id), zap.Error(err))

			return api.GetHypothesisProduct404JSONResponse(errBody(http.StatusNotFound, "hypothesis not found")), nil
		case errors.Is(err, product.ErrNotFound):
			s.logger.Error("get hypothesis product", zap.String("id", req.Id), zap.Error(err))

			return api.GetHypothesisProduct404JSONResponse(errBody(http.StatusNotFound, "no product attached")), nil
		default:
			s.logger.Error("get hypothesis product", zap.String("id", req.Id), zap.Error(err))

			return api.GetHypothesisProduct500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
		}
	}

	response, err := newProductResponse(p)
	if err != nil {
		s.logger.Error("build hypothesis product response", zap.String("id", p.ID), zap.Error(err))

		return api.GetHypothesisProduct500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.GetHypothesisProduct200JSONResponse(response), nil
}

func (s *Server) LaunchHypothesis(_ context.Context, req api.LaunchHypothesisRequestObject) (api.LaunchHypothesisResponseObject, error) {
	s.logger.Warn("hypothesis launch is disabled", zap.String("id", req.Id))
	return api.LaunchHypothesis400JSONResponse(errBody(http.StatusBadRequest, "hypothesis launch is not implemented")), nil
}

func (s *Server) GetHypothesisLaunchStatus(ctx context.Context, req api.GetHypothesisLaunchStatusRequestObject) (api.GetHypothesisLaunchStatusResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id in context")

		return api.GetHypothesisLaunchStatus500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	launchStatus, err := s.hypothesisService.GetLaunchStatus(ctx, userID, req.Id)
	if err != nil {
		switch {
		case errors.Is(err, hypothesis.ErrNotFound):
			s.logger.Error("get hypothesis launch status", zap.String("id", req.Id), zap.Error(err))

			return api.GetHypothesisLaunchStatus404JSONResponse(errBody(http.StatusNotFound, "hypothesis not found")), nil
		case errors.Is(err, services.ErrLaunchNotFound):
			s.logger.Error("get hypothesis launch status", zap.String("id", req.Id), zap.Error(err))

			return api.GetHypothesisLaunchStatus404JSONResponse(errBody(http.StatusNotFound, "hypothesis launch not found")), nil
		default:
			s.logger.Error("get hypothesis launch status", zap.String("id", req.Id), zap.Error(err))

			return api.GetHypothesisLaunchStatus500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
		}
	}

	return api.GetHypothesisLaunchStatus200JSONResponse(api.HypothesisLaunchStatusResponse{
		WorkflowId: launchStatus.WorkflowID,
		Status:     api.HypothesisLaunchStatusResponseStatus(launchStatus.Status),
	}), nil
}

func (s *Server) StopHypothesis(ctx context.Context, req api.StopHypothesisRequestObject) (api.StopHypothesisResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id in context")

		return api.StopHypothesis500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	stop, err := s.hypothesisService.Stop(ctx, userID, req.Id)
	if err != nil {
		switch {
		case errors.Is(err, hypothesis.ErrNotFound):
			s.logger.Error("stop hypothesis", zap.String("id", req.Id), zap.Error(err))

			return api.StopHypothesis404JSONResponse(errBody(http.StatusNotFound, "hypothesis not found")), nil
		case errors.Is(err, services.ErrStopLaunchInProgress):
			s.logger.Error("stop hypothesis", zap.String("id", req.Id), zap.Error(err))

			return api.StopHypothesis409JSONResponse(errBody(http.StatusConflict, "hypothesis launch is in progress")), nil
		case errors.Is(err, services.ErrStopInProgress):
			s.logger.Error("stop hypothesis", zap.String("id", req.Id), zap.Error(err))

			return api.StopHypothesis409JSONResponse(errBody(http.StatusConflict, "hypothesis stop is already in progress")), nil
		case errors.Is(err, services.ErrStopManual):
			s.logger.Error("stop hypothesis", zap.String("id", req.Id), zap.Error(err))

			return api.StopHypothesis409JSONResponse(errBody(http.StatusConflict, "hypothesis requires manual intervention")), nil
		case errors.Is(err, services.ErrYandexDirectNotConnected):
			s.logger.Error("stop hypothesis", zap.String("id", req.Id), zap.Error(err))

			return api.StopHypothesis409JSONResponse(errBodyWithCode(http.StatusConflict, "Yandex Direct connection is not connected", "yandex_direct_not_connected")), nil
		default:
			s.logger.Error("stop hypothesis", zap.String("id", req.Id), zap.Error(err))

			return api.StopHypothesis500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
		}
	}

	if stop.Hypothesis != nil {
		return api.StopHypothesis200JSONResponse(newHypothesisResponse(stop.Hypothesis)), nil
	}

	return api.StopHypothesis202JSONResponse(api.HypothesisStopResponse{WorkflowId: stop.WorkflowID, Status: api.HypothesisStopResponseStatusQueued}), nil
}

func (s *Server) UpdateHypothesis(ctx context.Context, req api.UpdateHypothesisRequestObject) (api.UpdateHypothesisResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id in context")

		return api.UpdateHypothesis500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	if req.Body == nil {
		return api.UpdateHypothesis400JSONResponse(errBody(http.StatusBadRequest, "invalid request body")), nil
	}

	updated, err := s.hypothesisService.Update(ctx, userID, req.Id, hypothesis.UpdateInput{
		Description: req.Body.Description,
	})
	if err != nil {
		switch {
		case errors.Is(err, hypothesis.ErrNotFound):
			s.logger.Error("update hypothesis", zap.String("id", req.Id), zap.Error(err))

			return api.UpdateHypothesis404JSONResponse(errBody(http.StatusNotFound, "hypothesis not found")), nil
		case errors.Is(err, services.ErrUpdateInProgress):
			s.logger.Error("update hypothesis", zap.String("id", req.Id), zap.Error(err))

			return api.UpdateHypothesis400JSONResponse(errBody(http.StatusBadRequest, "hypothesis in progress")), nil
		default:
			s.logger.Error("update hypothesis", zap.String("id", req.Id), zap.Error(err))

			return api.UpdateHypothesis500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
		}
	}

	return api.UpdateHypothesis200JSONResponse(newHypothesisResponse(updated)), nil
}

func (s *Server) DeleteHypothesis(ctx context.Context, req api.DeleteHypothesisRequestObject) (api.DeleteHypothesisResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id in context")

		return api.DeleteHypothesis500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	if err := s.hypothesisService.Delete(ctx, userID, req.Id); err != nil {
		switch {
		case errors.Is(err, hypothesis.ErrNotFound):
			s.logger.Error("delete hypothesis", zap.String("id", req.Id), zap.Error(err))

			return api.DeleteHypothesis404JSONResponse(errBody(http.StatusNotFound, "hypothesis not found")), nil
		case errors.Is(err, services.ErrDeleteInProgress):
			s.logger.Error("delete hypothesis", zap.String("id", req.Id), zap.Error(err))

			return api.DeleteHypothesis400JSONResponse(errBody(http.StatusBadRequest, "hypothesis in progress")), nil
		default:
			s.logger.Error("delete hypothesis", zap.String("id", req.Id), zap.Error(err))

			return api.DeleteHypothesis500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
		}
	}

	return api.DeleteHypothesis204Response{}, nil
}

func newHypothesisResponse(h *hypothesis.Hypothesis) api.HypothesisResponse {
	return api.HypothesisResponse{
		Description: h.Description,
		Id:          h.ID,
		Status:      string(h.Status),
		CreatedAt:   h.CreatedAt,
	}
}
