package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"app/internal/fileextract"
	"app/internal/repositories/file"
	"app/internal/repositories/product"
	"app/internal/repositories/productanalysis"
	"app/internal/storage"

	"github.com/google/uuid"
)

var ErrAnalysisReviewNotReady = errors.New("product analysis is not waiting for user input")
var ErrInvalidAnalysisReview = errors.New("invalid product analysis review")

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

func (s *ProductAnalysisService) SaveReview(ctx context.Context, userID, productID, stage string, selectedIDs []string, customCandidates []map[string]any) (*productanalysis.StageReview, error) {
	if err := validateReviewRequest(stage, selectedIDs); err != nil {
		return nil, err
	}

	if _, err := s.products.Get(ctx, userID, productID); err != nil {
		return nil, fmt.Errorf("get product for analysis review: %w", err)
	}

	runID, err := s.runs.LatestID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("get latest analysis for review: %w", err)
	}

	run, err := s.runs.Get(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("get analysis for review: %w", err)
	}

	if run.Status != "waiting_for_user_input" || run.CurrentStage != stage {
		return nil, ErrAnalysisReviewNotReady
	}

	customCandidates, err = normalizeCustomCandidates(customCandidates, stage)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidAnalysisReview, err)
	}

	if err := validateSelectedCandidates(stage, resultForStage(run, stage), selectedIDs, customCandidates); err != nil {
		return nil, err
	}

	customJSON, err := json.Marshal(customCandidates)
	if err != nil {
		return nil, fmt.Errorf("encode analysis review candidates: %w", err)
	}

	review, err := s.runs.SaveReview(ctx, runID, stage, selectedIDs, customJSON)
	if err != nil {
		return nil, fmt.Errorf("save analysis review: %w", err)
	}

	return review, nil
}

func validateReviewRequest(stage string, selectedIDs []string) error {
	if stage != "audience" && stage != "hypotheses" {
		return fmt.Errorf("%w: unsupported stage %q", ErrInvalidAnalysisReview, stage)
	}

	if len(selectedIDs) == 0 {
		return fmt.Errorf("%w: at least one candidate must be selected", ErrInvalidAnalysisReview)
	}

	return nil
}

func validateSelectedCandidates(stage string, result json.RawMessage, selectedIDs []string, customCandidates []map[string]any) error {
	allowedIDs, err := candidateIDs(stage, result)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidAnalysisReview, err)
	}

	for _, candidate := range customCandidates {
		allowedIDs[fmt.Sprint(candidate["candidate_id"])] = struct{}{}
	}

	seen := make(map[string]struct{}, len(selectedIDs))
	for _, id := range selectedIDs {
		if _, duplicate := seen[id]; duplicate {
			return fmt.Errorf("%w: selected candidate %q is duplicated", ErrInvalidAnalysisReview, id)
		}

		if _, allowed := allowedIDs[id]; !allowed {
			return fmt.Errorf("%w: selected candidate %q is unknown", ErrInvalidAnalysisReview, id)
		}

		seen[id] = struct{}{}
	}

	return nil
}

func resultForStage(run *productanalysis.Run, stage string) json.RawMessage {
	if stage == "audience" {
		return run.AudienceResult
	}

	return run.HypothesisResult
}

func candidateIDs(stage string, result json.RawMessage) (map[string]struct{}, error) {
	ids := map[string]struct{}{}
	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		return nil, fmt.Errorf("decode %s candidates: %w", stage, err)
	}

	key := "hypotheses"
	if stage == "audience" {
		key = "candidates"
	}

	items, ok := payload[key].([]any)
	if !ok {
		if stage == "audience" {
			if audience, nested := payload["audience"].(map[string]any); nested {
				items, _ = audience[key].([]any)
			}
		}
	}

	for _, item := range items {
		candidate, ok := item.(map[string]any)
		if !ok {
			continue
		}

		id, ok := candidate["candidate_id"].(string)
		if ok && id != "" {
			ids[id] = struct{}{}
		}
	}

	return ids, nil
}

func normalizeCustomCandidates(candidates []map[string]any, stage string) ([]map[string]any, error) {
	result := make([]map[string]any, 0, len(candidates))
	for index, candidate := range candidates {
		copyCandidate := make(map[string]any, len(candidate)+2)
		for key, value := range candidate {
			copyCandidate[key] = value
		}

		id, _ := copyCandidate["candidate_id"].(string)
		if strings.TrimSpace(id) == "" {
			copyCandidate["candidate_id"] = fmt.Sprintf("custom_%s_%03d", stage, index+1)
		}

		if stage == "audience" {
			segmentID, _ := copyCandidate["segment_id"].(string)
			if strings.TrimSpace(segmentID) == "" {
				copyCandidate["segment_id"] = copyCandidate["candidate_id"]
			}
		}

		result = append(result, copyCandidate)
	}

	return result, nil
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
