package services

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net/url"
	"strings"

	"app/internal/repositories/audience"
	"app/internal/repositories/hypothesislaunch"
)

const unifiedDraftFormat = "unified_draft"

type unifiedDraftManifest struct {
	Format     string                 `json:"format"`
	LandingURL string                 `json:"landing_url"`
	Audiences  []unifiedDraftAudience `json:"audiences"`
}

type unifiedDraftAudience struct {
	Rank    int              `json:"rank"`
	Keyword string           `json:"keyword"`
	Ads     []unifiedDraftAd `json:"ads"`
}

type unifiedDraftAd struct {
	Title       string `json:"title"`
	Text        string `json:"text"`
	ImageBase64 string `json:"image_base64"`
}

type unifiedDraftAdSnapshot struct {
	Title      string `json:"title"`
	Text       string `json:"text"`
	LandingURL string `json:"landing_url"`
	ImageKey   string `json:"image_key"`
}

type demographicExclusion struct {
	gender string
	age    int
	bucket string
}

func buildUnifiedDraftPlan(raw json.RawMessage, audiences []*audience.Audience, startDate string) ([]hypothesislaunch.ResourcePlan, error) {
	manifest, err := decodeUnifiedDraftManifest(raw)
	if err != nil {
		return nil, err
	}

	sourceByRank, err := indexUnifiedDraftAudiences(manifest, audiences)
	if err != nil {
		return nil, err
	}

	return planUnifiedDraftResources(manifest, sourceByRank, startDate)
}

func decodeUnifiedDraftManifest(raw json.RawMessage) (*unifiedDraftManifest, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var manifest unifiedDraftManifest
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decode unified draft preparation config: %w", err)
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("unified draft preparation config contains trailing data")
	}

	if manifest.Format != unifiedDraftFormat {
		return nil, fmt.Errorf("unsupported product preparation format %q", manifest.Format)
	}

	landingURL, err := url.ParseRequestURI(manifest.LandingURL)
	if err != nil || (landingURL.Scheme != "http" && landingURL.Scheme != "https") || landingURL.Host == "" {
		return nil, fmt.Errorf("unified draft landing_url must be an absolute HTTP(S) URL")
	}

	return &manifest, nil
}

func indexUnifiedDraftAudiences(manifest *unifiedDraftManifest, audiences []*audience.Audience) (map[int]*audience.Audience, error) {
	if len(manifest.Audiences) != 3 || len(audiences) != 3 {
		return nil, fmt.Errorf("unified draft requires exactly three manifest audiences and three source audiences")
	}

	sourceByRank := make(map[int]*audience.Audience, len(audiences))
	for _, source := range audiences {
		if source.Rank < 1 || source.Rank > 3 {
			return nil, fmt.Errorf("source audience %s has unsupported rank %d", source.ID, source.Rank)
		}

		if sourceByRank[source.Rank] != nil {
			return nil, fmt.Errorf("source audience rank %d is duplicated", source.Rank)
		}

		sourceByRank[source.Rank] = source
	}

	return sourceByRank, nil
}

func planUnifiedDraftResources(manifest *unifiedDraftManifest, sourceByRank map[int]*audience.Audience, startDate string) ([]hypothesislaunch.ResourcePlan, error) {
	plans := make([]hypothesislaunch.ResourcePlan, 0, 44)
	campaignPayload, err := json.Marshal(struct {
		Format     string `json:"format"`
		LandingURL string `json:"landing_url"`
	}{Format: manifest.Format, LandingURL: manifest.LandingURL})
	if err != nil {
		return nil, fmt.Errorf("snapshot unified draft manifest: %w", err)
	}

	plans = append(plans,
		hypothesislaunch.ResourcePlan{Kind: hypothesislaunch.ResourceCampaign, LogicalKey: "campaign", Position: 0, Name: stringPointer("{campaign_name}"), StartDate: stringPointer(startDate), Payload: campaignPayload},
		hypothesislaunch.ResourcePlan{Kind: hypothesislaunch.ResourceCampaignSuspend, LogicalKey: "campaign_suspend", Position: 1, Name: stringPointer("{campaign_name}")},
	)
	imageResources := make(map[string]struct{})
	var imagePlans, adPlans []hypothesislaunch.ResourcePlan
	for _, item := range manifest.Audiences {
		if item.Rank < 1 || item.Rank > 3 || sourceByRank[item.Rank] == nil {
			return nil, fmt.Errorf("manifest audience rank %d does not match a source audience", item.Rank)
		}

		setup, audienceImages, audienceAds, err := planUnifiedDraftAudience(item, sourceByRank[item.Rank], manifest.LandingURL, imageResources)
		if err != nil {
			return nil, err
		}

		delete(sourceByRank, item.Rank)
		plans = append(plans, setup...)
		imagePlans = append(imagePlans, audienceImages...)
		adPlans = append(adPlans, audienceAds...)
	}

	if len(sourceByRank) != 0 {
		return nil, fmt.Errorf("manifest is missing a source audience rank")
	}

	setupPlans := append([]hypothesislaunch.ResourcePlan(nil), plans[2:]...)
	plans = append([]hypothesislaunch.ResourcePlan(nil), plans[:2]...)
	position := 2
	for _, plan := range imagePlans {
		plan.Position = position
		position++
		plans = append(plans, plan)
	}

	for _, plan := range setupPlans {
		plan.Position = position
		position++
		plans = append(plans, plan)
	}

	for _, plan := range adPlans {
		plan.Position = position
		position++
		plans = append(plans, plan)
	}

	return plans, nil
}

