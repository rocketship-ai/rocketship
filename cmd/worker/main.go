package main

import (
	"os"

	"github.com/rocketship-ai/rocketship/internal/cli"
	"github.com/rocketship-ai/rocketship/internal/interpreter"
	"github.com/rocketship-ai/rocketship/internal/plugins"

	// Import plugins to trigger auto-registration
	_ "github.com/rocketship-ai/rocketship/internal/plugins/agent"
	_ "github.com/rocketship-ai/rocketship/internal/plugins/browser_use"
	_ "github.com/rocketship-ai/rocketship/internal/plugins/delay"
	_ "github.com/rocketship-ai/rocketship/internal/plugins/http"
	_ "github.com/rocketship-ai/rocketship/internal/plugins/log"
	_ "github.com/rocketship-ai/rocketship/internal/plugins/playwright"
	_ "github.com/rocketship-ai/rocketship/internal/plugins/script"
	_ "github.com/rocketship-ai/rocketship/internal/plugins/sql"
	_ "github.com/rocketship-ai/rocketship/internal/plugins/supabase"

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
	temporalNamespace := os.Getenv("TEMPORAL_NAMESPACE")
	if temporalNamespace == "" {
		temporalNamespace = "default"
	}

	logger.Debug("connecting to temporal", "host", temporalHost)
	c, err := client.Dial(client.Options{
		HostPort:  temporalHost,
		Namespace: temporalNamespace,
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
	w.RegisterWorkflow(interpreter.SuiteCleanupWorkflow)
	w.RegisterActivity(interpreter.LogForwarderActivity)
	w.RegisterActivity(interpreter.StepReporterActivity)
	w.RegisterActivity(interpreter.TemplateResolverActivity)

	plugins.RegisterAllWithTemporal(w)

	logger.Info("starting worker")
	if err := w.Run(worker.InterruptCh()); err != nil {
		logger.Error("failed to start worker", "error", err)
		os.Exit(1)
	}
}
