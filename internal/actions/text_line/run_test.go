//nolint:revive // package name follows action convention
package text_line

import (
	"os"
	"path/filepath"
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

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{"nil", &config.Step{}, true},
		{"no path", &config.Step{TextLine: &config.TextLine{Line: "x"}}, true},
		{"present no line", &config.Step{TextLine: &config.TextLine{Path: "/tmp/x"}}, true},
		{"present with newline", &config.Step{TextLine: &config.TextLine{Path: "/tmp/x", Line: "a\nb"}}, true},
		{"bad regex", &config.Step{TextLine: &config.TextLine{Path: "/tmp/x", Line: "x", Regexp: "[unterm"}}, true},
		{"both anchors", &config.Step{TextLine: &config.TextLine{Path: "/tmp/x", Line: "x", InsertAfter: "a", InsertBefore: "b"}}, true},
		{"unknown state", &config.Step{TextLine: &config.TextLine{Path: "/tmp/x", Line: "x", State: "maybe"}}, true},
		{"absent no anchor", &config.Step{TextLine: &config.TextLine{Path: "/tmp/x", State: "absent"}}, true},
		{"ok present", &config.Step{TextLine: &config.TextLine{Path: "/tmp/x", Line: "x"}}, false},
		{"ok absent", &config.Step{TextLine: &config.TextLine{Path: "/tmp/x", Line: "x", State: "absent"}}, false},
		{"ok absent regex", &config.Step{TextLine: &config.TextLine{Path: "/tmp/x", Regexp: "^foo", State: "absent"}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := (&Handler{}).Validate(c.step)
			if (err != nil) != c.wantErr {
				t.Errorf("err=%v wantErr=%v", err, c.wantErr)
			}
		})
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustRun(t *testing.T, plan bool, step *config.Step) *executor.Result {
	t.Helper()
	res, err := (&Handler{}).Run(newCtx(t, plan), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return res.(*executor.Result)
}

func TestPresent_AppendWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	writeFile(t, path, "alpha\nbeta\n")

	r := mustRun(t, false, &config.Step{TextLine: &config.TextLine{Path: path, Line: "gamma"}})
	if !r.Changed {
		t.Error("expected Changed=true on append")
	}
	got, _ := os.ReadFile(path)
	want := "alpha\nbeta\ngamma\n"
	if string(got) != want {
		t.Errorf("file mismatch\n got %q\nwant %q", got, want)
	}
}

func TestPresent_NoopWhenExact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	writeFile(t, path, "alpha\nbeta\n")

	r := mustRun(t, false, &config.Step{TextLine: &config.TextLine{Path: path, Line: "alpha"}})
	if r.Changed {
		t.Errorf("expected no change; reason=%q", r.Reason)
	}
}

func TestPresent_RegexpReplace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sshd_config")
	writeFile(t, path, "Port 22\nPermitRootLogin yes\nUseDNS no\n")

	r := mustRun(t, false, &config.Step{TextLine: &config.TextLine{
		Path:   path,
		Line:   "PermitRootLogin no",
		Regexp: "^PermitRootLogin",
	}})
	if !r.Changed {
		t.Fatal("expected change on regex replace")
	}
	got, _ := os.ReadFile(path)
	want := "Port 22\nPermitRootLogin no\nUseDNS no\n"
	if string(got) != want {
		t.Errorf("file mismatch\n got %q\nwant %q", got, want)
	}
}

func TestPresent_IdempotentRegexpReplace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sshd_config")
	writeFile(t, path, "Port 22\nPermitRootLogin no\nUseDNS no\n")

	step := &config.Step{TextLine: &config.TextLine{
		Path:   path,
		Line:   "PermitRootLogin no",
		Regexp: "^PermitRootLogin",
	}}
	_ = mustRun(t, false, step)
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("expected idempotent second run; reason=%q", r.Reason)
	}
}

func TestPresent_InsertAfter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	writeFile(t, path, "alpha\nbeta\n")

	r := mustRun(t, false, &config.Step{TextLine: &config.TextLine{
		Path:        path,
		Line:        "between",
		InsertAfter: "^alpha$",
	}})
	if !r.Changed {
		t.Fatal("expected change on insert_after")
	}
	got, _ := os.ReadFile(path)
	want := "alpha\nbetween\nbeta\n"
	if string(got) != want {
		t.Errorf("file mismatch\n got %q\nwant %q", got, want)
	}
}

