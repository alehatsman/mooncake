package executor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// TestScaleRetryDelay covers spec-69's promotion of MT-62's backoff
// strategies to a package-level helper. Mirror of the shell-package
// backoff_test.go that this replaces — sharing one source of truth
// keeps the curves coherent across handlers.
func TestScaleRetryDelay(t *testing.T) {
	base := 100 * time.Millisecond
	cases := []struct {
		name     string
		strategy string
		attempt  int
		want     time.Duration
	}{
		{"fixed default (empty strategy)", "", 1, base},
		{"fixed default (empty strategy), attempt 3", "", 3, base},
		{"fixed explicit", "fixed", 5, base},
		{"unknown strategy falls back to fixed", "weird", 2, base},
		{"linear attempt 1", "linear", 1, base},
		{"linear attempt 2", "linear", 2, 2 * base},
		{"linear attempt 5", "linear", 5, 5 * base},
		{"exponential attempt 1", "exponential", 1, base},
		{"exponential attempt 2", "exponential", 2, 2 * base},
		{"exponential attempt 4", "exponential", 4, 8 * base},
		// attempt clamped to >= 1 so a 0 or negative input doesn't
		// silently return a zero delay.
		{"attempt clamp at 0", "linear", 0, base},
		{"attempt clamp at -3", "linear", -3, base},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := scaleRetryDelay(base, tc.strategy, tc.attempt)
			if got != tc.want {
				t.Errorf("scaleRetryDelay(%v, %q, %d) = %v, want %v",
					base, tc.strategy, tc.attempt, got, tc.want)
			}
		})
	}
}

// TestScaleRetryDelay_ExponentialCap guards against overflow on absurd
// attempt counts. The shift is capped at 30 so the multiplier stays
// in int64 range; this test pins that cap.
func TestScaleRetryDelay_ExponentialCap(t *testing.T) {
	base := 1 * time.Nanosecond
	// At attempt=100 the natural multiplier (2^99) would overflow.
	// Cap kicks in at shift=30 → multiplier=2^30.
	got := scaleRetryDelay(base, "exponential", 100)
	want := base * time.Duration(1<<30)
	if got != want {
		t.Errorf("scaleRetryDelay exponential cap: got %v, want %v", got, want)
	}
}

// TestRunWithRetry_NoRetryReturnsOnFirstAttempt — step without a
// Retry policy means maxAttempts=1; one call, no loop.
func TestRunWithRetry_NoRetryReturnsOnFirstAttempt(t *testing.T) {
	step := &config.Step{Shell: &config.ShellAction{Cmd: "noop"}}
	calls := 0
	fakeErr := errors.New("first failure")

	_, err := runWithRetry(context.Background(), step, nil,
		func(attempt int) (actions.Result, error) {
			calls++
			return nil, fakeErr
		},
		nil,
	)

	if calls != 1 {
		t.Errorf("expected 1 attempt without Retry policy, got %d", calls)
	}
	if !errors.Is(err, fakeErr) {
		t.Errorf("expected raw err passthrough; got %v", err)
	}
}

// TestRunWithRetry_HonorsMaxAttempts — RetryAttempts:N means N+1
// total calls (1 original + N retries).
func TestRunWithRetry_HonorsMaxAttempts(t *testing.T) {
	step := &config.Step{
		Shell: &config.ShellAction{Cmd: "noop"},
		Retry: &config.RetryPolicy{Attempts: 3},
	}
	calls := 0
	_, err := runWithRetry(context.Background(), step, nil,
		func(attempt int) (actions.Result, error) {
			calls++
			return nil, fmt.Errorf("attempt %d failed", attempt)
		},
		nil,
	)
	if calls != 4 {
		t.Errorf("expected 4 calls (1 + 3 retries), got %d", calls)
	}
	if err == nil {
		t.Fatal("expected final err after all retries failed")
	}
}

