package diff

import (
	"fmt"
	"io"

	"github.com/alehatsman/mooncake/internal/actions"
)

func init() {
	Register(matchFirewall)
}

// matchFirewall recognises a typed actions.Diff describing an
// os.firewall mutation. The handler uses ResourceOther +
// Attributes["kind"]=="os.firewall"; dispatch is on the attribute
// rather than a dedicated ResourceKind constant.
//
// Accepts both the typed *actions.FirewallDiff After and the
// map[string]interface{} JSON-decoded shape.
func matchFirewall(detail any) Renderer {
	d, ok := detail.(*actions.Diff)
	if !ok || d == nil {
		return nil
	}
	if d.Resource.Attributes["kind"] != "os.firewall" {
		return nil
	}
	fd, ok := toFirewallDiff(d.After)
	if !ok {
		return nil
	}
	return &firewallRenderer{op: d.Operation, payload: fd}
}

func toFirewallDiff(after any) (actions.FirewallDiff, bool) {
	switch v := after.(type) {
	case *actions.FirewallDiff:
		if v == nil {
			return actions.FirewallDiff{}, false
		}
		return *v, true
	case actions.FirewallDiff:
		return v, true
	case map[string]interface{}:
		out := actions.FirewallDiff{}
		if s, ok := v["backend"].(string); ok {
			out.Backend = s
		}
		if s, ok := v["state"].(string); ok {
			out.State = s
		}
		if c, ok := v["rule_count"].(float64); ok {
			out.RuleCount = int(c)
		}
		// Backend defaults to "auto" in the handler; in the JSON
		// path a missing backend means nothing usable to render.
		any := out.Backend != "" || out.State != "" || out.RuleCount > 0
		return out, any
	}
	return actions.FirewallDiff{}, false
}

type firewallRenderer struct {
	op      actions.Operation
	payload actions.FirewallDiff
}

func (r *firewallRenderer) Kind() string { return "firewall" }

// Render writes a one-line summary. Rule details are intentionally
// omitted (per FirewallDiff doc: rule payloads may carry sensitive
// ports / source addresses). The renderer surfaces backend, count,
// and intent:
//
//	firewall add ufw: 3 rules
//	firewall add ufw: 1 rule
//	firewall remove ufw: 2 rules
func (r *firewallRenderer) Render(w io.Writer, format Format) error {
	switch format {
	case FormatText, "":
		verb := firewallOperationVerb(r.op)
		backend := r.payload.Backend
		if backend == "" {
			backend = "auto"
		}
		unit := "rules"
		if r.payload.RuleCount == 1 {
			unit = "rule"
		}
		_, err := fmt.Fprintf(w, "  firewall %s %s: %d %s\n",
			verb, backend, r.payload.RuleCount, unit)
		return err
	case FormatJSON, FormatYAML:
		_, err := fmt.Fprintf(w, "firewall %s: %d rules\n",
			firewallOperationVerb(r.op), r.payload.RuleCount)
		return err
	}
	return fmt.Errorf("unsupported format: %s", format)
}

// firewallOperationVerb maps Op{Create,Update,Delete,Noop} to the
// user-facing verb for plan output. Firewall steps don't have an
// "upgrade" notion so OpUpdate maps to "update" (the literal verb)
// rather than a domain-specific verb.
func firewallOperationVerb(op actions.Operation) string {
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
