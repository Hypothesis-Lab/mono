package services

import (
	"context"
	"fmt"

	"app/internal/repositories/hypothesislaunch"
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

func (s *HypothesisService) ensureHypothesisLaunchResourceActivity(ctx context.Context, resourceID string) error {
	resource, err := s.launches.GetResource(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("get hypothesis launch resource activity: %w", err)
	}

	if resource.ExternalID != nil {
		return nil
	}

	externalID, err := s.findOrCreateLaunchResource(ctx, resource)
	if err != nil {
		return err
	}

	if err := s.launches.RecordResourceCreated(ctx, resource.ID, externalID); err != nil {
		return fmt.Errorf("record hypothesis launch resource activity: %w", err)
	}

	return nil
}

//nolint:cyclop // Resource kinds require distinct provider APIs and dependency lookups.
func (s *HypothesisService) findOrCreateLaunchResource(ctx context.Context, resource *hypothesislaunch.Resource) (int64, error) {
	switch resource.Kind {
	case hypothesislaunch.ResourceCampaign:
		if resource.Name == nil || resource.StartDate == nil {
			return 0, fmt.Errorf("campaign resource %s has incomplete launch data", resource.ID)
		}

		id, found, err := s.yandex.FindCampaignByName(ctx, *resource.Name)
		if err != nil {
			return 0, fmt.Errorf("find campaign resource %s: %w", resource.ID, err)
		}

		if found {
			return id, nil
		}

		if err := s.prepareHypothesisLaunchResourceCreate(ctx, resource); err != nil {
			return 0, err
		}

		id, err = s.yandex.CreateCampaign(ctx, *resource.Name, *resource.StartDate)
		if err != nil {
			return 0, fmt.Errorf("create campaign resource %s: %w", resource.ID, err)
		}

		return id, nil
	case hypothesislaunch.ResourceRetargetingList:
		if resource.Name == nil || resource.Gender == nil || resource.Age == nil {
			return 0, fmt.Errorf("retargeting list resource %s has incomplete launch data", resource.ID)
		}

		id, found, err := s.yandex.FindDemographicRetargetingListByName(ctx, *resource.Name)
		if err != nil {
			return 0, fmt.Errorf("find retargeting list resource %s: %w", resource.ID, err)
		}

		if found {
			return id, nil
		}

		if err := s.prepareHypothesisLaunchResourceCreate(ctx, resource); err != nil {
			return 0, err
		}

		id, err = s.yandex.CreateDemographicRetargetingList(ctx, *resource.Name, *resource.Gender, *resource.Age)
		if err != nil {
			return 0, fmt.Errorf("create retargeting list resource %s: %w", resource.ID, err)
		}

		return id, nil
	case hypothesislaunch.ResourceAdGroup:
		if resource.Name == nil {
			return 0, fmt.Errorf("ad group resource %s has no name", resource.ID)
		}

		campaignID, err := s.launches.DependencyExternalID(ctx, resource.LaunchID, hypothesislaunch.ResourceCampaign, "campaign")
		if err != nil {
			return 0, fmt.Errorf("get campaign dependency for ad group resource %s: %w", resource.ID, err)
		}

		id, found, err := s.yandex.FindUserProfileAdGroup(ctx, campaignID, *resource.Name)
		if err != nil {
			return 0, fmt.Errorf("find ad group resource %s: %w", resource.ID, err)
		}

		if found {
			return id, nil
		}

		if err := s.prepareHypothesisLaunchResourceCreate(ctx, resource); err != nil {
			return 0, err
		}

		id, err = s.yandex.CreateUserProfileAdGroup(ctx, campaignID, *resource.Name, []int64{russiaYandexRegionID})
		if err != nil {
			return 0, fmt.Errorf("create ad group resource %s: %w", resource.ID, err)
		}

		return id, nil
	case hypothesislaunch.ResourceAudienceTarget:
		adGroupID, err := s.launches.DependencyExternalID(ctx, resource.LaunchID, hypothesislaunch.ResourceAdGroup, resource.LogicalKey)
		if err != nil {
			return 0, fmt.Errorf("get ad group dependency for audience target resource %s: %w", resource.ID, err)
		}

		retargetingListID, err := s.launches.DependencyExternalID(ctx, resource.LaunchID, hypothesislaunch.ResourceRetargetingList, resource.LogicalKey)
		if err != nil {
			return 0, fmt.Errorf("get retargeting list dependency for audience target resource %s: %w", resource.ID, err)
		}

		id, found, err := s.yandex.FindAudienceTarget(ctx, adGroupID, retargetingListID)
		if err != nil {
			return 0, fmt.Errorf("find audience target resource %s: %w", resource.ID, err)
		}

		if found {
			return id, nil
		}

		if err := s.prepareHypothesisLaunchResourceCreate(ctx, resource); err != nil {
			return 0, err
		}

		id, err = s.yandex.CreateAudienceTarget(ctx, adGroupID, retargetingListID)
		if err != nil {
			return 0, fmt.Errorf("create audience target resource %s: %w", resource.ID, err)
		}

		return id, nil
	default:
		return 0, fmt.Errorf("unsupported hypothesis launch resource kind %q", resource.Kind)
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

	if resource.ExternalID == nil || resource.Status == "deleted" {
		return nil
	}

	service, objectName, deleteResource, err := s.launchResourceDelete(resource)
	if err != nil {
		return err
	}

	exists, err := s.yandex.Exists(ctx, service, objectName, *resource.ExternalID)
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

func (s *HypothesisService) launchResourceDelete(resource *hypothesislaunch.Resource) (string, string, func(context.Context, int64) error, error) {
	switch resource.Kind {
	case hypothesislaunch.ResourceCampaign:
		return "campaigns", "Campaigns", s.yandex.DeleteCampaign, nil
	case hypothesislaunch.ResourceRetargetingList:
		return "retargetinglists", "RetargetingLists", s.yandex.DeleteRetargetingList, nil
	case hypothesislaunch.ResourceAdGroup:
		return "adgroups", "AdGroups", s.yandex.DeleteAdGroup, nil
	case hypothesislaunch.ResourceAudienceTarget:
		return "audiencetargets", "AudienceTargets", s.yandex.DeleteAudienceTarget, nil
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
