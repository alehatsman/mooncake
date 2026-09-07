package mod

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/modules"
)

// gitFreshEnv strips GIT_* so `git init` in a temp dir cannot be redirected at
// the parent repo when tests run from a git hook.
func gitFreshEnv() []string {
	in := os.Environ()
	out := make([]string, 0, len(in))
	for _, e := range in {
		if strings.HasPrefix(e, "GIT_") {
			continue
		}
		out = append(out, e)
	}
	return out
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-c", "core.hooksPath=/dev/null"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Env = gitFreshEnv()
	if dir != "" {
		cmd.Dir = dir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// makeFixtureRepo builds a tagged repo exporting one component and returns the
// bare clone source. body lets a test vary the component so two fixtures hash
// differently.
func makeFixtureRepo(t *testing.T, tag, body string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	work := filepath.Join(dir, "work")
	if err := os.MkdirAll(filepath.Join(work, "components"), 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, work, "init", "-q")
	runGit(t, work, "config", "user.email", "test@example.com")
	runGit(t, work, "config", "user.name", "Test")
	write(t, filepath.Join(work, "index.yml"),
		"name: testmod\nexports:\n  default: components/install.yml\n")
	write(t, filepath.Join(work, "components", "install.yml"), body)
	runGit(t, work, "add", ".")
	runGit(t, work, "commit", "-q", "-m", "init")
	runGit(t, work, "tag", tag)

	bare := filepath.Join(dir, "bare.git")
	runGit(t, "", "clone", "--bare", "-q", work, bare)
	return bare
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// modEnv points the mod subcommands at a fixture repo and a scratch cache for
// the duration of a test.
func modEnv(t *testing.T, bare string) {
	t.Helper()
	cacheRoot := t.TempDir()
	prev := newCLIFetcher
	newCLIFetcher = func() *modules.Fetcher {
		return &modules.Fetcher{
			Root:     cacheRoot,
			CloneURL: func(_ modules.Reference) string { return "file://" + bare },
		}
	}
	t.Cleanup(func() { newCLIFetcher = prev })
}

// writePlaybook drops a mooncake.yml declaring the given alias -> source pairs
// and returns its path.
func writePlaybook(t *testing.T, mods map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString("modules:\n")
	for alias, src := range mods {
		b.WriteString("  " + alias + ": " + src + "\n")
	}
	b.WriteString("steps:\n  - log: hi\n")
	path := filepath.Join(dir, "mooncake.yml")
	write(t, path, b.String())
	return path
}

// runMod invokes one `mooncake mod <sub>` through the real CLI wiring so flag
// parsing and the Action signature are exercised, not bypassed.
func runMod(t *testing.T, sub string, args ...string) error {
	t.Helper()
	app := &cli.App{
		Writer:         os.Stderr,
		Commands:       []*cli.Command{Command()},
		ExitErrHandler: func(*cli.Context, error) {}, // don't os.Exit on cli.Exit
	}
	return app.Run(append([]string{"mooncake", "mod", sub}, args...))
}

func TestModTidy_PinsAndIsIdempotent(t *testing.T) {
	bare := makeFixtureRepo(t, "v1.0.0", "name: install\nsteps:\n  - log: one\n")
	modEnv(t, bare)
	playbook := writePlaybook(t, map[string]string{
		"testmod": "github.com/owner/testmod@v1.0.0",
	})
	lockPath := lockPathFor(playbook)

	if err := runMod(t, "tidy", "--playbook", playbook); err != nil {
		t.Fatalf("mod tidy: %v", err)
	}
	first, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("lockfile not written: %v", err)
	}
	if !strings.Contains(string(first), "github.com/owner/testmod@v1.0.0") {
		t.Errorf("lockfile missing the ref:\n%s", first)
	}
	if !strings.Contains(string(first), modules.HashPrefix) {
		t.Errorf("lockfile missing an %s hash:\n%s", modules.HashPrefix, first)
	}

	// A second tidy with nothing changed must produce no diff.
	if err := runMod(t, "tidy", "--playbook", playbook); err != nil {
		t.Fatalf("second mod tidy: %v", err)
	}
	second, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("tidy is not idempotent:\n--- first ---\n%s\n--- second ---\n%s",
			first, second)
	}
}

func TestModTidy_DropsUndeclaredEntries(t *testing.T) {
	bare := makeFixtureRepo(t, "v1.0.0", "name: install\nsteps:\n  - log: one\n")
	modEnv(t, bare)
	playbook := writePlaybook(t, map[string]string{
		"testmod": "github.com/owner/testmod@v1.0.0",
	})
	lockPath := lockPathFor(playbook)

	// Seed a stale entry that the playbook does not declare.
	stale := &modules.Lock{}
	stale.Set(modules.LockEntry{Ref: "gone.example/o/r@v9", Hash: "h1:stale"})
	if err := stale.SaveLock(lockPath); err != nil {
		t.Fatal(err)
	}

	if err := runMod(t, "tidy", "--playbook", playbook); err != nil {
		t.Fatalf("mod tidy: %v", err)
	}
	out, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "gone.example/o/r@v9") {
		t.Errorf("tidy kept an undeclared entry:\n%s", out)
	}
}

