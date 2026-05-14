//go:build !linux && !darwin

package doctor

import "errors"

// diskFree returns ENOTSUPP on platforms where we haven't wired up a probe
// (e.g. Windows). checkDiskSpace surfaces this as info, not error.
func diskFree(_ string) (uint64, error) {
	return 0, errors.New("disk-space probe not implemented on this platform")
}
