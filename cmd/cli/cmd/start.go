package cmd

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var (
	composeFile string
	ciMode      bool
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Rocketship runtime",
	Long: `Start the Rocketship runtime using Docker Compose.
This will spin up Temporal, Engine, Worker, and LocalStack containers.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dockerDir := ".docker"
		if _, err := os.Stat(dockerDir); os.IsNotExist(err) {
			if err := os.Mkdir(dockerDir, 0755); err != nil {
				return fmt.Errorf("failed to create .docker directory: %w", err)
			}
		}

		if _, err := os.Stat(filepath.Join(dockerDir, "docker-compose.yaml")); os.IsNotExist(err) {
			fmt.Println("Generating docker-compose.yaml...")
			if err := generateDockerComposeFile(dockerDir); err != nil {
				return fmt.Errorf("failed to generate docker-compose.yaml: %w", err)
			}
		}

		if _, err := os.Stat(".env.rocketship"); os.IsNotExist(err) {
			fmt.Println("Generating .env.rocketship...")
			if err := generateEnvFile(); err != nil {
				return fmt.Errorf("failed to generate .env.rocketship: %w", err)
			}
		}

		fmt.Println("Starting Docker Compose...")
		composeCmd := exec.Command("docker", "compose", "-f", composeFile, "up", "-d")
		composeCmd.Stdout = os.Stdout
		composeCmd.Stderr = os.Stderr
		if err := composeCmd.Run(); err != nil {
			return fmt.Errorf("failed to start Docker Compose: %w", err)
		}

		fmt.Println("Waiting for Engine to be ready...")
		if err := waitForEngine(); err != nil {
			return fmt.Errorf("failed to wait for Engine: %w", err)
		}

		fmt.Println("Rocketship runtime is ready!")
		return nil
	},
}

func init() {
	RootCmd.AddCommand(startCmd)

	startCmd.Flags().StringVar(&composeFile, "compose-file", ".docker/docker-compose.yaml", "Path to Docker Compose file")
	startCmd.Flags().BoolVar(&ciMode, "ci", false, "Run in CI mode")
}

func generateDockerComposeFile(dockerDir string) error {
	content := `version: "3.9"
services:
  temporal:
    image: temporalio/server:1.23
    ports: ["7233:7233"]
    environment:
      - DB=sqlite
  engine:
    build:
      context: .
      dockerfile: Dockerfile.engine
    depends_on: [temporal]
    environment:
      - TEMPORAL_HOST=temporal:7233
    ports: ["7700:7700", "7701:7701"]
  worker:
    build:
      context: .
      dockerfile: Dockerfile.worker
    depends_on: [temporal, engine]
    environment:
      - TEMPORAL_HOST=temporal:7233
  localstack:
    image: localstack/localstack:3
    ports: ["4566:4566"]
`
	return os.WriteFile(filepath.Join(dockerDir, "docker-compose.yaml"), []byte(content), 0644)
}

func generateEnvFile() error {
	content := `# AWS credentials for LocalStack
AWS_ACCESS_KEY_ID=test
AWS_SECRET_ACCESS_KEY=test
AWS_DEFAULT_REGION=us-east-1

# Service ports
TEMPORAL_PORT=7233
ENGINE_GRPC_PORT=7700
ENGINE_HTTP_PORT=7701
LOCALSTACK_PORT=4566

# Service versions
TEMPORAL_VERSION=1.23
LOCALSTACK_VERSION=3
`
	return os.WriteFile(".env.rocketship", []byte(content), 0644)
}

func waitForEngine() error {
	healthzURL := "http://localhost:7701/healthz"
	maxRetries := 60
	retryInterval := time.Second

	for i := 0; i < maxRetries; i++ {
		resp, err := http.Get(healthzURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			return nil
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(retryInterval)
	}

	return fmt.Errorf("engine not ready after %d seconds", maxRetries)
}
