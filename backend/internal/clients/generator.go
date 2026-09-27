package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"app/internal/config"
	"app/internal/repositories/product"
)

var (
	ErrProductAlreadyQueued       = errors.New("product already queued")
	ErrProductGenerationNotFound  = errors.New("product generation not found")
	ErrProductGenerationNotFailed = errors.New("product generation is not failed")
)

type GeneratorClient struct {
	endpoint   string
	httpClient *http.Client
}

type generatorJobRequest struct {
	ProductID string         `json:"product_id"`
	Kind      string         `json:"kind"`
	Input     generatorInput `json:"input"`
}

type generatorInput struct {
	Product    generatorProduct    `json:"product"`
	Experiment generatorExperiment `json:"experiment"`
	Campaign   generatorCampaign   `json:"campaign"`
}

type generatorProduct struct {
	Brand            string          `json:"brand"`
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	Category         string          `json:"category"`
	CustomCategory   *string         `json:"custom_category"`
	Country          string          `json:"country"`
	Town             *string         `json:"town"`
	SourceURL        *string         `json:"source_url"`
	Facts            json.RawMessage `json:"facts"`
	KeyFactIDs       []string        `json:"key_fact_ids"`
	Price            json.RawMessage `json:"price"`
	FactsCheckedAt   *string         `json:"facts_checked_at"`
	EligibilityRules json.RawMessage `json:"eligibility_rules"`
	Keywords         []string        `json:"keywords"`
	ForbiddenClaims  []string        `json:"forbidden_claims"`
	KnownConstraints []string        `json:"known_constraints"`
	Unknowns         []string        `json:"unknowns"`
}

type generatorExperiment struct {
	BusinessGoal      string  `json:"business_goal"`
	GoalOrigin        string  `json:"goal_origin"`
	TargetAction      string  `json:"target_action"`
	PrimaryMetric     string  `json:"primary_metric"`
	MetricDirection   string  `json:"metric_direction"`
	MetricNumerator   string  `json:"metric_numerator"`
	MetricDenominator string  `json:"metric_denominator"`
	SuccessCriterion  *string `json:"success_criterion"`
}

type generatorCampaign struct {
	AllowedChannels       []string `json:"allowed_channels"`
	ContactablePersonaIDs []string `json:"contactable_persona_ids"`
	Note                  string   `json:"note"`
}

type generatorJobResponse struct {
	ID string `json:"id"`
}

type generatorGenerationStatusResponse struct {
	Status string `json:"status"`
}

type completedJobResponse struct {
	ID        string          `json:"id"`
	ProductID string          `json:"product_id"`
	Kind      string          `json:"kind"`
	Result    json.RawMessage `json:"result"`
}

type CompletedJob struct {
	ID        string
	ProductID string
	Kind      string
	Result    json.RawMessage
}

func NewGeneratorClient(cfg *config.Config) *GeneratorClient {
	return &GeneratorClient{
		endpoint:   strings.TrimRight(cfg.GeneratorURL, "/") + "/jobs",
		httpClient: http.DefaultClient,
	}
}

