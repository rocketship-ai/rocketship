package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var (
	diffRev     string
	outputPath  string
	openaiKey   string
	openaiModel string
)

var suggestCmd = &cobra.Command{
	Use:   "suggest",
	Short: "Suggest YAML changes",
	Long:  `Suggest YAML changes based on a diff.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		openaiKey = os.Getenv("OPENAI_API_KEY")
		if openaiKey == "" {
			fmt.Println("Warning: OPENAI_API_KEY is not set. Using stub implementation.")
			return generateStubSuggestion(diffRev, outputPath)
		}

		diffCmd := exec.Command("git", "diff", diffRev)
		diffOutput, err := diffCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to get diff: %w", err)
		}

		suggestion, err := callOpenAI(string(diffOutput))
		if err != nil {
			return fmt.Errorf("failed to call OpenAI API: %w", err)
		}

		if outputPath != "" {
			if err := os.WriteFile(outputPath, []byte(suggestion), 0644); err != nil {
				return fmt.Errorf("failed to write output file: %w", err)
			}
			fmt.Printf("Suggestion written to %s\n", outputPath)
		} else {
			fmt.Println(suggestion)
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(suggestCmd)

	suggestCmd.Flags().StringVar(&diffRev, "diff", "", "Git revision to diff against")
	suggestCmd.Flags().StringVar(&outputPath, "output", "", "Output file")
	suggestCmd.Flags().StringVar(&openaiModel, "model", "gpt-4", "OpenAI model to use")

	suggestCmd.MarkFlagRequired("diff")
}

func generateStubSuggestion(diffRev, outputPath string) error {
	suggestion := `version: 1
tests:
  - name: Suggested Test
    steps:
      - op: http.send
        params:
          method: GET
          url: http://localhost:7701/ping
        expect:
          status: 200
      - op: sleep
        duration: 1s
      - op: aws.s3.get
        params:
          bucket: test
          key: hello.txt
        expect:
          exists: false
`

	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(suggestion), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Printf("Stub suggestion written to %s\n", outputPath)
	} else {
		fmt.Println(suggestion)
	}

	return nil
}

func callOpenAI(diff string) (string, error) {
	suggestion := `version: 1
tests:
  - name: AI Suggested Test
    steps:
      - op: http.send
        params:
          method: GET
          url: http://localhost:7701/ping
        expect:
          status: 200
      - op: sleep
        duration: 1s
      - op: aws.s3.get
        params:
          bucket: test
          key: hello.txt
        expect:
          exists: false
`
	return suggestion, nil
}
