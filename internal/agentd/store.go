package agentd

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

type RunStatus string

const (
	StatusQueued      RunStatus = "queued"
	StatusRunning     RunStatus = "running"
	StatusSuccess     RunStatus = "success"
	StatusFailed      RunStatus = "failed"
	StatusInterrupted RunStatus = "interrupted"
)

// Run is the persisted record for a single submitted plan.
type Run struct {
	ID        string   `json:"id"`
	PlanPath  string   `json:"plan_path"`
	VarsFiles []string `json:"vars_files,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	// Names is the spec-50 step-name filter; AND'd with Tags by the
	// planner. Empty = no filter active.
	Names      []string   `json:"names,omitempty"`
	Goal       string     `json:"goal,omitempty"`
	BaseDir    string     `json:"base_dir"`
	Status     RunStatus  `json:"status"`
	QueuedAt   time.Time  `json:"queued_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Error      string     `json:"error,omitempty"`
	DaemonPID  int        `json:"daemon_pid"`
}

// IsTerminal reports whether the run has reached a final state.
func (r *Run) IsTerminal() bool {
	switch r.Status {
	case StatusSuccess, StatusFailed, StatusInterrupted:
		return true
	}
	return false
}

// SubmitReq is the input to Store.Create.
type SubmitReq struct {
	PlanPath  string
	VarsFiles []string
	Tags      []string
	Names     []string
	Goal      string
	BaseDir   string
}

// ListFilter scopes Store.List output.
type ListFilter struct {
	Status RunStatus // empty = any
	Limit  int       // 0 = default (50)
	Before string    // run_id; results strictly older than this id
}

// Store is the file-backed run record store. One subdirectory per run:
//
//	<root>/<run_id>/record.json
//	<root>/<run_id>/events.jsonl
//
// Run IDs are ULIDs, so lexicographic order matches submission order.
// Reads (Get/List) and writes (Create/Update/AppendEvent) are safe to
// interleave: writes use temp-file + atomic rename, so readers see either the
// previous or next consistent record state.
type Store struct {
	root string
}

func NewStore(stateDir string) (*Store, error) {
	root := filepath.Join(stateDir, "runs")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create runs dir: %w", err)
	}
	return &Store{root: root}, nil
}

// Root returns the absolute path under which run directories live.
func (s *Store) Root() string { return s.root }

// RunDir returns the directory holding a run's files. Panics if id is not a
// well-formed ULID — call validateID first if id came from a user.
func (s *Store) RunDir(id string) string { return filepath.Join(s.root, id) }

// ResultPath returns the path to a run's result.json — the daemon's
// apply.KernelResult serialised after the run reaches a terminal state.
// Consumed by controller-side fleet.Apply via GET /v1/runs/{id}/result
// (R2.1c) and by any future SDK / MCP caller that wants the typed
// kernel-surface tail. Always returns the same path even before the
// file exists; missing-file is the caller's "result not ready" signal.
func (s *Store) ResultPath(id string) string {
	return filepath.Join(s.root, id, "result.json")
}

// EventsPath returns the path to a run's events.jsonl.
func (s *Store) EventsPath(id string) string {
	return filepath.Join(s.root, id, "events.jsonl")
}

func (s *Store) recordPath(id string) string {
	return filepath.Join(s.root, id, "record.json")
}

// validateID rejects anything that isn't a ULID. ULIDs are 26-char Crockford
// base32. Reject other inputs to keep arbitrary path components out of the
// store directory.
func validateID(id string) error {
	if len(id) != 26 {
		return fmt.Errorf("invalid run id length: %d", len(id))
	}
	for _, c := range id {
		// Crockford base32: 0-9, A-H, J-K, M-N, P-T, V-Z (no I, L, O, U).
		valid := (c >= '0' && c <= '9') ||
			(c >= 'A' && c <= 'H') ||
			c == 'J' || c == 'K' ||
			c == 'M' || c == 'N' ||
			(c >= 'P' && c <= 'T') ||
			(c >= 'V' && c <= 'Z')
		if !valid {
			return fmt.Errorf("invalid run id char: %q", c)
		}
	}
	return nil
}

func newRunID() string {
	return ulid.MustNew(ulid.Now(), rand.Reader).String()
}

