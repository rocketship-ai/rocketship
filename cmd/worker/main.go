package main

import (
	"log"
	"os"

	"github.com/rocketship-ai/rocketship/internal/interpreter"
	"github.com/rocketship-ai/rocketship/internal/plugins"
	"github.com/rocketship-ai/rocketship/internal/plugins/delay"
	"github.com/rocketship-ai/rocketship/internal/plugins/http"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	temporalHost := os.Getenv("TEMPORAL_HOST")
	if temporalHost == "" {
		panic("TEMPORAL_HOST is not set")
	}

	c, err := client.Dial(client.Options{
		HostPort: temporalHost,
	})
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()

	w := worker.New(c, "test-workflows", worker.Options{})

	w.RegisterWorkflow(interpreter.TestWorkflow)

	plugins.RegisterWithTemporal(w, &delay.DelayPlugin{})
	plugins.RegisterWithTemporal(w, &http.HTTPPlugin{})

	log.Println("Starting worker")
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("Failed to start worker: %v", err)
	}
}
