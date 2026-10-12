package yandexoauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	oauthBaseURL          = "https://oauth.yandex.ru"
	authorizationEndpoint = oauthBaseURL + "/authorize"
	tokenEndpoint         = oauthBaseURL + "/token"
)

var ErrInvalidConfirmationCode = errors.New("yandex confirmation code is invalid or expired")

type Token struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
}

type Client struct {
	clientID     string
	clientSecret string
	httpClient   *http.Client
}

func New(clientID, clientSecret string) *Client {
	return &Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   http.DefaultClient,
	}
}

func (c *Client) AuthorizationURL() string {
	query := url.Values{
		"response_type": {"code"},
		"client_id":     {c.clientID},
	}

	return authorizationEndpoint + "?" + query.Encode()
}

func (c *Client) ExchangeCode(ctx context.Context, code string) (*Token, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create yandex oauth token request: %w", err)
	}

	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("send yandex oauth token request: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read yandex oauth token response: %w", err)
	}

	return parseTokenResponse(body, response.StatusCode)
}

func parseTokenResponse(body []byte, statusCode int) (*Token, error) {
	var tokenResponse struct {
		AccessToken  string      `json:"access_token"`
		RefreshToken string      `json:"refresh_token"`
		ExpiresIn    json.Number `json:"expires_in"`
		Error        string      `json:"error"`
	}
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
			return nil, fmt.Errorf("decode yandex oauth token error response with HTTP status %d: %w", statusCode, err)
		}

		return nil, fmt.Errorf("decode yandex oauth token response: %w", err)
	}

	if tokenResponse.Error != "" {
		if tokenResponse.Error == "invalid_grant" || tokenResponse.Error == "bad_verification_code" {
			return nil, ErrInvalidConfirmationCode
		}

		return nil, fmt.Errorf("yandex oauth token exchange failed with provider error %q", tokenResponse.Error)
	}

	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("yandex oauth token request returned HTTP status %d", statusCode)
	}

	if tokenResponse.AccessToken == "" || tokenResponse.RefreshToken == "" {
		return nil, errors.New("yandex oauth token response did not contain required tokens")
	}

	expiresIn, err := strconv.ParseInt(tokenResponse.ExpiresIn.String(), 10, 64)
	if err != nil || expiresIn <= 0 {
		return nil, errors.New("yandex oauth token response contained an invalid expiry")
	}

	return &Token{
		AccessToken:  tokenResponse.AccessToken,
		RefreshToken: tokenResponse.RefreshToken,
		ExpiresIn:    expiresIn,
	}, nil
}
