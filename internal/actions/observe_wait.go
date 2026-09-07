package actions

import (
	"fmt"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
)

// Defaults and bounds for the observe `wait:` modifier.
const (
	// DefaultWaitFor is the total budget when `for:` is unset. Matches what
	// the retired wait.* actions used, so migrated steps behave the same.
	DefaultWaitFor = 60 * time.Second

	// DefaultWaitInterval is the gap between attempts when `interval:` is unset.
	DefaultWaitInterval = time.Second

	// MinWaitInterval floors the poll gap. A typo like `interval: 1ms` would
	// otherwise turn a wait into a busy loop hammering a remote endpoint.
	MinWaitInterval = 100 * time.Millisecond
)

// waitConditions maps every accepted `until:` value onto the single thing a
// wait can poll: the handler's Found flag.
//
// The synonyms exist so a step reads like what it means — `until: open` for a
// port, `until: stopped` for a service — while staying exactly synonyms. There
// is one condition per observe handler, never a second condition language.
var waitConditions = map[string]bool{
	// Found == true
	"found":   true,
	"present": true,
	"open":    true,
	"up":      true,
	"ready":   true,
	"running": true,
	// Found == false
	"gone":    false,
	"absent":  false,
	"closed":  false,
	"down":    false,
	"stopped": false,
}

// WaitCondition resolves an `until:` value to the Found state being waited for.
// An empty value means the default, `found`.
func WaitCondition(until string) (bool, error) {
	if until == "" {
		return true, nil
	}
	want, ok := waitConditions[strings.ToLower(strings.TrimSpace(until))]
	if !ok {
		return false, fmt.Errorf(
			"unknown wait condition %q; use one of: %s", until, waitConditionNames())
	}
	return want, nil
}

// waitConditionNames renders the accepted `until:` values for an error message,
// grouped by what they mean so the reader sees the two real choices.
func waitConditionNames() string {
	return "found/present/open/up/ready/running (wait for it to appear), " +
		"gone/absent/closed/down/stopped (wait for it to disappear)"
}

// ValidateWait checks a `wait:` block without running it. Called from each
// handler's Validate so a bad condition or duration fails at plan time rather
// than sixty seconds into an apply.
func ValidateWait(actionName string, w *config.WaitSpec) error {
	if w == nil {
		return nil
	}
	if _, err := WaitCondition(w.Until); err != nil {
		return fmt.Errorf("%s.wait: %w", actionName, err)
	}
	if w.For != "" {
		if _, err := time.ParseDuration(w.For); err != nil {
			return fmt.Errorf("%s.wait: invalid for %q: %w", actionName, w.For, err)
		}
	}
	if w.Interval != "" {
		if _, err := time.ParseDuration(w.Interval); err != nil {
			return fmt.Errorf("%s.wait: invalid interval %q: %w", actionName, w.Interval, err)
		}
	}
	return nil
}

// waitTimings resolves the budget and poll gap, applying defaults and the
// interval floor. Assumes ValidateWait already accepted the strings.
func waitTimings(w *config.WaitSpec) (budget, interval time.Duration) {
	budget, interval = DefaultWaitFor, DefaultWaitInterval
	if w.For != "" {
		if d, err := time.ParseDuration(w.For); err == nil {
			budget = d
		}
	}
	if w.Interval != "" {
		if d, err := time.ParseDuration(w.Interval); err == nil {
			interval = d
		}
	}
	if interval < MinWaitInterval {
		interval = MinWaitInterval
	}
	return budget, interval
}

// WaitTimeoutError is returned when a `wait:` budget elapses without the
// condition holding. Typed so callers can distinguish "the thing never showed
// up" from "the probe itself broke".
type WaitTimeoutError struct {
	Action    string
	Target    string
	Until     string
	Budget    time.Duration
	Attempts  int
	LastError string
}

func (e *WaitTimeoutError) Error() string {
	until := e.Until
	if until == "" {
		until = "found"
	}
	msg := fmt.Sprintf("%s: waited %s for %s to be %s (%d attempts)",
		e.Action, e.Budget, e.Target, until, e.Attempts)
	if e.LastError != "" {
		msg += "; last probe error: " + e.LastError
	}
	return msg
}

// ObserveWait polls probe until the wait condition holds or the budget elapses.
//
// It always returns the LAST observation, satisfied or not, so an `as:` capture
// downstream of a timed-out wait still sees real data rather than a zero value.
// The error is non-nil exactly when the condition never held; the caller
// decides whether that fails the step (it does — see the spec).
//
// The context is checked between attempts, so SIGINT during a five-minute wait
// aborts promptly instead of running to the budget.
func ObserveWait(ctx Context, actionName, target string, w *config.WaitSpec,
	probe func() ObserveResult) (ObserveResult, error) {

	want, err := WaitCondition(w.Until)
	if err != nil {
		return ObserveResult{}, fmt.Errorf("%s.wait: %w", actionName, err)
	}
	budget, interval := waitTimings(w)

	ctx.Logger().Infof("Waiting up to %s for %s to be %s (every %s)",
		budget, target, orDefault(w.Until, "found"), interval)

	deadline := time.Now().Add(budget)
	attempts := 0
	var last ObserveResult

	for {
		attempts++
		last = probe()
		if last.Found == want {
			ctx.Logger().Debugf("%s: %s reached %s after %d attempt(s)",
				actionName, target, orDefault(w.Until, "found"), attempts)
			return last, nil
		}

		// Sleep only if another attempt would still fit inside the budget —
		// otherwise we'd burn a full interval past the deadline before giving up.
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		sleep := interval
		if sleep > remaining {
			sleep = remaining
		}

		select {
		case <-ctx.Ctx().Done():
			return last, ctx.Ctx().Err()
		case <-time.After(sleep):
		}
	}

	return last, &WaitTimeoutError{
		Action:    actionName,
		Target:    target,
		Until:     w.Until,
		Budget:    budget,
		Attempts:  attempts,
		LastError: last.Error,
	}
}

func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
