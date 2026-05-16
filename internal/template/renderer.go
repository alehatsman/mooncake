// Package template provides template rendering functionality using pongo2.
package template

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/flosch/pongo2/v6"
)

var (
	// templateExprRe matches {{ expr }} blocks, excluding {% %} control tags.
	templateExprRe = regexp.MustCompile(`\{\{[^%{][^}]*\}\}`)
	// rootVarRe extracts the root identifier from a {{ expr }} block.
	rootVarRe = regexp.MustCompile(`\{\{-?\s*([a-zA-Z_][a-zA-Z0-9_]*)`)
	// pongo2Builtins are names resolved from pongo2's internal context, not user vars.
	pongo2Builtins = map[string]bool{
		"true": true, "false": true, "none": true, "pongo2": true,
	}
)

var (
	// once ensures filters are registered only once
	once sync.Once
	// filterRegisterError stores any error from filter registration
	filterRegisterError error
)

// Renderer defines the interface for template rendering.
type Renderer interface {
	Render(template string, variables map[string]interface{}) (string, error)
	// RenderPreserving renders like Render but preserves {{ expr }} placeholders
	// for any root variable not present in variables, rather than silently
	// substituting empty string. Use this for plan-time rendering where
	// execute-time variables (registered results from previous steps) are
	// not yet in scope.
	RenderPreserving(template string, variables map[string]interface{}) (string, error)
}

// Pongo2Renderer implements Renderer using the pongo2 template engine.
type Pongo2Renderer struct {
	// Mutex to protect concurrent access to pongo2.FromString
	// The pongo2 TemplateSet is not thread-safe
	mu sync.Mutex
}

// NewPongo2Renderer creates a new Pongo2Renderer with filters registered.
// Returns an error if filter registration fails (e.g., filter name already registered).
func NewPongo2Renderer() (Renderer, error) {
	r := &Pongo2Renderer{}

	// Register custom filters once globally
	once.Do(func() {
		// pongo2 inherits Django's default of HTML-escaping output. That
		// makes sense for web templates and is dangerous nonsense for a
		// CLI/agent tool: a user logging "{{ x | default:'<unset>' }}"
		// would see "&lt;unset&gt;" in both the terminal and the
		// `print.message` event payload, and `log:` is the canonical
		// "show me a value" surface. Turn it off globally — mooncake
		// renders nothing to HTML.
		pongo2.SetAutoescape(false)

		if err := pongo2.RegisterFilter("expanduser", r.expandUserFilter); err != nil {
			// Store error for later return
			filterRegisterError = fmt.Errorf("failed to register expanduser filter: %w", err)
		}
		// MT-70: pongo2 renders non-scalar values (maps, slices) as
		// `<TYPE Value>` — a useless reflect.Value repr that leaks into
		// log:/print events when users try `{{ cfg }}` to inspect a
		// `read.json` result. Register `tojson` (and `json` alias) so
		// users have an explicit, well-defined way to serialize.
		if err := pongo2.RegisterFilter("tojson", toJSONFilter); err != nil {
			filterRegisterError = fmt.Errorf("failed to register tojson filter: %w", err)
		}
		if err := pongo2.RegisterFilter("json", toJSONFilter); err != nil {
			filterRegisterError = fmt.Errorf("failed to register json filter: %w", err)
		}
		// proposal-09: `{{ apply_started_at | strftime:"%Y%m%d_%H%M%S" }}`
		// for rolling-backup / timestamped-dest patterns. Pongo2's
		// built-in `{% now %}` tag silently no-ops in mooncake's
		// configuration (echoes the format string back), so playbooks
		// route through this filter instead and the scope ships an
		// `apply_started_at` variable set once per run.
		if err := pongo2.RegisterFilter("strftime", strftimeFilter); err != nil {
			filterRegisterError = fmt.Errorf("failed to register strftime filter: %w", err)
		}
	})

	// Check if filter registration failed
	if filterRegisterError != nil {
		return nil, filterRegisterError
	}

	return r, nil
}

