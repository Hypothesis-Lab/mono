package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"app/internal/clients/yandex_direct"
	"app/internal/repositories/hypothesislaunch"

	"go.uber.org/zap"
)

func (s *HypothesisService) beginHypothesisLaunchRollbackActivity(ctx context.Context, launchID, reason string) error {
	return s.launches.BeginRollback(ctx, launchID, reason)
}

func (s *HypothesisService) stopHypothesisLaunchWithResidualsActivity(ctx context.Context, launchID, reason string) error {
	_, err := s.launches.StopWithResiduals(ctx, launchID, reason)
	return err
}

func (s *HypothesisService) rollbackHypothesisLaunchResourcesActivity(ctx context.Context, launchID string) error {
	if err := s.launches.EnsureRollbackAllowed(ctx, launchID); err != nil {
		return err
	}

	resources, err := s.launches.RollbackResources(ctx, launchID)
	if err != nil {
		return err
	}

	blocked := make(map[string]bool)
	campaignBlocked := false
	var rollbackErrors []error
	for _, resource := range resources {
		if err := s.launches.EnsureRollbackAllowed(ctx, launchID); err != nil {
			return errors.Join(errors.Join(rollbackErrors...), err)
		}

		if isRollbackParentBlocked(resource, blocked, campaignBlocked) {
			if err := s.retainOrBlockRollbackParent(ctx, resource, &campaignBlocked, blocked); err != nil {
				rollbackErrors = append(rollbackErrors, err)
			}

			continue
		}

		outcome, err := s.rollbackHypothesisLaunchResource(ctx, resource)
		if err != nil {
			rollbackErrors = append(rollbackErrors, err)
			recordRollbackBlock(resource, &campaignBlocked, blocked)
			continue
		}

		if outcome == "retained" {
			recordRollbackBlock(resource, &campaignBlocked, blocked)
		}
	}

	return errors.Join(rollbackErrors...)
}

func (s *HypothesisService) rollbackHypothesisLaunchResource(ctx context.Context, resource hypothesislaunch.Resource) (string, error) {
	started := time.Now()
	outcome, err := s.rollbackResource(ctx, resource)
	fields := []zap.Field{
		zap.String("launch_id", resource.LaunchID),
		zap.String("resource_id", resource.ID),
		zap.String("resource_kind", resource.Kind),
		zap.Int64("elapsed_ms", time.Since(started).Milliseconds()),
		zap.String("outcome", outcome),
	}
	if err != nil {
		fields = append(fields, zap.Error(err))
		s.logger.Error("hypothesis launch resource rollback failed", fields...)
	} else {
		s.logger.Info("hypothesis launch resource rollback finished", fields...)
	}

	return outcome, err
}

func (s *HypothesisService) rollbackResource(ctx context.Context, resource hypothesislaunch.Resource) (string, error) {
	switch resource.Status {
	case "compensated", "deleted":
		return "compensated", nil
	case "retained":
		return "retained", nil
	}

	if resource.Kind == hypothesislaunch.ResourceCampaignSuspend {
		if err := s.launches.MarkResourceCompensated(ctx, resource.ID); err != nil {
			return "error", fmt.Errorf("record campaign suspension action rollback: %w", err)
		}

		return "compensated", nil
	}

	if resource.ExternalID == nil && resource.ExternalKey == nil {
		if resource.Status == "pending" {
			if err := s.launches.MarkResourceCompensated(ctx, resource.ID); err != nil {
				return "error", fmt.Errorf("mark never-attempted resource %s compensated: %w", resource.ID, err)
			}

			return "compensated", nil
		}

		return "error", fmt.Errorf("resource %s has no recorded provider identity after status %q; outcome is unknown", resource.ID, resource.Status)
	}

	if resource.OwnedByLaunch == nil {
		return "error", fmt.Errorf("resource %s provider identity has unknown launch ownership", resource.ID)
	}

	if !*resource.OwnedByLaunch {
		if err := s.launches.MarkResourceRetained(ctx, resource.ID); err != nil {
			return "error", fmt.Errorf("record reused resource %s retained: %w", resource.ID, err)
		}

		return "retained", nil
	}

	return s.rollbackOwnedResource(ctx, resource)
}

func (s *HypothesisService) rollbackOwnedResource(ctx context.Context, resource hypothesislaunch.Resource) (string, error) {
	yandex, err := s.yandexClientForLaunch(ctx, resource.LaunchID, "")
	if err != nil {
		return "error", err
	}

	if resource.Kind == hypothesislaunch.ResourceAdImage {
		return s.rollbackAdImage(ctx, yandex, resource)
	}

	if resource.ExternalID == nil {
		return "error", fmt.Errorf("owned resource %s has no numeric provider identity", resource.ID)
	}

	service, objectName, deleteResource, err := rollbackResourceDelete(yandex, resource)
	if err != nil {
		return "error", err
	}

	exists, err := yandex.ExistsForRollback(ctx, service, objectName, *resource.ExternalID)
	if err != nil {
		return "error", fmt.Errorf("check resource %s before rollback deletion: %w", resource.ID, err)
	}

	if exists {
		if err := deleteResource(ctx, *resource.ExternalID); err != nil {
			return "error", fmt.Errorf("delete resource %s during launch rollback: %w", resource.ID, err)
		}
	}

	exists, err = yandex.ExistsForRollback(ctx, service, objectName, *resource.ExternalID)
	if err != nil {
		return "error", fmt.Errorf("verify resource %s absent after rollback deletion: %w", resource.ID, err)
	}

	if exists {
		return "error", fmt.Errorf("resource %s remains after rollback deletion", resource.ID)
	}

	if err := s.launches.MarkResourceCompensated(ctx, resource.ID); err != nil {
		return "error", fmt.Errorf("record resource %s rollback complete: %w", resource.ID, err)
	}

	return "compensated", nil
}

