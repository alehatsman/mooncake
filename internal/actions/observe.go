package actions

import (
	"encoding/json"
	"time"
)

// ObserveResult is the shared envelope returned by every observe.*
// handler (spec-59). The typed per-handler payload lives in Value;
// the universal fields (Found, AsOf, Error) wrap it so consumers can
// branch on observation outcome without knowing the per-handler type.
//
// When a handler returns its result, it stores the ObserveResult under
// the well-known key "observe" in the executor.Result.Data map (along
// with the un-wrapped value at "value") so spec-37 `as:` capture
// exposes both shapes to downstream templates:
//
//   - {{ name.value.<field> }}  — the typed per-handler payload
//   - {{ name.found }}          — universal Found flag
//   - {{ name.as_of }}          — observation timestamp
//   - {{ name.error }}          — user-facing error when Found=false
//
// Mutation is forbidden by contract: observe handlers must return
// Changed=false and declare empty Diff / nil Reverse / Cost{Risk:1}
// / Permissions{ReadOnly:true} per spec-59's ABI specialization.
type ObserveResult struct {
	// Found is true if the observed resource exists / is reachable.
	// Each handler defines what "found" means in its own contract.
	Found bool `json:"found"`

	// Value is the typed per-handler payload (e.g. PortObservation,
	// ProcessObservation). Stored as any in this generic envelope.
	Value any `json:"value,omitempty"`

	// AsOf is the wall-clock time the observation was taken. Set by
	// the handler, not by the caller.
	AsOf time.Time `json:"as_of"`

	// Error carries a user-facing error string when Found=false because
	// the observation itself failed (DNS error, permission denied,
	// transport failure). Empty when Found=false because the resource
	// genuinely doesn't exist.
	Error string `json:"error,omitempty"`
}

// ObserveValueToMap converts an observe handler's typed Value struct
// into a map[string]any keyed by the struct's json tags. The template
// engine can't reflect through arbitrary Go struct fields, so every
// observe handler should round-trip its Value through this helper
// before publishing to Result.Data — that's what makes
// `{{ name.value.<field> }}` resolve at apply time.
//
// On marshal failure (e.g. value contains an unmarshallable type)
// returns the original value unchanged. Defensive — observe handlers
// control their own Value types, so this path should never fire in
// practice.
func ObserveValueToMap(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return v
	}
	return out
}

// PlanDeferred returns the synthetic observation result that every
// observe.* handler returns in plan mode by default (per spec-59 G4).
// The shape matches a real observation so downstream templates that
// reference {{ name.value.<field> }} don't blow up in plan mode — they
// see a zero value of the typed Value, with Found=false and the
// reason in Error.
//
// emptyValue should be a zero-value of the handler's typed payload
// struct, so templates that index into it still type-check.
func PlanDeferred(emptyValue any) ObserveResult {
	return ObserveResult{
		Found: false,
		Value: emptyValue,
		AsOf:  time.Now(),
		Error: "observation deferred to apply mode",
	}
}
