package main

import (
	"log"

	"github.com/rocketship-ai/rocketship/internal/cli"
	"github.com/spf13/cobra/doc"
)

func main() {
	// Get the root command
	rootCmd := cli.NewRootCmd()

	// Generate markdown documentation
	if err := doc.GenMarkdownTree(rootCmd, "./docs/src/reference"); err != nil {
		log.Fatal(err)
	}
}
