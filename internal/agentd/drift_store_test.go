package agentd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPlanScope_Stability(t *testing.T) {
	s1 := PlanScope("/home/user", "/home/user/plans/dev.yml", []string{"vars.yml"}, []string{"prod"})
	s2 := PlanScope("/home/user", "/home/user/plans/dev.yml", []string{"vars.yml"}, []string{"prod"})
	if s1 != s2 {
		t.Errorf("scope not stable: %q != %q", s1, s2)
	}
}

func TestPlanScope_SortingInsensitive(t *testing.T) {
	s1 := PlanScope("/base", "/plan.yml", []string{"b.yml", "a.yml"}, []string{"z", "a"})
	s2 := PlanScope("/base", "/plan.yml", []string{"a.yml", "b.yml"}, []string{"a", "z"})
	if s1 != s2 {
		t.Errorf("scope not sort-insensitive: %q != %q", s1, s2)
	}
}

func TestPlanScope_Discriminates(t *testing.T) {
	s1 := PlanScope("/base", "/plan.yml", nil, nil)
	s2 := PlanScope("/base", "/other.yml", nil, nil)
	if s1 == s2 {
		t.Error("different plans should produce different scopes")
	}

	s3 := PlanScope("/base", "/plan.yml", nil, []string{"prod"})
	if s1 == s3 {
		t.Error("different tags should produce different scopes")
	}
}

func TestWriteReadLastApplied_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	scope := PlanScope("/base", "/plan.yml", nil, nil)
	rec := LastAppliedRecord{
		Scope:     scope,
		PlanPath:  "/base/plan.yml",
		BaseDir:   "/base",
		VarsFiles: []string{"vars.yml"},
		Tags:      []string{"prod"},
		AppliedAt: time.Now().UTC().Truncate(time.Second),
		RunID:     "01ABCDEFGHJKMNPQRSTVWXYZ00",
	}

	if err := WriteLastApplied(dir, rec); err != nil {
		t.Fatalf("WriteLastApplied: %v", err)
	}

	got, err := ReadLastApplied(dir, scope)
	if err != nil {
		t.Fatalf("ReadLastApplied: %v", err)
	}

	if got.Scope != rec.Scope {
		t.Errorf("Scope: want %q, got %q", rec.Scope, got.Scope)
	}
	if got.PlanPath != rec.PlanPath {
		t.Errorf("PlanPath: want %q, got %q", rec.PlanPath, got.PlanPath)
	}
	if got.RunID != rec.RunID {
		t.Errorf("RunID: want %q, got %q", rec.RunID, got.RunID)
	}
	if !got.AppliedAt.Equal(rec.AppliedAt) {
		t.Errorf("AppliedAt: want %v, got %v", rec.AppliedAt, got.AppliedAt)
	}
}

func TestReadLastApplied_NotExist(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadLastApplied(dir, "nonexistent")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected ErrNotExist, got %v", err)
	}
}

func TestWriteLastApplied_Atomic(t *testing.T) {
	dir := t.TempDir()
	scope := PlanScope("/b", "/p.yml", nil, nil)

	rec1 := LastAppliedRecord{Scope: scope, PlanPath: "/p.yml", RunID: "run1", AppliedAt: time.Now().UTC()}
	rec2 := LastAppliedRecord{Scope: scope, PlanPath: "/p.yml", RunID: "run2", AppliedAt: time.Now().UTC()}

	if err := WriteLastApplied(dir, rec1); err != nil {
		t.Fatalf("write rec1: %v", err)
	}
	if err := WriteLastApplied(dir, rec2); err != nil {
		t.Fatalf("write rec2: %v", err)
	}

	got, err := ReadLastApplied(dir, scope)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.RunID != "run2" {
		t.Errorf("expected latest run, got RunID=%q", got.RunID)
	}

	// No .tmp files should linger.
	entries, _ := os.ReadDir(filepath.Join(dir, "drift"))
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("lingering .tmp file: %s", e.Name())
		}
	}
}

func TestListLastApplied_Empty(t *testing.T) {
	dir := t.TempDir()
	recs, err := ListLastApplied(dir)
	if err != nil {
		t.Fatalf("ListLastApplied on empty dir: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("expected 0 records, got %d", len(recs))
	}
}

func TestListLastApplied_MultipleScopes(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()

	for i, planPath := range []string{"/plan1.yml", "/plan2.yml", "/plan3.yml"} {
		scope := PlanScope("/base", planPath, nil, nil)
		rec := LastAppliedRecord{
			Scope:     scope,
			PlanPath:  planPath,
			AppliedAt: now,
			RunID:     "run" + string(rune('0'+i)),
		}
		if err := WriteLastApplied(dir, rec); err != nil {
			t.Fatalf("write %s: %v", planPath, err)
		}
	}

	recs, err := ListLastApplied(dir)
	if err != nil {
		t.Fatalf("ListLastApplied: %v", err)
	}
	if len(recs) != 3 {
		t.Errorf("expected 3 records, got %d", len(recs))
	}
}

// TestWorker_WritesLastAppliedOnSuccess verifies that a successful run
// leaves a last-applied record on disk in the drift/ subdirectory.
func TestWorker_WritesLastAppliedOnSuccess(t *testing.T) {
	cfg, client, stop := startTestServer(t)
	defer stop()

	planPath := writeTrivialPlan(t)
	sub := submitRun(t, client, planPath)
	run := waitForTerminal(t, client, sub.RunID, 10*time.Second)
	if run.Status != StatusSuccess {
		t.Fatalf("expected success, got %s: %s", run.Status, run.Error)
	}

	// The handler defaults BaseDir to filepath.Dir(planPath) when not supplied.
	expectedBaseDir := filepath.Dir(planPath)
	scope := PlanScope(expectedBaseDir, planPath, nil, nil)
	rec, err := ReadLastApplied(cfg.StateDir, scope)
	if err != nil {
		t.Fatalf("ReadLastApplied after success: %v", err)
	}
	if rec.PlanPath != planPath {
		t.Errorf("PlanPath: want %q, got %q", planPath, rec.PlanPath)
	}
	if rec.RunID != sub.RunID {
		t.Errorf("RunID: want %q, got %q", sub.RunID, rec.RunID)
	}
}
