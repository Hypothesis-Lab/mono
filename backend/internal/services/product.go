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

type ProductService struct {
	products   *product.Repository
	hypotheses *hypothesis.Repository
	files      *file.Repository
	storage    *storage.S3
}

func NewProductService(
	products *product.Repository,
	hypotheses *hypothesis.Repository,
	files *file.Repository,
	storage *storage.S3,
) *ProductService {
	return &ProductService{
		products:   products,
		hypotheses: hypotheses,
		files:      files,
		storage:    storage,
	}
}

func (s *ProductService) Create(ctx context.Context, userID string, in product.CreateInput) (*product.Product, error) {
	if _, err := s.hypotheses.Get(ctx, userID, in.HypothesisID); err != nil {
		if errors.Is(err, hypothesis.ErrNotFound) {
			return nil, hypothesis.ErrNotFound
		}

		return nil, fmt.Errorf("getting hypothesis: %w", err)
	}

	created, err := s.products.Create(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("creating product: %w", err)
	}

	return created, nil
}

func (s *ProductService) Update(ctx context.Context, userID, id string, in product.UpdateInput) (*product.Product, error) {
	existing, err := s.products.Get(ctx, userID, id)
	if err != nil {
		if errors.Is(err, product.ErrNotFound) {
			return nil, product.ErrNotFound
		}

		return nil, fmt.Errorf("getting product: %w", err)
	}

	if in.Category == nil {
		if in.CustomCategory != nil && existing.Category != "other" {
			return nil, product.ErrCustomCategoryNotAllowed
		}
	} else if *in.Category == "other" {
		if existing.Category != "other" && in.CustomCategory == nil {
			return nil, product.ErrCustomCategoryRequired
		}
	} else {
		if in.CustomCategory != nil {
			return nil, product.ErrCustomCategoryNotAllowed
		}
		in.ClearCustomCategory = true
	}

	updated, err := s.products.Update(ctx, userID, id, in)
	if err != nil {
		if errors.Is(err, product.ErrNotFound) {
			return nil, product.ErrNotFound
		}

		return nil, fmt.Errorf("updating product: %w", err)
	}

	return updated, nil
}

func (s *ProductService) Delete(ctx context.Context, userID, id string) error {
	p, err := s.products.Get(ctx, userID, id)
	if err != nil {
		if errors.Is(err, product.ErrNotFound) {
			return product.ErrNotFound
		}

		return fmt.Errorf("getting product: %w", err)
	}

	files, err := s.files.ListByHypothesisID(ctx, p.HypothesisID)
	if err != nil {
		return fmt.Errorf("listing files: %w", err)
	}

	for _, f := range files {
		if err := s.storage.Delete(ctx, f.S3Key); err != nil {
			return fmt.Errorf("deleting file: %w", err)
		}
	}

	if err := s.files.DeleteByHypothesisID(ctx, p.HypothesisID); err != nil {
		return fmt.Errorf("deleting file records: %w", err)
	}

	if err := s.products.Delete(ctx, userID, id); err != nil {
		return fmt.Errorf("deleting product: %w", err)
	}

	return nil
}
