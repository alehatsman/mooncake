package executor_test

// Spec-23 §2 try/catch/finally executor tests. End-to-end via the
// planner+executor combo — writes a YAML config with a try-block,
// applies it, and asserts filesystem side effects.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/alehatsman/mooncake/internal/register"
)

// TestTry_HappyPath verifies the no-error path: every try child
// succeeds, catch is skipped, finally runs.
func TestTry_HappyPath(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	finallyMark := filepath.Join(dir, "finally-ran")
	catchMark := filepath.Join(dir, "catch-ran")

	yaml := `version: "1.0"
steps:
  - name: deploy
    try:
      - file.write: { path: ` + a + `, content: A }
      - file.write: { path: ` + b + `, content: B }
    catch:
      - file.write: { path: ` + catchMark + `, content: catch }
    finally:
      - file.write: { path: ` + finallyMark + `, content: finally }
`
	if err := runConfig(t, yaml); err != nil {
		t.Fatalf("apply errored unexpectedly: %v", err)
	}
	for _, p := range []string{a, b} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected file %s to exist after try success; got %v", p, err)
		}
	}
	if _, err := os.Stat(finallyMark); err != nil {
		t.Errorf("expected finally marker %s to exist; got %v", finallyMark, err)
	}
	if _, err := os.Stat(catchMark); err == nil {
		t.Errorf("catch must NOT run when try succeeds; %s exists", catchMark)
	}
}

// TestTry_FailurePath: middle try step fails. Subsequent try steps
// skip; catch runs; finally runs; overall apply errors (exit non-zero
// per spec-23 acceptance criteria).
func TestTry_FailurePath(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first")
	skipped := filepath.Join(dir, "skipped")
	catchMark := filepath.Join(dir, "catch-ran")
	finallyMark := filepath.Join(dir, "finally-ran")

	// /dev/null/... is ENOTDIR — same trick spec-30 transaction tests
	// use to force a file.write failure deterministically.
	badPath := "/dev/null/cannot-write-here"

	yaml := `version: "1.0"
steps:
  - name: deploy
    try:
      - file.write: { path: ` + first + `, content: first }
      - file.write: { path: ` + badPath + `, content: bad }
      - file.write: { path: ` + skipped + `, content: skipped }
    catch:
      - file.write: { path: ` + catchMark + `, content: catch }
    finally:
      - file.write: { path: ` + finallyMark + `, content: finally }
`
	err := runConfig(t, yaml)
	if err == nil {
		t.Fatal("expected apply to error (try-block failed and catch does not swallow); got nil")
	}

	// First try child wrote; second errored; third must have skipped.
	if _, err := os.Stat(first); err != nil {
		t.Errorf("expected %s to exist (first try child succeeded); got %v", first, err)
	}
	if _, err := os.Stat(skipped); err == nil {
		t.Errorf("third try child must skip after sibling failure; %s exists", skipped)
	}
	// Catch must fire; finally must fire.
	if _, err := os.Stat(catchMark); err != nil {
		t.Errorf("expected catch marker %s after try failure; got %v", catchMark, err)
	}
	if _, err := os.Stat(finallyMark); err != nil {
		t.Errorf("expected finally marker %s; got %v", finallyMark, err)
	}
}

// TestTry_FinallyRunsEvenWhenCatchAlsoFails: catch errors after a try
// failure. Finally must still run. Per spec ("the compound Step
// propagates the later error"), the catch error replaces the try
// error in the propagated message.
func TestTry_FinallyRunsEvenWhenCatchAlsoFails(t *testing.T) {
	dir := t.TempDir()
	finallyMark := filepath.Join(dir, "finally-ran")

	badTry := "/dev/null/x"
	badCatch := "/dev/null/y"

	yaml := `version: "1.0"
steps:
  - name: deploy
    try:
      - file.write: { path: ` + badTry + `, content: a }
    catch:
      - file.write: { path: ` + badCatch + `, content: b }
    finally:
      - file.write: { path: ` + finallyMark + `, content: c }
`
	err := runConfig(t, yaml)
	if err == nil {
		t.Fatal("expected error when both try and catch fail")
	}
	if _, err := os.Stat(finallyMark); err != nil {
		t.Errorf("expected finally marker %s even after catch fails; got %v", finallyMark, err)
	}
}