func (c *GeneratorClient) QueueProduct(ctx context.Context, p *product.Product) (string, error) {
	requestBody, err := json.Marshal(generatorJobRequest{
		ProductID: p.ID,
		Kind:      "audiences",
		Input: generatorInput{
			Product: generatorProduct{
				Brand:            p.Brand,
				Name:             p.Name,
				Description:      p.Description,
				Category:         p.Category,
				CustomCategory:   p.CustomCategory,
				Country:          p.Country,
				Town:             p.Town,
				SourceURL:        p.Link,
				Facts:            p.Facts,
				KeyFactIDs:       p.KeyFactIDs,
				Price:            p.Price,
				FactsCheckedAt:   p.FactsCheckedAt,
				EligibilityRules: p.EligibilityRules,
				Keywords:         p.Keywords,
				ForbiddenClaims:  p.ForbiddenClaims,
				KnownConstraints: p.KnownConstraints,
				Unknowns:         p.Unknowns,
			},
			Experiment: generatorExperiment{
				BusinessGoal:      p.ExperimentGoal,
				GoalOrigin:        p.GoalOrigin,
				TargetAction:      p.TargetAction,
				PrimaryMetric:     p.PrimaryMetric,
				MetricDirection:   p.MetricDirection,
				MetricNumerator:   p.MetricNumerator,
				MetricDenominator: p.MetricDenominator,
				SuccessCriterion:  p.SuccessCriterion,
			},
			Campaign: generatorCampaign{
				AllowedChannels:       p.AllowedChannels,
				ContactablePersonaIDs: p.ContactablePersonaIDs,
				Note:                  p.CampaignNote,
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("marshal generator job request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return "", fmt.Errorf("create generator job request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("send generator job request: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("read generator job response: %w", err)
	}

	if response.StatusCode == http.StatusConflict {
		return "", ErrProductAlreadyQueued
	}

	if response.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("generator job request returned HTTP status %d: %s", response.StatusCode, responseBody)
	}

	var result generatorJobResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return "", fmt.Errorf("decode generator job response: %w", err)
	}

	if result.ID == "" {
		return "", fmt.Errorf("generator job response did not contain an id")
	}

	return result.ID, nil
}

func (c *GeneratorClient) ProductGenerationStatus(ctx context.Context, productID string) (string, error) {
	endpoint := strings.TrimSuffix(c.endpoint, "/jobs") + "/products/" + url.PathEscape(productID) + "/generation-status"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("create generator generation status request: %w", err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("send generator generation status request: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("read generator generation status response: %w", err)
	}

	if response.StatusCode == http.StatusNotFound {
		return "", ErrProductGenerationNotFound
	}

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("generator generation status request returned HTTP status %d: %s", response.StatusCode, responseBody)
	}

	var result generatorGenerationStatusResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return "", fmt.Errorf("decode generator generation status response: %w", err)
	}

	if result.Status != "created" && result.Status != "working" && result.Status != "completed" && result.Status != "failed" {
		return "", fmt.Errorf("generator returned invalid generation status %q", result.Status)
	}

	return result.Status, nil
}

func (c *GeneratorClient) RetryProductGeneration(ctx context.Context, productID string) error {
	endpoint := strings.TrimSuffix(c.endpoint, "/jobs") + "/products/" + url.PathEscape(productID) + "/generation-retry"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create generator retry request: %w", err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("send generator retry request: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read generator retry response: %w", err)
	}

	if response.StatusCode == http.StatusNotFound {
		return ErrProductGenerationNotFailed
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("generator retry request returned HTTP status %d: %s", response.StatusCode, responseBody)
	}

	return nil
}

func (c *GeneratorClient) NextCompletedJob(ctx context.Context, kind string) (*CompletedJob, error) {
	endpoint := strings.TrimSuffix(c.endpoint, "/jobs") + "/completed-jobs/next?kind=" + url.QueryEscape(kind)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create completed generator job request: %w", err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("send completed generator job request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNoContent {
		return nil, nil //nolint:nilnil // no completed job is a valid result
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read completed generator job response: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("completed generator job request returned HTTP status %d: %s", response.StatusCode, responseBody)
	}

	var responseData completedJobResponse
	if err := json.Unmarshal(responseBody, &responseData); err != nil {
		return nil, fmt.Errorf("decode completed generator job response: %w", err)
	}

	if responseData.ID == "" || responseData.ProductID == "" || responseData.Kind == "" {
		return nil, errors.New("completed generator job response did not contain an id, product id, and kind")
	}

	return &CompletedJob{
		ID:        responseData.ID,
		ProductID: responseData.ProductID,
		Kind:      responseData.Kind,
		Result:    responseData.Result,
	}, nil
}

func (c *GeneratorClient) MarkJobPersisted(ctx context.Context, jobID string) error {
	endpoint := strings.TrimSuffix(c.endpoint, "/jobs") + "/completed-jobs/" + url.PathEscape(jobID) + "/persisted"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create persist generator job request: %w", err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("send persist generator job request: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read persist generator job response: %w", err)
	}

	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("persist generator job request returned HTTP status %d: %s", response.StatusCode, responseBody)
	}

	return nil
}
