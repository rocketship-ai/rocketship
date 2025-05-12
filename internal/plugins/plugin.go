package plugins

import (
	"context"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
)

type Plugin interface {
	GetType() string
	Activity(ctx context.Context, p map[string]interface{}) (interface{}, error)
}

func RegisterWithTemporal(w worker.Worker, c Plugin) {
	w.RegisterActivityWithOptions(
		c.Activity,
		activity.RegisterOptions{Name: c.GetType()},
	)
}