// strftimeFilter formats a time.Time using POSIX strftime-style codes.
// Use as `{{ apply_started_at | strftime:"%Y%m%d_%H%M%S" }}`. Unknown
// codes return a pongo2 error so a typo in the format string fails
// loudly at render time rather than silently passing through (the
// proposal-09 design note: the original `{% now %}` tag was silent and
// the proposal explicitly asks for the opposite).
//
// Supported codes are the POSIX/strftime canon most playbooks reach
// for; see strftimeFormat for the full table. Direct computation
// (t.Year, t.Month, …) rather than routing through Go's reference
// time avoids the `2006`/`04`/etc. accidental-match pitfalls when the
// surrounding literal text contains digits.
func strftimeFilter(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	t, ok := in.Interface().(time.Time)
	if !ok {
		return nil, &pongo2.Error{
			Sender:    "filter:strftime",
			OrigError: fmt.Errorf("strftime: input is %T, want time.Time", in.Interface()),
		}
	}
	formatted, err := strftimeFormat(t, param.String())
	if err != nil {
		return nil, &pongo2.Error{
			Sender:    "filter:strftime",
			OrigError: err,
		}
	}
	return pongo2.AsValue(formatted), nil
}

// strftimeFormat walks the format string and substitutes each `%X`
// code with the corresponding piece of t. Literal text passes through
// unchanged. Returns an error on unknown codes (callers map this to
// a pongo2.Error so templates fail loudly).
//
// The supported set is the standard POSIX strftime subset plus a few
// common composites — anything a playbook is likely to reach for in
// a backup-suffix or audit-log filename:
//
//	%Y  4-digit year (2026)            %M  minute  (00–59)
//	%y  2-digit year (26)              %S  second  (00–59)
//	%m  month   (01–12)                %I  hour    (01–12)
//	%d  day     (01–31)                %p  AM/PM
//	%H  hour    (00–23)                %j  day-of-year (001–366)
//	%a  Mon abbrev (Mon)               %A  weekday  (Monday)
//	%b  Month abbrev (Jan)             %B  Month    (January)
//	%Z  TZ abbrev (MST)                %z  TZ offset (-0700)
//	%F  shortcut for %Y-%m-%d          %T  shortcut for %H:%M:%S
//	%R  shortcut for %H:%M             %%  literal '%'
func strftimeFormat(t time.Time, format string) (string, error) {
	var b strings.Builder
	b.Grow(len(format) + 8)
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			b.WriteByte(format[i])
			continue
		}
		if i+1 >= len(format) {
			return "", fmt.Errorf("strftime: trailing %% with no code")
		}
		i++
		switch format[i] {
		case 'Y':
			fmt.Fprintf(&b, "%04d", t.Year())
		case 'y':
			fmt.Fprintf(&b, "%02d", t.Year()%100)
		case 'm':
			fmt.Fprintf(&b, "%02d", int(t.Month()))
		case 'd':
			fmt.Fprintf(&b, "%02d", t.Day())
		case 'H':
			fmt.Fprintf(&b, "%02d", t.Hour())
		case 'I':
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			fmt.Fprintf(&b, "%02d", h)
		case 'M':
			fmt.Fprintf(&b, "%02d", t.Minute())
		case 'S':
			fmt.Fprintf(&b, "%02d", t.Second())
		case 'p':
			if t.Hour() < 12 {
				b.WriteString("AM")
			} else {
				b.WriteString("PM")
			}
		case 'j':
			fmt.Fprintf(&b, "%03d", t.YearDay())
		case 'a':
			b.WriteString(t.Weekday().String()[:3])
		case 'A':
			b.WriteString(t.Weekday().String())
		case 'b':
			b.WriteString(t.Month().String()[:3])
		case 'B':
			b.WriteString(t.Month().String())
		case 'Z':
			zone, _ := t.Zone()
			b.WriteString(zone)
		case 'z':
			b.WriteString(t.Format("-0700"))
		case 'F':
			fmt.Fprintf(&b, "%04d-%02d-%02d", t.Year(), int(t.Month()), t.Day())
		case 'T':
			fmt.Fprintf(&b, "%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second())
		case 'R':
			fmt.Fprintf(&b, "%02d:%02d", t.Hour(), t.Minute())
		case '%':
			b.WriteByte('%')
		default:
			return "", fmt.Errorf("strftime: unknown format code %%%c", format[i])
		}
	}
	return b.String(), nil
}

// toJSONFilter serializes any pongo2 Value as JSON. Use as
// `{{ var | tojson }}` (or the `json` alias). For maps/slices this
// produces the compact JSON representation instead of pongo2's
// `<TYPE Value>` reflect repr. Scalars round-trip through JSON encoding
// too — strings stay quoted, numbers stay numbers — so the filter is
// safe to apply unconditionally.
func toJSONFilter(in *pongo2.Value, _ *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	b, err := json.Marshal(in.Interface())
	if err != nil {
		return nil, &pongo2.Error{
			Sender:    "filter:tojson",
			OrigError: fmt.Errorf("tojson: %w", err),
		}
	}
	return pongo2.AsValue(string(b)), nil
}

