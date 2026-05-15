package main

import (
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestIssue87_SIGINTExitsCleanly verifies that `mooncake apply` actually
// exits when the user hits Ctrl-C (or any caller sends SIGINT). Pre-fix
// the apply process stayed alive after the shell child died and
// required `kill -KILL`. Test wires this by building the binary,
// starting an apply with a long-running shell step, sending SIGINT,
// and asserting the process exits with code 130.
func TestIssue87_SIGINTExitsCleanly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal handling differs on Windows")
	}
	if testing.Short() {
		t.Skip("builds + execs the CLI; not short-mode friendly")
	}

	// Build a one-off binary. The test runs from cmd/, so build .
	// to produce a binary against the same package.
	bin := t.TempDir() + "/mooncake-test"
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	cfg := t.TempDir() + "/slow.yml"
	if err := writeFile(cfg, "- shell: sleep 30\n"); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "apply", "-c", cfg, "--output-format", "json")
	// New process group so we can send SIGINT to the leader without
	// touching the test runner.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Give the apply a beat to start its shell child.
	time.Sleep(800 * time.Millisecond)

	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("send SIGINT: %v", err)
	}

	// 3 seconds is plenty — the handler should exit immediately. If
	// we hit this timeout the bug is back.
	doneCh := make(chan error, 1)
	go func() { doneCh <- cmd.Wait() }()
	select {
	case err := <-doneCh:
		if err == nil {
			t.Fatal("apply exited 0 on SIGINT; expected non-zero")
		}
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("expected *exec.ExitError, got %T: %v", err, err)
		}
		if got := exitErr.ExitCode(); got != 130 {
			t.Errorf("exit code = %d, want 130 (SIGINT)", got)
		}
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("apply did not exit within 3s of SIGINT — issue-87 regression")
	}
}

// TestIssue87_SIGTERMExitsCleanly is the systemd-style counterpart —
// SIGTERM should produce exit code 143.
func TestIssue87_SIGTERMExitsCleanly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal handling differs on Windows")
	}
	if testing.Short() {
		t.Skip("builds + execs the CLI")
	}

	bin := t.TempDir() + "/mooncake-test"
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	cfg := t.TempDir() + "/slow.yml"
	if err := writeFile(cfg, "- shell: sleep 30\n"); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "apply", "-c", cfg, "--output-format", "json")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	time.Sleep(800 * time.Millisecond)
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}

	doneCh := make(chan error, 1)
	go func() { doneCh <- cmd.Wait() }()
	select {
	case err := <-doneCh:
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("expected *exec.ExitError, got %T: %v", err, err)
		}
		if got := exitErr.ExitCode(); got != 143 {
			t.Errorf("exit code = %d, want 143 (SIGTERM)", got)
		}
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("apply did not exit within 3s of SIGTERM")
	}
}

// writeFile writes content to path, creating parents as needed. Tiny
// helper to keep the table-style tests above readable.
func writeFile(path, content string) error {
	return runSh("mkdir -p $(dirname '" + path + "') && cat > '" + path + "' <<'EOF'\n" + content + "EOF")
}

func runSh(script string) error {
	c := exec.Command("sh", "-c", script)
	out, err := c.CombinedOutput()
	if err != nil {
		return &shErr{script: script, output: string(out), err: err}
	}
	return nil
}

type shErr struct {
	script string
	output string
	err    error
}

func (e *shErr) Error() string {
	return "sh: " + strings.TrimSpace(e.output) + ": " + e.err.Error()
}
