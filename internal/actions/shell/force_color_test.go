package shell

import "testing"

func TestHasEnv(t *testing.T) {
	cases := []struct {
		name string
		env  []string
		key  string
		want bool
	}{
		{"present-with-value", []string{"FOO=bar", "BAZ=qux"}, "FOO", true},
		{"present-empty-value", []string{"FOO="}, "FOO", true},
		{"absent", []string{"BAR=baz"}, "FOO", false},
		{"empty-env", nil, "FOO", false},
		// Prefix-match must NOT confuse "FOO" with "FOOBAR".
		{"prefix-not-equal", []string{"FOOBAR=baz"}, "FOO", false},
		// Case sensitivity: env vars are conventionally upper; this
		// pins the contract so future "case-insensitive" refactors
		// surface in CI.
		{"case-sensitive", []string{"foo=bar"}, "FOO", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := hasEnv(c.env, c.key); got != c.want {
				t.Errorf("hasEnv(%v, %q) = %v, want %v", c.env, c.key, got, c.want)
			}
		})
	}
}

// shouldForceChildColor's TTY-detection arm depends on whether the
// test process's stdout is a real terminal. Under `go test` that's
// always false (stdout is a pipe). We pin only the env-driven
// negative cases here; the TTY arm is exercised end-to-end by the
// integration smoke test in the repo's pre-push gate.
func TestShouldForceChildColor_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if shouldForceChildColor() {
		t.Errorf("NO_COLOR set should suppress force-color regardless of TTY state")
	}
}

func TestShouldForceChildColor_NoTTY(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	if shouldForceChildColor() {
		t.Errorf("under go test, stdout is a pipe; want false")
	}
}
