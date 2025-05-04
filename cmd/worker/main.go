package main

import (
	"log"
	"os"

	"github.com/rocketship-ai/rocketship/internal/plugins"
	"github.com/rocketship-ai/rocketship/internal/plugins/aws/ddb"
	"github.com/rocketship-ai/rocketship/internal/plugins/aws/s3"
	"github.com/rocketship-ai/rocketship/internal/plugins/aws/sqs"
	"github.com/rocketship-ai/rocketship/internal/plugins/http"
	"github.com/rocketship-ai/rocketship/internal/interpreter"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	temporalHost := os.Getenv("TEMPORAL_HOST")
	if temporalHost == "" {
		temporalHost = "localhost:7233"
	}

	c, err := client.NewClient(client.Options{
		HostPort: temporalHost,
	})
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()

	w := worker.New(c, "test-workflows", worker.Options{})

	w.RegisterWorkflow(interpreter.TestWorkflow)

	plugins.RegisterWithTemporal(w, &http.HTTPPlugin{})
	plugins.RegisterWithTemporal(w, &s3.S3Plugin{})
	plugins.RegisterWithTemporal(w, &ddb.DynamoDBPlugin{})
	plugins.RegisterWithTemporal(w, &sqs.SQSPlugin{})

	log.Println("Starting worker")
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("Failed to start worker: %v", err)
	}
}
