package http

import "app/test/harness"

type StatelessSuite struct {
	harness.StatelessIntegrationSuite
}

func NewStatelessSuite() *StatelessSuite {
	return &StatelessSuite{}
}
