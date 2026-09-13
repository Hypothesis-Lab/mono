package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"unicode/utf8"

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

	if req.Body == nil {
		return api.CreateProduct400JSONResponse(errBody(http.StatusBadRequest, "invalid request body")), nil
	}

	if message := validateCreateProduct(*req.Body); message != "" {
		return api.CreateProduct400JSONResponse(errBody(http.StatusBadRequest, message)), nil
	}

	in, err := createProductInput(*req.Body)
	if err != nil {
		s.logger.Error("encode create product input", zap.Error(err))

		return api.CreateProduct500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	created, err := s.productService.Create(ctx, userID, in)
	if err != nil {
		if errors.Is(err, hypothesis.ErrNotFound) {
			s.logger.Error("create product", zap.String("hypothesis_id", req.Body.HypothesisId), zap.Error(err))

			return api.CreateProduct404JSONResponse(errBody(http.StatusNotFound, "hypothesis not found")), nil
		}

		s.logger.Error("create product", zap.String("hypothesis_id", req.Body.HypothesisId), zap.Error(err))

		return api.CreateProduct500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	response, err := newProductResponse(created)
	if err != nil {
		s.logger.Error("build create product response", zap.String("id", created.ID), zap.Error(err))

		return api.CreateProduct500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.CreateProduct201JSONResponse(response), nil
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

	if message := validateUpdateProduct(req.Body); message != "" {
		return api.UpdateProduct400JSONResponse(errBody(http.StatusBadRequest, message)), nil
	}

	in, err := updateProductInput(req.Body)
	if err != nil {
		s.logger.Error("encode update product input", zap.String("id", req.Id), zap.Error(err))

		return api.UpdateProduct500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	updated, err := s.productService.Update(ctx, userID, req.Id, in)
	if err != nil {
		if errors.Is(err, product.ErrCustomCategoryRequired) || errors.Is(err, product.ErrCustomCategoryNotAllowed) {
			return api.UpdateProduct400JSONResponse(errBody(http.StatusBadRequest, err.Error())), nil
		}
		if errors.Is(err, product.ErrNotFound) {
			s.logger.Error("update product", zap.String("id", req.Id), zap.Error(err))

			return api.UpdateProduct404JSONResponse(errBody(http.StatusNotFound, "product not found")), nil
		}

		s.logger.Error("update product", zap.String("id", req.Id), zap.Error(err))

		return api.UpdateProduct500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	response, err := newProductResponse(updated)
	if err != nil {
		s.logger.Error("build update product response", zap.String("id", updated.ID), zap.Error(err))

		return api.UpdateProduct500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.UpdateProduct200JSONResponse(response), nil
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

func validateCreateProduct(req api.CreateProductRequest) string {
	if strings.TrimSpace(req.Name) == "" {
		return "name is required"
	}
	if strings.TrimSpace(req.Description) == "" {
		return "description is required"
	}
	if req.Category == "" || !req.Category.Valid() {
		return "category is required"
	}
	if message := validateCustomCategory(req.Category, req.CustomCategory); message != "" {
		return message
	}
	if strings.TrimSpace(req.Brand) == "" {
		return "brand is required"
	}
	if strings.TrimSpace(req.Country) == "" {
		return "country is required"
	}
	if strings.TrimSpace(req.ExperimentGoal) == "" {
		return "experiment_goal is required"
	}
	if strings.TrimSpace(req.TargetAction) == "" {
		return "target_action is required"
	}
	if message := validateFacts(req.Facts); message != "" {
		return message
	}
	if message := validateEligibilityRules(req.EligibilityRules); message != "" {
		return message
	}
	if message := validateAllowedChannels(req.AllowedChannels); message != "" {
		return message
	}
	if strings.TrimSpace(req.HypothesisId) == "" {
		return "hypothesis_id is required"
	}

	return ""
}

func validateUpdateProduct(req *api.UpdateProductRequest) string {
	if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
		return "name must not be empty"
	}
	if req.Description != nil && strings.TrimSpace(*req.Description) == "" {
		return "description must not be empty"
	}
	if req.Category != nil && !req.Category.Valid() {
		return "invalid category"
	}
	if req.CustomCategory != nil {
		if message := validateCustomCategoryValue(*req.CustomCategory); message != "" {
			return message
		}
		if req.Category != nil && *req.Category != api.ProductCategoryOther {
			return "custom_category is only allowed when category is other"
		}
	}
	if req.Brand != nil && strings.TrimSpace(*req.Brand) == "" {
		return "brand must not be empty"
	}
	if req.Country != nil && strings.TrimSpace(*req.Country) == "" {
		return "country must not be empty"
	}
	if req.ExperimentGoal != nil && strings.TrimSpace(*req.ExperimentGoal) == "" {
		return "experiment_goal must not be empty"
	}
	if req.TargetAction != nil && strings.TrimSpace(*req.TargetAction) == "" {
		return "target_action must not be empty"
	}
	if req.Facts != nil {
		if message := validateFacts(*req.Facts); message != "" {
			return message
		}
	}
	if req.EligibilityRules != nil {
		if message := validateEligibilityRules(*req.EligibilityRules); message != "" {
			return message
		}
	}
	if req.AllowedChannels != nil {
		if message := validateAllowedChannels(*req.AllowedChannels); message != "" {
			return message
		}
	}

	return ""
}

func validateFacts(facts []api.ProductFact) string {
	if len(facts) < 3 {
		return "at least three facts are required"
	}
	for _, fact := range facts {
		if strings.TrimSpace(fact.FactId) == "" || strings.TrimSpace(fact.Text) == "" {
			return "fact_id and text are required for every fact"
		}
	}

	return ""
}

func validateEligibilityRules(rules []api.EligibilityRule) string {
	for _, rule := range rules {
		if strings.TrimSpace(rule.RuleId) == "" || strings.TrimSpace(rule.Field) == "" || strings.TrimSpace(rule.Description) == "" || !validEligibilityValue(rule.Value) || !rule.Operator.Valid() {
			return "rule_id, field, operator, value, and description are required for every eligibility rule"
		}
	}

	return ""
}

func validEligibilityValue(value interface{}) bool {
	switch v := value.(type) {
	case string, bool, int, int8, int16, int32, int64:
		return true
	case float64:
		return math.Trunc(v) == v
	case []interface{}:
		if len(v) == 0 {
			return true
		}
		_, isStringSlice := v[0].(string)
		for _, item := range v {
			if isStringSlice {
				if _, ok := item.(string); !ok {
					return false
				}
				continue
			}
			number, ok := item.(float64)
			if !ok || math.Trunc(number) != number {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func validateAllowedChannels(channels []string) string {
	if len(channels) == 0 {
		return "at least one allowed channel is required"
	}
	for _, channel := range channels {
		if strings.TrimSpace(channel) == "" {
			return "allowed channels must not be empty"
		}
	}

	return ""
}

func validateCustomCategory(category api.ProductCategory, customCategory *string) string {
	if category != api.ProductCategoryOther {
		if customCategory != nil {
			return "custom_category is only allowed when category is other"
		}

		return ""
	}
	if customCategory == nil {
		return "custom_category is required when category is other"
	}

	return validateCustomCategoryValue(*customCategory)
}

func validateCustomCategoryValue(customCategory string) string {
	length := utf8.RuneCountInString(strings.TrimSpace(customCategory))
	if length < 1 || length > 64 {
		return "custom_category must be between 1 and 64 characters"
	}

	return ""
}

func createProductInput(req api.CreateProductRequest) (product.CreateInput, error) {
	facts, err := json.Marshal(req.Facts)
	if err != nil {
		return product.CreateInput{}, fmt.Errorf("marshal product facts: %w", err)
	}
	rules, err := json.Marshal(req.EligibilityRules)
	if err != nil {
		return product.CreateInput{}, fmt.Errorf("marshal eligibility rules: %w", err)
	}

	keywords := []string{}
	if req.Keywords != nil {
		keywords = *req.Keywords
	}

	return product.CreateInput{
		Name:             req.Name,
		Description:      req.Description,
		Category:         string(req.Category),
		CustomCategory:   normalizedCustomCategory(req.CustomCategory),
		Brand:            req.Brand,
		Country:          req.Country,
		Town:             req.Town,
		ExperimentGoal:   req.ExperimentGoal,
		TargetAction:     req.TargetAction,
		Facts:            facts,
		EligibilityRules: rules,
		AllowedChannels:  req.AllowedChannels,
		ForbiddenClaims:  req.ForbiddenClaims,
		KnownConstraints: req.KnownConstraints,
		Keywords:         keywords,
		Price:            req.Price,
		Link:             req.Link,
		HypothesisID:     req.HypothesisId,
	}, nil
}

func updateProductInput(req *api.UpdateProductRequest) (product.UpdateInput, error) {
	in := product.UpdateInput{
		Name:             req.Name,
		Description:      req.Description,
		CustomCategory:   normalizedCustomCategory(req.CustomCategory),
		Brand:            req.Brand,
		Country:          req.Country,
		Town:             req.Town,
		ExperimentGoal:   req.ExperimentGoal,
		TargetAction:     req.TargetAction,
		AllowedChannels:  req.AllowedChannels,
		ForbiddenClaims:  req.ForbiddenClaims,
		KnownConstraints: req.KnownConstraints,
		Keywords:         req.Keywords,
		Price:            req.Price,
		Link:             req.Link,
	}
	if req.Category != nil {
		category := string(*req.Category)
		in.Category = &category
	}
	if req.Facts != nil {
		facts, err := json.Marshal(*req.Facts)
		if err != nil {
			return product.UpdateInput{}, fmt.Errorf("marshal product facts: %w", err)
		}
		rawFacts := json.RawMessage(facts)
		in.Facts = &rawFacts
	}
	if req.EligibilityRules != nil {
		rules, err := json.Marshal(*req.EligibilityRules)
		if err != nil {
			return product.UpdateInput{}, fmt.Errorf("marshal eligibility rules: %w", err)
		}
		rawRules := json.RawMessage(rules)
		in.EligibilityRules = &rawRules
	}

	return in, nil
}

func normalizedCustomCategory(customCategory *string) *string {
	if customCategory == nil {
		return nil
	}

	normalized := strings.TrimSpace(*customCategory)

	return &normalized
}

func newProductResponse(p *product.Product) (api.ProductResponse, error) {
	var facts []api.ProductFact
	if err := json.Unmarshal(p.Facts, &facts); err != nil {
		return api.ProductResponse{}, fmt.Errorf("unmarshal product facts: %w", err)
	}
	var rules []api.EligibilityRule
	if err := json.Unmarshal(p.EligibilityRules, &rules); err != nil {
		return api.ProductResponse{}, fmt.Errorf("unmarshal eligibility rules: %w", err)
	}

	return api.ProductResponse{
		AllowedChannels:  p.AllowedChannels,
		Brand:            p.Brand,
		Category:         api.ProductCategory(p.Category),
		CustomCategory:   p.CustomCategory,
		Description:      p.Description,
		EligibilityRules: rules,
		ExperimentGoal:   p.ExperimentGoal,
		Facts:            facts,
		ForbiddenClaims:  p.ForbiddenClaims,
		Country:          p.Country,
		HypothesisId:     p.HypothesisID,
		Id:               p.ID,
		Keywords:         p.Keywords,
		KnownConstraints: p.KnownConstraints,
		Link:             p.Link,
		Name:             p.Name,
		Price:            p.Price,
		TargetAction:     p.TargetAction,
		Town:             p.Town,
	}, nil
}
