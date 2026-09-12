package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"app/internal/api"
	"app/internal/clients"
	"app/internal/http/middlewares"
	"app/internal/repositories/audience"
	"app/internal/repositories/product"

	"go.uber.org/zap"
)

func (s *Server) QueueProductAudienceGeneration(ctx context.Context, req api.QueueProductAudienceGenerationRequestObject) (api.QueueProductAudienceGenerationResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id in context")

		return api.QueueProductAudienceGeneration500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	p, err := s.products.Get(ctx, userID, req.Id)
	if err != nil {
		if errors.Is(err, product.ErrNotFound) {
			s.logger.Error("get product for audience generation", zap.String("id", req.Id), zap.Error(err))

			return api.QueueProductAudienceGeneration404JSONResponse(errBody(http.StatusNotFound, "product not found")), nil
		}

		s.logger.Error("get product for audience generation", zap.String("id", req.Id), zap.Error(err))

		return api.QueueProductAudienceGeneration500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	if message := validateProductForGeneration(p); message != "" {
		return api.QueueProductAudienceGeneration400JSONResponse(errBody(http.StatusBadRequest, message)), nil
	}

	if _, err := s.generator.QueueProduct(ctx, p); err != nil {
		if errors.Is(err, clients.ErrProductAlreadyQueued) {
			s.logger.Error("queue product audience generation", zap.String("id", req.Id), zap.Error(err))

			return api.QueueProductAudienceGeneration409JSONResponse(errBody(http.StatusConflict, "The product is already queued up.")), nil
		}

		s.logger.Error("queue product audience generation", zap.String("id", req.Id), zap.Error(err))

		return api.QueueProductAudienceGeneration500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.QueueProductAudienceGeneration201JSONResponse(api.AudienceGenerationStatusResponse{Status: api.Created}), nil
}

func (s *Server) GetProductAudienceGenerationStatus(ctx context.Context, req api.GetProductAudienceGenerationStatusRequestObject) (api.GetProductAudienceGenerationStatusResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id in context")

		return api.GetProductAudienceGenerationStatus500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	if _, err := s.products.Get(ctx, userID, req.Id); err != nil {
		if errors.Is(err, product.ErrNotFound) {
			s.logger.Error("get product for audience generation status", zap.String("id", req.Id), zap.Error(err))

			return api.GetProductAudienceGenerationStatus404JSONResponse(errBody(http.StatusNotFound, "product not found")), nil
		}

		s.logger.Error("get product for audience generation status", zap.String("id", req.Id), zap.Error(err))

		return api.GetProductAudienceGenerationStatus500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	generationStatus, err := s.generator.ProductGenerationStatus(ctx, req.Id)
	if err != nil {
		if errors.Is(err, clients.ErrProductGenerationNotFound) {
			s.logger.Error("get product audience generation status", zap.String("id", req.Id), zap.Error(err))

			return api.GetProductAudienceGenerationStatus404JSONResponse(errBody(http.StatusNotFound, "audience generation not found")), nil
		}

		s.logger.Error("get product audience generation status", zap.String("id", req.Id), zap.Error(err))

		return api.GetProductAudienceGenerationStatus500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.GetProductAudienceGenerationStatus200JSONResponse(api.AudienceGenerationStatusResponse{Status: api.AudienceGenerationStatusResponseStatus(generationStatus)}), nil
}

func (s *Server) RetryProductAudienceGeneration(ctx context.Context, req api.RetryProductAudienceGenerationRequestObject) (api.RetryProductAudienceGenerationResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id in context")

		return api.RetryProductAudienceGeneration500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	if _, err := s.products.Get(ctx, userID, req.Id); err != nil {
		if errors.Is(err, product.ErrNotFound) {
			s.logger.Error("get product for audience generation retry", zap.String("id", req.Id), zap.Error(err))

			return api.RetryProductAudienceGeneration404JSONResponse(errBody(http.StatusNotFound, "product not found")), nil
		}

		s.logger.Error("get product for audience generation retry", zap.String("id", req.Id), zap.Error(err))

		return api.RetryProductAudienceGeneration500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	if err := s.generator.RetryProductGeneration(ctx, req.Id); err != nil {
		if errors.Is(err, clients.ErrProductGenerationNotFailed) {
			s.logger.Error("retry product audience generation", zap.String("id", req.Id), zap.Error(err))

			return api.RetryProductAudienceGeneration409JSONResponse(errBody(http.StatusConflict, "The audience generation is not failed.")), nil
		}

		s.logger.Error("retry product audience generation", zap.String("id", req.Id), zap.Error(err))

		return api.RetryProductAudienceGeneration500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.RetryProductAudienceGeneration200JSONResponse(api.AudienceGenerationStatusResponse{Status: api.Created}), nil
}

func (s *Server) ListProductAudiences(ctx context.Context, req api.ListProductAudiencesRequestObject) (api.ListProductAudiencesResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id in context")

		return api.ListProductAudiences500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	if _, err := s.products.Get(ctx, userID, req.Id); err != nil {
		if errors.Is(err, product.ErrNotFound) {
			s.logger.Error("get product for audiences", zap.String("id", req.Id), zap.Error(err))

			return api.ListProductAudiences404JSONResponse(errBody(http.StatusNotFound, "product not found")), nil
		}

		s.logger.Error("get product for audiences", zap.String("id", req.Id), zap.Error(err))

		return api.ListProductAudiences500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	items, err := s.audiences.ListByProductID(ctx, req.Id)
	if err != nil {
		s.logger.Error("list product audiences", zap.String("id", req.Id), zap.Error(err))

		return api.ListProductAudiences500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	resp := make(api.ListProductAudiences200JSONResponse, 0, len(items))
	for _, item := range items {
		response, err := newAudienceResponse(item)
		if err != nil {
			s.logger.Error("build audience response", zap.String("id", item.ID), zap.Error(err))

			return api.ListProductAudiences500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
		}
		resp = append(resp, response)
	}

	return resp, nil
}

func validateProductForGeneration(p *product.Product) string {
	var facts []api.ProductFact
	if err := json.Unmarshal(p.Facts, &facts); err != nil {
		return "product facts are invalid"
	}
	var rules []api.EligibilityRule
	if err := json.Unmarshal(p.EligibilityRules, &rules); err != nil {
		return "product eligibility rules are invalid"
	}

	return validateCreateProduct(api.CreateProductRequest{
		AllowedChannels:  p.AllowedChannels,
		Brand:            p.Brand,
		Category:         api.ProductCategory(p.Category),
		Description:      p.Description,
		EligibilityRules: rules,
		ExperimentGoal:   p.ExperimentGoal,
		Facts:            facts,
		ForbiddenClaims:  p.ForbiddenClaims,
		Geography:        p.Geography,
		HypothesisId:     p.HypothesisID,
		KnownConstraints: p.KnownConstraints,
		Name:             p.Name,
		TargetAction:     p.TargetAction,
	})
}

func newAudienceResponse(a *audience.Audience) (api.AudienceResponse, error) {
	var persona, research, metrics, hypotheses interface{}
	for _, value := range []struct {
		name   string
		raw    json.RawMessage
		target *interface{}
	}{
		{name: "persona", raw: a.Persona, target: &persona},
		{name: "research", raw: a.Research, target: &research},
		{name: "metrics", raw: a.Metrics, target: &metrics},
		{name: "hypotheses", raw: a.Hypotheses, target: &hypotheses},
	} {
		if err := json.Unmarshal(value.raw, value.target); err != nil {
			return api.AudienceResponse{}, fmt.Errorf("unmarshal audience %s: %w", value.name, err)
		}
	}

	return api.AudienceResponse{
		CreatedAt:      a.CreatedAt,
		GeneratorJobId: a.GeneratorJobID,
		Hypotheses:     hypotheses,
		Id:             a.ID,
		Metrics:        metrics,
		Persona:        persona,
		PersonaId:      a.PersonaID,
		ProductId:      a.ProductID,
		Rank:           a.Rank,
		Research:       research,
		Score:          float32(a.Score),
	}, nil
}
