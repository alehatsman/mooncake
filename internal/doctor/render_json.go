package doctor

import (
	"encoding/json"
	"io"
)

// RenderJSON dumps the Report struct with two-space indent. The contract
// is the field tags on Report and Result — adjust those, not this function.
func RenderJSON(out io.Writer, rep *Report) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}
