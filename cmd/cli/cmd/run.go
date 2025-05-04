package cmd

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	testFile     string
	testName     string
	outputFormat string
	outputFile   string
)

type JUnitTestSuite struct {
	XMLName    xml.Name        `xml:"testsuite"`
	Name       string          `xml:"name,attr"`
	Tests      int             `xml:"tests,attr"`
	Failures   int             `xml:"failures,attr"`
	Errors     int             `xml:"errors,attr"`
	Time       float64         `xml:"time,attr"`
	Timestamp  string          `xml:"timestamp,attr"`
	TestCases  []JUnitTestCase `xml:"testcase"`
}

type JUnitTestCase struct {
	Name      string       `xml:"name,attr"`
	Classname string       `xml:"classname,attr"`
	Time      float64      `xml:"time,attr"`
	Failure   *JUnitFailure `xml:"failure,omitempty"`
}

type JUnitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Content string `xml:",chardata"`
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a test",
	Long: `Run a test defined in a YAML file.
This will POST the YAML to the Engine, stream logs, and set the exit code.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		yamlData, err := os.ReadFile(testFile)
		if err != nil {
			return fmt.Errorf("failed to read YAML file: %w", err)
		}

		conn, err := grpc.NewClient("localhost:7700", grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return fmt.Errorf("failed to connect to Engine: %w", err)
		}
		defer func() { _ = conn.Close() }()

		client := generated.NewEngineClient(conn)

		ctx := context.Background()
		resp, err := client.CreateRun(ctx, &generated.CreateRunRequest{
			YamlPayload: yamlData,
		})
		if err != nil {
			return fmt.Errorf("failed to create run: %w", err)
		}

		runID := resp.RunId
		fmt.Printf("Run created with ID: %s\n", runID)

		streamCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		stream, err := client.StreamLogs(streamCtx, &generated.LogStreamRequest{
			RunId: runID,
		})
		if err != nil {
			return fmt.Errorf("failed to stream logs: %w", err)
		}

		var logs []string
		startTime := time.Now()

		for {
			logLine, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("error streaming logs: %w", err)
			}

			fmt.Printf("%s %s\n", logLine.Ts, logLine.Msg)

			logs = append(logs, logLine.Msg)

		}

		listResp, err := client.ListRuns(ctx, &generated.ListRunsRequest{})
		if err != nil {
			return fmt.Errorf("failed to get run status: %w", err)
		}

		var status string
		for _, run := range listResp.Runs {
			if run.RunId == runID {
				status = run.Status
				break
			}
		}

		if outputFormat == "junit" && outputFile != "" {
			if err := generateJUnitOutput(outputFile, status, logs, startTime); err != nil {
				return fmt.Errorf("failed to generate JUnit output: %w", err)
			}
		}

		if status != "PASSED" {
			return fmt.Errorf("test failed with status: %s", status)
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVar(&testFile, "file", "", "Path to YAML file")
	runCmd.Flags().StringVar(&testName, "test", "", "Name of test to run")
	runCmd.Flags().StringVar(&outputFormat, "format", "", "Output format (junit)")
	runCmd.Flags().StringVar(&outputFile, "output", "", "Output file")

	if err := runCmd.MarkFlagRequired("file"); err != nil {
		return
	}
}

func generateJUnitOutput(outputFile string, status string, logs []string, startTime time.Time) error {
	suite := JUnitTestSuite{
		Name:      "Rocketship",
		Tests:     1,
		Failures:  0,
		Errors:    0,
		Time:      time.Since(startTime).Seconds(),
		Timestamp: startTime.Format(time.RFC3339),
		TestCases: []JUnitTestCase{
			{
				Name:      "RocketshipTest",
				Classname: "Rocketship",
				Time:      time.Since(startTime).Seconds(),
			},
		},
	}

	if status != "PASSED" {
		suite.Failures = 1
		suite.TestCases[0].Failure = &JUnitFailure{
			Message: "Test failed",
			Type:    "TestFailure",
			Content: fmt.Sprintf("Test failed with status: %s\n%s", status, formatLogs(logs)),
		}
	}

	outputDir := filepath.Dir(outputFile)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() { _ = file.Close() }()

	if _, err := file.WriteString(xml.Header); err != nil {
		return fmt.Errorf("failed to write XML header: %w", err)
	}

	encoder := xml.NewEncoder(file)
	encoder.Indent("", "  ")
	if err := encoder.Encode(suite); err != nil {
		return fmt.Errorf("failed to encode JUnit XML: %w", err)
	}

	return nil
}

func formatLogs(logs []string) string {
	var result string
	for _, log := range logs {
		result += log + "\n"
	}
	return result
}
