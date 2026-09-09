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

type IntegrationSuite struct {
	suite.Suite
}

func (s *IntegrationSuite) SetupTest() {
	resetIntegrationState(s.T())
}

func (s *IntegrationSuite) SetupSubTest() {
	resetIntegrationState(s.T())
}

type StatelessIntegrationSuite struct {
	suite.Suite
}

func resetIntegrationState(t *testing.T) {
	env := CurrentEnv()
	require.NotNil(t, env)

	require.NoError(t, resetDatabase(env.DatabaseURL()))
	require.NoError(t, resetS3(env.S3, env.S3Bucket()))
}
