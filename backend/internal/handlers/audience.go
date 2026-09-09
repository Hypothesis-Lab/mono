package handlers

import (
	"context"
	"errors"
	"net/http"

	"app/internal/api"
	"app/internal/http/middlewares"
	"app/internal/repositories/audience"
	"app/internal/repositories/product"

	"go.uber.org/zap"
)

func validateAudienceAges(ageFrom, ageTo *int) error {
	if ageFrom != nil && ageTo != nil && *ageFrom > *ageTo {
		return errors.New("age_from must be less than or equal to age_to")
	}

	return nil
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

		s.logger.Error("get product for audiences", zap.Error(err))

		return api.ListProductAudiences500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	items, err := s.audiences.ListByProductID(ctx, req.Id)
	if err != nil {
		s.logger.Error("list product audiences", zap.Error(err))

		return api.ListProductAudiences500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	resp := make(api.ListProductAudiences200JSONResponse, 0, len(items))
	for _, a := range items {
		resp = append(resp, newAudienceResponse(a))
	}

	return resp, nil
}

func toInt32Ptr(v *int) *int32 {
	if v == nil {
		return nil
	}
	converted := int32(*v)
	return &converted
}

func toStringPtr(v *api.AudienceGender) *string {
	if v == nil {
		return nil
	}
	converted := string(*v)
	return &converted
}

func toAudienceGenderPtr(v *string) *api.AudienceGender {
	if v == nil {
		return nil
	}
	converted := api.AudienceGender(*v)
	return &converted
}

func (s *Server) CreateProductAudience(ctx context.Context, req api.CreateProductAudienceRequestObject) (api.CreateProductAudienceResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id in context")

		return api.CreateProductAudience500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	if req.Body.Description == "" {
		return api.CreateProductAudience400JSONResponse(errBody(http.StatusBadRequest, "description is required")), nil
	}

	if err := validateAudienceAges(req.Body.AgeFrom, req.Body.AgeTo); err != nil {
		return api.CreateProductAudience400JSONResponse(errBody(http.StatusBadRequest, err.Error())), nil
	}

	if _, err := s.products.Get(ctx, userID, req.Id); err != nil {
		if errors.Is(err, product.ErrNotFound) {
			s.logger.Error("get product for create audience", zap.String("id", req.Id), zap.Error(err))

			return api.CreateProductAudience404JSONResponse(errBody(http.StatusNotFound, "product not found")), nil
		}

		s.logger.Error("get product for create audience", zap.Error(err))

		return api.CreateProductAudience500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	created, err := s.audiences.Create(ctx, audience.CreateInput{
		ProductID:         req.Id,
		Description:       req.Body.Description,
		AgeFrom:           toInt32Ptr(req.Body.AgeFrom),
		AgeTo:             toInt32Ptr(req.Body.AgeTo),
		Gender:            toStringPtr(req.Body.Gender),
		Interests:         req.Body.Interests,
		Geography:         req.Body.Geography,
		PainPoints:        req.Body.PainPoints,
		DesiresMotivation: req.Body.DesiresMotivation,
	})
	if err != nil {
		s.logger.Error("create audience", zap.Error(err))

		return api.CreateProductAudience500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.CreateProductAudience201JSONResponse(newAudienceResponse(created)), nil
}

func (s *Server) GetAudience(ctx context.Context, req api.GetAudienceRequestObject) (api.GetAudienceResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id in context")

		return api.GetAudience500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	a, err := s.audiences.Get(ctx, userID, req.Id)
	if err != nil {
		if errors.Is(err, audience.ErrNotFound) {
			s.logger.Error("get audience", zap.String("id", req.Id), zap.Error(err))

			return api.GetAudience404JSONResponse(errBody(http.StatusNotFound, "audience not found")), nil
		}

		s.logger.Error("get audience", zap.Error(err))

		return api.GetAudience500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.GetAudience200JSONResponse(newAudienceResponse(a)), nil
}

func (s *Server) UpdateAudience(ctx context.Context, req api.UpdateAudienceRequestObject) (api.UpdateAudienceResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id in context")

		return api.UpdateAudience500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	if req.Body == nil {
		return api.UpdateAudience400JSONResponse(errBody(http.StatusBadRequest, "invalid request body")), nil
	}

	if err := validateAudienceAges(req.Body.AgeFrom, req.Body.AgeTo); err != nil {
		return api.UpdateAudience400JSONResponse(errBody(http.StatusBadRequest, err.Error())), nil
	}

	updated, err := s.audiences.Update(ctx, userID, req.Id, audience.UpdateInput{
		Description:       req.Body.Description,
		AgeFrom:           toInt32Ptr(req.Body.AgeFrom),
		AgeTo:             toInt32Ptr(req.Body.AgeTo),
		Gender:            toStringPtr(req.Body.Gender),
		Interests:         req.Body.Interests,
		Geography:         req.Body.Geography,
		PainPoints:        req.Body.PainPoints,
		DesiresMotivation: req.Body.DesiresMotivation,
	})
	if err != nil {
		if errors.Is(err, audience.ErrNotFound) {
			s.logger.Error("update audience", zap.String("id", req.Id), zap.Error(err))

			return api.UpdateAudience404JSONResponse(errBody(http.StatusNotFound, "audience not found")), nil
		}

		s.logger.Error("update audience", zap.Error(err))

		return api.UpdateAudience500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.UpdateAudience200JSONResponse(newAudienceResponse(updated)), nil
}

func (s *Server) DeleteAudience(ctx context.Context, req api.DeleteAudienceRequestObject) (api.DeleteAudienceResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id in context")

		return api.DeleteAudience500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	if err := s.audiences.Delete(ctx, userID, req.Id); err != nil {
		if errors.Is(err, audience.ErrNotFound) {
			s.logger.Error("delete audience", zap.String("id", req.Id), zap.Error(err))

			return api.DeleteAudience404JSONResponse(errBody(http.StatusNotFound, "audience not found")), nil
		}

		s.logger.Error("delete audience", zap.Error(err))

		return api.DeleteAudience500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.DeleteAudience204Response{}, nil
}

func newAudienceResponse(a *audience.Audience) api.AudienceResponse {
	ageFrom := (*int)(nil)
	if a.AgeFrom != nil {
		converted := int(*a.AgeFrom)
		ageFrom = &converted
	}

	ageTo := (*int)(nil)
	if a.AgeTo != nil {
		converted := int(*a.AgeTo)
		ageTo = &converted
	}

	return api.AudienceResponse{
		Id:                a.ID,
		ProductId:         a.ProductID,
		Description:       a.Description,
		AgeFrom:           ageFrom,
		AgeTo:             ageTo,
		Gender:            toAudienceGenderPtr(a.Gender),
		Interests:         a.Interests,
		Geography:         a.Geography,
		PainPoints:        a.PainPoints,
		DesiresMotivation: a.DesiresMotivation,
		CreatedAt:         &a.CreatedAt,
	}
}
