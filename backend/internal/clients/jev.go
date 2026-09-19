package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"app/internal/config"
)

const (
	jevDecisionsURL = "https://openrouter.ai/api/alpha/decisions"
	jevModel        = "~typesafe/jev-latest"
)

type JevClient struct {
	apiKey     string
	httpClient *http.Client
}

type JevDecisionRequest struct {
	State     any                            `json:"state"`
	Questions map[string]JevDecisionQuestion `json:"questions"`
}

type JevDecisionQuestion struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

type JevDecisionResponse struct {
	Model   string                     `json:"model"`
	Answers map[string]json.RawMessage `json:"answers"`
	Raw     json.RawMessage            `json:"-"`
}

func NewJevClient(cfg *config.Config) *JevClient {
	return &JevClient{
		apiKey:     cfg.OpenRouterAPIKey,
		httpClient: http.DefaultClient,
	}
}

func (c *JevClient) Decide(ctx context.Context, input JevDecisionRequest) (*JevDecisionResponse, error) {
	if c.apiKey == "" {
		return nil, errors.New("openrouter API key is required")
	}

	if input.State == nil {
		return nil, errors.New("jev decision state is required")
	}

	if len(input.Questions) == 0 {
		return nil, errors.New("jev decision questions are required")
	}

	body, err := json.Marshal(struct {
		Model string `json:"model"`
		JevDecisionRequest
	}{
		Model:              jevModel,
		JevDecisionRequest: input,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal jev decision request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, jevDecisionsURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create jev decision request: %w", err)
	}

	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("send jev decision request: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read jev decision response: %w", err)
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("jev decision request returned HTTP status %d: %s", response.StatusCode, responseBody)
	}

	var result JevDecisionResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, fmt.Errorf("decode jev decision response: %w", err)
	}

	if len(result.Answers) == 0 {
		return nil, errors.New("jev decision response did not contain answers")
	}

	result.Raw = responseBody
	return &result, nil
}
