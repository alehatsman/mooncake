package security

import (
	"bytes"
	"strings"
	"testing"
)

// TestStdinProvider_Caches verifies the per-process cache: two
// resolutions of the same key only invoke the prompt function once.
// This is the "we won't pester the user multiple times for the same
// secret" contract.
func TestStdinProvider_Caches(t *testing.T) {
	calls := 0
	p := NewStdinProvider()
	p.out = &bytes.Buffer{}
	p.promptFn = func(_ int) ([]byte, error) {
		calls++
		return []byte("the-value"), nil
	}

	v1, err := p.Resolve("my-key")
	if err != nil {
		t.Fatalf("first Resolve: %v", err)
	}
	v2, err := p.Resolve("my-key")
	if err != nil {
		t.Fatalf("second Resolve: %v", err)
	}
	if v1 != "the-value" || v2 != "the-value" {
		t.Errorf("got %q / %q, want both 'the-value'", v1, v2)
	}
	if calls != 1 {
		t.Errorf("prompt invoked %d times, want 1", calls)
	}
}

// TestStdinProvider_DifferentKeysPromptSeparately: cache keyed by ref
// — different keys should prompt independently.
func TestStdinProvider_DifferentKeysPromptSeparately(t *testing.T) {
	calls := 0
	p := NewStdinProvider()
	p.out = &bytes.Buffer{}
	p.promptFn = func(_ int) ([]byte, error) {
		calls++
		if calls == 1 {
			return []byte("first"), nil
		}
		return []byte("second"), nil
	}

	a, _ := p.Resolve("alpha")
	b, _ := p.Resolve("beta")
	if a != "first" || b != "second" {
		t.Errorf("got a=%q b=%q, want a='first' b='second'", a, b)
	}
	if calls != 2 {
		t.Errorf("prompt invoked %d times, want 2", calls)
	}
}

func TestStdinProvider_EmptyKeyErrors(t *testing.T) {
	p := NewStdinProvider()
	_, err := p.Resolve("  ")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestStdinProvider_EmptyInputErrors(t *testing.T) {
	p := NewStdinProvider()
	p.out = &bytes.Buffer{}
	p.promptFn = func(_ int) ([]byte, error) { return []byte(""), nil }
	_, err := p.Resolve("any")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	if !strings.Contains(err.Error(), "empty input") {
		t.Errorf("err = %v, want 'empty input'", err)
	}
}

func TestStdinProvider_TrimsTrailingNewline(t *testing.T) {
	p := NewStdinProvider()
	p.out = &bytes.Buffer{}
	p.promptFn = func(_ int) ([]byte, error) { return []byte("password\r\n"), nil }
	got, err := p.Resolve("key")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "password" {
		t.Errorf("got %q, want 'password'", got)
	}
}

// TestStdinProvider_PromptsToErrorWriter: the prompt goes to the
// provider's out writer (stderr in production), never stdout. Apply
// stdout must stay parseable.
func TestStdinProvider_PromptsToErrorWriter(t *testing.T) {
	out := &bytes.Buffer{}
	p := NewStdinProvider()
	p.out = out
	p.promptFn = func(_ int) ([]byte, error) { return []byte("x"), nil }
	_, _ = p.Resolve("show-me")

	got := out.String()
	if !strings.Contains(got, "stdin:show-me") {
		t.Errorf("prompt text missing ref: %q", got)
	}
	if !strings.Contains(got, "Enter secret") {
		t.Errorf("prompt text missing 'Enter secret': %q", got)
	}
}

// TestStdinProvider_ReadErrorSurfaces: term.ReadPassword can fail
// (e.g. ECANCELED on Ctrl-C). The provider must surface a clean error
// rather than panicking or returning empty.
func TestStdinProvider_ReadErrorSurfaces(t *testing.T) {
	p := NewStdinProvider()
	p.out = &bytes.Buffer{}
	p.promptFn = func(_ int) ([]byte, error) {
		return nil, &readErr{}
	}
	_, err := p.Resolve("any")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "read failed") {
		t.Errorf("err = %v, want 'read failed'", err)
	}
}

type readErr struct{}

func (readErr) Error() string { return "synthetic read error" }
