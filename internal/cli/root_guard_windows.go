//go:build windows

package cli

func runningAsRoot() bool {
	return false
}
