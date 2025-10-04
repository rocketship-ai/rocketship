package main

import (
	"fmt"
	"os"

	"github.com/rocketship-ai/rocketship/internal/cli"
)

func main() {
	if err := cli.EnsureNonRoot(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	cmd := cli.NewRootCmd()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
