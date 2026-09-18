package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"app/internal/clients/yandex_direct"
	"app/internal/repositories/adcampaign"
	"app/internal/repositories/audience"
	"app/internal/repositories/file"
	"app/internal/repositories/hypothesis"
	"app/internal/repositories/product"
	"app/internal/storage"
)

const russiaYandexRegionID int64 = 225

var (
	ErrAlreadyInProgress   = errors.New("hypothesis already in progress")
	ErrHypothesisNoProduct = errors.New("hypothesis has no product")
	ErrUpdateInProgress    = errors.New("cannot update hypothesis in progress")
	ErrDeleteInProgress    = errors.New("cannot delete hypothesis in progress")
)

type HypothesisService struct {
	hypotheses *hypothesis.Repository
	products   *product.Repository
	audiences  *audience.Repository
	files      *file.Repository
	storage    *storage.S3
	campaigns  *adcampaign.Repository
	yandex     *yandexdirect.Client
}

type createdAudienceTargeting struct {
	adGroupID         int64
	retargetingListID int64
	audienceTargetID  int64
}

func NewHypothesisService(
	hypotheses *hypothesis.Repository,
	products *product.Repository,
	audiences *audience.Repository,
	files *file.Repository,
	storage *storage.S3,
	campaigns *adcampaign.Repository,
	yandex *yandexdirect.Client,
) *HypothesisService {
	return &HypothesisService{
		hypotheses: hypotheses,
		products:   products,
		audiences:  audiences,
		files:      files,
		storage:    storage,
		campaigns:  campaigns,
		yandex:     yandex,
	}
}

func (s *HypothesisService) Create(ctx context.Context, userID, description string) (*hypothesis.Hypothesis, error) {
	created, err := s.hypotheses.Create(ctx, hypothesis.CreateInput{
		UserID:      userID,
		Description: description,
	})
	if err != nil {
		return nil, fmt.Errorf("creating hypothesis: %w", err)
	}

	return created, nil
}

func (s *HypothesisService) List(ctx context.Context, userID string) ([]*hypothesis.Hypothesis, error) {
	items, err := s.hypotheses.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing hypotheses: %w", err)
	}

	return items, nil
}

func (s *HypothesisService) Get(ctx context.Context, userID, id string) (*hypothesis.Hypothesis, error) {
	h, err := s.hypotheses.Get(ctx, userID, id)
	if err != nil {
		if errors.Is(err, hypothesis.ErrNotFound) {
			return nil, hypothesis.ErrNotFound
		}

		return nil, fmt.Errorf("getting hypothesis: %w", err)
	}

	return h, nil
}

func (s *HypothesisService) GetProduct(ctx context.Context, userID, id string) (*product.Product, error) {
	if _, err := s.hypotheses.Get(ctx, userID, id); err != nil {
		if errors.Is(err, hypothesis.ErrNotFound) {
			return nil, hypothesis.ErrNotFound
		}

		return nil, fmt.Errorf("getting hypothesis: %w", err)
	}

	p, err := s.products.GetByHypothesisID(ctx, id)
	if err != nil {
		if errors.Is(err, product.ErrNotFound) {
			return nil, product.ErrNotFound
		}

		return nil, fmt.Errorf("getting product: %w", err)
	}

	return p, nil
}

