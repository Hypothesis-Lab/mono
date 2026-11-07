package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"app/internal/clients/yandex_direct"
	"app/internal/repositories/adcampaign"
	"app/internal/repositories/audience"
	"app/internal/repositories/file"
	"app/internal/repositories/hypothesis"
	"app/internal/repositories/hypothesislaunch"
	"app/internal/repositories/product"
	"app/internal/storage"
)

const russiaYandexRegionID int64 = 225

var (
	ErrAlreadyInProgress    = errors.New("hypothesis already in progress")
	ErrHypothesisNoProduct  = errors.New("hypothesis has no product")
	ErrUpdateInProgress     = errors.New("cannot update hypothesis in progress")
	ErrDeleteInProgress     = errors.New("cannot delete hypothesis in progress")
	ErrStopLaunchInProgress = errors.New("cannot stop hypothesis while launch is in progress")
	ErrStopInProgress       = errors.New("hypothesis stop is already in progress")
	ErrStopManual           = errors.New("hypothesis requires manual intervention")
	ErrLaunchNotFound       = errors.New("hypothesis launch not found")
	ErrHypothesisNoAudience = errors.New("hypothesis product has no generated audiences")
)

type HypothesisService struct {
	hypotheses  *hypothesis.Repository
	products    *product.Repository
	audiences   *audience.Repository
	files       *file.Repository
	storage     *storage.S3
	campaigns   *adcampaign.Repository
	connections *YandexDirectConnectionService
	launches    *hypothesislaunch.Repository
}

type createdAudienceTargeting struct {
	adGroupID         int64
	retargetingListID int64
	audienceTargetID  int64
}

type LaunchStatus struct {
	WorkflowID string
	Status     string
}

type StopStatus struct {
	Hypothesis *hypothesis.Hypothesis
	WorkflowID string
}

func NewHypothesisService(
	hypotheses *hypothesis.Repository,
	products *product.Repository,
	audiences *audience.Repository,
	files *file.Repository,
	storage *storage.S3,
	campaigns *adcampaign.Repository,
	connections *YandexDirectConnectionService,
	launches *hypothesislaunch.Repository,
) *HypothesisService {
	return &HypothesisService{
		hypotheses:  hypotheses,
		products:    products,
		audiences:   audiences,
		files:       files,
		storage:     storage,
		campaigns:   campaigns,
		connections: connections,
		launches:    launches,
	}
}

func (s *HypothesisService) Create(ctx context.Context, userID, description string) (*hypothesis.Hypothesis, error) {
	created, err := s.hypotheses.Create(ctx, hypothesis.CreateInput{
		UserID:      userID,
		Description: description,
	})
	if err != nil {
		return nil, fmt.Errorf("creating hypothesis: %w", err)
	}

	return created, nil
}

func (s *HypothesisService) List(ctx context.Context, userID string) ([]*hypothesis.Hypothesis, error) {
	items, err := s.hypotheses.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing hypotheses: %w", err)
	}

	return items, nil
}

func (s *HypothesisService) Get(ctx context.Context, userID, id string) (*hypothesis.Hypothesis, error) {
	h, err := s.hypotheses.Get(ctx, userID, id)
	if err != nil {
		if errors.Is(err, hypothesis.ErrNotFound) {
			return nil, hypothesis.ErrNotFound
		}

		return nil, fmt.Errorf("getting hypothesis: %w", err)
	}

	return h, nil
}

func (s *HypothesisService) GetProduct(ctx context.Context, userID, id string) (*product.Product, error) {
	if _, err := s.hypotheses.Get(ctx, userID, id); err != nil {
		if errors.Is(err, hypothesis.ErrNotFound) {
			return nil, hypothesis.ErrNotFound
		}

		return nil, fmt.Errorf("getting hypothesis: %w", err)
	}

	p, err := s.products.GetByHypothesisID(ctx, id)
	if err != nil {
		if errors.Is(err, product.ErrNotFound) {
			return nil, product.ErrNotFound
		}

		return nil, fmt.Errorf("getting product: %w", err)
	}

	return p, nil
}

