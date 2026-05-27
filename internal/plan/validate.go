package plan

import (
	"errors"
	"fmt"
	"time"

	"github.com/alehatsman/mooncake/internal/facts"
)

// StaleReason identifies why a plan was rejected as stale at apply time.
// Returned via StaleError so callers can present specific messages or
// honor a typed --allow-stale override.
type StaleReason string

const (
	StaleReasonHostMismatch StaleReason = "host_mismatch"
	StaleReasonHashMismatch StaleReason = "input_files_changed"
	StaleReasonFileMissing  StaleReason = "input_file_missing"
	StaleReasonAgeExceeded  StaleReason = "max_age_exceeded"
)

// StaleError describes a stale-plan rejection. Callers compare Reason
// to StaleReason constants; the human Message is suitable for direct
// display.
type StaleError struct {
	Reason  StaleReason
	Message string
}

func (e *StaleError) Error() string { return e.Message }

// IsStaleError reports whether err is a stale-plan rejection.
func IsStaleError(err error) bool {
	var se *StaleError
	return errors.As(err, &se)
}

// ValidateOptions controls which checks ValidateForApply runs and what
// overrides the caller has explicitly enabled.
type ValidateOptions struct {
	// MaxAge, when non-zero, rejects plans older than this duration.
	MaxAge time.Duration
	// AllowStale, when true, demotes all stale-plan rejections to a
	// best-effort warning (returned as nil error). The caller is
	// responsible for logging the reasons separately if desired.
	AllowStale bool
}

// ValidateForApply is the convenience shim around
// ValidateForApplyWithReasons that drops the per-check reason list.
// Existing callers that only care about pass/fail keep working.
func ValidateForApply(p *Plan, opts ValidateOptions) error {
	_, err := ValidateForApplyWithReasons(p, opts)
	return err
}

// ValidateForApplyWithReasons checks that a plan loaded from disk is
// safe to apply against the current host and returns BOTH:
//
//   - reasons: every stale-check that would have rejected the plan,
//     populated regardless of AllowStale. Callers running with
//     `--allow-stale` use this to surface "we allowed apply despite
//     X / Y" so the operator sees what was overridden.
//   - err: the first *StaleError that fired (when AllowStale is
//     false), wrapped via standard `errors.Is/As`. nil when no
//     check failed OR when AllowStale demoted them all.
//
// Checks (in order):
//
//  1. The host facts subset (os_family, arch, distro_family) matches
//     the values captured at plan time.
//  2. The on-disk contents of every input file (root config + all
//     includes) hash to the value captured at plan time. Detects
//     unrelated edits to the YAML between plan and apply.
//  3. If opts.MaxAge is set, the plan must be younger than that.
//
// Hash I/O errors that aren't "file missing" (perm-denied, EIO, …)
// short-circuit immediately and return as the raw wrap — they
// aren't stale-plan conditions, they're system errors.
func ValidateForApplyWithReasons(p *Plan, opts ValidateOptions) ([]StaleReason, error) {
	var reasons []StaleReason
	var firstErr error
	record := func(se *StaleError) {
		reasons = append(reasons, se.Reason)
		if firstErr == nil && !opts.AllowStale {
			firstErr = se
		}
	}

	// 1. Host facts subset
	current := facts.Collect()
	want := p.GeneratedOn
	if want.OsFamily != "" || want.Arch != "" || want.DistroFamily != "" {
		if want.OsFamily != current.OS ||
			want.Arch != current.Arch ||
			want.DistroFamily != current.Distribution {
			record(&StaleError{
				Reason: StaleReasonHostMismatch,
				Message: fmt.Sprintf("plan was built on %s/%s/%s; applying on %s/%s/%s",
					want.OsFamily, want.Arch, want.DistroFamily,
					current.OS, current.Arch, current.Distribution),
			})
		}
	}

	// 2. Input-file hash
	if p.InputFilesHash != "" && len(p.InputFiles) > 0 {
		got, err := HashInputFiles(p.InputFiles)
		if err != nil {
			if errors.Is(err, ErrInputFileMissing) {
				record(&StaleError{
					Reason:  StaleReasonFileMissing,
					Message: err.Error(),
				})
			} else {
				// Non-stale I/O error short-circuits — it isn't a
				// stale-plan condition, it's a system fault.
				return reasons, fmt.Errorf("hash plan inputs: %w", err)
			}
		}
		if got != "" && got != p.InputFilesHash {
			record(&StaleError{
				Reason:  StaleReasonHashMismatch,
				Message: "plan input files have changed since the plan was built",
			})
		}
	}

	// 3. Age — MT-65: the comparison is strict (age > max), but
	// rounding the displayed age to whole seconds made a 1.005s plan
	// look identical to a 1s limit in the error message ("plan is 1s
	// old; --max-plan-age is 1s"). Show millisecond precision so the
	// operator can see the reported age really did exceed the
	// configured max.
	if opts.MaxAge > 0 && !p.GeneratedAt.IsZero() {
		age := time.Since(p.GeneratedAt)
		if age > opts.MaxAge {
			record(&StaleError{
				Reason: StaleReasonAgeExceeded,
				Message: fmt.Sprintf("plan is %s old; --max-plan-age is %s",
					age.Round(time.Millisecond), opts.MaxAge),
			})
		}
	}

	return reasons, firstErr
}
