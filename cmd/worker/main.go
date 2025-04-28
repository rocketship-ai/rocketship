package main

import (
	"log"
	"os"

	"github.com/rocketship-ai/rocketship/internal/connectors"
	"github.com/rocketship-ai/rocketship/internal/connectors/aws/ddb"
	"github.com/rocketship-ai/rocketship/internal/connectors/aws/s3"
	"github.com/rocketship-ai/rocketship/internal/connectors/aws/sqs"
	"github.com/rocketship-ai/rocketship/internal/connectors/http"
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

	connectors.RegisterWithTemporal(w, &http.HTTPConnector{})
	connectors.RegisterWithTemporal(w, &s3.S3Connector{})
	connectors.RegisterWithTemporal(w, &ddb.DynamoDBConnector{})
	connectors.RegisterWithTemporal(w, &sqs.SQSConnector{})

	log.Println("Starting worker")
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("Failed to start worker: %v", err)
	}
}