// bareVarRe matches `{{ var }}` or `{{ var.field.field }}` patterns
// with a dotted identifier chain only — no filters, no arithmetic, no
// brackets. We rewrite those when the resolved value is a non-scalar
// so pongo2 doesn't emit `<TYPE Value>`; expressions with explicit
// filters (`{{ var | upper }}`) are left alone so the user's choice
// wins.
var bareVarRe = regexp.MustCompile(`\{\{-?\s*([a-zA-Z_][a-zA-Z0-9_]*(?:\.[a-zA-Z_][a-zA-Z0-9_]*)*)\s*-?\}\}`)

// autoJSONNonScalars rewrites `{{ var }}` and `{{ var.field.field }}`
// references whose resolved value is a Go map or slice into
// `{{ <path> | tojson }}`. Pongo2 stringifies those types as the
// useless `<TYPE Value>` form (see pongo2/v6 variable.go); the user
// almost always meant "show me the content", so we route through the
// tojson filter. Expressions that already carry a filter or index
// access fall through to pongo2 unchanged.
func autoJSONNonScalars(tmpl string, vars map[string]interface{}) string {
	return bareVarRe.ReplaceAllStringFunc(tmpl, func(match string) string {
		m := bareVarRe.FindStringSubmatch(match)
		if len(m) < 2 {
			return match
		}
		v, ok := resolveDottedPath(m[1], vars)
		if !ok || v == nil {
			return match
		}
		if !isNonScalar(v) {
			return match
		}
		// Inject the filter just before the closing `}}`, preserving
		// any pongo2 whitespace-control hyphen (`-}}`). Trim trailing
		// space on the head so `| tojson` sits next to the path.
		closeIdx := strings.LastIndex(match, "}}")
		if closeIdx < 0 {
			return match
		}
		head := strings.TrimRight(match[:closeIdx], " \t")
		return head + " | tojson " + match[closeIdx:]
	})
}

