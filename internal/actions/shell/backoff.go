package shell

import "time"

// ScaleRetryDelay applies a retry backoff strategy to the base delay.
// attempt is 1-indexed: 1 is the first retry's sleep, 2 the second, etc.
//
// Strategies:
//   - "fixed"       — every retry waits `base` (the default; back-compat)
//   - "linear"      — wait is base * attempt → base, 2·base, 3·base, …
//   - "exponential" — wait is base * 2^(attempt-1) → base, 2·base, 4·base, …
//
// Unknown strategies fall back to "fixed" so a typo doesn't silently
// turn into a different sleep curve. Validation against the enum
// happens at parse time elsewhere.
func ScaleRetryDelay(base time.Duration, strategy string, attempt int) time.Duration {
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
