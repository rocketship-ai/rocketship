package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rocketship-ai/rocketship/internal/dsl"
	"github.com/spf13/cobra"
)

// NewValidateCmd creates a new validate command
func NewValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [file_or_directory]",
		Short: "Validate Rocketship test files against the JSON schema",
		Long: `Validate one or more Rocketship test files against the JSON schema.
This command checks test file syntax, structure, and configuration without executing tests.

When validating a directory, Rocketship uses the same discovery logic as the run command:
- For a .rocketship directory, all *.yaml test files are validated (excluding .rocketship/tmp/)
- For other directories, only files named "rocketship.yaml" are validated

Examples:
  rocketship validate rocketship.yaml              # Validate a single file
  rocketship validate .rocketship                  # Validate all YAML test files in .rocketship
  rocketship validate ./tests/                     # Validate all rocketship.yaml files in directory
  rocketship validate test1.yaml test2.yaml        # Validate multiple files`,
		RunE: runValidate,
	}
}

func runValidate(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please specify at least one file or directory to validate")
	}

	var files []string
	totalValid := 0
	totalInvalid := 0

	// Collect all files to validate
	for _, arg := range args {
		stat, err := os.Stat(arg)
		if err != nil {
			Logger.Error("failed to access path", "path", arg, "error", err)
			totalInvalid++
			continue
		}

		if stat.IsDir() {
			// Match run command behavior:
			// - For .rocketship directories, validate all YAML test files (excluding .rocketship/tmp/)
			// - For other directories, only validate rocketship.yaml files
			var dirFiles []string
			if filepath.Base(arg) == ".rocketship" {
				dirFiles, err = findYamlTestFiles(arg)
			} else {
				dirFiles, err = findRocketshipFiles(arg)
			}
			if err != nil {
				Logger.Error("failed to scan directory", "path", arg, "error", err)
				totalInvalid++
				continue
			}
			files = append(files, dirFiles...)
		} else {
			files = append(files, arg)
		}
	}

	if len(files) == 0 {
		return fmt.Errorf("no Rocketship test files found to validate")
	}

	Logger.Info("validating files", "count", len(files))

	// Validate each file
	for _, file := range files {
		if err := validateFile(file); err != nil {
			Logger.Error("validation failed", "file", file, "error", err)
			totalInvalid++
		} else {
			Logger.Info("validation passed", "file", file)
			totalValid++
		}
	}

	// Summary
	Logger.Info("validation complete", "valid", totalValid, "invalid", totalInvalid, "total", len(files))

	if totalInvalid > 0 {
		return fmt.Errorf("validation failed for %d file(s)", totalInvalid)
	}

	fmt.Printf("✅ All %d file(s) passed validation\n", totalValid)
	return nil
}

func validateFile(filePath string) error {
	yamlData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	config, err := dsl.ParseYAML(yamlData)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Additional summary for verbose output
	Logger.Debug("file details",
		"name", config.Name,
		"tests", len(config.Tests),
		"description", config.Description,
	)

	return nil
}
