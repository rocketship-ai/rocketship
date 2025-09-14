package main

import (
	"os"

	"github.com/rocketship-ai/rocketship/internal/cli"
	"github.com/rocketship-ai/rocketship/internal/interpreter"
	"github.com/rocketship-ai/rocketship/internal/plugins"
	
	// Import plugins to trigger auto-registration
	_ "github.com/rocketship-ai/rocketship/internal/plugins/agent"
	_ "github.com/rocketship-ai/rocketship/internal/plugins/browser"
	_ "github.com/rocketship-ai/rocketship/internal/plugins/delay"
	_ "github.com/rocketship-ai/rocketship/internal/plugins/http"
	_ "github.com/rocketship-ai/rocketship/internal/plugins/log"
	_ "github.com/rocketship-ai/rocketship/internal/plugins/script"
	_ "github.com/rocketship-ai/rocketship/internal/plugins/sql"
	_ "github.com/rocketship-ai/rocketship/internal/plugins/supabase"
	_ "github.com/rocketship-ai/rocketship/internal/plugins/s3"

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
	w.RegisterActivity(interpreter.LogForwarderActivity)

	plugins.RegisterAllWithTemporal(w)

	logger.Info("starting worker")
	if err := w.Run(worker.InterruptCh()); err != nil {
		logger.Error("failed to start worker", "error", err)
		os.Exit(1)
	}
}
