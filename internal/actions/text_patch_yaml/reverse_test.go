//nolint:revive // package name follows action convention
package text_patch_yaml

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

func TestReverse_Cycle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	const original = "server:\n  port: 80\n  host: localhost\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	step := &config.Step{
		TextPatchYAML: &config.TextPatchYAML{
			Path: path,
			Set:  map[string]interface{}{"server.port": 8080},
		},
	}
	h := &Handler{}

	res, err := h.Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	r := res.(*executor.Result)
	info := r.ReverseData.(*filehandler.FileReverseInfo)
	if string(info.Content) != original {
		t.Fatalf("captured Content = %q, want %q", info.Content, original)
	}

	reverseStep, err := h.Reverse(nil, step, r)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}

	fh := &filehandler.Handler{}
	if _, err := fh.Run(newCtx(t, false), reverseStep); err != nil {
		t.Fatalf("reverse apply: %v", err)
	}
	got, _ := os.ReadFile(path)
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
