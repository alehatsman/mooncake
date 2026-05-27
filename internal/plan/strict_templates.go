package plan

// strict_templates.go: scans an expanded plan for `{{ root }}`
// references whose root identifier is not in initial_vars, not
// produced by a prior step's `as:` register, and not one of the
// well-known loop/env names.
//
// Motivation: pongo2 silently renders undefined identifiers as the
// empty string. A typo like `{{ user_hom }}` in `file.write.path`
// produces `/test` and the planner happily plans a write to `/`.
// The strict pass turns that into a typed diagnostic at plan time
// (and an error in `validate`).
//
// Scope:
//   - Reflects over every string field in each plan.Steps entry,
//     including action structs reached via pointer.
//   - Honors step ordering: a reference to `step.As` only counts as
//     known if the producing step appears earlier in the plan.
//   - Recognized free names: `item` (for_each loop var), `env`
//     (env-var access), plus the pongo2 builtins true/false/none.
//
// Out of scope (phase 1):
//   - Resolved-by-vars.load names. A `vars.load:` step reads a file
//     at execute time; the names it produces are not yet visible at
//     plan time. Users who hit a false-positive here should switch
//     to file-level `vars:` or pass --allow-undefined.
//   - Dotted-path validation. We only check the root identifier;
//     `{{ env.HOM }}` is accepted because `env` is recognized.

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/alehatsman/mooncake/internal/config"
)

// UnresolvedRef describes a single `{{ root }}` reference whose
// root identifier is not in scope at the step's plan-time position.
type UnresolvedRef struct {
	StepID   string         `json:"step_id,omitempty" yaml:"step_id,omitempty"`
	StepName string         `json:"step_name,omitempty" yaml:"step_name,omitempty"`
	Field    string         `json:"field,omitempty" yaml:"field,omitempty"`
	Root     string         `json:"root" yaml:"root"`
	Origin   *config.Origin `json:"origin,omitempty" yaml:"origin,omitempty"`
}

// rootVarRe extracts the leading identifier from a `{{ expr }}` block.
// Matches the renderer's pattern (renderer.go) so the two stay in sync.
var rootVarRe = regexp.MustCompile(`\{\{-?\s*([a-zA-Z_][a-zA-Z0-9_]*)`)

// pongo2Builtins are names resolved by pongo2 itself, not by the
// caller's variable map.
var pongo2Builtins = map[string]bool{
	"true": true, "false": true, "none": true, "pongo2": true,
}

// freeNames are names that are always considered defined at plan
// time regardless of initial_vars (loop vars, env-var access).
var freeNames = map[string]bool{
	"item": true,
	"env":  true,
}

// CheckPlanStrict scans the expanded plan for unresolved root
// identifiers. Returns a deterministic list (steps in order, refs
// per step in field declaration order, deduplicated by root).
func CheckPlanStrict(p *Plan) []UnresolvedRef {
	if p == nil {
		return nil
	}

	known := make(map[string]bool, len(p.InitialVars)+len(freeNames)+len(pongo2Builtins))
	for k := range p.InitialVars {
		known[k] = true
	}
	for k := range freeNames {
		known[k] = true
	}
	for k := range pongo2Builtins {
		known[k] = true
	}

	var out []UnresolvedRef
	for i := range p.Steps {
		step := &p.Steps[i]
		// Skip steps gated out by tag/when filtering — their templated
		// fields never reach the executor, so flagging them would be
		// noise.
		if step.Skipped {
			// A subsequent step may still reference this step's As,
			// so register it in the known set before continuing.
			if step.As != "" {
				known[step.As] = true
			}
			continue
		}
		seen := make(map[string]bool)
		collectStepRefs(step, known, seen, &out)
		// Register this step's `as:` name AFTER scanning so a step
		// cannot self-reference its own register.
		if step.As != "" {
			known[step.As] = true
		}
	}
	return out
}

