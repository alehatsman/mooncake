//nolint:revive // package name follows action convention
package text_patch_ini

import (
	"os"
	"path/filepath"
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

// ---- interface + validate ----

func TestRun_ImplementsRunner(t *testing.T) {
	var _ actions.Runner = &Handler{}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{"nil step", &config.Step{}, true},
		{"no path", &config.Step{TextPatchINI: &config.TextPatchINI{Set: map[string]string{"a.b": "c"}}}, true},
		{
			"empty set and delete",
			&config.Step{TextPatchINI: &config.TextPatchINI{Path: "/tmp/x"}},
			true,
		},
		{
			"conflict same key in set and delete",
			&config.Step{TextPatchINI: &config.TextPatchINI{
				Path:   "/tmp/x",
				Set:    map[string]string{"S.k": "v"},
				Delete: []string{"S.k"},
			}},
			true,
		},
		{
			"empty key in set",
			&config.Step{TextPatchINI: &config.TextPatchINI{
				Path: "/tmp/x",
				Set:  map[string]string{"  ": "v"},
			}},
			true,
		},
		{
			"empty key in delete",
			&config.Step{TextPatchINI: &config.TextPatchINI{
				Path:   "/tmp/x",
				Delete: []string{""},
			}},
			true,
		},
		{
			"ok set only",
			&config.Step{TextPatchINI: &config.TextPatchINI{
				Path: "/tmp/x",
				Set:  map[string]string{"S.k": "v"},
			}},
			false,
		},
		{
			"ok delete only",
			&config.Step{TextPatchINI: &config.TextPatchINI{
				Path:   "/tmp/x",
				Delete: []string{"S.k"},
			}},
			false,
		},
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

// ---- parser/emitter golden round-trip ----

func TestParser_RoundTripPreservesByteIdentical(t *testing.T) {
	inputs := []string{
		"",
		"\n",
		"[A]\nkey=val\n",
		"[A]\nkey = val\n[B]\nkey2 = val2\n",
		"# top comment\n[A]\n; section comment\nkey=val\n\n[B]\nx=1\n",
		"Port 22\nPermitRootLogin no\n",                          // ssh_config style
		"[Section]\r\nkey=val\r\n",                               // CRLF
		"[A]\n\tindented=val\n  spaced = val2\n",                 // mixed indent
		"[A]\nkey =   val with spaces   \n",                      // value spacing
		"only_top_level=here\n[After]\nx=y\n",
	}
	for _, in := range inputs {
		t.Run(strings.ReplaceAll(in, "\n", "\\n"), func(t *testing.T) {
			doc := parseINI([]byte(in))
			out := string(doc.render())
			if out != in {
				t.Errorf("round-trip mismatch\n got %q\nwant %q", out, in)
			}
		})
	}
}

// ---- handler end-to-end ----

func TestSet_ExistingSection_IdempotentSecondRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "php.ini")
	writeFile(t, path, "[PHP]\nmemory_limit = 128M\nmax_execution_time = 30\n")

	step := &config.Step{TextPatchINI: &config.TextPatchINI{
		Path: path,
		Set:  map[string]string{"PHP.memory_limit": "256M"},
	}}

	r1 := mustRun(t, false, step)
	if !r1.Changed {
		t.Fatalf("first run should change; reason=%q", r1.Reason)
	}
	after1, _ := os.ReadFile(path)
	if !strings.Contains(string(after1), "memory_limit = 256M") {
		t.Errorf("memory_limit not updated: %q", after1)
	}

	// Second run must be a no-op and the file must remain byte-identical.
	r2 := mustRun(t, false, step)
	if r2.Changed {
		t.Errorf("second run should be idempotent; reason=%q", r2.Reason)
	}
	after2, _ := os.ReadFile(path)
	if string(after1) != string(after2) {
		t.Errorf("file changed across idempotent runs:\nrun1=%q\nrun2=%q", after1, after2)
	}
}

