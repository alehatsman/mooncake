package install

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scriptedExecutor returns canned (stdout, stderr, code) tuples
// per-command-regex, in order. Useful for asserting that Bootstrap
// issues commands in the right sequence and short-circuits where it
// should. When no entry matches, returns ("", "", 0, nil) — i.e. a
// silent success, so unspecified probes don't fail the test.
type scriptedExecutor struct {
	t        *testing.T
	commands []string // every cmd that came in, in order
	writes   []writeCall
	copies   []copyCall

	// canned[i] applies to the i-th Run call. Falls through to ("",
	// "", 0, nil) when index >= len(canned).
	canned []runReply
}

type runReply struct {
	stdout string
	stderr string
	code   int
	err    error
}

func (s *scriptedExecutor) Run(_ context.Context, cmd string) (string, string, int, error) {
	idx := len(s.commands)
	s.commands = append(s.commands, cmd)
	if idx < len(s.canned) {
		r := s.canned[idx]
		return r.stdout, r.stderr, r.code, r.err
	}
	return "", "", 0, nil
}

func (s *scriptedExecutor) WriteFile(_ context.Context, path string, data []byte, mode fs.FileMode) error {
	s.writes = append(s.writes, writeCall{path: path, data: append([]byte(nil), data...), mode: mode})
	return nil
}

func (s *scriptedExecutor) CopyLocalFile(_ context.Context, src, dest string, mode fs.FileMode) error {
	s.copies = append(s.copies, copyCall{src: src, dest: dest, mode: mode})
	return nil
}

func tempBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "mooncake")
	if err := os.WriteFile(p, []byte("#!/bin/sh\necho mooncake 0.9.0\n"), 0o755); err != nil {
		t.Fatalf("seed binary: %v", err)
	}
	return p
}

// TestBootstrap_IdempotentShortCircuit pins the same-version + active
// short-circuit. Steps 4-6 must not run; the token is read from the
// canonical path; AlreadyOK=true is set.
func TestBootstrap_IdempotentShortCircuit(t *testing.T) {
	bin := tempBinary(t)
	// Sequence: id -u (root), --version probe, IsActiveCmd, cat token.
	exec := &scriptedExecutor{
		t: t,
		canned: []runReply{
			{stdout: "0\n"},              // 1. id -u
			{stdout: "mooncake 0.9.0\n"}, // 2. --version
			{stdout: "active\n"},         // 3. IsActiveCmd
			{stdout: "secret-token\n"},   // 4. cat token (sudoer.Run via root: no wrap)
		},
	}
	res, err := Bootstrap(context.Background(), exec, BootstrapOptions{
		OS:                "linux",
		Arch:              "amd64",
		Port:              7878,
		LocalBinary:       bin,
		ControllerVersion: "0.9.0",
		ReachableHost:     "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if !res.AlreadyOK {
		t.Fatalf("expected AlreadyOK=true (same version + active)")
	}
	if res.Token != "secret-token" {
		t.Errorf("Token = %q", res.Token)
	}
	// No binary stage, no unit write — short-circuit fired.
	if len(exec.copies) != 0 {
		t.Errorf("CopyLocalFile should not have been called on short-circuit; got %v", exec.copies)
	}
	if len(exec.writes) != 0 {
		t.Errorf("WriteFile should not have been called on short-circuit; got %v", exec.writes)
	}
}

// TestBootstrap_VersionMismatchRefuses pins the upgrade gate: different
// version installed + Upgrade=false errors out before staging anything.
func TestBootstrap_VersionMismatchRefuses(t *testing.T) {
	bin := tempBinary(t)
	exec := &scriptedExecutor{
		t: t,
		canned: []runReply{
			{stdout: "0\n"},              // id -u
			{stdout: "mooncake 0.8.0\n"}, // older version installed
			{stdout: "active\n"},         // IsActiveCmd
		},
	}
	_, err := Bootstrap(context.Background(), exec, BootstrapOptions{
		OS:                "linux",
		Port:              7878,
		LocalBinary:       bin,
		ControllerVersion: "0.9.0",
		Upgrade:           false,
		ReachableHost:     "127.0.0.1",
	})
	if err == nil {
		t.Fatal("expected error for version mismatch without --upgrade")
	}
	if !strings.Contains(err.Error(), "--upgrade") {
		t.Errorf("error should mention --upgrade flag; got %v", err)
	}
	if len(exec.copies) != 0 {
		t.Errorf("binary should not have been staged on refusal; got %v", exec.copies)
	}
}

// TestBootstrap_RejectsBadOS pins the OS allowlist — Windows goes
// through bootstrapWindows in fleet/, never through install.Bootstrap.
func TestBootstrap_RejectsBadOS(t *testing.T) {
	bin := tempBinary(t)
	exec := &scriptedExecutor{t: t}
	_, err := Bootstrap(context.Background(), exec, BootstrapOptions{
		OS:            "windows",
		LocalBinary:   bin,
		ReachableHost: "127.0.0.1",
	})
	if err == nil {
		t.Fatal("expected error for unsupported os")
	}
	if !strings.Contains(err.Error(), "unsupported os") {
		t.Errorf("error should mention unsupported os; got %v", err)
	}
}

// TestBootstrap_WriterReceivesProgress pins that progress lines flow
// through opts.Writer with the LogPrefix in `[prefix] ` form. Drives
// the controller-side multiplex output shape (cmd/fleet.go prints these
// directly to the user during fleet bootstrap).
func TestBootstrap_WriterReceivesProgress(t *testing.T) {
	bin := tempBinary(t)
	var buf bytes.Buffer
	exec := &scriptedExecutor{
		t: t,
		canned: []runReply{
			{stdout: "0\n"},              // id -u
			{stdout: "mooncake 0.9.0\n"}, // --version (same)
			{stdout: "active\n"},         // IsActiveCmd
			{stdout: "tok\n"},            // cat token
		},
	}
	_, err := Bootstrap(context.Background(), exec, BootstrapOptions{
		OS:                "linux",
		Arch:              "amd64",
		Port:              7878,
		LocalBinary:       bin,
		ControllerVersion: "0.9.0",
		ReachableHost:     "127.0.0.1",
		Writer:            &buf,
		LogPrefix:         "main_pc",
	})
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "[main_pc]") {
		t.Errorf("output missing log prefix `[main_pc]`:\n%s", got)
	}
	if !strings.Contains(got, "existing install: version 0.9.0") {
		t.Errorf("output missing existing-install line:\n%s", got)
	}
}

