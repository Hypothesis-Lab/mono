package services

import (
	"context"
	"fmt"
	"time"

	"app/internal/clients"
	"app/internal/repositories/audience"
	"app/internal/services/pipelines"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

type generatorPipeline interface {
	Kind() string
	Persist(context.Context, *clients.CompletedJob) error
}

type GeneratorWorker struct {
	generator *clients.GeneratorClient
	pipelines []generatorPipeline
	logger    *zap.Logger
	cancel    context.CancelFunc
	done      chan struct{}
}

func NewGeneratorWorker(lc fx.Lifecycle, generator *clients.GeneratorClient, audiences *audience.Repository, logger *zap.Logger) *GeneratorWorker {
	worker := &GeneratorWorker{
		generator: generator,
		pipelines: []generatorPipeline{pipelines.NewAudiencePipeline(audiences)},
		logger:    logger,
		done:      make(chan struct{}),
	}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			workerCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
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

func (w *GeneratorWorker) run(ctx context.Context) {
	defer close(w.done)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		if err := w.sync(ctx); err != nil && ctx.Err() == nil {
			w.logger.Error("sync generator jobs", zap.Error(err))
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *GeneratorWorker) sync(ctx context.Context) error {
	for _, pipeline := range w.pipelines {
		job, err := w.generator.NextCompletedJob(ctx, pipeline.Kind())
		if err != nil {
			return fmt.Errorf("get completed %s generator job: %w", pipeline.Kind(), err)
		}

		if job == nil {
			continue
		}

		if job.Kind != pipeline.Kind() {
			return fmt.Errorf("generator returned kind %q for %s job", job.Kind, pipeline.Kind())
		}

		if err := pipeline.Persist(ctx, job); err != nil {
			return fmt.Errorf("persist completed %s generator job: %w", pipeline.Kind(), err)
		}

		if err := w.generator.MarkJobPersisted(ctx, job.ID); err != nil {
			return fmt.Errorf("acknowledge persisted %s generator job: %w", pipeline.Kind(), err)
		}
	}

	return nil
}
