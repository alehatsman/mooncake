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

// ErrNoHistory is returned by Last/Recent/At when the log file is absent
// or contains no valid entries.
var ErrNoHistory = errors.New("no run history")

// ErrIndexOutOfRange is returned by At when the 1-based newest-first index
// falls outside the available entries.
var ErrIndexOutOfRange = errors.New("history index out of range")

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
	entries, err := readAll()
	if err != nil {
		return Entry{}, err
	}
	if len(entries) == 0 {
		return Entry{}, ErrNoHistory
	}
	return entries[len(entries)-1], nil
}

// Recent returns up to n most-recent entries, ordered NEWEST-FIRST.
// Returns ErrNoHistory when the log is missing or empty. n <= 0 yields
// the full history.
func Recent(n int) ([]Entry, error) {
	entries, err := readAll()
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, ErrNoHistory
	}
	reversed := make([]Entry, len(entries))
	for i, e := range entries {
		reversed[len(entries)-1-i] = e
	}
	if n > 0 && n < len(reversed) {
		reversed = reversed[:n]
	}
	return reversed, nil
}

// At returns the entry at the given 1-based newest-first index
// (At(1) == Last()). Returns ErrIndexOutOfRange when index < 1 or
// index > total entries. Returns ErrNoHistory when the log is missing
// or empty regardless of index.
func At(index int) (Entry, error) {
	entries, err := readAll()
	if err != nil {
		return Entry{}, err
	}
	if len(entries) == 0 {
		return Entry{}, ErrNoHistory
	}
	if index < 1 || index > len(entries) {
		return Entry{}, ErrIndexOutOfRange
	}
	return entries[len(entries)-index], nil
}

// readAll parses runs.jsonl into a slice in OLDEST-FIRST order (the
// file's native order — entries are appended). Invalid lines are
// silently skipped so a single garbled write doesn't poison the log.
// A missing file returns (nil, nil), not ErrNoHistory — callers
// decide whether emptiness is meaningful.
func readAll() ([]Entry, error) {
	path, err := logPath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path) // #nosec G304 -- path derived from os.UserHomeDir
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot open run log: %w", err)
	}
	defer func() { _ = f.Close() }()

	var out []Entry
	scanner := bufio.NewScanner(f)
	// Default token size is 64KB which is plenty for a one-line summary;
	// raise the cap just in case a Config path is unusually long.
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Entry
		if json.Unmarshal(line, &e) == nil {
			out = append(out, e)
		}
	}
	return out, scanner.Err()
}