// TestTry_CatchOnlyRunsOnFailure: no try error → catch must NOT run.
// finally always runs. Combined as a single test to keep coverage tight.
func TestTry_NoCatchWhenTrySucceeds(t *testing.T) {
	dir := t.TempDir()
	tryMark := filepath.Join(dir, "try-ok")
	catchMark := filepath.Join(dir, "catch-ran")

	yaml := `version: "1.0"
steps:
  - name: ok
    try:
      - file.write: { path: ` + tryMark + `, content: ok }
    catch:
      - file.write: { path: ` + catchMark + `, content: ran }
`
	if err := runConfig(t, yaml); err != nil {
		t.Fatalf("apply errored unexpectedly: %v", err)
	}
	if _, err := os.Stat(tryMark); err != nil {
		t.Errorf("expected try marker %s; got %v", tryMark, err)
	}
	if _, err := os.Stat(catchMark); err == nil {
		t.Error("catch must not run when try succeeds")
	}
}

// TestTry_ValidationRejectsOrphanCatch: catch without try is invalid.
func TestTry_ValidationRejectsOrphanCatch(t *testing.T) {
	yaml := `version: "1.0"
steps:
  - name: bad
    catch:
      - file.write: { path: /tmp/x, content: x }
`
	err := runConfig(t, yaml)
	if err == nil {
		t.Fatal("expected validation error for catch without try")
	}
	// Orphan catch fails the schema's oneOf (catch alone matches no
	// branch). The error message is the generic "Step must have
	// exactly one action..." — accept it as a signal that the schema
	// did reject the config.
	if !strings.Contains(err.Error(), "Step must have exactly one action") &&
		!strings.Contains(err.Error(), "catch") &&
		!strings.Contains(err.Error(), "try") {
		t.Errorf("err should signal validation failure; got: %v", err)
	}
}

// --- Issue #23: continue_on_error: true on a try compound -----------------
//
// `continue_on_error` is a universal Step field. Before the fix, it was
// silently dropped on `try:` compound steps — the inner failure ran
// through catch/finally as expected, then halted the outer run instead
// of tolerating per the operator's directive.

func TestTry_ContinueOnError_LetsRunProceed(t *testing.T) {
	dir := t.TempDir()
	catchMark := filepath.Join(dir, "catch-ran")
	afterMark := filepath.Join(dir, "after-ran")
	badPath := "/dev/null/x"

	yaml := `version: "1.0"
steps:
  - name: deploy
    continue_on_error: true
    try:
      - file.write: { path: ` + badPath + `, content: bad }
    catch:
      - file.write: { path: ` + catchMark + `, content: catch }

  - name: after
    file.write: { path: ` + afterMark + `, content: after }
`
	err := runConfig(t, yaml)
	if err != nil {
		t.Fatalf("expected apply to succeed with continue_on_error on the compound; got %v", err)
	}
	// Catch ran (handled the failure).
	if _, err := os.Stat(catchMark); err != nil {
		t.Errorf("expected catch to run; %s missing: %v", catchMark, err)
	}
	// The next top-level step ran.
	if _, err := os.Stat(afterMark); err != nil {
		t.Errorf("expected after-try step to run when continue_on_error was set on the compound; %s missing: %v", afterMark, err)
	}
}

func TestTry_NoContinueOnError_HaltsAsBefore(t *testing.T) {
	// Negative: without continue_on_error, the existing TestTry_FailurePath
	// behavior is preserved. Confirms the swallow is opt-in.
	dir := t.TempDir()
	catchMark := filepath.Join(dir, "catch-ran")
	afterMark := filepath.Join(dir, "after-ran")
	badPath := "/dev/null/x"

	yaml := `version: "1.0"
steps:
  - name: deploy
    try:
      - file.write: { path: ` + badPath + `, content: bad }
    catch:
      - file.write: { path: ` + catchMark + `, content: catch }

  - name: after
    file.write: { path: ` + afterMark + `, content: after }
`
	err := runConfig(t, yaml)
	if err == nil {
		t.Fatal("expected error to propagate without continue_on_error")
	}
	// Catch still fires (the inner failure-handling path is untouched).
	if _, err := os.Stat(catchMark); err != nil {
		t.Errorf("expected catch to run regardless; %s missing: %v", catchMark, err)
	}
	// The after step must NOT have run.
	if _, err := os.Stat(afterMark); err == nil {
		t.Errorf("after-try step must NOT run without continue_on_error; %s exists", afterMark)
	}
}
