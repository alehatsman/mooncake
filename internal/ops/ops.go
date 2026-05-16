// Package ops persists a compact record of every kernel operation
// (apply / plan / rollback / replay / inspect) to ~/.mooncake/ops.jsonl.
//
// ops.jsonl is the operation log; runs.jsonl is the run log. The two
// are decoupled: ops can exist with no runs (`mooncake plan`), one op
// can span N runs (a rollback that reverses N steps), and ops can
// chain (replay-of-a-replay). See spec-68 §"The op_id schema decision"
// for the rationale.
//
// ops.jsonl is append-only JSONL with one Entry per line. The Runs
// list is left empty on creation; the corresponding run_ids land in
// runs.jsonl and are derived back at read time by the resolver.
package ops

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

// ErrNotFound is returned by Read when no op has the requested id.
var ErrNotFound = errors.New("op not found")

// Entry is a single operation record written to ops.jsonl.
//
// Schema mirrors spec-68 §"The op_id schema decision" — additive only;
// readers ignore unknown fields so future additions don't break old
// consumers.
type Entry struct {
	TS       time.Time `json:"ts"`
	OpID     string    `json:"op_id"`
	Command  string    `json:"command"`
	Args     []string  `json:"args,omitempty"`
	Actor    string    `json:"actor,omitempty"`
	Parent   string    `json:"parent,omitempty"`
	Config   string    `json:"config,omitempty"`
	PlanOnly bool      `json:"plan_only,omitempty"`
}

// NewOpID mints a ULID-shaped op identifier of the form
// "op/<26-char base32>". The prefix lets a single noun-detector route
// "op/..." inputs to the op resolver without parsing the body.
//
// ULID gives lexicographic time-ordering plus uniqueness without
// coordination — same generator the existing fleet/agentd packages
// already use (see internal/agentd/store.go).
func NewOpID() string {
	id := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)
	return "op/" + id.String()
}

// NewRunID mints a ULID-shaped run identifier of the form
// "r/<26-char base32>". Lives here rather than in runlog so a single
// op-creation site can mint both ids and link them up-front.
func NewRunID() string {
	id := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)
	return "r/" + id.String()
}

// LogPath returns the ops.jsonl path under ~/.mooncake.
func LogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".mooncake", "ops.jsonl"), nil
}

// Append writes e as a single JSON line to ~/.mooncake/ops.jsonl.
// Best-effort: write failures are returned to the caller; mooncake
// itself treats them as non-fatal (logging is not the critical path
// for `apply`).
func Append(e Entry) error {
	path, err := LogPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("cannot create ops log directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 -- derived from os.UserHomeDir
	if err != nil {
		return fmt.Errorf("cannot open ops log: %w", err)
	}
	encErr := json.NewEncoder(f).Encode(e)
	if closeErr := f.Close(); closeErr != nil && encErr == nil {
		return closeErr
	}
	return encErr
}

// Read returns the op with the given id. ErrNotFound on absence.
//
// Linear scan over ops.jsonl. A year of an operator's personal use
// is ~10k entries; if a fleet member produces 100k+ this becomes the
// place to add an on-disk index — JSONL stays authoritative.
func Read(opID string) (Entry, error) {
	entries, err := ReadAll()
	if err != nil {
		return Entry{}, err
	}
	for _, e := range entries {
		if e.OpID == opID {
			return e, nil
		}
	}
	return Entry{}, ErrNotFound
}

// ReadAll returns every parseable entry from ops.jsonl in append
// order (oldest first). Invalid lines are silently skipped so a
// single garbled write doesn't poison the log.
//
// A missing file returns (nil, nil) — callers decide whether
// emptiness is meaningful.
func ReadAll() ([]Entry, error) {
	path, err := LogPath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path) // #nosec G304 -- derived from os.UserHomeDir
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot open ops log: %w", err)
	}
	defer func() { _ = f.Close() }()

	var out []Entry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 8*1024), 1<<20)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Entry
		if json.Unmarshal(line, &e) == nil && strings.HasPrefix(e.OpID, "op/") {
			out = append(out, e)
		}
	}
	return out, scanner.Err()
}
