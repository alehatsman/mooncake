package shell

import (
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// TestRetry_TriggeredEvenWhenFailedWhenIsFalse guards MT-48. The
// pre-fix behavior evaluated failed_when before the retry decision,
// so `retry: { attempts: 3 } + failed_when: false` masked the first
// failure → retry loop saw "success" → step exited after a single
// attempt instead of retrying.
//
// We can't easily count attempts from inside the test without
// running real commands, but a 600ms+ wall clock under three failing
// retries with a 200ms delay is a strong-enough proxy: only ~92ms
// would elapse if the retry skipped.
func TestRetry_TriggeredEvenWhenFailedWhenIsFalse(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	step := &config.Step{
		Shell:       &config.ShellAction{Cmd: "exit 1"},
		Retry:       &config.RetryPolicy{Attempts: 3, Delay: "200ms"},
		FailedWhen:  "false",
	}

	start := time.Now()
	res, err := h.Execute(ctx, step)
	elapsed := time.Since(start)

	// failed_when: false masks the final failure, so Execute should
	// return without an error.
	if err != nil {
		t.Errorf("failed_when: false should mask final failure; got err = %v", err)
	}
	if r, ok := res.(*executor.Result); ok && r.Failed {
		t.Errorf("Failed should be false after failed_when mask; result = %+v", r)
	}

	// The retry loop ran (3 retries × 200ms delay = ~600ms wall clock).
	// Before MT-48 the elapsed time was ~92ms — a single attempt with
	// no retries because failed_when masked the very first failure.
	if elapsed < 500*time.Millisecond {
		t.Errorf("retries didn't happen — elapsed=%v, want >= 500ms for 3 retries × 200ms", elapsed)
	}
}