// TestDetectIsRoot pins the uid 0 contract.
func TestDetectIsRoot(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want bool
	}{
		{"root", "0\n", true},
		{"normal user", "1000\n", false},
		{"trailing whitespace stripped", "  0  \n", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			exec := &scriptedExecutor{t: t, canned: []runReply{{stdout: c.out}}}
			got, err := DetectIsRoot(context.Background(), exec)
			if err != nil {
				t.Fatalf("DetectIsRoot: %v", err)
			}
			if got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
			if exec.commands[0] != "id -u" {
				t.Errorf("expected `id -u`, got %q", exec.commands[0])
			}
		})
	}
}

// TestPlaceBinary_TwoStageWithSudo pins the contract: CopyLocalFile
// goes to /tmp/mooncake.<rand>, the move command uses mkdir -p +
// mv -f, and a non-root sudoer wraps the move in `sudo -n sh -c`.
func TestPlaceBinary_TwoStageWithSudo(t *testing.T) {
	bin := tempBinary(t)
	exec := &scriptedExecutor{t: t}
	sudoer := NewSudoer(exec, false, false) // non-root, sudo required
	inst := Installer{OS: "linux", Port: 7878}

	if err := inst.PlaceBinary(context.Background(), exec, sudoer, bin); err != nil {
		t.Fatalf("PlaceBinary: %v", err)
	}
	if len(exec.copies) != 1 {
		t.Fatalf("CopyLocalFile calls = %d, want 1", len(exec.copies))
	}
	if exec.copies[0].dest == "" || !strings.HasPrefix(exec.copies[0].dest, "/tmp/mooncake.") {
		t.Errorf("stage path should be /tmp/mooncake.<rand>; got %q", exec.copies[0].dest)
	}
	if exec.copies[0].mode != 0o755 {
		t.Errorf("stage mode = %o, want 0755", exec.copies[0].mode)
	}
	if len(exec.commands) != 1 {
		t.Fatalf("commands = %d, want 1 (the mv)", len(exec.commands))
	}
	gotCmd := exec.commands[0]
	if !strings.HasPrefix(gotCmd, "sudo -n sh -c '") {
		t.Errorf("non-root mv should be sudo-wrapped; got %q", gotCmd)
	}
	if !strings.Contains(gotCmd, "mkdir -p /usr/local/bin") {
		t.Errorf("mv command should mkdir -p the target dir; got %q", gotCmd)
	}
	if !strings.Contains(gotCmd, "mv -f") {
		t.Errorf("mv command should use -f; got %q", gotCmd)
	}
	if !strings.Contains(gotCmd, "/usr/local/bin/mooncake") {
		t.Errorf("mv target should be /usr/local/bin/mooncake; got %q", gotCmd)
	}
}