func TestSet_NewSection_AppendsAtEOF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg.ini")
	writeFile(t, path, "[Existing]\nfoo=bar\n")

	r := mustRun(t, false, &config.Step{TextPatchINI: &config.TextPatchINI{
		Path: path,
		Set:  map[string]string{"NewSec.key": "val"},
	}})
	if !r.Changed {
		t.Fatal("expected change")
	}
	got, _ := os.ReadFile(path)
	want := "[Existing]\nfoo=bar\n\n[NewSec]\nkey=val"
	if string(got) != want && string(got) != want+"\n" {
		t.Errorf("unexpected output:\n got %q\nwant prefix %q", got, want)
	}
	if !strings.Contains(string(got), "[NewSec]") || !strings.Contains(string(got), "key=val") {
		t.Errorf("new section not properly written: %q", got)
	}
}

func TestSet_Sectionless_SshConfigStyle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ssh_config")
	writeFile(t, path, "Port 22\nPermitRootLogin no\n")

	// Sectionless: no dot in key.
	r := mustRun(t, false, &config.Step{TextPatchINI: &config.TextPatchINI{
		Path: path,
		Set:  map[string]string{"Port": "2222"},
	}})
	if !r.Changed {
		t.Fatal("expected change")
	}
	got, _ := os.ReadFile(path)
	want := "Port 2222\nPermitRootLogin no\n"
	if string(got) != want {
		t.Errorf("ssh_config edit mismatch\n got %q\nwant %q", got, want)
	}
}

func TestSet_ValueDrift_OldToNew(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.ini")
	writeFile(t, path, "[S]\nk=old\n")
	_ = mustRun(t, false, &config.Step{TextPatchINI: &config.TextPatchINI{
		Path: path,
		Set:  map[string]string{"S.k": "new"},
	}})
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "k=new") {
		t.Errorf("expected drift old→new: %q", got)
	}
	if strings.Contains(string(got), "k=old") {
		t.Errorf("old value still present: %q", got)
	}
}

func TestDelete_ExistingKey_RemovesLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.ini")
	writeFile(t, path, "[S]\nkeep=1\ndrop=2\nalso=3\n")
	r := mustRun(t, false, &config.Step{TextPatchINI: &config.TextPatchINI{
		Path:   path,
		Delete: []string{"S.drop"},
	}})
	if !r.Changed {
		t.Fatal("expected change")
	}
	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "drop") {
		t.Errorf("drop= line not removed: %q", got)
	}
	if !strings.Contains(string(got), "keep=1") || !strings.Contains(string(got), "also=3") {
		t.Errorf("siblings missing: %q", got)
	}
}

func TestDelete_MissingKey_Noop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.ini")
	original := "[S]\nkeep=1\n"
	writeFile(t, path, original)
	r := mustRun(t, false, &config.Step{TextPatchINI: &config.TextPatchINI{
		Path:   path,
		Delete: []string{"S.absent_key"},
	}})
	if r.Changed {
		t.Errorf("delete of missing key should be noop; reason=%q", r.Reason)
	}
	got, _ := os.ReadFile(path)
	if string(got) != original {
		t.Errorf("file should be untouched: %q", got)
	}
}

func TestBackup_CreatesBakOnChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.ini")
	original := "[S]\nk=1\n"
	writeFile(t, path, original)
	_ = mustRun(t, false, &config.Step{TextPatchINI: &config.TextPatchINI{
		Path:   path,
		Set:    map[string]string{"S.k": "2"},
		Backup: true,
	}})
	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	if string(bak) != original {
		t.Errorf("backup content wrong: %q", bak)
	}
}

func TestPreservesAdjacentComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.ini")
	original := "[S]\n# comment about k\nk=old\n; another\nother=stay\n"
	writeFile(t, path, original)
	_ = mustRun(t, false, &config.Step{TextPatchINI: &config.TextPatchINI{
		Path: path,
		Set:  map[string]string{"S.k": "new"},
	}})
	got, _ := os.ReadFile(path)
	for _, line := range []string{"# comment about k", "; another", "other=stay"} {
		if !strings.Contains(string(got), line) {
			t.Errorf("missing %q in output:\n%s", line, got)
		}
	}
	if !strings.Contains(string(got), "k=new") {
		t.Errorf("missing updated value: %q", got)
	}
}

