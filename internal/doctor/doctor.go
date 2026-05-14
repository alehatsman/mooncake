package doctor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Options bundles user-facing flags from cmd/doctor.go. Zero values are
// valid; callers default Out/Err to os.Stdout/os.Stderr if nil.
type Options struct {
	Format      string   // "text" or "json"
	Strict      bool     // exit non-zero on warnings
	Sections    []string // filter to these sections (empty = all)
	SkipProject bool
	NoColor     bool
	Out         io.Writer
	Err         io.Writer
}

// Report is the output of one doctor run. Ordering of Results is
// deterministic (catalogue order, filtered by Options.Sections). Field
// names participate in the JSON contract; see spec-41 §"JSON output".
type Report struct {
	Cwd       string   `json:"cwd"`
	Ok        int      `json:"ok"`
	Warnings  int      `json:"warnings"`
	Errors    int      `json:"errors"`
	Infos     int      `json:"infos"`
	Results   []Result `json:"checks"`
	StartedAt string   `json:"started_at"`
	DurationMs int64   `json:"duration_ms"`
}

// HasErrors reports whether any check came back as StatusError.
func (r *Report) HasErrors() bool { return r.Errors > 0 }

// HasWarnings reports whether any check came back as StatusWarning.
func (r *Report) HasWarnings() bool { return r.Warnings > 0 }

// Run executes all registered checks honouring Options. It returns the
// assembled Report plus the exit code the CLI should use (0 on success or
// warnings-with-no-strict, 1 on errors, or warnings under --strict).
//
// Run never returns an error for "expected" outcomes — failures are surfaced
// inside the Report. A non-nil error indicates the doctor itself broke
// (e.g. cannot determine cwd).
func Run(opts Options) (*Report, int, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, 1, fmt.Errorf("cannot determine cwd: %w", err)
	}
	home, _ := os.UserHomeDir()
	homeDir := filepath.Join(home, ".mooncake")

	dctx := Context{
		Ctx:         context.Background(),
		HomeDir:     homeDir,
		Cwd:         cwd,
		SkipProject: opts.SkipProject,
		PresetPaths: presetSearchPaths(),
	}

	start := time.Now()
	checks := selectChecks(opts.Sections)
	results := make([]Result, 0, len(checks))
	for _, c := range checks {
		results = append(results, normaliseStatus(c.Run(dctx)))
	}

	rep := &Report{
		Cwd:        cwd,
		Results:    results,
		StartedAt:  start.UTC().Format(time.RFC3339),
		DurationMs: time.Since(start).Milliseconds(),
	}
	for _, r := range results {
		switch r.Status {
		case StatusOK:
			rep.Ok++
		case StatusInfo:
			rep.Infos++
		case StatusWarning:
			rep.Warnings++
		case StatusError:
			rep.Errors++
		}
	}

	exit := 0
	switch {
	case rep.HasErrors():
		exit = 1
	case opts.Strict && rep.HasWarnings():
		exit = 1
	}
	return rep, exit, nil
}

// normaliseStatus mirrors Status into the JSON-bound StatusS string field so
// the typed value and the wire-format token can't drift.
func normaliseStatus(r Result) Result {
	r.StatusS = r.Status.String()
	return r
}

// selectChecks returns the catalogue filtered to opts.Sections (empty list
// keeps everything) in deterministic catalogue order.
func selectChecks(sections []string) []Check {
	all := registeredChecks()
	if len(sections) == 0 {
		return all
	}
	allow := map[string]struct{}{}
	for _, s := range sections {
		allow[s] = struct{}{}
	}
	out := make([]Check, 0, len(all))
	for _, c := range all {
		if _, ok := allow[c.Section()]; ok {
			out = append(out, c)
		}
	}
	return out
}

// errUnreachable is returned by per-check probes when a context deadline
// fires. Translated into Status by the caller.
var errUnreachable = errors.New("probe timed out")

// withDeadline runs fn against a per-check timeout. Implementation lives in
// one place so checks don't each reinvent the dial pattern.
func withDeadline(parent context.Context, fn func(ctx context.Context) error) error {
	cctx, cancel := context.WithTimeout(parent, time.Duration(PerCheckTimeout)*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- fn(cctx) }()
	select {
	case err := <-done:
		return err
	case <-cctx.Done():
		return errUnreachable
	}
}
