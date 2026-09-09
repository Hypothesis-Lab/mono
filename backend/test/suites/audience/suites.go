package audience

import "app/test/harness"

type StatefulSuite struct {
	harness.IntegrationSuite
}

type StatelessSuite struct {
	harness.StatelessIntegrationSuite
}

func NewStatefulSuite() *StatefulSuite {
	return &StatefulSuite{}
}

func NewStatelessSuite() *StatelessSuite {
	return &StatelessSuite{}
}