func planUnifiedDraftAudience(item unifiedDraftAudience, source *audience.Audience, landingURL string, imageResources map[string]struct{}) ([]hypothesislaunch.ResourcePlan, []hypothesislaunch.ResourcePlan, []hypothesislaunch.ResourcePlan, error) {
	if strings.TrimSpace(item.Keyword) == "" || len([]rune(item.Keyword)) > 4096 {
		return nil, nil, nil, fmt.Errorf("manifest audience rank %d has invalid keyword", item.Rank)
	}

	if len(item.Ads) != 3 {
		return nil, nil, nil, fmt.Errorf("manifest audience rank %d requires exactly three ads", item.Rank)
	}

	gender, age, err := audienceDemographics(source)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("validate source audience rank %d demographics: %w", item.Rank, err)
	}

	groupKey := fmt.Sprintf("rank:%d", item.Rank)
	groupName := fmt.Sprintf("{campaign_name}-rank-%d", item.Rank)
	groupPayload, err := marshalPreparationPayload(map[string]int{"rank": item.Rank})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("snapshot unified ad group rank %d: %w", item.Rank, err)
	}

	setup := []hypothesislaunch.ResourcePlan{{Kind: hypothesislaunch.ResourceUnifiedAdGroup, LogicalKey: groupKey, Name: stringPointer(groupName), Payload: groupPayload}}
	for _, exclusion := range demographicExclusions(gender, age) {
		key := fmt.Sprintf("rank:%d:exclude:%s:%s", item.Rank, exclusion.gender, exclusion.bucket)
		payload, err := marshalPreparationPayload(map[string]string{"age_bucket": providerAgeBucket(exclusion.bucket)})
		if err != nil {
			return nil, nil, nil, fmt.Errorf("snapshot demographic exclusion %s: %w", key, err)
		}

		setup = append(setup, hypothesislaunch.ResourcePlan{Kind: hypothesislaunch.ResourceDemographicExclusion, LogicalKey: key, Gender: stringPointer(providerGender(exclusion.gender)), Age: intPointer(exclusion.age), Payload: payload})
	}

	keywordPayload, err := marshalPreparationPayload(map[string]string{"keyword": item.Keyword})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("snapshot keyword for rank %d: %w", item.Rank, err)
	}

	setup = append(setup, hypothesislaunch.ResourcePlan{Kind: hypothesislaunch.ResourceKeyword, LogicalKey: groupKey, Payload: keywordPayload})
	var images, ads []hypothesislaunch.ResourcePlan
	for index, ad := range item.Ads {
		imageKey, imageLogicalKey, imagePayload, err := planUnifiedDraftImage(item.Rank, index+1, ad, imageResources)
		if err != nil {
			return nil, nil, nil, err
		}

		if imagePayload != nil {
			images = append(images, hypothesislaunch.ResourcePlan{Kind: hypothesislaunch.ResourceAdImage, LogicalKey: imageLogicalKey, Name: stringPointer("hyp-" + imageKey), Payload: imagePayload})
		}

		adPayload, err := marshalPreparationPayload(unifiedDraftAdSnapshot{Title: ad.Title, Text: ad.Text, LandingURL: landingURL, ImageKey: imageLogicalKey})
		if err != nil {
			return nil, nil, nil, fmt.Errorf("snapshot responsive ad rank %d ad %d: %w", item.Rank, index+1, err)
		}

		ads = append(ads, hypothesislaunch.ResourcePlan{Kind: hypothesislaunch.ResourceResponsiveAd, LogicalKey: fmt.Sprintf("rank:%d:ad:%d", item.Rank, index+1), Name: stringPointer(groupName), Payload: adPayload})
	}

	return setup, images, ads, nil
}

