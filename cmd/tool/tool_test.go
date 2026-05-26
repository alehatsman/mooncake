package tool

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/lockfile"
	"github.com/urfave/cli/v2"
)

// seedToolEnv writes a mooncake.lock + a populated install dir for the
// given (name, version) under an XDG-isolated tmp tree, chdirs into the
// lockfile dir, and returns paths the caller can assert on.
type toolEnv struct {
	cwd        string
	storeRoot  string
	installDir string
}

func seedToolEnv(t *testing.T, name, version, bin string) toolEnv {
	t.Helper()

	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)
	storeRoot := filepath.Join(xdg, "mooncake", "tools")

	cwd := t.TempDir()
	chdir(t, cwd)

	// Populate install dir with the bin so Locate returns the right path.
	installDir := filepath.Join(storeRoot, name, version)
	binAbs := filepath.Join(installDir, bin)
	if err := os.MkdirAll(filepath.Dir(binAbs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(binAbs, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	// Seed lockfile.
	lock := &lockfile.Lock{}
	lock.Set(lockfile.Entry{
		Backend:      "archive-url",
		Name:         name,
		Version:      version,
		ResolvedURL:  "https://example.com/" + name + ".tar.gz",
		SHA256:       "sha256:deadbeef",
		Bin:          bin,
		LockedAt:     "2026-05-12T19:14:02Z",
		LockedByArch: "linux-amd64",
	})
	if err := lock.Save(filepath.Join(cwd, lockfile.Filename)); err != nil {
		t.Fatalf("save lockfile: %v", err)
	}

	return toolEnv{cwd: cwd, storeRoot: storeRoot, installDir: installDir}
}

// chdir wraps os.Chdir with t.Cleanup to restore the original CWD.
func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

// captureStdout runs fn, capturing what it writes to os.Stdout, and
// returns the captured bytes.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	errCh := make(chan error, 1)
	go func() {
		errCh <- fn()
		_ = w.Close()
	}()

	out, _ := io.ReadAll(r)
	return string(out), <-errCh
}

// newCLICtx builds a cli.Context for an action invocation with args and flags.
func newCLICtx(t *testing.T, cmd *cli.Command, args []string, flags map[string]string) *cli.Context {
	t.Helper()
	set := flag.NewFlagSet(cmd.Name, flag.ContinueOnError)
	for _, f := range cmd.Flags {
		if sf, ok := f.(*cli.StringFlag); ok {
			set.String(sf.Name, sf.Value, sf.Usage)
		}
	}
	for k, v := range flags {
		_ = set.Set(k, v)
	}
	if err := set.Parse(args); err != nil {
		t.Fatalf("flag parse: %v", err)
	}
	return cli.NewContext(nil, set, nil)
}

// ----------------------------------------------------------------------

func TestToolWhich_Installed(t *testing.T) {
	env := seedToolEnv(t, "demo", "1.0.0", "bin/demo")

	out, err := captureStdout(t, func() error {
		return toolWhichAction(newCLICtx(t, toolWhichCommand(), []string{"demo"}, nil))
	})
	if err != nil {
		t.Fatalf("which: %v", err)
	}
	want := filepath.Join(env.installDir, "bin", "demo")
	if strings.TrimSpace(out) != want {
		t.Errorf("which output:\n  got  %q\n  want %q", strings.TrimSpace(out), want)
	}
}

func TestToolWhich_UnknownTool(t *testing.T) {
	seedToolEnv(t, "demo", "1.0.0", "bin/demo")

	_, err := captureStdout(t, func() error {
		return toolWhichAction(newCLICtx(t, toolWhichCommand(), []string{"unknown-tool"}, nil))
	})
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if !strings.Contains(err.Error(), "not declared") {
		t.Errorf("error should mention 'not declared': %v", err)
	}
}

func TestToolWhich_MissingArg(t *testing.T) {
	seedToolEnv(t, "demo", "1.0.0", "bin/demo")

	_, err := captureStdout(t, func() error {
		return toolWhichAction(newCLICtx(t, toolWhichCommand(), nil, nil))
	})
	if err == nil {
		t.Fatal("expected usage error")
	}
}

func TestToolWhich_NoLockfile(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)
	chdir(t, t.TempDir()) // no lockfile

	_, err := captureStdout(t, func() error {
		return toolWhichAction(newCLICtx(t, toolWhichCommand(), []string{"demo"}, nil))
	})
	if err == nil {
		t.Fatal("expected error when no lockfile present")
	}
	if !strings.Contains(err.Error(), "mooncake.lock") {
		t.Errorf("error should mention mooncake.lock: %v", err)
	}
}