// TestPlaceBinary_UserModeNoSudo pins that AsUser + NoSudo skips the
// sudo wrap and targets ~/.local/bin.
func TestPlaceBinary_UserModeNoSudo(t *testing.T) {
	bin := tempBinary(t)
	exec := &scriptedExecutor{t: t}
	sudoer := NewSudoer(exec, false, true) // NoSudo (user mode)
	inst := Installer{OS: "linux", Port: 7878, AsUser: true}

	if err := inst.PlaceBinary(context.Background(), exec, sudoer, bin); err != nil {
		t.Fatalf("PlaceBinary: %v", err)
	}
	if strings.Contains(exec.commands[0], "sudo -n") {
		t.Errorf("user mode should not sudo-wrap; got %q", exec.commands[0])
	}
	if !strings.Contains(exec.commands[0], "~/.local/bin/mooncake") {
		t.Errorf("user mode should target ~/.local/bin; got %q", exec.commands[0])
	}
}

// TestPlaceUnit_RendersAndStages pins the unit-install flow: render +
// stage to /tmp + sudo-mv into UnitPath. The rendered body should
// have {{PORT}} substituted with the Installer's port.
func TestPlaceUnit_RendersAndStages(t *testing.T) {
	exec := &scriptedExecutor{t: t}
	sudoer := NewSudoer(exec, true, false) // root, no wrap
	inst := Installer{OS: "linux", Port: 7878}

	if err := inst.PlaceUnit(context.Background(), exec, sudoer); err != nil {
		t.Fatalf("PlaceUnit: %v", err)
	}
	if len(exec.writes) != 1 {
		t.Fatalf("WriteFile calls = %d, want 1", len(exec.writes))
	}
	if !bytes.Contains(exec.writes[0].data, []byte("0.0.0.0:7878")) {
		t.Errorf("rendered unit missing port substitution:\n%s", exec.writes[0].data)
	}
	if exec.writes[0].mode != 0o644 {
		t.Errorf("unit mode = %o, want 0644", exec.writes[0].mode)
	}
	if !strings.Contains(exec.commands[0], fmt.Sprintf("mv -f %s /etc/systemd/system/mooncake-agentd.service", exec.writes[0].path)) {
		t.Errorf("mv command shape wrong; got %q", exec.commands[0])
	}
}

// TestReadToken_TrimsAndErrorsOnEmpty pins the contract: leading/
// trailing whitespace stripped; empty file (which the daemon writes
// briefly during startup) becomes a clear error rather than a silent
// "" token.
func TestReadToken_TrimsAndErrorsOnEmpty(t *testing.T) {
	t.Run("happy", func(t *testing.T) {
		exec := &scriptedExecutor{t: t, canned: []runReply{{stdout: "  abc-def-token  \n"}}}
		inst := Installer{OS: "linux"}
		got, err := inst.ReadToken(context.Background(), NewSudoer(exec, true, false))
		if err != nil {
			t.Fatalf("ReadToken: %v", err)
		}
		if got != "abc-def-token" {
			t.Errorf("token = %q", got)
		}
	})
	t.Run("empty errors", func(t *testing.T) {
		exec := &scriptedExecutor{t: t, canned: []runReply{{stdout: "\n"}}}
		inst := Installer{OS: "linux"}
		_, err := inst.ReadToken(context.Background(), NewSudoer(exec, true, false))
		if err == nil {
			t.Fatal("empty token file should error")
		}
	})
}
