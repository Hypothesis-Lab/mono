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

type HypothesisLaunchStarter struct {
	launches *hypothesislaunch.Repository
	temporal client.Client
	logger   *zap.Logger
	cancel   context.CancelFunc
	done     chan struct{}
}

func NewHypothesisLaunchStarter(lc fx.Lifecycle, launches *hypothesislaunch.Repository, temporal client.Client, logger *zap.Logger) *HypothesisLaunchStarter {
	starter := &HypothesisLaunchStarter{
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

func (s *HypothesisLaunchStarter) run(ctx context.Context) {
	defer close(s.done)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		if err := s.startNext(ctx); err != nil && ctx.Err() == nil {
			s.logger.Error("start queued hypothesis launch", zap.Error(err))
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *HypothesisLaunchStarter) startNext(ctx context.Context) error {
	item, err := s.launches.ClaimOutbox(ctx)
	if errors.Is(err, hypothesislaunch.ErrOutboxEmpty) {
		return nil
	}

	if err != nil {
		return err
	}

	resources, err := s.launches.WorkflowResources(ctx, item.LaunchID)
	if err != nil {
		return s.release(ctx, item.ID, err)
	}

	input := hypothesisLaunchInput{
		UserID:       item.UserID,
		HypothesisID: item.HypothesisID,
		LaunchID:     item.LaunchID,
		Resources:    resources,
	}
	_, err = s.temporal.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                    item.WorkflowID,
		TaskQueue:             hypothesisLaunchTaskQueue,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
		Memo: map[string]interface{}{
			"operation":      "launch",
			"hypothesis_id":  item.HypothesisID,
			"launch_id":      item.LaunchID,
			"resource_count": len(resources),
		},
	}, hypothesisLaunchWorkflow, input)
	if err == nil {
		return s.launches.MarkOutboxDispatched(ctx, item.ID)
	}

	var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &alreadyStarted) {
		return s.launches.MarkOutboxDispatched(ctx, item.ID)
	}

	if _, describeErr := s.temporal.DescribeWorkflowExecution(ctx, item.WorkflowID, ""); describeErr == nil {
		return s.launches.MarkOutboxDispatched(ctx, item.ID)
	}

	return s.release(ctx, item.ID, fmt.Errorf("start hypothesis launch workflow: %w", err))
}

func (s *HypothesisLaunchStarter) release(ctx context.Context, outboxID string, cause error) error {
	if err := s.launches.ReleaseOutbox(ctx, outboxID, cause); err != nil {
		return errors.Join(cause, err)
	}

	return cause
}
