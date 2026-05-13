package executor

import (
	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/metrics"
)

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

	// Metrics holds live daemon metrics (cpu_usage_pct, memory_used_pct, etc.)
	// Set once at run start, read-only during execution.
	Metrics *metrics.Metrics
}

// NewVariableScope returns an empty scope ready for use.
func NewVariableScope() *VariableScope {
	return &VariableScope{
		User:    make(map[string]interface{}),
		Results: make(map[string]RegisteredResult),
	}
}

// ToMap merges all sections into map[string]interface{} for the template/expression engine.
// Priority (higher overwrites lower): Facts < Metrics < User < Results < Loop.
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
	return &VariableScope{
		User:    newUser,
		Facts:   s.Facts,
		Loop:    nil,
		Results: newResults,
		Metrics: s.Metrics,
	}
}
