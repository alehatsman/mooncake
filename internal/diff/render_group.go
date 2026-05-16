package diff

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
)

func init() {
	Register(matchGroup)
}

// matchGroup recognises a typed actions.Diff describing an os.group
// mutation. Dispatch follows the same os.user convention: the
// handler uses ResourceOther + Attributes["kind"] == "os.group"
// rather than a dedicated ResourceKind.
func matchGroup(detail any) Renderer {
	d, ok := detail.(*actions.Diff)
	if !ok || d == nil {
		return nil
	}
	if d.Resource.Attributes["kind"] != "os.group" {
		return nil
	}
	gd, ok := toGroupDiff(d.After)
	if !ok {
		return nil
	}
	return &groupRenderer{op: d.Operation, payload: gd}
}

func toGroupDiff(after any) (actions.GroupDiff, bool) {
	switch v := after.(type) {
	case *actions.GroupDiff:
		if v == nil {
			return actions.GroupDiff{}, false
		}
		return *v, true
	case actions.GroupDiff:
		return v, true
	case map[string]interface{}:
		out := actions.GroupDiff{}
		if s, ok := v["name"].(string); ok {
			out.Name = s
		}
		if s, ok := v["state"].(string); ok {
			out.State = s
		}
		if u, ok := v["gid"].(float64); ok {
			gi := int(u)
			out.GID = &gi
		}
		if b, ok := v["system"].(bool); ok {
			out.System = b
		}
		any := out.Name != "" || out.State != "" || out.GID != nil || out.System
		return out, any
	}
	return actions.GroupDiff{}, false
}

type groupRenderer struct {
	op      actions.Operation
	payload actions.GroupDiff
}

func (r *groupRenderer) Kind() string { return "group" }

// Render writes a one-line summary. Verbs reuse the user vocabulary
// for consistency:
//
//	group create deploy (gid=1001)
//	group remove staff
//	group create docker (system)
func (r *groupRenderer) Render(w io.Writer, format Format) error {
	switch format {
	case FormatText, "":
		verb := userOperationVerb(r.op) // shared verb mapping (create/update/remove)
		name := r.payload.Name
		if name == "" {
			name = "<unnamed>"
		}
		var parts []string
		if r.payload.GID != nil {
			parts = append(parts, "gid="+strconv.Itoa(*r.payload.GID))
		}
		if r.payload.System {
			parts = append(parts, "system")
		}
		line := fmt.Sprintf("  group %s %s", verb, name)
		if len(parts) > 0 {
			line += " (" + strings.Join(parts, " ") + ")"
		}
		_, err := fmt.Fprintln(w, line)
		return err
	case FormatJSON, FormatYAML:
		_, err := fmt.Fprintf(w, "group %s: %s\n", userOperationVerb(r.op), r.payload.Name)
		return err
	}
	return fmt.Errorf("unsupported format: %s", format)
}
