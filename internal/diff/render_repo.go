package diff

import (
	"fmt"
	"io"

	"github.com/alehatsman/mooncake/internal/actions"
)

func init() {
	Register(matchRepo)
}

// matchRepo recognises a typed actions.Diff describing a pkg.repo
// mutation. Dispatch is on Attributes["kind"]=="pkg.repo". Accepts
// the typed *actions.RepoDiff pointer or the JSON-decoded map shape.
func matchRepo(detail any) Renderer {
	d, ok := detail.(*actions.Diff)
	if !ok || d == nil {
		return nil
	}
	if d.Resource.Attributes["kind"] != "pkg.repo" {
		return nil
	}
	after, ok := toRepoDiff(d.After)
	if !ok {
		return nil
	}
	return &repoRenderer{op: d.Operation, payload: after}
}

func toRepoDiff(v any) (actions.RepoDiff, bool) {
	switch x := v.(type) {
	case *actions.RepoDiff:
		if x == nil {
			return actions.RepoDiff{}, false
		}
		return *x, true
	case actions.RepoDiff:
		return x, true
	case map[string]interface{}:
		out := actions.RepoDiff{}
		if s, ok := x["name"].(string); ok {
			out.Name = s
		}
		if s, ok := x["state"].(string); ok {
			out.State = s
		}
		if s, ok := x["driver"].(string); ok {
			out.Driver = s
		}
		any := out.Name != "" || out.State != "" || out.Driver != ""
		return out, any
	}
	return actions.RepoDiff{}, false
}

type repoRenderer struct {
	op      actions.Operation
	payload actions.RepoDiff
}

func (r *repoRenderer) Kind() string { return "pkg.repo" }

// Render writes a one-line summary:
//
//	pkg.repo add docker (apt)
//	pkg.repo add nginx-stable (dnf)
//	pkg.repo remove old-repo (apt)
//	pkg.repo add custom
func (r *repoRenderer) Render(w io.Writer, format Format) error {
	switch format {
	case FormatText, "":
		verb := repoVerb(r.op)
		name := r.payload.Name
		if name == "" {
			name = "<unnamed>"
		}
		line := fmt.Sprintf("  pkg.repo %s %s", verb, name)
		if r.payload.Driver != "" {
			line += " (" + r.payload.Driver + ")"
		}
		_, err := fmt.Fprintln(w, line)
		return err
	case FormatJSON, FormatYAML:
		_, err := fmt.Fprintf(w, "pkg.repo %s: %s\n", repoVerb(r.op), r.payload.Name)
		return err
	}
	return fmt.Errorf("unsupported format: %s", format)
}

// repoVerb maps OpCreate → "add", OpDelete → "remove" so the
// operator-facing verbs match the package renderer ("install",
// "remove") without re-using "install" (a repo is added, not
// installed).
func repoVerb(op actions.Operation) string {
	switch op {
	case actions.OpCreate:
		return "add"
	case actions.OpUpdate:
		return "update"
	case actions.OpDelete:
		return "remove"
	case actions.OpNoop:
		return "noop"
	default:
		return "change"
	}
}