func planUnifiedDraftImage(rank, adIndex int, ad unifiedDraftAd, images map[string]struct{}) (string, string, json.RawMessage, error) {
	if strings.TrimSpace(ad.Title) == "" || len([]rune(ad.Title)) > 56 || strings.TrimSpace(ad.Text) == "" || len([]rune(ad.Text)) > 81 {
		return "", "", nil, fmt.Errorf("manifest audience rank %d ad %d has invalid title or text length", rank, adIndex)
	}

	imageBytes, err := base64.StdEncoding.DecodeString(ad.ImageBase64)
	if err != nil || len(imageBytes) == 0 || len(imageBytes) > 5*1024*1024 {
		return "", "", nil, fmt.Errorf("manifest audience rank %d ad %d has invalid or oversized PNG image", rank, adIndex)
	}

	config, err := png.DecodeConfig(bytes.NewReader(imageBytes))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 4096 || config.Height > 4096 {
		return "", "", nil, fmt.Errorf("manifest audience rank %d ad %d image must be a decodable PNG within 4096x4096", rank, adIndex)
	}

	hash := sha256.Sum256(imageBytes)
	imageKey := hex.EncodeToString(hash[:])
	logicalKey := "image:" + imageKey
	if _, exists := images[imageKey]; exists {
		return imageKey, logicalKey, nil, nil
	}

	payload, err := marshalPreparationPayload(struct {
		ImageBase64   string `json:"image_base64"`
		ContentSHA256 string `json:"content_sha256"`
	}{ImageBase64: ad.ImageBase64, ContentSHA256: imageKey})
	if err != nil {
		return "", "", nil, fmt.Errorf("snapshot image for audience rank %d ad %d: %w", rank, adIndex, err)
	}

	images[imageKey] = struct{}{}
	return imageKey, logicalKey, payload, nil
}

func marshalPreparationPayload(value any) (json.RawMessage, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	return encoded, nil
}

func demographicExclusions(gender string, desiredAge int) []demographicExclusion {
	opposite := "женщина"
	if gender == "женщина" {
		opposite = "мужчина"
	}

	result := []demographicExclusion{{gender: opposite, age: 0, bucket: "all"}}
	buckets := []struct {
		age   int
		label string
	}{{0, "0-17"}, {18, "18-24"}, {25, "25-34"}, {35, "35-44"}, {45, "45-54"}, {55, "55+"}}
	desired, _ := demographicAgeBucket(desiredAge)
	for _, bucket := range buckets {
		if bucket.label != desired {
			result = append(result, demographicExclusion{gender: gender, age: bucket.age, bucket: bucket.label})
		}
	}

	return result
}

func audienceDemographics(source *audience.Audience) (string, int, error) {
	var persona struct {
		Gender string `json:"gender"`
		Age    int    `json:"age"`
	}
	if err := json.Unmarshal(source.Persona, &persona); err != nil {
		return "", 0, fmt.Errorf("decode persona: %w", err)
	}

	if persona.Gender != "женщина" && persona.Gender != "мужчина" {
		return "", 0, fmt.Errorf("unsupported gender %q", persona.Gender)
	}

	if _, ok := demographicAgeBucket(persona.Age); !ok {
		return "", 0, fmt.Errorf("unsupported age %d", persona.Age)
	}

	return persona.Gender, persona.Age, nil
}

func demographicAgeBucket(age int) (string, bool) {
	switch {
	case age >= 0 && age <= 17:
		return "0-17", true
	case age >= 18 && age <= 24:
		return "18-24", true
	case age >= 25 && age <= 34:
		return "25-34", true
	case age >= 35 && age <= 44:
		return "35-44", true
	case age >= 45 && age <= 54:
		return "45-54", true
	case age >= 55 && age <= 100:
		return "55+", true
	default:
		return "", false
	}
}

func providerGender(gender string) string {
	if gender == "женщина" {
		return "GENDER_FEMALE"
	}

	return "GENDER_MALE"
}

func providerAgeBucket(bucket string) string {
	return map[string]string{
		"all":   "",
		"0-17":  "AGE_0_17",
		"18-24": "AGE_18_24",
		"25-34": "AGE_25_34",
		"35-44": "AGE_35_44",
		"45-54": "AGE_45_54",
		"55+":   "AGE_55",
	}[bucket]
}

func stringPointer(value string) *string { return &value }

func intPointer(value int) *int { return &value }