func (s *HypothesisService) StartLaunch(ctx context.Context, userID, id string) (string, error) {
	if _, err := s.hypotheses.Get(ctx, userID, id); err != nil {
		if errors.Is(err, hypothesis.ErrNotFound) {
			return "", hypothesis.ErrNotFound
		}

		return "", fmt.Errorf("get hypothesis before launch: %w", err)
	}

	if err := s.connections.EnsureConnected(ctx, userID); err != nil {
		return "", fmt.Errorf("check yandex direct connection before launch: %w", err)
	}

	launch, err := s.launches.Create(ctx, userID, id, time.Now().Format(time.DateOnly))
	if err != nil {
		switch {
		case errors.Is(err, hypothesislaunch.ErrHypothesisGone):
			return "", hypothesis.ErrNotFound
		case errors.Is(err, hypothesislaunch.ErrAlreadyActive):
			return "", ErrAlreadyInProgress
		case errors.Is(err, hypothesislaunch.ErrNoProduct):
			return "", ErrHypothesisNoProduct
		case errors.Is(err, hypothesislaunch.ErrNoAudiences):
			return "", ErrHypothesisNoAudience
		default:
			return "", fmt.Errorf("create hypothesis launch: %w", err)
		}
	}

	return launch.WorkflowID, nil
}

func (s *HypothesisService) GetLaunchStatus(ctx context.Context, userID, id string) (*LaunchStatus, error) {
	if _, err := s.hypotheses.Get(ctx, userID, id); err != nil {
		if errors.Is(err, hypothesis.ErrNotFound) {
			return nil, hypothesis.ErrNotFound
		}

		return nil, fmt.Errorf("get hypothesis launch status: %w", err)
	}

	launch, err := s.launches.GetLatest(ctx, userID, id)
	if err != nil {
		if errors.Is(err, hypothesislaunch.ErrNotFound) {
			return nil, ErrLaunchNotFound
		}

		return nil, fmt.Errorf("get hypothesis launch status: %w", err)
	}

	return &LaunchStatus{WorkflowID: launch.WorkflowID, Status: launchWorkflowStatus(launch.Status)}, nil
}

func launchWorkflowStatus(status string) string {
	switch status {
	case "queued", "running":
		return status
	case "succeeded":
		return "completed"
	default:
		return "failed"
	}
}

func (s *HypothesisService) Stop(ctx context.Context, userID, id string) (*StopStatus, error) {
	h, err := s.hypotheses.Get(ctx, userID, id)
	if err != nil {
		if errors.Is(err, hypothesis.ErrNotFound) {
			return nil, hypothesis.ErrNotFound
		}

		return nil, fmt.Errorf("getting hypothesis: %w", err)
	}

	if h.Status == hypothesis.StatusIdle {
		return &StopStatus{Hypothesis: h}, nil
	}

	if h.Status == hypothesis.StatusLaunching {
		return nil, ErrStopLaunchInProgress
	}

	if h.Status == hypothesis.StatusStopping {
		return nil, ErrStopInProgress
	}

	if h.Status == hypothesis.StatusManual {
		return nil, ErrStopManual
	}

	if err := s.connections.EnsureConnected(ctx, userID); err != nil {
		return nil, fmt.Errorf("check yandex direct connection before stop: %w", err)
	}

	stop, err := s.launches.CreateStop(ctx, userID, id)
	if err != nil {
		switch {
		case errors.Is(err, hypothesislaunch.ErrHypothesisGone):
			return nil, hypothesis.ErrNotFound
		case errors.Is(err, hypothesislaunch.ErrStopActive):
			return nil, ErrStopInProgress
		case errors.Is(err, hypothesislaunch.ErrStopManual):
			return nil, ErrStopManual
		default:
			return nil, fmt.Errorf("create hypothesis stop: %w", err)
		}
	}

	return &StopStatus{WorkflowID: stop.WorkflowID}, nil
}

func (s *HypothesisService) Update(ctx context.Context, userID, id string, in hypothesis.UpdateInput) (*hypothesis.Hypothesis, error) {
	h, err := s.hypotheses.Get(ctx, userID, id)
	if err != nil {
		if errors.Is(err, hypothesis.ErrNotFound) {
			return nil, hypothesis.ErrNotFound
		}

		return nil, fmt.Errorf("getting hypothesis: %w", err)
	}

	if h.Status != hypothesis.StatusIdle {
		return nil, ErrUpdateInProgress
	}

	updated, err := s.hypotheses.Update(ctx, userID, id, in)
	if err != nil {
		if errors.Is(err, hypothesis.ErrNotFound) {
			return nil, hypothesis.ErrNotFound
		}

		return nil, fmt.Errorf("updating hypothesis: %w", err)
	}

	return updated, nil
}

