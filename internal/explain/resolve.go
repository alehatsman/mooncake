package explain

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/ops"
	"github.com/alehatsman/mooncake/internal/runlog"
)

// Options tune the resolver. Zero value is the typical agent call.
type Options struct {
	// ExamplesLimit caps the number of example excerpts returned for a
	// kind: action result. Semantics (F044):
	//   < 0  — caller has no preference; use the default of 3.
	//   == 0 — caller explicitly asked for no examples.
	//   > 0  — return up to that many examples.
	//
	// The MCP / CLI boundary validates the user-supplied range (must be
	// 0..10 inclusive). The MCP layer maps "argument absent" to -1 so
	// this field can carry the absent-vs-zero distinction.
	ExamplesLimit int

	// ExamplesRoot is the directory to scan for *.yml example files.
	// Empty means scan the in-tree examples/ directory next to the
	// running binary's working directory. Tests override this.
	ExamplesRoot string

	// SchemaJSON overrides the embedded schema. Tests use this to feed
	// a fake schema without touching the global registry.
	SchemaJSON []byte

	// RunsReader / OpsReader override the default JSONL readers
	// (~/.mooncake/runs.jsonl and ~/.mooncake/ops.jsonl). Tests inject
	// these to feed fixtures; production callers leave both nil and get
	// the default reader.
	RunsReader func() ([]runlog.Entry, error)
	OpsReader  func() ([]ops.Entry, error)
}

// Resolve takes a noun and returns a typed Result. Wave 1: action only;
// any other shape falls through to NotFound.
func Resolve(noun string, opts Options) Result {
	noun = strings.TrimSpace(noun)
	if noun == "" {
		return notFound(noun, "empty noun", nil)
	}

	switch detectKind(noun) {
	case KindAction:
		return resolveAction(noun, opts)
	case KindRun:
		return resolveRun(noun, opts)
	case KindOp:
		return resolveOp(noun, opts)
	case KindResource:
		return resolveResource(noun, opts)
	default:
		return notFound(noun, "unrecognised noun shape", nil)
	}
}

// detectKind infers what kind of noun the input is, by prefix.
// Same rules as spec-68 §CLI surface.
func detectKind(noun string) Kind {
	switch {
	case strings.HasPrefix(noun, "r/"):
		return KindRun
	case strings.HasPrefix(noun, "op/"):
		return KindOp
	case strings.Contains(noun, ":") && !strings.HasPrefix(noun, "http"):
		// <kind>:<rest> — resource handle. Excluding http(s)// URLs which
		// also contain ":" but mean something else.
		return KindResource
	case looksLikeActionVerb(noun):
		return KindAction
	default:
		return KindNotFound
	}
}

// looksLikeActionVerb is a cheap pre-filter to avoid running the full
// lookup pipeline on obvious garbage. The actual existence check is in
// resolveAction.
func looksLikeActionVerb(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}

func resolveAction(noun string, opts Options) Result {
	h, ok := actions.Get(noun)
	if !ok {
		return notFound(noun, "unknown action verb", actionCandidates(noun))
	}

	md := h.Metadata()

	payload := &ActionPayload{
		Name:         noun,
		Metadata:     toWireMetadata(md),
		Schema:       extractSchemaSlice(noun, opts.SchemaJSON),
		DiffShape:    diffShapeFor(h),
		ReverseShape: reverseShapeFor(h),
		Examples:     findExamples(noun, opts),
		SpecOrigin:   nil, // wave 1: not wired
	}

	return Result{Kind: KindAction, Action: payload}
}

func toWireMetadata(md actions.ActionMetadata) ActionMetadata {
	return ActionMetadata{
		Description:        md.Description,
		Category:           string(md.Category),
		Version:            md.Version,
		SupportedPlatforms: md.SupportedPlatforms,
		SupportsDryRun:     md.SupportsDryRun,
		SupportsBecome:     md.SupportsBecome,
		RequiresSudo:       md.RequiresSudo,
		ImplementsCheck:    md.ImplementsCheck,
		EmitsEvents:        md.EmitsEvents,
	}
}