// Create allocates a new run ID, writes the initial `queued` record, and
// creates an empty events.jsonl. Returns the persisted Run.
func (s *Store) Create(req SubmitReq) (*Run, error) {
	id := newRunID()
	dir := s.RunDir(id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create run dir: %w", err)
	}
	run := &Run{
		ID:        id,
		PlanPath:  req.PlanPath,
		VarsFiles: req.VarsFiles,
		Tags:      req.Tags,
		Names:     req.Names,
		Goal:      req.Goal,
		BaseDir:   req.BaseDir,
		Status:    StatusQueued,
		QueuedAt:  time.Now().UTC(),
		DaemonPID: os.Getpid(),
	}
	if err := s.writeRecord(run); err != nil {
		return nil, err
	}
	// Touch events.jsonl so SSE handlers can open it before any event lands.
	f, err := os.OpenFile(s.EventsPath(id), os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("touch events.jsonl: %w", err)
	}
	_ = f.Close()
	return run, nil
}

// Get loads a run record. Returns os.ErrNotExist when the id is unknown.
func (s *Store) Get(id string) (*Run, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	return s.readRecord(id)
}

// Update reads the current run, applies fn, and writes the result atomically.
// Caller must NOT mutate the Run inside fn except through return-by-pointer
// (we pass the same pointer back).
func (s *Store) Update(id string, fn func(*Run) error) (*Run, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	run, err := s.readRecord(id)
	if err != nil {
		return nil, err
	}
	if err := fn(run); err != nil {
		return nil, err
	}
	if err := s.writeRecord(run); err != nil {
		return nil, err
	}
	return run, nil
}

// List returns runs newest-first (by ULID, which sorts by submission time).
func (s *Store) List(filter ListFilter) ([]*Run, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}

	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if validateID(e.Name()) != nil {
			continue
		}
		ids = append(ids, e.Name())
	}
	// Newest first.
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))

	out := make([]*Run, 0, limit)
	for _, id := range ids {
		if filter.Before != "" && id >= filter.Before {
			continue
		}
		r, err := s.readRecord(id)
		if err != nil {
			// Skip records that disappear or are mid-write; not fatal.
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, err
		}
		if filter.Status != "" && r.Status != filter.Status {
			continue
		}
		out = append(out, r)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// Reconcile scans the store at startup and marks any record left in
// queued/running by a previous daemon as `interrupted`. Returns the list of
// ids that were transitioned.
func (s *Store) Reconcile(currentPID int) ([]string, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, fmt.Errorf("read runs dir: %w", err)
	}
	var changed []string
	now := time.Now().UTC()
	for _, e := range entries {
		if !e.IsDir() || validateID(e.Name()) != nil {
			continue
		}
		r, err := s.readRecord(e.Name())
		if err != nil {
			continue
		}
		if r.Status != StatusQueued && r.Status != StatusRunning {
			continue
		}
		if r.DaemonPID == currentPID {
			continue
		}
		r.Status = StatusInterrupted
		r.Error = "daemon restarted"
		r.FinishedAt = &now
		if err := s.writeRecord(r); err != nil {
			return changed, err
		}
		changed = append(changed, r.ID)
	}
	return changed, nil
}

// AppendEvent appends one JSONL line to the run's events log. Caller is
// responsible for serializing concurrent writes; the JSONL subscriber owns
// this for a given run.
func (s *Store) AppendEvent(id string, line []byte) error {
	if err := validateID(id); err != nil {
		return err
	}
	f, err := os.OpenFile(s.EventsPath(id), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if !strings.HasSuffix(string(line), "\n") {
		line = append(line, '\n')
	}
	_, err = f.Write(line)
	return err
}

func (s *Store) readRecord(id string) (*Run, error) {
	data, err := os.ReadFile(s.recordPath(id))
	if err != nil {
		return nil, err
	}
	var run Run
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, fmt.Errorf("decode record %s: %w", id, err)
	}
	return &run, nil
}

// writeRecord atomically replaces the run's record.json via temp file + rename.
func (s *Store) writeRecord(run *Run) error {
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	final := s.recordPath(run.ID)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write tmp record: %w", err)
	}
	return os.Rename(tmp, final)
}
