package print

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// TestValidate covers the new accept/reject rules for the structured-
// log form.
func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		step    *config.Step
		wantErr string
	}{
		{
			name:    "nil log",
			step:    &config.Step{Log: nil},
			wantErr: "configuration is nil",
		},
		{
			name:    "all empty",
			step:    &config.Step{Log: &config.PrintAction{}},
			wantErr: "at least one of msg, title, data",
		},
		{
			name: "msg only — legacy form",
			step: &config.Step{Log: &config.PrintAction{Msg: "hi"}},
		},
		{
			name: "data only",
			step: &config.Step{Log: &config.PrintAction{Data: map[string]any{"k": "v"}}},
		},
		{
			name: "title only",
			step: &config.Step{Log: &config.PrintAction{Title: "Header"}},
		},
		{
			name:    "bad format",
			step:    &config.Step{Log: &config.PrintAction{Data: 1, Format: "yaml"}},
			wantErr: "format must be kv or json",
		},
		{
			name: "format=kv allowed",
			step: &config.Step{Log: &config.PrintAction{Data: 1, Format: "kv"}},
		},
		{
			name: "format=json allowed",
			step: &config.Step{Log: &config.PrintAction{Data: 1, Format: "json"}},
		},
		{
			name:    "negative budget",
			step:    &config.Step{Log: &config.PrintAction{Msg: "hi", Budget: -1}},
			wantErr: "budget must be >= 0",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := (&Handler{}).Validate(c.step)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("expected error containing %q, got %v", c.wantErr, err)
			}
		})
	}
}

// TestRender_KVMap exercises the default kv format on a map: keys are
// alphabetized, aligned, and separated by two spaces.
func TestRender_KVMap(t *testing.T) {
	ctx := newCtx(t, false)
	step := &config.Step{Log: &config.PrintAction{
		Title: "Service config",
		Data: map[string]any{
			"port":    8080,
			"tls":     true,
			"backend": "ollama",
		},
	}}
	res, err := (&Handler{}).Run(ctx, step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := res.(*executor.Result).Stdout

	wantSubs := []string{
		"Service config",
		"backend  ollama",
		"port     8080",
		"tls      true",
	}
	for _, s := range wantSubs {
		if !strings.Contains(out, s) {
			t.Errorf("output missing %q\n--- got ---\n%s", s, out)
		}
	}

	// Keys must be sorted alphabetically: backend < port < tls.
	bi := strings.Index(out, "backend")
	pi := strings.Index(out, "port")
	ti := strings.Index(out, "tls")
	if !(bi < pi && pi < ti) {
		t.Errorf("kv keys not alphabetized: backend@%d port@%d tls@%d", bi, pi, ti)
	}
}

// TestRender_KVScalar: a bare scalar Data renders via fmt.Sprintf.
func TestRender_KVScalar(t *testing.T) {
	ctx := newCtx(t, false)
	step := &config.Step{Log: &config.PrintAction{Data: 42}}
	res, _ := (&Handler{}).Run(ctx, step)
	if got := res.(*executor.Result).Stdout; got != "42" {
		t.Errorf("expected scalar render %q, got %q", "42", got)
	}
}

// TestRender_KVSlice: a slice renders one bullet per item.
func TestRender_KVSlice(t *testing.T) {
	ctx := newCtx(t, false)
	step := &config.Step{Log: &config.PrintAction{
		Data: []any{"alpha", "beta", "gamma"},
	}}
	res, _ := (&Handler{}).Run(ctx, step)
	out := res.(*executor.Result).Stdout
	for _, s := range []string{"- alpha", "- beta", "- gamma"} {
		if !strings.Contains(out, s) {
			t.Errorf("slice render missing %q\n--- got ---\n%s", s, out)
		}
	}
}

// TestRender_JSONFormat: format=json emits pretty-printed JSON.
func TestRender_JSONFormat(t *testing.T) {
	ctx := newCtx(t, false)
	step := &config.Step{Log: &config.PrintAction{
		Format: "json",
		Data:   map[string]any{"port": 8080, "tls": true},
	}}
	res, _ := (&Handler{}).Run(ctx, step)
	out := res.(*executor.Result).Stdout

	// Output must be valid JSON when parsed back.
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json output not parseable: %v\n--- got ---\n%s", err, out)
	}
	if parsed["port"] != float64(8080) || parsed["tls"] != true {
		t.Errorf("json round-trip lost data: %v", parsed)
	}
	// Must be indented (2-space) — assert at least one indented line.
	if !strings.Contains(out, "\n  ") {
		t.Errorf("json output not pretty-printed:\n%s", out)
	}
}

