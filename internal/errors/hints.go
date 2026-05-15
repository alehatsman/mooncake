// Package errors provides error hint inference for structured step failure output.
package errors

import (
	"fmt"
	"regexp"
	"strings"
)

// Hint holds an inferred human-readable suggestion and an optional YAML fix.
type Hint struct {
	Text          string
	SuggestedStep string
}

var (
	// Capture the command name from bash/sh's "<shell>: <cmd>: command not found"
	// form before falling back to the postfix "command not found: <cmd>" form.
	// Both shapes terminate the captured name with a colon or whitespace so
	// the suggestion doesn't emit a trailing punctuation character.
	reCommandNotFoundPrefix = regexp.MustCompile(`(?i)[^:\s\n]*sh:[^:\n]*?:\s*([^:\s\n]+):\s*command not found`)
	reCommandNotFound       = regexp.MustCompile(`(?i)command not found[:\s]+([^:\s\n]+)`)
	reCurlNotFound          = regexp.MustCompile(`(?i)curl: command not found`)
	reNoSuchFile            = regexp.MustCompile(`(?i)no such file or directory[:\s]+'?([^'\n]+)'?`)
)

type hintRule struct {
	match func(stderr string) (bool, string)
	hint  func(match string) Hint
}

var rules = []hintRule{
	// "curl: command not found" — specific before the generic one
	{
		match: func(s string) (bool, string) { return reCurlNotFound.MatchString(s), "curl" },
		hint: func(_ string) Hint {
			return Hint{
				Text:          "curl is not installed",
				SuggestedStep: "pkg:\n  name: curl\n  state: present",
			}
		},
	},
	// "<shell>: <cmd>: command not found" — the bash/sh prefix form. Matched
	// before the postfix rule so we extract the right field when both shapes
	// could in theory bracket the literal.
	{
		match: func(s string) (bool, string) {
			m := reCommandNotFoundPrefix.FindStringSubmatch(s)
			if m == nil {
				return false, ""
			}
			return true, m[1]
		},
		hint: func(name string) Hint {
			return Hint{
				Text:          fmt.Sprintf("%s is not installed", name),
				SuggestedStep: fmt.Sprintf("pkg:\n  name: %s\n  state: present", name),
			}
		},
	},
	// "command not found: <name>"
	{
		match: func(s string) (bool, string) {
			m := reCommandNotFound.FindStringSubmatch(s)
			if m == nil {
				return false, ""
			}
			return true, m[1]
		},
		hint: func(name string) Hint {
			return Hint{
				Text:          fmt.Sprintf("%s is not installed", name),
				SuggestedStep: fmt.Sprintf("pkg:\n  name: %s\n  state: present", name),
			}
		},
	},
	// "permission denied"
	{
		match: func(s string) (bool, string) {
			return strings.Contains(strings.ToLower(s), "permission denied") || strings.Contains(s, "EACCES"), ""
		},
		hint: func(_ string) Hint {
			return Hint{Text: "insufficient permissions; try running with sudo"}
		},
	},
	// "No such file or directory"
	{
		match: func(s string) (bool, string) {
			m := reNoSuchFile.FindStringSubmatch(s)
			if m == nil {
				return false, ""
			}
			return true, strings.TrimSpace(m[1])
		},
		hint: func(path string) Hint {
			return Hint{Text: fmt.Sprintf("path does not exist: %s", path)}
		},
	},
	// "address already in use"
	{
		match: func(s string) (bool, string) {
			return strings.Contains(strings.ToLower(s), "address already in use"), ""
		},
		hint: func(_ string) Hint {
			return Hint{Text: "port already bound; check running processes"}
		},
	},
}

// InferHint returns the first matching hint for the given stderr output.
// Returns a zero-value Hint (empty Text) when no rule matches.
func InferHint(stderr string) Hint {
	for _, r := range rules {
		ok, match := r.match(stderr)
		if ok {
			return r.hint(match)
		}
	}
	return Hint{}
}
