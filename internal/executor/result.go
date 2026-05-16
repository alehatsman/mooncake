package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"time"
)

// Result represents the outcome of executing a step and can be registered
// to variables for use in subsequent steps via the "register" field.
//
// Field usage varies by step type:
//
// Shell steps:
//   - Stdout: captured standard output from the command
//   - Stderr: captured standard error from the command
//   - Rc: exit code (0 for success, non-zero for failure)
//   - Failed: true if Rc != 0
//   - Changed: always true (commands are assumed to make changes)
//
// File steps (file with state: file or directory):
//   - Rc: 0 for success, 1 for failure
//   - Failed: true if file/directory operation failed
//   - Changed: true if file/directory was created or content modified
//
// Template steps:
//   - Rc: 0 for success, 1 for failure
//   - Failed: true if template rendering or file write failed
//   - Changed: true if output file was created or content changed
//
// Variable steps (vars, include_vars):
//   - All fields remain at default values (not currently used)
//
// The Skipped field is reserved for future use but not currently set by any step type.
type Result struct {
	// Stdout contains the standard output from shell commands.
	// Only populated by shell steps.
	Stdout string `json:"stdout"`

	// Stderr contains the standard error from shell commands.
	// Only populated by shell steps.
	Stderr string `json:"stderr"`

	// Rc is the return/exit code.
	// For shell steps: the command's exit code (0 = success).
	// For file/template steps: 0 for success, 1 for failure.
	Rc int `json:"rc"`

	// Failed indicates whether the step execution failed.
	// Set to true when shell commands exit non-zero or when file/template operations error.
	Failed bool `json:"failed"`

	// Changed indicates whether the step made modifications to the system.
	// Shell steps: always true (commands assumed to make changes).
	// File steps: true if file/directory was created or modified.
	// Template steps: true if output file was created or content changed.
	Changed bool `json:"changed"`

	// WouldChange indicates that a plan-mode (non-mutating) inspection
	// predicts the step would change the system if executed.
	// Set by handlers running in ModePlan (Spec 16). Mirrors today's
	// CheckResult.WouldChange and is the eventual replacement.
	WouldChange bool `json:"would_change,omitempty"`

	// Reason is a short human-readable description of the result, e.g.
	// "would create directory", "already matches", "content differs".
	// Populated alongside WouldChange and Changed.
	Reason string `json:"reason,omitempty"`

	// Checkable indicates whether the action supports plan-mode inspection.
	// False for actions like shell where no prediction is possible (the
	// command must run to know its effect). Set by handlers in ModePlan.
	Checkable bool `json:"checkable,omitempty"`

	// Skipped is reserved for future use to indicate skipped steps.
	// Currently not set by any step type.
	Skipped bool `json:"skipped"`

	// Data holds custom result data set by actions via SetData.
	// This allows actions to provide additional structured information
	// that can be accessed in templates and registered results.
	Data map[string]interface{} `json:"data,omitempty"`

	// Detail holds action-specific plan-mode data (e.g. effects.ContentDiff).
	// Only populated in ModePlan. Not serialised into registered results.
	Detail any `json:"-"`

	// AppliedDiff is the typed actions.Diff captured at apply time for
	// handlers that implement actions.Differ. The diff is computed
	// just before runner.Run executes, so it represents what the
	// handler intends to change (Before / After of the predicted
	// mutation). nil for handlers that don't implement Differ or for
	// plan-mode (where the diff already flows through the StepChecked
	// event). Lives on Result so the apply.captureSubscriber can
	// thread it into the spec-68 runlog StepEntry — typed Diff per
	// step lets `mooncake explain r/<id>` and `explain <resource>`
	// show what each step did, not just that it ran.
	//
	// Kept as `any` (rather than *actions.Diff) to keep the executor
	// package free of an actions.Differ-payload type dependency; the
	// apply layer narrows it when projecting to runlog.
	AppliedDiff any `json:"-"`

	// ReverseData holds action-specific apply-time state captured for
	// later use by the handler's Reverse() method (spec-22 phase 5).
	// Typically populated in ModeApply BEFORE the mutation runs so
	// post-apply Reverse can construct an inverse step from the
	// pre-state. Parallel to Detail rather than overloading it —
	// Detail is plan-time output (ContentDiff for `--diff`),
	// ReverseData is apply-time capture, distinct lifetimes and
	// consumers.
	//
	// Wire encoding (R2.1c phase 2): the field is serialised through
	// a discriminator envelope (`{"type": "<go-type-name>", "data":
	// {...}}`) so it round-trips through the agentd /v1/runs/{id}/
	// result endpoint and fleet-wide Reverse() can compose against
	// the captured pre-state of each peer. The default `json:"-"` is
	// gone — Result.MarshalJSON / UnmarshalJSON handle the envelope
	// explicitly via the registry in reverse_registry.go.
	ReverseData any `json:"-"`

	// Timing information
	StartTime time.Time     `json:"start_time,omitempty"`
	EndTime   time.Time     `json:"end_time,omitempty"`
	Duration  time.Duration `json:"duration_ms,omitempty"` // Duration in time.Duration format
}

