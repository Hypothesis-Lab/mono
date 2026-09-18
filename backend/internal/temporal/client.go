package temporal

import (
	"context"

	"app/internal/config"

	"go.temporal.io/sdk/client"
	"go.uber.org/fx"
)

func NewClient(lc fx.Lifecycle, cfg *config.Config) (client.Client, error) {
	temporalClient, err := client.Dial(client.Options{HostPort: cfg.TemporalAddress})
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			temporalClient.Close()

			return nil
		},
	})

	return temporalClient, nil
}
