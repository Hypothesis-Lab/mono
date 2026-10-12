package services

import (
	"context"
	"fmt"
	"time"

	"app/internal/repositories/hypothesislaunch"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/fx"
)

const (
	hypothesisLaunchTaskQueue                              = "hypothesis-launch"
	hypothesisLaunchActivityName                           = "hypothesis.launch"
	resetHypothesisLaunchActivityName                      = "hypothesis.reset-launch-status"
	markHypothesisLaunchRunningActivityName                = "hypothesis.launch.mark-running"
	ensureHypothesisLaunchResourceActivityName             = "hypothesis.launch.ensure-resource"
	deleteHypothesisLaunchResourceActivityName             = "hypothesis.launch.delete-resource"
	completeHypothesisLaunchActivityName                   = "hypothesis.launch.complete"
	failHypothesisLaunchActivityName                       = "hypothesis.launch.fail"
	requireHypothesisLaunchReconciliationActivityName      = "hypothesis.launch.require-reconciliation"
	markHypothesisStopRunningActivityName                  = "hypothesis.stop.mark-running"
	completeHypothesisStopActivityName                     = "hypothesis.stop.complete"
	requireHypothesisStopManualActivityName                = "hypothesis.stop.require-manual-intervention"
	hypothesisLaunchWorkflowChangeID                       = "hypothesis-launch-resource-ledger"
	hypothesisLaunchWorkflowMetadataChangeID               = "hypothesis-launch-resource-metadata"
	hypothesisStopWorkflowMetadataChangeID                 = "hypothesis-stop-resource-metadata"
	createHypothesisLaunchResourceActivityName             = "hypothesis.launch.create-resource"
	deleteHypothesisLaunchResourceWithMetadataActivityName = "hypothesis.launch.delete-resource-with-metadata"
)

type hypothesisLaunchInput struct {
	UserID       string
	HypothesisID string
	LaunchID     string
	ResourceIDs  []string
	Resources    []hypothesislaunch.WorkflowResource
}

type hypothesisStopInput struct {
	UserID         string
	HypothesisID   string
	StopID         string
	SourceLaunchID string
	ResourceIDs    []string
	Resources      []hypothesislaunch.WorkflowResource
}

type HypothesisLaunchWorker struct {
	worker worker.Worker
}

