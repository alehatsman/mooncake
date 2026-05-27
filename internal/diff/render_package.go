package diff

import (
	"fmt"
	"io"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
)

func init() {
	Register(matchPackage)
	Register(matchPkgUpgrade)
}

// matchPackage recognises a typed actions.Diff describing a package
// mutation. The Diff arrives either:
//
//   - as *actions.Diff with Resource.Kind == ResourcePackage (the
//     in-memory plan path), After populated with *actions.PackageDiff;
//   - as *actions.Diff after json-decoding (a saved JSON plan
//     re-loaded), where After is now a map[string]interface{}.
//
// Both shapes are handled by toPackageDiff below.
//
// Spec-66 wave 6 added a sibling pkg.repo renderer that also reports
// Resource.Kind == ResourcePackage but discriminates via
// Attributes["kind"] == "pkg.repo". Defer to that matcher when the
// attribute is set; the pkg handler itself sets no kind attribute
// (left to a future cleanup that adds it for symmetry).
func matchPackage(detail any) Renderer {
	d, ok := detail.(*actions.Diff)
	if !ok || d == nil {
		return nil
	}
	if d.Resource.Kind != actions.ResourcePackage {
		return nil
	}
	if k := d.Resource.Attributes["kind"]; k != "" && k != "pkg" {
		return nil
	}
	pd, ok := toPackageDiff(d.After)
	if !ok {
		return nil
	}
	return &packageRenderer{op: d.Operation, payload: pd}
}

// toPackageDiff coerces a Diff.After value into the typed PackageDiff
// shape regardless of whether it arrived as the typed pointer or a
// json-decoded generic map.
func toPackageDiff(after any) (actions.PackageDiff, bool) {
	switch v := after.(type) {
	case *actions.PackageDiff:
		if v == nil {
			return actions.PackageDiff{}, false
		}
		return *v, true
	case actions.PackageDiff:
		return v, true
	case map[string]interface{}:
		out := actions.PackageDiff{}
		if names, ok := v["names"].([]interface{}); ok {
			for _, n := range names {
				if s, ok := n.(string); ok {
					out.Names = append(out.Names, s)
				}
			}
		}
		if s, ok := v["state"].(string); ok {
			out.State = s
		}
		if s, ok := v["manager"].(string); ok {
			out.Manager = s
		}
		return out, len(out.Names) > 0 || out.State != "" || out.Manager != ""
	}
	return actions.PackageDiff{}, false
}

type packageRenderer struct {
	op      actions.Operation
	payload actions.PackageDiff
}

func (r *packageRenderer) Kind() string { return "package" }

// Render writes a one-line summary of the package mutation. Today's
// pkg.Differ does not probe the package manager at plan time so there
// are no before/after versions to print; the renderer surfaces what
// IS available: the verb (install/upgrade/remove), the manager (when
// pinned), and the package list.
//
// Sample text output (two-space indent applied by the renderer so
// callers paste it under a step header without re-indenting):
//
//	pkg install apt: vim
//	pkg install: vim, tmux, git
//	pkg upgrade brew: terraform
//	pkg remove apt: nginx
func (r *packageRenderer) Render(w io.Writer, format Format) error {
	switch format {
	case FormatText, "":
		verb := operationVerb(r.op)
		mgr := ""
		if r.payload.Manager != "" {
			mgr = " " + r.payload.Manager
		}
		names := strings.Join(r.payload.Names, ", ")
		if names == "" {
			names = "<unnamed>"
		}
		_, err := fmt.Fprintf(w, "  pkg %s%s: %s\n", verb, mgr, names)
		return err
	case FormatJSON, FormatYAML:
		// JSON / YAML callers serialize the plan struct directly via
		// the existing encoder; the typed PackageDiff already marshals.
		// Returning a one-line text fallback keeps a future Render
		// caller in these formats useful without dragging in a typed
		// marshaller here.
		verb := operationVerb(r.op)
		_, err := fmt.Fprintf(w, "pkg %s: %s\n", verb, strings.Join(r.payload.Names, ","))
		return err
	}
	return fmt.Errorf("unsupported format: %s", format)
}

// operationVerb maps spec-22 Operation values to the user-facing verb
// shown in `mooncake plan --diff` output. Unknown operations fall
// back to "change" rather than the raw enum string to keep operator
// output readable.
func operationVerb(op actions.Operation) string {
	switch op {
	case actions.OpCreate:
		return "install"
	case actions.OpUpdate:
		return "upgrade"
	case actions.OpDelete:
		return "remove"
	case actions.OpNoop:
		return "noop"
	default:
		return "change"
	}
}

// matchPkgUpgrade recognises a typed actions.Diff describing a
// pkg.upgrade mutation. Dispatch on Attributes["kind"]=="pkg.upgrade".
// Accepts typed pointer or JSON-decoded map shape.
func matchPkgUpgrade(detail any) Renderer {
	d, ok := detail.(*actions.Diff)
	if !ok || d == nil {
		return nil
	}
	if d.Resource.Attributes["kind"] != "pkg.upgrade" {
		return nil
	}
	after, ok := toPkgUpgradeDiff(d.After)
	if !ok {
		return nil
	}
	return &pkgUpgradeRenderer{op: d.Operation, payload: after}
}

func toPkgUpgradeDiff(v any) (actions.PkgUpgradeDiff, bool) {
	switch x := v.(type) {
	case *actions.PkgUpgradeDiff:
		if x == nil {
			return actions.PkgUpgradeDiff{}, false
		}
		return *x, true
	case actions.PkgUpgradeDiff:
		return x, true
	case map[string]interface{}:
		out := actions.PkgUpgradeDiff{}
		if names, ok := x["names"].([]interface{}); ok {
			for _, n := range names {
				if s, ok := n.(string); ok {
					out.Names = append(out.Names, s)
				}
			}
		}
		if b, ok := x["autoremove"].(bool); ok {
			out.Autoremove = b
		}
		if s, ok := x["manager"].(string); ok {
			out.Manager = s
		}
		if b, ok := x["full_upgrade"].(bool); ok {
			out.FullUpgrade = b
		}
		any := len(out.Names) > 0 || out.Autoremove || out.Manager != "" || out.FullUpgrade
		return out, any
	}
	return actions.PkgUpgradeDiff{}, false
}

type pkgUpgradeRenderer struct {
	op      actions.Operation
	payload actions.PkgUpgradeDiff
}

func (r *pkgUpgradeRenderer) Kind() string { return "pkg.upgrade" }

// Render writes a one-line summary:
//
//	pkg.upgrade apt: <system> [autoremove]
//	pkg.upgrade dnf: nginx, redis
//	pkg.upgrade: <system>
func (r *pkgUpgradeRenderer) Render(w io.Writer, format Format) error {
	switch format {
	case FormatText, "":
		mgr := ""
		if r.payload.Manager != "" {
			mgr = " " + r.payload.Manager
		}
		target := "<system>"
		if !r.payload.FullUpgrade && len(r.payload.Names) > 0 {
			target = strings.Join(r.payload.Names, ", ")
		}
		line := fmt.Sprintf("  pkg.upgrade%s: %s", mgr, target)
		if r.payload.Autoremove {
			line += " [autoremove]"
		}
		_, err := fmt.Fprintln(w, line)
		return err
	case FormatJSON, FormatYAML:
		_, err := fmt.Fprintf(w, "pkg.upgrade %s: full=%t names=%d\n",
			r.payload.Manager, r.payload.FullUpgrade, len(r.payload.Names))
		return err
	}
	return fmt.Errorf("unsupported format: %s", format)
}
