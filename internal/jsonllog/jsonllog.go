// Package jsonllog provides a single shared append-only writer for the
// JSONL log files mooncake maintains under ~/.mooncake (currently
// runs.jsonl and ops.jsonl).
//
// Both call sites used to ship near-identical 18-line Append bodies
// — MkdirAll(dir, 0o700) + OpenFile(O_APPEND|O_CREATE|O_WRONLY, 0o600)
// + json.Encode + Close — flagged as a clone by `dupl`. This package
// holds the canonical version; the per-log packages keep the path
// derivation and the per-entry types they own.
package jsonllog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Append serialises v as a single JSON line and writes it to path,
// creating the parent directory with 0o700 and the file with 0o600
// if either does not exist.
//
// Best-effort: callers (subscribers under apply.Runner.Run) typically
// discard the returned error so a transient log-write failure doesn't
// take down a successful apply. Returning the error keeps the writer
// reusable by callers that *do* want to surface it (tests, future
// audit-critical paths).
func Append(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("cannot create log directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 -- caller derives path from os.UserHomeDir
	if err != nil {
		return fmt.Errorf("cannot open log: %w", err)
	}
	encErr := json.NewEncoder(f).Encode(v)
	if closeErr := f.Close(); closeErr != nil && encErr == nil {
		return closeErr
	}
	return encErr
}
