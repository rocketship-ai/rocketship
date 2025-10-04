//go:build windows

package cli

import "os"

func pathOwnedByCurrentUser(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		return false, err
	}
	return true, nil
}
