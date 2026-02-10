//go:build unix

package assert

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"syscall"
)

// checkFileOwner checks if a file matches the expected owner.
// Returns (expected, actual, error).
func checkFileOwner(info os.FileInfo, expectedOwner string) (string, string, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", "", fmt.Errorf("failed to get file stat info")
	}

	actualUID := stat.Uid

	// Resolve expected UID
	expectedUID, err := resolveUserID(expectedOwner)
	if err != nil {
		return "", "", err
	}

	// Format display strings
	actualStr := formatUserDisplay(actualUID)
	expectedStr := formatUserDisplay(expectedUID)

	if actualUID != expectedUID {
		return expectedStr, actualStr, fmt.Errorf("owner mismatch")
	}

	return expectedStr, actualStr, nil
}

// checkFileGroup checks if a file matches the expected group.
// Returns (expected, actual, error).
func checkFileGroup(info os.FileInfo, expectedGroup string) (string, string, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", "", fmt.Errorf("failed to get file stat info")
	}

	actualGID := stat.Gid

	// Resolve expected GID
	expectedGID, err := resolveGroupID(expectedGroup)
	if err != nil {
		return "", "", err
	}

	// Format display strings
	actualStr := formatGroupDisplay(actualGID)
	expectedStr := formatGroupDisplay(expectedGID)

	if actualGID != expectedGID {
		return expectedStr, actualStr, fmt.Errorf("group mismatch")
	}

	return expectedStr, actualStr, nil
}

// resolveUserID converts a username or UID string to a uint32 UID.
func resolveUserID(userSpec string) (uint32, error) {
	// Try parsing as UID first
	if uid, err := strconv.ParseUint(userSpec, 10, 32); err == nil {
		return uint32(uid), nil
	}

	// Look up as username
	u, err := user.Lookup(userSpec)
	if err != nil {
		return 0, fmt.Errorf("failed to lookup user %q: %w", userSpec, err)
	}

	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid UID for user %q: %w", userSpec, err)
	}

	return uint32(uid), nil
}

// resolveGroupID converts a group name or GID string to a uint32 GID.
func resolveGroupID(groupSpec string) (uint32, error) {
	// Try parsing as GID first
	if gid, err := strconv.ParseUint(groupSpec, 10, 32); err == nil {
		return uint32(gid), nil
	}

	// Look up as group name
	g, err := user.LookupGroup(groupSpec)
	if err != nil {
		return 0, fmt.Errorf("failed to lookup group %q: %w", groupSpec, err)
	}

	gid, err := strconv.ParseUint(g.Gid, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid GID for group %q: %w", groupSpec, err)
	}

	return uint32(gid), nil
}

// formatUserDisplay formats a UID for display (username + UID).
func formatUserDisplay(uid uint32) string {
	u, err := user.LookupId(fmt.Sprintf("%d", uid))
	if err == nil {
		return fmt.Sprintf("%s (%d)", u.Username, uid)
	}
	return fmt.Sprintf("%d", uid)
}

// formatGroupDisplay formats a GID for display (group name + GID).
func formatGroupDisplay(gid uint32) string {
	g, err := user.LookupGroupId(fmt.Sprintf("%d", gid))
	if err == nil {
		return fmt.Sprintf("%s (%d)", g.Name, gid)
	}
	return fmt.Sprintf("%d", gid)
}
