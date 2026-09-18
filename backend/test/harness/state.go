package harness

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

var (
	envMu      sync.RWMutex
	currentEnv *Env
)

type IntegrationSuite struct {
	suite.Suite
}

type StatelessIntegrationSuite struct {
	suite.Suite
}

func (s *IntegrationSuite) SetupTest() {
	resetIntegrationState(s.T())
}

func (s *IntegrationSuite) SetupSubTest() {
	resetIntegrationState(s.T())
}

func SetEnv(env *Env) {
	envMu.Lock()
	defer envMu.Unlock()

	currentEnv = env
}

func CurrentEnv() *Env {
	envMu.RLock()
	defer envMu.RUnlock()

	return currentEnv
}

func resetIntegrationState(t *testing.T) {
	t.Helper()

	env := CurrentEnv()
	require.NotNil(t, env)

	require.NoError(t, resetDatabase(env.DatabaseURL()))
	require.NoError(t, resetS3(env.S3, env.S3Bucket()))
}
