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
	if message := validateCreateProductDetails(req); message != "" {
		return message
	}
	if message := validateCreateProductGoal(req); message != "" {
		return message
	}
	if message := validateCreateProductMetrics(req); message != "" {
		return message
	}
	if message := validateFacts(req.Facts); message != "" {
		return message
	}
	if message := validateFactReferences(req.Facts, req.KeyFactIds, req.Price); message != "" {
		return message
	}
	if message := validateEligibilityRules(req.EligibilityRules); message != "" {
		return message
	}
	if message := validateChannels(req.AllowedChannels); message != "" {
		return message
	}
	if strings.TrimSpace(req.HypothesisId) == "" {
		return "hypothesis_id is required"
	}

	return ""
}

func validateCreateProductDetails(req api.CreateProductRequest) string {
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

	return ""
}

func validateCreateProductGoal(req api.CreateProductRequest) string {
	if strings.TrimSpace(req.ExperimentGoal) == "" {
		return "experiment_goal is required"
	}
	if !req.GoalOrigin.Valid() {
		return "goal_origin is required"
	}
	if strings.TrimSpace(req.TargetAction) == "" {
		return "target_action is required"
	}

	return ""
}

func validateCreateProductMetrics(req api.CreateProductRequest) string {
	if strings.TrimSpace(req.PrimaryMetric) == "" || !req.MetricDirection.Valid() || strings.TrimSpace(req.MetricNumerator) == "" || strings.TrimSpace(req.MetricDenominator) == "" {
		return "primary_metric, metric_direction, metric_numerator, and metric_denominator are required"
	}

	return ""
}

func validateUpdateProduct(req *api.UpdateProductRequest) string {
	if message := validateUpdateProductDetails(req); message != "" {
		return message
	}
	if message := validateUpdateProductGoal(req); message != "" {
		return message
	}
	if message := validateUpdateProductMetrics(req); message != "" {
		return message
	}
	if message := validateUpdateProductCollections(req); message != "" {
		return message
	}
	if req.Price != nil && (!req.Price.Status.Valid() || strings.TrimSpace(req.Price.Description) == "") {
		return "price status and description are required"
	}

	return ""
}

func validateUpdateProductDetails(req *api.UpdateProductRequest) string {
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

	return ""
}

func validateUpdateProductGoal(req *api.UpdateProductRequest) string {
	if req.ExperimentGoal != nil && strings.TrimSpace(*req.ExperimentGoal) == "" {
		return "experiment_goal must not be empty"
	}
	if req.TargetAction != nil && strings.TrimSpace(*req.TargetAction) == "" {
		return "target_action must not be empty"
	}

	return ""
}

func validateUpdateProductMetrics(req *api.UpdateProductRequest) string {
	if req.PrimaryMetric != nil && strings.TrimSpace(*req.PrimaryMetric) == "" {
		return "primary_metric must not be empty"
	}
	if req.MetricDirection != nil && !req.MetricDirection.Valid() {
		return "invalid metric_direction"
	}
	if req.MetricNumerator != nil && strings.TrimSpace(*req.MetricNumerator) == "" {
		return "metric_numerator must not be empty"
	}
	if req.MetricDenominator != nil && strings.TrimSpace(*req.MetricDenominator) == "" {
		return "metric_denominator must not be empty"
	}
	if req.GoalOrigin != nil && !req.GoalOrigin.Valid() {
		return "invalid goal_origin"
	}

	return ""
}

