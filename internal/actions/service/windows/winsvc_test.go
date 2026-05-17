package windows

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
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
// windows-only tests. Duplicated from the parent service package's
// test helper (Go test packages can't import helpers from a sibling
// package without a separate testutil layer).
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

// TestQuotePS_EscapesSingleQuote pins the only injection surface in
// winsvc.go. Single-quote PS strings are literal — no expansion, no
// backtick escapes — so doubling an embedded single quote is the
// safe escape.
func TestQuotePS_EscapesSingleQuote(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"MSSQLSERVER", "'MSSQLSERVER'"},
		{"My Service", "'My Service'"},
		{"O'Brien", "'O''Brien'"},
		{"", "''"},
		{"'", "''''"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := QuotePS(c.in); got != c.want {
				t.Errorf("QuotePS(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// stubExec replaces the powershell hook with a recorder. Returns
// the recorded scripts so tests can pin the exact PowerShell
// invoked. The replyFn lets a test return different stdout per
// script (used for the multi-call probe→write flow).
func stubExec(t *testing.T, replyFn func(script string) (string, error)) *[]string {
	t.Helper()
	calls := []string{}
	prev := Exec
	Exec = func(script string) (string, error) {
		calls = append(calls, script)
		return replyFn(script)
	}
	t.Cleanup(func() { Exec = prev })
	return &calls
}

// TestHandle_RefusesUnit pins the v1 scope: Unit declarations on
// windows error with a clear "use sc.exe create first" message
// rather than silently no-op or generate confusing SCM errors.
func TestHandle_RefusesUnit(t *testing.T) {
	stubExec(t, func(string) (string, error) {
		t.Errorf("PowerShell must not be invoked when Unit is set on windows")
		return "", nil
	})
	step := config.Step{
		OsService: &config.ServiceAction{
			Name: "MyService",
			Unit: &config.ServiceUnit{Content: "..."},
		},
	}
	err := Handle("MyService", step.OsService, step, newMockExecutionContext())
	if err == nil {
		t.Fatal("expected SetupError refusing unit on windows")
	}
	var se *executor.SetupError
	if !errors.As(err, &se) {
		t.Errorf("error must be *SetupError; got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "unit") {
		t.Errorf("error must mention 'unit'; got: %v", err)
	}
}

// TestHandle_RefusesMissingService pins the v1 scope for the
// "service doesn't exist" path. Get-Service returns nothing →
// SetupError with a hint pointing at sc.exe create. Avoids the
// silent-no-op trap where a typo in the service name reports
// success.
func TestHandle_RefusesMissingService(t *testing.T) {
	stubExec(t, func(script string) (string, error) {
		if strings.Contains(script, "Get-Service") {
			return "", nil
		}
		return "", nil
	})
	step := config.Step{OsService: &config.ServiceAction{Name: "DoesNotExist", State: "started"}}
	err := Handle("DoesNotExist", step.OsService, step, newMockExecutionContext())
	if err == nil {
		t.Fatal("expected error for missing service")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error must mention 'not found'; got: %v", err)
	}
}

// TestHandle_StartAlreadyRunning_Noop verifies the idempotent path:
// service is Running, state=started → Changed=false, no
// Start-Service call.
func TestHandle_StartAlreadyRunning_Noop(t *testing.T) {
	calls := stubExec(t, func(script string) (string, error) {
		if strings.Contains(script, "Get-Service") {
			return `{"Name":"MyService","DisplayName":"My","Status":4,"StartType":2,"CanStop":true}`, nil
		}
		t.Errorf("unexpected PowerShell call: %s", script)
		return "", nil
	})
	step := config.Step{OsService: &config.ServiceAction{Name: "MyService", State: "started"}}
	ec := newMockExecutionContext()
	err := Handle("MyService", step.OsService, step, ec)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if ec.CurrentResult == nil || ec.CurrentResult.Changed {
		t.Errorf("expected Changed=false; got %+v", ec.CurrentResult)
	}
	if len(*calls) != 1 {
		t.Errorf("expected 1 PowerShell call; got %d: %v", len(*calls), *calls)
	}
}

// TestHandle_StartWhenStopped invokes Start-Service when the
// service is in Stopped state. Verifies the script shape matches
// what the operator would expect to see in audit logs.
func TestHandle_StartWhenStopped(t *testing.T) {
	calls := stubExec(t, func(script string) (string, error) {
		if strings.Contains(script, "Get-Service") {
			return `{"Name":"MyService","Status":1,"StartType":2,"CanStop":true}`, nil
		}
		if strings.Contains(script, "Start-Service") {
			return "", nil
		}
		return "", nil
	})
	step := config.Step{OsService: &config.ServiceAction{Name: "MyService", State: "started"}}
	ec := newMockExecutionContext()
	err := Handle("MyService", step.OsService, step, ec)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !ec.CurrentResult.Changed {
		t.Errorf("expected Changed=true; got %+v", ec.CurrentResult)
	}
	if len(*calls) != 2 {
		t.Fatalf("expected 2 calls (Get-Service + Start-Service); got %v", *calls)
	}
	if !strings.Contains((*calls)[1], "Start-Service") || !strings.Contains((*calls)[1], "MyService") {
		t.Errorf("second call should be Start-Service MyService; got %q", (*calls)[1])
	}
}

// TestHandle_StopNotStoppable refuses to stop a service whose
// CanStop flag is false.
func TestHandle_StopNotStoppable(t *testing.T) {
	stubExec(t, func(script string) (string, error) {
		if strings.Contains(script, "Get-Service") {
			return `{"Name":"CriticalSvc","Status":4,"StartType":2,"CanStop":false}`, nil
		}
		t.Errorf("must not call Stop-Service when CanStop=false; got: %s", script)
		return "", nil
	})
	step := config.Step{OsService: &config.ServiceAction{Name: "CriticalSvc", State: "stopped"}}
	err := Handle("CriticalSvc", step.OsService, step, newMockExecutionContext())
	if err == nil {
		t.Fatal("expected error for non-stoppable service")
	}
	if !strings.Contains(err.Error(), "not stoppable") {
		t.Errorf("error must mention 'not stoppable'; got: %v", err)
	}
}

// TestHandle_Restart_AlwaysChanged_AlwaysRuns. Restart-Service
// deliberately bounces the service even if it wasn't running
// pre-call (it stops if running, then starts).
func TestHandle_Restart_AlwaysChanged(t *testing.T) {
	stubExec(t, func(script string) (string, error) {
		if strings.Contains(script, "Get-Service") {
			return `{"Name":"MyService","Status":4,"StartType":2,"CanStop":true}`, nil
		}
		return "", nil
	})
	step := config.Step{OsService: &config.ServiceAction{Name: "MyService", State: "restarted"}}
	ec := newMockExecutionContext()
	if err := Handle("MyService", step.OsService, step, ec); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !ec.CurrentResult.Changed {
		t.Errorf("state=restarted must always report Changed=true")
	}
}

// TestHandle_Reload_Refused — SCM has no reload analog.
func TestHandle_Reload_Refused(t *testing.T) {
	stubExec(t, func(script string) (string, error) {
		if strings.Contains(script, "Get-Service") {
			return `{"Name":"MyService","Status":4,"StartType":2,"CanStop":true}`, nil
		}
		return "", nil
	})
	step := config.Step{OsService: &config.ServiceAction{Name: "MyService", State: "reloaded"}}
	err := Handle("MyService", step.OsService, step, newMockExecutionContext())
	if err == nil {
		t.Fatal("expected error for reloaded state on windows")
	}
	if !strings.Contains(err.Error(), "reloaded") {
		t.Errorf("error must mention reloaded; got: %v", err)
	}
}

// TestHandle_EnabledTrue_FromDisabled flips StartType
// Disabled → Automatic via Set-Service.
func TestHandle_EnabledTrue_FromDisabled(t *testing.T) {
	calls := stubExec(t, func(script string) (string, error) {
		if strings.Contains(script, "Get-Service") {
			return `{"Name":"MyService","Status":1,"StartType":4,"CanStop":true}`, nil
		}
		return "", nil
	})
	enabled := true
	step := config.Step{OsService: &config.ServiceAction{Name: "MyService", Enabled: &enabled}}
	ec := newMockExecutionContext()
	if err := Handle("MyService", step.OsService, step, ec); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !ec.CurrentResult.Changed {
		t.Errorf("expected Changed=true for startup change")
	}
	if len(*calls) != 2 || !strings.Contains((*calls)[1], "Automatic") {
		t.Errorf("expected Set-Service ... Automatic; got %v", *calls)
	}
}

// TestCapturePriorState_TagsPlatform pins the windows branch of
// runApply's reverse-capture: the snapshot must carry
// Platform="windows" so Reverse() picks the enabled-only policy.
func TestCapturePriorState_TagsPlatform(t *testing.T) {
	stubExec(t, func(string) (string, error) {
		return `{"Name":"MSSQL","Status":4,"StartType":2,"CanStop":true}`, nil
	})
	enabled := true
	step := config.Step{OsService: &config.ServiceAction{
		Name:    "MSSQL",
		State:   "started",
		Enabled: &enabled,
	}}
	info := CapturePriorState("MSSQL", step, newMockExecutionContext())
	if info == nil {
		t.Fatal("CapturePriorState returned nil")
	}
	if info.Platform != "windows" {
		t.Errorf("Platform = %q, want windows", info.Platform)
	}
	if !info.PriorActive {
		t.Errorf("PriorActive must be true (Status=4 / Running)")
	}
	if !info.PriorEnabled {
		t.Errorf("PriorEnabled must be true (StartType=2 / Automatic)")
	}
	if !info.HadStateIntent || !info.HadEnabledIntent {
		t.Errorf("intent flags should mirror the step: HadStateIntent=%v HadEnabledIntent=%v",
			info.HadStateIntent, info.HadEnabledIntent)
	}
}

// TestCapturePriorState_StartTypeMapping pins the
// StartType→PriorEnabled mapping: Automatic (2) → true; Manual (3)
// and Disabled (4) → false.
func TestCapturePriorState_StartTypeMapping(t *testing.T) {
	for _, c := range []struct {
		name      string
		startType int
		want      bool
	}{
		{"automatic", 2, true},
		{"manual", 3, false},
		{"disabled", 4, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			stubExec(t, func(string) (string, error) {
				return fmt.Sprintf(`{"Name":"S","Status":1,"StartType":%d,"CanStop":true}`, c.startType), nil
			})
			info := CapturePriorState("S", config.Step{
				OsService: &config.ServiceAction{Name: "S"},
			}, newMockExecutionContext())
			if info.PriorEnabled != c.want {
				t.Errorf("StartType=%d: PriorEnabled = %v, want %v", c.startType, info.PriorEnabled, c.want)
			}
		})
	}
}

// TestCapturePriorState_MissingServiceReturnsZeroValues pins the
// best-effort contract: probe failure / missing service yields a
// tagged info with zero values (NOT nil, NOT an error).
func TestCapturePriorState_MissingServiceReturnsZeroValues(t *testing.T) {
	stubExec(t, func(string) (string, error) {
		return "", nil
	})
	info := CapturePriorState("Missing", config.Step{
		OsService: &config.ServiceAction{Name: "Missing"},
	}, newMockExecutionContext())
	if info == nil {
		t.Fatal("CapturePriorState must never return nil")
	}
	if info.Platform != "windows" {
		t.Errorf("Platform = %q, want windows", info.Platform)
	}
	if info.PriorActive || info.PriorEnabled {
		t.Errorf("missing service: prior flags must be zero; got %+v", info)
	}
}

// TestHandle_EnabledAlreadyDisabled_Noop pins the idempotent path
// on the enabled side.
func TestHandle_EnabledAlreadyDisabled_Noop(t *testing.T) {
	calls := stubExec(t, func(script string) (string, error) {
		if strings.Contains(script, "Get-Service") {
			return `{"Name":"MyService","Status":1,"StartType":4,"CanStop":true}`, nil
		}
		t.Errorf("unexpected call: %s", script)
		return "", nil
	})
	enabled := false
	step := config.Step{OsService: &config.ServiceAction{Name: "MyService", Enabled: &enabled}}
	ec := newMockExecutionContext()
	if err := Handle("MyService", step.OsService, step, ec); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if ec.CurrentResult.Changed {
		t.Errorf("expected Changed=false; already Disabled")
	}
	if len(*calls) != 1 {
		t.Errorf("expected 1 call (Get-Service only); got %d", len(*calls))
	}
}
