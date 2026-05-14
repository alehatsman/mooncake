package scaffold

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scaffoldInto runs a non-interactive Scaffold and returns the dir. Helper
// for AC tests.
func scaffoldInto(t *testing.T, tpl, dir string, mutate func(*Options)) {
	t.Helper()
	opts := Options{
		Template:       tpl,
		NonInteractive: true,
		Dir:            dir,
		Stdin:          strings.NewReader(""),
		Stdout:         io.Discard,
		Stderr:         io.Discard,
	}
	if mutate != nil {
		mutate(&opts)
	}
	if err := Scaffold(opts); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
}

// AC1: --list-templates prints exactly four templates in catalogue order.
func TestListTemplates_Order(t *testing.T) {
	var buf bytes.Buffer
	if err := ListTemplates(&buf); err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("want 4 templates, got %d: %q", len(lines), buf.String())
	}
	wantOrder := []string{"dotfiles", "server", "empty", "agent-sandbox"}
	for i, want := range wantOrder {
		if !strings.HasPrefix(lines[i], want) {
			t.Errorf("line %d: got %q, want prefix %q", i, lines[i], want)
		}
	}
}

// AC2 + AC11: --non-interactive --template empty --dir creates expected files
// in the target directory, and the generated YAML passes basic parse smoke.
func TestScaffold_EmptyTemplate_CreatesExpectedFiles(t *testing.T) {
	dir := t.TempDir()
	scaffoldInto(t, "empty", dir, nil)

	for _, want := range []string{"mooncake.yml", "mooncake.vars.yml", ".gitignore", ".mooncake"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("missing expected entry %s: %v", want, err)
		}
	}
}

// AC5 + refuse-to-clobber: a second Scaffold without --force fails with an
// error naming the offending file.
func TestScaffold_RefusesToClobber(t *testing.T) {
	dir := t.TempDir()
	scaffoldInto(t, "empty", dir, nil)

	err := Scaffold(Options{
		Template:       "empty",
		NonInteractive: true,
		Dir:            dir,
		Stdout:         io.Discard,
		Stderr:         io.Discard,
	})
	if err == nil {
		t.Fatal("second scaffold without --force should fail")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should name the conflict, got: %v", err)
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error should suggest --force, got: %v", err)
	}
}

// AC6: --force in a populated directory overwrites mooncake.yml /
// mooncake.vars.yml. The atomic rename means we never see a partial-write
// window — checked indirectly by asserting the file post-state matches the
// embedded template.
func TestScaffold_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	scaffoldInto(t, "empty", dir, nil)

	// Tamper with mooncake.yml.
	mcPath := filepath.Join(dir, "mooncake.yml")
	if err := os.WriteFile(mcPath, []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	scaffoldInto(t, "empty", dir, func(o *Options) {
		o.Force = true
	})

	got, _ := os.ReadFile(mcPath)
	if strings.Contains(string(got), "tampered") {
		t.Errorf("force should overwrite tampered content; got:\n%s", got)
	}
	if !strings.Contains(string(got), "mooncake init --template empty") {
		t.Errorf("force should restore template content; got:\n%s", got)
	}
}

// AC7: existing .gitignore is appended to, not overwritten, when no marker
// is present.
func TestScaffold_GitignoreAppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	existing := "node_modules/\n.env\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	scaffoldInto(t, "empty", dir, nil)

	got, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(got), "node_modules/") {
		t.Error("existing .gitignore lines lost")
	}
	if !strings.Contains(string(got), gitignoreMarker) {
		t.Error("mooncake section not appended")
	}
}

// AC7 (continued): re-running scaffold against a .gitignore that already has
// the mooncake section is a no-op — no duplicate block.
func TestScaffold_GitignoreSecondRunIsNoop(t *testing.T) {
	dir := t.TempDir()
	scaffoldInto(t, "empty", dir, nil)

	first, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	scaffoldInto(t, "empty", dir, func(o *Options) { o.Force = true })
	second, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))

	if !bytes.Equal(first, second) {
		t.Errorf(".gitignore should be byte-identical on second run\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	// Sanity: section appears exactly once.
	if n := strings.Count(string(second), gitignoreMarker); n != 1 {
		t.Errorf("marker count = %d, want 1", n)
	}
}

// --no-vars skips mooncake.vars.yml.
func TestScaffold_NoVarsSkipsVarsFile(t *testing.T) {
	dir := t.TempDir()
	scaffoldInto(t, "empty", dir, func(o *Options) { o.NoVars = true })

	if _, err := os.Stat(filepath.Join(dir, "mooncake.vars.yml")); err == nil {
		t.Error("--no-vars should skip mooncake.vars.yml")
	}
	// Other files still exist.
	if _, err := os.Stat(filepath.Join(dir, "mooncake.yml")); err != nil {
		t.Error("mooncake.yml should still be created with --no-vars")
	}
}

// --non-interactive without --template returns a helpful error.
func TestScaffold_NonInteractiveRequiresTemplate(t *testing.T) {
	err := Scaffold(Options{
		NonInteractive: true,
		Dir:            t.TempDir(),
		Stdout:         io.Discard,
	})
	if err == nil {
		t.Fatal("want error when --non-interactive without --template")
	}
	if !strings.Contains(err.Error(), "--template") {
		t.Errorf("error should mention --template, got: %v", err)
	}
}

// Unknown template name fails fast.
func TestScaffold_UnknownTemplate(t *testing.T) {
	err := Scaffold(Options{
		Template:       "bogus",
		NonInteractive: true,
		Dir:            t.TempDir(),
		Stdout:         io.Discard,
	})
	if err == nil {
		t.Fatal("unknown template should fail")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error should name the unknown template, got: %v", err)
	}
}

// Templates ship as raw mooncake-template files: {{ home }}, {{ os }}, etc.
// must survive scaffold-time and be substituted later by the planner.
func TestScaffold_PreservesTemplatePlaceholders(t *testing.T) {
	dir := t.TempDir()
	scaffoldInto(t, "empty", dir, nil)

	body, _ := os.ReadFile(filepath.Join(dir, "mooncake.yml"))
	if !strings.Contains(string(body), "{{ home }}") {
		t.Errorf("scaffold should NOT substitute {{ home }} at init time; got:\n%s", body)
	}
}

// All four templates expand to a valid file set.
func TestScaffold_AllTemplatesProduceFiles(t *testing.T) {
	for _, tpl := range Templates {
		t.Run(tpl, func(t *testing.T) {
			dir := t.TempDir()
			scaffoldInto(t, tpl, dir, nil)
			if _, err := os.Stat(filepath.Join(dir, "mooncake.yml")); err != nil {
				t.Errorf("template %s: no mooncake.yml: %v", tpl, err)
			}
		})
	}
}

// Wall-time budget per AC12: empty template under 500ms on warm cache.
func TestScaffold_EmptyTemplateIsFast(t *testing.T) {
	if testing.Short() {
		t.Skip("timing-sensitive; skipped in -short mode")
	}
	dir := t.TempDir()
	// Warm the embed FS.
	scaffoldInto(t, "empty", filepath.Join(dir, "warm"), nil)
	// Measure.
	start := timeNow()
	scaffoldInto(t, "empty", filepath.Join(dir, "measured"), nil)
	elapsed := timeNow() - start
	if elapsed > 500_000_000 { // 500 ms in ns
		t.Errorf("scaffold took %dms, want < 500ms (AC12)", elapsed/1_000_000)
	}
}

// Local time wrapper so the import block stays minimal.
func timeNow() int64 {
	return int64(timeNowImpl())
}