func (s *HypothesisService) Launch(ctx context.Context, userID, id string) (*hypothesis.Hypothesis, error) {
	h, err := s.hypotheses.Get(ctx, userID, id)
	if err != nil {
		if errors.Is(err, hypothesis.ErrNotFound) {
			return nil, hypothesis.ErrNotFound
		}

		return nil, fmt.Errorf("getting hypothesis: %w", err)
	}

	if h.Status == hypothesis.StatusInProgress {
		return nil, ErrAlreadyInProgress
	}

	p, err := s.products.GetByHypothesisID(ctx, id)
	if err != nil {
		if errors.Is(err, product.ErrNotFound) {
			return nil, ErrHypothesisNoProduct
		}

		return nil, fmt.Errorf("getting product: %w", err)
	}

	campaignID, err := s.yandex.CreateCampaign(ctx, p.Name, time.Now().Format(time.DateOnly))
	if err != nil {
		return nil, fmt.Errorf("create yandex campaign: %w", err)
	}

	if err := s.campaigns.Create(ctx, adcampaign.Campaign{
		ProductID:  p.ID,
		Provider:   adcampaign.ProviderYandex,
		ExternalID: campaignID,
	}); err != nil {
		if deleteErr := s.yandex.DeleteCampaign(ctx, campaignID); deleteErr != nil {
			return nil, fmt.Errorf("store yandex campaign %d: %w", campaignID, errors.Join(err, fmt.Errorf("delete created yandex campaign: %w", deleteErr)))
		}

		return nil, fmt.Errorf("store yandex campaign %d: %w", campaignID, err)
	}

	launched, err := s.hypotheses.SetStatus(ctx, userID, id, hypothesis.StatusInProgress)
	if err != nil {
		if deleteErr := s.yandex.DeleteCampaign(ctx, campaignID); deleteErr != nil {
			return nil, fmt.Errorf("set hypothesis status after creating yandex campaign %d: %w", campaignID, errors.Join(err, fmt.Errorf("delete yandex campaign: %w", deleteErr)))
		}

		if deleteErr := s.campaigns.Delete(ctx, p.ID, adcampaign.ProviderYandex); deleteErr != nil {
			return nil, fmt.Errorf("set hypothesis status after creating yandex campaign %d: %w", campaignID, errors.Join(err, fmt.Errorf("delete local ad campaign: %w", deleteErr)))
		}

		return nil, fmt.Errorf("setting status: %w", err)
	}

	createdTargeting, err := s.createAudienceTargeting(ctx, p, campaignID)
	if err != nil {
		return nil, s.rollbackLaunch(ctx, userID, id, p.ID, campaignID, createdTargeting, err)
	}

	return launched, nil
}

func (s *HypothesisService) Stop(ctx context.Context, userID, id string) (*hypothesis.Hypothesis, error) {
	h, err := s.hypotheses.Get(ctx, userID, id)
	if err != nil {
		if errors.Is(err, hypothesis.ErrNotFound) {
			return nil, hypothesis.ErrNotFound
		}

		return nil, fmt.Errorf("getting hypothesis: %w", err)
	}

	if h.Status == hypothesis.StatusIdle {
		return h, nil
	}

	p, err := s.products.GetByHypothesisID(ctx, id)
	if err != nil {
		if errors.Is(err, product.ErrNotFound) {
			return s.hypotheses.SetStatus(ctx, userID, id, hypothesis.StatusIdle)
		}

		return nil, fmt.Errorf("getting product: %w", err)
	}

	campaign, err := s.campaigns.GetByProductID(ctx, p.ID, adcampaign.ProviderYandex)
	if err != nil {
		if errors.Is(err, adcampaign.ErrNotFound) {
			stopped, err := s.hypotheses.SetStatus(ctx, userID, id, hypothesis.StatusIdle)
			if err != nil {
				return nil, fmt.Errorf("set status without yandex campaign: %w", err)
			}

			return stopped, nil
		}

		return nil, fmt.Errorf("getting yandex campaign: %w", err)
	}

	if err := s.yandex.DeleteCampaign(ctx, campaign.ExternalID); err != nil {
		return nil, fmt.Errorf("delete yandex campaign: %w", err)
	}

	if err := s.campaigns.Delete(ctx, p.ID, adcampaign.ProviderYandex); err != nil {
		return nil, fmt.Errorf("delete local yandex campaign: %w", err)
	}

	stopped, err := s.hypotheses.SetStatus(ctx, userID, id, hypothesis.StatusIdle)
	if err != nil {
		return nil, fmt.Errorf("setting status: %w", err)
	}

	return stopped, nil
}

func (s *HypothesisService) Update(ctx context.Context, userID, id string, in hypothesis.UpdateInput) (*hypothesis.Hypothesis, error) {
	h, err := s.hypotheses.Get(ctx, userID, id)
	if err != nil {
		if errors.Is(err, hypothesis.ErrNotFound) {
			return nil, hypothesis.ErrNotFound
		}

		return nil, fmt.Errorf("getting hypothesis: %w", err)
	}

	if h.Status == hypothesis.StatusInProgress {
		return nil, ErrUpdateInProgress
	}

	updated, err := s.hypotheses.Update(ctx, userID, id, in)
	if err != nil {
		if errors.Is(err, hypothesis.ErrNotFound) {
			return nil, hypothesis.ErrNotFound
		}

		return nil, fmt.Errorf("updating hypothesis: %w", err)
	}

	return updated, nil
}