func (s *HypothesisService) rollbackAdImage(ctx context.Context, yandex *yandexdirect.Client, resource hypothesislaunch.Resource) (string, error) {
	if resource.ExternalKey == nil {
		return "error", fmt.Errorf("owned image resource %s has no provider hash", resource.ID)
	}

	referenced, err := s.launches.ImageReferencedByOtherLaunch(ctx, resource.ID, *resource.ExternalKey)
	if err != nil {
		return "error", err
	}

	if referenced {
		return "error", fmt.Errorf("owned image resource %s is referenced by another launch; cleanup is unresolved", resource.ID)
	}

	exists, err := yandex.ImageExistsForRollback(ctx, *resource.ExternalKey)
	if err != nil {
		return "error", fmt.Errorf("check image %s before rollback deletion: %w", resource.ID, err)
	}

	if exists {
		if err := yandex.DeleteAdImage(ctx, *resource.ExternalKey); err != nil {
			return "error", fmt.Errorf("delete image %s during launch rollback: %w", resource.ID, err)
		}
	}

	exists, err = yandex.ImageExistsForRollback(ctx, *resource.ExternalKey)
	if err != nil {
		return "error", fmt.Errorf("verify image %s absent after rollback deletion: %w", resource.ID, err)
	}

	if exists {
		return "error", fmt.Errorf("image %s remains after rollback deletion", resource.ID)
	}

	if err := s.launches.MarkResourceCompensated(ctx, resource.ID); err != nil {
		return "error", fmt.Errorf("record image %s rollback complete: %w", resource.ID, err)
	}

	return "compensated", nil
}

func rollbackResourceDelete(yandex *yandexdirect.Client, resource hypothesislaunch.Resource) (string, string, func(context.Context, int64) error, error) {
	switch resource.Kind {
	case hypothesislaunch.ResourceCampaign:
		return "campaigns", "Campaigns", yandex.DeleteCampaign, nil
	case hypothesislaunch.ResourceRetargetingList:
		return "retargetinglists", "RetargetingLists", yandex.DeleteRetargetingList, nil
	case hypothesislaunch.ResourceAdGroup, hypothesislaunch.ResourceUnifiedAdGroup:
		return "adgroups", "AdGroups", yandex.DeleteAdGroup, nil
	case hypothesislaunch.ResourceAudienceTarget:
		return "audiencetargets", "AudienceTargets", yandex.DeleteAudienceTarget, nil
	case hypothesislaunch.ResourceDemographicExclusion:
		return "bidmodifiers", "BidModifiers", yandex.DeleteDemographicExclusion, nil
	case hypothesislaunch.ResourceKeyword:
		return "keywords", "Keywords", yandex.DeleteKeyword, nil
	case hypothesislaunch.ResourceResponsiveAd:
		return "ads", "Ads", yandex.DeleteAd, nil
	default:
		return "", "", nil, fmt.Errorf("unsupported rollback resource kind %q", resource.Kind)
	}
}

func isRollbackParentBlocked(resource hypothesislaunch.Resource, blocked map[string]bool, campaignBlocked bool) bool {
	if resource.Kind == hypothesislaunch.ResourceCampaign {
		return campaignBlocked
	}

	return blocked[resource.Kind+"\x00"+resource.LogicalKey]
}

func recordRollbackBlock(resource hypothesislaunch.Resource, campaignBlocked *bool, blocked map[string]bool) {
	switch resource.Kind {
	case hypothesislaunch.ResourceResponsiveAd, hypothesislaunch.ResourceKeyword, hypothesislaunch.ResourceDemographicExclusion:
		blocked[hypothesislaunch.ResourceUnifiedAdGroup+"\x00"+audienceGroupKey(resource.LogicalKey)] = true
	case hypothesislaunch.ResourceAudienceTarget:
		blocked[hypothesislaunch.ResourceAdGroup+"\x00"+resource.LogicalKey] = true
		blocked[hypothesislaunch.ResourceRetargetingList+"\x00"+resource.LogicalKey] = true
	case hypothesislaunch.ResourceUnifiedAdGroup, hypothesislaunch.ResourceAdGroup, hypothesislaunch.ResourceRetargetingList:
		*campaignBlocked = true
	}
}

func (s *HypothesisService) retainOrBlockRollbackParent(ctx context.Context, resource hypothesislaunch.Resource, campaignBlocked *bool, blocked map[string]bool) error {
	if resource.OwnedByLaunch != nil && !*resource.OwnedByLaunch {
		if err := s.launches.MarkResourceRetained(ctx, resource.ID); err != nil {
			return fmt.Errorf("record blocked resource %s retained: %w", resource.ID, err)
		}

		retained := resource
		retained.Status = "retained"
		recordRollbackBlock(retained, campaignBlocked, blocked)
		return nil
	}

	recordRollbackBlock(resource, campaignBlocked, blocked)
	return fmt.Errorf("resource %s retained for safety because a dependent resource is unresolved", resource.ID)
}
