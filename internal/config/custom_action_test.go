package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// notifyOnly is a test predicate: it recognizes a single custom action,
// standing in for a real registry's Has method.
func notifyOnly(name string) bool { return name == "notify.webhook" }

func writeTemp(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "plan.yml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// A typed-key custom action validates clean and decodes into the carrier when
// the registry predicate recognizes it — byte-identical syntax to a built-in.
func TestReadConfig_TypedKeyCustomAction(t *testing.T) {
	p := writeTemp(t, `
- name: notify deploy success
  notify.webhook:
    url: https://hooks.example.com/deploy
    method: POST
`)
	cfg, diags, err := ReadConfigWithValidation(p, notifyOnly)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if errs := filterErrors(diags); len(errs) > 0 {
		t.Fatalf("unexpected error diagnostics: %v", errs)
	}
	if len(cfg.Steps) != 1 {
		t.Fatalf("got %d steps, want 1", len(cfg.Steps))
	}
	s := cfg.Steps[0]
	if got := s.DetermineActionType(); got != "notify.webhook" {
		t.Errorf("action type = %q, want notify.webhook", got)
	}
	if s.Action != "notify.webhook" {
		t.Errorf("Action = %q, want notify.webhook", s.Action)
	}
	if got, _ := s.With["url"].(string); got != "https://hooks.example.com/deploy" {
		t.Errorf("With[url] = %q, want the webhook URL", got)
	}
	if got, _ := s.With["method"].(string); got != "POST" {
		t.Errorf("With[method] = %q, want POST", got)
	}
}

// Without the predicate (nil registry) the same plan is rejected as an unknown
// field — exactly today's behavior, so existing callers are unaffected.
func TestReadConfig_TypedKeyRejectedWithoutRegistry(t *testing.T) {
	p := writeTemp(t, `
- name: notify deploy success
  notify.webhook:
    url: https://hooks.example.com/deploy
`)
	_, diags, err := ReadConfigWithValidation(p, nil)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(filterErrors(diags)) == 0 {
		t.Fatalf("expected an unknown-field diagnostic for notify.webhook without a registry")
	}
}

// A misspelled custom action is NOT folded (the predicate rejects it) and so
// still surfaces as an unknown field — typo-catching is preserved.
func TestReadConfig_TypoNotFolded(t *testing.T) {
	p := writeTemp(t, `
- name: oops
  notifyy.webhook:
    url: https://hooks.example.com/deploy
`)
	_, diags, err := ReadConfigWithValidation(p, notifyOnly)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(filterErrors(diags)) == 0 {
		t.Fatalf("expected an unknown-field diagnostic for the typo notifyy.webhook")
	}
}

// A custom action nested inside a transaction is folded too (recursion through
// the reflection-derived []Step compound fields).
func TestReadConfig_TypedKeyInsideTransaction(t *testing.T) {
	p := writeTemp(t, `
- name: deploy then notify
  transaction:
    - name: notify
      notify.webhook:
        url: https://hooks.example.com/deploy
`)
	cfg, diags, err := ReadConfigWithValidation(p, notifyOnly)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if errs := filterErrors(diags); len(errs) > 0 {
		t.Fatalf("unexpected error diagnostics: %v", errs)
	}
	if len(cfg.Steps) != 1 || len(cfg.Steps[0].Transaction) != 1 {
		t.Fatalf("unexpected shape: %+v", cfg.Steps)
	}
	if got := cfg.Steps[0].Transaction[0].Action; got != "notify.webhook" {
		t.Errorf("nested Action = %q, want notify.webhook", got)
	}
}

// Folding follows a deliberate boundary: a Step's own control-flow compounds
// (transaction/try/…) are folded, but steps nested inside a typed sub-struct
// like artifact.capture are NOT. Such a typed-key custom there is not silently
// dropped — it surfaces a clear unknown-field error, and the carrier remains
// available as the escape hatch. This test pins that contract.
func TestReadConfig_TypedKeyInArtifactCaptureRejected(t *testing.T) {
	p := writeTemp(t, `
- name: cap
  artifact.capture:
    name: snap
    steps:
      - name: notify
        notify.webhook:
          url: https://hooks.example.com/deploy
`)
	_, diags, err := ReadConfigWithValidation(p, notifyOnly)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	errs := filterErrors(diags)
	if len(errs) == 0 {
		t.Fatalf("expected an unknown-field error for notify.webhook nested in artifact.capture")
	}
	if !strings.Contains(errs[0].Message, "notify.webhook") {
		t.Errorf("error should name the unfolded key, got: %s", errs[0].Message)
	}
}

func filterErrors(diags []Diagnostic) []Diagnostic {
	var out []Diagnostic
	for _, d := range diags {
		if d.Severity == "error" {
			out = append(out, d)
		}
	}
	return out
}
