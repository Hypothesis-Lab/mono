package yandexdirect

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"app/internal/config"
)

const (
	yandexDirectURL = "https://api.direct.yandex.com/json/v5"
)

var (
	ageRangePattern = regexp.MustCompile(`(\d+)\D+(\d+)`)
	agePlusPattern  = regexp.MustCompile(`(\d+)\+`)
)

type yandexCampaignAddRequest struct {
	Method string `json:"method"`
	Params struct {
		Campaigns []yandexCampaign `json:"Campaigns"`
	} `json:"params"`
}

type yandexCampaign struct {
	Name              string                  `json:"Name"`
	StartDate         string                  `json:"StartDate"`
	CpmBannerCampaign yandexCpmBannerCampaign `json:"CpmBannerCampaign"`
}

type yandexCpmBannerCampaign struct {
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

type yandexAdGroupAddRequest struct {
	Method string `json:"method"`
	Params struct {
		AdGroups []yandexAdGroup `json:"AdGroups"`
	} `json:"params"`
}

type yandexAdGroup struct {
	Name                        string   `json:"Name"`
	CampaignID                  int64    `json:"CampaignId"`
	RegionIDs                   []int64  `json:"RegionIds"`
	CpmBannerUserProfileAdGroup struct{} `json:"CpmBannerUserProfileAdGroup"`
}

type yandexAdGroupDeleteRequest struct {
	Method string `json:"method"`
	Params struct {
		SelectionCriteria struct {
			IDs []int64 `json:"Ids"`
		} `json:"SelectionCriteria"`
	} `json:"params"`
}

type yandexRetargetingListAddRequest struct {
	Method string `json:"method"`
	Params struct {
		RetargetingLists []yandexRetargetingList `json:"RetargetingLists"`
	} `json:"params"`
}

type yandexRetargetingList struct {
	Type  string                  `json:"Type"`
	Name  string                  `json:"Name"`
	Rules []yandexRetargetingRule `json:"Rules"`
}

type yandexRetargetingRule struct {
	Arguments []yandexRetargetingArgument `json:"Arguments"`
	Operator  string                      `json:"Operator"`
}

type yandexRetargetingArgument struct {
	ExternalID int64 `json:"ExternalId"`
}

type yandexRetargetingListDeleteRequest struct {
	Method string `json:"method"`
	Params struct {
		SelectionCriteria struct {
			IDs []int64 `json:"Ids"`
		} `json:"SelectionCriteria"`
	} `json:"params"`
}

type yandexAudienceTargetAddRequest struct {
	Method string `json:"method"`
	Params struct {
		AudienceTargets []yandexAudienceTarget `json:"AudienceTargets"`
	} `json:"params"`
}

type yandexAudienceTarget struct {
	AdGroupID         int64 `json:"AdGroupId"`
	RetargetingListID int64 `json:"RetargetingListId"`
}

type yandexAudienceTargetDeleteRequest struct {
	Method string `json:"method"`
	Params struct {
		SelectionCriteria struct {
			IDs []int64 `json:"Ids"`
		} `json:"SelectionCriteria"`
	} `json:"params"`
}

type yandexAddResponse struct {
	Result struct {
		AddResults []yandexActionResult `json:"AddResults"`
	} `json:"result"`
	Error *yandexRequestError `json:"error"`
}

type yandexDeleteResponse struct {
	Result struct {
		DeleteResults []yandexActionResult `json:"DeleteResults"`
	} `json:"result"`
	Error *yandexRequestError `json:"error"`
}

type yandexActionResult struct {
	ID     int64 `json:"Id"`
	Errors []struct {
		Code    int    `json:"Code"`
		Message string `json:"Message"`
		Details string `json:"Details"`
	} `json:"Errors"`
}

type yandexDictionariesGetRequest struct {
	Method string `json:"method"`
	Params struct {
		DictionaryNames []string `json:"DictionaryNames"`
	} `json:"params"`
}

type yandexDictionariesGetResponse struct {
	Result struct {
		AudienceDemographicProfiles []yandexDemographicProfile   `json:"AudienceDemographicProfiles"`
		AudienceCriteriaTypes       []yandexAudienceCriteriaType `json:"AudienceCriteriaTypes"`
	} `json:"result"`
	Error *yandexRequestError `json:"error"`
}

type yandexListResponse struct {
	Result map[string]json.RawMessage `json:"result"`
	Error  *yandexRequestError        `json:"error"`
}

type Page struct {
	Limit  int
	Offset int
}

type GetOptions struct {
	SelectionCriteria map[string]any
	FieldNames        []string
	AdditionalFields  map[string][]string
	Page              Page
}

type GetResult struct {
	Objects   []json.RawMessage
	LimitedBy *int
}

type yandexDemographicProfile struct {
	ID          int64  `json:"Id"`
	Type        string `json:"Type"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
}

type yandexAudienceCriteriaType struct {
	Type string `json:"Type"`
	Name string `json:"Name"`
}

type yandexRequestError struct {
	RequestID   string `json:"request_id"`
	ErrorCode   int    `json:"error_code"`
	ErrorString string `json:"error_string"`
	ErrorDetail string `json:"error_detail"`
}

type Client struct {
	token      string
	endpoint   string
	httpClient *http.Client
}

func New(token string) *Client {
	return &Client{
		token:      token,
		endpoint:   yandexDirectURL,
		httpClient: http.DefaultClient,
	}
}

func NewFromConfig(cfg *config.Config) *Client {
	return New(cfg.YandexDirectToken)
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

func (c *Client) CreateCampaign(ctx context.Context, name, startDate string) (int64, error) {
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
		CpmBannerCampaign: yandexCpmBannerCampaign{
			BiddingStrategy: yandexBiddingStrategy{
				Search:  yandexStrategy{BiddingStrategyType: "SERVING_OFF"},
				Network: yandexStrategy{BiddingStrategyType: "MANUAL_CPM"},
			},
		},
	}}

	var result yandexCampaignAddResponse
	if err := c.post(ctx, "campaigns", requestBody, &result); err != nil {
		return 0, fmt.Errorf("create yandex direct campaign: %w", err)
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

func (c *Client) DeleteCampaign(ctx context.Context, id int64) error {
	if c.token == "" {
		return errors.New("yandex direct token is required")
	}

	if id == 0 {
		return errors.New("yandex direct campaign id is required")
	}

	requestBody := yandexCampaignDeleteRequest{Method: "delete"}
	requestBody.Params.SelectionCriteria.IDs = []int64{id}

	var result yandexCampaignDeleteResponse
	if err := c.post(ctx, "campaigns", requestBody, &result); err != nil {
		return fmt.Errorf("delete yandex direct campaign: %w", err)
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

func (c *Client) CreateUserProfileAdGroup(ctx context.Context, campaignID int64, name string) (int64, error) {
	requestBody := yandexAdGroupAddRequest{Method: "add"}
	requestBody.Params.AdGroups = []yandexAdGroup{{Name: name, CampaignID: campaignID, RegionIDs: []int64{0}}}

	var result yandexAddResponse
	if err := c.post(ctx, "adgroups", requestBody, &result); err != nil {
		return 0, fmt.Errorf("create yandex direct ad group: %w", err)
	}

	return addResultID(result, "ad group")
}

func (c *Client) CreateDemographicRetargetingList(ctx context.Context, name, gender string, age int) (int64, error) {
	dictionariesRequest := yandexDictionariesGetRequest{Method: "get"}
	dictionariesRequest.Params.DictionaryNames = []string{"AudienceCriteriaTypes", "AudienceDemographicProfiles"}

	var dictionaries yandexDictionariesGetResponse
	if err := c.post(ctx, "dictionaries", dictionariesRequest, &dictionaries); err != nil {
		return 0, fmt.Errorf("get yandex direct demographic profiles: %w", err)
	}

	if len(dictionaries.Result.AudienceDemographicProfiles) == 0 || len(dictionaries.Result.AudienceCriteriaTypes) == 0 {
		return 0, errors.New("yandex direct returned no demographic profiles or criteria types")
	}

	genderID, ageID, err := demographicProfileIDs(dictionaries.Result.AudienceDemographicProfiles, dictionaries.Result.AudienceCriteriaTypes, gender, age)
	if err != nil {
		return 0, err
	}

	requestBody := yandexRetargetingListAddRequest{Method: "add"}
	requestBody.Params.RetargetingLists = []yandexRetargetingList{{
		Type: "AUDIENCE",
		Name: name,
		Rules: []yandexRetargetingRule{
			{Arguments: []yandexRetargetingArgument{{ExternalID: genderID}}, Operator: "ANY"},
			{Arguments: []yandexRetargetingArgument{{ExternalID: ageID}}, Operator: "ANY"},
		},
	}}

	var result yandexAddResponse
	if err := c.post(ctx, "retargetinglists", requestBody, &result); err != nil {
		return 0, fmt.Errorf("create yandex direct retargeting list: %w", err)
	}

	return addResultID(result, "retargeting list")
}

func (c *Client) CreateAudienceTarget(ctx context.Context, adGroupID, retargetingListID int64) (int64, error) {
	requestBody := yandexAudienceTargetAddRequest{Method: "add"}
	requestBody.Params.AudienceTargets = []yandexAudienceTarget{{AdGroupID: adGroupID, RetargetingListID: retargetingListID}}

	var result yandexAddResponse
	if err := c.post(ctx, "audiencetargets", requestBody, &result); err != nil {
		return 0, fmt.Errorf("create yandex direct audience target: %w", err)
	}

	return addResultID(result, "audience target")
}

func (c *Client) DeleteAdGroup(ctx context.Context, id int64) error {
	requestBody := yandexAdGroupDeleteRequest{Method: "delete"}
	requestBody.Params.SelectionCriteria.IDs = []int64{id}
	return c.delete(ctx, "adgroups", requestBody, "ad group")
}

func (c *Client) DeleteRetargetingList(ctx context.Context, id int64) error {
	requestBody := yandexRetargetingListDeleteRequest{Method: "delete"}
	requestBody.Params.SelectionCriteria.IDs = []int64{id}
	return c.delete(ctx, "retargetinglists", requestBody, "retargeting list")
}

func (c *Client) DeleteAudienceTarget(ctx context.Context, id int64) error {
	requestBody := yandexAudienceTargetDeleteRequest{Method: "delete"}
	requestBody.Params.SelectionCriteria.IDs = []int64{id}
	return c.delete(ctx, "audiencetargets", requestBody, "audience target")
}

func (c *Client) Get(ctx context.Context, service, objectName string, options GetOptions) (GetResult, error) {
	params := map[string]any{
		"SelectionCriteria": options.SelectionCriteria,
		"FieldNames":        options.FieldNames,
		"Page": map[string]int{
			"Limit":  options.Page.Limit,
			"Offset": options.Page.Offset,
		},
	}
	for name, fields := range options.AdditionalFields {
		params[name] = fields
	}

	var response yandexListResponse
	if err := c.post(ctx, service, map[string]any{"method": "get", "params": params}, &response); err != nil {
		return GetResult{}, fmt.Errorf("get yandex direct %s: %w", service, err)
	}

	result := GetResult{}
	if rawObjects, ok := response.Result[objectName]; ok {
		if err := json.Unmarshal(rawObjects, &result.Objects); err != nil {
			return GetResult{}, fmt.Errorf("decode yandex direct %s objects: %w", service, err)
		}
	}

	if rawLimitedBy, ok := response.Result["LimitedBy"]; ok {
		var limitedBy int
		if err := json.Unmarshal(rawLimitedBy, &limitedBy); err != nil {
			return GetResult{}, fmt.Errorf("decode yandex direct %s page limit: %w", service, err)
		}

		result.LimitedBy = &limitedBy
	}

	return result, nil
}

func (c *Client) delete(ctx context.Context, service string, requestBody any, name string) error {
	if c.token == "" {
		return errors.New("yandex direct token is required")
	}

	var result yandexDeleteResponse
	if err := c.post(ctx, service, requestBody, &result); err != nil {
		return fmt.Errorf("delete yandex direct %s: %w", name, err)
	}

	if len(result.Result.DeleteResults) != 1 {
		return fmt.Errorf("yandex direct %s deletion response contained %d results", name, len(result.Result.DeleteResults))
	}

	if len(result.Result.DeleteResults[0].Errors) > 0 {
		itemError := result.Result.DeleteResults[0].Errors[0]
		return fmt.Errorf("yandex direct %s deletion failed: code=%d message=%q details=%q", name, itemError.Code, itemError.Message, itemError.Details)
	}

	return nil
}

func addResultID(result yandexAddResponse, name string) (int64, error) {
	if len(result.Result.AddResults) != 1 {
		return 0, fmt.Errorf("yandex direct %s response contained %d results", name, len(result.Result.AddResults))
	}

	item := result.Result.AddResults[0]
	if len(item.Errors) > 0 {
		itemError := item.Errors[0]
		return 0, fmt.Errorf("yandex direct %s creation failed: code=%d message=%q details=%q", name, itemError.Code, itemError.Message, itemError.Details)
	}

	return item.ID, nil
}

func (c *Client) post(ctx context.Context, service string, requestBody, result any) error {
	if c.token == "" {
		return errors.New("yandex direct token is required")
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("marshal yandex direct %s request: %w", service, err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/"+service, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create yandex direct %s request: %w", service, err)
	}

	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Accept-Language", "en")
	request.Header.Set("Content-Type", "application/json; charset=utf-8")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("send yandex direct %s request: %w", service, err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read yandex direct %s response: %w", service, err)
	}

	if err := json.Unmarshal(responseBody, result); err != nil {
		return fmt.Errorf("decode yandex direct %s response: %w", service, err)
	}

	if responseError(result) != nil {
		return responseError(result)
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("yandex direct %s request returned HTTP status %d", service, response.StatusCode)
	}

	return nil
}

func demographicProfileIDs(profiles []yandexDemographicProfile, criteriaTypes []yandexAudienceCriteriaType, gender string, age int) (int64, int64, error) {
	genderName, ok := map[string]string{"женщина": "female", "мужчина": "male"}[gender]
	if !ok {
		return 0, 0, fmt.Errorf("unsupported persona gender %q", gender)
	}

	genderType, ageType := demographicCriteriaTypes(criteriaTypes)
	if genderType == "" || ageType == "" {
		return 0, 0, errors.New("yandex direct returned no gender or age demographic criteria type")
	}

	genderID, ageID := demographicProfileIDsByType(profiles, genderType, ageType, genderName, age)
	if genderID == 0 || ageID == 0 {
		return 0, 0, fmt.Errorf("yandex direct has no demographic profile for gender %q and age %d", gender, age)
	}

	return genderID, ageID, nil
}

func demographicCriteriaTypes(criteriaTypes []yandexAudienceCriteriaType) (string, string) {
	var genderType, ageType string
	for _, criteriaType := range criteriaTypes {
		switch strings.ToLower(criteriaType.Name) {
		case "gender":
			genderType = criteriaType.Type
		case "age":
			ageType = criteriaType.Type
		}
	}

	return genderType, ageType
}

func demographicProfileIDsByType(profiles []yandexDemographicProfile, genderType, ageType, genderName string, age int) (int64, int64) {
	var genderID, ageID int64
	for _, profile := range profiles {
		name := strings.ToLower(profile.Name + " " + profile.Description)
		if genderID == 0 && profile.Type == genderType && matchesGender(name, genderName) {
			genderID = profile.ID
		}

		if ageID == 0 && profile.Type == ageType && profileContainsAge(profile.Name+" "+profile.Description, age) {
			ageID = profile.ID
		}
	}

	return genderID, ageID
}

func responseError(result any) *yandexRequestError {
	switch response := result.(type) {
	case *yandexCampaignAddResponse:
		return response.Error
	case *yandexCampaignDeleteResponse:
		return response.Error
	case *yandexAddResponse:
		return response.Error
	case *yandexDeleteResponse:
		return response.Error
	case *yandexDictionariesGetResponse:
		return response.Error
	case *yandexListResponse:
		return response.Error
	default:
		return nil
	}
}

func matchesGender(value, gender string) bool {
	if gender == "male" && strings.Contains(value, "female") {
		return false
	}

	return strings.Contains(value, gender)
}

func profileContainsAge(value string, age int) bool {
	if parts := ageRangePattern.FindStringSubmatch(value); len(parts) == 3 {
		lower, lowerErr := strconv.Atoi(parts[1])
		upper, upperErr := strconv.Atoi(parts[2])
		if lowerErr == nil && upperErr == nil {
			return age >= lower && age <= upper
		}
	}

	if parts := agePlusPattern.FindStringSubmatch(value); len(parts) == 2 {
		lower, err := strconv.Atoi(parts[1])
		if err == nil {
			return age >= lower
		}
	}

	return false
}
