package executor

import (
	"fmt"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/metrics"
)

// resultOrigin tracks who wrote a given Results key, for the spec-37
// collision warning. Stored alongside Scope.Results in a parallel map so
// the value-shape exposed to template consumers stays unchanged.
type resultOrigin struct {
	// StepID is the writer step's ID at write time (e.g. "step-0042").
	StepID string
	// StepName is the user-defined step name (may be empty).
	StepName string
	// InLoop reports whether the writer had a non-nil LoopContext.
	InLoop bool
	// SourceKey is "file:line:column" from step.Origin, or "" if unset.
	// For_each-expanded sibling iterations share the same SourceKey, so
	// (InLoop && SourceKey == prev.SourceKey) is the same-loop signal.
	SourceKey string
}

// VariableScope holds all variables available to a step, each in its native type.
// ToMap() merges them at the template/expression engine boundary.
type VariableScope struct {
	// User holds vars from Vars actions and config vars: blocks.
	User map[string]interface{}

	// Facts holds system facts (os, arch, hostname, cpu_cores, etc.)
	// Set once at run start, read-only during execution.
	Facts *facts.Facts

	// Loop holds the current loop iteration state, or nil outside a loop.
	Loop *LoopContext

	// Results holds registered results keyed by step.As name.
	Results map[string]RegisteredResult

	// ResultOrigins tracks the writer for each Results entry, keyed by the
	// same `as:` name. Used by the spec-37 collision check to identify
	// for_each-sibling overwrites and skip the warning in that case.
	// Kept in lockstep with Results — clones, fresh-scope, and writes all
	// update both maps together.
	ResultOrigins map[string]resultOrigin

	// Metrics holds live daemon metrics (cpu_usage_pct, memory_used_pct, etc.)
	// Set once at run start, read-only during execution.
	Metrics *metrics.Metrics

	// Env holds the parent-process environment as a string→string map.
	// Exposed to templates as `env.*` so users can write
	// `{{ env.HOME }}` or `{{ env.MY_API_KEY }}`. Set once at run start
	// by AddGlobalVariables; read-only during execution. Secrets-in-env
	// flow this way too — auditors should treat env-driven values the
	// same way as user vars, not as system facts.
	Env map[string]string

	// ApplyStartedAt is the wall-clock time the run's scope was
	// initialized — set once by AddGlobalVariables and held constant
	// for every step that follows. Exposed to templates as
	// `apply_started_at` so playbooks can build rolling-backup paths
	// like `{{ apply_started_at | strftime:"%Y%m%d_%H%M%S" }}` where
	// every step in one apply shares the same suffix (the
	// shell:`$(date)` form can mismatch across step boundaries).
	// Zero value means "scope wasn't initialized by AddGlobalVariables"
	// — ToMap omits the key in that case so test fixtures stay clean.
	ApplyStartedAt time.Time
}

// NewVariableScope returns an empty scope ready for use.
func NewVariableScope() *VariableScope {
	return &VariableScope{
		User:          make(map[string]interface{}),
		Results:       make(map[string]RegisteredResult),
		ResultOrigins: make(map[string]resultOrigin),
	}
}

// ToMap merges all sections into map[string]interface{} for the template/expression engine.
// Priority (higher overwrites lower): Facts < Metrics < Env < User < Results < Loop.
// Env sits below User so a config-level `vars: { env: ... }` can shadow
// the namespace if a playbook needs to (matches the same shadowing
// freedom user vars already have over facts/metrics).
func (s *VariableScope) ToMap() map[string]interface{} {
	m := make(map[string]interface{}, 64)
	if s.Facts != nil {
		for k, v := range s.Facts.ToMap() {
			m[k] = v
		}
	}
	if s.Metrics != nil {
		for k, v := range s.Metrics.ToMap() {
			m[k] = v
		}
	}
	if len(s.Env) > 0 {
		// Wrap in a map[string]interface{} so pongo2's dotted access
		// (`{{ env.HOME }}`) resolves cleanly.
		envMap := make(map[string]interface{}, len(s.Env))
		for k, v := range s.Env {
			envMap[k] = v
		}
		m["env"] = envMap
	}
	if !s.ApplyStartedAt.IsZero() {
		m["apply_started_at"] = s.ApplyStartedAt
	}
	for k, v := range s.User {
		m[k] = v
	}
	for k, r := range s.Results {
		m[k] = r.ToMap()
	}
	if s.Loop != nil {
		m["item"] = s.Loop.Item
		m["index"] = s.Loop.Index
		m["first"] = s.Loop.First
		m["last"] = s.Loop.Last
	}
	return m
}

