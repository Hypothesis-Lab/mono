package services

import (
	"context"
	"fmt"
	"time"

	"app/internal/clients/yandex_direct"
	"app/internal/repositories/hypothesislaunch"

	"go.uber.org/zap"
)

func (s *HypothesisService) markHypothesisLaunchRunningActivity(ctx context.Context, launchID string) error {
	if err := s.launches.MarkRunning(ctx, launchID); err != nil {
		return fmt.Errorf("mark hypothesis launch running activity: %w", err)
	}

	return nil
}

func (s *HypothesisService) markHypothesisStopRunningActivity(ctx context.Context, stopID string) error {
	if err := s.launches.MarkStopRunning(ctx, stopID); err != nil {
		return fmt.Errorf("mark hypothesis stop running activity: %w", err)
	}

	return nil
}

func (s *HypothesisService) ensureHypothesisLaunchResourceActivity(ctx context.Context, resourceID string) (resultErr error) {
	started := time.Now()
	launchID, resourceKind, outcome := "", "", "error"
	defer func() {
		fields := []zap.Field{
			zap.String("launch_id", launchID),
			zap.String("resource_id", resourceID),
			zap.String("resource_kind", resourceKind),
			zap.Int64("elapsed_ms", time.Since(started).Milliseconds()),
			zap.String("outcome", outcome),
		}
		if resultErr != nil {
			fields = append(fields, zap.Error(resultErr))
		}

		s.logger.Info("hypothesis launch resource attempt finished", fields...)
	}()

	resource, err := s.launches.GetResource(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("get hypothesis launch resource activity: %w", err)
	}

	launchID, resourceKind = resource.LaunchID, resource.Kind
	if err := s.launches.EnsureCreatingAllowed(ctx, resource.LaunchID); err != nil {
		return err
	}

	if resource.ExternalID != nil {
		outcome = "reused"
		return nil
	}

	if resource.Kind == hypothesislaunch.ResourceAdImage && resource.ExternalKey != nil {
		outcome = "available"
		return nil
	}

	yandex, err := s.yandexClientForLaunch(ctx, resource.LaunchID, "")
	if err != nil {
		return err
	}

	requestCtx := yandexdirect.WithRequestTiming(ctx, func(service, method string, elapsed time.Duration, statusCode int) {
		s.logger.Info("yandex direct request finished",
			zap.String("launch_id", launchID),
			zap.String("resource_id", resource.ID),
			zap.String("resource_kind", resource.Kind),
			zap.String("service", service),
			zap.String("method", method),
			zap.Int64("elapsed_ms", elapsed.Milliseconds()),
			zap.Int("http_status", statusCode),
		)
	})
	result, err := s.resolveAndRecordLaunchResource(requestCtx, yandex, resource)
	if err != nil {
		return err
	}

	outcome = result

	return nil
}

func (s *HypothesisService) resolveAndRecordLaunchResource(ctx context.Context, yandex *yandexdirect.Client, resource *hypothesislaunch.Resource) (string, error) {
	if resource.Kind == hypothesislaunch.ResourceAdImage {
		externalKey, created, err := s.findOrCreateAdImage(ctx, yandex, resource)
		if err != nil {
			return "error", err
		}

		if err := s.launches.RecordResourceCreatedKey(ctx, resource.ID, externalKey, resourceOwnership(resource.Status, created)); err != nil {
			return "error", fmt.Errorf("record hypothesis ad image activity: %w", err)
		}

		return "available", nil
	}

	externalID, created, err := s.findOrCreateLaunchResource(ctx, yandex, resource)
	if err != nil {
		return "error", err
	}

	if err := s.launches.RecordResourceCreated(ctx, resource.ID, externalID, resourceOwnership(resource.Status, created)); err != nil {
		return "error", fmt.Errorf("record hypothesis launch resource activity: %w", err)
	}

	if resource.Kind == hypothesislaunch.ResourceCampaignSuspend {
		return "verified", nil
	}

	if created {
		return "created", nil
	}

	return "reused", nil
}