// NewResult creates a new Result with default values.
func NewResult() *Result {
	return &Result{
		Stdout:  "",
		Stderr:  "",
		Rc:      0,
		Failed:  false,
		Changed: false,
		Skipped: false,
	}
}

// Status returns a string representation of the result status.
func (r *Result) Status() string {
	if r.Failed {
		return "failed"
	}
	if r.Skipped {
		return "skipped"
	}
	if r.Changed {
		return "changed"
	}
	return "ok"
}

// ToMap converts Result to a map for use in template variables.
func (r *Result) ToMap() map[string]interface{} {
	m := map[string]interface{}{
		"stdout":      r.Stdout,
		"stderr":      r.Stderr,
		"rc":          r.Rc,
		"failed":      r.Failed,
		"changed":     r.Changed,
		"skipped":     r.Skipped,
		"duration_ms": r.Duration.Milliseconds(),
		"status":      r.Status(),
	}

	// Merge custom data fields into the map
	if r.Data != nil {
		for k, v := range r.Data {
			m[k] = v
		}
	}

	return m
}

// RegisterTo registers this result to the variables map under the given name.
// The result can be accessed using nested field syntax (e.g., "result.stdout", "result.rc") in templates and when conditions.
func (r *Result) RegisterTo(variables map[string]interface{}, name string) {
	variables[name] = r.ToMap()
}

// RegisteredResult is a snapshot of a Result stored in VariableScope.Results.
// It is a flat copy — no pointer aliasing — so the scope can be safely cloned.
type RegisteredResult struct {
	Stdout     string
	Stderr     string
	Rc         int
	Failed     bool
	Changed    bool
	Skipped    bool
	DurationMs int64
	Data       map[string]interface{}
}

// ToRegisteredResult converts a *Result into a RegisteredResult snapshot.
func (r *Result) ToRegisteredResult() RegisteredResult {
	var data map[string]interface{}
	if r.Data != nil {
		data = make(map[string]interface{}, len(r.Data))
		for k, v := range r.Data {
			data[k] = v
		}
	}
	return RegisteredResult{
		Stdout:     r.Stdout,
		Stderr:     r.Stderr,
		Rc:         r.Rc,
		Failed:     r.Failed,
		Changed:    r.Changed,
		Skipped:    r.Skipped,
		DurationMs: r.Duration.Milliseconds(),
		Data:       data,
	}
}

// ToMap converts a RegisteredResult to map[string]interface{} for template engines.
func (r RegisteredResult) ToMap() map[string]interface{} {
	m := map[string]interface{}{
		"stdout":      r.Stdout,
		"stderr":      r.Stderr,
		"rc":          r.Rc,
		"failed":      r.Failed,
		"changed":     r.Changed,
		"skipped":     r.Skipped,
		"duration_ms": r.DurationMs,
	}
	for k, v := range r.Data {
		m[k] = v
	}
	return m
}

// --- actions.Result interface implementation ---
// These methods allow Result to be used as an actions.Result,
// avoiding circular import dependencies between executor and actions packages.

// SetChanged marks whether the action made changes.
func (r *Result) SetChanged(changed bool) {
	r.Changed = changed
}

// SetStdout sets the stdout output.
func (r *Result) SetStdout(stdout string) {
	r.Stdout = stdout
}

// SetStderr sets the stderr output.
func (r *Result) SetStderr(stderr string) {
	r.Stderr = stderr
}

// SetFailed marks the result as failed.
func (r *Result) SetFailed(failed bool) {
	r.Failed = failed
	if failed {
		r.Rc = 1
	}
}

// SetData sets custom result data.
// This merges the provided data into the result's ToMap output,
// allowing actions to provide additional structured information.
func (r *Result) SetData(data map[string]interface{}) {
	if r.Data == nil {
		r.Data = make(map[string]interface{})
	}
	for k, v := range data {
		r.Data[k] = v
	}
}

