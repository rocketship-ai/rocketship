package cli

import (
	"fmt"
	"os"
)

// EnsureNonRoot returns an error if the CLI is running as root without explicit override.
func EnsureNonRoot() error {
	if runningAsRoot() && os.Getenv("ROCKETSHIP_ALLOW_ROOT") == "" {
		return fmt.Errorf("refusing to run as root. set ROCKETSHIP_ALLOW_ROOT=1 to override (not recommended)")
	}
	return nil
}
