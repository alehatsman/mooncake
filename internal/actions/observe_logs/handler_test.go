package observe_logs

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

func writeLog(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "test.log")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func TestValidate(t *testing.T) {
	h := &Handler{}
	if err := h.Validate(&config.Step{ObserveLogs: nil}); err == nil {
		t.Fatal("expected error for nil")
	}
	if err := h.Validate(&config.Step{ObserveLogs: &config.ObserveLogs{}}); err == nil {
		t.Fatal("expected error for missing source")
	}
	if err := h.Validate(&config.Step{ObserveLogs: &config.ObserveLogs{
		Path: "/tmp/x", JournalUnit: "sshd", Patterns: []string{"err"},
	}}); err == nil {
		t.Fatal("expected error for multiple sources")
	}
	if err := h.Validate(&config.Step{ObserveLogs: &config.ObserveLogs{
		Path: "/tmp/x",
	}}); err == nil {
		t.Fatal("expected error for missing patterns")
	}
	if err := h.Validate(&config.Step{ObserveLogs: &config.ObserveLogs{
		Path: "/tmp/x", Patterns: []string{"[invalid"},
	}}); err == nil {
		t.Fatal("expected error for bad regex")
	}
	if err := h.Validate(&config.Step{ObserveLogs: &config.ObserveLogs{
		Path: "/tmp/x", Patterns: []string{"err"}, Since: "not-a-duration",
	}}); err == nil {
		t.Fatal("expected error for bad since")
	}
	if err := h.Validate(&config.Step{ObserveLogs: &config.ObserveLogs{
		Path: "/tmp/x", Patterns: []string{"err"},
	}}); err != nil {
		t.Fatalf("expected no error: %v", err)
	}
}

func TestRun_FileSource_MatchesPatterns(t *testing.T) {
	content := `INFO request started
ERROR connection refused
INFO request started
ERROR timeout fetching upstream
WARN slow request
`
	p := writeLog(t, content)
	h := &Handler{}
	step := &config.Step{ObserveLogs: &config.ObserveLogs{
		Path:        p,
		Patterns:    []string{"ERROR", "WARN"},
		SampleLines: 3,
	}}
	res, err := h.Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data := res.(*executor.Result).Data
	if found, _ := data["found"].(bool); !found {
		t.Errorf("expected found=true; data=%v", data)
	}
	val, _ := data["value"].(map[string]any)
	matches, _ := val["matches"].([]any)
	if len(matches) != 2 {
		t.Fatalf("expected 2 match groups, got %d", len(matches))
	}
	first, _ := matches[0].(map[string]any)
	if first["pattern"] != "ERROR" {
		t.Errorf("first pattern = %v, want ERROR", first["pattern"])
	}
	if c, _ := first["count"].(float64); int(c) != 2 {
		t.Errorf("ERROR count = %v, want 2", first["count"])
	}
	second, _ := matches[1].(map[string]any)
	if c, _ := second["count"].(float64); int(c) != 1 {
		t.Errorf("WARN count = %v, want 1", second["count"])
	}
	if linesRead, _ := val["lines_read"].(float64); int(linesRead) != 5 {
		t.Errorf("lines_read = %v, want 5", val["lines_read"])
	}
}

func TestRun_FileSource_NoMatches_StillFound(t *testing.T) {
	p := writeLog(t, "INFO clean run\nINFO another clean line\n")
	h := &Handler{}
	step := &config.Step{ObserveLogs: &config.ObserveLogs{
		Path:     p,
		Patterns: []string{"ERROR", "FATAL"},
	}}
	res, err := h.Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data := res.(*executor.Result).Data
	if found, _ := data["found"].(bool); !found {
		t.Errorf("expected found=true even with zero matches")
	}
	val, _ := data["value"].(map[string]any)
	matches, _ := val["matches"].([]any)
	for _, m := range matches {
		mm, _ := m.(map[string]any)
		if c, _ := mm["count"].(float64); c != 0 {
			t.Errorf("pattern %v: count should be 0 (clean log); got %v", mm["pattern"], c)
		}
	}
}

func TestRun_FileSource_MaxLinesTruncates(t *testing.T) {
	// 100 lines all containing "ERROR"; cap at 10 → matches count is 10, truncated=true.
	content := ""
	for i := 0; i < 100; i++ {
		content += "ERROR line " + string(rune('A'+(i%26))) + "\n"
	}
	p := writeLog(t, content)
	h := &Handler{}
	step := &config.Step{ObserveLogs: &config.ObserveLogs{
		Path:     p,
		Patterns: []string{"ERROR"},
		MaxLines: 10,
	}}
	res, err := h.Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	val, _ := res.(*executor.Result).Data["value"].(map[string]any)
	if trunc, _ := val["truncated"].(bool); !trunc {
		t.Errorf("expected truncated=true")
	}
	matches, _ := val["matches"].([]any)
	first, _ := matches[0].(map[string]any)
	if c, _ := first["count"].(float64); int(c) != 10 {
		t.Errorf("count under max_lines=10 should be 10; got %v", first["count"])
	}
}

func TestRun_FileSource_MissingPath(t *testing.T) {
	h := &Handler{}
	step := &config.Step{ObserveLogs: &config.ObserveLogs{
		Path:     "/no-such-log-file-xyz",
		Patterns: []string{"ERROR"},
	}}
	res, err := h.Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data := res.(*executor.Result).Data
	if found, _ := data["found"].(bool); found {
		t.Errorf("expected found=false for missing file")
	}
	if errStr, _ := data["error"].(string); errStr == "" {
		t.Errorf("expected error message for missing file")
	}
}

func TestRun_PlanMode_Defers(t *testing.T) {
	h := &Handler{}
	step := &config.Step{ObserveLogs: &config.ObserveLogs{
		Path:     "/tmp/anything",
		Patterns: []string{"err"},
	}}
	res, err := h.Run(newCtx(t, true), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data := res.(*executor.Result).Data
	if found, _ := data["found"].(bool); found {
		t.Errorf("plan-mode Found must be false")
	}
}

func TestClassifySource(t *testing.T) {
	cases := []struct {
		o       config.ObserveLogs
		wantSrc string
		wantID  string
	}{
		{config.ObserveLogs{Path: "/var/log/foo"}, "file", "/var/log/foo"},
		{config.ObserveLogs{JournalUnit: "sshd"}, "journal", "sshd"},
		{config.ObserveLogs{Container: "nginx"}, "container", "nginx"},
	}
	for _, c := range cases {
		got, id := classifySource(&c.o)
		if got != c.wantSrc || id != c.wantID {
			t.Errorf("classifySource(%+v) = (%q, %q), want (%q, %q)", c.o, got, id, c.wantSrc, c.wantID)
		}
	}
}
