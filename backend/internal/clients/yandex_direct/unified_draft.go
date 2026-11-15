package yandexdirect

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type imageAddResponse struct {
	Result struct {
		AddResults []struct {
			Hash   string               `json:"AdImageHash"`
			Errors []actionNotification `json:"Errors"`
		} `json:"AddResults"`
	} `json:"result"`
	Error *yandexRequestError `json:"error"`
}

type actionNotification struct {
	Code    int    `json:"Code"`
	Message string `json:"Message"`
	Details string `json:"Details"`
}

type modifierAddResponse struct {
	Result struct {
		AddResults []struct {
			IDs    []int64              `json:"Ids"`
			Errors []actionNotification `json:"Errors"`
		} `json:"AddResults"`
	} `json:"result"`
	Error *yandexRequestError `json:"error"`
}

type campaignSuspendResponse struct {
	Result struct {
		SuspendResults []yandexActionResult `json:"SuspendResults"`
	} `json:"result"`
	Error *yandexRequestError `json:"error"`
}

type responsiveText struct {
	Text string `json:"Text"`
}

type responsiveTitle struct {
	Title string `json:"Title"`
}

type responsiveImage struct {
	ImageHash string `json:"ImageHash"`
}

type campaignServingState struct {
	ID     int64  `json:"Id"`
	State  string `json:"State"`
	Status string `json:"Status"`
}

func (c *Client) v501Endpoint() string {
	if strings.HasSuffix(c.endpoint, "/v5") {
		return strings.TrimSuffix(c.endpoint, "/v5") + "/v501"
	}

	return strings.TrimRight(c.endpoint, "/") + "/v501"
}

func (c *Client) FindUnifiedCampaignByName(ctx context.Context, name string) (int64, bool, error) {
	return c.findByNameAt(ctx, c.v501Endpoint(), "campaigns", "Campaigns", map[string]any{"Types": []string{"UNIFIED_CAMPAIGN"}}, name)
}

func (c *Client) CreateUnifiedDraftCampaign(ctx context.Context, name, startDate string) (int64, error) {
	if name == "" || startDate == "" {
		return 0, errors.New("unified draft campaign name and start date are required")
	}

	body := map[string]any{"method": "add", "params": map[string]any{"Campaigns": []any{map[string]any{
		"Name": name, "StartDate": startDate,
		"UnifiedCampaign": map[string]any{"BiddingStrategy": map[string]any{
			"Search":  map[string]any{"BiddingStrategyType": "HIGHEST_POSITION"},
			"Network": map[string]any{"BiddingStrategyType": "SERVING_OFF"},
		}},
	}}}}
	var response yandexCampaignAddResponse
	if err := c.postAt(ctx, c.v501Endpoint(), "campaigns", body, &response); err != nil {
		return 0, fmt.Errorf("create unified draft campaign: %w", err)
	}

	if len(response.Result.AddResults) != 1 {
		return 0, fmt.Errorf("unified draft campaign response contained %d results", len(response.Result.AddResults))
	}

	item := response.Result.AddResults[0]
	if len(item.Errors) > 0 {
		return 0, fmt.Errorf("create unified draft campaign: code=%d message=%q details=%q", item.Errors[0].Code, item.Errors[0].Message, item.Errors[0].Details)
	}

	if item.ID <= 0 {
		return 0, fmt.Errorf("unified draft campaign response contained invalid ID %d", item.ID)
	}

	return item.ID, nil
}

func (c *Client) SuspendCampaign(ctx context.Context, id int64) error {
	if id <= 0 {
		return errors.New("campaign ID must be positive")
	}

	body := map[string]any{"method": "suspend", "params": map[string]any{"SelectionCriteria": map[string]any{"Ids": []int64{id}}}}
	var response campaignSuspendResponse
	if err := c.postAt(ctx, c.v501Endpoint(), "campaigns", body, &response); err != nil {
		return fmt.Errorf("suspend campaign: %w", err)
	}

	if len(response.Result.SuspendResults) != 1 {
		return fmt.Errorf("suspend campaign response contained %d results", len(response.Result.SuspendResults))
	}

	if len(response.Result.SuspendResults[0].Errors) > 0 {
		item := response.Result.SuspendResults[0].Errors[0]
		return fmt.Errorf("suspend campaign failed: code=%d message=%q details=%q", item.Code, item.Message, item.Details)
	}

	return nil
}

