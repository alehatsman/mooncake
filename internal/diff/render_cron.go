package diff

import (
	"fmt"
	"io"

	"github.com/alehatsman/mooncake/internal/actions"
)

func init() {
	Register(matchCron)
}

// matchCron recognises a typed actions.Diff describing an os.cron
// mutation. Dispatch is on Attributes["kind"]=="os.cron". Accepts
// the typed *actions.CronDiff pointer or the JSON-decoded map shape.
func matchCron(detail any) Renderer {
	d, ok := detail.(*actions.Diff)
	if !ok || d == nil {
		return nil
	}
	if d.Resource.Attributes["kind"] != "os.cron" {
		return nil
	}
	cd, ok := toCronDiff(d.After)
	if !ok {
		return nil
	}
	return &cronRenderer{op: d.Operation, payload: cd}
}

func toCronDiff(after any) (actions.CronDiff, bool) {
	switch v := after.(type) {
	case *actions.CronDiff:
		if v == nil {
			return actions.CronDiff{}, false
		}
		return *v, true
	case actions.CronDiff:
		return v, true
	case map[string]interface{}:
		out := actions.CronDiff{}
		if s, ok := v["name"].(string); ok {
			out.Name = s
		}
		if s, ok := v["state"].(string); ok {
			out.State = s
		}
		if s, ok := v["user"].(string); ok {
			out.User = s
		}
		if s, ok := v["schedule"].(string); ok {
			out.Schedule = s
		}
		if s, ok := v["command"].(string); ok {
			out.Command = s
		}
		any := out.Name != "" || out.State != "" || out.User != "" ||
			out.Schedule != "" || out.Command != ""
		return out, any
	}
	return actions.CronDiff{}, false
}

type cronRenderer struct {
	op      actions.Operation
	payload actions.CronDiff
}

func (r *cronRenderer) Kind() string { return "cron" }

// Render writes a one-line summary:
//
//	cron add backup [user=deploy]: 0 3 * * * /usr/local/bin/backup.sh
//	cron add backup: @daily backup.sh
//	cron remove backup
func (r *cronRenderer) Render(w io.Writer, format Format) error {
	switch format {
	case FormatText, "":
		verb := cronOperationVerb(r.op)
		name := r.payload.Name
		if name == "" {
			name = "<unnamed>"
		}
		line := fmt.Sprintf("  cron %s %s", verb, name)
		if r.payload.User != "" {
			line += fmt.Sprintf(" [user=%s]", r.payload.User)
		}
		// Delete steps don't carry a schedule/command — the entry
		// is being removed, so show only the identity.
		if r.op != actions.OpDelete && (r.payload.Schedule != "" || r.payload.Command != "") {
			line += ":"
			if r.payload.Schedule != "" {
				line += " " + r.payload.Schedule
			}
			if r.payload.Command != "" {
				line += " " + r.payload.Command
			}
		}
		_, err := fmt.Fprintln(w, line)
		return err
	case FormatJSON, FormatYAML:
		_, err := fmt.Fprintf(w, "cron %s: %s\n", cronOperationVerb(r.op), r.payload.Name)
		return err
	}
	return fmt.Errorf("unsupported format: %s", format)
}

// cronOperationVerb — add (create) / update / remove (delete) / noop.
func cronOperationVerb(op actions.Operation) string {
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
