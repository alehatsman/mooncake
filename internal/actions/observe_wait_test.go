package actions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
)

func TestWaitCondition(t *testing.T) {
	tests := []struct {
		until   string
		want    bool
		wantErr bool
	}{
		{"", true, false}, // default
		{"found", true, false},
		{"present", true, false},
		{"open", true, false},
		{"up", true, false},
		{"ready", true, false},
		{"running", true, false},
		{"gone", false, false},
		{"absent", false, false},
		{"closed", false, false},
		{"down", false, false},
		{"stopped", false, false},
		{"OPEN", true, false},    // case-insensitive
		{"  open ", true, false}, // trimmed
		{"maybe", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.until, func(t *testing.T) {
			got, err := WaitCondition(tt.until)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("WaitCondition(%q) accepted an unknown condition", tt.until)
				}
				return
			}
			if err != nil {
				t.Fatalf("WaitCondition(%q): %v", tt.until, err)
			}
			if got != tt.want {
				t.Errorf("WaitCondition(%q) = %v, want %v", tt.until, got, tt.want)
			}
		})
	}
}

func TestWaitCondition_ErrorNamesTheAlternatives(t *testing.T) {
	_, err := WaitCondition("eventually")
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"found", "gone"} {
		if !contains(err.Error(), want) {
			t.Errorf("error should list %q as an option; got: %v", want, err)
		}
	}
}

func TestValidateWait(t *testing.T) {
	t.Run("nil is fine", func(t *testing.T) {
		if err := ValidateWait("observe.port", nil); err != nil {
			t.Errorf("nil wait should validate: %v", err)
		}
	})
	t.Run("bad condition", func(t *testing.T) {
		if err := ValidateWait("observe.port", &config.WaitSpec{Until: "soon"}); err == nil {
			t.Error("expected an error for an unknown condition")
		}
	})
	t.Run("bad for", func(t *testing.T) {
		if err := ValidateWait("observe.port", &config.WaitSpec{For: "30 seconds"}); err == nil {
			t.Error("expected an error for an unparseable duration")
		}
	})
	t.Run("bad interval", func(t *testing.T) {
		if err := ValidateWait("observe.port", &config.WaitSpec{Interval: "often"}); err == nil {
			t.Error("expected an error for an unparseable interval")
		}
	})
	t.Run("valid", func(t *testing.T) {
		w := &config.WaitSpec{For: "30s", Until: "open", Interval: "500ms"}
		if err := ValidateWait("observe.port", w); err != nil {
			t.Errorf("valid wait rejected: %v", err)
		}
	})
}

func TestWaitTimings(t *testing.T) {
	tests := []struct {
		name         string
		spec         *config.WaitSpec
		wantBudget   time.Duration
		wantInterval time.Duration
	}{
		{"defaults", &config.WaitSpec{}, DefaultWaitFor, DefaultWaitInterval},
		{"explicit", &config.WaitSpec{For: "5s", Interval: "250ms"}, 5 * time.Second, 250 * time.Millisecond},
		// A typo like `interval: 1ms` must not turn a wait into a busy loop
		// hammering a remote endpoint.
		{"interval floored", &config.WaitSpec{Interval: "1ms"}, DefaultWaitFor, MinWaitInterval},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			budget, interval := waitTimings(tt.spec)
			if budget != tt.wantBudget {
				t.Errorf("budget = %v, want %v", budget, tt.wantBudget)
			}
			if interval != tt.wantInterval {
				t.Errorf("interval = %v, want %v", interval, tt.wantInterval)
			}
		})
	}
}

func TestObserveWait_ReturnsOnceSatisfied(t *testing.T) {
	ctx := newFakeCtx(context.Background())
	calls := 0
	probe := func() ObserveResult {
		calls++
		return ObserveResult{Found: calls >= 3, Value: calls}
	}

	spec := &config.WaitSpec{For: "5s", Interval: "1ms"}
	got, err := ObserveWait(ctx, "observe.port", "localhost:5432", spec, probe)
	if err != nil {
		t.Fatalf("ObserveWait: %v", err)
	}
	if !got.Found {
		t.Error("returned observation should be the satisfying one")
	}
	if calls != 3 {
		t.Errorf("probed %d times, want exactly 3 (must stop as soon as satisfied)", calls)
	}
}

func TestObserveWait_UntilGone(t *testing.T) {
	ctx := newFakeCtx(context.Background())
	calls := 0
	probe := func() ObserveResult {
		calls++
		return ObserveResult{Found: calls < 2}
	}

	spec := &config.WaitSpec{For: "5s", Interval: "1ms", Until: "gone"}
	got, err := ObserveWait(ctx, "observe.port", "localhost:5432", spec, probe)
	if err != nil {
		t.Fatalf("ObserveWait: %v", err)
	}
	if got.Found {
		t.Error("until:gone should return an observation with Found=false")
	}
}

// A timed-out wait must still hand back the last observation, so a downstream
// `as:` capture sees real data rather than a zero value.
func TestObserveWait_TimeoutReturnsLastObservation(t *testing.T) {
	ctx := newFakeCtx(context.Background())
	probe := func() ObserveResult {
		return ObserveResult{Found: false, Value: "never", Error: "connection refused"}
	}

	spec := &config.WaitSpec{For: "20ms", Interval: "1ms"}
	got, err := ObserveWait(ctx, "observe.port", "localhost:5432", spec, probe)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if got.Value != "never" {
		t.Errorf("timed-out wait dropped the last observation: %+v", got)
	}

	var timeoutErr *WaitTimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("expected *WaitTimeoutError, got %T", err)
	}
	if timeoutErr.Attempts < 1 {
		t.Errorf("Attempts = %d, want at least 1", timeoutErr.Attempts)
	}
	msg := err.Error()
	for _, want := range []string{"localhost:5432", "found", "connection refused"} {
		if !contains(msg, want) {
			t.Errorf("timeout message should mention %q; got: %s", want, msg)
		}
	}
}

// SIGINT during a five-minute wait must abort promptly, not run to the budget.
func TestObserveWait_HonorsCancellation(t *testing.T) {
	runCtx, cancel := context.WithCancel(context.Background())
	ctx := newFakeCtx(runCtx)

	calls := 0
	probe := func() ObserveResult {
		calls++
		if calls == 2 {
			cancel()
		}
		return ObserveResult{Found: false}
	}

	spec := &config.WaitSpec{For: "1h", Interval: "1ms"}
	start := time.Now()
	_, err := ObserveWait(ctx, "observe.port", "target", spec, probe)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("cancellation took %v; should abort promptly", elapsed)
	}
}

// The budget is a ceiling: a wait must not sleep a full interval past it.
func TestObserveWait_DoesNotOvershootBudget(t *testing.T) {
	ctx := newFakeCtx(context.Background())
	probe := func() ObserveResult { return ObserveResult{Found: false} }

	spec := &config.WaitSpec{For: "30ms", Interval: "10s"}
	start := time.Now()
	_, err := ObserveWait(ctx, "observe.port", "target", spec, probe)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if elapsed > 2*time.Second {
		t.Errorf("waited %v for a 30ms budget with a 10s interval; "+
			"the sleep must be clamped to the remaining budget", elapsed)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOfSub(s, sub) >= 0)
}

func indexOfSub(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// newFakeCtx builds the minimum actions.Context ObserveWait uses: a logger and
// a cancellable Ctx.
func newFakeCtx(ctx context.Context) Context {
	return &mockContext{ctx: ctx, log: &mockLogger{}}
}
