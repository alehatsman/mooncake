// Package read_common implements the shared core for the `read.json` and
// `read.yaml` tier-1 actions (spec-38). Both sibling handlers inject their
// format-specific decoder; everything else — path-resolution, bounded
// read, pathquery extraction, redaction, result population, plan-mode
// behavior — flows through Read.
package read_common

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/pathquery"
	"github.com/alehatsman/mooncake/internal/security"
)

// DefaultMaxBytes caps in-memory reads at 4 MiB by default (spec-38 Open Q1).
// Caller overrides via the YAML `max_bytes:` field.
const DefaultMaxBytes int64 = 4 << 20

// Opts is the validated, normalized input passed from the sibling handler.
type Opts struct {
	Path     string
	Query    string
	MaxBytes int64
	Redact   []*regexp.Regexp
	// Parse converts the file bytes into a generic value (any combination
	// of map[string]any, []any, scalars). One injected per format.
	Parse func([]byte, *any) error
	// FormatName is "JSON" or "YAML"; used only in error messages.
	FormatName string
}

// Output is the data block bound into Scope.Results under the step's `as:`.
type Output struct {
	Path      string `json:"path"`
	Query     string `json:"query,omitempty"`
	Found     bool   `json:"found"`
	Value     any    `json:"value,omitempty"`
	BytesRead int64  `json:"bytes_read"`
}

// Validate is the shared YAML-time validator. Sibling handlers call this
// from their own Validate(step) after dereferencing the action struct.
func Validate(rf *config.ReadFile, actionName string) error {
	if rf == nil {
		return fmt.Errorf("%s: configuration is nil", actionName)
	}
	if rf.Path == "" {
		return fmt.Errorf("%s: path is required", actionName)
	}
	if rf.MaxBytes != nil && *rf.MaxBytes <= 0 {
		return fmt.Errorf("%s: max_bytes must be > 0 (got %d)", actionName, *rf.MaxBytes)
	}
	if err := pathquery.Validate(rf.Query); err != nil {
		return fmt.Errorf("%s: query: %w", actionName, err)
	}
	for i, p := range rf.Redact {
		if _, err := regexp.Compile(p); err != nil {
			return fmt.Errorf("%s: redact[%d]: invalid regex %q: %w", actionName, i, p, err)
		}
	}
	return nil
}

// Read executes the validated read pipeline and returns the populated
// Output. Errors are structured for the caller to wrap into a Result.
func Read(ctx actions.Context, opts Opts) (Output, error) {
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return Output{}, fmt.Errorf("read_common: context is not an ExecutionContext")
	}

	resolved, err := ec.Svc.PathUtil.ExpandPath(opts.Path, ec.CurrentDir, ctx.Variables())
	if err != nil {
		return Output{}, fmt.Errorf("read_common: expand path: %w", err)
	}

	maxBytes := opts.MaxBytes
	if maxBytes == 0 {
		maxBytes = DefaultMaxBytes
	}

	data, n, err := readBounded(resolved, maxBytes)
	if err != nil {
		return Output{Path: resolved}, err
	}

	var parsed any
	if err := opts.Parse(data, &parsed); err != nil {
		return Output{Path: resolved, BytesRead: n}, fmt.Errorf("%s parse %s: %w", opts.FormatName, resolved, err)
	}

	value, found, qerr := pathquery.Extract(parsed, opts.Query)
	if qerr != nil {
		return Output{Path: resolved, Query: opts.Query, BytesRead: n}, fmt.Errorf("read_common: query: %w", qerr)
	}

	// Apply user-supplied redaction patterns to the extracted value. The
	// run's main redactor (sensitive values) already runs at log time; this
	// extra pass lets the YAML author tag a file's contents specifically.
	if len(opts.Redact) > 0 && found {
		r := security.NewRedactor()
		for _, re := range opts.Redact {
			// Already validated at Validate(); compile errors here would
			// be a programmer bug.
			if perr := r.AddPattern(re.String()); perr != nil {
				return Output{}, fmt.Errorf("read_common: redact: %w", perr)
			}
		}
		value = r.RedactValue(value)
	}

	return Output{
		Path:      resolved,
		Query:     opts.Query,
		Found:     found,
		Value:     value,
		BytesRead: n,
	}, nil
}

// readBounded reads at most limit+1 bytes from path so we can detect
// oversize files without fully consuming them. Returns (bytes, bytesRead, err).
func readBounded(path string, limit int64) ([]byte, int64, error) {
	f, err := os.Open(path) // #nosec G304 — path comes from user YAML by design
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, 0, fmt.Errorf("read: file not found: %s", path)
		}
		return nil, 0, fmt.Errorf("read: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, 0, fmt.Errorf("read: %s: %w", path, err)
	}
	read := int64(len(data))
	if read > limit {
		return nil, read, fmt.Errorf("read: %s exceeds max_bytes=%d (set explicit max_bytes: to allow)", path, limit)
	}
	return data, read, nil
}

// RunRead is the full Handler.Run wrapper shared by read.json and
// read.yaml: it compiles redact patterns, wires the per-format Parse
// + FormatName into Opts, drives Read, and populates a typed
// executor.Result (Data + plan-mode Reason). The sibling handlers are
// one-liners on top.
//
// Note: callers pass rf already dereferenced from the parent Step;
// Validate guarantees rf is non-nil by the time Run reaches this path.
func RunRead(ctx actions.Context, rf *config.ReadFile, parse func([]byte, *any) error, formatName string) (actions.Result, error) {
	patterns, err := CompileRedactPatterns(rf.Redact)
	if err != nil {
		return nil, err
	}
	maxBytes := int64(0)
	if rf.MaxBytes != nil {
		maxBytes = *rf.MaxBytes
	}

	result := executor.NewResult()
	result.Operation = executor.OpQuery
	result.Target = rf.Path
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	out, err := Read(ctx, Opts{
		Path:       rf.Path,
		Query:      rf.Query,
		MaxBytes:   maxBytes,
		Redact:     patterns,
		Parse:      parse,
		FormatName: formatName,
	})
	if err != nil {
		return result, err
	}

	result.SetData(map[string]any{
		"path":       out.Path,
		"query":      out.Query,
		"found":      out.Found,
		"value":      out.Value,
		"bytes_read": out.BytesRead,
	})
	result.Changed = false
	if ctx.Mode() == actions.ModePlan {
		result.Checkable = true
		if out.Found {
			result.Reason = fmt.Sprintf("would read %d bytes from %s", out.BytesRead, out.Path)
		} else {
			result.Reason = fmt.Sprintf("would read %d bytes from %s; query path missed", out.BytesRead, out.Path)
		}
	}
	return result, nil
}

// CompileRedactPatterns is a tiny shim used by handlers to pre-compile
// the user's redact-pattern slice for the Opts struct.
func CompileRedactPatterns(patterns []string) ([]*regexp.Regexp, error) {
	out := make([]*regexp.Regexp, 0, len(patterns))
	for i, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("redact[%d]: invalid regex %q: %w", i, p, err)
		}
		out = append(out, re)
	}
	return out, nil
}