func resourceOwnership(status string, created bool) *bool {
	if created {
		return boolPointer(true)
	}

	if status == "pending" {
		return boolPointer(false)
	}

	return nil
}

func boolPointer(value bool) *bool { return &value }

func (s *HypothesisService) createHypothesisLaunchResourceActivity(ctx context.Context, resource hypothesislaunch.WorkflowResource) error {
	if err := s.ensureHypothesisLaunchResourceActivity(ctx, resource.ID); err != nil {
		return fmt.Errorf("create hypothesis launch resource %s: %w", resource.Kind, err)
	}

	return nil
}

//nolint:cyclop // Resource kinds require distinct provider APIs and dependency lookups.
func (s *HypothesisService) findOrCreateLaunchResource(ctx context.Context, yandex *yandexdirect.Client, resource *hypothesislaunch.Resource) (int64, bool, error) {
	switch resource.Kind {
	case hypothesislaunch.ResourceCampaign:
		preparationFormat, err := campaignPreparationFormat(resource.Payload)
		if err != nil {
			return 0, false, fmt.Errorf("decode campaign launch preparation mode: %w", err)
		}

		if preparationFormat == unifiedDraftFormat {
			return s.findOrCreateUnifiedCampaign(ctx, yandex, resource)
		}

		if preparationFormat != "" {
			return 0, false, fmt.Errorf("unsupported campaign launch preparation format %q", preparationFormat)
		}

		if resource.Name == nil || resource.StartDate == nil {
			return 0, false, fmt.Errorf("campaign resource %s has incomplete launch data", resource.ID)
		}

		id, found, err := yandex.FindCampaignByName(ctx, *resource.Name)
		if err != nil {
			return 0, false, fmt.Errorf("find campaign resource %s: %w", resource.ID, err)
		}

		if found {
			return id, false, nil
		}

		if err := s.prepareHypothesisLaunchResourceCreate(ctx, resource); err != nil {
			return 0, false, err
		}

		id, err = yandex.CreateCampaign(ctx, *resource.Name, *resource.StartDate)
		if err != nil {
			return 0, false, fmt.Errorf("create campaign resource %s: %w", resource.ID, err)
		}

		return id, true, nil
	case hypothesislaunch.ResourceRetargetingList:
		if resource.Name == nil || resource.Gender == nil || resource.Age == nil {
			return 0, false, fmt.Errorf("retargeting list resource %s has incomplete launch data", resource.ID)
		}

		id, found, err := yandex.FindDemographicRetargetingListByName(ctx, *resource.Name)
		if err != nil {
			return 0, false, fmt.Errorf("find retargeting list resource %s: %w", resource.ID, err)
		}

		if found {
			return id, false, nil
		}

		if err := s.prepareHypothesisLaunchResourceCreate(ctx, resource); err != nil {
			return 0, false, err
		}

		id, err = yandex.CreateDemographicRetargetingList(ctx, *resource.Name, *resource.Gender, *resource.Age)
		if err != nil {
			return 0, false, fmt.Errorf("create retargeting list resource %s: %w", resource.ID, err)
		}

		return id, true, nil
	case hypothesislaunch.ResourceAdGroup:
		if resource.Name == nil {
			return 0, false, fmt.Errorf("ad group resource %s has no name", resource.ID)
		}

		campaignID, err := s.launches.DependencyExternalID(ctx, resource.LaunchID, hypothesislaunch.ResourceCampaign, "campaign")
		if err != nil {
			return 0, false, fmt.Errorf("get campaign dependency for ad group resource %s: %w", resource.ID, err)
		}

		id, found, err := yandex.FindUserProfileAdGroup(ctx, campaignID, *resource.Name)
		if err != nil {
			return 0, false, fmt.Errorf("find ad group resource %s: %w", resource.ID, err)
		}

		if found {
			return id, false, nil
		}

		if err := s.prepareHypothesisLaunchResourceCreate(ctx, resource); err != nil {
			return 0, false, err
		}

		id, err = yandex.CreateUserProfileAdGroup(ctx, campaignID, *resource.Name, []int64{russiaYandexRegionID})
		if err != nil {
			return 0, false, fmt.Errorf("create ad group resource %s: %w", resource.ID, err)
		}

		return id, true, nil
	case hypothesislaunch.ResourceAudienceTarget:
		adGroupID, err := s.launches.DependencyExternalID(ctx, resource.LaunchID, hypothesislaunch.ResourceAdGroup, resource.LogicalKey)
		if err != nil {
			return 0, false, fmt.Errorf("get ad group dependency for audience target resource %s: %w", resource.ID, err)
		}

		retargetingListID, err := s.launches.DependencyExternalID(ctx, resource.LaunchID, hypothesislaunch.ResourceRetargetingList, resource.LogicalKey)
		if err != nil {
			return 0, false, fmt.Errorf("get retargeting list dependency for audience target resource %s: %w", resource.ID, err)
		}

		id, found, err := yandex.FindAudienceTarget(ctx, adGroupID, retargetingListID)
		if err != nil {
			return 0, false, fmt.Errorf("find audience target resource %s: %w", resource.ID, err)
		}

		if found {
			return id, false, nil
		}

		if err := s.prepareHypothesisLaunchResourceCreate(ctx, resource); err != nil {
			return 0, false, err
		}

		id, err = yandex.CreateAudienceTarget(ctx, adGroupID, retargetingListID)
		if err != nil {
			return 0, false, fmt.Errorf("create audience target resource %s: %w", resource.ID, err)
		}

		return id, true, nil
	default:
		return s.findOrCreateUnifiedDraftResource(ctx, yandex, resource)
	}
}