// resolveDottedPath walks a dotted access like `cfg.value.service`
// against the variables map. Returns the resolved value and true when
// every segment exists; otherwise (nil, false). Only handles map
// segments — that's all the rewrite needs.
func resolveDottedPath(path string, vars map[string]interface{}) (interface{}, bool) {
	parts := strings.Split(path, ".")
	var cur interface{} = vars
	for _, part := range parts {
		m, ok := cur.(map[string]interface{})
		if !ok {
			// Try map[string]string as a courtesy — Scope.ToMap merges
			// in some of those.
			if ms, okStr := cur.(map[string]string); okStr {
				v, present := ms[part]
				if !present {
					return nil, false
				}
				cur = v
				continue
			}
			return nil, false
		}
		v, present := m[part]
		if !present {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

// isNonScalar reports whether v is a Go map or slice — the two shapes
// pongo2 renders as `<TYPE Value>`. Everything else (strings, numbers,
// bools, structs, pointers) stays untouched.
func isNonScalar(v interface{}) bool {
	switch v.(type) {
	case map[string]interface{}, map[string]string,
		[]interface{}, []string, []map[string]interface{}:
		return true
	}
	return false
}

// expandUserFilter is a custom pongo2 filter for expanding ~ to user home directory
func (r *Pongo2Renderer) expandUserFilter(in *pongo2.Value, _ *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	path := in.String()

	// Expand ~ to $HOME
	// Note: Full path expansion (relative paths, etc.) happens in pathutil.ExpandPath
	// This filter only handles the ~ prefix to avoid circular dependencies
	if len(path) > 0 && path[0] == '~' {
		home := os.Getenv("HOME")
		if home == "" {
			return nil, &pongo2.Error{
				Sender:    "filter:expanduser",
				OrigError: fmt.Errorf("HOME environment variable not set"),
			}
		}

		if len(path) == 1 {
			// Just "~" expands to $HOME
			return pongo2.AsValue(home), nil
		} else if path[1] == '/' {
			// "~/foo" expands to "$HOME/foo"
			return pongo2.AsValue(home + path[1:]), nil
		}
		// "~foo" is left as-is (user home directory expansion not supported)
	}

	return pongo2.AsValue(path), nil
}

// RenderPreserving renders a template string with the given variables, preserving
// {{ expr }} placeholders for any root variable not present in variables.
// If the template contains any {%  %} control tags (for loops, if blocks, etc.),
// it falls back to regular Render — control tags introduce loop variables that
// are not in the user vars map and must not be intercepted.
func (r *Pongo2Renderer) RenderPreserving(tmpl string, vars map[string]interface{}) (string, error) {
	if vars == nil {
		vars = make(map[string]interface{})
	}
	if strings.Contains(tmpl, "{%") {
		return r.Render(tmpl, vars)
	}
	type entry struct{ placeholder, original string }
	var sentinels []entry
	n := 0

	modified := templateExprRe.ReplaceAllStringFunc(tmpl, func(expr string) string {
		m := rootVarRe.FindStringSubmatch(expr)
		if len(m) < 2 || pongo2Builtins[m[1]] {
			return expr
		}
		if _, ok := vars[m[1]]; ok {
			return expr
		}
		ph := fmt.Sprintf("__PRSV_%d__", n)
		n++
		sentinels = append(sentinels, entry{ph, expr})
		return ph
	})

	rendered, err := r.Render(modified, vars)
	if err != nil {
		return "", err
	}
	for _, s := range sentinels {
		rendered = strings.ReplaceAll(rendered, s.placeholder, s.original)
	}
	return rendered, nil
}

// Render renders a template string with the given variables.
func (r *Pongo2Renderer) Render(template string, variables map[string]interface{}) (string, error) {
	if variables == nil {
		variables = make(map[string]interface{})
	}

	// MT-70: pongo2 renders bare `{{ map_var }}` / `{{ slice_var }}` as
	// `<TYPE Value>` (its `Value.String()` for reflect.Map/Slice). For
	// our `log:` / `print.message` surface this leaks into JSON event
	// payloads with no recovery path. Auto-route those through the
	// `tojson` filter so users get the JSON content instead of an
	// internal reflect repr. Dotted access (`{{ var.field }}`) and
	// explicit filter chains keep their current rendering.
	template = autoJSONNonScalars(template, variables)

	// Lock to prevent race condition in pongo2.FromString
	// The pongo2 TemplateSet is not thread-safe for concurrent FromString calls
	r.mu.Lock()
	pongoTemplate, err := pongo2.FromString(template)
	r.mu.Unlock()

	if err != nil {
		return "", annotateFilterArgError(template, err)
	}

	output, err := pongoTemplate.Execute(variables)
	if err != nil {
		return "", err
	}

	return output, nil
}

// annotateFilterArgError detects the common-misuse signature of
// Jinja2-style filter arguments — `{{ x | default('y') }}` — which
// Pongo2 rejects in favor of its own `filter:value` syntax. When the
// error matches the signature, append a hint pointing the user at
// the correct form. Returns err unchanged when no signature match.
//
// Issue #18: every Ansible / Jinja2 tutorial uses `default('x')`;
// users will type it first, see Pongo2's "'}}' expected" pointing at
// the open paren, and conclude the engine is broken. The hint nudges
// them to the colon syntax without changing engine behavior.
func annotateFilterArgError(template string, err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	// Pongo2's parser error for the paren case has both markers:
	//   - "near '('" (the unexpected character)
	//   - "'}}' expected" (what it wanted instead)
	if !strings.Contains(msg, "near '('") || !strings.Contains(msg, "'}}' expected") {
		return err
	}
	// Verify the template actually has a `|<filter>(` pattern so we
	// don't surface a misleading hint when the open paren is from
	// some other construct.
	if !looksLikeFilterCall(template) {
		return err
	}
	return fmt.Errorf("%w\n  hint: pongo2 uses `{{ x | filter:value }}` (colon), not Jinja2's `filter(value)` (parens). Example: `{{ x | default:'fallback' }}`", err)
}

// looksLikeFilterCall returns true when the template contains a
// pipe-followed-by-identifier-followed-by-open-paren shape, the
// signature of a Jinja2 filter call. Conservative on purpose:
// emit the hint only when there's strong evidence the user reached
// for that form.
func looksLikeFilterCall(template string) bool {
	// Walk byte-wise; the shape we need to find is:
	//   `|` (with optional whitespace) → identifier-rune+ → `(`
	for i := 0; i < len(template); i++ {
		if template[i] != '|' {
			continue
		}
		j := i + 1
		for j < len(template) && (template[j] == ' ' || template[j] == '\t') {
			j++
		}
		// Identifier chars (Pongo2 filter names are ASCII).
		identStart := j
		for j < len(template) {
			c := template[j]
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || (c >= '0' && c <= '9' && j > identStart) {
				j++
				continue
			}
			break
		}
		if j == identStart {
			continue
		}
		if j < len(template) && template[j] == '(' {
			return true
		}
	}
	return false
}
