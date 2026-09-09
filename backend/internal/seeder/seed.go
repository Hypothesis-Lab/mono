package seeder

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"app/internal/config"
	"app/internal/db"
	"app/internal/repositories/user"
	"app/internal/services"
)

const (
	hypothesisStatusIdle       = "idle"
	hypothesisStatusInProgress = "in_progress"
)

var (
	productNamespace    = uuid.NewSHA1(uuid.NameSpaceDNS, []byte("app/product"))
	hypothesisNamespace = uuid.NewSHA1(uuid.NameSpaceDNS, []byte("app/hypothesis"))
)

type Seeder struct {
	cfg    *config.Config
	db     *db.DB
	auth   *services.UserAuthService
	users  *user.Repository
	logger *zap.Logger
}

func New(lc fx.Lifecycle, cfg *config.Config, database *db.DB, auth *services.UserAuthService, users *user.Repository, logger *zap.Logger) *Seeder {
	s := &Seeder{
		cfg:    cfg,
		db:     database,
		auth:   auth,
		users:  users,
		logger: logger,
	}

	if !cfg.DevSeedDB {
		return s
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return s.Seed(ctx)
		},
	})

	return s
}

func (s *Seeder) Seed(ctx context.Context) error {
	user, err := s.seedUser(ctx)
	if err != nil {
		return err
	}

	if err := s.seedHypotheses(ctx, user.ID); err != nil {
		return err
	}

	if err := s.seedProducts(ctx); err != nil {
		return err
	}

	s.logger.Info("database seeded")

	return nil
}

func (s *Seeder) seedHypotheses(ctx context.Context, userID string) error {
	const query = `
		INSERT INTO hypothesis (id, user_id, description, status)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO NOTHING`

	pool := s.db.Pool()

	rows := []struct {
		key         string
		description string
		status      string
	}{
		{key: "h1", description: "Users will pay more for a premium tier.", status: hypothesisStatusInProgress},
		{key: "h2", description: "Users can onboard without sales calls.", status: hypothesisStatusIdle},
		{key: "h3", description: "Most acquisition happens via mobile.", status: hypothesisStatusIdle},
	}

	for _, r := range rows {
		id := uuid.NewSHA1(hypothesisNamespace, []byte(r.key)).String()
		if _, err := pool.Exec(ctx, query, id, userID, r.description, r.status); err != nil {
			return fmt.Errorf("seed hypothesis %q: %w", r.key, err)
		}
	}

	return nil
}

func (s *Seeder) seedProducts(ctx context.Context) error {
	const query = `
		INSERT INTO product (id, name, description, category, keywords, hypothesis_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO NOTHING`

	pool := s.db.Pool()

	hypothesisH1 := uuid.NewSHA1(hypothesisNamespace, []byte("h1")).String()
	hypothesisH2 := uuid.NewSHA1(hypothesisNamespace, []byte("h2")).String()

	rows := []struct {
		key          string
		name         string
		description  *string
		category     string
		keywords     []string
		hypothesisID string
	}{
		{key: "p1", name: "Acme Platform", description: ptr("Self-serve SaaS for SMBs."), category: "office", keywords: []string{}, hypothesisID: hypothesisH1},
		{key: "p2", name: "Globex Suite", description: ptr("Enterprise workflow suite."), category: "office", keywords: []string{}, hypothesisID: hypothesisH2},
	}

	for _, r := range rows {
		id := uuid.NewSHA1(productNamespace, []byte(r.key)).String()
		if _, err := pool.Exec(ctx, query, id, r.name, r.description, r.category, r.keywords, r.hypothesisID); err != nil {
			return fmt.Errorf("seed product %q: %w", r.key, err)
		}
	}

	return nil
}

func (s *Seeder) seedUser(ctx context.Context) (*user.User, error) {
	const email = "a@a"

	_, err := s.auth.Register(ctx, email, "12341234")
	if err != nil {
		if !errors.Is(err, services.ErrEmailExists) {
			return nil, fmt.Errorf("seed user: %w", err)
		}
	} else {
		s.logger.Info("seeded default user")
	}

	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("get seeded user: %w", err)
	}

	return u, nil
}

func ptr[T any](v T) *T { return &v }