func validateUpdateProductCollections(req *api.UpdateProductRequest) string {
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
		if message := validateChannels(*req.AllowedChannels); message != "" {
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

func validateChannels(channels []string) string {
	for _, channel := range channels {
		if strings.TrimSpace(channel) == "" {
			return "allowed channels must not be empty"
		}
	}

	return ""
}

func validateFactReferences(facts []api.ProductFact, keyFactIDs []string, price api.ProductPrice) string {
	if len(keyFactIDs) == 0 {
		return "at least one key fact is required"
	}
	known := make(map[string]struct{}, len(facts))
	for _, fact := range facts {
		known[fact.FactId] = struct{}{}
	}
	for _, factID := range append(keyFactIDs, price.SourceFactIds...) {
		if _, ok := known[factID]; !ok {
			return "key facts and price sources must reference confirmed facts"
		}
	}
	if price.Status == api.Known && len(price.SourceFactIds) == 0 {
		return "known price requires source fact ids"
	}
	if !price.Status.Valid() || strings.TrimSpace(price.Description) == "" {
		return "price status and description are required"
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
	price, err := json.Marshal(req.Price)
	if err != nil {
		return product.CreateInput{}, fmt.Errorf("marshal product price: %w", err)
	}

	keywords := []string{}
	if req.Keywords != nil {
		keywords = *req.Keywords
	}

	return product.CreateInput{
		Name:                  req.Name,
		Description:           req.Description,
		Category:              string(req.Category),
		CustomCategory:        normalizedCustomCategory(req.CustomCategory),
		Brand:                 req.Brand,
		Country:               req.Country,
		Town:                  req.Town,
		ExperimentGoal:        req.ExperimentGoal,
		GoalOrigin:            string(req.GoalOrigin),
		TargetAction:          req.TargetAction,
		PrimaryMetric:         req.PrimaryMetric,
		MetricDirection:       string(req.MetricDirection),
		MetricNumerator:       req.MetricNumerator,
		MetricDenominator:     req.MetricDenominator,
		SuccessCriterion:      req.SuccessCriterion,
		Facts:                 facts,
		KeyFactIDs:            req.KeyFactIds,
		Price:                 price,
		FactsCheckedAt:        req.FactsCheckedAt,
		EligibilityRules:      rules,
		AllowedChannels:       req.AllowedChannels,
		ContactablePersonaIDs: req.ContactablePersonaIds,
		CampaignNote:          req.CampaignNote,
		ForbiddenClaims:       req.ForbiddenClaims,
		KnownConstraints:      req.KnownConstraints,
		Keywords:              keywords,
		Unknowns:              req.Unknowns,
		Link:                  req.Link,
		HypothesisID:          req.HypothesisId,
	}, nil
}

func updateProductInput(req *api.UpdateProductRequest) (product.UpdateInput, error) {
	in := product.UpdateInput{
		Name:                  req.Name,
		Description:           req.Description,
		CustomCategory:        normalizedCustomCategory(req.CustomCategory),
		Brand:                 req.Brand,
		Country:               req.Country,
		Town:                  req.Town,
		ExperimentGoal:        req.ExperimentGoal,
		TargetAction:          req.TargetAction,
		PrimaryMetric:         req.PrimaryMetric,
		MetricNumerator:       req.MetricNumerator,
		MetricDenominator:     req.MetricDenominator,
		AllowedChannels:       req.AllowedChannels,
		KeyFactIDs:            req.KeyFactIds,
		FactsCheckedAt:        req.FactsCheckedAt,
		ContactablePersonaIDs: req.ContactablePersonaIds,
		CampaignNote:          req.CampaignNote,
		ForbiddenClaims:       req.ForbiddenClaims,
		KnownConstraints:      req.KnownConstraints,
		Keywords:              req.Keywords,
		SuccessCriterion:      req.SuccessCriterion,
		Unknowns:              req.Unknowns,
		Link:                  req.Link,
	}
	if req.Category != nil {
		category := string(*req.Category)
		in.Category = &category
	}
	if req.GoalOrigin != nil {
		value := string(*req.GoalOrigin)
		in.GoalOrigin = &value
	}
	if req.MetricDirection != nil {
		value := string(*req.MetricDirection)
		in.MetricDirection = &value
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
	if req.Price != nil {
		price, err := json.Marshal(*req.Price)
		if err != nil {
			return product.UpdateInput{}, fmt.Errorf("marshal product price: %w", err)
		}
		rawPrice := json.RawMessage(price)
		in.Price = &rawPrice
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
	var price api.ProductPrice
	if err := json.Unmarshal(p.Price, &price); err != nil {
		return api.ProductResponse{}, fmt.Errorf("unmarshal product price: %w", err)
	}

	return api.ProductResponse{
		AllowedChannels:       p.AllowedChannels,
		Brand:                 p.Brand,
		CampaignNote:          p.CampaignNote,
		Category:              api.ProductCategory(p.Category),
		ContactablePersonaIds: p.ContactablePersonaIDs,
		Country:               p.Country,
		CustomCategory:        p.CustomCategory,
		Description:           p.Description,
		EligibilityRules:      rules,
		ExperimentGoal:        p.ExperimentGoal,
		Facts:                 facts,
		FactsCheckedAt:        p.FactsCheckedAt,
		ForbiddenClaims:       p.ForbiddenClaims,
		GoalOrigin:            api.ProductResponseGoalOrigin(p.GoalOrigin),
		HypothesisId:          p.HypothesisID,
		Id:                    p.ID,
		KeyFactIds:            p.KeyFactIDs,
		Keywords:              p.Keywords,
		KnownConstraints:      p.KnownConstraints,
		Link:                  p.Link,
		MetricDenominator:     p.MetricDenominator,
		MetricDirection:       api.ProductResponseMetricDirection(p.MetricDirection),
		MetricNumerator:       p.MetricNumerator,
		Name:                  p.Name,
		Price:                 price,
		PrimaryMetric:         p.PrimaryMetric,
		SuccessCriterion:      p.SuccessCriterion,
		TargetAction:          p.TargetAction,
		Town:                  p.Town,
		Unknowns:              p.Unknowns,
	}, nil
}
