package executor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alehatsman/mooncake/internal/config"
)

// dx proposal-01: synthesize a step label when the operator omitted
// `name:`. The rendered label answers "what is this step doing?"
// using a short summary of the action type + its key field. Plumbs
// through both the human renderer and the JSON event channel via
// StepStartedData.Name — agents reading events see the same string
// the console shows.
//
// Truncation: synthesized labels never exceed labelMaxLen characters
// (with trailing ellipsis when cut). The truncation point is chosen
// so the rendered glyph row stays within ~80 columns even at the
// deepest indent (level 6 ≈ 12 chars of leading whitespace + glyph
// + space + label).

const (
	labelMaxLen = 60
	labelEllip  = "…"
)

// synthesizeStepName returns a "<action>: <key>" label for a step
// whose Name is empty. The output is never empty — when no key
// field is set, falls back to the action type itself. Returns ""
// only when the step is empty/unrecognized (Step.ActionType() == "").
//
// This is the single source of truth: getStepDisplayName calls into
// it, the JSON event channel surfaces the result, and any future
// renderer (TUI, log) can call it directly.
func synthesizeStepName(step config.Step) string {
	// Compound shapes (transaction / try) sit on the Step itself,
	// not via the action-type registry; DetermineActionType returns
	// "unknown" for them. Surface those first so a `try: [...]`
	// with no inner action still renders a useful label.
	if len(step.Transaction) > 0 {
		return truncateLabel("transaction" + sep("transaction", "(?)") + fmt.Sprintf("(%d children)", len(step.Transaction)))
	}
	if len(step.Try) > 0 {
		body := tryLabelBody(step)
		return truncateLabel("try" + sep("try", body) + body)
	}

	at := step.DetermineActionType()
	// DetermineActionType returns "unknown" for fully empty steps and
	// "loop" for bare for_each / with_files wrappers. Neither yields a
	// useful synthesized label — fall back to the existing empty-name
	// behaviour (the caller's renderer paints a glyph with no body, but
	// at least we don't print a misleading "unknown" line).
	if at == "" || at == "unknown" || at == "loop" {
		return ""
	}
	body := stepLabelBody(step, at)
	if body == "" {
		return at
	}
	return truncateLabel(at + sep(at, body) + body)
}

// sep picks the separator between the action type and the label
// body. Verbose actions whose body looks like prose (shell:
// "echo hi", log: "hello world") get a colon; identity actions
// whose body is a path or identifier get a space (file.write
// /tmp/x, pkg jq). The rule isn't a deep semantic one — it just
// matches the proposal's reference table without over-engineering.
func sep(actionType, body string) string {
	switch {
	case strings.HasPrefix(body, "/") || strings.HasPrefix(body, "→ "):
		return " "
	case actionType == "shell", actionType == "log", actionType == "print":
		return ": "
	case actionType == "pkg", actionType == "pkg.install", actionType == "pkg.remove":
		return " "
	case strings.HasPrefix(actionType, "file."), strings.HasPrefix(actionType, "os."):
		return " "
	default:
		return ": "
	}
}

// stepLabelBody extracts the key field from the step for the given
// action type. Each per-action arm is intentionally narrow: pick the
// most identifying field (path, URL, name, command), don't try to
// stringify the whole config.
//
// Returns "" when no good key field is set; the caller falls back to
// the bare action type.
func stepLabelBody(step config.Step, at string) string {
	switch at {
	case "shell":
		if step.Shell != nil {
			return collapseWhitespace(step.Shell.Cmd)
		}
	case "file.write":
		if step.FileWrite != nil {
			return step.FileWrite.Path
		}
	case "file.download":
		if step.FileDownload != nil {
			return "→ " + step.FileDownload.Dest
		}
	case "file.copy":
		if step.FileCopy != nil {
			return step.FileCopy.Dest
		}
	case "file.template":
		if step.FileTemplate != nil {
			return step.FileTemplate.Dest
		}
	case "pkg", "pkg.install":
		if step.Pkg != nil {
			return pkgLabelBody(step.Pkg.Name, step.Pkg.Names)
		}
	case "log", "print":
		if step.Log != nil {
			return collapseWhitespace(step.Log.Msg)
		}
	case "http.request":
		if step.HTTPRequest != nil {
			return strings.ToUpper(httpMethodOrGet(step.HTTPRequest.Method)) + " " + step.HTTPRequest.URL
		}
	case "import":
		if step.Import != nil {
			return *step.Import
		}
	case "vars":
		if step.Vars != nil {
			return fmt.Sprintf("(%d keys)", len(*step.Vars))
		}
	case "transaction":
		return fmt.Sprintf("(%d children)", len(step.Transaction))
	case "try":
		return tryLabelBody(step)
	case "assert":
		if step.Assert != nil {
			return assertLabelBody(step.Assert)
		}
	case "os.user":
		if step.OsUser != nil {
			return step.OsUser.Name
		}
	case "os.group":
		if step.OsGroup != nil {
			return step.OsGroup.Name
		}
	case "os.service":
		if step.OsService != nil {
			return step.OsService.Name
		}
	}
	return ""
}

