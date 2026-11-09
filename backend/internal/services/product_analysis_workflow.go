package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"app/internal/clients"
	"app/internal/repositories/productanalysis"

	"github.com/google/uuid"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/fx"
)

const (
	productAnalysisTaskQueue      = "product-analysis"
	productAnalysisStageActivity  = "product-analysis.stage"
	productAnalysisFailActivity   = "product-analysis.fail"
	productAnalysisContinueSignal = "product-analysis.continue"
)

type productAnalysisStageResult struct {
	Pending bool
	Blocked bool
}

type ProductAnalysisWorker struct {
	worker worker.Worker
}

type productAnalysisActivities struct {
	repository *productanalysis.Repository
	generator  *clients.GeneratorClient
}

func NewProductAnalysisWorker(lc fx.Lifecycle, temporalClient client.Client, repository *productanalysis.Repository, generator *clients.GeneratorClient) *ProductAnalysisWorker {
	activities := &productAnalysisActivities{repository: repository, generator: generator}
	w := &ProductAnalysisWorker{worker: worker.New(temporalClient, productAnalysisTaskQueue, worker.Options{})}
	w.worker.RegisterWorkflow(productAnalysisWorkflow)
	w.worker.RegisterActivityWithOptions(activities.stage, activity.RegisterOptions{Name: productAnalysisStageActivity})
	w.worker.RegisterActivityWithOptions(activities.fail, activity.RegisterOptions{Name: productAnalysisFailActivity})
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error { return w.worker.Start() },
		OnStop:  func(context.Context) error { w.worker.Stop(); return nil },
	})
	return w
}

func productAnalysisWorkflow(ctx workflow.Context, runID string) error {
	activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: time.Second, MaximumInterval: 30 * time.Second, MaximumAttempts: 5},
	})
	for _, stage := range []string{"product", "audience"} {
		for {
			var state productAnalysisStageResult
			if err := workflow.ExecuteActivity(activityCtx, productAnalysisStageActivity, runID, stage).Get(ctx, &state); err != nil {
				if failErr := workflow.ExecuteActivity(activityCtx, productAnalysisFailActivity, runID).Get(ctx, nil); failErr != nil {
					return fmt.Errorf("run %s stage %s: %w; mark failed: %w", runID, stage, err, failErr)
				}

				return fmt.Errorf("run %s stage %s: %w", runID, stage, err)
			}

			if state.Blocked {
				return nil
			}

			if !state.Pending {
				break
			}

			if err := workflow.Sleep(ctx, 5*time.Second); err != nil {
				return fmt.Errorf("wait for run %s stage %s: %w", runID, stage, err)
			}
		}
	}

	var requestedStage string
	workflow.GetSignalChannel(ctx, productAnalysisContinueSignal).Receive(ctx, &requestedStage)
	if requestedStage != "hypotheses" {
		return fmt.Errorf("run %s received unsupported continuation stage %q", runID, requestedStage)
	}

	for {
		var state productAnalysisStageResult
		if err := workflow.ExecuteActivity(activityCtx, productAnalysisStageActivity, runID, "hypotheses").Get(ctx, &state); err != nil {
			if failErr := workflow.ExecuteActivity(activityCtx, productAnalysisFailActivity, runID).Get(ctx, nil); failErr != nil {
				return fmt.Errorf("run %s stage hypotheses: %w; mark failed: %w", runID, err, failErr)
			}

			return fmt.Errorf("run %s stage hypotheses: %w", runID, err)
		}

		if state.Blocked || !state.Pending {
			return nil
		}

		if err := workflow.Sleep(ctx, 5*time.Second); err != nil {
			return fmt.Errorf("wait for run %s stage hypotheses: %w", runID, err)
		}
	}
}

