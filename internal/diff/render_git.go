package diff

import (
	"fmt"
	"io"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
)

func init() {
	Register(matchGitCheckout)
	Register(matchGitConfig)
	Register(matchGitClone)
}

// matchGitCheckout recognises a typed actions.Diff describing a
// git.checkout mutation. Dispatch is on Attributes["kind"]=="git.checkout".
// Accepts the typed *actions.GitCheckoutDiff pointer or the JSON-
// decoded map shape.
func matchGitCheckout(detail any) Renderer {
	d, ok := detail.(*actions.Diff)
	if !ok || d == nil {
		return nil
	}
	if d.Resource.Attributes["kind"] != "git.checkout" {
		return nil
	}
	after, ok := toGitCheckoutDiff(d.After)
	if !ok {
		return nil
	}
	before, _ := toGitCheckoutDiff(d.Before)
	return &gitCheckoutRenderer{op: d.Operation, after: after, before: before}
}

func toGitCheckoutDiff(v any) (actions.GitCheckoutDiff, bool) {
	switch x := v.(type) {
	case *actions.GitCheckoutDiff:
		if x == nil {
			return actions.GitCheckoutDiff{}, false
		}
		return *x, true
	case actions.GitCheckoutDiff:
		return x, true
	case map[string]interface{}:
		out := actions.GitCheckoutDiff{}
		if s, ok := x["dest"].(string); ok {
			out.Dest = s
		}
		if s, ok := x["ref"].(string); ok {
			out.Ref = s
		}
		if s, ok := x["head_sha"].(string); ok {
			out.HeadSHA = s
		}
		any := out.Dest != "" || out.Ref != "" || out.HeadSHA != ""
		return out, any
	}
	return actions.GitCheckoutDiff{}, false
}

type gitCheckoutRenderer struct {
	op     actions.Operation
	after  actions.GitCheckoutDiff
	before actions.GitCheckoutDiff
}

func (r *gitCheckoutRenderer) Kind() string { return "git.checkout" }

// Render writes a one-line summary of the requested checkout:
//
//	git.checkout update /srv/app to v1.2.3 (from abc1234)
//	git.checkout update /srv/app to v1.2.3
//	git.checkout noop /srv/app at v1.2.3
func (r *gitCheckoutRenderer) Render(w io.Writer, format Format) error {
	switch format {
	case FormatText, "":
		verb := gitCheckoutVerb(r.op)
		dest := r.after.Dest
		if dest == "" {
			dest = "<unknown-dest>"
		}
		line := fmt.Sprintf("  git.checkout %s %s", verb, dest)
		if r.after.Ref != "" {
			if r.op == actions.OpNoop {
				line += " at " + r.after.Ref
			} else {
				line += " to " + r.after.Ref
			}
		}
		if r.before.HeadSHA != "" && r.op != actions.OpNoop {
			line += " (from " + shortSHA(r.before.HeadSHA) + ")"
		}
		_, err := fmt.Fprintln(w, line)
		return err
	case FormatJSON, FormatYAML:
		_, err := fmt.Fprintf(w, "git.checkout %s: %s -> %s\n",
			gitCheckoutVerb(r.op), r.after.Dest, r.after.Ref)
		return err
	}
	return fmt.Errorf("unsupported format: %s", format)
}

// matchGitConfig recognises a typed actions.Diff describing a
// git.config mutation. Dispatch is on Attributes["kind"]=="git.config".
// Accepts the typed *actions.GitConfigDiff pointer or the JSON-
// decoded map shape.
func matchGitConfig(detail any) Renderer {
	d, ok := detail.(*actions.Diff)
	if !ok || d == nil {
		return nil
	}
	if d.Resource.Attributes["kind"] != "git.config" {
		return nil
	}
	after, ok := toGitConfigDiff(d.After)
	if !ok {
		return nil
	}
	return &gitConfigRenderer{op: d.Operation, payload: after}
}

func toGitConfigDiff(v any) (actions.GitConfigDiff, bool) {
	switch x := v.(type) {
	case *actions.GitConfigDiff:
		if x == nil {
			return actions.GitConfigDiff{}, false
		}
		return *x, true
	case actions.GitConfigDiff:
		return x, true
	case map[string]interface{}:
		out := actions.GitConfigDiff{}
		if s, ok := x["scope"].(string); ok {
			out.Scope = s
		}
		if s, ok := x["repo"].(string); ok {
			out.Repo = s
		}
		if entries, ok := x["entries"].([]interface{}); ok {
			for _, raw := range entries {
				m, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				e := actions.GitConfigEntry{}
				if s, ok := m["key"].(string); ok {
					e.Key = s
				}
				if s, ok := m["value"].(string); ok {
					e.Value = s
				}
				if s, ok := m["op"].(string); ok {
					e.Op = s
				}
				if e.Key != "" {
					out.Entries = append(out.Entries, e)
				}
			}
		}
		any := out.Scope != "" || out.Repo != "" || len(out.Entries) > 0
		return out, any
	}
	return actions.GitConfigDiff{}, false
}

type gitConfigRenderer struct {
	op      actions.Operation
	payload actions.GitConfigDiff
}

func (r *gitConfigRenderer) Kind() string { return "git.config" }

