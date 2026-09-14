package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"app/internal/clients"
	"app/internal/repositories/adcampaign"
	"app/internal/repositories/file"
	"app/internal/repositories/hypothesis"
	"app/internal/repositories/product"
	"app/internal/storage"
)

var (
	ErrAlreadyInProgress   = errors.New("hypothesis already in progress")
	ErrHypothesisNoProduct = errors.New("hypothesis has no product")
	ErrUpdateInProgress    = errors.New("cannot update hypothesis in progress")
	ErrDeleteInProgress    = errors.New("cannot delete hypothesis in progress")
)

type HypothesisService struct {
	hypotheses *hypothesis.Repository
	products   *product.Repository
	files      *file.Repository
	storage    *storage.S3
	campaigns  *adcampaign.Repository
	yandex     *clients.YandexDirectClient
}

func NewHypothesisService(
	hypotheses *hypothesis.Repository,
	products *product.Repository,
	files *file.Repository,
	storage *storage.S3,
	campaigns *adcampaign.Repository,
	yandex *clients.YandexDirectClient,
) *HypothesisService {
	return &HypothesisService{
		hypotheses: hypotheses,
		products:   products,
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
		HypothesisID: id,
		Provider:     adcampaign.ProviderYandex,
		ExternalID:   campaignID,
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
		if deleteErr := s.campaigns.Delete(ctx, id, adcampaign.ProviderYandex); deleteErr != nil {
			return nil, fmt.Errorf("set hypothesis status after creating yandex campaign %d: %w", campaignID, errors.Join(err, fmt.Errorf("delete local ad campaign: %w", deleteErr)))
		}

		return nil, fmt.Errorf("setting status: %w", err)
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

	campaign, err := s.campaigns.GetByHypothesisID(ctx, id, adcampaign.ProviderYandex)
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

	if err := s.campaigns.Delete(ctx, id, adcampaign.ProviderYandex); err != nil {
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