func (s campaignServingState) notServing() bool {
	return s.State == "SUSPENDED" || (s.State == "OFF" && s.Status == "DRAFT")
}

func (c *Client) EnsureCampaignNotServing(ctx context.Context, id int64) error {
	state, err := c.campaignServingState(ctx, id)
	if err != nil {
		return err
	}

	if state.notServing() {
		return nil
	}

	if err := c.SuspendCampaign(ctx, id); err != nil {
		return err
	}

	return c.VerifyCampaignNotServing(ctx, id)
}

func (c *Client) VerifyCampaignNotServing(ctx context.Context, id int64) error {
	state, err := c.campaignServingState(ctx, id)
	if err != nil {
		return err
	}

	if !state.notServing() {
		return fmt.Errorf("campaign %d is not a non-serving draft or suspended (state %q status %q)", id, state.State, state.Status)
	}

	return nil
}

func (c *Client) campaignServingState(ctx context.Context, id int64) (campaignServingState, error) {
	if id <= 0 {
		return campaignServingState{}, errors.New("campaign serving state requires a positive ID")
	}

	result, err := c.getAt(ctx, c.v501Endpoint(), "campaigns", "Campaigns", GetOptions{SelectionCriteria: map[string]any{"Ids": []int64{id}}, FieldNames: []string{"Id", "State", "Status"}, Page: Page{Limit: 2}})
	if err != nil {
		return campaignServingState{}, fmt.Errorf("get campaign serving state: %w", err)
	}

	var item campaignServingState
	if err := decodeOne(result.Objects, &item, "campaign serving state"); err != nil {
		return campaignServingState{}, err
	}

	if item.ID != id {
		return campaignServingState{}, fmt.Errorf("campaign serving state returned ID %d, expected %d", item.ID, id)
	}

	return item, nil
}

func (c *Client) FindUnifiedAdGroup(ctx context.Context, campaignID int64, name string) (int64, bool, error) {
	return c.findByNameAt(ctx, c.v501Endpoint(), "adgroups", "AdGroups", map[string]any{"CampaignIds": []int64{campaignID}}, name)
}

func (c *Client) CreateUnifiedAdGroup(ctx context.Context, campaignID int64, name string, regionIDs []int64) (int64, error) {
	body := map[string]any{"method": "add", "params": map[string]any{"AdGroups": []any{map[string]any{"Name": name, "CampaignId": campaignID, "RegionIds": regionIDs}}}}
	return c.addAt(ctx, c.v501Endpoint(), "adgroups", body, "unified ad group")
}

func (c *Client) FindAdImageByName(ctx context.Context, name string) (string, bool, error) {
	objects, err := c.listAllAt(ctx, c.v501Endpoint(), "adimages", "AdImages", map[string]any{}, []string{"AdImageHash", "Name"}, nil)
	if err != nil {
		return "", false, fmt.Errorf("find ad image by name: %w", err)
	}

	var found string
	for _, raw := range objects {
		var item struct {
			Hash string `json:"AdImageHash"`
			Name string `json:"Name"`
		}

		if err := json.Unmarshal(raw, &item); err != nil {
			return "", false, fmt.Errorf("decode ad image: %w", err)
		}

		if item.Name == name {
			if found != "" {
				return "", false, fmt.Errorf("ad image name %q is not unique", name)
			}

			found = item.Hash
		}

	}

	return found, found != "", nil
}

func (c *Client) CreateAdImage(ctx context.Context, name string, data []byte) (string, error) {
	if name == "" || len(data) == 0 {
		return "", errors.New("ad image name and data are required")
	}

	body := map[string]any{"method": "add", "params": map[string]any{"AdImages": []any{map[string]any{"Name": name, "ImageData": base64.StdEncoding.EncodeToString(data), "Type": "REGULAR"}}}}
	var response imageAddResponse
	if err := c.postAt(ctx, c.v501Endpoint(), "adimages", body, &response); err != nil {
		return "", fmt.Errorf("create ad image: %w", err)
	}

	if len(response.Result.AddResults) != 1 {
		return "", fmt.Errorf("ad image response contained %d results", len(response.Result.AddResults))
	}

	item := response.Result.AddResults[0]
	if len(item.Errors) > 0 {
		return "", fmt.Errorf("create ad image failed: code=%d message=%q details=%q", item.Errors[0].Code, item.Errors[0].Message, item.Errors[0].Details)
	}

	if item.Hash == "" {
		return "", errors.New("ad image response contained an empty hash")
	}

	return item.Hash, nil
}

