package observe_logs

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"
)

// readFile tails the file at path and returns lines whose modtime
// window falls within since. Implementation: seek to (size - maxBytes)
// then scan forward to the end, capped at maxLines. Lines older than
// the since window are filtered out by file *mtime* (one-shot files
// don't carry per-line timestamps the way journald does — the entire
// file is in-window if its modtime is recent enough).
//
// For the typical "service just restarted, did it log errors in the
// last 30 seconds?" use case, this approximation is good enough: the
// caller only cares whether the patterns appeared *recently*; if the
// file hasn't been touched in 30 seconds, no match is in-window
// regardless of content.
func readFile(path string, since time.Duration, maxBytes int64, maxLines int) ([]string, bool, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, false, fmt.Errorf("stat %s: %w", path, err)
	}
	if st.IsDir() {
		return nil, false, fmt.Errorf("%s is a directory, not a log file", path)
	}
	// Reject empty since window; surface as zero matches rather than scanning.
	if time.Since(st.ModTime()) > since && since > 0 {
		// File hasn't been touched in the window — no in-window lines.
		// Return success with empty slice; downstream matchLines will
		// produce zero counts.
		return nil, false, nil
	}

	f, err := os.Open(path) //nolint:gosec // user-specified path is intended
	if err != nil {
		return nil, false, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck

	size := st.Size()
	start := int64(0)
	if size > maxBytes {
		start = size - maxBytes
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, false, fmt.Errorf("seek %s: %w", path, err)
	}

	// If we seeked mid-line, discard the partial first line.
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // up to 1 MiB per line
	first := start > 0
	var lines []string
	truncated := false
	for scanner.Scan() {
		if first {
			first = false
			continue
		}
		lines = append(lines, scanner.Text())
		if maxLines > 0 && len(lines) >= maxLines {
			truncated = true
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return lines, truncated, fmt.Errorf("scan %s: %w", path, err)
	}
	// Byte cap may have been hit by the seek; mark truncated if the
	// file was larger than the window we read.
	if size > maxBytes {
		truncated = true
	}
	return lines, truncated, nil
}