// Render writes a multi-line summary — header + one indented line per
// entry. Compact when entries fit on one line:
//
//	git.config noop global
//	git.config update global
//	    set user.email = dev@example.com
//	    set core.autocrlf = false
//	    unset push.default
//	git.config update local /srv/myrepo
//	    set core.hooksPath = .githooks
func (r *gitConfigRenderer) Render(w io.Writer, format Format) error {
	switch format {
	case FormatText, "":
		scope := r.payload.Scope
		if scope == "" {
			scope = "<unknown-scope>"
		}
		header := fmt.Sprintf("  git.config %s %s", gitConfigVerb(r.op), scope)
		if r.payload.Repo != "" {
			header += " " + r.payload.Repo
		}
		if _, err := fmt.Fprintln(w, header); err != nil {
			return err
		}
		for _, e := range r.payload.Entries {
			var line string
			switch e.Op {
			case "set":
				line = fmt.Sprintf("    set %s = %s", e.Key, e.Value)
			case "unset":
				line = fmt.Sprintf("    unset %s", e.Key)
			default:
				line = fmt.Sprintf("    %s %s", e.Op, e.Key)
			}
			if _, err := fmt.Fprintln(w, line); err != nil {
				return err
			}
		}
		return nil
	case FormatJSON, FormatYAML:
		_, err := fmt.Fprintf(w, "git.config %s: scope=%s entries=%d\n",
			gitConfigVerb(r.op), r.payload.Scope, len(r.payload.Entries))
		return err
	}
	return fmt.Errorf("unsupported format: %s", format)
}

// gitCheckoutVerb / gitConfigVerb map spec-22 Operation → user-facing
// verb. Both verbs share the same vocabulary as the other renderers
// (create/update/remove/noop); git.checkout adds a "noop" pathway
// even though its handler always reports OpUpdate today, in
// anticipation of the spec-66 wave 8 audit which may downgrade
// already-on-ref steps to OpNoop.
func gitCheckoutVerb(op actions.Operation) string {
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

func gitConfigVerb(op actions.Operation) string {
	switch op {
	case actions.OpUpdate:
		return "update"
	case actions.OpNoop:
		return "noop"
	case actions.OpCreate:
		return "create"
	case actions.OpDelete:
		return "remove"
	default:
		return "change"
	}
}

// shortSHA truncates a hex SHA to 7 chars for readability; passes
// shorter inputs through unchanged.
func shortSHA(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

// matchGitClone recognises a typed actions.Diff describing a
// git.clone mutation. Dispatch is on Attributes["kind"]=="git.clone".
// Accepts the typed *actions.GitCloneDiff pointer or the JSON-
// decoded map shape.
func matchGitClone(detail any) Renderer {
	d, ok := detail.(*actions.Diff)
	if !ok || d == nil {
		return nil
	}
	if d.Resource.Attributes["kind"] != "git.clone" {
		return nil
	}
	after, ok := toGitCloneDiff(d.After)
	if !ok {
		return nil
	}
	before, _ := toGitCloneDiff(d.Before)
	return &gitCloneRenderer{op: d.Operation, after: after, before: before}
}

func toGitCloneDiff(v any) (actions.GitCloneDiff, bool) {
	switch x := v.(type) {
	case *actions.GitCloneDiff:
		if x == nil {
			return actions.GitCloneDiff{}, false
		}
		return *x, true
	case actions.GitCloneDiff:
		return x, true
	case map[string]interface{}:
		out := actions.GitCloneDiff{}
		if s, ok := x["dest"].(string); ok {
			out.Dest = s
		}
		if s, ok := x["repo"].(string); ok {
			out.Repo = s
		}
		if s, ok := x["ref"].(string); ok {
			out.Ref = s
		}
		if s, ok := x["head_sha"].(string); ok {
			out.HeadSHA = s
		}
		any := out.Dest != "" || out.Repo != "" || out.Ref != "" || out.HeadSHA != ""
		return out, any
	}
	return actions.GitCloneDiff{}, false
}

type gitCloneRenderer struct {
	op     actions.Operation
	after  actions.GitCloneDiff
	before actions.GitCloneDiff
}

func (r *gitCloneRenderer) Kind() string { return "git.clone" }

// Render writes a one-line summary:
//
//	git.clone create /srv/app from https://github.com/x/y at main
//	git.clone update /srv/app from https://github.com/x/y at v1.2.3 (from abc1234)
//	git.clone noop /srv/app at v1.2.3
func (r *gitCloneRenderer) Render(w io.Writer, format Format) error {
	switch format {
	case FormatText, "":
		verb := gitCheckoutVerb(r.op) // reuse create/update/remove/noop vocab
		dest := r.after.Dest
		if dest == "" {
			dest = "<unknown-dest>"
		}
		line := fmt.Sprintf("  git.clone %s %s", verb, dest)
		if r.op != actions.OpNoop && r.after.Repo != "" {
			line += " from " + r.after.Repo
		}
		if r.after.Ref != "" {
			if r.op == actions.OpNoop {
				line += " at " + r.after.Ref
			} else {
				line += " at " + r.after.Ref
			}
		}
		if r.before.HeadSHA != "" && r.op != actions.OpNoop {
			line += " (from " + shortSHA(r.before.HeadSHA) + ")"
		}
		_, err := fmt.Fprintln(w, line)
		return err
	case FormatJSON, FormatYAML:
		_, err := fmt.Fprintf(w, "git.clone %s: %s <- %s @ %s\n",
			gitCheckoutVerb(r.op), r.after.Dest, r.after.Repo, r.after.Ref)
		return err
	}
	return fmt.Errorf("unsupported format: %s", format)
}
