package main

import (
	"os"

	"github.com/rocketship-ai/rocketship/internal/cli"
)

func main() {
	cmd := cli.NewRootCmd()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
