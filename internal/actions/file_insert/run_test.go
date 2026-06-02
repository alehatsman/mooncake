package file_insert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

func newCtx(t *testing.T, plan bool) *executor.ExecutionContext {
	t.Helper()
	r, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatal(err)
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: r,
			PathUtil: pathutil.NewPathExpander(r),
			Logger:   logger.NewLogger(logger.ErrorLevel),
			Mode:     planMode(plan),
			Stats:    executor.NewExecutionStats(),
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: "/tmp",
	}
}

// TestRun_InsertWouldHappen: plan reports the change, execute performs
// it, and the file ends up matching the prediction.
func TestRun_InsertWouldHappen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(path, []byte("# header\nline\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	step := &config.Step{
		TextInsert: &config.FileInsert{
			Path:     path,
			Anchor:   "# header",
			Position: "after",
			Content:  "new_line",
		},
	}
	h := &Handler{}

	res, err := h.Run(newCtx(t, true), step)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Errorf("plan: WouldChange should be true; reason=%q", r.Reason)
	}

	if _, err := h.Run(newCtx(t, false), step); err != nil {
		t.Fatalf("execute: %v", err)
	}
	content, _ := os.ReadFile(path)
	if !strings.Contains(string(content), "new_line") {
		t.Errorf("execute should have inserted; got %q", string(content))
	}
}

// TestRun_AnchorMissingIsIdempotent: anchor not present → no error,
// no change. MT-47 flipped the semantics from fail-loud to
// idempotent-success so a second run of a playbook that already
// inserted (and so altered/removed the anchor) succeeds cleanly.
func TestRun_AnchorMissingIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	original := []byte("only one line\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	step := &config.Step{
		TextInsert: &config.FileInsert{
			Path:     path,
			Anchor:   "no_such_anchor",
			Position: "after",
			Content:  "ignored",
		},
	}
	h := &Handler{}

	if _, err := h.Run(newCtx(t, true), step); err != nil {
		t.Errorf("plan: expected no error on missing anchor; got %v", err)
	}
	if _, err := h.Run(newCtx(t, false), step); err != nil {
		t.Errorf("execute: expected no error on missing anchor; got %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(original) {
		t.Errorf("file mutated despite missing anchor: %q", got)
	}
}

// TestRun_PreservesFileMode: an insert into a 0600 file must keep the
// original mode rather than clobbering it to 0644.
func TestRun_PreservesFileMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(path, []byte("# header\nline\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	step := &config.Step{
		TextInsert: &config.FileInsert{
			Path:     path,
			Anchor:   "# header",
			Position: "after",
			Content:  "new_line",
		},
	}
	h := &Handler{}
	if _, err := h.Run(newCtx(t, false), step); err != nil {
		t.Fatalf("execute: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode clobbered: want 0600, got %o", got)
	}
}

func TestRun_ImplementsRunner(t *testing.T) {
	var _ actions.Runner = &Handler{}
}

func planMode(b bool) actions.Mode {
	if b {
		return actions.ModePlan
	}
	return actions.ModeApply
}
