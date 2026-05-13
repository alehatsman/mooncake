package wait_port

import (
	"net"
	"strconv"
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

func planMode(b bool) actions.Mode {
	if b {
		return actions.ModePlan
	}
	return actions.ModeApply
}

func TestRun_ImplementsRunner(t *testing.T) {
	var _ actions.Runner = &Handler{}
}

func TestValidate_RequiresPort(t *testing.T) {
	cases := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{"missing", &config.Step{}, true},
		{"zero", &config.Step{WaitPort: &config.WaitPort{Port: 0}}, true},
		{"too_big", &config.Step{WaitPort: &config.WaitPort{Port: 70000}}, true},
		{"ok", &config.Step{WaitPort: &config.WaitPort{Port: 8080}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := (&Handler{}).Validate(c.step)
			if (err != nil) != c.wantErr {
				t.Errorf("err=%v, wantErr=%v", err, c.wantErr)
			}
		})
	}
}

// TestRun_Plan surfaces the dial target without dialing.
func TestRun_Plan(t *testing.T) {
	step := &config.Step{WaitPort: &config.WaitPort{Host: "example.com", Port: 8080}}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Error("plan should report WouldChange")
	}
	if !strings.Contains(r.Reason, "example.com:8080") {
		t.Errorf("reason should include address; got %q", r.Reason)
	}
}

// TestRun_Apply_OpenPort connects immediately when a listener is up.
func TestRun_Apply_OpenPort(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lis.Close() }()
	host, portStr, _ := net.SplitHostPort(lis.Addr().String())
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	step := &config.Step{WaitPort: &config.WaitPort{
		Host:    host,
		Port:    port,
		Timeout: "2s",
	}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Data["success"] != true {
		t.Errorf("expected success=true; got data=%v", r.Data)
	}
}

// TestRun_Apply_Timeout fails fast when nothing is listening.
func TestRun_Apply_Timeout(t *testing.T) {
	// Reserve a port then close so it's free → connection refused or timeout.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := lis.Addr().String()
	_ = lis.Close()
	host, portStr, _ := net.SplitHostPort(addr)
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	step := &config.Step{WaitPort: &config.WaitPort{
		Host:         host,
		Port:         port,
		Timeout:      "400ms",
		PollInterval: "100ms",
	}}
	_, runErr := (&Handler{}).Run(newCtx(t, false), step)
	if runErr == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(runErr.Error(), "timeout") {
		t.Errorf("expected timeout in error; got %v", runErr)
	}
}