// TestRender_MsgTemplated: legacy msg path still templates against
// variables exactly as before.
func TestRender_MsgTemplated(t *testing.T) {
	ctx := newCtx(t, false)
	step := &config.Step{Log: &config.PrintAction{Msg: "hello {{ who }}"}}
	res, _ := (&Handler{}).Run(ctx, step)
	got := res.(*executor.Result).Stdout
	if got != "hello world" {
		t.Errorf("expected templated msg %q, got %q", "hello world", got)
	}
}

// TestRender_MsgPlusData: msg renders first, then data block.
func TestRender_MsgPlusData(t *testing.T) {
	ctx := newCtx(t, false)
	step := &config.Step{Log: &config.PrintAction{
		Msg:  "summary:",
		Data: map[string]any{"a": 1},
	}}
	res, _ := (&Handler{}).Run(ctx, step)
	out := res.(*executor.Result).Stdout
	if !strings.HasPrefix(out, "summary:\n") {
		t.Errorf("msg should appear before data; got %q", out)
	}
	if !strings.Contains(out, "a  1") {
		t.Errorf("data block missing; got %q", out)
	}
}

// TestRender_BudgetExplicit: an explicit budget truncates and appends
// the suffix.
func TestRender_BudgetExplicit(t *testing.T) {
	ctx := newCtx(t, false)
	step := &config.Step{Log: &config.PrintAction{
		Msg:    strings.Repeat("x", 200),
		Budget: 50,
	}}
	res, _ := (&Handler{}).Run(ctx, step)
	out := res.(*executor.Result).Stdout
	if len(out) > 50 {
		t.Errorf("output not truncated to budget: len=%d (budget=50)", len(out))
	}
	if !strings.HasSuffix(out, "(truncated)") {
		t.Errorf("output missing truncation suffix: %q", out)
	}
}

// TestRender_BudgetDefaultPreservesShortOutput: with Budget==0, short
// outputs are not truncated (auto-default only triggers above 4 KiB).
func TestRender_BudgetDefaultPreservesShortOutput(t *testing.T) {
	ctx := newCtx(t, false)
	step := &config.Step{Log: &config.PrintAction{Msg: "short"}}
	res, _ := (&Handler{}).Run(ctx, step)
	if got := res.(*executor.Result).Stdout; got != "short" {
		t.Errorf("short output should not be touched; got %q", got)
	}
}

// TestRender_BudgetDefaultAutoApplies: huge output with Budget==0 gets
// auto-truncated at DefaultBudget.
func TestRender_BudgetDefaultAutoApplies(t *testing.T) {
	ctx := newCtx(t, false)
	step := &config.Step{Log: &config.PrintAction{
		Msg: strings.Repeat("x", DefaultBudget*2),
	}}
	res, _ := (&Handler{}).Run(ctx, step)
	out := res.(*executor.Result).Stdout
	if len(out) > DefaultBudget {
		t.Errorf("default budget not applied: len=%d (default=%d)", len(out), DefaultBudget)
	}
	if !strings.HasSuffix(out, "(truncated)") {
		t.Errorf("default-truncated output missing suffix: %q", out[len(out)-30:])
	}
}

// TestRender_BudgetUTF8Safe: truncation does not split a multibyte rune.
func TestRender_BudgetUTF8Safe(t *testing.T) {
	ctx := newCtx(t, false)
	step := &config.Step{Log: &config.PrintAction{
		Msg:    strings.Repeat("ä", 200), // 2 bytes per rune
		Budget: 50,
	}}
	res, _ := (&Handler{}).Run(ctx, step)
	out := res.(*executor.Result).Stdout
	if !utf8.ValidString(out) {
		t.Errorf("truncation produced invalid utf-8: %q", out)
	}
}

// TestPlanMode_DataPreview: plan-mode reason previews the first non-
// empty line of the rendered output, even when only Data is set.
func TestPlanMode_DataPreview(t *testing.T) {
	ctx := newCtx(t, true)
	step := &config.Step{Log: &config.PrintAction{
		Data: map[string]any{"only": "field"},
	}}
	res, _ := (&Handler{}).Run(ctx, step)
	r := res.(*executor.Result)
	if !strings.Contains(r.Reason, "only") {
		t.Errorf("plan preview should mention the kv key; got %q", r.Reason)
	}
}
