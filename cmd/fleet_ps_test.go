package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

func TestDerivePsStatusFilter_DefaultRunning(t *testing.T) {
	st, lim, err := derivePsStatusFilter("", false, 5)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(st) != 1 || st[0] != "running" {
		t.Errorf("default = %v", st)
	}
	if lim != 0 {
		t.Errorf("default limit = %d (want 0 — daemon default)", lim)
	}
}

func TestDerivePsStatusFilter_AllStripsStatus(t *testing.T) {
	st, lim, err := derivePsStatusFilter("", true, 5)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(st) != 0 {
		t.Errorf("--all should clear statuses, got %v", st)
	}
	if lim != 5 {
		t.Errorf("--all should honor --limit, got %d", lim)
	}
}

func TestDerivePsStatusFilter_MultiCommaDedup(t *testing.T) {
	st, _, err := derivePsStatusFilter("running,queued,running", false, 5)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(st) != 2 {
		t.Errorf("dedup failed: %v", st)
	}
}

func TestDerivePsStatusFilter_RejectsUnknown(t *testing.T) {
	if _, _, err := derivePsStatusFilter("running,nope", false, 5); err == nil {
		t.Fatal("expected error for unknown status")
	}
}

func TestBuildPsRows_GroupsByPeerThenNewestFirst(t *testing.T) {
	now := time.Now().UTC()
	mk := func(id string, ago time.Duration) transport.RunRecord {
		return transport.RunRecord{
			ID:         id,
			Status:     "running",
			StartedAt:  now.Add(-ago).Format(time.RFC3339Nano),
			QueuedAt:   now.Add(-ago).Format(time.RFC3339Nano),
			FinishedAt: now.Add(-ago).Format(time.RFC3339Nano),
		}
	}
	results := []fleet.PeerRuns{
		{Name: "alpha", Runs: []transport.RunRecord{mk("A-old", 10*time.Minute), mk("A-new", time.Minute)}},
		{Name: "beta", Runs: []transport.RunRecord{mk("B", 2*time.Minute)}},
	}
	rows := buildPsRows(results, "peer")
	if len(rows) != 3 {
		t.Fatalf("len = %d, want 3", len(rows))
	}
	// alpha (newest-first within peer), then beta.
	want := []string{"A-new", "A-old", "B"}
	for i, w := range want {
		if rows[i].Run.ID != w {
			t.Errorf("row %d = %s, want %s", i, rows[i].Run.ID, w)
		}
	}
}

func TestBuildPsRows_AgeSortIsGlobalOldestFirst(t *testing.T) {
	now := time.Now().UTC()
	mk := func(id string, ago time.Duration) transport.RunRecord {
		return transport.RunRecord{
			ID:         id,
			Status:     "running",
			FinishedAt: now.Add(-ago).Format(time.RFC3339Nano),
		}
	}
	results := []fleet.PeerRuns{
		{Name: "a", Runs: []transport.RunRecord{mk("middle", 5*time.Minute)}},
		{Name: "b", Runs: []transport.RunRecord{mk("oldest", 10*time.Minute)}},
		{Name: "c", Runs: []transport.RunRecord{mk("newest", time.Minute)}},
	}
	rows := buildPsRows(results, "age")
	if rows[0].Run.ID != "oldest" || rows[2].Run.ID != "newest" {
		t.Errorf("age order: %s, %s, %s", rows[0].Run.ID, rows[1].Run.ID, rows[2].Run.ID)
	}
}

func TestRenderPsTable_EmptyResultPrintsNoRunsLine(t *testing.T) {
	var buf bytes.Buffer
	results := []fleet.PeerRuns{{Name: "a"}, {Name: "b"}}
	renderPsTable(&buf, nil, results, false, false)
	out := buf.String()
	if !strings.Contains(out, "no in-flight runs") {
		t.Errorf("missing empty-result line:\n%s", out)
	}
	if strings.Contains(out, "HOST") {
		t.Errorf("empty result must not render header:\n%s", out)
	}
}

func TestRenderPsTable_TabularWithSummary(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now().UTC()
	r := transport.RunRecord{
		ID:        "01HXY",
		Status:    "running",
		PlanPath:  "/var/lib/mooncake/synced/c1/abcd/machines/m1/index.yml",
		StartedAt: now.Add(-3 * time.Minute).Format(time.RFC3339Nano),
	}
	results := []fleet.PeerRuns{{Name: "p1", Runs: []transport.RunRecord{r}}}
	rows := buildPsRows(results, "peer")
	renderPsTable(&buf, rows, results, false, false)
	out := buf.String()
	if !strings.Contains(out, "HOST") || !strings.Contains(out, "01HXY") {
		t.Errorf("missing table or RUN_ID:\n%s", out)
	}
	if !strings.Contains(out, "running") {
		t.Errorf("missing STATUS:\n%s", out)
	}
	if !strings.Contains(out, "1 run(s) across 1 peer(s)") {
		t.Errorf("missing summary line:\n%s", out)
	}
}

