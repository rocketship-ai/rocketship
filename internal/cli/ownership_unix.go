//go:build !windows

package cli

import (
	"os"
	"syscall"
)

func pathOwnedByCurrentUser(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return true, nil
	}
	return int(stat.Uid) == os.Getuid(), nil
}
