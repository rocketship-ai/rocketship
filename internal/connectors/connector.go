package connectors

import (
	"context"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
)

type Connector interface {
	Name() string
	Activity(ctx context.Context, p map[string]interface{}) (interface{}, error)
}

func Register(w worker.Worker, c Connector) {
	w.RegisterActivityWithOptions(
		c.Activity,
		activity.RegisterOptions{Name: c.Name()},
	)
}
