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
			Logger: logger.NewLogger(logger.ErrorLevel),
			Mode: planMode(plan),
			Stats: executor.NewExecutionStats(),
		},
		Variables: map[string]interface{}{},
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

// TestRun_AnchorMissing: anchor not present → error in both modes
// (the underlying performInsertion treats missing anchors as a hard
// failure; consistent behavior between plan and execute).
func TestRun_AnchorMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(path, []byte("only one line\n"), 0o644); err != nil {
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

	if _, err := h.Run(newCtx(t, true), step); err == nil {
		t.Error("plan: expected error when anchor missing")
	}
	if _, err := h.Run(newCtx(t, false), step); err == nil {
		t.Error("execute: expected error when anchor missing")
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