// extractSchemaSlice pulls the per-action node from schema.json
// (under "definitions"). Returns nil if the schema doesn't carry an
// entry for this verb — that's fine; the metadata above still wins.
func extractSchemaSlice(noun string, override []byte) *SchemaSlice {
	raw := override
	if len(raw) == 0 {
		raw = config.SchemaJSON()
	}
	if len(raw) == 0 {
		return nil
	}

	var doc struct {
		Definitions map[string]map[string]any `json:"definitions"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	node, ok := doc.Definitions[noun]
	if !ok {
		return nil
	}

	slice := &SchemaSlice{Raw: node}
	if req, ok := node["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				slice.Required = append(slice.Required, s)
			}
		}
	}
	if props, ok := node["properties"].(map[string]any); ok {
		slice.Properties = make(map[string]SchemaProperty, len(props))
		for name, v := range props {
			pmap, ok := v.(map[string]any)
			if !ok {
				continue
			}
			slice.Properties[name] = flattenSchemaProperty(pmap)
		}
	}
	return slice
}

func flattenSchemaProperty(p map[string]any) SchemaProperty {
	out := SchemaProperty{}
	if t, ok := p["type"].(string); ok {
		out.Type = t
	}
	if d, ok := p["description"].(string); ok {
		out.Description = d
	}
	out.Default = p["default"]
	if enum, ok := p["enum"].([]any); ok {
		out.Enum = enum
	}
	if oneOf, ok := p["oneOf"].([]any); ok {
		for _, alt := range oneOf {
			if amap, ok := alt.(map[string]any); ok {
				if t, ok := amap["type"].(string); ok {
					out.OneOf = append(out.OneOf, t)
				}
			}
		}
	}
	return out
}

// diffShapeFor reports whether a handler ships a typed Differ.
// Wave 1 keeps this coarse; wave 2 may invoke Differ.Diff against a
// sample to materialize before/after shape.
func diffShapeFor(h actions.Handler) DiffShape {
	if _, ok := h.(actions.Differ); ok {
		return DiffShape{Declared: true,
			Note: "Differ implemented; call mooncake plan to see the typed Diff for a concrete step."}
	}
	return DiffShape{Declared: false,
		Note: "No Differ; this action's Diff defaults to a coarse changed/unchanged signal."}
}

// reverseShapeFor reports Reverse declaration. The handler ABI also
// exposes Reversible via Capabilities; we use the interface assertion
// so this code stays loud about which handlers are actually Reverser.
func reverseShapeFor(h actions.Handler) ReverseShape {
	if _, ok := h.(actions.Reverser); ok {
		return ReverseShape{Declared: true,
			Caveat: "Reverse() runs only against a result whose Run captured ReverseData (apply mode, not plan mode)."}
	}
	return ReverseShape{Declared: false,
		Caveat: "This action does not declare Reverse; failed transactions cannot undo it."}
}

// notFound is the common typed not_found constructor.
func notFound(noun, reason string, candidates []NotFoundMatch) Result {
	return Result{
		Kind: KindNotFound,
		NotFound: &NotFoundPayload{
			Noun:       noun,
			Reason:     reason,
			Candidates: candidates,
		},
	}
}

// actionCandidates produces up to 5 nearest-prefix-match action names.
// Cheap heuristic: prefix match first, then substring. Avoids pulling
// in a Levenshtein library for a hint.
func actionCandidates(noun string) []NotFoundMatch {
	all := actions.List()
	type scored struct {
		name  string
		score int
	}
	lower := strings.ToLower(noun)
	var hits []scored
	for _, m := range all {
		name := m.Name
		ln := strings.ToLower(name)
		switch {
		case ln == lower:
			hits = append(hits, scored{name, 1000})
		case strings.HasPrefix(ln, lower):
			hits = append(hits, scored{name, 500 - len(name)})
		case strings.Contains(ln, lower):
			hits = append(hits, scored{name, 100 - len(name)})
		case strings.HasPrefix(lower, ln):
			hits = append(hits, scored{name, 50 - len(name)})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	if len(hits) > 5 {
		hits = hits[:5]
	}
	out := make([]NotFoundMatch, 0, len(hits))
	for _, h := range hits {
		out = append(out, NotFoundMatch{Kind: KindAction, ID: h.name})
	}
	return out
}
