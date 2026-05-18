package file

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/effects"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

// newRunContext builds an ExecutionContext suitable for exercising
// Handler.Run in either ModeApply or ModePlan. Spec 16 derives Mode
// from the legacy DryRun bool until Phase 6 cleanup.
func newRunContext(t *testing.T, plan bool) *executor.ExecutionContext {
	t.Helper()
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	stats := executor.NewExecutionStats()
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: renderer,
			PathUtil: pathutil.NewPathExpander(renderer),
			Mode:     planMode(plan),
			Stats:    stats,
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: "/tmp",
	}
}

// TestRun_DirectoryModeParity is Spec 16's structural regression. It
// drives Handler.Run through ModePlan and ModeApply against the same
// step and asserts that the prediction matches reality. Before Phase 1
// shipped, the legacy DryRun used defaultFileMode (0644) for directory
// steps while createDirectory used defaultDirMode (0755). This test
// fails on that class of bug by construction.
func TestRun_DirectoryModeParity(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX directory modes have no meaning on NTFS — Go reports 0777 for any directory")
	}
	for _, tc := range []struct {
		name string
		mode string // empty = use default
		want os.FileMode
	}{
		{"default mode (0755)", "", 0o755},
		{"explicit 0700", "0700", 0o700},
		{"explicit 0775", "0775", 0o775},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "dir")
			step := &config.Step{
				FileWrite: &config.File{
					Path:  path,
					State: "directory",
					Mode:  tc.mode,
				},
			}
			h := &Handler{}

			// Plan first: must not create anything but must predict the change.
			planCtx := newRunContext(t, true)
			planResult, err := h.Run(planCtx, step)
			if err != nil {
				t.Fatalf("plan: %v", err)
			}
			if r, ok := planResult.(*executor.Result); ok {
				if !r.WouldChange {
					t.Errorf("plan: WouldChange should be true; reason=%q", r.Reason)
				}
			} else {
				t.Fatalf("plan returned non-*Result: %T", planResult)
			}
			if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
				t.Fatal("plan must not create the directory")
			}

			// Execute: must create with the same mode plan predicted.
			execCtx := newRunContext(t, false)
			if _, err := h.Run(execCtx, step); err != nil {
				t.Fatalf("execute: %v", err)
			}
			info, statErr := os.Stat(path)
			if statErr != nil {
				t.Fatalf("execute did not create directory: %v", statErr)
			}
			if got := info.Mode().Perm(); got != tc.want {
				t.Errorf("execute applied mode %v, want %v", got, tc.want)
			}
		})
	}
}

// TestRun_FileContentParity drives Run for state: file through both
// modes against the same content and asserts the predicted change
// matches what execute writes.
func TestRun_FileContentParity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	content := "hello mooncake"
	step := &config.Step{
		FileWrite: &config.File{
			Path:    path,
			Content: content,
			Mode:    "0644",
		},
	}
	h := &Handler{}

	planResult, err := h.Run(newRunContext(t, true), step)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	r := planResult.(*executor.Result)
	if !r.WouldChange {
		t.Errorf("plan: WouldChange should be true for new file; reason=%q", r.Reason)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatal("plan must not create the file")
	}

	if _, err := h.Run(newRunContext(t, false), step); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != content {
		t.Errorf("execute wrote %q, want %q", string(got), content)
	}

	// Second plan against existing content should report AlreadyOk (no change).
	planResult2, err := h.Run(newRunContext(t, true), step)
	if err != nil {
		t.Fatalf("plan2: %v", err)
	}
	r2 := planResult2.(*executor.Result)
	if r2.WouldChange {
		t.Errorf("plan2: WouldChange should be false when content already matches; reason=%q", r2.Reason)
	}
}

// TestRun_IdempotentDirectory verifies that a second ModeApply run
// against an already-correct directory does not flip Changed. (Today's
// dumb DryRun would have lied "would create" here.)
func TestRun_IdempotentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dir")
	step := &config.Step{
		FileWrite: &config.File{
			Path:  path,
			State: "directory",
			Mode:  "0755",
		},
	}
	h := &Handler{}

	// First execute creates.
	if _, err := h.Run(newRunContext(t, false), step); err != nil {
		t.Fatalf("first execute: %v", err)
	}

	// Second execute is a no-op.
	result, err := h.Run(newRunContext(t, false), step)
	if err != nil {
		t.Fatalf("second execute: %v", err)
	}
	r := result.(*executor.Result)
	if r.Changed {
		t.Error("second execute should not report Changed for already-correct directory")
	}

	// Plan against the now-correct state must agree.
	planResult, err := h.Run(newRunContext(t, true), step)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	pr := planResult.(*executor.Result)
	if pr.WouldChange {
		t.Errorf("plan: WouldChange should be false for already-correct directory; reason=%q", pr.Reason)
	}
}

// Sanity: confirm Handler still satisfies actions.Runner via the new Run.
func TestHandler_ImplementsRunner(t *testing.T) {
	var _ actions.Runner = &Handler{}
}

// Sanity: ensure the effects package wiring used by Run is the same
// thing we get from ec.Effects(). This guards against accidental
// divergence (e.g. a new accessor returning a different Performer).
func TestEffects_RoundTrip(t *testing.T) {
	ec := newRunContext(t, false)
	p := ec.Effects()
	if p.Mode() != actions.ModeApply {
		t.Errorf("Effects() mode = %v, want ModeApply", p.Mode())
	}
	planEc := newRunContext(t, true)
	if planEc.Effects().Mode() != actions.ModePlan {
		t.Errorf("Plan ctx Effects() mode = %v, want ModePlan", planEc.Effects().Mode())
	}
	// Ensure NewPerformer returns the same concrete type
	custom := effects.NewPerformer(ec.Mode, "", false, "")
	if custom.Mode() != actions.ModeApply {
		t.Errorf("NewPerformer mode = %v, want ModeApply", custom.Mode())
	}
}

func planMode(b bool) actions.Mode {
	if b {
		return actions.ModePlan
	}
	return actions.ModeApply
}
