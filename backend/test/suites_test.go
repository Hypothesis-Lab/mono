package test

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"app/test/harness"
	"app/test/suites/audience"
	"app/test/suites/auth"
	"app/test/suites/file"
	"app/test/suites/hypothesis"
	"app/test/suites/http"
	"app/test/suites/product"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestSuites(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	env := harness.NewEnv()

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		defer cleanupCancel()
		env.Cleanup(cleanupCtx)
	})

	go func() {
		<-sigCh
		cancel()

		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		defer cleanupCancel()
		env.Cleanup(cleanupCtx)

		os.Exit(1)
	}()

	err := env.SetupEnvironment(ctx)
	require.NoError(t, err)

	harness.SetEnv(env)

	t.Run("AuthStateful", func(t *testing.T) {
		suite.Run(t, auth.NewStatefulSuite())
	})

	t.Run("AudienceStateful", func(t *testing.T) {
		suite.Run(t, audience.NewStatefulSuite())
	})

	t.Run("FileStateful", func(t *testing.T) {
		suite.Run(t, file.NewStatefulSuite())
	})

	t.Run("HypothesisStateful", func(t *testing.T) {
		suite.Run(t, hypothesis.NewStatefulSuite())
	})

	t.Run("HTTPStateless", func(t *testing.T) {
		t.Parallel()
		suite.Run(t, http.NewStatelessSuite())
	})

	t.Run("AuthStateless", func(t *testing.T) {
		t.Parallel()
		suite.Run(t, auth.NewStatelessSuite())
	})

	t.Run("ProductStateful", func(t *testing.T) {
		suite.Run(t, product.NewStatefulSuite())
	})
}
