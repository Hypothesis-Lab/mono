package services

import (
	"context"
	"fmt"
	"time"

	"app/internal/clients"
	"app/internal/repositories/audience"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

type AudienceGenerationWorker struct {
	generator *clients.GeneratorClient
	audiences *audience.Repository
	logger    *zap.Logger
	cancel    context.CancelFunc
	done      chan struct{}
}

func NewAudienceGenerationWorker(lc fx.Lifecycle, generator *clients.GeneratorClient, audiences *audience.Repository, logger *zap.Logger) *AudienceGenerationWorker {
	worker := &AudienceGenerationWorker{
		generator: generator,
		audiences: audiences,
		logger:    logger,
		done:      make(chan struct{}),
	}
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			workerCtx, cancel := context.WithCancel(context.Background())
			worker.cancel = cancel
			go worker.run(workerCtx)

			return nil
		},
		OnStop: func(ctx context.Context) error {
			worker.cancel()
			select {
			case <-worker.done:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})

	return worker
}

func (w *AudienceGenerationWorker) run(ctx context.Context) {
	defer close(w.done)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		if err := w.sync(ctx); err != nil && ctx.Err() == nil {
			w.logger.Error("sync generated audiences", zap.Error(err))
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *AudienceGenerationWorker) sync(ctx context.Context) error {
	job, err := w.generator.NextCompletedAudienceGeneration(ctx)
	if err != nil {
		return fmt.Errorf("get completed audience generation: %w", err)
	}
	if job == nil {
		return nil
	}

	inputs := make([]audience.GeneratedInput, 0, len(job.Audiences))
	for _, generated := range job.Audiences {
		inputs = append(inputs, audience.GeneratedInput{
			PersonaID:  generated.PersonaID,
			Persona:    generated.Persona,
			Research:   generated.Research,
			Metrics:    generated.Metrics,
			Hypotheses: generated.Hypotheses,
			Rank:       generated.Rank,
			Score:      generated.Score,
		})
	}
	if err := w.audiences.PersistGenerated(ctx, job.ID, job.ProductID, inputs); err != nil {
		return fmt.Errorf("persist generated audiences: %w", err)
	}
	if err := w.generator.MarkAudienceGenerationPersisted(ctx, job.ID); err != nil {
		return fmt.Errorf("acknowledge persisted audience generation: %w", err)
	}

	return nil
}
