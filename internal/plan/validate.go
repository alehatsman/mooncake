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
	StaleReasonHostMismatch  StaleReason = "host_mismatch"
	StaleReasonHashMismatch  StaleReason = "input_files_changed"
	StaleReasonFileMissing   StaleReason = "input_file_missing"
	StaleReasonAgeExceeded   StaleReason = "max_age_exceeded"
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

// ValidateForApply checks that a plan loaded from disk is safe to
// apply against the current host:
//
//  1. The host facts subset (os_family, arch, distro_family) matches
//     the values captured at plan time.
//  2. The on-disk contents of every input file (root config + all
//     includes) hash to the value captured at plan time. Detects
//     unrelated edits to the YAML between plan and apply.
//  3. If opts.MaxAge is set, the plan must be younger than that.
//
// Returns nil if all checks pass (or AllowStale is set). Returns a
// *StaleError otherwise.
func ValidateForApply(p *Plan, opts ValidateOptions) error {
	// 1. Host facts subset
	current := facts.Collect()
	want := p.GeneratedOn
	if want.OsFamily != "" || want.Arch != "" || want.DistroFamily != "" {
		if want.OsFamily != current.OS ||
			want.Arch != current.Arch ||
			want.DistroFamily != current.Distribution {
			err := &StaleError{
				Reason: StaleReasonHostMismatch,
				Message: fmt.Sprintf("plan was built on %s/%s/%s; applying on %s/%s/%s",
					want.OsFamily, want.Arch, want.DistroFamily,
					current.OS, current.Arch, current.Distribution),
			}
			if !opts.AllowStale {
				return err
			}
		}
	}

	// 2. Input-file hash
	if p.InputFilesHash != "" && len(p.InputFiles) > 0 {
		got, err := HashInputFiles(p.InputFiles)
		if err != nil {
			if errors.Is(err, ErrInputFileMissing) {
				se := &StaleError{
					Reason:  StaleReasonFileMissing,
					Message: err.Error(),
				}
				if !opts.AllowStale {
					return se
				}
			} else {
				return fmt.Errorf("hash plan inputs: %w", err)
			}
		}
		if got != "" && got != p.InputFilesHash {
			se := &StaleError{
				Reason:  StaleReasonHashMismatch,
				Message: "plan input files have changed since the plan was built",
			}
			if !opts.AllowStale {
				return se
			}
		}
	}

	// 3. Age
	if opts.MaxAge > 0 && !p.GeneratedAt.IsZero() {
		age := time.Since(p.GeneratedAt)
		if age > opts.MaxAge {
			se := &StaleError{
				Reason: StaleReasonAgeExceeded,
				Message: fmt.Sprintf("plan is %s old; --max-plan-age is %s",
					age.Round(time.Second), opts.MaxAge),
			}
			if !opts.AllowStale {
				return se
			}
		}
	}

	return nil
}
