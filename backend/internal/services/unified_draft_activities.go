package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"app/internal/clients/yandex_direct"
	"app/internal/repositories/hypothesislaunch"
)

func campaignPreparationFormat(payload json.RawMessage) (string, error) {
	if len(payload) == 0 || strings.TrimSpace(string(payload)) == "{}" {
		return "", nil
	}

	var manifest unifiedDraftManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return "", err
	}

	return manifest.Format, nil
}

func (s *HypothesisService) findOrCreateUnifiedCampaign(ctx context.Context, yandex *yandexdirect.Client, resource *hypothesislaunch.Resource) (int64, bool, error) {
	if resource.Name == nil || resource.StartDate == nil {
		return 0, false, fmt.Errorf("unified draft campaign resource %s has incomplete launch data", resource.ID)
	}

	id, found, err := yandex.FindUnifiedCampaignByName(ctx, *resource.Name)
	if err != nil {
		return 0, false, fmt.Errorf("find unified draft campaign resource %s: %w", resource.ID, err)
	}

	if found {
		return id, false, nil
	}

	if err := s.prepareHypothesisLaunchResourceCreate(ctx, resource); err != nil {
		return 0, false, err
	}

	id, err = yandex.CreateUnifiedDraftCampaign(ctx, *resource.Name, *resource.StartDate)
	if err != nil {
		return 0, false, fmt.Errorf("create unified draft campaign resource %s: %w", resource.ID, err)
	}

	return id, true, nil
}

func (s *HypothesisService) findOrCreateUnifiedDraftResource(ctx context.Context, yandex *yandexdirect.Client, resource *hypothesislaunch.Resource) (int64, bool, error) {
	switch resource.Kind {
	case hypothesislaunch.ResourceCampaignSuspend:
		return s.suspendUnifiedDraftCampaign(ctx, yandex, resource)
	case hypothesislaunch.ResourceUnifiedAdGroup:
		return s.findOrCreateUnifiedAdGroup(ctx, yandex, resource)
	case hypothesislaunch.ResourceDemographicExclusion:
		return s.findOrCreateDemographicExclusion(ctx, yandex, resource)
	case hypothesislaunch.ResourceKeyword:
		return s.findOrCreateKeyword(ctx, yandex, resource)
	case hypothesislaunch.ResourceResponsiveAd:
		return s.findOrCreateResponsiveAd(ctx, yandex, resource)
	default:
		return 0, false, fmt.Errorf("unsupported unified draft resource kind %q", resource.Kind)
	}
}

func (s *HypothesisService) suspendUnifiedDraftCampaign(ctx context.Context, yandex *yandexdirect.Client, resource *hypothesislaunch.Resource) (int64, bool, error) {
	campaign, err := s.launches.GetResourceByKey(ctx, resource.LaunchID, hypothesislaunch.ResourceCampaign, "campaign")
	if err != nil {
		return 0, false, fmt.Errorf("get campaign dependency for suspension resource %s: %w", resource.ID, err)
	}

	if campaign.ExternalID == nil {
		return 0, false, fmt.Errorf("campaign dependency for suspension resource %s has no external id", resource.ID)
	}

	if resource.Status == "pending" {
		if err := s.launches.MarkResourceCreating(ctx, resource.ID); err != nil {
			return 0, false, fmt.Errorf("mark campaign suspension resource %s creating: %w", resource.ID, err)
		}
	} else if resource.Status != "creating" {
		return 0, false, fmt.Errorf("campaign suspension resource %s cannot run from status %q", resource.ID, resource.Status)
	}

	if campaign.OwnedByLaunch == nil {
		return 0, false, fmt.Errorf("campaign %d ownership is unknown before suspension", *campaign.ExternalID)
	}

	if *campaign.OwnedByLaunch {
		if err := yandex.EnsureCampaignNotServing(ctx, *campaign.ExternalID); err != nil {
			return 0, false, fmt.Errorf("ensure unified draft campaign %d is not serving: %w", *campaign.ExternalID, err)
		}
	} else if err := yandex.VerifyCampaignNotServing(ctx, *campaign.ExternalID); err != nil {
		return 0, false, fmt.Errorf("verify reused campaign %d is not serving: %w", *campaign.ExternalID, err)
	}

	return *campaign.ExternalID, false, nil
}

