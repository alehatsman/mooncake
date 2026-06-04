package agentd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LastAppliedRecord is written after every successful run so the drift loop
// has a stable reference frame to inspect against.
type LastAppliedRecord struct {
	Scope     string    `json:"scope"`
	PlanPath  string    `json:"plan_path"`
	BaseDir   string    `json:"base_dir"`
	VarsFiles []string  `json:"vars_files,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	AppliedAt time.Time `json:"applied_at"`
	RunID     string    `json:"run_id"`
}

// PlanScope returns a stable, collision-resistant identifier for the
// combination of (baseDir, planPath, varsFiles, tags) that a run used.
// Used as the filename discriminator so two distinct plans on the same
// peer don't overwrite each other's records.
func PlanScope(baseDir, planPath string, varsFiles, tags []string) string {
	sv := make([]string, len(varsFiles))
	copy(sv, varsFiles)
	sort.Strings(sv)
	st := make([]string, len(tags))
	copy(st, tags)
	sort.Strings(st)

	h := sha256.New()
	for _, part := range []string{baseDir, "\x00", planPath, "\x00"} {
		_, _ = h.Write([]byte(part))
	}
	for _, v := range sv {
		_, _ = h.Write([]byte(v + "\x00"))
	}
	_, _ = h.Write([]byte("\x01"))
	for _, t := range st {
		_, _ = h.Write([]byte(t + "\x00"))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func driftDir(stateDir string) string {
	return filepath.Join(stateDir, "drift")
}

func lastAppliedPath(stateDir, scope string) string {
	return filepath.Join(driftDir(stateDir), "last-applied-"+scope+".json")
}

// WriteLastApplied atomically records the plan parameters used by a
// successful run. Only call for StatusSuccess — failed/interrupted runs
// must not update the reference frame.
func WriteLastApplied(stateDir string, rec LastAppliedRecord) error {
	dir := driftDir(stateDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create drift dir: %w", err)
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	path := lastAppliedPath(stateDir, rec.Scope)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write tmp last-applied: %w", err)
	}
	return os.Rename(tmp, path)
}

// ReadLastApplied loads the last-applied record for the given scope hash.
// Returns os.ErrNotExist when no apply has been recorded for that scope.
func ReadLastApplied(stateDir, scope string) (*LastAppliedRecord, error) {
	data, err := os.ReadFile(lastAppliedPath(stateDir, scope))
	if err != nil {
		return nil, err
	}
	var rec LastAppliedRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("decode last-applied: %w", err)
	}
	return &rec, nil
}

// ListLastApplied returns all last-applied records found in stateDir.
// Returns an empty slice (not an error) when the drift dir does not exist yet.
func ListLastApplied(stateDir string) ([]LastAppliedRecord, error) {
	dir := driftDir(stateDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read drift dir: %w", err)
	}
	var out []LastAppliedRecord
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasPrefix(e.Name(), "last-applied-") || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var rec LastAppliedRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}
