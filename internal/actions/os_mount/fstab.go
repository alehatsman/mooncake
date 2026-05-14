//nolint:revive // package name follows action convention
package os_mount

import (
	"path/filepath"
	"strconv"
	"strings"
)

// fstabEntry is the parsed form of one fstab data line. raw preserves
// the original byte content so unchanged lines round-trip byte-for-byte.
type fstabEntry struct {
	src     string
	dest    string
	fstype  string
	options []string
	dump    int
	pass    int
	raw     string
}

// fstabLine is the parser-friendly view of any line in /etc/fstab.
// Blank and comment-only lines carry only `raw` (and entry is nil).
type fstabLine struct {
	raw   string
	entry *fstabEntry
}

// parseFstab splits the file into lines (preserving original text)
// and decodes data lines into entries. Unparseable lines are kept
// byte-identical and treated as opaque.
func parseFstab(content string) []fstabLine {
	if content == "" {
		return nil
	}
	trimmed := strings.TrimSuffix(content, "\n")
	raws := strings.Split(trimmed, "\n")
	out := make([]fstabLine, 0, len(raws))
	for _, raw := range raws {
		stripped := strings.TrimSpace(raw)
		if stripped == "" || strings.HasPrefix(stripped, "#") {
			out = append(out, fstabLine{raw: raw})
			continue
		}
		entry, ok := parseEntry(stripped)
		if !ok {
			out = append(out, fstabLine{raw: raw})
			continue
		}
		entry.raw = raw
		out = append(out, fstabLine{raw: raw, entry: &entry})
	}
	return out
}

// parseEntry decodes a single fstab data line. fstab is whitespace-
// separated with up to 6 fields: <src> <dest> <fstype> <options>
// <dump> <pass>. dump/pass default to 0 when absent.
func parseEntry(line string) (fstabEntry, bool) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return fstabEntry{}, false
	}
	e := fstabEntry{
		src:    fields[0],
		dest:   fields[1],
		fstype: fields[2],
	}
	if len(fields) >= 4 {
		e.options = strings.Split(fields[3], ",")
	} else {
		e.options = []string{"defaults"}
	}
	if len(fields) >= 5 {
		if n, err := strconv.Atoi(fields[4]); err == nil {
			e.dump = n
		}
	}
	if len(fields) >= 6 {
		if n, err := strconv.Atoi(fields[5]); err == nil {
			e.pass = n
		}
	}
	return e, true
}

// renderEntry emits an entry as a single fstab line. Fields are
// separated by a single space; whitespace alignment is intentionally
// not preserved — fstab does not require columnar formatting.
func renderEntry(e fstabEntry) string {
	opts := strings.Join(e.options, ",")
	if opts == "" {
		opts = "defaults"
	}
	return strings.Join([]string{
		e.src,
		e.dest,
		e.fstype,
		opts,
		strconv.Itoa(e.dump),
		strconv.Itoa(e.pass),
	}, " ")
}

// findByDest returns the index of the data line whose dest matches
// the canonical form of `dest`. Returns -1 when missing.
func findByDest(lines []fstabLine, dest string) int {
	canon := canonicalDest(dest)
	for i, ln := range lines {
		if ln.entry == nil {
			continue
		}
		if canonicalDest(ln.entry.dest) == canon {
			return i
		}
	}
	return -1
}

// canonicalDest normalises a mount point ("/data/" -> "/data") so
// idempotency comparisons survive trailing slashes.
func canonicalDest(p string) string {
	if p == "" {
		return ""
	}
	return filepath.Clean(p)
}

// renderFstab reassembles the line slice into a byte string. Lines
// retain their original raw content (unchanged entries are byte-
// identical); mutated lines carry the re-rendered raw.
func renderFstab(lines []fstabLine) string {
	if len(lines) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, ln := range lines {
		sb.WriteString(ln.raw)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// entryMatches reports whether the live entry matches the desired
// src/fstype/options/dump/pass. Options compare as sets — the kernel
// is order-insensitive and round-tripping through mount(8) reorders.
func entryMatches(have, want fstabEntry) bool {
	if have.src != want.src {
		return false
	}
	if have.fstype != want.fstype {
		return false
	}
	if have.dump != want.dump || have.pass != want.pass {
		return false
	}
	return sameOptions(have.options, want.options)
}

func sameOptions(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := map[string]int{}
	for _, x := range a {
		counts[x]++
	}
	for _, x := range b {
		if counts[x] == 0 {
			return false
		}
		counts[x]--
	}
	return true
}

// parseMounts decodes /proc/mounts (or its stubbed equivalent) into a
// map keyed by canonical mount point. The first three fields match
// fstab exactly; remaining fields (options, dump/pass numerics from
// the kernel) parse the same way.
func parseMounts(content string) map[string]fstabEntry {
	out := map[string]fstabEntry{}
	for _, raw := range strings.Split(content, "\n") {
		stripped := strings.TrimSpace(raw)
		if stripped == "" || strings.HasPrefix(stripped, "#") {
			continue
		}
		e, ok := parseEntry(stripped)
		if !ok {
			continue
		}
		out[canonicalDest(e.dest)] = e
	}
	return out
}