func (a *productAnalysisActivities) stage(ctx context.Context, runID, stage string) (*productAnalysisStageResult, error) {
	run, err := a.repository.Get(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("load product analysis run for stage %s: %w", stage, err)
	}

	kind, input, err := a.stageInput(ctx, run, stage)
	if err != nil {
		return nil, fmt.Errorf("prepare product analysis stage %s: %w", stage, err)
	}

	jobID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("product-analysis:"+runID+":"+stage)).String()
	if err := a.repository.SetJob(ctx, runID, stage, jobID); err != nil {
		return nil, fmt.Errorf("save product analysis stage job: %w", err)
	}

	if err := a.generator.QueuePipelineJob(ctx, jobID, run.ProductID, kind, input); err != nil {
		return nil, fmt.Errorf("queue product analysis stage job: %w", err)
	}

	job, err := a.generator.GetPipelineJob(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("get product analysis stage job: %w", err)
	}

	if job.Kind != kind {
		return nil, fmt.Errorf("product analysis stage %s returned unexpected job kind %q", stage, job.Kind)
	}

	switch job.Status {
	case "queued", "running":
		return &productAnalysisStageResult{Pending: true}, nil
	case "failed":
		if err := a.generator.MarkJobPersisted(ctx, jobID); err != nil {
			return nil, fmt.Errorf("acknowledge failed product analysis stage job: %w", err)
		}

		return nil, temporal.NewNonRetryableApplicationError("generator job failed", "GeneratorJobFailed", nil)
	case "completed":
		return a.completeStage(ctx, runID, stage, jobID, job.Result)
	default:
		return nil, fmt.Errorf("unexpected generator job status %q", job.Status)
	}
}

func (a *productAnalysisActivities) completeStage(ctx context.Context, runID, stage, jobID string, result json.RawMessage) (*productAnalysisStageResult, error) {
	blocked, err := analysisResultBlocked(stage, result)
	if err != nil {
		return nil, fmt.Errorf("validate product analysis stage result: %w", err)
	}

	status := "running"
	if blocked {
		status = "blocked"
	} else if stage == "audience" {
		status = "waiting_for_user_input"
	} else if stage == "hypotheses" {
		status = "completed"
	}

	if err := a.generator.MarkJobPersisted(ctx, jobID); err != nil {
		return nil, fmt.Errorf("acknowledge product analysis stage job: %w", err)
	}

	if err := a.repository.SaveResult(ctx, runID, stage, result, status); err != nil {
		return nil, fmt.Errorf("save product analysis stage result: %w", err)
	}

	return &productAnalysisStageResult{Blocked: blocked}, nil
}

func (a *productAnalysisActivities) stageInput(ctx context.Context, run *productanalysis.Run, stage string) (string, json.RawMessage, error) {
	if stage == "product" {
		return "product_analysis", run.InputSnapshot, nil
	}

	var product struct {
		Passport json.RawMessage `json:"passport"`
	}
	if err := json.Unmarshal(run.ProductResult, &product); err != nil {
		return "", nil, fmt.Errorf("decode product passport for stage %s: %w", stage, err)
	}

	if len(product.Passport) == 0 {
		return "", nil, fmt.Errorf("product passport is missing for stage %s", stage)
	}

	if stage == "audience" {
		set, err := a.repository.GetSegmentSet(ctx, run.SegmentSetID)
		if err != nil {
			return "", nil, fmt.Errorf("load segment set for audience stage: %w", err)
		}

		var metadata struct {
			Report struct {
				DataProvenance string   `json:"data_provenance"`
				Warnings       []string `json:"warnings"`
			} `json:"report"`
		}
		if err := json.Unmarshal(set.Metadata, &metadata); err != nil {
			return "", nil, fmt.Errorf("decode segment dataset metadata: %w", err)
		}

		limitations := append([]string{"Сегменты исследовательские; проверьте качество, происхождение и активацию данных."}, metadata.Report.Warnings...)
		if metadata.Report.DataProvenance == "synthetic_demo" {
			limitations = append(limitations, "Данные синтетические: размеры сегментов не отражают реальных клиентов.")
		}

		input, err := json.Marshal(map[string]any{
			"product_passport": product.Passport, "segment_set_id": set.ID, "segments": set.Segments,
			"limitations": limitations,
		})
		return "audience_selection", input, err
	}

	if stage == "hypotheses" {
		audienceResult, err := applyAudienceReview(run.AudienceResult, run.AudienceReview)
		if err != nil {
			return "", nil, fmt.Errorf("apply audience review: %w", err)
		}

		input, err := json.Marshal(map[string]any{"product_passport": product.Passport, "audience_result": audienceResult})
		return "hypothesis_generation", input, err
	}

	return "", nil, fmt.Errorf("unknown product analysis stage %q", stage)
}

