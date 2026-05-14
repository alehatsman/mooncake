//nolint:revive // package name follows action convention
package text_line

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// TestReverse_Cycle is the standard apply→reverse→verify pass for
// text.line. Reuses the existing newCtx helper from run_test.go.
func TestReverse_Cycle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")
	const original = "alpha\nbeta\ngamma\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	step := &config.Step{
		TextLine: &config.TextLine{
			Path: path,
			Line: "beta=42",
			Regexp: "^beta",
		},
	}
	h := &Handler{}

	res, err := h.Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	r := res.(*executor.Result)
	if !r.Changed {
		t.Fatalf("apply: expected Changed=true")
	}
	got, _ := os.ReadFile(path)
	if string(got) == original {
		t.Fatal("apply: file content unchanged")
	}

	info := r.ReverseData.(*filehandler.FileReverseInfo)
	if string(info.Content) != original {
		t.Errorf("captured Content = %q, want %q", info.Content, original)
	}

	reverseStep, err := h.Reverse(nil, step, r)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if reverseStep.FileWrite == nil || reverseStep.FileWrite.Content != original {
		t.Fatalf("reverse step bad: %+v", reverseStep.FileWrite)
	}

	// Apply the reverse step via file.write handler.
	fh := &filehandler.Handler{}
	if _, err := fh.Run(newCtx(t, false), reverseStep); err != nil {
		t.Fatalf("reverse apply: %v", err)
	}
	got, _ = os.ReadFile(path)
	if string(got) != original {
		t.Errorf("after reverse: %q, want %q", got, original)
	}
}

func TestHandler_ImplementsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
	if !actions.IsReverser(&Handler{}) {
		t.Error("actions.IsReverser((*Handler)) = false; want true")
	}
}
