package fleet

import (
	"strings"
	"testing"
)

func TestJoinCommandArgs_SingleString(t *testing.T) {
	got, err := joinCommandArgs([]string{"systemctl is-active sshd"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "systemctl is-active sshd" {
		t.Errorf("single-string form = %q", got)
	}
}

func TestJoinCommandArgs_ArgvForm(t *testing.T) {
	// `mooncake fleet exec -- ls -la /tmp` yields the args after `--`.
	got, err := joinCommandArgs([]string{"ls", "-la", "/tmp"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ls -la /tmp" {
		t.Errorf("argv form = %q", got)
	}
}

func TestJoinCommandArgs_QuotingIsOperatorsProblem(t *testing.T) {
	// `mooncake fleet exec 'echo hi there'` yields a single arg —
	// urfave/cli treats the quoted arg as one. Passing through
	// verbatim is the contract; the peer's shell parses the rest.
	got, _ := joinCommandArgs([]string{"echo 'hi there'"})
	if got != "echo 'hi there'" {
		t.Errorf("quoted form = %q", got)
	}
}

func TestJoinCommandArgs_EmptyArgsErrors(t *testing.T) {
	_, err := joinCommandArgs(nil)
	if err == nil {
		t.Fatal("expected error on empty args")
	}
	if !strings.Contains(err.Error(), "missing command") {
		t.Errorf("error should mention missing command, got %q", err)
	}
}

func TestParseEnvFlags_EmptyIsNilMap(t *testing.T) {
	m, err := parseEnvFlags(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m != nil {
		t.Errorf("empty input should return nil map, got %v", m)
	}
}

func TestParseEnvFlags_AcceptsKEYVAL(t *testing.T) {
	m, err := parseEnvFlags([]string{"FOO=bar", "BAZ=qux"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if m["FOO"] != "bar" || m["BAZ"] != "qux" {
		t.Errorf("map = %v", m)
	}
}

func TestParseEnvFlags_EmptyValueAllowed(t *testing.T) {
	m, err := parseEnvFlags([]string{"FOO="})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if v, ok := m["FOO"]; !ok || v != "" {
		t.Errorf("FOO= should map to empty value, got %q ok=%v", v, ok)
	}
}

func TestParseEnvFlags_RejectsMissingEquals(t *testing.T) {
	_, err := parseEnvFlags([]string{"NOPE"})
	if err == nil {
		t.Fatal("expected error for missing '='")
	}
}

func TestParseEnvFlags_RejectsEmptyKey(t *testing.T) {
	_, err := parseEnvFlags([]string{"=bar"})
	if err == nil {
		t.Fatal("expected error for empty KEY")
	}
}