// pkgLabelBody renders the pkg's identity field. Single name wins
// (the common case); a multi-name batch falls back to first-plus-
// count so the label stays short.
func pkgLabelBody(name string, names []string) string {
	if name != "" {
		return name
	}
	if len(names) == 1 {
		return names[0]
	}
	if len(names) > 1 {
		return fmt.Sprintf("%s (+%d)", names[0], len(names)-1)
	}
	return ""
}

// tryLabelBody describes a try-step. The (N children, +catch /
// +finally) form mirrors the proposal's reference table and
// surfaces the actual structure without forcing the operator to
// re-read the YAML.
func tryLabelBody(step config.Step) string {
	parts := []string{fmt.Sprintf("%d children", len(step.Try))}
	if len(step.Catch) > 0 {
		parts = append(parts, "+catch")
	}
	if len(step.Finally) > 0 {
		parts = append(parts, "+finally")
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// assertLabelBody picks the most identifying field on an assert
// block. The assert action has multiple one-of sub-fields; we walk
// them in a deterministic order so labels stay stable across runs.
func assertLabelBody(a *config.Assert) string {
	if a.Command != nil {
		return "command"
	}
	if a.File != nil {
		return "file " + a.File.Path
	}
	if a.FileSHA256 != nil {
		return "file_sha256 " + a.FileSHA256.Path
	}
	// Surface the first set sub-field name so the label is at least
	// distinguishable across multiple unnamed assert steps.
	for _, name := range assertSubFieldNames(a) {
		return name
	}
	return ""
}

// assertSubFieldNames returns the names of populated Assert
// sub-fields, sorted for determinism. Used by assertLabelBody when
// no canonical "most identifying" field is set; gives operators
// SOMETHING to disambiguate adjacent unnamed asserts.
func assertSubFieldNames(a *config.Assert) []string {
	names := []string{}
	if a.Command != nil {
		names = append(names, "command")
	}
	if a.File != nil {
		names = append(names, "file")
	}
	if a.FileSHA256 != nil {
		names = append(names, "file_sha256")
	}
	sort.Strings(names)
	return names
}

// httpMethodOrGet defaults the HTTP method to GET when unset,
// matching the http.request handler's own normalization. Keeps the
// rendered label consistent with what the action actually does.
func httpMethodOrGet(m string) string {
	if strings.TrimSpace(m) == "" {
		return "GET"
	}
	return m
}

// collapseWhitespace squeezes runs of whitespace (including
// newlines from multi-line shell commands) into a single space so
// the label renders cleanly on one row. Doesn't trim leading /
// trailing whitespace beyond what TrimSpace handles — the
// truncation guard catches anything pathological.
func collapseWhitespace(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	out := strings.Builder{}
	out.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		isSpace := r == ' ' || r == '\t' || r == '\n' || r == '\r'
		if isSpace {
			if !prevSpace {
				out.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		out.WriteRune(r)
		prevSpace = false
	}
	return out.String()
}

// truncateLabel cuts a synthesized label to labelMaxLen runes (not
// bytes — a multi-byte ellipsis appended to a byte-truncated string
// would land in the middle of a code point). Cuts at labelMaxLen-1
// to leave room for the ellipsis.
func truncateLabel(s string) string {
	if s == "" {
		return ""
	}
	rs := []rune(s)
	if len(rs) <= labelMaxLen {
		return s
	}
	return string(rs[:labelMaxLen-1]) + labelEllip
}