// TestRunWithRetry_WrapsAfterNAttempts — when retry was configured
// and every attempt failed, the returned err wraps the count in a
// "step failed after N attempts" prefix. The message used to say
// "command failed" pre-F053 (when this helper lived in shell/
// backoff.go); the new phrasing is action-agnostic since spec-69
// phase 2 promoted the helper across all retryable actions.
// Assertion is on the "after N attempts" tail, not the prefix, so
// the test survives further message tweaks.
func TestRunWithRetry_WrapsAfterNAttempts(t *testing.T) {
	step := &config.Step{
		Shell: &config.ShellAction{Cmd: "noop"},
		Retry: &config.RetryPolicy{Attempts: 2},
	}
	_, err := runWithRetry(context.Background(), step, nil,
		func(attempt int) (actions.Result, error) {
			return nil, fmt.Errorf("nope")
		},
		nil,
	)
	if err == nil {
		t.Fatal("expected err")
	}
	if !strings.Contains(err.Error(), "after") {
		t.Errorf("want 'after N attempts' in err message; got %v", err)
	}
}

// TestRunWithRetry_StopsOnSuccess — first nil-err attempt is the
// terminal one. No further attempts even when more were budgeted.
func TestRunWithRetry_StopsOnSuccess(t *testing.T) {
	step := &config.Step{
		Shell: &config.ShellAction{Cmd: "noop"},
		Retry: &config.RetryPolicy{Attempts: 5},
	}
	calls := 0
	_, err := runWithRetry(context.Background(), step, nil,
		func(attempt int) (actions.Result, error) {
			calls++
			if attempt >= 2 {
				return nil, nil
			}
			return nil, fmt.Errorf("transient %d", attempt)
		},
		nil,
	)
	if err != nil {
		t.Errorf("expected nil err after success; got %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls (1 failure + 1 success); got %d", calls)
	}
}

// TestRunWithRetry_NonRetryableErrBreaks — when isRetryable says no,
// the loop bails after the first failed attempt without sleeping.
func TestRunWithRetry_NonRetryableErrBreaks(t *testing.T) {
	step := &config.Step{
		Shell: &config.ShellAction{Cmd: "noop"},
		Retry: &config.RetryPolicy{Attempts: 5, Delay: "10s"}, // long delay we shouldn't sleep
	}
	calls := 0
	start := time.Now()
	_, err := runWithRetry(context.Background(), step, nil,
		func(attempt int) (actions.Result, error) {
			calls++
			return nil, fmt.Errorf("permanent")
		},
		func(actions.Result, error) bool { return false },
	)
	elapsed := time.Since(start)

	if calls != 1 {
		t.Errorf("non-retryable err should break after first attempt; got %d calls", calls)
	}
	if err == nil {
		t.Fatal("expected err")
	}
	// We never reach the sleep branch; elapsed should be well under
	// the 10s delay.
	if elapsed > 1*time.Second {
		t.Errorf("isRetryable=false should have skipped the sleep; elapsed=%v", elapsed)
	}
}