func TestPresent_InsertBefore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	writeFile(t, path, "alpha\nbeta\n")

	r := mustRun(t, false, &config.Step{TextLine: &config.TextLine{
		Path:         path,
		Line:         "between",
		InsertBefore: "^beta$",
	}})
	if !r.Changed {
		t.Fatal("expected change on insert_before")
	}
	got, _ := os.ReadFile(path)
	want := "alpha\nbetween\nbeta\n"
	if string(got) != want {
		t.Errorf("file mismatch\n got %q\nwant %q", got, want)
	}
}

func TestPresent_CreateFileWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.conf")
	r := mustRun(t, false, &config.Step{TextLine: &config.TextLine{Path: path, Line: "hello"}})
	if !r.Changed {
		t.Fatal("expected file create")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello\n" {
		t.Errorf("file mismatch: got %q", got)
	}
}

func TestAbsent_RemovesMatchingLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	writeFile(t, path, "keep\ndrop\nkeep\ndrop\n")

	r := mustRun(t, false, &config.Step{TextLine: &config.TextLine{
		Path:  path,
		Line:  "drop",
		State: "absent",
	}})
	if !r.Changed {
		t.Fatal("expected change on absent")
	}
	got, _ := os.ReadFile(path)
	want := "keep\nkeep\n"
	if string(got) != want {
		t.Errorf("file mismatch\n got %q\nwant %q", got, want)
	}
}

func TestAbsent_RegexpRemoval(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	writeFile(t, path, "keep\n# comment 1\n# comment 2\nkeep\n")

	r := mustRun(t, false, &config.Step{TextLine: &config.TextLine{
		Path:   path,
		Regexp: "^# comment",
		State:  "absent",
	}})
	if !r.Changed {
		t.Fatal("expected removals")
	}
	got, _ := os.ReadFile(path)
	want := "keep\nkeep\n"
	if string(got) != want {
		t.Errorf("file mismatch\n got %q\nwant %q", got, want)
	}
}

func TestAbsent_NoopWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	writeFile(t, path, "keep\n")
	r := mustRun(t, false, &config.Step{TextLine: &config.TextLine{
		Path:  path,
		Line:  "drop",
		State: "absent",
	}})
	if r.Changed {
		t.Errorf("expected no change; reason=%q", r.Reason)
	}
}

func TestAbsent_FileMissing(t *testing.T) {
	r := mustRun(t, false, &config.Step{TextLine: &config.TextLine{
		Path:  filepath.Join(t.TempDir(), "nope"),
		Line:  "x",
		State: "absent",
	}})
	if r.Changed {
		t.Errorf("absent on missing file should be noop; reason=%q", r.Reason)
	}
}

func TestPlan_ReportsReasonAndDoesNotWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	writeFile(t, path, "alpha\n")
	r := mustRun(t, true, &config.Step{TextLine: &config.TextLine{
		Path: path,
		Line: "beta",
	}})
	if !r.WouldChange {
		t.Errorf("plan should report WouldChange; reason=%q", r.Reason)
	}
	// File must be unchanged on plan mode.
	got, _ := os.ReadFile(path)
	if string(got) != "alpha\n" {
		t.Errorf("plan must not write; got %q", got)
	}
}

func TestPlan_AlreadyOk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	writeFile(t, path, "alpha\n")
	r := mustRun(t, true, &config.Step{TextLine: &config.TextLine{Path: path, Line: "alpha"}})
	if r.WouldChange {
		t.Errorf("plan with present line should not WouldChange; reason=%q", r.Reason)
	}
}

func TestPreservesFileWithoutTrailingNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	writeFile(t, path, "alpha\nbeta") // no trailing newline

	r := mustRun(t, false, &config.Step{TextLine: &config.TextLine{
		Path:   path,
		Line:   "BETA",
		Regexp: "^beta$",
	}})
	if !r.Changed {
		t.Fatal("expected change")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "alpha\nBETA" {
		t.Errorf("expected trailing-newline preservation; got %q", got)
	}
}

func TestBackupOnChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	writeFile(t, path, "alpha\n")
	_ = mustRun(t, false, &config.Step{TextLine: &config.TextLine{
		Path:   path,
		Line:   "beta",
		Backup: true,
	}})
	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("backup file: %v", err)
	}
	if string(bak) != "alpha\n" {
		t.Errorf("backup content wrong: %q", bak)
	}
}
