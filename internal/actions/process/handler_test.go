package process_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	_ "github.com/alehatsman/mooncake/internal/actions/process"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

func applyCtx() actions.Context {
	return actions.NewTestContext(actions.TestContextConfig{Mode: actions.ModeApply})
}

func planCtx() actions.Context {
	return actions.NewTestContext(actions.TestContextConfig{Mode: actions.ModePlan})
}

func ctxWithCancel() (actions.Context, context.CancelFunc) {
	base, cancel := context.WithCancel(context.Background())
	return actions.NewTestContext(actions.TestContextConfig{
		Mode: actions.ModeApply,
		Ctx:  base,
	}), cancel
}

func processStep(p *config.ProcessAction) *config.Step {
	return &config.Step{Process: p}
}

// ── Validate ─────────────────────────────────────────────────────────────────

func getHandler(t *testing.T) actions.Handler {
	t.Helper()
	h, ok := actions.Get("process")
	if !ok {
		t.Fatal("process handler not registered")
	}
	return h
}

func TestValidate_MissingName(t *testing.T) {
	h := getHandler(t)
	err := h.Validate(processStep(&config.ProcessAction{Command: []string{"echo"}}))
	if err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("expected name error, got %v", err)
	}
}

func TestValidate_MissingCommand(t *testing.T) {
	h := getHandler(t)
	err := h.Validate(processStep(&config.ProcessAction{Name: "foo"}))
	if err == nil || !strings.Contains(err.Error(), "command") {
		t.Fatalf("expected command error, got %v", err)
	}
}

func TestValidate_BadState(t *testing.T) {
	h := getHandler(t)
	err := h.Validate(processStep(&config.ProcessAction{Name: "foo", Command: []string{"echo"}, State: "invalid"}))
	if err == nil || !strings.Contains(err.Error(), "state") {
		t.Fatalf("expected state error, got %v", err)
	}
}

func TestValidate_BadScope(t *testing.T) {
	h := getHandler(t)
	err := h.Validate(processStep(&config.ProcessAction{Name: "foo", Command: []string{"echo"}, Scope: "galaxy"}))
	if err == nil || !strings.Contains(err.Error(), "scope") {
		t.Fatalf("expected scope error, got %v", err)
	}
}

func TestValidate_BadHealthTimeout(t *testing.T) {
	h := getHandler(t)
	err := h.Validate(processStep(&config.ProcessAction{
		Name:    "foo",
		Command: []string{"echo"},
		Health:  &config.ProcessHealth{Timeout: "not-a-duration"},
	}))
	if err == nil {
		t.Fatal("expected timeout parse error")
	}
}

// ── Plan mode ─────────────────────────────────────────────────────────────────

func TestRun_PlanMode(t *testing.T) {
	h := getHandler(t)
	r, err := h.Run(planCtx(), processStep(&config.ProcessAction{Name: "foo", Command: []string{"echo", "hi"}}))
	if err != nil {
		t.Fatal(err)
	}
	rr := r.(*executor.Result)
	if !rr.WouldChange {
		t.Error("expected WouldChange=true in plan mode")
	}
	if !strings.Contains(rr.Reason, "foo") {
		t.Errorf("expected reason to mention process name, got %q", rr.Reason)
	}
}

// ── Start + stop (scope: session) ────────────────────────────────────────────

func TestRun_StartAndStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skips process lifecycle test in -short mode")
	}
	h := getHandler(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	logOut := filepath.Join(home, "out.log")
	logErr := filepath.Join(home, "err.log")

	step := processStep(&config.ProcessAction{
		Name:    "test-sleep",
		Command: []string{"sleep", "60"},
		Scope:   "session",
		Log:     &config.ProcessLog{Stdout: logOut, Stderr: logErr},
	})

	// Start.
	r, err := h.Run(applyCtx(), step)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	rr := r.(*executor.Result)
	if !rr.Changed {
		t.Error("expected Changed=true on first start")
	}
	pid := rr.Data["pid"].(int)
	if pid <= 0 {
		t.Fatalf("bad pid %d", pid)
	}
	t.Cleanup(func() {
		if proc, err := os.FindProcess(pid); err == nil {
			_ = proc.Kill()
		}
	})

	// Second run: already running — should be no-op.
	r2, err := h.Run(applyCtx(), step)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	rr2 := r2.(*executor.Result)
	if rr2.Changed {
		t.Error("expected Changed=false when already running")
	}

	// Stop.
	stepStop := processStep(&config.ProcessAction{
		Name:    "test-sleep",
		Command: []string{"sleep", "60"},
		State:   "stopped",
	})
	r3, err := h.Run(applyCtx(), stepStop)
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	rr3 := r3.(*executor.Result)
	if !rr3.Changed {
		t.Error("expected Changed=true when stopping a running process")
	}
}

// ── Health gate: port_open ────────────────────────────────────────────────────