func (s *HypothesisService) Delete(ctx context.Context, userID, id string) error {
	h, err := s.hypotheses.Get(ctx, userID, id)
	if err != nil {
		if errors.Is(err, hypothesis.ErrNotFound) {
			return hypothesis.ErrNotFound
		}

		return fmt.Errorf("getting hypothesis: %w", err)
	}

	if h.Status == hypothesis.StatusInProgress {
		return ErrDeleteInProgress
	}

	files, err := s.files.ListByHypothesisID(ctx, id)
	if err != nil {
		return fmt.Errorf("listing files: %w", err)
	}

	for _, f := range files {
		if err := s.storage.Delete(ctx, f.S3Key); err != nil {
			return fmt.Errorf("deleting file: %w", err)
		}
	}

	if err := s.hypotheses.Delete(ctx, userID, id); err != nil {
		return fmt.Errorf("deleting hypothesis: %w", err)
	}

	return nil
}

func (s *HypothesisService) createAudienceTargeting(ctx context.Context, p *product.Product, campaignID int64) ([]createdAudienceTargeting, error) {
	createdTargeting := []createdAudienceTargeting{}
	audiences, err := s.audiences.ListByProductID(ctx, p.ID)
	if err != nil {
		return createdTargeting, fmt.Errorf("list product audiences: %w", err)
	}

	if len(audiences) == 0 {
		return createdTargeting, errors.New("product has no generated audiences")
	}

	for _, audience := range audiences {
		var persona struct {
			Gender string `json:"gender"`
			Age    int    `json:"age"`
		}
		if err := json.Unmarshal(audience.Persona, &persona); err != nil {
			return createdTargeting, fmt.Errorf("decode audience %s persona: %w", audience.ID, err)
		}

		retargetingListID, err := s.yandex.CreateDemographicRetargetingList(ctx, "product-"+p.ID+"-"+audience.PersonaID, persona.Gender, persona.Age)
		if err != nil {
			return createdTargeting, fmt.Errorf("create retargeting list for audience %s: %w", audience.ID, err)
		}

		createdTargeting = append(createdTargeting, createdAudienceTargeting{retargetingListID: retargetingListID})

		adGroupID, err := s.yandex.CreateUserProfileAdGroup(ctx, campaignID, "product-"+p.ID+"-"+audience.PersonaID, []int64{russiaYandexRegionID})
		if err != nil {
			return createdTargeting, fmt.Errorf("create ad group for audience %s: %w", audience.ID, err)
		}

		createdTargeting[len(createdTargeting)-1].adGroupID = adGroupID

		audienceTargetID, err := s.yandex.CreateAudienceTarget(ctx, adGroupID, retargetingListID)
		if err != nil {
			return createdTargeting, fmt.Errorf("create audience target for audience %s: %w", audience.ID, err)
		}

		createdTargeting[len(createdTargeting)-1].audienceTargetID = audienceTargetID
	}

	return createdTargeting, nil
}

func (s *HypothesisService) rollbackLaunch(ctx context.Context, userID, hypothesisID, productID string, campaignID int64, targetings []createdAudienceTargeting, launchErr error) error {
	for index := len(targetings) - 1; index >= 0; index-- {
		targeting := targetings[index]
		launchErr = deleteAudienceTargeting(ctx, s.yandex, targeting, launchErr)
	}

	if deleteErr := s.yandex.DeleteCampaign(ctx, campaignID); deleteErr != nil {
		return fmt.Errorf("%w; delete yandex campaign: %w", launchErr, deleteErr)
	}

	if deleteErr := s.campaigns.Delete(ctx, productID, adcampaign.ProviderYandex); deleteErr != nil {
		return fmt.Errorf("%w; delete local campaign: %w", launchErr, deleteErr)
	}

	if _, err := s.hypotheses.SetStatus(ctx, userID, hypothesisID, hypothesis.StatusIdle); err != nil {
		return fmt.Errorf("%w; reset hypothesis status: %w", launchErr, err)
	}

	return launchErr
}

func deleteAudienceTargeting(ctx context.Context, yandex *yandexdirect.Client, targeting createdAudienceTargeting, launchErr error) error {
	if targeting.audienceTargetID != 0 {
		if err := yandex.DeleteAudienceTarget(ctx, targeting.audienceTargetID); err != nil {
			launchErr = fmt.Errorf("%w; delete audience target: %w", launchErr, err)
		}
	}

	if targeting.adGroupID != 0 {
		if err := yandex.DeleteAdGroup(ctx, targeting.adGroupID); err != nil {
			launchErr = fmt.Errorf("%w; delete ad group: %w", launchErr, err)
		}
	}

	if targeting.retargetingListID != 0 {
		if err := yandex.DeleteRetargetingList(ctx, targeting.retargetingListID); err != nil {
			launchErr = fmt.Errorf("%w; delete retargeting list: %w", launchErr, err)
		}
	}

	return launchErr
}