// TestRunWithRetry_BackoffHonored — MT-62 invariant: scaleRetryDelay
// is consulted with the configured strategy. The linear path waits
// base, then 2·base; exponential waits base, then 2·base, then 4·base.
//
// We can't easily count sleeps, so we measure wall clock with a small
// base. A 3-attempt linear schedule with 50ms base gives ~50 + ~100ms
// = ~150ms of sleeping minimum. A 4-attempt exponential gives ~50 +
// ~100 + ~200ms = ~350ms.
//
// Loose lower bounds (≥150ms / ≥350ms) keep this hermetic on slow CI
// without false-pinning the exact curve.
func TestRunWithRetry_BackoffHonored(t *testing.T) {
	if testing.Short() {
		t.Skip("wall-clock dependent")
	}
	t.Run("linear", func(t *testing.T) {
		step := &config.Step{
			Shell: &config.ShellAction{Cmd: "noop"},
			Retry: &config.RetryPolicy{Attempts: 2, Delay: "50ms", Backoff: "linear"},
		}
		start := time.Now()
		_, _ = runWithRetry(context.Background(), step, nil,
			func(int) (actions.Result, error) { return nil, fmt.Errorf("fail") },
			nil,
		)
		elapsed := time.Since(start)
		// 2 retries with linear: sleeps of 50ms + 100ms = 150ms.
		if elapsed < 140*time.Millisecond {
			t.Errorf("linear backoff didn't fire; elapsed=%v want >= 140ms", elapsed)
		}
	})

	t.Run("exponential", func(t *testing.T) {
		step := &config.Step{
			Shell: &config.ShellAction{Cmd: "noop"},
			Retry: &config.RetryPolicy{Attempts: 3, Delay: "50ms", Backoff: "exponential"},
		}
		start := time.Now()
		_, _ = runWithRetry(context.Background(), step, nil,
			func(int) (actions.Result, error) { return nil, fmt.Errorf("fail") },
			nil,
		)
		elapsed := time.Since(start)
		// 3 retries with exponential: sleeps of 50 + 100 + 200ms = 350ms.
		if elapsed < 340*time.Millisecond {
			t.Errorf("exponential backoff didn't fire; elapsed=%v want >= 340ms", elapsed)
		}
	})

	t.Run("fixed (default)", func(t *testing.T) {
		step := &config.Step{
			Shell: &config.ShellAction{Cmd: "noop"},
			Retry: &config.RetryPolicy{Attempts: 3, Delay: "50ms"},
		}
		start := time.Now()
		_, _ = runWithRetry(context.Background(), step, nil,
			func(int) (actions.Result, error) { return nil, fmt.Errorf("fail") },
			nil,
		)
		elapsed := time.Since(start)
		// 3 retries fixed: 50ms × 3 = 150ms.
		if elapsed < 140*time.Millisecond {
			t.Errorf("fixed delay didn't fire; elapsed=%v want >= 140ms", elapsed)
		}
		// Upper-ish bound: should be well under the exponential curve
		// even on a slow CI (would be ~350ms+ if backoff misfired).
		if elapsed > 500*time.Millisecond {
			t.Errorf("fixed delay slept too long (linear or exponential leak?); elapsed=%v", elapsed)
		}
	})
}

// TestRunWithRetry_BadDelayParseFallsThrough — an unparseable
// retry_delay logs but doesn't sleep and doesn't break the retry
// loop. Matches the pre-spec-69 shell behavior; callers shouldn't be
// punished for a malformed duration string beyond losing the inter-
// attempt pause.
func TestRunWithRetry_BadDelayParseFallsThrough(t *testing.T) {
	step := &config.Step{
		Shell: &config.ShellAction{Cmd: "noop"},
		Retry: &config.RetryPolicy{Attempts: 2, Delay: "not-a-duration"},
	}
	calls := 0
	start := time.Now()
	_, err := runWithRetry(context.Background(), step, nil,
		func(int) (actions.Result, error) {
			calls++
			return nil, fmt.Errorf("fail")
		},
		nil,
	)
	elapsed := time.Since(start)
	if calls != 3 {
		t.Errorf("unparseable delay should not abort retries; got %d calls", calls)
	}
	if err == nil {
		t.Fatal("expected err after all retries failed")
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("unparseable delay should result in ~0 wait; elapsed=%v", elapsed)
	}
}