// --- R2.1c phase 2: ReverseData wire encoding ------------------------------

// reverseDataEnvelope is the wire shape for a typed ReverseData
// payload. Type is the discriminator registered via
// RegisterReverseDataType (conventionally the Go type name, e.g.
// "FileReverseInfo"); Data is the raw JSON of the concrete payload.
// Keeping Data as json.RawMessage so the second-pass unmarshal into
// the concrete type doesn't re-allocate or re-decode the wrapping
// envelope.
type reverseDataEnvelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// resultJSON is a type alias of Result that strips method receivers
// (specifically MarshalJSON/UnmarshalJSON below) so the default
// encoder can serialise every other field via its json tag without
// infinite recursion. ReverseData on the alias keeps its `json:"-"`
// — the envelope is injected explicitly in MarshalJSON / read
// explicitly in UnmarshalJSON.
type resultJSON Result

// resultWire is the on-the-wire encoded shape. Same field list as
// Result via the alias, plus the discriminator envelope for
// ReverseData. Using embedding (rather than a flat re-declaration)
// keeps the tag-keyed fields in lockstep with Result automatically
// — adding a future field to Result that should round-trip just
// works via the alias.
type resultWire struct {
	resultJSON
	ReverseData *reverseDataEnvelope `json:"reverse_data,omitempty"`
}

// MarshalJSON serialises Result with ReverseData wrapped in a
// discriminator envelope when populated. The discriminator is the
// payload's Go type name (e.g. "FileReverseInfo"); the receiver
// must be a pointer to a registered type so the decode side can
// look it up.
//
// When ReverseData is nil, the output matches the pre-phase-2
// shape — no `reverse_data` key at all. Old daemons / clients that
// don't know about the field are unaffected on the read side, and
// new code that consumes them sees `nil` ReverseData (which the
// existing "ReverseData is nil" refusal in handlers' Reverse()
// already handles).
func (r *Result) MarshalJSON() ([]byte, error) {
	w := resultWire{resultJSON: resultJSON(*r)}
	if r.ReverseData != nil {
		typeName := reverseDataTypeName(r.ReverseData)
		if typeName == "" {
			return nil, fmt.Errorf("executor.Result.MarshalJSON: cannot derive type name for ReverseData %T (must be a pointer to a named struct)", r.ReverseData)
		}
		raw, err := json.Marshal(r.ReverseData)
		if err != nil {
			return nil, fmt.Errorf("executor.Result.MarshalJSON: marshal ReverseData: %w", err)
		}
		w.ReverseData = &reverseDataEnvelope{Type: typeName, Data: raw}
	}
	return json.Marshal(w)
}

// UnmarshalJSON deserialises Result, looking up the ReverseData
// payload type via the registry and materialising the concrete type
// when known. Unknown discriminators decode to nil ReverseData —
// forward compatibility for newer daemons whose handler types this
// binary doesn't know about. Returns an error only when the
// envelope itself is malformed or the concrete type's Unmarshal
// fails (the latter signals real corruption, not unknown types).
func (r *Result) UnmarshalJSON(b []byte) error {
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		return nil
	}
	var w resultWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	*r = Result(w.resultJSON)
	if w.ReverseData == nil || w.ReverseData.Type == "" {
		return nil
	}
	target, ok := newReverseData(w.ReverseData.Type)
	if !ok {
		// Unknown type — leave ReverseData nil. Handlers'
		// Reverse() will surface "no ReverseData captured" and the
		// transaction layer treats that the same as a pre-phase-2
		// peer. Logging is intentionally absent: a fleet of mixed
		// versions would spam.
		return nil
	}
	if err := json.Unmarshal(w.ReverseData.Data, target); err != nil {
		return fmt.Errorf("executor.Result.UnmarshalJSON: decode ReverseData payload %q: %w", w.ReverseData.Type, err)
	}
	r.ReverseData = target
	return nil
}

// reverseDataTypeName returns the Go type name of a ReverseData
// payload. Convention: payloads are passed as pointers to named
// structs (e.g. `*FileReverseInfo`), so we strip one indirection
// and read the element's Name(). Returns "" when the input isn't
// a pointer to a named type — MarshalJSON treats that as an error.
func reverseDataTypeName(v any) string {
	t := reflect.TypeOf(v)
	if t == nil {
		return ""
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Name()
}