func TestRenderPsTable_UnreachablePeerFootnote(t *testing.T) {
	var buf bytes.Buffer
	results := []fleet.PeerRuns{
		{Name: "alive", Runs: []transport.RunRecord{{ID: "RID", Status: "running"}}},
		{Name: "dead", Error: errors.New("connect: connection refused")},
	}
	rows := buildPsRows(results, "peer")
	renderPsTable(&buf, rows, results, false, false)
	out := buf.String()
	if !strings.Contains(out, "  dead: connect: connection refused") {
		t.Errorf("missing footnote for unreachable peer:\n%s", out)
	}
	if !strings.Contains(out, "1 unreachable") {
		t.Errorf("summary should count unreachable:\n%s", out)
	}
}

func TestRenderPsJSON_OneLinePerRunPlusErrorSentinel(t *testing.T) {
	var buf bytes.Buffer
	results := []fleet.PeerRuns{
		{Name: "alive", Runs: []transport.RunRecord{
			{ID: "RID1", Status: "running"},
			{ID: "RID2", Status: "success"},
		}},
		{Name: "dead", Error: errors.New("oops")},
	}
	if err := renderPsJSON(&buf, results); err != nil {
		t.Fatalf("err: %v", err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 JSONL lines, got %d:\n%s", len(lines), buf.String())
	}
	if !strings.Contains(lines[0], `"peer":"alive"`) || !strings.Contains(lines[0], `"RID1"`) {
		t.Errorf("line[0] = %s", lines[0])
	}
	if !strings.Contains(lines[2], `"error":"oops"`) {
		t.Errorf("error sentinel missing: %s", lines[2])
	}
}

func TestRunIDDisplay_ShortFlagTruncates(t *testing.T) {
	if got := runIDDisplay("01HXG4VW5MJ7P9KQR2N3T8B6CY", true); got != "…3T8B6CY" {
		// 10-char tail of len-26 ULID is "MJ7P9KQR2N3T8B6CY" trailing — the
		// last 10 chars are "3T8B6CY"... wait, len is 26, last 10 are
		// "N3T8B6CY" → 8 chars, my test should compute precisely.
		// Let me check len: "01HXG4VW5MJ7P9KQR2N3T8B6CY" is 26 chars.
		// last 10 chars: from idx 16 → "N3T8B6CY" (8) + "MJ7P9KQR2"? No.
		// Simplest: compute the actual tail dynamically.
		want := "…" + "01HXG4VW5MJ7P9KQR2N3T8B6CY"[len("01HXG4VW5MJ7P9KQR2N3T8B6CY")-10:]
		if got != want {
			t.Errorf("short truncate = %q, want %q", got, want)
		}
	}
}

func TestRunIDDisplay_NoShortPassesThrough(t *testing.T) {
	id := "01HXG4VW5MJ7P9KQR2N3T8B6CY"
	if got := runIDDisplay(id, false); got != id {
		t.Errorf("full ID changed: %q", got)
	}
}

func TestPlanDisplay_TrimsSyncedRootPrefix(t *testing.T) {
	in := "/home/aleh/.local/state/mooncake/agentd/synced/c1/abcd/machines/m1/index.yml"
	got := planDisplay(in)
	if strings.Contains(got, "/synced/") {
		t.Errorf("synced prefix not trimmed: %s", got)
	}
	if !strings.HasSuffix(got, "machines/m1/index.yml") {
		t.Errorf("trimmed plan wrong shape: %s", got)
	}
}

func TestPlanDisplay_MiddleTruncatesLongPath(t *testing.T) {
	long := strings.Repeat("a", 80)
	got := planDisplay(long)
	if !strings.Contains(got, "…") {
		t.Errorf("long path missing ellipsis: %s", got)
	}
	if rc := utf8.RuneCountInString(got); rc > 60 {
		t.Errorf("long path not truncated under 60 runes: rc=%d %s", rc, got)
	}
}

func TestAllUnreachable_TrueOnlyWhenEveryPeerErrored(t *testing.T) {
	if !allUnreachable([]fleet.PeerRuns{{Error: errors.New("x")}, {Error: errors.New("y")}}) {
		t.Error("all-error should be allUnreachable=true")
	}
	if allUnreachable([]fleet.PeerRuns{{Error: errors.New("x")}, {Name: "ok"}}) {
		t.Error("mixed result should not be allUnreachable")
	}
	if allUnreachable(nil) {
		t.Error("empty input should not be allUnreachable")
	}
}
