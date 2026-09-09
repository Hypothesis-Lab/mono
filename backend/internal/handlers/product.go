package handlers

import (
	"context"
	"errors"
	"net/http"

	"app/internal/api"
	"app/internal/http/middlewares"
	"app/internal/repositories/hypothesis"
	"app/internal/repositories/product"

	"go.uber.org/zap"
)

func (s *Server) CreateProduct(ctx context.Context, req api.CreateProductRequestObject) (api.CreateProductResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id in context")

		return api.CreateProduct500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	if req.Body.Name == "" {
		return api.CreateProduct400JSONResponse(errBody(http.StatusBadRequest, "name is required")), nil
	}

	if req.Body.Category == "" || !req.Body.Category.Valid() {
		return api.CreateProduct400JSONResponse(errBody(http.StatusBadRequest, "category is required")), nil
	}

	if req.Body.HypothesisId == "" {
		return api.CreateProduct400JSONResponse(errBody(http.StatusBadRequest, "hypothesis_id is required")), nil
	}

	keywords := []string{}
	if req.Body.Keywords != nil {
		keywords = *req.Body.Keywords
	}

	created, err := s.productService.Create(ctx, userID, product.CreateInput{
		Name:         req.Body.Name,
		Description:  req.Body.Description,
		Category:     string(req.Body.Category),
		Keywords:     keywords,
		Price:        req.Body.Price,
		Link:         req.Body.Link,
		HypothesisID: req.Body.HypothesisId,
	})
	if err != nil {
		if errors.Is(err, hypothesis.ErrNotFound) {
			s.logger.Error("create product", zap.String("hypothesis_id", req.Body.HypothesisId), zap.Error(err))

			return api.CreateProduct404JSONResponse(errBody(http.StatusNotFound, "hypothesis not found")), nil
		}

		s.logger.Error("create product", zap.String("hypothesis_id", req.Body.HypothesisId), zap.Error(err))

		return api.CreateProduct500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.CreateProduct201JSONResponse(newProductResponse(created)), nil
}

func (s *Server) UpdateProduct(ctx context.Context, req api.UpdateProductRequestObject) (api.UpdateProductResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id in context")

		return api.UpdateProduct500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	if req.Body == nil {
		return api.UpdateProduct400JSONResponse(errBody(http.StatusBadRequest, "invalid request body")), nil
	}

	in := product.UpdateInput{
		Name:        req.Body.Name,
		Description: req.Body.Description,
		Price:       req.Body.Price,
		Link:        req.Body.Link,
	}

	if req.Body.Category != nil {
		if !req.Body.Category.Valid() {
			return api.UpdateProduct400JSONResponse(errBody(http.StatusBadRequest, "invalid category")), nil
		}

		category := string(*req.Body.Category)
		in.Category = &category
	}

	if req.Body.Keywords != nil {
		in.Keywords = req.Body.Keywords
	}

	updated, err := s.productService.Update(ctx, userID, req.Id, in)
	if err != nil {
		if errors.Is(err, product.ErrNotFound) {
			s.logger.Error("update product", zap.String("id", req.Id), zap.Error(err))

			return api.UpdateProduct404JSONResponse(errBody(http.StatusNotFound, "product not found")), nil
		}

		s.logger.Error("update product", zap.String("id", req.Id), zap.Error(err))

		return api.UpdateProduct500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.UpdateProduct200JSONResponse(newProductResponse(updated)), nil
}

func (s *Server) DeleteProduct(ctx context.Context, req api.DeleteProductRequestObject) (api.DeleteProductResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("missing user id in context")

		return api.DeleteProduct500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	if err := s.productService.Delete(ctx, userID, req.Id); err != nil {
		if errors.Is(err, product.ErrNotFound) {
			s.logger.Error("delete product", zap.String("id", req.Id), zap.Error(err))

			return api.DeleteProduct404JSONResponse(errBody(http.StatusNotFound, "product not found")), nil
		}

		s.logger.Error("delete product", zap.String("id", req.Id), zap.Error(err))

		return api.DeleteProduct500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.DeleteProduct204Response{}, nil
}

func newProductResponse(p *product.Product) api.ProductResponse {
	return api.ProductResponse{
		Category:     api.ProductCategory(p.Category),
		Description:  p.Description,
		HypothesisId: p.HypothesisID,
		Id:           p.ID,
		Keywords:     p.Keywords,
		Link:         p.Link,
		Name:         p.Name,
		Price:        p.Price,
	}
}
