package discovery

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ParseSSHConfig parses ~/.ssh/config-style files and returns one Candidate
// per concrete (non-wildcard, non-pattern) Host entry. It implements only
// the subset of ssh_config(5) the discovery flow needs:
//
//   - `Host <name>...` blocks (multiple names per line allowed).
//   - `HostName`, `User`, `Port` inside each block.
//   - Case-insensitive keywords (`HOST`, `Host`, `host` all match).
//   - Inline comments (`# ...`) and blank lines.
//
// Explicitly ignored: `Match`, `Include`, `*` and `?` wildcards in Host
// names, any other directive. Wildcards aren't usable as fleet candidates
// — they describe patterns, not specific machines.
//
// Returns an empty slice (no error) when path doesn't exist; an absent
// ~/.ssh/config is a normal state, not a failure. Returns a wrapped error
// for I/O failures or malformed syntax.
func ParseSSHConfig(path string) ([]Candidate, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	return parseSSHConfigReader(f)
}

// parseSSHConfigReader is the io.Reader-driven core, factored out so
// tests can drive it without writing to disk.
func parseSSHConfigReader(r io.Reader) ([]Candidate, error) {
	var (
		out     []Candidate
		current []Candidate // hosts in the active block (multi-name lines)
	)

	flush := func() {
		out = append(out, current...)
		current = nil
	}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := stripComment(scanner.Text())
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		keyword, rest, _ := splitKeywordValue(line)
		switch strings.ToLower(keyword) {
		case "host":
			// New block — flush the previous one.
			flush()
			for _, name := range strings.Fields(rest) {
				if hostNameIsPattern(name) {
					continue
				}
				current = append(current, Candidate{
					Name:    name,
					Sources: []string{SourceSSHConfig},
				})
			}
		case "hostname":
			for i := range current {
				if current[i].Addr == "" {
					current[i].Addr = strings.TrimSpace(rest)
				}
			}
		case "user":
			for i := range current {
				if current[i].SSHUser == "" {
					current[i].SSHUser = strings.TrimSpace(rest)
				}
			}
		case "port":
			port, perr := strconv.Atoi(strings.TrimSpace(rest))
			if perr != nil {
				return nil, fmt.Errorf("ssh_config: invalid Port %q: %w", rest, perr)
			}
			for i := range current {
				if current[i].SSHPort == 0 {
					current[i].SSHPort = port
				}
			}
		default:
			// Unknown keyword: ignore. ssh_config has dozens we don't care
			// about (IdentityFile, ForwardAgent, etc.) and the convention
			// is to ignore unknowns rather than error.
		}
	}
	flush()

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}

	// Default HostName to the alias name if the operator didn't supply
	// one. This matches ssh(1)'s own behavior — `ssh foo` without a
	// HostName line treats `foo` as both alias and hostname.
	for i := range out {
		if out[i].Addr == "" {
			out[i].Addr = out[i].Name
		}
	}
	return out, nil
}

// DefaultSSHConfigPath returns ~/.ssh/config, or "" when HOME is
// unresolvable. The empty case is normal in test environments; callers
// should treat it as "no SSH config to parse."
func DefaultSSHConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".ssh", "config")
}

// stripComment trims everything after an unquoted `#` on a line. Mirrors
// ssh_config(5) parsing — comments may follow values.
func stripComment(s string) string {
	if i := strings.Index(s, "#"); i >= 0 {
		return s[:i]
	}
	return s
}

// splitKeywordValue splits a single directive line into its keyword and
// value. ssh_config accepts both whitespace and `=` between the two, so
// we honor either. Returns (keyword, value, true) when a value is
// present; the bool is false for keyword-only lines (which we don't
// recognize, but treating cleanly is cheaper than guarding callers).
func splitKeywordValue(line string) (string, string, bool) {
	// Find first `=` or whitespace run.
	for i, r := range line {
		if r == '=' || r == ' ' || r == '\t' {
			keyword := line[:i]
			rest := strings.TrimLeft(line[i+1:], " \t=")
			return keyword, rest, true
		}
	}
	return line, "", false
}

// hostNameIsPattern reports whether name contains ssh_config wildcards.
// `Host *` and `Host *.example.com` shouldn't surface as candidates.
func hostNameIsPattern(name string) bool {
	return strings.ContainsAny(name, "*?!")
}