// TestRunWithRetry_RetryableSeesResult — confirms the spec-69 phase
// 3 widening: Retryable.IsRetryable receives both result and err so a
// handler like the eventual http.request migration can branch on
// status code carried in result.Data without inventing typed errors.
func TestRunWithRetry_RetryableSeesResult(t *testing.T) {
	step := &config.Step{
		Shell: &config.ShellAction{Cmd: "noop"},
		Retry: &config.RetryPolicy{Attempts: 3},
	}
	calls := 0
	var sawResult actions.Result
	var sawErr error
	_, _ = runWithRetry(context.Background(), step, nil,
		func(attempt int) (actions.Result, error) {
			calls++
			r := NewResult()
			r.SetData(map[string]interface{}{"status_code": 503})
			return r, fmt.Errorf("synthetic 5xx attempt %d", attempt)
		},
		func(res actions.Result, err error) bool {
			sawResult = res
			sawErr = err
			// Retry while we see a 503; stop on any other status.
			if r, ok := res.(*Result); ok {
				if sc, ok := r.Data["status_code"].(int); ok && sc == 503 {
					return true
				}
			}
			return false
		},
	)
	if sawResult == nil {
		t.Fatal("isRetryable was never called with a non-nil result")
	}
	if sawErr == nil {
		t.Fatal("isRetryable was never called with a non-nil err")
	}
	if calls != 4 {
		t.Errorf("expected 4 calls (1 + 3 retries since isRetryable kept returning true); got %d", calls)
	}
}

// TestRunWithRetry_CtxCancelDuringDelay is the F053 regression: a
// ctx cancelled mid-sleep must abort `runWithRetry` immediately
// (~10 ms tolerance for goroutine scheduling), NOT block the full
// retry delay. Pre-fix, `time.Sleep(delay)` was uncancellable and
// the call sat unresponsive for the entire 30 s delay; that's the
// UX cliff F053 closes for every spec-69-migrated action (shell/
// cmd/download/http_request/package/os_user/os_cron/pkg_upgrade).
//
// We pin the contract three ways:
//  1. Elapsed time stays well below the configured delay.
//  2. The returned err is ctx.Err() (so callers can distinguish
//     "cancelled" from "all attempts failed").
//  3. No further attemptFn calls happen after the cancel — the
//     loop drops out instead of running the next attempt.
func TestRunWithRetry_CtxCancelDuringDelay(t *testing.T) {
	const delay = 30 * time.Second
	step := &config.Step{
		Shell: &config.ShellAction{Cmd: "noop"},
		Retry: &config.RetryPolicy{Attempts: 3, Delay: delay.String()},
	}
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	// Cancel ctx ~20 ms after the first attempt fails — well before
	// the 30 s sleep would expire. The retry loop must observe the
	// cancellation via sleepCtx's select and return ctx.Err().
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := runWithRetry(ctx, step, nil,
		func(attempt int) (actions.Result, error) {
			calls++
			return nil, errors.New("simulated failure")
		},
		nil,
	)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled (so callers can distinguish cancel from failure)", err)
	}
	if elapsed > time.Second {
		t.Errorf("elapsed = %v, want < 1s — ctx cancel must abort sleep, not wait for the %s delay", elapsed, delay)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 — cancelled retry must not invoke attemptFn again", calls)
	}
}

// TestSleepCtx_TimerPath covers the non-cancelled branch: when the
// timer fires first, sleepCtx returns nil and the loop continues.
func TestSleepCtx_TimerPath(t *testing.T) {
	start := time.Now()
	if err := sleepCtx(context.Background(), 30*time.Millisecond); err != nil {
		t.Errorf("sleepCtx returned err on timer path: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 25*time.Millisecond {
		t.Errorf("sleepCtx returned too early: %v < 25ms", elapsed)
	}
}

// TestSleepCtx_NilCtx covers the defensive fallback for callers
// that construct a `RunServices` without `Ctx` (tests, mostly). A
// nil ctx degrades to a plain `time.Sleep` — not a documented
// production contract, just paranoia so the test surface keeps
// working without retrofitting every fake RunServices.
func TestSleepCtx_NilCtx(t *testing.T) {
	start := time.Now()
	if err := sleepCtx(nil, 20*time.Millisecond); err != nil {
		t.Errorf("nil ctx should degrade to time.Sleep, got err: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 15*time.Millisecond {
		t.Errorf("nil-ctx fallback returned too early: %v < 15ms", elapsed)
	}
}