func applyAudienceReview(result json.RawMessage, review *productanalysis.StageReview) (json.RawMessage, error) {
	if review == nil {
		return result, nil
	}

	var envelope map[string]any
	if err := json.Unmarshal(result, &envelope); err != nil {
		return nil, fmt.Errorf("decode audience result: %w", err)
	}

	audience, ok := envelope["audience"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("audience result is missing audience object")
	}

	candidates, _ := audience["candidates"].([]any)
	byID := make(map[string]map[string]any, len(candidates))
	for _, item := range candidates {
		candidate, ok := item.(map[string]any)
		if !ok {
			continue
		}

		id, _ := candidate["candidate_id"].(string)
		if id != "" {
			byID[id] = candidate
		}
	}

	custom := []map[string]any{}
	if err := json.Unmarshal(review.CustomCandidates, &custom); err != nil {
		return nil, fmt.Errorf("decode audience custom candidates: %w", err)
	}

	for _, candidate := range custom {
		id, _ := candidate["candidate_id"].(string)
		if id != "" {
			byID[id] = candidate
		}
	}

	selected := make([]any, 0, len(review.SelectedCandidateIDs))
	for _, id := range review.SelectedCandidateIDs {
		candidate, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("selected audience candidate %q is missing", id)
		}

		selected = append(selected, candidate)
	}

	audience["segments"] = selected
	audience["selected_candidate_ids"] = review.SelectedCandidateIDs
	audience["custom_candidates"] = custom
	envelope["audience"] = audience

	updated, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("encode reviewed audience result: %w", err)
	}

	return updated, nil
}

func (a *productAnalysisActivities) fail(ctx context.Context, runID string) error {
	return a.repository.Fail(ctx, runID)
}

func analysisResultBlocked(stage string, result json.RawMessage) (bool, error) {
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(result, &decoded); err != nil {
		return false, fmt.Errorf("decode %s analysis result: %w", stage, err)
	}

	var status struct {
		Status           string            `json:"status"`
		ExtractionStatus string            `json:"extraction_status"`
		Records          []json.RawMessage `json:"records"`
		Segments         []json.RawMessage `json:"segments"`
	}
	switch stage {
	case "product":
		if err := json.Unmarshal(decoded["passport"], &status); err != nil {
			return false, fmt.Errorf("decode product passport status: %w", err)
		}

		if !validAnalysisResultStatus(status.ExtractionStatus) {
			return false, fmt.Errorf("invalid product extraction status %q", status.ExtractionStatus)
		}

		return status.ExtractionStatus == "blocked" || len(status.Records) == 0, nil
	case "audience":
		if err := json.Unmarshal(decoded["audience"], &status); err != nil {
			return false, fmt.Errorf("decode audience selection status: %w", err)
		}

		if !validAnalysisResultStatus(status.Status) {
			return false, fmt.Errorf("invalid audience selection status %q", status.Status)
		}

		return status.Status == "blocked" || len(status.Segments) == 0, nil
	case "hypotheses":
		if err := json.Unmarshal(result, &status); err != nil {
			return false, fmt.Errorf("decode hypothesis generation status: %w", err)
		}

		if !validAnalysisResultStatus(status.Status) {
			return false, fmt.Errorf("invalid hypothesis generation status %q", status.Status)
		}

		return status.Status == "blocked", nil
	default:
		return false, fmt.Errorf("unknown result stage %q", stage)
	}
}

func validAnalysisResultStatus(status string) bool {
	return status == "complete" || status == "partial" || status == "blocked"
}