// shadowedKeys returns the subset of incoming keys that collide with a system
// fact or metric key. Called before merging user vars to power shadow warnings.
func (s *VariableScope) shadowedKeys(incoming map[string]interface{}) []string {
	if s.Facts == nil && s.Metrics == nil {
		return nil
	}
	reserved := make(map[string]struct{})
	if s.Facts != nil {
		for k := range s.Facts.ToMap() {
			reserved[k] = struct{}{}
		}
	}
	if s.Metrics != nil {
		for k := range s.Metrics.ToMap() {
			reserved[k] = struct{}{}
		}
	}
	var shadowed []string
	for k := range incoming {
		if _, ok := reserved[k]; ok {
			shadowed = append(shadowed, k)
		}
	}
	return shadowed
}

// captureResult binds result into Scope.Results under step.As, gated by
// the spec-37 plan-mode policy: in ModeApply the bind always happens; in
// ModePlan it only happens when the step's action declares CaptureInPlan.
// Emits a collision warning on overwrite, exempting for_each-sibling
// iterations (same source location + both in a loop). No-op when As is "".
// resolveCaptureInPlan is a test seam over the action registry. The default
// reads ActionMetadata.CaptureInPlan from the handler registered for the
// given action type. Tests in this package may swap it to drive the
// plan-mode gate without registering a fixture action.
var resolveCaptureInPlan = func(actionType string) bool {
	h, ok := actions.Get(actionType)
	if !ok {
		return false
	}
	return h.Metadata().CaptureInPlan
}

func captureResult(ec *ExecutionContext, step config.Step, result RegisteredResult) {
	name := step.As
	if name == "" {
		return
	}
	// Tests construct ExecutionContext without Svc; in that case fall
	// through to the unconditional write that predates spec-37 so they
	// keep working without each test re-creating the full service shape.
	if ec.Svc != nil && ec.Mode() == actions.ModePlan {
		if !resolveCaptureInPlan(step.DetermineActionType()) {
			return
		}
	}
	if ec.Scope.Results == nil {
		ec.Scope.Results = make(map[string]RegisteredResult)
	}
	if ec.Scope.ResultOrigins == nil {
		ec.Scope.ResultOrigins = make(map[string]resultOrigin)
	}
	cur := makeResultOrigin(step)
	if prev, exists := ec.Scope.ResultOrigins[name]; exists && !sameForEach(cur, prev) {
		if ec.Svc != nil && ec.Svc.Logger != nil {
			ec.Svc.Logger.Infof(
				"  [WARNING] output capture: name %q overwritten by step %s (previous value discarded)",
				name, step.ID,
			)
		}
	}
	ec.Scope.Results[name] = result
	ec.Scope.ResultOrigins[name] = cur
}

// makeResultOrigin snapshots the writer step's identity for the side table.
func makeResultOrigin(step config.Step) resultOrigin {
	o := resultOrigin{
		StepID:   step.ID,
		StepName: step.Name,
		InLoop:   step.LoopContext != nil,
	}
	if step.Origin != nil {
		o.SourceKey = fmt.Sprintf("%s:%d:%d", step.Origin.FilePath, step.Origin.Line, step.Origin.Column)
	}
	return o
}

// sameForEach reports whether two writes came from sibling iterations of
// the same for_each / for_each_file step. The signal: both were loop
// iterations AND both share the same Origin source location (file:line:col).
// Two distinct loops at different source positions don't match; a loop
// iteration and an unrelated non-loop step never match.
func sameForEach(cur, prev resultOrigin) bool {
	if !cur.InLoop || !prev.InLoop {
		return false
	}
	if cur.SourceKey == "" {
		return false
	}
	return cur.SourceKey == prev.SourceKey
}

// Clone deep-copies User and Results; shares Facts and Metrics pointers (read-only after init).
// Loop is intentionally NOT copied — it is per-step state.
func (s *VariableScope) Clone() *VariableScope {
	newUser := make(map[string]interface{}, len(s.User))
	for k, v := range s.User {
		newUser[k] = v
	}
	newResults := make(map[string]RegisteredResult, len(s.Results))
	for k, v := range s.Results {
		newResults[k] = v
	}
	newOrigins := make(map[string]resultOrigin, len(s.ResultOrigins))
	for k, v := range s.ResultOrigins {
		newOrigins[k] = v
	}
	return &VariableScope{
		User:          newUser,
		Facts:         s.Facts,
		Loop:          nil,
		Results:       newResults,
		ResultOrigins: newOrigins,
		Metrics:       s.Metrics,
		// Env is set once at run start and read-only thereafter; share
		// the parent's map by reference — same convention as Facts /
		// Metrics — to avoid copying the entire process environment on
		// every step.
		Env: s.Env,
		// ApplyStartedAt is run-wide (set once by AddGlobalVariables);
		// propagate it so clones expose `{{ apply_started_at }}` in
		// templates. Pre-fix this field was silently dropped — caught
		// during the F053 cold-read pass; harmless today because
		// ec.Clone() has no production callers, but the moment a future
		// caller introduces one (sub-include / for_each body) the
		// missing field would surprise template authors.
		ApplyStartedAt: s.ApplyStartedAt,
	}
}
