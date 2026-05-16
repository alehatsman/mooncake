package diff

import (
	"fmt"
	"io"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
)

func init() {
	Register(matchMount)
}

// matchMount recognises a typed actions.Diff describing an os.mount
// mutation. Dispatch is on Attributes["kind"]=="os.mount". Accepts
// the typed *actions.MountDiff pointer or the JSON-decoded map shape.
func matchMount(detail any) Renderer {
	d, ok := detail.(*actions.Diff)
	if !ok || d == nil {
		return nil
	}
	if d.Resource.Attributes["kind"] != "os.mount" {
		return nil
	}
	md, ok := toMountDiff(d.After)
	if !ok {
		return nil
	}
	return &mountRenderer{op: d.Operation, payload: md}
}

func toMountDiff(after any) (actions.MountDiff, bool) {
	switch v := after.(type) {
	case *actions.MountDiff:
		if v == nil {
			return actions.MountDiff{}, false
		}
		return *v, true
	case actions.MountDiff:
		return v, true
	case map[string]interface{}:
		out := actions.MountDiff{}
		if s, ok := v["src"].(string); ok {
			out.Src = s
		}
		if s, ok := v["dest"].(string); ok {
			out.Dest = s
		}
		if s, ok := v["fstype"].(string); ok {
			out.FSType = s
		}
		if s, ok := v["state"].(string); ok {
			out.State = s
		}
		if opts, ok := v["options"].([]interface{}); ok {
			for _, x := range opts {
				if s, ok := x.(string); ok {
					out.Options = append(out.Options, s)
				}
			}
		}
		any := out.Src != "" || out.Dest != "" || out.FSType != "" ||
			out.State != "" || len(out.Options) > 0
		return out, any
	}
	return actions.MountDiff{}, false
}

type mountRenderer struct {
	op      actions.Operation
	payload actions.MountDiff
}

func (r *mountRenderer) Kind() string { return "mount" }

// Render writes a one-line summary:
//
//	mount add /dev/sdb /mnt/data ext4 (defaults,noatime)
//	mount add UUID=abcd /var/log xfs
//	mount unmount /mnt/data
//	mount remove /mnt/data
func (r *mountRenderer) Render(w io.Writer, format Format) error {
	switch format {
	case FormatText, "":
		verb := mountOperationVerb(r.op, r.payload.State)
		dest := r.payload.Dest
		if dest == "" {
			dest = "<unnamed>"
		}
		line := fmt.Sprintf("  mount %s", verb)
		// Delete + unmount intent: show identity only.
		if r.op == actions.OpDelete || r.payload.State == "unmounted" {
			line += " " + dest
		} else {
			// Add / update intent: show src + dest + fstype + options
			// when we have them.
			if r.payload.Src != "" {
				line += " " + r.payload.Src + " " + dest
			} else {
				line += " " + dest
			}
			if r.payload.FSType != "" {
				line += " " + r.payload.FSType
			}
			if len(r.payload.Options) > 0 {
				line += " (" + strings.Join(r.payload.Options, ",") + ")"
			}
		}
		_, err := fmt.Fprintln(w, line)
		return err
	case FormatJSON, FormatYAML:
		_, err := fmt.Fprintf(w, "mount %s: %s\n",
			mountOperationVerb(r.op, r.payload.State), r.payload.Dest)
		return err
	}
	return fmt.Errorf("unsupported format: %s", format)
}

// mountOperationVerb maps Op + State to the user-facing verb. The
// handler classifies State=unmounted as OpUpdate (fstab keeps but
// run-state changes); we use "unmount" instead of "update" for that
// case so the operator sees the actual lifecycle action.
func mountOperationVerb(op actions.Operation, state string) string {
	if op == actions.OpUpdate && state == "unmounted" {
		return "unmount"
	}
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
