// Package runlog persists a compact record of every mooncake run to
// ~/.mooncake/runs.jsonl and provides a reader for the most recent entry.
package runlog

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ErrNoHistory is returned by Last when the log file is absent or empty.
var ErrNoHistory = errors.New("no run history")

// Entry is a single run record written to the log.
type Entry struct {
	TS         time.Time `json:"ts"`
	Config     string    `json:"config"`
	Changed    int       `json:"changed"`
	Ok         int       `json:"ok"`
	Skipped    int       `json:"skipped"`
	Failed     int       `json:"failed"`
	DurationMs int64     `json:"duration_ms"`
}

// logPath returns the path to the run log file.
func logPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".mooncake", "runs.jsonl"), nil
}

// Append writes e as a single JSON line to ~/.mooncake/runs.jsonl.
// The directory is created if it does not exist. Write failures are
// returned but callers should treat them as best-effort.
func Append(e Entry) error {
	path, err := logPath()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("cannot create run log directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 -- path derived from os.UserHomeDir
	if err != nil {
		return fmt.Errorf("cannot open run log: %w", err)
	}
	encErr := json.NewEncoder(f).Encode(e)
	if closeErr := f.Close(); closeErr != nil && encErr == nil {
		return closeErr
	}
	return encErr
}

// Last returns the most recent entry in the log.
// Returns ErrNoHistory when the file does not exist or contains no valid lines.
func Last() (Entry, error) {
	path, err := logPath()
	if err != nil {
		return Entry{}, err
	}
	f, err := os.Open(path) // #nosec G304 -- path derived from os.UserHomeDir
	if errors.Is(err, os.ErrNotExist) {
		return Entry{}, ErrNoHistory
	}
	if err != nil {
		return Entry{}, fmt.Errorf("cannot open run log: %w", err)
	}
	defer func() { _ = f.Close() }()

	var last Entry
	found := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Entry
		if json.Unmarshal(line, &e) == nil {
			last = e
			found = true
		}
	}
	if !found {
		return Entry{}, ErrNoHistory
	}
	return last, nil
}