func NewHypothesisLaunchWorker(lc fx.Lifecycle, temporalClient client.Client, service *HypothesisService) *HypothesisLaunchWorker {
	launchWorker := &HypothesisLaunchWorker{
		worker: worker.New(temporalClient, hypothesisLaunchTaskQueue, worker.Options{}),
	}
	launchWorker.worker.RegisterWorkflow(hypothesisLaunchWorkflow)
	launchWorker.worker.RegisterWorkflow(hypothesisStopWorkflow)
	launchWorker.worker.RegisterActivityWithOptions(service.launchHypothesisActivity, activity.RegisterOptions{Name: hypothesisLaunchActivityName})
	launchWorker.worker.RegisterActivityWithOptions(service.resetHypothesisLaunchStatusActivity, activity.RegisterOptions{Name: resetHypothesisLaunchActivityName})
	launchWorker.worker.RegisterActivityWithOptions(service.markHypothesisLaunchRunningActivity, activity.RegisterOptions{Name: markHypothesisLaunchRunningActivityName})
	launchWorker.worker.RegisterActivityWithOptions(service.ensureHypothesisLaunchResourceActivity, activity.RegisterOptions{Name: ensureHypothesisLaunchResourceActivityName})
	launchWorker.worker.RegisterActivityWithOptions(service.deleteHypothesisLaunchResourceActivity, activity.RegisterOptions{Name: deleteHypothesisLaunchResourceActivityName})
	launchWorker.worker.RegisterActivityWithOptions(service.createHypothesisLaunchResourceActivity, activity.RegisterOptions{Name: createHypothesisLaunchResourceActivityName})
	launchWorker.worker.RegisterActivityWithOptions(service.deleteHypothesisLaunchResourceActivityWithMetadata, activity.RegisterOptions{Name: deleteHypothesisLaunchResourceWithMetadataActivityName})
	launchWorker.worker.RegisterActivityWithOptions(service.completeHypothesisLaunchActivity, activity.RegisterOptions{Name: completeHypothesisLaunchActivityName})
	launchWorker.worker.RegisterActivityWithOptions(service.failHypothesisLaunchActivity, activity.RegisterOptions{Name: failHypothesisLaunchActivityName})
	launchWorker.worker.RegisterActivityWithOptions(service.requireHypothesisLaunchReconciliationActivity, activity.RegisterOptions{Name: requireHypothesisLaunchReconciliationActivityName})
	launchWorker.worker.RegisterActivityWithOptions(service.markHypothesisStopRunningActivity, activity.RegisterOptions{Name: markHypothesisStopRunningActivityName})
	launchWorker.worker.RegisterActivityWithOptions(service.completeHypothesisStopActivity, activity.RegisterOptions{Name: completeHypothesisStopActivityName})
	launchWorker.worker.RegisterActivityWithOptions(service.requireHypothesisStopManualActivity, activity.RegisterOptions{Name: requireHypothesisStopManualActivityName})

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
	version := workflow.GetVersion(ctx, hypothesisLaunchWorkflowChangeID, workflow.DefaultVersion, 1)
	if version == workflow.DefaultVersion {
		return legacyHypothesisLaunchWorkflow(ctx, input)
	}

	stateCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumAttempts: 5,
		},
	})
	if err := workflow.ExecuteActivity(stateCtx, markHypothesisLaunchRunningActivityName, input.LaunchID).Get(ctx, nil); err != nil {
		return fmt.Errorf("mark hypothesis launch running: %w", err)
	}

	resourceCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumInterval: 5 * time.Second,
			MaximumAttempts: 3,
		},
	})
	metadataVersion := workflow.GetVersion(ctx, hypothesisLaunchWorkflowMetadataChangeID, workflow.DefaultVersion, 1)
	if metadataVersion == workflow.DefaultVersion {
		created := make([]string, 0, len(input.ResourceIDs))
		for _, resourceID := range input.ResourceIDs {
			if err := workflow.ExecuteActivity(resourceCtx, ensureHypothesisLaunchResourceActivityName, resourceID).Get(ctx, nil); err != nil {
				return failHypothesisLaunchWorkflow(ctx, stateCtx, input.LaunchID, created, err, true)
			}

			created = append(created, resourceID)
		}

		if err := workflow.ExecuteActivity(stateCtx, completeHypothesisLaunchActivityName, input.LaunchID).Get(ctx, nil); err != nil {
			return failHypothesisLaunchWorkflow(ctx, stateCtx, input.LaunchID, created, err, false)
		}

		return nil
	}

	created := make([]hypothesislaunch.WorkflowResource, 0, len(input.Resources))
	for _, resource := range input.Resources {
		activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			ActivityID:          resourceActivityID("launch-create", resource),
			StartToCloseTimeout: 5 * time.Minute,
			RetryPolicy: &temporal.RetryPolicy{
				InitialInterval: time.Second,
				MaximumInterval: 5 * time.Second,
				MaximumAttempts: 3,
			},
		})
		if err := workflow.ExecuteActivity(activityCtx, createHypothesisLaunchResourceActivityName, resource).Get(ctx, nil); err != nil {
			return failHypothesisLaunchWorkflowWithMetadata(ctx, stateCtx, input.LaunchID, created, err, true)
		}

		created = append(created, resource)
	}

	if err := workflow.ExecuteActivity(stateCtx, completeHypothesisLaunchActivityName, input.LaunchID).Get(ctx, nil); err != nil {
		return failHypothesisLaunchWorkflowWithMetadata(ctx, stateCtx, input.LaunchID, created, err, false)
	}

	return nil
}

func legacyHypothesisLaunchWorkflow(ctx workflow.Context, input hypothesisLaunchInput) error {
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

func failHypothesisLaunchWorkflow(ctx workflow.Context, stateCtx workflow.Context, launchID string, created []string, launchErr error, requiresReconciliation bool) error {
	if requiresReconciliation {
		if err := workflow.ExecuteActivity(stateCtx, requireHypothesisLaunchReconciliationActivityName, launchID, launchErr.Error(), true).Get(ctx, nil); err != nil {
			return fmt.Errorf("launch hypothesis: %w; mark reconciliation required: %w", launchErr, err)
		}

		return fmt.Errorf("launch hypothesis requires reconciliation: %w", launchErr)
	}

	deleteCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumAttempts: 5,
		},
	})
	for index := len(created) - 1; index >= 0; index-- {
		if err := workflow.ExecuteActivity(deleteCtx, deleteHypothesisLaunchResourceActivityName, created[index]).Get(ctx, nil); err != nil {
			reconciliationErr := workflow.ExecuteActivity(stateCtx, requireHypothesisLaunchReconciliationActivityName, launchID, launchErr.Error(), false).Get(ctx, nil)
			if reconciliationErr != nil {
				return fmt.Errorf("launch hypothesis: %w; compensate resource: %w; mark reconciliation required: %w", launchErr, err, reconciliationErr)
			}

			return fmt.Errorf("launch hypothesis: %w; compensate resource: %w", launchErr, err)
		}
	}

	if err := workflow.ExecuteActivity(stateCtx, failHypothesisLaunchActivityName, launchID, launchErr.Error()).Get(ctx, nil); err != nil {
		return fmt.Errorf("launch hypothesis: %w; mark failed: %w", launchErr, err)
	}

	return fmt.Errorf("launch hypothesis: %w", launchErr)
}