func (s *HypothesisService) Delete(ctx context.Context, userID, id string) error {
	h, err := s.hypotheses.Get(ctx, userID, id)
	if err != nil {
		if errors.Is(err, hypothesis.ErrNotFound) {
			return hypothesis.ErrNotFound
		}

		return fmt.Errorf("getting hypothesis: %w", err)
	}

	if h.Status != hypothesis.StatusIdle {
		return ErrDeleteInProgress
	}

	files, err := s.files.ListByHypothesisID(ctx, id)
	if err != nil {
		return fmt.Errorf("listing files: %w", err)
	}

	for _, f := range files {
		if err := s.storage.Delete(ctx, f.S3Key); err != nil {
			return fmt.Errorf("deleting file: %w", err)
		}

		if err := s.storage.Delete(ctx, f.S3Key+".extracted.txt"); err != nil {
			return fmt.Errorf("deleting extracted hypothesis file text: %w", err)
		}
	}

	if err := s.hypotheses.Delete(ctx, userID, id); err != nil {
		return fmt.Errorf("deleting hypothesis: %w", err)
	}

	return nil
}

func (s *HypothesisService) yandexClientForLaunch(ctx context.Context, launchID, userID string) (*yandexdirect.Client, error) {
	if userID == "" {
		var err error
		userID, err = s.launches.UserIDForLaunch(ctx, launchID)
		if err != nil {
			return nil, fmt.Errorf("resolve hypothesis launch user: %w", err)
		}
	}

	yandex, err := s.connections.NewClient(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("create yandex direct client for hypothesis launch: %w", err)
	}

	return yandex, nil
}

func (s *HypothesisService) launchHypothesisActivity(ctx context.Context, input hypothesisLaunchInput) error {
	userID := input.UserID
	yandex, err := s.yandexClientForLaunch(ctx, input.LaunchID, userID)
	if err != nil {
		return err
	}

	if userID == "" {
		userID, err = s.launches.UserIDForLaunch(ctx, input.LaunchID)
		if err != nil {
			return fmt.Errorf("resolve hypothesis launch user for status: %w", err)
		}
	}

	p, err := s.products.GetByHypothesisID(ctx, input.HypothesisID)
	if err != nil {
		if errors.Is(err, product.ErrNotFound) {
			return ErrHypothesisNoProduct
		}

		return fmt.Errorf("get product in launch activity: %w", err)
	}

	campaignID, err := yandex.CreateCampaign(ctx, p.Name, time.Now().Format(time.DateOnly))
	if err != nil {
		return fmt.Errorf("create yandex campaign in launch activity: %w", err)
	}

	if err := s.campaigns.Create(ctx, adcampaign.Campaign{
		ProductID:  p.ID,
		Provider:   adcampaign.ProviderYandex,
		ExternalID: campaignID,
	}); err != nil {
		if deleteErr := yandex.DeleteCampaign(ctx, campaignID); deleteErr != nil {
			return fmt.Errorf("store yandex campaign %d in launch activity: %w", campaignID, errors.Join(err, fmt.Errorf("delete created yandex campaign: %w", deleteErr)))
		}

		return fmt.Errorf("store yandex campaign %d in launch activity: %w", campaignID, err)
	}

	createdTargeting, err := s.createAudienceTargeting(ctx, yandex, p, campaignID)
	if err != nil {
		return s.rollbackLaunch(ctx, yandex, userID, input.HypothesisID, p.ID, campaignID, createdTargeting, err)
	}

	if _, err := s.hypotheses.SetStatus(ctx, userID, input.HypothesisID, hypothesis.StatusInProgress); err != nil {
		return s.rollbackLaunch(ctx, yandex, userID, input.HypothesisID, p.ID, campaignID, createdTargeting, fmt.Errorf("mark hypothesis in progress after launch: %w", err))
	}

	return nil
}

func (s *HypothesisService) resetHypothesisLaunchStatusActivity(ctx context.Context, input hypothesisLaunchInput) error {
	userID := input.UserID
	if userID == "" {
		var err error
		userID, err = s.launches.UserIDForLaunch(ctx, input.LaunchID)
		if err != nil {
			return fmt.Errorf("resolve hypothesis launch user for reset: %w", err)
		}
	}

	if _, err := s.hypotheses.SetStatus(ctx, userID, input.HypothesisID, hypothesis.StatusIdle); err != nil {
		return fmt.Errorf("reset hypothesis status after launch: %w", err)
	}

	return nil
}

