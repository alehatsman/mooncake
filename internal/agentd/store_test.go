package agentd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func TestStoreCreateGetRoundTrip(t *testing.T) {
	s := newStore(t)
	run, err := s.Create(SubmitReq{
		PlanPath: "/tmp/x.yml",
		Goal:     "goal",
		BaseDir:  "/tmp",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if run.Status != StatusQueued {
		t.Errorf("want queued, got %q", run.Status)
	}
	if len(run.ID) != 26 {
		t.Errorf("want ULID id, got %q", run.ID)
	}

	got, err := s.Get(run.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.PlanPath != "/tmp/x.yml" || got.Goal != "goal" || got.BaseDir != "/tmp" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestStoreGetInvalidID(t *testing.T) {
	s := newStore(t)
	_, err := s.Get("../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path-traversal id, got nil")
	}
}

func TestStoreGetUnknownID(t *testing.T) {
	s := newStore(t)
	// Well-formed ULID that doesn't exist.
	_, err := s.Get("01H00000000000000000000000")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("want ErrNotExist, got %v", err)
	}
}

func TestStoreListNewestFirst(t *testing.T) {
	s := newStore(t)
	var ids []string
	for i := 0; i < 5; i++ {
		r, err := s.Create(SubmitReq{PlanPath: "/p.yml", BaseDir: "/"})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		ids = append(ids, r.ID)
		// Small spacing so ULIDs differ in time portion. ULIDs include
		// monotonic randomness, so even back-to-back creates sort correctly,
		// but a sleep makes intent explicit.
		time.Sleep(2 * time.Millisecond)
	}
	got, err := s.List(ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("want 5 runs, got %d", len(got))
	}
	// Newest (last created) should be first.
	if got[0].ID != ids[4] {
		t.Errorf("want newest=%s first, got %s", ids[4], got[0].ID)
	}
	if got[4].ID != ids[0] {
		t.Errorf("want oldest=%s last, got %s", ids[0], got[4].ID)
	}
}

func TestStoreListBeforePaging(t *testing.T) {
	s := newStore(t)
	var ids []string
	for i := 0; i < 4; i++ {
		r, _ := s.Create(SubmitReq{PlanPath: "/p.yml", BaseDir: "/"})
		ids = append(ids, r.ID)
		time.Sleep(2 * time.Millisecond)
	}
	// Page strictly older than the newest.
	got, err := s.List(ListFilter{Before: ids[3]})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("want 3 runs before newest, got %d", len(got))
	}
	if got[0].ID != ids[2] {
		t.Errorf("page boundary wrong: %s", got[0].ID)
	}
}

func TestStoreListStatusFilter(t *testing.T) {
	s := newStore(t)
	r1, _ := s.Create(SubmitReq{PlanPath: "/p.yml", BaseDir: "/"})
	r2, _ := s.Create(SubmitReq{PlanPath: "/p.yml", BaseDir: "/"})
	_, _ = s.Update(r1.ID, func(r *Run) error { r.Status = StatusSuccess; return nil })
	_, _ = s.Update(r2.ID, func(r *Run) error { r.Status = StatusFailed; return nil })

	got, err := s.List(ListFilter{Status: StatusFailed})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].ID != r2.ID {
		t.Errorf("status filter returned %+v", got)
	}
}

func TestStoreUpdateAtomic(t *testing.T) {
	s := newStore(t)
	r, _ := s.Create(SubmitReq{PlanPath: "/p.yml", BaseDir: "/"})

	now := time.Now().UTC()
	got, err := s.Update(r.ID, func(rr *Run) error {
		rr.Status = StatusRunning
		rr.StartedAt = &now
		return nil
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Status != StatusRunning {
		t.Errorf("status not updated")
	}
	// No leftover .tmp.
	matches, _ := filepath.Glob(filepath.Join(s.Root(), r.ID, "*.tmp"))
	if len(matches) > 0 {
		t.Errorf("Update left temp files: %v", matches)
	}
}

func TestStoreReconcileMarksRunningInterrupted(t *testing.T) {
	s := newStore(t)
	r, _ := s.Create(SubmitReq{PlanPath: "/p.yml", BaseDir: "/"})

	// Simulate a previous-daemon-owned running record.
	_, _ = s.Update(r.ID, func(rr *Run) error {
		rr.Status = StatusRunning
		rr.DaemonPID = 99999 // some other pid
		return nil
	})

	changed, err := s.Reconcile(os.Getpid())
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(changed) != 1 || changed[0] != r.ID {
		t.Errorf("want changed=[%s], got %v", r.ID, changed)
	}
	got, _ := s.Get(r.ID)
	if got.Status != StatusInterrupted {
		t.Errorf("want interrupted, got %s", got.Status)
	}
	if !strings.Contains(got.Error, "daemon restarted") {
		t.Errorf("want error mentioning 'daemon restarted', got %q", got.Error)
	}
	if got.FinishedAt == nil {
		t.Errorf("want finished_at set on interrupted run")
	}
}

func TestStoreReconcileSkipsTerminal(t *testing.T) {
	s := newStore(t)
	r, _ := s.Create(SubmitReq{PlanPath: "/p.yml", BaseDir: "/"})
	_, _ = s.Update(r.ID, func(rr *Run) error {
		rr.Status = StatusSuccess
		return nil
	})

	changed, err := s.Reconcile(os.Getpid())
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(changed) != 0 {
		t.Errorf("terminal runs should not be reconciled, got %v", changed)
	}
}

func TestStoreAppendEvent(t *testing.T) {
	s := newStore(t)
	r, _ := s.Create(SubmitReq{PlanPath: "/p.yml", BaseDir: "/"})

	if err := s.AppendEvent(r.ID, []byte(`{"seq":1,"type":"test"}`)); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	if err := s.AppendEvent(r.ID, []byte(`{"seq":2,"type":"test"}`)); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	data, err := os.ReadFile(s.EventsPath(r.ID))
	if err != nil {
		t.Fatalf("read events.jsonl: %v", err)
	}
	lines := strings.Count(string(data), "\n")
	if lines != 2 {
		t.Errorf("want 2 lines, got %d:\n%s", lines, data)
	}
}

func TestValidateID(t *testing.T) {
	good := newRunID()
	if err := validateID(good); err != nil {
		t.Errorf("newRunID produced invalid id: %s err=%v", good, err)
	}
	cases := []string{
		"",
		"too-short",
		"../../etc/passwd",
		"01H0000000000000000000000I", // contains 'I' (forbidden in Crockford)
		"01H0000000000000000000000l", // lowercase
	}
	for _, bad := range cases {
		if err := validateID(bad); err == nil {
			t.Errorf("validateID(%q) should fail", bad)
		}
	}
}