func TestPreservesSectionOrdering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.ini")
	original := "[Alpha]\na=1\n\n[Beta]\nb=2\n\n[Gamma]\ng=3\n"
	writeFile(t, path, original)
	_ = mustRun(t, false, &config.Step{TextPatchINI: &config.TextPatchINI{
		Path: path,
		Set: map[string]string{
			"Alpha.a": "10",
			"Beta.b":  "20",
			"Gamma.g": "30",
		},
	}})
	got, _ := os.ReadFile(path)
	posA := strings.Index(string(got), "[Alpha]")
	posB := strings.Index(string(got), "[Beta]")
	posG := strings.Index(string(got), "[Gamma]")
	if !(posA < posB && posB < posG) {
		t.Errorf("section order lost: A=%d B=%d G=%d\n%s", posA, posB, posG, got)
	}
}

func TestCRLF_RoundTripPreserved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "win.ini")
	original := "[S]\r\nkey=old\r\n"
	writeFile(t, path, original)
	_ = mustRun(t, false, &config.Step{TextPatchINI: &config.TextPatchINI{
		Path: path,
		Set:  map[string]string{"S.key": "new"},
	}})
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "\r\n") {
		t.Errorf("CRLF not preserved: %q", got)
	}
	if strings.Contains(strings.ReplaceAll(string(got), "\r\n", ""), "\n") {
		t.Errorf("lone LF leaked through: %q", got)
	}
	if !strings.Contains(string(got), "key=new") {
		t.Errorf("value not updated: %q", got)
	}
}

func TestPlanMode_ReportsWouldChange_NoWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.ini")
	original := "[S]\nk=old\n"
	writeFile(t, path, original)

	r := mustRun(t, true, &config.Step{TextPatchINI: &config.TextPatchINI{
		Path: path,
		Set:  map[string]string{"S.k": "new"},
	}})
	if !r.WouldChange {
		t.Errorf("plan should report WouldChange; reason=%q", r.Reason)
	}
	if r.Changed {
		t.Errorf("plan must not set Changed")
	}
	got, _ := os.ReadFile(path)
	if string(got) != original {
		t.Errorf("plan must not write; got %q", got)
	}
}

func TestPlanMode_IdempotentAfterApply(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.ini")
	writeFile(t, path, "[S]\nk=old\n")
	step := &config.Step{TextPatchINI: &config.TextPatchINI{
		Path: path,
		Set:  map[string]string{"S.k": "new"},
	}}
	_ = mustRun(t, false, step) // apply once
	r := mustRun(t, true, step) // plan after apply
	if r.WouldChange {
		t.Errorf("plan after successful apply should be a noop; reason=%q", r.Reason)
	}
}

func TestFile_DoesNotExist_SetCreatesIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.ini")
	r := mustRun(t, false, &config.Step{TextPatchINI: &config.TextPatchINI{
		Path: path,
		Set:  map[string]string{"S.k": "v"},
	}})
	if !r.Changed {
		t.Fatal("expected file creation")
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "[S]") || !strings.Contains(string(got), "k=v") {
		t.Errorf("created file content wrong: %q", got)
	}
}

func TestFile_DoesNotExist_DeleteOnlyIsNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.ini")
	r := mustRun(t, false, &config.Step{TextPatchINI: &config.TextPatchINI{
		Path:   path,
		Delete: []string{"S.k"},
	}})
	if r.Changed {
		t.Errorf("delete on missing file should be noop; reason=%q", r.Reason)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file should not be created: %v", err)
	}
}

func TestSplitKey(t *testing.T) {
	cases := []struct {
		in       string
		wantSec  string
		wantKey  string
	}{
		{"PHP.memory_limit", "PHP", "memory_limit"},
		{"Section.key.with.dots", "Section", "key.with.dots"},
		{"bareKey", "", "bareKey"},
	}
	for _, c := range cases {
		sec, key := splitKey(c.in)
		if sec != c.wantSec || key != c.wantKey {
			t.Errorf("splitKey(%q) = (%q,%q), want (%q,%q)", c.in, sec, key, c.wantSec, c.wantKey)
		}
	}
}
