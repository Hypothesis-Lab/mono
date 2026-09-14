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
	yandexDirectURL        = "https://api.direct.yandex.com/json/v5/campaigns"
	yandexDirectSandboxURL = "https://api-sandbox.direct.yandex.com/json/v5/campaigns"
)

type YandexDirectClient struct {
	token      string
	endpoint   string
	httpClient *http.Client
}

type yandexCampaignAddRequest struct {
	Method string `json:"method"`
	Params struct {
		Campaigns []yandexCampaign `json:"Campaigns"`
	} `json:"params"`
}

type yandexCampaign struct {
	Name         string             `json:"Name"`
	StartDate    string             `json:"StartDate"`
	TextCampaign yandexTextCampaign `json:"TextCampaign"`
}

type yandexTextCampaign struct {
	BiddingStrategy yandexBiddingStrategy `json:"BiddingStrategy"`
}

type yandexBiddingStrategy struct {
	Search  yandexStrategy `json:"Search"`
	Network yandexStrategy `json:"Network"`
}

type yandexStrategy struct {
	BiddingStrategyType string `json:"BiddingStrategyType"`
}

type yandexCampaignAddResponse struct {
	Result struct {
		AddResults []struct {
			ID     int64 `json:"Id"`
			Errors []struct {
				Code    int    `json:"Code"`
				Message string `json:"Message"`
				Details string `json:"Details"`
			} `json:"Errors"`
		} `json:"AddResults"`
	} `json:"result"`
	Error *yandexRequestError `json:"error"`
}

type yandexCampaignDeleteRequest struct {
	Method string `json:"method"`
	Params struct {
		SelectionCriteria struct {
			IDs []int64 `json:"Ids"`
		} `json:"SelectionCriteria"`
	} `json:"params"`
}

type yandexCampaignDeleteResponse struct {
	Result struct {
		DeleteResults []struct {
			ID     int64 `json:"Id"`
			Errors []struct {
				Code    int    `json:"Code"`
				Message string `json:"Message"`
				Details string `json:"Details"`
			} `json:"Errors"`
		} `json:"DeleteResults"`
	} `json:"result"`
	Error *yandexRequestError `json:"error"`
}

type yandexRequestError struct {
	RequestID   string `json:"request_id"`
	ErrorCode   int    `json:"error_code"`
	ErrorString string `json:"error_string"`
	ErrorDetail string `json:"error_detail"`
}

func (e *yandexRequestError) Error() string {
	return fmt.Sprintf(
		"yandex direct request failed: code=%d string=%q detail=%q request_id=%q",
		e.ErrorCode,
		e.ErrorString,
		e.ErrorDetail,
		e.RequestID,
	)
}

func NewYandexDirectClient(token string, sandbox bool) *YandexDirectClient {
	endpoint := yandexDirectURL
	if sandbox {
		endpoint = yandexDirectSandboxURL
	}

	return &YandexDirectClient{
		token:      token,
		endpoint:   endpoint,
		httpClient: http.DefaultClient,
	}
}

func NewYandexDirectClientFromConfig(cfg *config.Config) *YandexDirectClient {
	return NewYandexDirectClient(cfg.YandexDirectToken, false)
}

func (c *YandexDirectClient) CreateCampaign(ctx context.Context, name, startDate string) (int64, error) {
	if c.token == "" {
		return 0, errors.New("yandex direct token is required")
	}
	if name == "" {
		return 0, errors.New("campaign name is required")
	}
	if startDate == "" {
		return 0, errors.New("campaign start date is required")
	}

	requestBody := yandexCampaignAddRequest{Method: "add"}
	requestBody.Params.Campaigns = []yandexCampaign{{
		Name:      name,
		StartDate: startDate,
		TextCampaign: yandexTextCampaign{
			BiddingStrategy: yandexBiddingStrategy{
				Search:  yandexStrategy{BiddingStrategyType: "HIGHEST_POSITION"},
				Network: yandexStrategy{BiddingStrategyType: "SERVING_OFF"},
			},
		},
	}}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return 0, fmt.Errorf("marshal yandex direct campaign request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("create yandex direct campaign request: %w", err)
	}

	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Accept-Language", "en")
	request.Header.Set("Content-Type", "application/json; charset=utf-8")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return 0, fmt.Errorf("send yandex direct campaign request: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return 0, fmt.Errorf("read yandex direct campaign response: %w", err)
	}

	var result yandexCampaignAddResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return 0, fmt.Errorf("decode yandex direct campaign response: %w", err)
	}

	if result.Error != nil {
		return 0, result.Error
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return 0, fmt.Errorf("yandex direct campaign request returned HTTP status %d", response.StatusCode)
	}
	if len(result.Result.AddResults) != 1 {
		return 0, fmt.Errorf("yandex direct campaign response contained %d results", len(result.Result.AddResults))
	}
	if len(result.Result.AddResults[0].Errors) > 0 {
		itemError := result.Result.AddResults[0].Errors[0]
		return 0, fmt.Errorf(
			"yandex direct campaign creation failed: code=%d message=%q details=%q",
			itemError.Code,
			itemError.Message,
			itemError.Details,
		)
	}

	return result.Result.AddResults[0].ID, nil
}

func (c *YandexDirectClient) DeleteCampaign(ctx context.Context, id int64) error {
	if c.token == "" {
		return errors.New("yandex direct token is required")
	}
	if id == 0 {
		return errors.New("yandex direct campaign id is required")
	}

	requestBody := yandexCampaignDeleteRequest{Method: "delete"}
	requestBody.Params.SelectionCriteria.IDs = []int64{id}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("marshal yandex direct campaign deletion request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create yandex direct campaign deletion request: %w", err)
	}

	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Accept-Language", "en")
	request.Header.Set("Content-Type", "application/json; charset=utf-8")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("send yandex direct campaign deletion request: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read yandex direct campaign deletion response: %w", err)
	}

	var result yandexCampaignDeleteResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return fmt.Errorf("decode yandex direct campaign deletion response: %w", err)
	}

	if result.Error != nil {
		return result.Error
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("yandex direct campaign deletion request returned HTTP status %d", response.StatusCode)
	}
	if len(result.Result.DeleteResults) != 1 {
		return fmt.Errorf("yandex direct campaign deletion response contained %d results", len(result.Result.DeleteResults))
	}
	if len(result.Result.DeleteResults[0].Errors) > 0 {
		itemError := result.Result.DeleteResults[0].Errors[0]
		return fmt.Errorf(
			"yandex direct campaign deletion failed: code=%d message=%q details=%q",
			itemError.Code,
			itemError.Message,
			itemError.Details,
		)
	}

	return nil
}
