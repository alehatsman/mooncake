package tool

import "strings"

// splitLines splits s on \n, preserving order and dropping a single
// trailing empty line if present (so a file ending in \n round-trips
// without growing).
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	out := strings.Split(s, "\n")
	if len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

// splitFields splits a single line into whitespace-separated tokens.
func splitFields(line string) []string {
	return strings.Fields(line)
}

// joinLines is the inverse of splitLines; always ends with a trailing newline.
func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}
