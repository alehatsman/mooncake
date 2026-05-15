package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStrictFields_RejectsRegister guards the issue-17 / MT-44
// regression: the docs used to say `register: NAME`, spec-21 renamed
// it to `as: NAME`, and the YAML unmarshaler silently dropped the
// unknown key. With strict mode the user gets a clear error pointing
// at the typo.
func TestStrictFields_RejectsRegister(t *testing.T) {
	body := `version: "1.0"
steps:
  - name: bad
    shell: echo hi
    register: r
`
	diags := readWithDiags(t, body)
	if !hasDiagMessage(diags, "register") {
		t.Errorf("expected diagnostic mentioning 'register'; got %v", diags)
	}
}

// TestStrictFields_RejectsSha256OnFileDownload guards finding #44:
// `sha256:` looks like a checksum field but the real field is
// `checksum:` — silently dropped before the fix, the verify path
// never ran.
func TestStrictFields_RejectsSha256OnFileDownload(t *testing.T) {
	body := `version: "1.0"
steps:
  - file.download:
      url: https://example.com/x
      dest: /tmp/x
      sha256: "00"
`
	diags := readWithDiags(t, body)
	if !hasDiagMessage(diags, "sha256") {
		t.Errorf("expected diagnostic mentioning 'sha256'; got %v", diags)
	}
}

// TestStrictFields_AcceptsValidCapture is the negative control —
// the documented `as: NAME` form must pass clean.
func TestStrictFields_AcceptsValidCapture(t *testing.T) {
	body := `version: "1.0"
steps:
  - name: ok
    shell: echo hi
    as: r
`
	diags := readWithDiags(t, body)
	for _, d := range diags {
		if d.Severity == "error" {
			t.Errorf("expected no errors on valid `as:` form; got %+v", d)
		}
	}
}

// TestStrictFields_AcceptsValidFileDownload — the canonical
// `checksum:` field must pass without strict-mode noise.
func TestStrictFields_AcceptsValidFileDownload(t *testing.T) {
	body := `version: "1.0"
steps:
  - file.download:
      url: https://example.com/x
      dest: /tmp/x
      checksum: "sha256:0000000000000000000000000000000000000000000000000000000000000000"
`
	diags := readWithDiags(t, body)
	for _, d := range diags {
		if d.Severity == "error" {
			t.Errorf("expected no errors on canonical checksum: form; got %+v", d)
		}
	}
}

// readWithDiags writes body to a temp file and runs the reader,
// returning the diagnostics (not just the typed error).
func readWithDiags(t *testing.T, body string) []Diagnostic {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "test.yml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	r := &YAMLConfigReader{}
	_, diags, err := r.ReadConfigWithValidation(p)
	if err != nil {
		t.Logf("reader err (may be expected for strict-mode rejects): %v", err)
	}
	return diags
}

func hasDiagMessage(diags []Diagnostic, needle string) bool {
	for _, d := range diags {
		if strings.Contains(d.Message, needle) {
			return true
		}
	}
	return false
}