func (s *HypothesisService) prepareHypothesisLaunchResourceCreate(ctx context.Context, resource *hypothesislaunch.Resource) error {
	if resource.Status == "creating" {
		return fmt.Errorf("hypothesis launch resource %s may have been created but is not yet visible in yandex", resource.ID)
	}

	if resource.Status != "pending" {
		return fmt.Errorf("hypothesis launch resource %s cannot be created from status %q", resource.ID, resource.Status)
	}

	if err := s.launches.MarkResourceCreating(ctx, resource.ID); err != nil {
		return fmt.Errorf("mark hypothesis launch resource %s creating: %w", resource.ID, err)
	}

	return nil
}

func (s *HypothesisService) deleteHypothesisLaunchResourceActivity(ctx context.Context, resourceID string) error {
	resource, err := s.launches.GetResource(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("get delete hypothesis launch resource activity: %w", err)
	}

	return s.deleteHypothesisLaunchResource(ctx, resource)
}

func (s *HypothesisService) deleteHypothesisLaunchResource(ctx context.Context, resource *hypothesislaunch.Resource) error {
	if resource.Status == "deleted" {
		return nil
	}

	if resource.Kind == hypothesislaunch.ResourceCampaignSuspend {
		if err := s.launches.MarkResourceDeleted(ctx, resource.ID); err != nil {
			return fmt.Errorf("record campaign suspension action as compensated: %w", err)
		}

		return nil
	}

	if resource.Kind == hypothesislaunch.ResourceAdImage {
		if resource.ExternalKey == nil && resource.Status == "pending" {
			if err := s.launches.MarkResourceDeleted(ctx, resource.ID); err != nil {
				return fmt.Errorf("mark uncreated ad image resource deleted: %w", err)
			}

			return nil
		}

		return fmt.Errorf("ad image resource %s may be shared and cannot be safely deleted", resource.ID)
	}

	if resource.ExternalID == nil {
		return nil
	}

	yandex, err := s.yandexClientForLaunch(ctx, resource.LaunchID, "")
	if err != nil {
		return err
	}

	service, objectName, deleteResource, err := launchResourceDelete(yandex, resource)
	if err != nil {
		return err
	}

	exists, err := yandex.Exists(ctx, service, objectName, *resource.ExternalID)
	if err != nil {
		return fmt.Errorf("check hypothesis launch resource %s before deletion: %w", resource.ID, err)
	}

	if exists {
		if err := deleteResource(ctx, *resource.ExternalID); err != nil {
			return fmt.Errorf("delete hypothesis launch resource %s: %w", resource.ID, err)
		}
	}

	if err := s.launches.MarkResourceDeleted(ctx, resource.ID); err != nil {
		return fmt.Errorf("record deleted hypothesis launch resource %s: %w", resource.ID, err)
	}

	return nil
}

