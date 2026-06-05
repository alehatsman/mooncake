package mooncake

// queries.go — direct read-path helpers: Read / Grep / Glob (#143).
//
// These MUST NOT go through plan-build or the executor. No Step, no ModePlan,
// no event funnel — direct filesystem queries at native latency.

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ReadOptions configures a Read call.
type ReadOptions struct {
	// Offset is the byte offset to start reading from. 0 means start of file.
	Offset int64
	// Limit is the maximum number of bytes to return. 0 means no limit.
	Limit int64
}

// GrepOptions configures a Grep call.
type GrepOptions struct {
	// Dir is the root directory to search. Defaults to the process working
	// directory when empty.
	Dir string
	// Extensions filters results to files with these extensions (without the
	// leading dot, e.g. "go", "ts"). Empty means all files.
	Extensions []string
	// MaxResults caps the number of matches returned. 0 means no cap.
	MaxResults int
	// CaseInsensitive makes the pattern match case-insensitively.
	CaseInsensitive bool
}

// GlobOptions configures a Glob call.
type GlobOptions struct {
	// Dir is the root directory the pattern is resolved against. Defaults to
	// the process working directory when empty.
	Dir string
}

// Match is a single content match returned by Grep.
type Match struct {
	// Path is the file path containing the match.
	Path string
	// Line is the 1-based line number of the match.
	Line int
	// Content is the raw line content (trimmed of trailing newline).
	Content string
}

// Read returns the content of path. If opts.Offset is non-zero the read starts
// at that byte offset; if opts.Limit is non-zero at most that many bytes are
// returned. Does NOT go through plan-build or the executor — direct OS call.
func Read(path string, opts ReadOptions) ([]byte, error) {
	f, err := os.Open(path) // #nosec G304 G122 -- caller-supplied path is intentional
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	if opts.Offset > 0 {
		if _, err := f.Seek(opts.Offset, io.SeekStart); err != nil {
			return nil, err
		}
	}

	if opts.Limit > 0 {
		buf := make([]byte, opts.Limit)
		n, err := io.ReadFull(f, buf)
		if err != nil && err != io.ErrUnexpectedEOF {
			return nil, err
		}
		return buf[:n], nil
	}

	return io.ReadAll(f)
}

// Grep searches for lines matching pattern (RE2 regex) in files rooted at
// opts.Dir (default: process cwd). Does NOT go through plan-build or the
// executor — direct filesystem walk.
func Grep(pattern string, opts GrepOptions) ([]Match, error) {
	flags := ""
	if opts.CaseInsensitive {
		flags = "(?i)"
	}
	re, err := regexp.Compile(flags + pattern)
	if err != nil {
		return nil, err
	}

	dir := opts.Dir
	if dir == "" {
		if dir, err = os.Getwd(); err != nil {
			return nil, err
		}
	}

	extSet := make(map[string]struct{}, len(opts.Extensions))
	for _, e := range opts.Extensions {
		extSet["."+strings.TrimPrefix(e, ".")] = struct{}{}
	}

	var matches []Match
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if len(extSet) > 0 {
			if _, ok := extSet[filepath.Ext(path)]; !ok {
				return nil
			}
		}
		if opts.MaxResults > 0 && len(matches) >= opts.MaxResults {
			return filepath.SkipAll
		}

		f, openErr := os.Open(path) // #nosec G304 G122 -- path from WalkDir
		if openErr != nil {
			return nil
		}
		defer func() { _ = f.Close() }()

		scanner := bufio.NewScanner(f)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			line := scanner.Text()
			if re.MatchString(line) {
				matches = append(matches, Match{Path: path, Line: lineNo, Content: line})
				if opts.MaxResults > 0 && len(matches) >= opts.MaxResults {
					return filepath.SkipAll
				}
			}
		}
		return scanner.Err()
	})
	if err != nil {
		return nil, err
	}
	return matches, nil
}

// Glob returns paths matching pattern. If opts.Dir is non-empty the pattern is
// joined to that directory first; otherwise pattern is used as-is (absolute or
// relative to cwd). Does NOT go through plan-build or the executor — direct
// filepath.Glob call.
func Glob(pattern string, opts GlobOptions) ([]string, error) {
	if opts.Dir != "" {
		pattern = filepath.Join(opts.Dir, pattern)
	}
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	if paths == nil {
		return []string{}, nil
	}
	return paths, nil
}
