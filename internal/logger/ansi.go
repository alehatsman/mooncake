package logger

import (
	"regexp"
	"unicode/utf8"
)

// ANSI escape code patterns
var (
	// ansiRegex matches all ANSI escape sequences
	// Covers CSI sequences, OSC sequences, and simple escape codes
	ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][0-9];[^\x07]*\x07|\x1b[=>]|\x1b\[[0-9;]*m`)
)

// StripANSI removes all ANSI escape codes from a string.
// This includes color codes, cursor movement, and other terminal control sequences.
//
// Example:
//   StripANSI("\x1b[31mRed Text\x1b[0m") → "Red Text"
func StripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// VisibleLength returns the number of visible characters in a string,
// excluding ANSI escape codes.
//
// This properly counts UTF-8 characters (runes) rather than bytes,
// so emoji and other multi-byte characters count as 1.
//
// Example:
//   VisibleLength("\x1b[31mRed\x1b[0m") → 3
//   VisibleLength("Hello 🌙") → 7
func VisibleLength(s string) int {
	stripped := StripANSI(s)
	return utf8.RuneCountInString(stripped)
}

// TruncateANSI truncates a string to the specified visible width while
// preserving ANSI escape codes. If truncation occurs, an ellipsis is added.
//
// The function:
// 1. Preserves all ANSI codes before the truncation point
// 2. Counts only visible characters for width calculation
// 3. Adds "..." when truncation occurs
// 4. Handles UTF-8 characters correctly
//
// Example:
//   TruncateANSI("\x1b[31mLong Red Text\x1b[0m", 8) → "\x1b[31mLong...\x1b[0m"
func TruncateANSI(s string, maxWidth int) string {
	// Check if truncation is needed
	visibleLen := VisibleLength(s)
	if visibleLen <= maxWidth {
		return s
	}

	// Handle edge cases
	if maxWidth < 4 {
		// Too narrow for ellipsis, just return first N visible chars
		return extractVisiblePrefix(s, maxWidth)
	}

	// Reserve space for ellipsis
	targetWidth := maxWidth - 3

	// Extract prefix with ANSI codes preserved
	prefix := extractVisiblePrefix(s, targetWidth)

	// Add ellipsis
	return prefix + "..."
}

// extractVisiblePrefix extracts the first N visible characters from a string,
// preserving all ANSI codes that appear before or within those characters.
func extractVisiblePrefix(s string, visibleCount int) string {
	if visibleCount <= 0 {
		return ""
	}

	result := make([]byte, 0, len(s))
	visibleChars := 0
	i := 0

	for i < len(s) && visibleChars < visibleCount {
		// Check for ANSI escape sequence at current position
		if s[i] == '\x1b' && i+1 < len(s) {
			// Find the end of the ANSI sequence
			ansiEnd := findANSIEnd(s, i)
			if ansiEnd > i {
				// Copy the entire ANSI sequence (doesn't count toward visible length)
				result = append(result, s[i:ansiEnd]...)
				i = ansiEnd
				continue
			}
		}

		// Regular character - decode UTF-8 rune
		r, size := utf8.DecodeRuneInString(s[i:])
		if r != utf8.RuneError {
			// Valid UTF-8 character
			result = append(result, s[i:i+size]...)
			visibleChars++
			i += size
		} else {
			// Invalid UTF-8, skip byte
			i++
		}
	}

	// Copy any remaining ANSI codes after the truncation point
	// This ensures we close any open color/style codes
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) {
			ansiEnd := findANSIEnd(s, i)
			if ansiEnd > i {
				result = append(result, s[i:ansiEnd]...)
				i = ansiEnd
				continue
			}
		}
		break
	}

	return string(result)
}

// findANSIEnd finds the end position of an ANSI escape sequence starting at pos.
// Returns the position after the sequence, or pos if not a valid ANSI sequence.
func findANSIEnd(s string, pos int) int {
	if pos >= len(s) || s[pos] != '\x1b' {
		return pos
	}

	i := pos + 1
	if i >= len(s) {
		return pos
	}

	// Check for CSI sequence: ESC [ ... letter
	if s[i] == '[' {
		i++
		// Skip parameter bytes (0-9, ;, :)
		for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == ';' || s[i] == ':') {
			i++
		}
		// Final byte (letter)
		if i < len(s) && ((s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z')) {
			return i + 1
		}
	}

	// Check for OSC sequence: ESC ] ... BEL or ESC \
	if s[i] == ']' {
		i++
		// Find BEL (0x07) or ST (ESC \)
		for i < len(s) {
			if s[i] == '\x07' {
				return i + 1
			}
			if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '\\' {
				return i + 2
			}
			i++
		}
	}

	// Simple escape sequences: ESC =, ESC >, etc.
	if s[i] == '=' || s[i] == '>' {
		return i + 1
	}

	// Unknown sequence, just skip ESC
	return pos + 1
}