func (s *HypothesisService) createAudienceTargeting(ctx context.Context, yandex *yandexdirect.Client, p *product.Product, campaignID int64) ([]createdAudienceTargeting, error) {
	createdTargeting := []createdAudienceTargeting{}
	audiences, err := s.audiences.ListByProductID(ctx, p.ID)
	if err != nil {
		return createdTargeting, fmt.Errorf("list product audiences: %w", err)
	}

	if len(audiences) == 0 {
		return createdTargeting, errors.New("product has no generated audiences")
	}

	for _, audience := range audiences {
		var persona struct {
			Gender string `json:"gender"`
			Age    int    `json:"age"`
		}
		if err := json.Unmarshal(audience.Persona, &persona); err != nil {
			return createdTargeting, fmt.Errorf("decode audience %s persona: %w", audience.ID, err)
		}

		retargetingListID, err := yandex.CreateDemographicRetargetingList(ctx, "product-"+p.ID+"-"+audience.PersonaID, persona.Gender, persona.Age)
		if err != nil {
			return createdTargeting, fmt.Errorf("create retargeting list for audience %s: %w", audience.ID, err)
		}

		createdTargeting = append(createdTargeting, createdAudienceTargeting{retargetingListID: retargetingListID})

		adGroupID, err := yandex.CreateUserProfileAdGroup(ctx, campaignID, "product-"+p.ID+"-"+audience.PersonaID, []int64{russiaYandexRegionID})
		if err != nil {
			return createdTargeting, fmt.Errorf("create ad group for audience %s: %w", audience.ID, err)
		}

		createdTargeting[len(createdTargeting)-1].adGroupID = adGroupID

		audienceTargetID, err := yandex.CreateAudienceTarget(ctx, adGroupID, retargetingListID)
		if err != nil {
			return createdTargeting, fmt.Errorf("create audience target for audience %s: %w", audience.ID, err)
		}

		createdTargeting[len(createdTargeting)-1].audienceTargetID = audienceTargetID
	}

	return createdTargeting, nil
}

func (s *HypothesisService) rollbackLaunch(ctx context.Context, yandex *yandexdirect.Client, userID, hypothesisID, productID string, campaignID int64, targetings []createdAudienceTargeting, launchErr error) error {
	for index := len(targetings) - 1; index >= 0; index-- {
		targeting := targetings[index]
		launchErr = deleteAudienceTargeting(ctx, yandex, targeting, launchErr)
	}

	if deleteErr := yandex.DeleteCampaign(ctx, campaignID); deleteErr != nil {
		return fmt.Errorf("%w; delete yandex campaign: %w", launchErr, deleteErr)
	}

	if deleteErr := s.campaigns.Delete(ctx, productID, adcampaign.ProviderYandex); deleteErr != nil {
		return fmt.Errorf("%w; delete local campaign: %w", launchErr, deleteErr)
	}

	if _, err := s.hypotheses.SetStatus(ctx, userID, hypothesisID, hypothesis.StatusIdle); err != nil {
		return fmt.Errorf("%w; reset hypothesis status: %w", launchErr, err)
	}

	return launchErr
}

func deleteAudienceTargeting(ctx context.Context, yandex *yandexdirect.Client, targeting createdAudienceTargeting, launchErr error) error {
	if targeting.audienceTargetID != 0 {
		if err := yandex.DeleteAudienceTarget(ctx, targeting.audienceTargetID); err != nil {
			launchErr = fmt.Errorf("%w; delete audience target: %w", launchErr, err)
		}
	}

	if targeting.adGroupID != 0 {
		if err := yandex.DeleteAdGroup(ctx, targeting.adGroupID); err != nil {
			launchErr = fmt.Errorf("%w; delete ad group: %w", launchErr, err)
		}
	}

	if targeting.retargetingListID != 0 {
		if err := yandex.DeleteRetargetingList(ctx, targeting.retargetingListID); err != nil {
			launchErr = fmt.Errorf("%w; delete retargeting list: %w", launchErr, err)
		}
	}

	return launchErr
}
