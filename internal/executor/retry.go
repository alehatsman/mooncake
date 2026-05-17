package executor

import (
	"fmt"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/logger"
)

// scaleRetryDelay applies a retry backoff strategy to the base delay.
// attempt is 1-indexed: 1 is the first retry's sleep, 2 the second, etc.
//
// Strategies:
//   - "fixed"       — every retry waits `base` (the default; back-compat)
//   - "linear"      — wait is base * attempt → base, 2·base, 3·base, …
//   - "exponential" — wait is base * 2^(attempt-1) → base, 2·base, 4·base, …
//
// Unknown strategies fall back to "fixed" so a typo doesn't silently
// turn into a different sleep curve. Validation against the enum
// happens at parse time in internal/config.
//
// Originally lived in internal/actions/shell/backoff.go; promoted to
// the executor package by spec-69 phase 2 so every action can use it
// uniformly (MT-62 only fixed the shell handler; command, download,
// http_request all still had the bare-delay bug pre-promotion).
func scaleRetryDelay(base time.Duration, strategy string, attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	switch strategy {
	case "linear":
		return base * time.Duration(attempt)
	case "exponential":
		shift := attempt - 1
		// Cap to keep the multiplier from overflowing on absurd inputs.
		if shift > 30 {
			shift = 30
		}
		return base * time.Duration(1<<shift)
	default: // "fixed" or unknown
		return base
	}
}

// runWithRetry executes a single-attempt runner under the step's
// retry policy (attempts + delay + backoff). Returns the final
// result/err — no override application here; finalizeOverrides
// runs once after the loop in the caller.
//
// MT-48 invariant: the decision to retry is based on the *raw* err
// returned by attemptFn, never on a post-failed_when verdict. This
// is the whole reason retry+override centralization belongs in the
// executor: when both lived in each handler, MT-48 had to be fixed
// twice (shell + command) and MT-62 was fixed in only one of those
// two near-duplicate loops.
//
// attemptFn returns (result, err). err == nil → success; we stop
// retrying and return. err != nil + isRetryable(err) → keep
// retrying. err != nil + !isRetryable(err) → fail fast.
func runWithRetry(
	step *config.Step,
	log logger.Logger,
	attemptFn func(attempt int) (actions.Result, error),
	isRetryable func(error) bool,
) (actions.Result, error) {
	maxAttempts := step.RetryAttempts() + 1
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var lastResult actions.Result
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 && log != nil {
			log.Debugf("  Retry attempt %d/%d", attempt-1, step.RetryAttempts())
		}

		result, err := attemptFn(attempt)
		if err == nil {
			return result, nil
		}

		lastResult = result
		lastErr = err

		if isRetryable != nil && !isRetryable(err) {
			break
		}

		if attempt < maxAttempts && step.RetryDelayDuration() != "" {
			base, parseErr := time.ParseDuration(step.RetryDelayDuration())
			if parseErr != nil {
				if log != nil {
					log.Debugf("  Invalid retry_delay %q: %v", step.RetryDelayDuration(), parseErr)
				}
			} else {
				delay := scaleRetryDelay(base, step.RetryBackoffStrategy(), attempt)
				if log != nil {
					log.Debugf("  Waiting %s before retry (backoff=%s)...", delay, step.RetryBackoffStrategy())
				}
				time.Sleep(delay)
			}
		}
	}

	// All attempts failed; wrap with attempt count if retry was
	// configured. Matches shell/command's pre-spec-69 message.
	if step.RetryAttempts() > 0 {
		return lastResult, fmt.Errorf("command failed after %d attempts: %w", maxAttempts, lastErr)
	}
	return lastResult, lastErr
}
