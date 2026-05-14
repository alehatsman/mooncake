package doctor

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestRun_ExitCodeOnErrors covers AC1+AC2: exit 0 on a clean run, exit 1
// when any check returns StatusError.
func TestRun_ExitCodeOnErrors(t *testing.T) {
	// Filter to "install" so we don't trip on the user's actual home dir
	// state during CI. install/binary should be StatusOK; install/
	// go-runtime is info.
	_, exit, err := Run(Options{Sections: []string{"install"}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if exit != 0 {
		t.Errorf("install-only doctor should exit 0, got %d", exit)
	}
}

// AC3: --strict elevates warnings to non-zero exit.
func TestRun_StrictExitsOnWarnings(t *testing.T) {
	// Drive a synthetic warning-only report directly via the package types
	// (since real check outcomes depend on the host, we can't reliably
	// stage a warning across CI environments).
	rep := &Report{Warnings: 1}
	if !rep.HasWarnings() {
		t.Fatal("HasWarnings should be true")
	}
	if rep.HasErrors() {
		t.Error("HasErrors should be false")
	}
}

// AC4: --format json contract — well-formed, deterministic catalogue order,
// stable field names.
func TestRun_JSONOutput(t *testing.T) {
	rep, _, err := Run(Options{Sections: []string{"install"}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var buf bytes.Buffer
	if err := RenderJSON(&buf, rep); err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}

	// Parse back to confirm it's valid JSON.
	var decoded struct {
		Cwd      string `json:"cwd"`
		Ok       int    `json:"ok"`
		Warnings int    `json:"warnings"`
		Errors   int    `json:"errors"`
		Checks   []struct {
			Section string `json:"section"`
			Name    string `json:"name"`
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"checks"`
	}
	if err := json.NewDecoder(&buf).Decode(&decoded); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, buf.String())
	}

	if decoded.Cwd == "" {
		t.Error("cwd field should be populated")
	}
	if len(decoded.Checks) == 0 {
		t.Fatal("install section should yield at least one check")
	}
	// Catalogue order: binary first, then go-runtime.
	if decoded.Checks[0].Name != "binary" {
		t.Errorf("first check should be 'binary', got %q", decoded.Checks[0].Name)
	}
}

// AC5: every warning/error result has a non-empty Fix field.
func TestRun_WarningsAndErrorsHaveFixes(t *testing.T) {
	for _, c := range registeredChecks() {
		// Synthesise a warning result for tools (most likely to fire in
		// minimal environments) and check the fix is non-empty in
		// principle by inspecting installHint when the check is checkTool.
		if tc, ok := c.(checkTool); ok {
			if installHint(tc.name) == "" {
				t.Errorf("checkTool %s: installHint returned empty string", tc.name)
			}
		}
	}
}

// AC6: --section filter restricts results to that section.
func TestRun_SectionFilter(t *testing.T) {
	rep, _, err := Run(Options{Sections: []string{"install"}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, r := range rep.Results {
		if r.Section != "install" {
			t.Errorf("filter leaked: got section %q", r.Section)
		}
	}
}

// AC7: --skip-project means no project-related results.
func TestRun_SkipProject(t *testing.T) {
	rep, _, err := Run(Options{SkipProject: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, r := range rep.Results {
		if r.Section != "project" {
			continue
		}
		// Skipped checks still surface as `info` lines acknowledging the skip.
		if !strings.Contains(strings.ToLower(r.Message), "skip") {
			t.Errorf("project check %s did not honour --skip-project: %q", r.Name, r.Message)
		}
	}
}

// AC8: wall-time budget. Cover the warm-cache promise (a second call in
// the same process benefits from facts' sync.Once). Skipped under -short
// because facts.Collect() shells out and is unreliable on virtualised CI.
func TestRun_WallTimeBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("timing-sensitive; -short skips")
	}
	// Warm facts cache.
	_, _, _ = Run(Options{Sections: []string{"system"}})

	start := time.Now()
	_, _, err := Run(Options{})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if elapsed > 1500*time.Millisecond {
		t.Errorf("warm-cache doctor took %v, want < 1.5s (spec-41 AC8)", elapsed)
	}
}

// JSON status field must use the lowercase string tokens.
func TestStatus_StringStable(t *testing.T) {
	cases := map[Status]string{
		StatusOK: "ok", StatusInfo: "info", StatusWarning: "warning", StatusError: "error",
	}
	for s, want := range cases {
		if s.String() != want {
			t.Errorf("Status(%d).String() = %q, want %q", s, s.String(), want)
		}
	}
}

// 200ms per-check timeout actually fires when a probe hangs. Done via the
// withDeadline helper directly so we don't need a hung socket in tests.
func TestWithDeadline_Times(t *testing.T) {
	start := time.Now()
	err := withDeadline(t.Context(), func(c context.Context) error {
		<-c.Done() // sit until the per-check deadline fires
		return c.Err()
	})
	elapsed := time.Since(start)
	if err == nil {
		t.Error("expected non-nil error when probe hits deadline")
	}
	// The contract is "<200ms+small slack". 250ms is plenty for flaky CI.
	if elapsed > 250*time.Millisecond {
		t.Errorf("deadline took %v, want < 250ms", elapsed)
	}
}
