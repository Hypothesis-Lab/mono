package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"app/internal/config"
	"app/internal/repositories/product"
)

type GeneratorClient struct {
	endpoint   string
	httpClient *http.Client
}

type generatorJobRequest struct {
	ProductID string           `json:"product_id"`
	Input     generatorProduct `json:"input"`
}

type generatorProduct struct {
	Brand            string          `json:"brand"`
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	Category         string          `json:"category"`
	Geography        string          `json:"geography"`
	ExperimentGoal   string          `json:"experiment_goal"`
	TargetAction     string          `json:"target_action"`
	SourceURL        *string         `json:"source_url"`
	Facts            json.RawMessage `json:"facts"`
	EligibilityRules json.RawMessage `json:"eligibility_rules"`
	AllowedChannels  []string        `json:"allowed_channels"`
	ForbiddenClaims  []string        `json:"forbidden_claims"`
	KnownConstraints []string        `json:"known_constraints"`
}

type generatorJobResponse struct {
	ID string `json:"id"`
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
		Input: generatorProduct{
			Brand:            p.Brand,
			Name:             p.Name,
			Description:      p.Description,
			Category:         p.Category,
			Geography:        p.Geography,
			ExperimentGoal:   p.ExperimentGoal,
			TargetAction:     p.TargetAction,
			SourceURL:        p.Link,
			Facts:            p.Facts,
			EligibilityRules: p.EligibilityRules,
			AllowedChannels:  p.AllowedChannels,
			ForbiddenClaims:  p.ForbiddenClaims,
			KnownConstraints: p.KnownConstraints,
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
