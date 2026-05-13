package wait_command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func planMode(b bool) actions.Mode {
	if b {
		return actions.ModePlan
	}
	return actions.ModeApply
}

func TestRun_ImplementsRunner(t *testing.T) {
	var _ actions.Runner = &Handler{}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{"nil", &config.Step{}, true},
		{"missing cmd", &config.Step{WaitCommand: &config.WaitCommand{}}, true},
		{"ok", &config.Step{WaitCommand: &config.WaitCommand{Cmd: "true"}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := (&Handler{}).Validate(c.step)
			if (err != nil) != c.wantErr {
				t.Errorf("err=%v wantErr=%v", err, c.wantErr)
			}
		})
	}
}

// TestRun_Plan surfaces the command.
func TestRun_Plan(t *testing.T) {
	step := &config.Step{WaitCommand: &config.WaitCommand{Cmd: "pg_isready"}}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Error("plan should report WouldChange")
	}
	if !strings.Contains(r.Reason, "pg_isready") {
		t.Errorf("reason should include cmd; got %q", r.Reason)
	}
}

// TestRun_Apply_HappyPath: `true` exits 0 on first try.
func TestRun_Apply_HappyPath(t *testing.T) {
	step := &config.Step{WaitCommand: &config.WaitCommand{
		Cmd:     "true",
		Timeout: "2s",
	}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Data["success"] != true {
		t.Errorf("expected success=true; data=%v", r.Data)
	}
	if r.Data["iterations"].(int) != 1 {
		t.Errorf("expected single iteration; got %v", r.Data["iterations"])
	}
}

// TestRun_Apply_FileBecomesReady: poll until a marker file is created.
func TestRun_Apply_FileBecomesReady(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "ready")
	cmd := "test -e " + marker

	go func() {
		time.Sleep(150 * time.Millisecond)
		_ = os.WriteFile(marker, nil, 0o644)
	}()

	step := &config.Step{WaitCommand: &config.WaitCommand{
		Cmd:          cmd,
		Timeout:      "2s",
		PollInterval: "100ms",
	}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Data["iterations"].(int) < 2 {
		t.Errorf("expected multiple iterations; got %v", r.Data["iterations"])
	}
}

// TestRun_Apply_Timeout: command never reaches expected exit.
func TestRun_Apply_Timeout(t *testing.T) {
	step := &config.Step{WaitCommand: &config.WaitCommand{
		Cmd:          "false",
		Timeout:      "200ms",
		PollInterval: "100ms",
	}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected timeout")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("expected timeout error; got %v", err)
	}
}

// TestRun_Apply_ExpectNonZero: a non-zero exit code can be the success condition.
func TestRun_Apply_ExpectNonZero(t *testing.T) {
	step := &config.Step{WaitCommand: &config.WaitCommand{
		Cmd:        "false",
		ExpectExit: 1,
		Timeout:    "2s",
	}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Data["success"] != true {
		t.Errorf("expected success=true; data=%v", r.Data)
	}
}
