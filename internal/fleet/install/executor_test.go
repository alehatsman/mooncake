package install

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeExecutor records every Run/WriteFile/CopyLocalFile call and
// returns canned responses. Used by the Sudoer tests and (later) the
// install.Bootstrap orchestration tests. Real transport behavior is
// covered by the transport package's own integration tests.
type fakeExecutor struct {
	commands []string
	runOut   string
	runErr   string
	runCode  int
	runFail  error

	writes []writeCall
	copies []copyCall
}

type writeCall struct {
	path string
	data []byte
	mode fs.FileMode
}

type copyCall struct {
	src, dest string
	mode      fs.FileMode
}

func (f *fakeExecutor) Run(_ context.Context, cmd string) (string, string, int, error) {
	f.commands = append(f.commands, cmd)
	return f.runOut, f.runErr, f.runCode, f.runFail
}

func (f *fakeExecutor) WriteFile(_ context.Context, path string, data []byte, mode fs.FileMode) error {
	f.writes = append(f.writes, writeCall{path: path, data: append([]byte(nil), data...), mode: mode})
	return nil
}

func (f *fakeExecutor) CopyLocalFile(_ context.Context, src, dest string, mode fs.FileMode) error {
	f.copies = append(f.copies, copyCall{src: src, dest: dest, mode: mode})
	return nil
}

// TestSudoer_RootSkipsPrefix pins that an IsRoot=true Sudoer routes
// cmds through Executor.Run unchanged.
func TestSudoer_RootSkipsPrefix(t *testing.T) {
	f := &fakeExecutor{}
	s := NewSudoer(f, true, false)
	_, _, _, _ = s.Run(context.Background(), "systemctl daemon-reload")
	if len(f.commands) != 1 {
		t.Fatalf("commands = %v", f.commands)
	}
	if f.commands[0] != "systemctl daemon-reload" {
		t.Errorf("root should not wrap with sudo; got %q", f.commands[0])
	}
}

// TestSudoer_NoSudoSkipsPrefix pins that NoSudo=true (user-mode
// install) also bypasses sudo, even when IsRoot=false. This is the
// branch that makes `--user` work without ever asking for an
// escalation prompt.
func TestSudoer_NoSudoSkipsPrefix(t *testing.T) {
	f := &fakeExecutor{}
	s := NewSudoer(f, false, true)
	_, _, _, _ = s.Run(context.Background(), "systemctl --user daemon-reload")
	if len(f.commands) != 1 || f.commands[0] != "systemctl --user daemon-reload" {
		t.Fatalf("NoSudo should not wrap with sudo; got %v", f.commands)
	}
}

func TestSudoer_NonRootWrapsWithSudo(t *testing.T) {
	f := &fakeExecutor{}
	s := NewSudoer(f, false, false)
	_, _, _, _ = s.Run(context.Background(), "systemctl daemon-reload")
	if len(f.commands) != 1 {
		t.Fatalf("commands = %v", f.commands)
	}
	got := f.commands[0]
	if !strings.HasPrefix(got, "sudo -n sh -c '") {
		t.Errorf("non-root should wrap with sudo -n sh -c '...'; got %q", got)
	}
	if !strings.HasSuffix(got, "'") {
		t.Errorf("missing closing quote; got %q", got)
	}
	if !strings.Contains(got, "systemctl daemon-reload") {
		t.Errorf("inner command missing; got %q", got)
	}
}

// TestSudoer_NonRootEscapesEmbeddedQuotes pins the single-quote-escape
// contract. The orchestrator composes commands with paths that almost
// never contain quotes today, but a misbehaving command embedded with
// `'` would break sh -c parsing without escaping.
func TestSudoer_NonRootEscapesEmbeddedQuotes(t *testing.T) {
	f := &fakeExecutor{}
	s := NewSudoer(f, false, false)
	_, _, _, _ = s.Run(context.Background(), `echo "it's working"`)
	got := f.commands[0]
	if !strings.Contains(got, `'"'"'`) {
		t.Errorf("embedded ' should be escaped to '\"'\"'; got %q", got)
	}
}

// TestLocalExecutor_Run pins the exit-code propagation contract.
func TestLocalExecutor_Run(t *testing.T) {
	e := NewLocalExecutor()

	t.Run("zero exit captures stdout", func(t *testing.T) {
		out, _, code, err := e.Run(context.Background(), "echo hello")
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if code != 0 || strings.TrimSpace(out) != "hello" {
			t.Errorf("got code=%d out=%q", code, out)
		}
	})

	t.Run("non-zero exit returns code with err=nil", func(t *testing.T) {
		_, _, code, err := e.Run(context.Background(), "exit 7")
		if err != nil {
			t.Fatalf("Run should not error on non-zero exit; got %v", err)
		}
		if code != 7 {
			t.Errorf("expected code=7, got %d", code)
		}
	})

	t.Run("captures stderr", func(t *testing.T) {
		_, stderr, _, err := e.Run(context.Background(), "echo oops 1>&2")
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if strings.TrimSpace(stderr) != "oops" {
			t.Errorf("stderr = %q", stderr)
		}
	})

	t.Run("respects context cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, _, _, err := e.Run(ctx, "sleep 5")
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})
}

// TestLocalExecutor_WriteFile pins the mode-bits contract (umask
// ignored) and the simple happy-path write.
func TestLocalExecutor_WriteFile(t *testing.T) {
	e := NewLocalExecutor()
	dir := t.TempDir()
	path := filepath.Join(dir, "agentd.token")
	if err := e.WriteFile(context.Background(), path, []byte("abc\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "abc\n" {
		t.Errorf("content = %q", got)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o, want 0600 (umask is supposed to be overridden)", st.Mode().Perm())
	}
}

// TestLocalExecutor_CopyLocalFile pins the binary-copy path: mode bits
// + content equivalence + truncation of any prior content at dest.
func TestLocalExecutor_CopyLocalFile(t *testing.T) {
	e := NewLocalExecutor()
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	dest := filepath.Join(dir, "dest.bin")

	if err := os.WriteFile(src, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("seed src: %v", err)
	}
	// Pre-existing dest with longer content — verify it's truncated.
	if err := os.WriteFile(dest, []byte("XXXXXXXXXXXXXXXXXXXX"), 0o644); err != nil {
		t.Fatalf("seed dest: %v", err)
	}

	if err := e.CopyLocalFile(context.Background(), src, dest, 0o755); err != nil {
		t.Fatalf("CopyLocalFile: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(got) != "hello world" {
		t.Errorf("dest content = %q; pre-existing bytes leaked through", got)
	}
	st, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if st.Mode().Perm() != 0o755 {
		t.Errorf("mode = %o, want 0755", st.Mode().Perm())
	}
}

// TestLocalExecutor_CopyLocalFile_MissingSrc pins the error path: a
// missing source file produces a clear error and doesn't leave a
// truncated dest behind.
func TestLocalExecutor_CopyLocalFile_MissingSrc(t *testing.T) {
	e := NewLocalExecutor()
	dir := t.TempDir()
	dest := filepath.Join(dir, "dest.bin")
	err := e.CopyLocalFile(context.Background(), filepath.Join(dir, "no-such-file"), dest, 0o755)
	if err == nil {
		t.Fatal("expected error for missing source")
	}
	if _, statErr := os.Stat(dest); statErr == nil {
		t.Errorf("dest should not have been created on error: %s", dest)
	}
}
