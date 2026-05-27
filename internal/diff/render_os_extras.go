package diff

import (
	"fmt"
	"io"

	"github.com/alehatsman/mooncake/internal/actions"
)

func init() {
	Register(matchSSHKey)
	Register(matchSysctl)
}

// ------ os.ssh_key ----------------------------------------------------------

// matchSSHKey recognises a typed actions.Diff describing an
// os.ssh_key mutation. Dispatch on Attributes["kind"]=="os.ssh_key".
// Accepts typed pointer or JSON-decoded map shape.
func matchSSHKey(detail any) Renderer {
	d, ok := detail.(*actions.Diff)
	if !ok || d == nil {
		return nil
	}
	if d.Resource.Attributes["kind"] != "os.ssh_key" {
		return nil
	}
	after, ok := toSSHKeyDiff(d.After)
	if !ok {
		return nil
	}
	return &sshKeyRenderer{op: d.Operation, payload: after}
}

func toSSHKeyDiff(v any) (actions.SSHKeyDiff, bool) {
	switch x := v.(type) {
	case *actions.SSHKeyDiff:
		if x == nil {
			return actions.SSHKeyDiff{}, false
		}
		return *x, true
	case actions.SSHKeyDiff:
		return x, true
	case map[string]interface{}:
		out := actions.SSHKeyDiff{}
		if s, ok := x["user"].(string); ok {
			out.User = s
		}
		if s, ok := x["state"].(string); ok {
			out.State = s
		}
		if f, ok := x["key_count"].(float64); ok {
			out.KeyCount = int(f)
		}
		if s, ok := x["path"].(string); ok {
			out.Path = s
		}
		if b, ok := x["exclusive"].(bool); ok {
			out.Exclusive = b
		}
		any := out.User != "" || out.State != "" || out.KeyCount != 0 ||
			out.Path != "" || out.Exclusive
		return out, any
	}
	return actions.SSHKeyDiff{}, false
}

type sshKeyRenderer struct {
	op      actions.Operation
	payload actions.SSHKeyDiff
}

func (r *sshKeyRenderer) Kind() string { return "os.ssh_key" }

// Render writes a one-line summary:
//
//	os.ssh_key add deploy: 2 keys
//	os.ssh_key add deploy: 1 key [exclusive]
//	os.ssh_key add deploy: 1 key at /etc/ssh/keys
//	os.ssh_key remove deploy
func (r *sshKeyRenderer) Render(w io.Writer, format Format) error {
	switch format {
	case FormatText, "":
		verb := sshKeyVerb(r.op)
		user := r.payload.User
		if user == "" {
			user = "<unnamed>"
		}
		line := fmt.Sprintf("  os.ssh_key %s %s", verb, user)
		if r.op != actions.OpDelete {
			noun := "keys"
			if r.payload.KeyCount == 1 {
				noun = "key"
			}
			line += fmt.Sprintf(": %d %s", r.payload.KeyCount, noun)
			if r.payload.Path != "" {
				line += " at " + r.payload.Path
			}
			if r.payload.Exclusive {
				line += " [exclusive]"
			}
		}
		_, err := fmt.Fprintln(w, line)
		return err
	case FormatJSON, FormatYAML:
		_, err := fmt.Fprintf(w, "os.ssh_key %s: %s (%d keys)\n",
			sshKeyVerb(r.op), r.payload.User, r.payload.KeyCount)
		return err
	}
	return fmt.Errorf("unsupported format: %s", format)
}

func sshKeyVerb(op actions.Operation) string {
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

// ------ os.sysctl -----------------------------------------------------------

// matchSysctl recognises a typed actions.Diff describing an
// os.sysctl mutation. Dispatch on Attributes["kind"]=="os.sysctl".
// Accepts typed pointer or JSON-decoded map shape.
func matchSysctl(detail any) Renderer {
	d, ok := detail.(*actions.Diff)
	if !ok || d == nil {
		return nil
	}
	if d.Resource.Attributes["kind"] != "os.sysctl" {
		return nil
	}
	after, ok := toSysctlDiff(d.After)
	if !ok {
		return nil
	}
	return &sysctlRenderer{op: d.Operation, payload: after}
}

func toSysctlDiff(v any) (actions.SysctlDiff, bool) {
	switch x := v.(type) {
	case *actions.SysctlDiff:
		if x == nil {
			return actions.SysctlDiff{}, false
		}
		return *x, true
	case actions.SysctlDiff:
		return x, true
	case map[string]interface{}:
		out := actions.SysctlDiff{}
		if s, ok := x["name"].(string); ok {
			out.Name = s
		}
		if s, ok := x["state"].(string); ok {
			out.State = s
		}
		if s, ok := x["value"].(string); ok {
			out.Value = s
		}
		any := out.Name != "" || out.State != "" || out.Value != ""
		return out, any
	}
	return actions.SysctlDiff{}, false
}

type sysctlRenderer struct {
	op      actions.Operation
	payload actions.SysctlDiff
}

func (r *sysctlRenderer) Kind() string { return "os.sysctl" }

// Render writes a one-line summary:
//
//	os.sysctl set net.ipv4.ip_forward = 1
//	os.sysctl remove net.ipv4.ip_forward
func (r *sysctlRenderer) Render(w io.Writer, format Format) error {
	switch format {
	case FormatText, "":
		name := r.payload.Name
		if name == "" {
			name = "<unnamed>"
		}
		var line string
		switch r.op {
		case actions.OpDelete:
			line = fmt.Sprintf("  os.sysctl remove %s", name)
		case actions.OpNoop:
			line = fmt.Sprintf("  os.sysctl noop %s", name)
		default:
			line = fmt.Sprintf("  os.sysctl set %s", name)
			if r.payload.Value != "" {
				line += " = " + r.payload.Value
			}
		}
		_, err := fmt.Fprintln(w, line)
		return err
	case FormatJSON, FormatYAML:
		_, err := fmt.Fprintf(w, "os.sysctl %s: %s = %s\n",
			r.op, r.payload.Name, r.payload.Value)
		return err
	}
	return fmt.Errorf("unsupported format: %s", format)
}
