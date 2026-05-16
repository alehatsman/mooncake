package diff

import (
	"fmt"
	"io"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
)

func init() {
	Register(matchService)
}

// matchService recognises a typed actions.Diff describing an
// os.systemd mutation. Dispatch is on the After payload type rather
// than Resource.Kind because ResourceService is also used by the
// legacy os.service handler (which still emits its package-local
// ServiceSnapshot). A future wave can migrate os.service to
// *actions.ServiceDiff and this renderer will start covering it too.
//
// Accepts the typed pointer / value / JSON-decoded map shapes.
func matchService(detail any) Renderer {
	d, ok := detail.(*actions.Diff)
	if !ok || d == nil {
		return nil
	}
	sd, ok := toServiceDiff(d.After)
	if !ok {
		return nil
	}
	return &serviceRenderer{op: d.Operation, payload: sd}
}

func toServiceDiff(after any) (actions.ServiceDiff, bool) {
	switch v := after.(type) {
	case *actions.ServiceDiff:
		if v == nil {
			return actions.ServiceDiff{}, false
		}
		return *v, true
	case actions.ServiceDiff:
		return v, true
	case map[string]interface{}:
		// Only claim a JSON-decoded map when it carries fields
		// distinctive of ServiceDiff (sections / started). Avoids
		// stealing the dispatch from the legacy os.service path
		// which also lands a map after JSON decode.
		_, hasSections := v["sections"]
		_, hasStarted := v["started"]
		_, hasPath := v["path"]
		if !hasSections && !hasStarted && !hasPath {
			return actions.ServiceDiff{}, false
		}
		out := actions.ServiceDiff{}
		if s, ok := v["name"].(string); ok {
			out.Name = s
		}
		if s, ok := v["state"].(string); ok {
			out.State = s
		}
		if s, ok := v["path"].(string); ok {
			out.Path = s
		}
		if secs, ok := v["sections"].([]interface{}); ok {
			for _, x := range secs {
				if s, ok := x.(string); ok {
					out.Sections = append(out.Sections, s)
				}
			}
		}
		if b, ok := v["enabled"].(bool); ok {
			out.Enabled = &b
		}
		if b, ok := v["started"].(bool); ok {
			out.Started = &b
		}
		return out, true
	}
	return actions.ServiceDiff{}, false
}

type serviceRenderer struct {
	op      actions.Operation
	payload actions.ServiceDiff
}

func (r *serviceRenderer) Kind() string { return "service" }

// Render writes a one-line summary of the systemd unit mutation:
//
//	service install myapp.service (Service Install) enabled started
//	service install myapp.timer (Timer Install) enabled
//	service remove old.service
func (r *serviceRenderer) Render(w io.Writer, format Format) error {
	switch format {
	case FormatText, "":
		verb := serviceOperationVerb(r.op)
		name := r.payload.Name
		if name == "" {
			name = "<unnamed>"
		}
		line := fmt.Sprintf("  service %s %s", verb, name)
		if len(r.payload.Sections) > 0 {
			line += " (" + strings.Join(r.payload.Sections, " ") + ")"
		}
		var flags []string
		if r.payload.Enabled != nil {
			if *r.payload.Enabled {
				flags = append(flags, "enabled")
			} else {
				flags = append(flags, "disabled")
			}
		}
		if r.payload.Started != nil {
			if *r.payload.Started {
				flags = append(flags, "started")
			} else {
				flags = append(flags, "stopped")
			}
		}
		if len(flags) > 0 {
			line += " " + strings.Join(flags, " ")
		}
		_, err := fmt.Fprintln(w, line)
		return err
	case FormatJSON, FormatYAML:
		_, err := fmt.Fprintf(w, "service %s: %s\n",
			serviceOperationVerb(r.op), r.payload.Name)
		return err
	}
	return fmt.Errorf("unsupported format: %s", format)
}

// serviceOperationVerb — install (create) / update / remove (delete)
// / noop. Picked install/remove over create/delete because the user-
// facing systemctl story is "install a unit" / "remove a unit."
func serviceOperationVerb(op actions.Operation) string {
	switch op {
	case actions.OpCreate:
		return "install"
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