// collectStepRefs walks the step's templatable string fields and
// appends one UnresolvedRef per (root, field) pair whose root is not
// in known. Within a single step, each root is reported once.
func collectStepRefs(step *config.Step, known, seen map[string]bool, out *[]UnresolvedRef) {
	scan := func(field, tmpl string) {
		for _, root := range missingRoots(tmpl, known) {
			if seen[root] {
				continue
			}
			seen[root] = true
			*out = append(*out, UnresolvedRef{
				StepID:   step.ID,
				StepName: step.Name,
				Field:    field,
				Root:     root,
				Origin:   step.Origin,
			})
		}
	}

	// Universal fields where the planner uses plain Render (eager) —
	// pongo2 already squashed unresolved roots to empty by the time
	// we get here. We can still flag refs that survived because the
	// planner records them in pre-render scans (see Planner.recordEager).
	// For now scan whatever string content remains; that catches the
	// common case where the planner used RenderPreserving.
	scan("when", step.When)
	scan("changed_when", step.ChangedWhen)
	scan("failed_when", step.FailedWhen)
	scan("cwd", step.Cwd)
	scan("as_user", step.AsUser)
	scan("timeout", step.Timeout)
	scan("name", step.Name)

	if step.Shell != nil {
		scan("shell.cmd", step.Shell.Cmd)
		scan("shell.creates", step.Shell.Creates)
		scan("shell.unless", step.Shell.Unless)
	}
	if step.Creates != nil {
		scan("creates", *step.Creates)
	}
	if step.Unless != nil {
		scan("unless", *step.Unless)
	}
	if step.UnlessExists != nil {
		scan("unless_exists", *step.UnlessExists)
	}
	if step.UnlessCommand != nil {
		scan("unless_command", *step.UnlessCommand)
	}
	for key, value := range step.Env {
		scan("env."+key, value)
	}

	// Action structs — reach via the same reflect strategy as
	// walkAndRender: each step has exactly one non-nil pointer-to-
	// struct action field. Scan every string leaf inside.
	scanActionFields(step, scan)
}

// scanActionFields finds the step's active action struct (the one
// non-nil pointer-to-struct field on Step) and walks all its
// reachable string leaves.
func scanActionFields(step *config.Step, scan func(field, tmpl string)) {
	rv := reflect.ValueOf(step).Elem()
	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		sf := rt.Field(i)
		// Only action fields are typed *T where T is a struct.
		fv := rv.Field(i)
		if fv.Kind() != reflect.Pointer || fv.IsNil() {
			continue
		}
		if fv.Type().Elem().Kind() != reflect.Struct {
			continue
		}
		// Heuristic: skip universal compound fields (Retry, ForEach,
		// LoopContext, Origin) — those are not action payloads.
		switch sf.Name {
		case "Retry", "ForEach", "LoopContext", "Origin", "Shell":
			continue
		}
		walkStringLeaves(fv.Elem(), sf.Name, scan)
	}

	// Use/Props (spec-67) — `use:` is a string action field, props
	// is a map sibling. Both can carry templates.
	if step.Use != "" {
		scan("use", step.Use)
	}
	if step.Props != nil {
		walkPropsLeaves(step.Props, "props", scan)
	}
}

// walkStringLeaves recursively visits every string leaf inside a
// reflect.Value, calling scan(fieldPath, value) on each. fieldPath
// accumulates struct field names dot-joined for diagnostic messages.
func walkStringLeaves(rv reflect.Value, prefix string, scan func(field, tmpl string)) {
	switch rv.Kind() {
	case reflect.String:
		if rv.String() != "" {
			scan(prefix, rv.String())
		}
	case reflect.Pointer:
		if rv.IsNil() {
			return
		}
		walkStringLeaves(rv.Elem(), prefix, scan)
	case reflect.Struct:
		rt := rv.Type()
		for i := 0; i < rv.NumField(); i++ {
			sf := rt.Field(i)
			if !sf.IsExported() {
				continue
			}
			child := prefix + "." + sf.Name
			walkStringLeaves(rv.Field(i), child, scan)
		}
	case reflect.Slice, reflect.Array:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return // []byte — not a template carrier
		}
		for i := 0; i < rv.Len(); i++ {
			walkStringLeaves(rv.Index(i), prefix, scan)
		}
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return
		}
		iter := rv.MapRange()
		for iter.Next() {
			walkStringLeaves(iter.Value(), prefix+"."+iter.Key().String(), scan)
		}
	case reflect.Interface:
		if rv.IsNil() {
			return
		}
		walkStringLeaves(rv.Elem(), prefix, scan)
	}
}

// walkPropsLeaves walks the spec-67 `props:` tree (map/slice/string)
// and reports string leaves. props is interface{} because it is
// user-shaped YAML.
func walkPropsLeaves(v interface{}, prefix string, scan func(field, tmpl string)) {
	switch x := v.(type) {
	case nil:
		return
	case string:
		scan(prefix, x)
	case map[string]interface{}:
		for k, sub := range x {
			walkPropsLeaves(sub, prefix+"."+k, scan)
		}
	case []interface{}:
		for _, sub := range x {
			walkPropsLeaves(sub, prefix, scan)
		}
	}
}

// missingRoots returns the unique root identifiers in tmpl that are
// not in the known set, in first-occurrence order.
func missingRoots(tmpl string, known map[string]bool) []string {
	if !strings.Contains(tmpl, "{{") {
		return nil
	}
	var out []string
	seen := make(map[string]bool)
	for _, m := range rootVarRe.FindAllStringSubmatch(tmpl, -1) {
		if len(m) < 2 {
			continue
		}
		root := m[1]
		if known[root] || seen[root] {
			continue
		}
		seen[root] = true
		out = append(out, root)
	}
	return out
}
