package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"app/internal/config"
)

type GeneratorClient struct {
	endpoint   string
	httpClient *http.Client
}

type PipelineJob struct {
	ID     string          `json:"id"`
	Kind   string          `json:"kind"`
	Status string          `json:"status"`
	Result json.RawMessage `json:"result"`
}

func NewGeneratorClient(cfg *config.Config) *GeneratorClient {
	return &GeneratorClient{
		endpoint:   strings.TrimRight(cfg.GeneratorURL, "/") + "/jobs",
		httpClient: http.DefaultClient,
	}
}

func (c *GeneratorClient) QueuePipelineJob(ctx context.Context, jobID, productID, kind string, input json.RawMessage) error {
	requestBody, err := json.Marshal(struct {
		ID        string          `json:"id"`
		ProductID string          `json:"product_id"`
		Kind      string          `json:"kind"`
		Input     json.RawMessage `json:"input"`
	}{ID: jobID, ProductID: productID, Kind: kind, Input: input})
	if err != nil {
		return fmt.Errorf("marshal pipeline job request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("create pipeline job request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("send pipeline job request: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read pipeline job response: %w", err)
	}

	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf("queue pipeline job returned HTTP %d: %s", response.StatusCode, body)
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		return fmt.Errorf("decode pipeline job response: %w", err)
	}

	if created.ID != jobID {
		return fmt.Errorf("pipeline job returned unexpected id %q", created.ID)
	}

	return nil
}

func (c *GeneratorClient) GetPipelineJob(ctx context.Context, jobID string) (*PipelineJob, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+"/"+url.PathEscape(jobID), nil)
	if err != nil {
		return nil, fmt.Errorf("create get pipeline job request: %w", err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("send get pipeline job request: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read get pipeline job response: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get pipeline job returned HTTP %d: %s", response.StatusCode, body)
	}

	var job PipelineJob
	if err := json.Unmarshal(body, &job); err != nil {
		return nil, fmt.Errorf("decode pipeline job: %w", err)
	}

	if job.ID != jobID {
		return nil, fmt.Errorf("get pipeline job returned unexpected id %q", job.ID)
	}

	return &job, nil
}

func (c *GeneratorClient) MarkJobPersisted(ctx context.Context, jobID string) error {
	endpoint := c.endpoint + "/" + url.PathEscape(jobID) + "/acknowledged"
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