func (c *Client) FindResponsiveAd(ctx context.Context, groupID int64, imageHash, title, text, href string) (int64, bool, error) {
	objects, err := c.listAllAt(ctx, c.v501Endpoint(), "ads", "Ads", map[string]any{"AdGroupIds": []int64{groupID}}, []string{"Id", "AdGroupId", "Status", "State", "Type"}, map[string][]string{"ResponsiveAdFieldNames": {"Titles", "Texts", "Href", "AdImages"}})
	if err != nil {
		return 0, false, fmt.Errorf("find responsive ad: %w", err)
	}

	var found int64
	for _, raw := range objects {
		var ad struct {
			ID           int64 `json:"Id"`
			AdGroupID    int64 `json:"AdGroupId"`
			ResponsiveAd struct {
				Titles   []responsiveTitle `json:"Titles"`
				Texts    []responsiveText  `json:"Texts"`
				Href     string            `json:"Href"`
				AdImages struct {
					Items []responsiveImage `json:"Items"`
				} `json:"AdImages"`
			} `json:"ResponsiveAd"`
		}

		if err := json.Unmarshal(raw, &ad); err != nil {
			return 0, false, fmt.Errorf("decode responsive ad: %w", err)
		}

		if ad.AdGroupID != groupID || !hasTitle(ad.ResponsiveAd.Titles, title) || !hasText(ad.ResponsiveAd.Texts, text) || ad.ResponsiveAd.Href != href || !hasImageHash(ad.ResponsiveAd.AdImages.Items, imageHash) {
			continue
		}

		if found != 0 {
			return 0, false, errors.New("responsive ad match is not unique")
		}

		found = ad.ID
	}

	return found, found != 0, nil
}

func (c *Client) CreateResponsiveAd(ctx context.Context, groupID int64, imageHash, title, text, href string) (int64, error) {
	responsive := map[string]any{"Titles": []string{title}, "Texts": []string{text}, "Href": href, "AdImageHashes": []string{imageHash}}
	body := map[string]any{"method": "add", "params": map[string]any{"Ads": []any{map[string]any{"AdGroupId": groupID, "ResponsiveAd": responsive}}}}
	return c.addAt(ctx, c.v501Endpoint(), "ads", body, "responsive ad")
}

func (c *Client) VerifyDraftAd(ctx context.Context, id int64) error {
	result, err := c.getAt(ctx, c.v501Endpoint(), "ads", "Ads", GetOptions{SelectionCriteria: map[string]any{"Ids": []int64{id}}, FieldNames: []string{"Id", "Status", "State"}, Page: Page{Limit: 2}})
	if err != nil {
		return fmt.Errorf("verify draft ad %d: %w", id, err)
	}

	var ad struct {
		ID     int64  `json:"Id"`
		Status string `json:"Status"`
		State  string `json:"State"`
	}

	if err := decodeOne(result.Objects, &ad, "draft ad"); err != nil {
		return err
	}

	if ad.ID != id || ad.Status != "DRAFT" || ad.State != "OFF" {
		return fmt.Errorf("ad %d is not an off draft (status=%q state=%q)", id, ad.Status, ad.State)
	}

	return nil
}

func (c *Client) DeleteAd(ctx context.Context, id int64) error {
	return c.deleteAt(ctx, c.v501Endpoint(), "ads", map[string]any{"method": "delete", "params": map[string]any{"SelectionCriteria": map[string]any{"Ids": []int64{id}}}}, "ad")
}

func (c *Client) FindKeyword(ctx context.Context, groupID int64, keyword string) (int64, bool, error) {
	return c.findNamedObject(ctx, "keywords", "Keywords", map[string]any{"AdGroupIds": []int64{groupID}}, "Keyword", keyword)
}

func (c *Client) CreateKeyword(ctx context.Context, groupID int64, keyword string) (int64, error) {
	body := map[string]any{"method": "add", "params": map[string]any{"Keywords": []any{map[string]any{"AdGroupId": groupID, "Keyword": keyword}}}}
	return c.addAt(ctx, c.v501Endpoint(), "keywords", body, "keyword")
}

func (c *Client) DeleteKeyword(ctx context.Context, id int64) error {
	return c.deleteAt(ctx, c.v501Endpoint(), "keywords", map[string]any{"method": "delete", "params": map[string]any{"SelectionCriteria": map[string]any{"Ids": []int64{id}}}}, "keyword")
}

