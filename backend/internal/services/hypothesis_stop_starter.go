package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"app/internal/repositories/hypothesislaunch"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type HypothesisStopStarter struct {
	launches *hypothesislaunch.Repository
	temporal client.Client
	logger   *zap.Logger
	cancel   context.CancelFunc
	done     chan struct{}
}

func NewHypothesisStopStarter(lc fx.Lifecycle, launches *hypothesislaunch.Repository, temporal client.Client, logger *zap.Logger) *HypothesisStopStarter {
	starter := &HypothesisStopStarter{
		launches: launches,
		temporal: temporal,
		logger:   logger,
		done:     make(chan struct{}),
	}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			starterCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
			starter.cancel = cancel
			go starter.run(starterCtx)

			return nil
		},
		OnStop: func(ctx context.Context) error {
			starter.cancel()
			select {
			case <-starter.done:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})

	return starter
}

func (s *HypothesisStopStarter) run(ctx context.Context) {
	defer close(s.done)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		if err := s.startNext(ctx); err != nil && ctx.Err() == nil {
			s.logger.Error("start queued hypothesis stop", zap.Error(err))
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *HypothesisStopStarter) startNext(ctx context.Context) error {
	item, err := s.launches.ClaimStopOutbox(ctx)
	if errors.Is(err, hypothesislaunch.ErrStopOutboxEmpty) {
		return nil
	}

	if err != nil {
		return err
	}

	resources, err := s.launches.StopWorkflowResources(ctx, item.StopID)
	if err != nil {
		return s.release(ctx, item.ID, err)
	}

	input := hypothesisStopInput{
		UserID:         item.UserID,
		HypothesisID:   item.HypothesisID,
		StopID:         item.StopID,
		SourceLaunchID: item.SourceLaunchID,
		Resources:      resources,
	}
	_, err = s.temporal.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                    item.WorkflowID,
		TaskQueue:             hypothesisLaunchTaskQueue,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
		Memo: map[string]interface{}{
			"operation":        "stop",
			"hypothesis_id":    item.HypothesisID,
			"stop_id":          item.StopID,
			"source_launch_id": item.SourceLaunchID,
			"resource_count":   len(resources),
		},
	}, hypothesisStopWorkflow, input)
	if err == nil {
		return s.launches.MarkStopOutboxDispatched(ctx, item.ID)
	}

	var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &alreadyStarted) {
		return s.launches.MarkStopOutboxDispatched(ctx, item.ID)
	}

	if _, describeErr := s.temporal.DescribeWorkflowExecution(ctx, item.WorkflowID, ""); describeErr == nil {
		return s.launches.MarkStopOutboxDispatched(ctx, item.ID)
	}

	return s.release(ctx, item.ID, fmt.Errorf("start hypothesis stop workflow: %w", err))
}

func (s *HypothesisStopStarter) release(ctx context.Context, outboxID string, cause error) error {
	if err := s.launches.ReleaseStopOutbox(ctx, outboxID, cause); err != nil {
		return errors.Join(cause, err)
	}

	return cause
}
