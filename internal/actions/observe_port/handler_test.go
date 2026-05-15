package observe_port

import (
	"net"
	"strconv"
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
	mode := actions.ModeApply
	if plan {
		mode = actions.ModePlan
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: r,
			PathUtil: pathutil.NewPathExpander(r),
			Logger:   logger.NewLogger(logger.ErrorLevel),
			Mode:     mode,
			Stats:    executor.NewExecutionStats(),
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: "/tmp",
	}
}

// listenAndPort opens a TCP listener on a free port and returns the
// port + a teardown.
func listenAndPort(t *testing.T) (int, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		_ = ln.Close()
		t.Fatalf("split: %v", err)
	}
	port, _ := strconv.Atoi(portStr)
	return port, func() { _ = ln.Close() }
}

func TestValidate_RequiresPort(t *testing.T) {
	h := &Handler{}
	if err := h.Validate(&config.Step{ObservePort: nil}); err == nil {
		t.Fatal("expected error for nil config")
	}
	if err := h.Validate(&config.Step{ObservePort: &config.ObservePort{Port: 0}}); err == nil {
		t.Fatal("expected error for port 0")
	}
	if err := h.Validate(&config.Step{ObservePort: &config.ObservePort{Port: 70000}}); err == nil {
		t.Fatal("expected error for port > 65535")
	}
	if err := h.Validate(&config.Step{ObservePort: &config.ObservePort{Port: 80, Protocol: "icmp"}}); err == nil {
		t.Fatal("expected error for bad protocol")
	}
	if err := h.Validate(&config.Step{ObservePort: &config.ObservePort{Port: 80, Timeout: "not-a-duration"}}); err == nil {
		t.Fatal("expected error for bad timeout")
	}
	if err := h.Validate(&config.Step{ObservePort: &config.ObservePort{Port: 80}}); err != nil {
		t.Fatalf("expected no error for valid config: %v", err)
	}
}

func TestRun_OpenPort_Found(t *testing.T) {
	port, cleanup := listenAndPort(t)
	defer cleanup()

	h := &Handler{}
	step := &config.Step{ObservePort: &config.ObservePort{
		Host: "127.0.0.1",
		Port: port,
	}}
	ctx := newCtx(t, false)
	res, err := h.Run(ctx, step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res == nil {
		t.Fatal("Run returned nil result")
	}
	er, ok := res.(*executor.Result)
	if !ok {
		t.Fatalf("expected *executor.Result; got %T", res)
	}
	data := er.Data
	if found, _ := data["found"].(bool); !found {
		t.Errorf("expected found=true for open port; got %v (data=%v)", found, data)
	}
	val, ok := data["value"].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any value (JSON-shaped); got %T", data["value"])
	}
	if open, _ := val["open"].(bool); !open {
		t.Errorf("expected value.open=true; got %v", val)
	}
	if got, _ := val["port"].(float64); int(got) != port {
		t.Errorf("expected value.port=%d; got %v", port, val["port"])
	}
}

func TestRun_ClosedPort_NotFound(t *testing.T) {
	// Get a free port and immediately close it — nobody is listening.
	port, cleanup := listenAndPort(t)
	cleanup()

	h := &Handler{}
	step := &config.Step{ObservePort: &config.ObservePort{
		Host:    "127.0.0.1",
		Port:    port,
		Timeout: "200ms",
	}}
	ctx := newCtx(t, false)
	res, err := h.Run(ctx, step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	er, ok := res.(*executor.Result)
	if !ok {
		t.Fatalf("expected *executor.Result; got %T", res)
	}
	data := er.Data
	if found, _ := data["found"].(bool); found {
		t.Errorf("expected found=false for closed port; got %v", found)
	}
	val, ok := data["value"].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any value; got %T", data["value"])
	}
	if open, _ := val["open"].(bool); open {
		t.Errorf("expected value.open=false; got %v", val)
	}
}

func TestRun_PlanMode_Defers(t *testing.T) {
	h := &Handler{}
	step := &config.Step{ObservePort: &config.ObservePort{Host: "localhost", Port: 80}}
	ctx := newCtx(t, true)
	res, err := h.Run(ctx, step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	er, ok := res.(*executor.Result)
	if !ok {
		t.Fatalf("expected *executor.Result; got %T", res)
	}
	data := er.Data
	// Plan-mode envelope: Found=false, Error includes "deferred"
	if found, _ := data["found"].(bool); found {
		t.Errorf("plan-mode Found should be false; got true")
	}
	if errStr, _ := data["error"].(string); errStr == "" {
		t.Errorf("plan-mode Error should explain the defer; got empty")
	}
	// Typed payload should still be present (zero-value PortObservation
	// rendered as map[string]any for template-engine compat)
	if _, ok := data["value"].(map[string]any); !ok {
		t.Errorf("plan-mode should still publish the typed payload as a map; got %T", data["value"])
	}
}

func TestCost_ReadOnly(t *testing.T) {
	h := &Handler{}
	cost, err := h.Cost(nil, &config.Step{})
	if err != nil {
		t.Fatalf("Cost: %v", err)
	}
	if cost.Risk != 1 {
		t.Errorf("Risk = %d, want 1 (read-only)", cost.Risk)
	}
	if !cost.Reversible {
		t.Errorf("Reversible should be true for observations")
	}
}

func TestPermissions_Network(t *testing.T) {
	h := &Handler{}
	perms := h.Permissions(&config.Step{})
	if !perms.Network {
		t.Errorf("observe.port opens a socket; Network should be true")
	}
	if perms.Sudo {
		t.Errorf("observe.port should not require Sudo")
	}
}

func TestReverse_Noop(t *testing.T) {
	h := &Handler{}
	step, err := h.Reverse(nil, &config.Step{}, nil)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if step != nil {
		t.Errorf("expected nil Step (no reverse needed); got %v", step)
	}
}

func TestDiff_Noop(t *testing.T) {
	h := &Handler{}
	step := &config.Step{ObservePort: &config.ObservePort{Host: "localhost", Port: 80}}
	d, err := h.Diff(nil, step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpNoop {
		t.Errorf("Operation = %q, want %q", d.Operation, actions.OpNoop)
	}
	if d.Resource.Identifier != "tcp:localhost:80" {
		t.Errorf("Identifier = %q, want tcp:localhost:80", d.Resource.Identifier)
	}
}
