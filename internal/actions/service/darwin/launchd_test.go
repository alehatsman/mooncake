package darwin

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/service/shared"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/security"
	"github.com/alehatsman/mooncake/internal/template"
)

// newMockExecutionContext builds a hermetic ExecutionContext for
// darwin-only tests. Duplicated from the parent service package's
// test helper (Go test packages can't import helpers from a
// sibling package without a separate testutil layer); the
// duplication is ~20 lines and keeps the per-OS sub-package's test
// surface self-contained.
func newMockExecutionContext() *executor.ExecutionContext {
	tmpl, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template:       tmpl,
			Evaluator:      expression.NewExprEvaluator(),
			PathUtil:       pathutil.NewPathExpander(tmpl),
			Logger:         &testutil.MockLogger{Logs: []string{}},
			EventPublisher: &testutil.MockPublisher{Events: []events.Event{}},
			Redactor:       security.NewRedactor(),
			SudoPass:       "",
			Stats:          executor.NewExecutionStats(),
			Mode:           actions.ModeApply,
		},
		Scope:         executor.NewVariableScope(),
		CurrentStepID: "step-1",
	}
}

func TestIsSystemScope(t *testing.T) {
	cases := []struct {
		name    string
		scope   string
		become  bool
		wantSys bool
	}{
		{"scope=system", "system", false, true},
		{"scope=System (case)", "System", false, true},
		{"scope=user", "user", false, false},
		{"scope=user beats become=true", "user", true, false},
		{"scope=system beats become=false", "system", false, true},
		{"no scope, become=true", "", true, true},
		{"no scope, become=false", "", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sa := &config.ServiceAction{Name: "svc", Scope: c.scope}
			step := config.Step{OsService: sa}
			if c.become {
				step.AsUser = "root"
			}
			got := isSystemScope(sa, step)
			if got != c.wantSys {
				t.Errorf("isSystemScope=%v, want %v", got, c.wantSys)
			}
		})
	}
}

func TestIsServiceLoaded(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping launchd test on non-macOS")
	}

	ctx := newMockExecutionContext()
	step := config.Step{}

	loaded, err := IsServiceLoaded("com.nonexistent.test", step, ctx)
	if err != nil {
		t.Logf("IsServiceLoaded error (expected in test env): %v", err)
	} else if loaded {
		t.Error("IsServiceLoaded() should return false for nonexistent service")
	}
}

func TestBootstrap(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping launchd test on non-macOS")
	}

	ctx := newMockExecutionContext()
	tmpDir := t.TempDir()
	plistPath := filepath.Join(tmpDir, "test.plist")

	plistContent := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.test.bootstrap</string>
</dict>
</plist>`

	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		t.Fatalf("Failed to create plist: %v", err)
	}

	step := config.Step{}
	err := bootstrap("gui/501", plistPath, step, ctx)
	t.Logf("bootstrap error (expected): %v", err)
}

func TestExecuteLaunchctlCommand_Error(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping launchd test on non-macOS")
	}

	ctx := newMockExecutionContext()
	step := config.Step{}

	err := executeLaunchctlCommand("invalid-subcommand", "gui/501", "/tmp/test.plist", step, ctx, nil, "success", "error")
	if err == nil {
		t.Log("executeLaunchctlCommand with invalid command succeeded (unexpected)")
	} else {
		t.Logf("executeLaunchctlCommand error (expected): %v", err)
	}
}

func TestManageState_Started(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping launchd test on non-macOS")
	}

	ctx := newMockExecutionContext()
	tmpDir := t.TempDir()
	plistPath := filepath.Join(tmpDir, "test-started.plist")

	plistContent := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.test.started</string>
</dict>
</plist>`

	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		t.Fatalf("Failed to create plist: %v", err)
	}

	step := config.Step{}
	changed, err := manageState("com.test.started", "gui/501/com.test.started", plistPath, "gui/501", shared.StateStarted, false, step, ctx)
	t.Logf("manageState result: changed=%v, err=%v", changed, err)
}

func TestManageEnabled(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping launchd test on non-macOS")
	}

	ctx := newMockExecutionContext()
	tmpDir := t.TempDir()
	plistPath := filepath.Join(tmpDir, "test-enabled.plist")

	plistContent := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.test.enabled</string>
</dict>
</plist>`

	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		t.Fatalf("Failed to create plist: %v", err)
	}

	step := config.Step{}
	changed, err := manageEnabled("gui/501/com.test.enabled", plistPath, "gui/501", true, false, step, ctx)
	t.Logf("manageEnabled result: changed=%v, err=%v", changed, err)
}

func TestBootout(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping launchd test on non-macOS")
	}

	ctx := newMockExecutionContext()
	step := config.Step{}

	err := bootout("gui/501", "/tmp/nonexistent.plist", step, ctx)
	t.Logf("bootout error (expected): %v", err)
}

func TestKickstart(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping launchd test on non-macOS")
	}

	ctx := newMockExecutionContext()
	step := config.Step{}

	err := kickstart("gui/501/com.test.service", false, step, ctx)
	t.Logf("kickstart error (expected): %v", err)
}

func TestKill(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping launchd test on non-macOS")
	}

	ctx := newMockExecutionContext()
	step := config.Step{}

	err := kill("gui/501/com.test.service", step, ctx)
	t.Logf("kill error (expected): %v", err)
}
