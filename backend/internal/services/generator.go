package services

import (
	"context"
	"fmt"
	"time"

	"app/internal/clients"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

type GeneratorPipeline interface {
	Kind() string
	Persist(context.Context, *clients.CompletedJob) error
}

type GeneratorWorkerParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Generator *clients.GeneratorClient
	Pipelines []GeneratorPipeline `group:"generator-pipelines"`
	Logger    *zap.Logger
}

type GeneratorWorker struct {
	generator *clients.GeneratorClient
	pipelines []GeneratorPipeline
	logger    *zap.Logger
	cancel    context.CancelFunc
	done      chan struct{}
}

func NewGeneratorWorker(params GeneratorWorkerParams) *GeneratorWorker {
	worker := &GeneratorWorker{
		generator: params.Generator,
		pipelines: params.Pipelines,
		logger:    params.Logger,
		done:      make(chan struct{}),
	}
	params.Lifecycle.Append(fx.Hook{
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