func TestModVerify_OKThenMismatch(t *testing.T) {
	bare := makeFixtureRepo(t, "v1.0.0", "name: install\nsteps:\n  - log: one\n")
	modEnv(t, bare)
	playbook := writePlaybook(t, map[string]string{
		"testmod": "github.com/owner/testmod@v1.0.0",
	})

	if err := runMod(t, "tidy", "--playbook", playbook); err != nil {
		t.Fatalf("mod tidy: %v", err)
	}
	if err := runMod(t, "verify", "--playbook", playbook); err != nil {
		t.Fatalf("verify right after tidy should pass: %v", err)
	}

	// Corrupt the pin: verify must fail non-zero.
	lockPath := lockPathFor(playbook)
	lock, err := modules.LoadLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	lock.Set(modules.LockEntry{Ref: "github.com/owner/testmod@v1.0.0", Hash: "h1:tampered"})
	if err := lock.SaveLock(lockPath); err != nil {
		t.Fatal(err)
	}

	err = runMod(t, "verify", "--playbook", playbook)
	if err == nil {
		t.Fatal("verify passed against a tampered pin")
	}
	var exit cli.ExitCoder
	if !errors.As(err, &exit) || exit.ExitCode() == 0 {
		t.Errorf("verify should exit non-zero, got %#v", err)
	}
}

func TestModVerify_RequiresALockfile(t *testing.T) {
	playbook := writePlaybook(t, map[string]string{})
	err := runMod(t, "verify", "--playbook", playbook)
	if err == nil {
		t.Fatal("verify without a lockfile returned no error")
	}
	if !strings.Contains(err.Error(), "mod tidy") {
		t.Errorf("error should point at `mooncake mod tidy`, got: %v", err)
	}
}

func TestModList_JSONReportsCacheAndLockState(t *testing.T) {
	bare := makeFixtureRepo(t, "v1.0.0", "name: install\nsteps:\n  - log: one\n")
	modEnv(t, bare)
	playbook := writePlaybook(t, map[string]string{
		"testmod": "github.com/owner/testmod@v1.0.0",
	})

	// Before tidy: neither cached nor locked.
	out := captureStdout(t, func() {
		if err := runMod(t, "list", "--playbook", playbook, "--format", "json"); err != nil {
			t.Fatalf("mod list: %v", err)
		}
	})
	var before listResult
	if err := json.Unmarshal([]byte(out), &before); err != nil {
		t.Fatalf("mod list --format json emitted invalid JSON: %v\n%s", err, out)
	}
	if len(before.Modules) != 1 {
		t.Fatalf("want 1 module, got %d", len(before.Modules))
	}
	if before.Modules[0].Cached || before.Modules[0].Locked {
		t.Errorf("before tidy: cached=%v locked=%v, want both false",
			before.Modules[0].Cached, before.Modules[0].Locked)
	}
	if before.Modules[0].Ref != "github.com/owner/testmod@v1.0.0" {
		t.Errorf("ref = %q", before.Modules[0].Ref)
	}

	if err := runMod(t, "tidy", "--playbook", playbook); err != nil {
		t.Fatalf("mod tidy: %v", err)
	}

	out = captureStdout(t, func() {
		if err := runMod(t, "list", "--playbook", playbook, "--format", "json"); err != nil {
			t.Fatalf("mod list: %v", err)
		}
	})
	var after listResult
	if err := json.Unmarshal([]byte(out), &after); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if !after.Modules[0].Cached || !after.Modules[0].Locked {
		t.Errorf("after tidy: cached=%v locked=%v, want both true",
			after.Modules[0].Cached, after.Modules[0].Locked)
	}
}

// captureStdout swaps os.Stdout for a pipe around fn. The mod commands print
// with fmt.Printf, so this is the only way to assert on their output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prev := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var b strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				b.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		done <- b.String()
	}()
	fn()
	os.Stdout = prev
	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
}
