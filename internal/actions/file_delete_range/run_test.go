package file_delete_range

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
		Variables:  map[string]interface{}{},
		Template:   r,
		PathUtil:   pathutil.NewPathExpander(r),
		Logger:     logger.NewLogger(logger.ErrorLevel),
		CurrentDir: "/tmp",
		CurrentMode: planMode(plan),
		Stats:      executor.NewExecutionStats(),
	}
}

func TestRun_DeletionWouldHappen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	original := "before\n<<START\nmiddle\nEND>>\nafter\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	step := &config.Step{
		TextDeleteRange: &config.FileDeleteRange{
			Path:        path,
			StartAnchor: "<<START",
			EndAnchor:   "END>>",
			Inclusive:   true,
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
	cur, _ := os.ReadFile(path)
	if string(cur) != original {
		t.Error("plan must not modify the file")
	}

	if _, err := h.Run(newCtx(t, false), step); err != nil {
		t.Fatalf("execute: %v", err)
	}
	cur, _ = os.ReadFile(path)
	if strings.Contains(string(cur), "middle") {
		t.Errorf("execute should have deleted middle; got %q", string(cur))
	}
}

// TestRun_NoRange: range absent → error in both modes (consistent
// failure between plan and execute).
func TestRun_NoRange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(path, []byte("no anchors here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	step := &config.Step{
		TextDeleteRange: &config.FileDeleteRange{
			Path:        path,
			StartAnchor: "missing_start",
			EndAnchor:   "missing_end",
		},
	}
	h := &Handler{}

	if _, err := h.Run(newCtx(t, true), step); err == nil {
		t.Error("plan: expected error when range absent")
	}
	if _, err := h.Run(newCtx(t, false), step); err == nil {
		t.Error("execute: expected error when range absent")
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
