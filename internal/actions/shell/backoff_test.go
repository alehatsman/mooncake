package shell

import (
	"testing"
	"time"
)

// TestMT62_ScaleRetryDelay covers manual-test #62 (2026-05-15):
// retry.backoff was a documented field but never read — `linear` and
// `exponential` were silently treated as `fixed`, so external-API
// integrations got thundering-herd retries instead of the backoff
// curves they configured.
func TestMT62_ScaleRetryDelay(t *testing.T) {
	base := 100 * time.Millisecond
	cases := []struct {
		strategy string
		attempt  int
		want     time.Duration
	}{
		// fixed (default): every retry waits the bare delay
		{"fixed", 1, 100 * time.Millisecond},
		{"fixed", 5, 100 * time.Millisecond},
		{"", 1, 100 * time.Millisecond}, // empty == fixed
		{"nonsense", 3, 100 * time.Millisecond}, // unknown == fixed

		// linear: attempt × base
		{"linear", 1, 100 * time.Millisecond},
		{"linear", 2, 200 * time.Millisecond},
		{"linear", 3, 300 * time.Millisecond},

		// exponential: 2^(attempt-1) × base
		{"exponential", 1, 100 * time.Millisecond},
		{"exponential", 2, 200 * time.Millisecond},
		{"exponential", 3, 400 * time.Millisecond},
		{"exponential", 4, 800 * time.Millisecond},
	}
	for _, c := range cases {
		got := ScaleRetryDelay(base, c.strategy, c.attempt)
		if got != c.want {
			t.Errorf("ScaleRetryDelay(%s, base=%s, attempt=%d) = %s, want %s",
				c.strategy, base, c.attempt, got, c.want)
		}
	}
}

func TestMT62_ScaleRetryDelay_GuardsLowAttempt(t *testing.T) {
	// attempt < 1 should be treated as 1 to avoid pathological zero
	// or negative sleeps from caller bugs.
	got := ScaleRetryDelay(50*time.Millisecond, "linear", 0)
	if got != 50*time.Millisecond {
		t.Errorf("attempt=0 → %s, want 50ms (clamped)", got)
	}
}

func TestMT62_ScaleRetryDelay_ExponentialClamp(t *testing.T) {
	// Absurd attempt counts must not overflow the multiplier. Cap is
	// 30 shifts (~17 minutes at 1ms base) which is well above any
	// realistic retry policy.
	got := ScaleRetryDelay(1*time.Millisecond, "exponential", 100)
	want := time.Duration(1<<30) * time.Millisecond
	if got != want {
		t.Errorf("exponential clamp = %s, want %s", got, want)
	}
}
