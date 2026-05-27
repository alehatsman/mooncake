package kernel

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alehatsman/mooncake/internal/plan"
)

// FormatUnresolvedTemplates renders the strict-template diagnostics
// in the same shape the config validator uses ("file:line:col:
// message"), grouped by step so an operator can fix multiple typos
// in one pass.
//
// Output is always terminated by a newline; an empty slice returns "".
func FormatUnresolvedTemplates(refs []plan.UnresolvedRef) string {
	if len(refs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("unresolved template variable(s):\n\n")

	// Group by (file, line) so refs with the same origin print as
	// one block. Keeps output scannable when a step has multiple
	// typos.
	type key struct {
		file      string
		line, col int
	}
	groups := make(map[key][]plan.UnresolvedRef)
	var order []key
	for _, r := range refs {
		k := key{}
		if r.Origin != nil {
			k = key{r.Origin.FilePath, r.Origin.Line, r.Origin.Column}
		}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], r)
	}
	sort.SliceStable(order, func(i, j int) bool {
		if order[i].file != order[j].file {
			return order[i].file < order[j].file
		}
		if order[i].line != order[j].line {
			return order[i].line < order[j].line
		}
		return order[i].col < order[j].col
	})

	for _, k := range order {
		refs := groups[k]
		// Header: file:line:col  (in step "<name>")
		if k.file != "" {
			fmt.Fprintf(&b, "  %s:%d:%d", k.file, k.line, k.col)
			if name := refs[0].StepName; name != "" {
				fmt.Fprintf(&b, "  (step: %s)", name)
			}
			b.WriteString("\n")
		} else if name := refs[0].StepName; name != "" {
			fmt.Fprintf(&b, "  step: %s\n", name)
		}
		for _, r := range refs {
			field := r.Field
			if field == "" {
				field = "<unknown>"
			}
			fmt.Fprintf(&b, "    %s → {{ %s }} is undefined\n", field, r.Root)
		}
		b.WriteString("\n")
	}
	b.WriteString("Fix: define the variable in mooncake.vars.yml, pass it via --vars, or use --allow-undefined to downgrade to a warning.\n")
	return b.String()
}