func (s *HypothesisService) findOrCreateUnifiedAdGroup(ctx context.Context, yandex *yandexdirect.Client, resource *hypothesislaunch.Resource) (int64, bool, error) {
	if resource.Name == nil {
		return 0, false, fmt.Errorf("unified ad group resource %s has no name", resource.ID)
	}

	campaignID, err := s.launches.DependencyExternalID(ctx, resource.LaunchID, hypothesislaunch.ResourceCampaign, "campaign")
	if err != nil {
		return 0, false, fmt.Errorf("get campaign dependency for unified ad group %s: %w", resource.ID, err)
	}

	id, found, err := yandex.FindUnifiedAdGroup(ctx, campaignID, *resource.Name)
	if err != nil {
		return 0, false, fmt.Errorf("find unified ad group resource %s: %w", resource.ID, err)
	}

	if found {
		return id, false, nil
	}

	if err := s.prepareHypothesisLaunchResourceCreate(ctx, resource); err != nil {
		return 0, false, err
	}

	id, err = yandex.CreateUnifiedAdGroup(ctx, campaignID, *resource.Name, []int64{russiaYandexRegionID})
	if err != nil {
		return 0, false, fmt.Errorf("create unified ad group resource %s: %w", resource.ID, err)
	}

	return id, true, nil
}

func (s *HypothesisService) findOrCreateDemographicExclusion(ctx context.Context, yandex *yandexdirect.Client, resource *hypothesislaunch.Resource) (int64, bool, error) {
	var payload struct {
		AgeBucket string `json:"age_bucket"`
	}
	if err := json.Unmarshal(resource.Payload, &payload); err != nil {
		return 0, false, fmt.Errorf("decode demographic exclusion resource %s: %w", resource.ID, err)
	}

	if resource.Gender == nil {
		return 0, false, fmt.Errorf("demographic exclusion resource %s has no gender", resource.ID)
	}

	groupID, err := s.launches.DependencyExternalID(ctx, resource.LaunchID, hypothesislaunch.ResourceUnifiedAdGroup, exclusionGroupKey(resource.LogicalKey))
	if err != nil {
		return 0, false, fmt.Errorf("get ad group dependency for demographic exclusion %s: %w", resource.ID, err)
	}

	id, found, err := yandex.FindDemographicExclusion(ctx, groupID, *resource.Gender, payload.AgeBucket)
	if err != nil {
		return 0, false, fmt.Errorf("find demographic exclusion resource %s: %w", resource.ID, err)
	}

	if found {
		return id, false, nil
	}

	if err := s.prepareHypothesisLaunchResourceCreate(ctx, resource); err != nil {
		return 0, false, err
	}

	id, err = yandex.CreateDemographicExclusion(ctx, groupID, *resource.Gender, payload.AgeBucket)
	if err != nil {
		return 0, false, fmt.Errorf("create demographic exclusion resource %s: %w", resource.ID, err)
	}

	return id, true, nil
}

func (s *HypothesisService) findOrCreateKeyword(ctx context.Context, yandex *yandexdirect.Client, resource *hypothesislaunch.Resource) (int64, bool, error) {
	var payload struct {
		Keyword string `json:"keyword"`
	}
	if err := json.Unmarshal(resource.Payload, &payload); err != nil {
		return 0, false, fmt.Errorf("decode keyword resource %s: %w", resource.ID, err)
	}

	if payload.Keyword == "" {
		return 0, false, fmt.Errorf("keyword resource %s has empty keyword", resource.ID)
	}

	groupID, err := s.launches.DependencyExternalID(ctx, resource.LaunchID, hypothesislaunch.ResourceUnifiedAdGroup, resource.LogicalKey)
	if err != nil {
		return 0, false, fmt.Errorf("get ad group dependency for keyword resource %s: %w", resource.ID, err)
	}

	id, found, err := yandex.FindKeyword(ctx, groupID, payload.Keyword)
	if err != nil {
		return 0, false, fmt.Errorf("find keyword resource %s: %w", resource.ID, err)
	}

	if found {
		return id, false, nil
	}

	if err := s.prepareHypothesisLaunchResourceCreate(ctx, resource); err != nil {
		return 0, false, err
	}

	id, err = yandex.CreateKeyword(ctx, groupID, payload.Keyword)
	if err != nil {
		return 0, false, fmt.Errorf("create keyword resource %s: %w", resource.ID, err)
	}

	return id, true, nil
}

