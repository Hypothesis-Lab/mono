package services

import (
	"context"
	"errors"
	"fmt"

	"app/internal/repositories/file"
	"app/internal/repositories/hypothesis"
	"app/internal/repositories/product"
	"app/internal/storage"
)

var (
	ErrAlreadyInProgress   = errors.New("hypothesis already in progress")
	ErrHypothesisNoProduct = errors.New("hypothesis has no product")
	ErrNotInProgress       = errors.New("hypothesis not in progress")
	ErrUpdateInProgress    = errors.New("cannot update hypothesis in progress")
	ErrDeleteInProgress    = errors.New("cannot delete hypothesis in progress")
)

type HypothesisService struct {
	hypotheses *hypothesis.Repository
	products   *product.Repository
	files      *file.Repository
	storage    *storage.S3
}

func NewHypothesisService(
	hypotheses *hypothesis.Repository,
	products *product.Repository,
	files *file.Repository,
	storage *storage.S3,
) *HypothesisService {
	return &HypothesisService{
		hypotheses: hypotheses,
		products:   products,
		files:      files,
		storage:    storage,
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

	if _, err := s.products.GetByHypothesisID(ctx, id); err != nil {
		if errors.Is(err, product.ErrNotFound) {
			return nil, ErrHypothesisNoProduct
		}

		return nil, fmt.Errorf("getting product: %w", err)
	}

	launched, err := s.hypotheses.SetStatus(ctx, userID, id, hypothesis.StatusInProgress)
	if err != nil {
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

	if h.Status != hypothesis.StatusInProgress {
		return nil, ErrNotInProgress
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
