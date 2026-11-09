package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"app/internal/repositories/productanalysis"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type ProductAnalysisStarter struct {
	runs     *productanalysis.Repository
	temporal client.Client
	logger   *zap.Logger
	cancel   context.CancelFunc
	done     chan struct{}
}

func NewProductAnalysisStarter(lc fx.Lifecycle, runs *productanalysis.Repository, temporalClient client.Client, logger *zap.Logger) *ProductAnalysisStarter {
	s := &ProductAnalysisStarter{runs: runs, temporal: temporalClient, logger: logger, done: make(chan struct{})}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			starterCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
			s.cancel = cancel
			go s.run(starterCtx)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			s.cancel()
			select {
			case <-s.done:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
	return s
}

func (s *ProductAnalysisStarter) Continue(ctx context.Context, runID, stage string) error {
	workflowID := "product-analysis-" + runID
	err := s.temporal.SignalWorkflow(ctx, workflowID, "", productAnalysisContinueSignal, stage)
	if err == nil {
		return nil
	}

	var notFound *serviceerror.NotFound
	if !errors.As(err, &notFound) {
		return fmt.Errorf("signal product analysis workflow %s to continue %s: %w", runID, stage, err)
	}

	_, err = s.temporal.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                    workflowID,
		TaskQueue:             productAnalysisTaskQueue,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
	}, productAnalysisWorkflow, runID)
	if err != nil {
		return fmt.Errorf("restart product analysis workflow %s to continue %s: %w", runID, stage, err)
	}

	if err := s.temporal.SignalWorkflow(ctx, workflowID, "", productAnalysisContinueSignal, stage); err != nil {
		return fmt.Errorf("signal restarted product analysis workflow %s to continue %s: %w", runID, stage, err)
	}

	return nil
}

func (s *ProductAnalysisStarter) run(ctx context.Context) {
	defer close(s.done)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if err := s.startQueued(ctx); err != nil && ctx.Err() == nil {
			s.logger.Error("start queued product analysis", zap.Error(err))
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *ProductAnalysisStarter) startQueued(ctx context.Context) error {
	ids, err := s.runs.QueuedIDs(ctx)
	if err != nil {
		return fmt.Errorf("list product analysis runs for workflow dispatch: %w", err)
	}

	for _, id := range ids {
		_, err := s.temporal.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID:                    "product-analysis-" + id,
			TaskQueue:             productAnalysisTaskQueue,
			WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
		}, productAnalysisWorkflow, id)
		var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
		if err != nil && !errors.As(err, &alreadyStarted) {
			return fmt.Errorf("start product analysis workflow %s: %w", id, err)
		}
	}

	return nil
}
