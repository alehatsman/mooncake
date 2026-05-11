//go:build !windows

package security

import (
	"fmt"
	"os"
	"syscall"
)

func checkFileOwnership(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot verify file ownership")
	}
	currentUID := os.Getuid()
	if int(stat.Uid) != currentUID {
		return fmt.Errorf("password file must be owned by current user (uid %d), found uid %d", currentUID, stat.Uid)
	}
	return nil
}