func TestToolList_FormatAndStatus(t *testing.T) {
	seedToolEnv(t, "demo", "1.0.0", "bin/demo")

	out, err := captureStdout(t, func() error {
		return toolListAction(newCLICtx(t, toolListCommand(), nil, nil))
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out, "demo") || !strings.Contains(out, "1.0.0") {
		t.Errorf("list output missing tool details: %q", out)
	}
	if !strings.Contains(out, "archive-url") {
		t.Errorf("list output missing backend: %q", out)
	}
	if !strings.Contains(out, "installed") {
		t.Errorf("list output should report installed: %q", out)
	}
}

func TestToolEnv_ZshShellExportLines(t *testing.T) {
	env := seedToolEnv(t, "demo", "1.0.0", "bin/demo")

	out, err := captureStdout(t, func() error {
		return toolEnvAction(newCLICtx(t, toolEnvCommand(), nil, map[string]string{"shell": "zsh"}))
	})
	if err != nil {
		t.Fatalf("env: %v", err)
	}
	binDir := filepath.Join(env.installDir, "bin")
	wantLine := `export PATH="` + binDir + `:$PATH"`
	if !strings.Contains(out, wantLine) {
		t.Errorf("env output missing %q in:\n%s", wantLine, out)
	}
}

func TestToolEnv_FishShellExportLines(t *testing.T) {
	env := seedToolEnv(t, "demo", "1.0.0", "bin/demo")

	out, err := captureStdout(t, func() error {
		return toolEnvAction(newCLICtx(t, toolEnvCommand(), nil, map[string]string{"shell": "fish"}))
	})
	if err != nil {
		t.Fatalf("env: %v", err)
	}
	binDir := filepath.Join(env.installDir, "bin")
	if !strings.Contains(out, "set -gx PATH "+binDir+" $PATH") {
		t.Errorf("fish output missing set -gx PATH line:\n%s", out)
	}
}

func TestToolEnv_UnsupportedShell(t *testing.T) {
	seedToolEnv(t, "demo", "1.0.0", "bin/demo")

	_, err := captureStdout(t, func() error {
		return toolEnvAction(newCLICtx(t, toolEnvCommand(), nil, map[string]string{"shell": "powershell"}))
	})
	if err == nil {
		t.Fatal("expected unsupported-shell error")
	}
}

func TestToolEnv_MiseBackendEmitsHint(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)
	cwd := t.TempDir()
	chdir(t, cwd)

	// Seed lockfile with a mise entry.
	lock := &lockfile.Lock{}
	lock.Set(lockfile.Entry{
		Backend:  "mise",
		Name:     "node",
		Version:  "24.0.0",
		LockedAt: "2026-05-12T19:14:02Z",
	})
	if err := lock.Save(filepath.Join(cwd, lockfile.Filename)); err != nil {
		t.Fatalf("save lockfile: %v", err)
	}

	out, err := captureStdout(t, func() error {
		return toolEnvAction(newCLICtx(t, toolEnvCommand(), nil, map[string]string{"shell": "zsh"}))
	})
	if err != nil {
		t.Fatalf("env: %v", err)
	}
	// Mise-backed entries get a comment hint, not an export line.
	if !strings.Contains(out, "mise") {
		t.Errorf("expected mise hint in output, got:\n%s", out)
	}
	if strings.Contains(out, "export PATH") {
		t.Errorf("mise-backed entry should not emit export PATH, got:\n%s", out)
	}
}