func failHypothesisLaunchWorkflowWithMetadata(ctx workflow.Context, stateCtx workflow.Context, launchID string, created []hypothesislaunch.WorkflowResource, launchErr error, requiresReconciliation bool) error {
	if requiresReconciliation {
		if err := workflow.ExecuteActivity(stateCtx, requireHypothesisLaunchReconciliationActivityName, launchID, launchErr.Error(), true).Get(ctx, nil); err != nil {
			return fmt.Errorf("launch hypothesis: %w; mark reconciliation required: %w", launchErr, err)
		}

		return fmt.Errorf("launch hypothesis requires reconciliation: %w", launchErr)
	}

	for index := len(created) - 1; index >= 0; index-- {
		resource := created[index]
		activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			ActivityID:          resourceActivityID("launch-delete", resource),
			StartToCloseTimeout: time.Minute,
			RetryPolicy: &temporal.RetryPolicy{
				InitialInterval: time.Second,
				MaximumAttempts: 5,
			},
		})
		if err := workflow.ExecuteActivity(activityCtx, deleteHypothesisLaunchResourceWithMetadataActivityName, resource).Get(ctx, nil); err != nil {
			reconciliationErr := workflow.ExecuteActivity(stateCtx, requireHypothesisLaunchReconciliationActivityName, launchID, launchErr.Error(), false).Get(ctx, nil)
			if reconciliationErr != nil {
				return fmt.Errorf("launch hypothesis: %w; compensate resource: %w; mark reconciliation required: %w", launchErr, err, reconciliationErr)
			}

			return fmt.Errorf("launch hypothesis: %w; compensate resource: %w", launchErr, err)
		}
	}

	if err := workflow.ExecuteActivity(stateCtx, failHypothesisLaunchActivityName, launchID, launchErr.Error()).Get(ctx, nil); err != nil {
		return fmt.Errorf("launch hypothesis: %w; mark failed: %w", launchErr, err)
	}

	return fmt.Errorf("launch hypothesis: %w", launchErr)
}

func hypothesisStopWorkflow(ctx workflow.Context, input hypothesisStopInput) error {
	stateCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumAttempts: 5,
		},
	})
	if err := workflow.ExecuteActivity(stateCtx, markHypothesisStopRunningActivityName, input.StopID).Get(ctx, nil); err != nil {
		return fmt.Errorf("mark hypothesis stop running: %w", err)
	}

	deleteCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumInterval: 5 * time.Second,
			MaximumAttempts: 3,
		},
	})
	metadataVersion := workflow.GetVersion(ctx, hypothesisStopWorkflowMetadataChangeID, workflow.DefaultVersion, 1)
	if metadataVersion == workflow.DefaultVersion {
		for _, resourceID := range input.ResourceIDs {
			if err := workflow.ExecuteActivity(deleteCtx, deleteHypothesisLaunchResourceActivityName, resourceID).Get(ctx, nil); err != nil {
				return requireHypothesisStopManualIntervention(ctx, stateCtx, input.StopID, err)
			}
		}
	} else {
		for _, resource := range input.Resources {
			activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
				ActivityID:          resourceActivityID("stop-delete", resource),
				StartToCloseTimeout: time.Minute,
				RetryPolicy: &temporal.RetryPolicy{
					InitialInterval: time.Second,
					MaximumInterval: 5 * time.Second,
					MaximumAttempts: 3,
				},
			})
			if err := workflow.ExecuteActivity(activityCtx, deleteHypothesisLaunchResourceWithMetadataActivityName, resource).Get(ctx, nil); err != nil {
				return requireHypothesisStopManualIntervention(ctx, stateCtx, input.StopID, err)
			}
		}
	}

	if err := workflow.ExecuteActivity(stateCtx, completeHypothesisStopActivityName, input.StopID).Get(ctx, nil); err != nil {
		return requireHypothesisStopManualIntervention(ctx, stateCtx, input.StopID, err)
	}

	return nil
}

func resourceActivityID(prefix string, resource hypothesislaunch.WorkflowResource) string {
	return fmt.Sprintf("%s-%s-%s", prefix, resource.Kind, resource.LogicalKey)
}

func requireHypothesisStopManualIntervention(ctx workflow.Context, stateCtx workflow.Context, stopID string, stopErr error) error {
	if err := workflow.ExecuteActivity(stateCtx, requireHypothesisStopManualActivityName, stopID, stopErr.Error()).Get(ctx, nil); err != nil {
		return fmt.Errorf("stop hypothesis: %w; require manual intervention: %w", stopErr, err)
	}

	return fmt.Errorf("stop hypothesis requires manual intervention: %w", stopErr)
}
