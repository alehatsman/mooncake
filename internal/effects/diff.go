package effects

import (
	"bytes"
	"fmt"
	"strings"
)

const (
	diffContextLines = 3
	diffMaxBytes     = 4096
)

// unifiedDiff returns a unified diff string between oldB and newB for the
// given path. Returns empty string if content is identical or binary.
func unifiedDiff(path string, oldB, newB []byte) string {
	if bytes.Equal(oldB, newB) {
		return ""
	}
	if isBinary(oldB) || isBinary(newB) {
		return fmt.Sprintf("Binary files %s differ\n", path)
	}

	aLines := splitLines(string(oldB))
	bLines := splitLines(string(newB))

	edits := diffLines(aLines, bLines)
	hunks := buildHunks(edits, aLines, bLines, diffContextLines)
	if len(hunks) == 0 {
		return ""
	}

	var buf strings.Builder
	fmt.Fprintf(&buf, "--- %s\n+++ %s (proposed)\n", path, path)
	for _, h := range hunks {
		buf.WriteString(h)
	}

	result := buf.String()
	if len(result) > diffMaxBytes {
		return result[:diffMaxBytes] + "\n[...diff truncated...]\n"
	}
	return result
}

// isBinary reports whether b appears to contain binary (non-text) content.
func isBinary(b []byte) bool {
	return bytes.IndexByte(b, 0) >= 0
}

// splitLines splits s into lines preserving the trailing newline on each line.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.SplitAfter(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

type editKind int8

const (
	editEqual  editKind = iota
	editInsert          // line in b only
	editDelete          // line in a only
)

type edit struct {
	kind editKind
	aIdx int // 1-based index in a (valid for Equal and Delete)
	bIdx int // 1-based index in b (valid for Equal and Insert)
}

// diffLines computes the edit script between a and b using LCS (O(m*n) DP).
func diffLines(a, b []string) []edit {
	m, n := len(a), len(b)

	// dp[i][j] = LCS length of a[:i] and b[:j]
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to build edit list (iterative, avoids stack overflow on large files)
	edits := make([]edit, 0, m+n)
	i, j := m, n
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && a[i-1] == b[j-1]:
			edits = append(edits, edit{editEqual, i, j})
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			edits = append(edits, edit{editInsert, i, j})
			j--
		default:
			edits = append(edits, edit{editDelete, i, j})
			i--
		}
	}

	// Reverse: backtrack produces edits in reverse order
	for l, r := 0, len(edits)-1; l < r; l, r = l+1, r-1 {
		edits[l], edits[r] = edits[r], edits[l]
	}
	return edits
}

// buildHunks groups edits into unified-diff hunk strings, each surrounded by
// `context` lines of unchanged context.
func buildHunks(edits []edit, a, b []string, context int) []string {
	// Mark which edit indices contain changes
	type span struct{ start, end int } // [start, end) indices into edits
	var changeSpans []span
	i := 0
	for i < len(edits) {
		if edits[i].kind == editEqual {
			i++
			continue
		}
		start := i
		for i < len(edits) && edits[i].kind != editEqual {
			i++
		}
		changeSpans = append(changeSpans, span{start, i})
	}

	if len(changeSpans) == 0 {
		return nil
	}

	// Expand each change span by context lines and merge overlapping spans
	type hunkSpan struct{ start, end int }
	var hunkSpans []hunkSpan
	for _, cs := range changeSpans {
		lo := cs.start - context
		if lo < 0 {
			lo = 0
		}
		hi := cs.end + context
		if hi > len(edits) {
			hi = len(edits)
		}
		if len(hunkSpans) > 0 && lo <= hunkSpans[len(hunkSpans)-1].end {
			hunkSpans[len(hunkSpans)-1].end = hi
		} else {
			hunkSpans = append(hunkSpans, hunkSpan{lo, hi})
		}
	}

	var hunks []string
	for _, hs := range hunkSpans {
		slice := edits[hs.start:hs.end]

		// Compute hunk header counts
		aStart, bStart := 0, 0
		aCount, bCount := 0, 0
		for _, e := range slice {
			switch e.kind {
			case editEqual:
				if aStart == 0 {
					aStart = e.aIdx
				}
				if bStart == 0 {
					bStart = e.bIdx
				}
				aCount++
				bCount++
			case editDelete:
				if aStart == 0 {
					aStart = e.aIdx
				}
				aCount++
			case editInsert:
				if bStart == 0 {
					bStart = e.bIdx
				}
				bCount++
			}
		}
		if aStart == 0 {
			aStart = 1
		}
		if bStart == 0 {
			bStart = 1
		}

		var buf strings.Builder
		fmt.Fprintf(&buf, "@@ -%d,%d +%d,%d @@\n", aStart, aCount, bStart, bCount)
		for _, e := range slice {
			switch e.kind {
			case editEqual:
				buf.WriteString(" ")
				buf.WriteString(a[e.aIdx-1])
				if !strings.HasSuffix(a[e.aIdx-1], "\n") {
					buf.WriteString("\n")
				}
			case editDelete:
				buf.WriteString("-")
				buf.WriteString(a[e.aIdx-1])
				if !strings.HasSuffix(a[e.aIdx-1], "\n") {
					buf.WriteString("\n")
				}
			case editInsert:
				buf.WriteString("+")
				buf.WriteString(b[e.bIdx-1])
				if !strings.HasSuffix(b[e.bIdx-1], "\n") {
					buf.WriteString("\n")
				}
			}
		}
		hunks = append(hunks, buf.String())
	}
	return hunks
}
