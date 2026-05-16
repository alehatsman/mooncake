package diff

import (
	"fmt"
	"io"
	"strings"

	"github.com/alehatsman/mooncake/internal/effects"
)

func init() {
	Register(matchFile)
}

// matchFile recognises the two shapes a file-content diff arrives in:
//   - effects.ContentDiff (in-memory plan, freshly built)
//   - map[string]interface{} (round-tripped through saved JSON plan)
//
// Both carry a "unified_diff" field; the second form is what
// json.Decode produces when StepInspection.Detail was an interface
// whose concrete type the decoder doesn't know.
func matchFile(detail any) Renderer {
	switch v := detail.(type) {
	case effects.ContentDiff:
		if v.UnifiedDiff == "" {
			return nil
		}
		return &fileRenderer{unified: v.UnifiedDiff}
	case map[string]interface{}:
		s, _ := v["unified_diff"].(string)
		if s == "" {
			return nil
		}
		return &fileRenderer{unified: s}
	}
	return nil
}

type fileRenderer struct {
	unified string
}

func (r *fileRenderer) Kind() string { return "file" }

// Render writes the unified-diff text. The text format prefixes each
// line with two spaces — same shape cmd/mooncake.go used before the
// renderer was extracted, so wave-1 output is byte-equivalent for
// file diffs.
func (r *fileRenderer) Render(w io.Writer, format Format) error {
	switch format {
	case FormatText, "":
		for _, line := range strings.Split(strings.TrimRight(r.unified, "\n"), "\n") {
			if _, err := fmt.Fprintf(w, "  %s\n", line); err != nil {
				return err
			}
		}
		return nil
	case FormatJSON, FormatYAML:
		// JSON / YAML callers serialize the plan struct directly;
		// the renderer is invoked for human-readable output only.
		// Returning the raw unified-diff text is the safest fallback
		// if a future caller does invoke Render in these formats.
		_, err := io.WriteString(w, r.unified)
		return err
	}
	return fmt.Errorf("unsupported format: %s", format)
}