func (c *Client) FindDemographicExclusion(ctx context.Context, groupID int64, gender, age string) (int64, bool, error) {
	return c.findDemographicModifier(ctx, groupID, gender, age)
}

func (c *Client) CreateDemographicExclusion(ctx context.Context, groupID int64, gender, age string) (int64, error) {
	demographics := map[string]any{"Gender": gender, "BidModifier": 0}
	if age != "" {
		demographics["Age"] = age
	}

	body := map[string]any{"method": "add", "params": map[string]any{"BidModifiers": []any{map[string]any{"AdGroupId": groupID, "DemographicsAdjustments": []any{demographics}}}}}
	var response modifierAddResponse
	if err := c.postAt(ctx, c.v501Endpoint(), "bidmodifiers", body, &response); err != nil {
		return 0, fmt.Errorf("create demographic exclusion: %w", err)
	}

	if len(response.Result.AddResults) != 1 {
		return 0, fmt.Errorf("demographic exclusion response contained %d results", len(response.Result.AddResults))
	}

	item := response.Result.AddResults[0]
	if len(item.Errors) > 0 {
		return 0, fmt.Errorf("create demographic exclusion failed: code=%d message=%q details=%q", item.Errors[0].Code, item.Errors[0].Message, item.Errors[0].Details)
	}

	if len(item.IDs) != 1 || item.IDs[0] <= 0 {
		return 0, errors.New("demographic exclusion response did not contain one positive ID")
	}

	return item.IDs[0], nil
}

func (c *Client) DeleteDemographicExclusion(ctx context.Context, id int64) error {
	return c.deleteAt(ctx, c.v501Endpoint(), "bidmodifiers", map[string]any{"method": "delete", "params": map[string]any{"SelectionCriteria": map[string]any{"Ids": []int64{id}}}}, "demographic exclusion")
}

func (c *Client) findDemographicModifier(ctx context.Context, groupID int64, gender, age string) (int64, bool, error) {
	objects, err := c.listAllAt(ctx, c.v501Endpoint(), "bidmodifiers", "BidModifiers",
		map[string]any{"AdGroupIds": []int64{groupID}, "Levels": []string{"AD_GROUP"}, "Types": []string{"DEMOGRAPHICS_ADJUSTMENT"}},
		[]string{"Id", "AdGroupId", "Type"},
		map[string][]string{"DemographicsAdjustmentFieldNames": {"Gender", "Age", "BidModifier", "Enabled"}})
	if err != nil {
		return 0, false, fmt.Errorf("find demographic exclusion: %w", err)
	}

	var found int64
	for _, raw := range objects {
		var item struct {
			ID           int64 `json:"Id"`
			AdGroupID    int64 `json:"AdGroupId"`
			Demographics *struct {
				Gender      string `json:"Gender"`
				Age         string `json:"Age"`
				BidModifier *int   `json:"BidModifier"`
				Enabled     string `json:"Enabled"`
			} `json:"DemographicsAdjustment"`
		}

		if err := json.Unmarshal(raw, &item); err != nil {
			return 0, false, fmt.Errorf("decode demographic modifier: %w", err)
		}

		if item.AdGroupID != groupID || item.Demographics == nil || item.Demographics.Gender != gender || item.Demographics.Age != age {
			continue
		}

		if item.Demographics.BidModifier == nil || *item.Demographics.BidModifier != 0 || item.Demographics.Enabled != "YES" {
			return 0, false, fmt.Errorf("demographic modifier %d exists but is not an enabled zero-bid exclusion", item.ID)
		}

		if found != 0 {
			return 0, false, errors.New("demographic exclusion match is not unique")
		}

		found = item.ID
	}

	return found, found != 0, nil
}

func (c *Client) findNamedObject(ctx context.Context, service, objectName string, criteria map[string]any, field, value string) (int64, bool, error) {
	objects, err := c.listAllAt(ctx, c.v501Endpoint(), service, objectName, criteria, []string{"Id", "AdGroupId", field}, nil)
	if err != nil {
		return 0, false, fmt.Errorf("find %s: %w", strings.TrimSuffix(objectName, "s"), err)
	}

	var found int64
	for _, raw := range objects {
		var item map[string]json.RawMessage
		if err := json.Unmarshal(raw, &item); err != nil {
			return 0, false, fmt.Errorf("decode %s: %w", objectName, err)
		}

		var id int64
		var got string
		if err := json.Unmarshal(item["Id"], &id); err != nil {
			return 0, false, fmt.Errorf("decode %s ID: %w", objectName, err)
		}

		if err := json.Unmarshal(item[field], &got); err != nil {
			return 0, false, fmt.Errorf("decode %s value: %w", objectName, err)
		}

		if got == value {
			if found != 0 {
				return 0, false, fmt.Errorf("%s match is not unique", objectName)
			}

			found = id
		}

	}

	return found, found != 0, nil
}