func TestRun_HealthGate_PortOpen(t *testing.T) {
	if testing.Short() {
		t.Skip("skips process lifecycle test in -short mode")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("cannot listen:", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	defer ln.Close()

	h := getHandler(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	step := processStep(&config.ProcessAction{
		Name:    "health-test",
		Command: []string{"sleep", "60"},
		Health: &config.ProcessHealth{
			PortOpen: port,
			Timeout:  "5s",
		},
		Log: &config.ProcessLog{
			Stdout: filepath.Join(home, "out.log"),
			Stderr: filepath.Join(home, "err.log"),
		},
	})

	r, err := h.Run(applyCtx(), step)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	rr := r.(*executor.Result)
	if pid, ok := rr.Data["pid"].(int); ok && pid > 0 {
		t.Cleanup(func() {
			if proc, err := os.FindProcess(pid); err == nil {
				_ = proc.Kill()
			}
		})
	}
}

// ── Health gate: http_ok ──────────────────────────────────────────────────────

func TestRun_HealthGate_HTTPOk(t *testing.T) {
	if testing.Short() {
		t.Skip("skips process lifecycle test in -short mode")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := getHandler(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	step := processStep(&config.ProcessAction{
		Name:    "http-health-test",
		Command: []string{"sleep", "60"},
		Health: &config.ProcessHealth{
			HTTPOk:  srv.URL,
			Timeout: "5s",
		},
		Log: &config.ProcessLog{
			Stdout: filepath.Join(home, "out.log"),
			Stderr: filepath.Join(home, "err.log"),
		},
	})

	r, err := h.Run(applyCtx(), step)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	rr := r.(*executor.Result)
	if pid, ok := rr.Data["pid"].(int); ok && pid > 0 {
		t.Cleanup(func() {
			if proc, err := os.FindProcess(pid); err == nil {
				_ = proc.Kill()
			}
		})
	}
}

// ── Health gate: timeout ──────────────────────────────────────────────────────

func TestRun_HealthGate_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skips process lifecycle test in -short mode")
	}
	port := unusedPort(t)

	h := getHandler(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	step := processStep(&config.ProcessAction{
		Name:    "health-timeout-test",
		Command: []string{"sleep", "60"},
		Health: &config.ProcessHealth{
			PortOpen: port,
			Timeout:  "500ms",
		},
		Log: &config.ProcessLog{
			Stdout: filepath.Join(home, "out.log"),
			Stderr: filepath.Join(home, "err.log"),
		},
	})

	_, err := h.Run(applyCtx(), step)
	if err == nil || !strings.Contains(err.Error(), "health gate") {
		t.Fatalf("expected health gate timeout error, got %v", err)
	}
}

// ── Absent ────────────────────────────────────────────────────────────────────

func TestRun_Absent_NoProcess(t *testing.T) {
	h := getHandler(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	step := processStep(&config.ProcessAction{
		Name:    "no-such-process",
		Command: []string{"echo"},
		State:   "absent",
	})
	r, err := h.Run(applyCtx(), step)
	if err != nil {
		t.Fatal(err)
	}
	rr := r.(*executor.Result)
	if rr.Changed {
		t.Error("expected Changed=false when nothing to remove")
	}
}

// ── Restarted ─────────────────────────────────────────────────────────────────

func TestRun_Restarted(t *testing.T) {
	if testing.Short() {
		t.Skip("skips process lifecycle test in -short mode")
	}
	h := getHandler(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	logOut := filepath.Join(home, "out.log")
	logErr := filepath.Join(home, "err.log")

	start := processStep(&config.ProcessAction{
		Name:    "restart-test",
		Command: []string{"sleep", "60"},
		Log:     &config.ProcessLog{Stdout: logOut, Stderr: logErr},
	})
	r, err := h.Run(applyCtx(), start)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	pid1 := r.(*executor.Result).Data["pid"].(int)
	t.Cleanup(func() {
		if proc, err := os.FindProcess(pid1); err == nil {
			_ = proc.Kill()
		}
	})

	restart := processStep(&config.ProcessAction{
		Name:    "restart-test",
		Command: []string{"sleep", "60"},
		State:   "restarted",
		Log:     &config.ProcessLog{Stdout: logOut, Stderr: logErr},
	})
	r2, err := h.Run(applyCtx(), restart)
	if err != nil {
		t.Fatalf("restart: %v", err)
	}
	rr2 := r2.(*executor.Result)
	if !rr2.Changed {
		t.Error("expected Changed=true on restart")
	}
	pid2 := rr2.Data["pid"].(int)
	if pid2 == pid1 {
		t.Errorf("expected new pid after restart, got same pid %d", pid1)
	}
	t.Cleanup(func() {
		if proc, err := os.FindProcess(pid2); err == nil {
			_ = proc.Kill()
		}
	})
}

// ── Scope: plan ───────────────────────────────────────────────────────────────

func TestRun_ScopePlan_KilledOnContextCancel(t *testing.T) {
	if testing.Short() {
		t.Skip("skips process lifecycle test in -short mode")
	}
	h := getHandler(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	ctx, cancel := ctxWithCancel()

	step := processStep(&config.ProcessAction{
		Name:    "plan-scope-test",
		Command: []string{"sleep", "60"},
		Scope:   "plan",
		Log: &config.ProcessLog{
			Stdout: filepath.Join(home, "out.log"),
			Stderr: filepath.Join(home, "err.log"),
		},
	})

	r, err := h.Run(ctx, step)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	pid := r.(*executor.Result).Data["pid"].(int)

	// Cancel the context (simulates plan exit).
	cancel()

	// Give the goroutine a moment to kill the process.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !isRunning(pid) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if isRunning(pid) {
		t.Error("process still running after context cancel — scope: plan cleanup failed")
		if proc, err := os.FindProcess(pid); err == nil {
			_ = proc.Kill()
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func unusedPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("cannot listen:", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func isRunning(pid int) bool {
	pidFile := fmt.Sprintf("/proc/%d", pid)
	if _, err := os.Stat(pidFile); err == nil {
		data, _ := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
		// Zombie counts as dead for our purposes.
		return !strings.Contains(string(data), "\nState:\tZ")
	}
	return false
}

func TestPidFileRoundtrip(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "test.pid")
	pid := os.Getpid()
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(pid)+"\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	got, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || got != pid {
		t.Fatalf("pid roundtrip failed: got %d, want %d", got, pid)
	}
}
