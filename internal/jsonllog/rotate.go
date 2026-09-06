package jsonllog

import (
	"fmt"
	"os"
)

// MaxBytes is the size at which a log rolls over. Append checks the
// current file before writing; when it is at or above this size the
// file is renamed to "<path>.1" (replacing any previous roll) and a
// fresh log starts.
//
// One rolled generation, not N: it bounds each log at 2×MaxBytes with
// no config surface and no numbering scheme to reason about at 3am.
// Operators who want longer retention copy the roll somewhere else.
//
// Nothing bounded these files before. On a box running mooncake daily,
// runs.jsonl reached 9.0 MB / 24,903 lines and ops.jsonl 5.4 MB, with
// no rotation, no cap, and a doctor check that called it healthy (#176).
var MaxBytes int64 = 16 << 20 // 16 MiB

// rollSuffix is appended to the previous generation's path.
const rollSuffix = ".1"

// rotateIfNeeded rolls path when it has reached MaxBytes. A missing file
// is not an error — that is the common case. Rotation failures are
// returned so Append can surface them, but Append treats them as
// non-fatal: losing rotation is better than losing the append.
func rotateIfNeeded(path string) error {
	if MaxBytes <= 0 {
		return nil
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot stat log: %w", err)
	}
	if info.Size() < MaxBytes {
		return nil
	}
	// os.Rename replaces an existing destination on POSIX but not on
	// Windows, so remove the previous roll first. A missing one is fine.
	if err := os.Remove(path + rollSuffix); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot remove previous roll: %w", err)
	}
	if err := os.Rename(path, path+rollSuffix); err != nil {
		return fmt.Errorf("cannot roll log: %w", err)
	}
	return nil
}
