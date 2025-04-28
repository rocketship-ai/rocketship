package main

import (
	"fmt"
	"os"

	"github.com/rocketship-ai/rocketship/cmd/rocketship/cmd"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
