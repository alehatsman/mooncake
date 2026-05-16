package diff

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
)

func init() {
	Register(matchUser)
}

// matchUser recognises a typed actions.Diff describing an os.user
// mutation. The os.user handler currently uses Resource.Kind ==
// ResourceOther with a discriminating Attributes["kind"] == "os.user"
// (rather than a dedicated ResourceKind constant); the renderer
// dispatches on the attribute so adding a dedicated kind later
// doesn't require a renderer change.
//
// Accepts both *actions.Diff with *actions.UserDiff After (in-memory)
// and the map[string]interface{} shape (JSON-decoded saved plan).
func matchUser(detail any) Renderer {
	d, ok := detail.(*actions.Diff)
	if !ok || d == nil {
		return nil
	}
	if d.Resource.Attributes["kind"] != "os.user" {
		return nil
	}
	ud, ok := toUserDiff(d.After)
	if !ok {
		return nil
	}
	return &userRenderer{op: d.Operation, payload: ud}
}

func toUserDiff(after any) (actions.UserDiff, bool) {
	switch v := after.(type) {
	case *actions.UserDiff:
		if v == nil {
			return actions.UserDiff{}, false
		}
		return *v, true
	case actions.UserDiff:
		return v, true
	case map[string]interface{}:
		out := actions.UserDiff{}
		if s, ok := v["name"].(string); ok {
			out.Name = s
		}
		if s, ok := v["state"].(string); ok {
			out.State = s
		}
		if u, ok := v["uid"].(float64); ok {
			ui := int(u)
			out.UID = &ui
		}
		if s, ok := v["shell"].(string); ok {
			out.Shell = s
		}
		if s, ok := v["home"].(string); ok {
			out.Home = s
		}
		if g, ok := v["groups"].([]interface{}); ok {
			for _, item := range g {
				if s, ok := item.(string); ok {
					out.Groups = append(out.Groups, s)
				}
			}
		}
		if b, ok := v["system"].(bool); ok {
			out.System = b
		}
		any := out.Name != "" || out.State != "" || out.UID != nil ||
			out.Shell != "" || out.Home != "" || len(out.Groups) > 0 || out.System
		return out, any
	}
	return actions.UserDiff{}, false
}

type userRenderer struct {
	op      actions.Operation
	payload actions.UserDiff
}

func (r *userRenderer) Kind() string { return "user" }

// Render writes a one-line summary of the user mutation. Verbs use
// the same OpCreate/Delete vocabulary as the package renderer but
// the user-facing labels match getent-shell expectations
// (create/remove instead of install/uninstall).
//
//	user create alice (uid=1001 shell=/bin/zsh groups=docker,sudo)
//	user remove alice
//	user create deploy (system groups=deploy,docker)
func (r *userRenderer) Render(w io.Writer, format Format) error {
	switch format {
	case FormatText, "":
		verb := userOperationVerb(r.op)
		name := r.payload.Name
		if name == "" {
			name = "<unnamed>"
		}
		attrs := userAttrs(r.payload)
		line := fmt.Sprintf("  user %s %s", verb, name)
		if attrs != "" {
			line += " (" + attrs + ")"
		}
		_, err := fmt.Fprintln(w, line)
		return err
	case FormatJSON, FormatYAML:
		_, err := fmt.Fprintf(w, "user %s: %s\n", userOperationVerb(r.op), r.payload.Name)
		return err
	}
	return fmt.Errorf("unsupported format: %s", format)
}

func userAttrs(u actions.UserDiff) string {
	var parts []string
	if u.UID != nil {
		parts = append(parts, "uid="+strconv.Itoa(*u.UID))
	}
	if u.Shell != "" {
		parts = append(parts, "shell="+u.Shell)
	}
	if u.Home != "" {
		parts = append(parts, "home="+u.Home)
	}
	if u.System {
		parts = append(parts, "system")
	}
	if len(u.Groups) > 0 {
		parts = append(parts, "groups="+strings.Join(u.Groups, ","))
	}
	return strings.Join(parts, " ")
}

// userOperationVerb maps spec-22 Op{Create,Delete,...} → the
// user-facing verb. Unlike the package renderer, OpUpdate is not
// reachable today (the os.user handler classifies state=present →
// OpCreate and state=absent → OpDelete; there's no "modify
// existing" Op because the handler can't tell present-but-different
// from present-but-same without probing). Keep OpUpdate handled for
// forward-compat.
func userOperationVerb(op actions.Operation) string {
	switch op {
	case actions.OpCreate:
		return "create"
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
