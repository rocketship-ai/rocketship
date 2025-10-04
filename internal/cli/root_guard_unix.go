//go:build !windows

package cli

import "os"

func runningAsRoot() bool {
	return os.Geteuid() == 0
}