func (s *HypothesisService) deleteHypothesisLaunchResourceActivityWithMetadata(ctx context.Context, resource hypothesislaunch.WorkflowResource) error {
	if err := s.deleteHypothesisLaunchResourceActivity(ctx, resource.ID); err != nil {
		return fmt.Errorf("delete hypothesis launch resource %s: %w", resource.Kind, err)
	}

	return nil
}

func launchResourceDelete(yandex *yandexdirect.Client, resource *hypothesislaunch.Resource) (string, string, func(context.Context, int64) error, error) {
	switch resource.Kind {
	case hypothesislaunch.ResourceCampaign:
		return "campaigns", "Campaigns", yandex.DeleteCampaign, nil
	case hypothesislaunch.ResourceRetargetingList:
		return "retargetinglists", "RetargetingLists", yandex.DeleteRetargetingList, nil
	case hypothesislaunch.ResourceAdGroup:
		return "adgroups", "AdGroups", yandex.DeleteAdGroup, nil
	case hypothesislaunch.ResourceAudienceTarget:
		return "audiencetargets", "AudienceTargets", yandex.DeleteAudienceTarget, nil
	case hypothesislaunch.ResourceUnifiedAdGroup:
		return "adgroups", "AdGroups", yandex.DeleteAdGroup, nil
	case hypothesislaunch.ResourceDemographicExclusion:
		return "bidmodifiers", "BidModifiers", yandex.DeleteDemographicExclusion, nil
	case hypothesislaunch.ResourceKeyword:
		return "keywords", "Keywords", yandex.DeleteKeyword, nil
	case hypothesislaunch.ResourceResponsiveAd:
		return "ads", "Ads", yandex.DeleteAd, nil
	default:
		return "", "", nil, fmt.Errorf("unsupported hypothesis launch resource kind %q", resource.Kind)
	}
}

func (s *HypothesisService) completeHypothesisLaunchActivity(ctx context.Context, launchID string) error {
	if err := s.launches.Complete(ctx, launchID); err != nil {
		return fmt.Errorf("complete hypothesis launch activity: %w", err)
	}

	return nil
}

func (s *HypothesisService) completeHypothesisStopActivity(ctx context.Context, stopID string) error {
	if err := s.launches.CompleteStop(ctx, stopID); err != nil {
		return fmt.Errorf("complete hypothesis stop activity: %w", err)
	}

	return nil
}

func (s *HypothesisService) failHypothesisLaunchActivity(ctx context.Context, launchID, reason string) error {
	if err := s.launches.Fail(ctx, launchID, reason); err != nil {
		return fmt.Errorf("fail hypothesis launch activity: %w", err)
	}

	return nil
}

func (s *HypothesisService) requireHypothesisLaunchReconciliationActivity(ctx context.Context, launchID, reason string, retry bool) error {
	if err := s.launches.RequireReconciliation(ctx, launchID, reason, retry); err != nil {
		return fmt.Errorf("require hypothesis launch reconciliation activity: %w", err)
	}

	return nil
}

func (s *HypothesisService) requireHypothesisStopManualActivity(ctx context.Context, stopID, reason string) error {
	if err := s.launches.RequireStopManualIntervention(ctx, stopID, reason); err != nil {
		return fmt.Errorf("require hypothesis stop manual intervention activity: %w", err)
	}

	return nil
}
