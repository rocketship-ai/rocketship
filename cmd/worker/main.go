package main

import (
	"os"

	"github.com/rocketship-ai/rocketship/internal/cli"
	"github.com/rocketship-ai/rocketship/internal/interpreter"
	"github.com/rocketship-ai/rocketship/internal/plugins"
	"github.com/rocketship-ai/rocketship/internal/plugins/delay"
	"github.com/rocketship-ai/rocketship/internal/plugins/http"
	"github.com/rocketship-ai/rocketship/internal/plugins/script"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	// Initialize logging
	cli.InitLogging()
	logger := cli.Logger

	temporalHost := os.Getenv("TEMPORAL_HOST")
	if temporalHost == "" {
		logger.Error("TEMPORAL_HOST environment variable is not set")
		os.Exit(1)
	}

	logger.Debug("connecting to temporal", "host", temporalHost)
	c, err := client.Dial(client.Options{
		HostPort: temporalHost,
	})
	if err != nil {
		logger.Error("failed to create temporal client", "error", err)
		os.Exit(1)
	}
	defer c.Close()

	logger.Debug("creating worker for task queue", "queue", "test-workflows")
	w := worker.New(c, "test-workflows", worker.Options{})

	logger.Debug("registering workflow and plugins")
	w.RegisterWorkflow(interpreter.TestWorkflow)

	plugins.RegisterWithTemporal(w, &delay.DelayPlugin{})
	plugins.RegisterWithTemporal(w, &http.HTTPPlugin{})
	plugins.RegisterWithTemporal(w, &script.ScriptPlugin{})

	logger.Info("starting worker")
	if err := w.Run(worker.InterruptCh()); err != nil {
		logger.Error("failed to start worker", "error", err)
		os.Exit(1)
	}
}