func (s *HypothesisService) findOrCreateResponsiveAd(ctx context.Context, yandex *yandexdirect.Client, resource *hypothesislaunch.Resource) (int64, bool, error) {
	var payload unifiedDraftAdSnapshot
	if err := json.Unmarshal(resource.Payload, &payload); err != nil {
		return 0, false, fmt.Errorf("decode responsive ad resource %s: %w", resource.ID, err)
	}

	groupID, err := s.launches.DependencyExternalID(ctx, resource.LaunchID, hypothesislaunch.ResourceUnifiedAdGroup, adGroupKey(resource.LogicalKey))
	if err != nil {
		return 0, false, fmt.Errorf("get ad group dependency for responsive ad resource %s: %w", resource.ID, err)
	}

	imageHash, err := s.launches.DependencyExternalKey(ctx, resource.LaunchID, hypothesislaunch.ResourceAdImage, payload.ImageKey)
	if err != nil {
		return 0, false, fmt.Errorf("get image dependency for responsive ad resource %s: %w", resource.ID, err)
	}

	id, found, err := yandex.FindResponsiveAd(ctx, groupID, imageHash, payload.Title, payload.Text, payload.LandingURL)
	if err != nil {
		return 0, false, fmt.Errorf("find responsive ad resource %s: %w", resource.ID, err)
	}

	if !found {
		if err := s.prepareHypothesisLaunchResourceCreate(ctx, resource); err != nil {
			return 0, false, err
		}

		id, err = yandex.CreateResponsiveAd(ctx, groupID, imageHash, payload.Title, payload.Text, payload.LandingURL)
		if err != nil {
			return 0, false, fmt.Errorf("create responsive ad resource %s: %w", resource.ID, err)
		}
	}

	if err := yandex.VerifyDraftAd(ctx, id); err != nil {
		return 0, false, fmt.Errorf("verify responsive ad resource %s is draft: %w", resource.ID, err)
	}

	return id, !found, nil
}

func (s *HypothesisService) findOrCreateAdImage(ctx context.Context, yandex *yandexdirect.Client, resource *hypothesislaunch.Resource) (string, bool, error) {
	var payload struct {
		ImageBase64 string `json:"image_base64"`
	}
	if err := json.Unmarshal(resource.Payload, &payload); err != nil {
		return "", false, fmt.Errorf("decode ad image resource %s: %w", resource.ID, err)
	}

	if resource.Name == nil {
		return "", false, fmt.Errorf("ad image resource %s has no name", resource.ID)
	}

	hash, found, err := yandex.FindAdImageByName(ctx, *resource.Name)
	if err != nil {
		return "", false, fmt.Errorf("find ad image resource %s: %w", resource.ID, err)
	}

	if found {
		return hash, false, nil
	}

	if err := s.prepareHypothesisLaunchResourceCreate(ctx, resource); err != nil {
		return "", false, err
	}

	imageData, err := base64.StdEncoding.DecodeString(payload.ImageBase64)
	if err != nil {
		return "", false, fmt.Errorf("decode ad image bytes for resource %s: %w", resource.ID, err)
	}

	hash, created, err := yandex.UploadAdImageTracked(ctx, *resource.Name, imageData)
	if err != nil {
		return "", false, fmt.Errorf("create ad image resource %s: %w", resource.ID, err)
	}

	return hash, created, nil
}

func exclusionGroupKey(logicalKey string) string {
	return audienceGroupKey(logicalKey)
}

func adGroupKey(logicalKey string) string {
	return audienceGroupKey(logicalKey)
}

func audienceGroupKey(logicalKey string) string {
	first := strings.IndexByte(logicalKey, ':')
	if first < 0 {
		return logicalKey
	}

	second := strings.IndexByte(logicalKey[first+1:], ':')
	if second < 0 {
		return logicalKey
	}

	return logicalKey[:first+1+second]
}
