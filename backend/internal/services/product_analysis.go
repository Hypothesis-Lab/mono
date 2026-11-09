package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"app/internal/fileextract"
	"app/internal/repositories/file"
	"app/internal/repositories/product"
	"app/internal/repositories/productanalysis"
	"app/internal/storage"

	"github.com/google/uuid"
)

type ProductAnalysisService struct {
	products *product.Repository
	files    *file.Repository
	storage  *storage.S3
	runs     *productanalysis.Repository
}

type analysisSource struct {
	Source  string `json:"source"`
	Content string `json:"content"`
}

type productAnalysisSnapshot struct {
	TargetProduct      string           `json:"target_product"`
	AsOfDate           *string          `json:"as_of_date"`
	Sources            []analysisSource `json:"sources"`
	UnavailableSources []string         `json:"unavailable_sources"`
}

func NewProductAnalysisService(products *product.Repository, files *file.Repository, storage *storage.S3, runs *productanalysis.Repository) *ProductAnalysisService {
	return &ProductAnalysisService{products: products, files: files, storage: storage, runs: runs}
}

func (s *ProductAnalysisService) Start(ctx context.Context, userID, productID string) (*productanalysis.Run, error) {
	p, err := s.products.Get(ctx, userID, productID)
	if err != nil {
		return nil, fmt.Errorf("get product for analysis: %w", err)
	}

	set, err := s.runs.LatestSegmentSet(ctx)
	if err != nil {
		return nil, fmt.Errorf("get segment dataset for analysis: %w", err)
	}

	if len(set.Segments) == 0 {
		return nil, fmt.Errorf("segment dataset %s has no segments", set.ID)
	}

	form, err := json.Marshal(map[string]any{
		"name": p.Name, "description": p.Description, "category": p.Category, "custom_category": p.CustomCategory,
		"brand": p.Brand, "country": p.Country, "town": p.Town, "experiment_goal": p.ExperimentGoal,
		"goal_origin": p.GoalOrigin, "target_action": p.TargetAction, "primary_metric": p.PrimaryMetric,
		"metric_direction": p.MetricDirection, "metric_numerator": p.MetricNumerator,
		"metric_denominator": p.MetricDenominator, "success_criterion": p.SuccessCriterion,
		"price": p.Price, "allowed_channels": p.AllowedChannels,
		"forbidden_claims": p.ForbiddenClaims, "known_constraints": p.KnownConstraints,
		"keywords": p.Keywords, "unknowns": p.Unknowns, "link": p.Link,
	})
	if err != nil {
		return nil, fmt.Errorf("encode saved product form: %w", err)
	}

	files, err := s.files.ListByHypothesisID(ctx, p.HypothesisID)
	if err != nil {
		return nil, fmt.Errorf("list product analysis documents: %w", err)
	}

	snapshot := productAnalysisSnapshot{
		TargetProduct:      p.Name,
		Sources:            []analysisSource{{Source: "product_form", Content: string(form)}},
		UnavailableSources: []string{},
	}
	for _, document := range files {
		name := "document:" + document.ID + ":" + document.Filename
		text, err := s.documentText(ctx, document)
		if err != nil {
			return nil, fmt.Errorf("read product document %s: %w", document.ID, err)
		}

		if strings.TrimSpace(text) == "" {
			snapshot.UnavailableSources = append(snapshot.UnavailableSources, name)
			continue
		}

		snapshot.Sources = append(snapshot.Sources, analysisSource{Source: name, Content: text})
	}

	input, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("encode product analysis snapshot: %w", err)
	}

	id := uuid.NewString()
	if err := s.runs.Create(ctx, id, productID, set.ID, input); err != nil {
		return nil, fmt.Errorf("store product analysis run: %w", err)
	}

	run, err := s.runs.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get created product analysis run: %w", err)
	}

	return run, nil
}

func (s *ProductAnalysisService) Latest(ctx context.Context, userID, productID string) (*productanalysis.Run, error) {
	if _, err := s.products.Get(ctx, userID, productID); err != nil {
		return nil, fmt.Errorf("get product for analysis status: %w", err)
	}

	id, err := s.runs.LatestID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("get latest product analysis status: %w", err)
	}

	run, err := s.runs.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get product analysis status: %w", err)
	}

	return run, nil
}

func (s *ProductAnalysisService) documentText(ctx context.Context, document *file.File) (string, error) {
	key := document.S3Key + ".extracted.txt"
	text, err := s.storage.Download(ctx, key)
	if err == nil {
		return string(text), nil
	}

	original, err := s.storage.Download(ctx, document.S3Key)
	if err != nil {
		return "", fmt.Errorf("download original document: %w", err)
	}

	extracted, err := fileextract.Extract(ctx, document.Filename, original)
	if err != nil {
		return "", fmt.Errorf("extract existing document: %w", err)
	}

	if err := s.storage.Upload(ctx, key, "text/plain; charset=utf-8", int64(len(extracted)), bytes.NewBufferString(extracted)); err != nil {
		return "", fmt.Errorf("store extracted existing document: %w", err)
	}

	return extracted, nil
}
