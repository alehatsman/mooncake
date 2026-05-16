package explain

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// findExamples walks the examples tree and returns excerpts where the
// given action appears as a step key. Best-effort — silently returns
// nil if the tree isn't present.
//
// Match rule: a line of the form `<indent>- action:` or
// `<indent>action:` where action is exactly the noun. We pick the line
// with the action key plus up to 5 indented lines after it as the
// excerpt.
//
// Limited by Options.ExamplesLimit (default 3). The cap protects
// MCP-tool output budget; the agent can re-call with a higher limit.
func findExamples(noun string, opts Options) []ExampleHit {
	// F044: zero is a valid request meaning "no examples please"; any
	// negative value (including the zero-Options sentinel) means
	// "caller has no preference — use the default of 3." This matches
	// the MCP/CLI wire contract where the schema declares minimum: 0
	// and the absent-or-omitted case maps to <0 at the boundary.
	limit := opts.ExamplesLimit
	if limit < 0 {
		limit = 3
	}
	if limit == 0 {
		return nil
	}

	root := opts.ExamplesRoot
	if root == "" {
		root = defaultExamplesRoot()
	}
	if root == "" {
		return nil
	}

	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".yml") && !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil
	}
	sort.Strings(files)

	var hits []ExampleHit
	for _, f := range files {
		hit, ok := scanExampleFile(f, noun)
		if !ok {
			continue
		}
		// Trim the path back to a tree-relative form so the wire shape
		// doesn't leak the absolute path of the host that produced it.
		hit.Path = relativeFromRepoRoot(root, f)
		hits = append(hits, hit)
		if len(hits) >= limit {
			break
		}
	}
	return hits
}

func scanExampleFile(path, noun string) (ExampleHit, bool) {
	f, err := os.Open(path) //nolint:gosec // example files; path is from our walk
	if err != nil {
		return ExampleHit{}, false
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	// Allow long lines (example YAML can have content blocks).
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

	const excerptTail = 5

	var captured []string
	var inMatch bool
	var matchIndent int

	for scanner.Scan() {
		line := scanner.Text()

		if inMatch {
			indent := leadingSpaces(line)
			// Stop the excerpt when we de-indent below the captured key
			// AND the line isn't blank.
			if strings.TrimSpace(line) != "" && indent <= matchIndent && len(captured) > 1 {
				break
			}
			captured = append(captured, line)
			if len(captured) >= excerptTail+1 {
				break
			}
			continue
		}

		if matchesActionKey(line, noun) {
			inMatch = true
			captured = []string{line}
			matchIndent = leadingSpaces(line)
		}
	}

	if !inMatch {
		return ExampleHit{}, false
	}
	return ExampleHit{
		Path:    path,
		Excerpt: strings.Join(captured, "\n") + "\n",
	}, true
}

// matchesActionKey reports whether a YAML line opens the given action,
// either as a top-level step key (`- pkg.install:`) or inline
// (`pkg.install:`). Quoted forms are accepted.
func matchesActionKey(line, noun string) bool {
	s := strings.TrimSpace(line)
	if s == "" || strings.HasPrefix(s, "#") {
		return false
	}
	// `- noun:` form
	if strings.HasPrefix(s, "- ") {
		s = strings.TrimSpace(s[2:])
	}
	// Strip optional surrounding quotes around the key.
	for _, q := range []string{`"`, `'`} {
		s = strings.TrimPrefix(s, q)
		s = strings.TrimPrefix(strings.TrimSuffix(s, q+":"), ":")
	}
	// We want `noun:` or `noun: <anything>` exactly.
	if !strings.HasPrefix(s, noun) {
		return false
	}
	rest := s[len(noun):]
	if rest == "" {
		return false
	}
	if rest[0] != ':' {
		return false
	}
	return true
}

func leadingSpaces(s string) int {
	n := 0
	for _, r := range s {
		if r != ' ' {
			break
		}
		n++
	}
	return n
}

// defaultExamplesRoot finds `./examples/` by walking up from the
// current working directory. Returns "" if not located.
func defaultExamplesRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		candidate := filepath.Join(wd, "examples")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return ""
		}
		wd = parent
	}
}

// relativeFromRepoRoot rewrites an absolute path to "examples/..."
// when the absolute path lives under root. Falls back to the original
// path otherwise.
func relativeFromRepoRoot(root, abs string) string {
	rel, err := filepath.Rel(filepath.Dir(root), abs)
	if err != nil {
		return abs
	}
	// On Windows, normalize to forward slash for stable wire output.
	return filepath.ToSlash(rel)
}
