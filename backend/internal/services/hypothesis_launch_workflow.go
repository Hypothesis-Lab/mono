package services

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/fx"
)

const (
	hypothesisLaunchTaskQueue         = "hypothesis-launch"
	hypothesisLaunchActivityName      = "hypothesis.launch"
	resetHypothesisLaunchActivityName = "hypothesis.reset-launch-status"
)

type hypothesisLaunchInput struct {
	UserID       string
	HypothesisID string
}

type HypothesisLaunchWorker struct {
	worker worker.Worker
}

func NewHypothesisLaunchWorker(lc fx.Lifecycle, temporalClient client.Client, service *HypothesisService) *HypothesisLaunchWorker {
	launchWorker := &HypothesisLaunchWorker{
		worker: worker.New(temporalClient, hypothesisLaunchTaskQueue, worker.Options{}),
	}
	launchWorker.worker.RegisterWorkflow(hypothesisLaunchWorkflow)
	launchWorker.worker.RegisterActivityWithOptions(service.launchHypothesisActivity, activity.RegisterOptions{Name: hypothesisLaunchActivityName})
	launchWorker.worker.RegisterActivityWithOptions(service.resetHypothesisLaunchStatusActivity, activity.RegisterOptions{Name: resetHypothesisLaunchActivityName})

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			return launchWorker.worker.Start()
		},
		OnStop: func(context.Context) error {
			launchWorker.worker.Stop()

			return nil
		},
	})

	return launchWorker
}

func hypothesisLaunchWorkflow(ctx workflow.Context, input hypothesisLaunchInput) error {
	launchCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	})
	err := workflow.ExecuteActivity(launchCtx, hypothesisLaunchActivityName, input).Get(ctx, nil)
	if err == nil {
		return nil
	}

	resetCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumAttempts: 5,
		},
	})
	if resetErr := workflow.ExecuteActivity(resetCtx, resetHypothesisLaunchActivityName, input).Get(ctx, nil); resetErr != nil {
		return fmt.Errorf("reset hypothesis after failed launch: %w", resetErr)
	}

	return fmt.Errorf("launch hypothesis: %w", err)
}
