package diff

import "io"

// Format selects an output shape for Renderer.Render.
//
// Wave 1 uses FormatText only — cmd/mooncake.go renders to a terminal
// and the JSON / YAML paths emit the raw plan via the existing
// encoder. Subsequent waves wire FormatJSON / FormatYAML when per-kind
// payloads need custom marshalling (most don't; the typed structs
// marshal naturally).
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
	FormatYAML Format = "yaml"
)

// Renderer turns one step's typed Diff payload into output. Each
// implementation knows exactly one kind of payload — render_file
// handles effects.ContentDiff, future renderers will handle
// PackageDiff / UserDiff / etc.
type Renderer interface {
	// Kind returns a stable identifier for telemetry / debugging. Not
	// used for dispatch — Lookup dispatches on the dynamic type of
	// detail, not this string.
	Kind() string

	// Render writes the rendered diff to w in the given format. An
	// empty Render output is allowed (e.g. a binary file whose diff
	// would be noise) — callers treat empty output as "nothing to
	// show for this step."
	Render(w io.Writer, format Format) error
}

// matchFunc reports whether a concrete renderer recognizes a given
// StepInspection.Detail. Returning a non-nil Renderer means "I'll
// render this"; returning nil means "pass." Registered renderers are
// tried in registration order; first match wins.
type matchFunc func(detail any) Renderer

var registry []matchFunc

// Register adds a matchFunc to the registry. Called from each
// render_<kind>.go file's init(). Not thread-safe — registration
// happens once at package init time before any Lookup call.
func Register(m matchFunc) {
	registry = append(registry, m)
}

// Lookup returns a Renderer for the given Detail value, or nil when
// no registered renderer recognizes it. Callers fall back to their
// own placeholder text (e.g. "would update") when Lookup returns nil.
func Lookup(detail any) Renderer {
	if detail == nil {
		return nil
	}
	for _, m := range registry {
		if r := m(detail); r != nil {
			return r
		}
	}
	return nil
}