func (c *Client) findByNameAt(ctx context.Context, endpoint, service, objectName string, criteria map[string]any, name string) (int64, bool, error) {
	return c.findByCriteriaAt(ctx, endpoint, service, objectName, criteria, "Name", name)
}

func (c *Client) findByCriteriaAt(ctx context.Context, endpoint, service, objectName string, criteria map[string]any, field, value string) (int64, bool, error) {
	objects, err := c.listAllAt(ctx, endpoint, service, objectName, criteria, []string{"Id", field}, nil)
	if err != nil {
		return 0, false, fmt.Errorf("find yandex direct %s: %w", objectName, err)
	}

	var found int64
	for _, raw := range objects {
		var item map[string]json.RawMessage
		if err := json.Unmarshal(raw, &item); err != nil {
			return 0, false, fmt.Errorf("decode %s: %w", objectName, err)
		}

		var id int64
		var got string
		if err := json.Unmarshal(item["Id"], &id); err != nil {
			return 0, false, err
		}

		if err := json.Unmarshal(item[field], &got); err != nil {
			return 0, false, err
		}

		if got == value {
			if found != 0 {
				return 0, false, fmt.Errorf("%s value %q is not unique", objectName, value)
			}

			found = id
		}

	}

	return found, found != 0, nil
}

func (c *Client) listAllAt(ctx context.Context, endpoint, service, objectName string, criteria map[string]any, fields []string, additional map[string][]string) ([]json.RawMessage, error) {
	var objects []json.RawMessage
	offset := 0
	for {
		result, err := c.getAt(ctx, endpoint, service, objectName, GetOptions{
			SelectionCriteria: criteria,
			FieldNames:        fields,
			AdditionalFields:  additional,
			Page:              Page{Limit: 1000, Offset: offset},
		})
		if err != nil {
			return nil, err
		}

		objects = append(objects, result.Objects...)
		if result.LimitedBy == nil {
			return objects, nil
		}

		if *result.LimitedBy <= offset {
			return nil, fmt.Errorf("%s pagination did not advance beyond offset %d", service, offset)
		}

		offset = *result.LimitedBy
	}
}

func (c *Client) addAt(ctx context.Context, endpoint, service string, body any, name string) (int64, error) {
	var result yandexAddResponse
	if err := c.postAt(ctx, endpoint, service, body, &result); err != nil {
		return 0, fmt.Errorf("create yandex direct %s: %w", name, err)
	}

	return addResultID(result, name)
}

func (c *Client) deleteAt(ctx context.Context, endpoint, service string, body any, name string) error {
	var result yandexDeleteResponse
	if err := c.postAt(ctx, endpoint, service, body, &result); err != nil {
		return fmt.Errorf("delete yandex direct %s: %w", name, err)
	}

	if len(result.Result.DeleteResults) != 1 {
		return fmt.Errorf("yandex direct %s deletion response contained %d results", name, len(result.Result.DeleteResults))
	}

	if len(result.Result.DeleteResults[0].Errors) > 0 {
		e := result.Result.DeleteResults[0].Errors[0]
		return fmt.Errorf("yandex direct %s deletion failed: code=%d message=%q details=%q", name, e.Code, e.Message, e.Details)
	}

	return nil
}

func decodeOne(objects []json.RawMessage, target any, name string) error {
	if len(objects) != 1 {
		return fmt.Errorf("%s response contained %d objects", name, len(objects))
	}

	if err := json.Unmarshal(objects[0], target); err != nil {
		return fmt.Errorf("decode %s: %w", name, err)
	}

	return nil
}

func hasTitle(values []responsiveTitle, value string) bool {
	for _, item := range values {
		if item.Title == value {
			return true
		}
	}

	return false
}

func hasText(values []responsiveText, value string) bool {
	for _, item := range values {
		if item.Text == value {
			return true
		}

	}

	return false
}

func hasImageHash(values []responsiveImage, value string) bool {
	for _, item := range values {
		if item.ImageHash == value {
			return true
		}
	}

	return false
}
