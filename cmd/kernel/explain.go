package kernel

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"

	"github.com/alehatsman/mooncake/internal/explain"
)

// ExplainCommand returns the `mooncake explain` cli.Command.
func ExplainCommand() *cli.Command {
	return &cli.Command{
		Name:      "explain",
		Usage:     "Look up typed information about a mooncake noun (action verb, run, resource, op)",
		ArgsUsage: "<noun>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Value:   outputFormatText,
				Usage:   "Output format: text, json, or yaml",
			},
			&cli.IntFlag{
				Name:  "examples-limit",
				Value: 3,
				Usage: "Max example excerpts to include for kind:action results",
			},
		},
		Action: explainAction,
	}
}

// explainAction resolves a noun (action verb, run id, resource handle, op id)
// and renders the typed payload as text / JSON / YAML. See spec-68.
//
// Wave 1: only kind:action resolves; run / resource / op fall through to a
// typed not_found.
func explainAction(c *cli.Context) error {
	if c.NArg() != 1 {
		return fmt.Errorf("usage: mooncake explain <noun>")
	}
	noun := c.Args().First()

	format := c.String("format")
	switch format {
	case outputFormatText, outputFormatJSON, outputFormatYAML:
	default:
		return fmt.Errorf("invalid format: %s (use 'text', 'json', or 'yaml')", format)
	}

	// F044: mirror the MCP-side validation. The flag default is 3 so
	// users who omit the flag never hit this; users who pass an
	// out-of-range value get a clear rejection.
	limit := c.Int("examples-limit")
	const explainExamplesLimitMax = 10
	if limit < 0 {
		return fmt.Errorf("--examples-limit must be >= 0 (got %d)", limit)
	}
	if limit > explainExamplesLimitMax {
		return fmt.Errorf("--examples-limit must be <= %d (got %d)", explainExamplesLimitMax, limit)
	}

	result := explain.Resolve(noun, explain.Options{
		ExamplesLimit: limit,
	})

	// not_found on action lookups is an agent-recoverable signal, but on the
	// CLI we want a non-zero exit so shell pipelines / `&&` chains stop.
	switch format {
	case outputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			return err
		}
	case outputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		enc.SetIndent(yamlIndentSpaces)
		defer func() { _ = enc.Close() }()
		if err := enc.Encode(result); err != nil {
			return err
		}
	case outputFormatText:
		renderExplainText(os.Stdout, result)
	}

	if result.Kind == explain.KindNotFound {
		return cli.Exit("", 1)
	}
	return nil
}

func renderExplainText(w io.Writer, r explain.Result) {
	switch r.Kind {
	case explain.KindAction:
		renderExplainActionText(w, r.Action)
	case explain.KindRun:
		renderExplainRunText(w, r.Run)
	case explain.KindResource:
		renderExplainResourceText(w, r.Resource)
	case explain.KindOp:
		renderExplainOpText(w, r.Op)
	case explain.KindNotFound:
		renderExplainNotFoundText(w, r.NotFound)
	default:
		fmt.Fprintf(w, "kind: %s (no text renderer)\n", r.Kind)
	}
}

func renderExplainRunText(w io.Writer, p *explain.RunPayload) {
	fmt.Fprintf(w, "run: %s\n", p.RunID)
	if p.OpID != "" {
		fmt.Fprintf(w, "  op:       %s\n", p.OpID)
	}
	fmt.Fprintf(w, "  ts:       %s\n", p.TS.Format(time.RFC3339))
	if p.Config != "" {
		fmt.Fprintf(w, "  config:   %s\n", p.Config)
	}
	if p.DurationMs > 0 {
		fmt.Fprintf(w, "  duration: %dms\n", p.DurationMs)
	}
	fmt.Fprintf(w, "  totals:   changed=%d ok=%d skipped=%d failed=%d\n",
		p.Totals.Changed, p.Totals.Ok, p.Totals.Skipped, p.Totals.Failed)
	if p.Caveats.IrreversibleStepCount > 0 {
		fmt.Fprintf(w, "  caveats:  %d irreversible step(s)\n", p.Caveats.IrreversibleStepCount)
	}
	if len(p.Steps) > 0 {
		fmt.Fprintln(w, "\nsteps:")
		for _, s := range p.Steps {
			rev := ""
			if s.Reversible {
				rev = " (reversible)"
			}
			res := s.Resource
			if res == "" {
				res = "-"
			}
			fmt.Fprintf(w, "  %2d. %-20s %-7s  %s%s\n", s.Index, s.Action, s.Result, res, rev)
			// Spec-68 wave 2.5: one-line typed-Diff summary when the
			// step's handler captured one. Indented to make the
			// per-step block scannable without crowding the resource
			// line above.
			if summary := summarizeDiff(s.Diff); summary != "" {
				fmt.Fprintf(w, "      diff: %s\n", summary)
			}
			// The reason a step failed is why anyone opens a past run.
			// Kept to the first line here — text mode stays scannable
			// and the JSON / YAML formats carry the full message.
			if s.Error != "" {
				fmt.Fprintf(w, "      error: %s\n", firstLine(s.Error))
			}
		}
	}
}

// firstLine returns the leading line of msg, ellipsized when more
// follows. Step errors fold in the tail of a command's output, which is
// exactly what makes them useful in the JSON payload and unreadable in a
// per-step text row.
func firstLine(msg string) string {
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		return strings.TrimRight(msg[:i], "\r") + " …"
	}
	return msg
}

// summarizeDiff condenses a typed actions.Diff JSON payload to a
// one-line "kind=K op=O" hint. Returns "" when raw is empty or can't
// be parsed — text mode stays terse and the JSON / YAML formats still
// carry the full payload for callers that want the detail.
func summarizeDiff(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var probe struct {
		Resource struct {
			Kind       string `json:"kind"`
			Identifier string `json:"identifier"`
		} `json:"resource"`
		Operation string `json:"operation"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return ""
	}
	parts := []string{}
	if probe.Operation != "" {
		parts = append(parts, "op="+probe.Operation)
	}
	if probe.Resource.Kind != "" {
		parts = append(parts, "kind="+probe.Resource.Kind)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

func renderExplainResourceText(w io.Writer, p *explain.ResourcePayload) {
	fmt.Fprintf(w, "resource: %s\n", p.Resource)
	if len(p.History) == 0 {
		fmt.Fprintln(w, "  history: (none — this resource has not been touched by any logged run)")
		return
	}
	fmt.Fprintln(w, "\nhistory (newest first):")
	for _, h := range p.History {
		rev := ""
		if h.Reversible {
			rev = " (reversible)"
		}
		// F045: when the same run touched this resource multiple times,
		// the rows would otherwise be visually identical (same TS, same
		// RunID, same Action, same Result). step=N gives readers a
		// stable ordering key. Pre-spec-68 runs have no step index;
		// omit the suffix there.
		step := ""
		if h.StepIndex > 0 {
			step = fmt.Sprintf(" step=%d", h.StepIndex)
		}
		fmt.Fprintf(w, "  %s  %-20s %-7s  run=%s%s%s\n",
			h.TS.Format(time.RFC3339), h.Action, h.Result, h.RunID, step, rev)
		// Spec-68 wave 2.5: typed-Diff summary when available. Same
		// one-line hint as the run-payload renderer; the indent lines
		// the diff up under the resource handle it belongs to.
		if summary := summarizeDiff(h.Diff); summary != "" {
			fmt.Fprintf(w, "      diff: %s\n", summary)
		}
	}
}

func renderExplainOpText(w io.Writer, p *explain.OpPayload) {
	fmt.Fprintf(w, "op: %s\n", p.OpID)
	fmt.Fprintf(w, "  ts:       %s\n", p.TS.Format(time.RFC3339))
	fmt.Fprintf(w, "  command:  %s\n", p.Command)
	if len(p.Args) > 0 {
		fmt.Fprintf(w, "  args:     %s\n", strings.Join(p.Args, " "))
	}
	if p.Actor != "" {
		fmt.Fprintf(w, "  actor:    %s\n", p.Actor)
	}
	if p.Parent != "" {
		fmt.Fprintf(w, "  parent:   %s\n", p.Parent)
	}
	if p.Config != "" {
		fmt.Fprintf(w, "  config:   %s\n", p.Config)
	}
	if p.PlanOnly {
		fmt.Fprintln(w, "  plan_only: true")
	}
	if len(p.Runs) > 0 {
		fmt.Fprintln(w, "\nruns:")
		for _, r := range p.Runs {
			fmt.Fprintf(w, "  - %s\n", r)
		}
	} else if !p.PlanOnly {
		fmt.Fprintln(w, "  runs:     (none yet)")
	}
}

func renderExplainActionText(w io.Writer, p *explain.ActionPayload) {
	fmt.Fprintf(w, "action: %s\n", p.Name)
	if p.Metadata.Description != "" {
		fmt.Fprintf(w, "  description: %s\n", p.Metadata.Description)
	}
	if p.Metadata.Category != "" {
		fmt.Fprintf(w, "  category:    %s\n", p.Metadata.Category)
	}
	if p.Metadata.Version != "" {
		fmt.Fprintf(w, "  version:     %s\n", p.Metadata.Version)
	}
	if len(p.Metadata.SupportedPlatforms) > 0 {
		fmt.Fprintf(w, "  platforms:   %s\n", strings.Join(p.Metadata.SupportedPlatforms, ", "))
	}
	fmt.Fprintf(w, "  dry_run:     %t\n", p.Metadata.SupportsDryRun)
	fmt.Fprintf(w, "  become:      %t\n", p.Metadata.SupportsBecome)
	fmt.Fprintf(w, "  check:       %t\n", p.Metadata.ImplementsCheck)

	if p.Schema != nil && len(p.Schema.Properties) > 0 {
		fmt.Fprintln(w, "\nschema:")
		required := map[string]bool{}
		for _, r := range p.Schema.Required {
			required[r] = true
		}
		names := make([]string, 0, len(p.Schema.Properties))
		for n := range p.Schema.Properties {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			prop := p.Schema.Properties[n]
			marker := " "
			if required[n] {
				marker = "*"
			}
			t := prop.Type
			if t == "" && len(prop.OneOf) > 0 {
				t = strings.Join(prop.OneOf, "|")
			}
			fmt.Fprintf(w, "  %s %-20s %s", marker, n, t)
			if prop.Description != "" {
				fmt.Fprintf(w, "  — %s", prop.Description)
			}
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, "  (* required)")
	}

	fmt.Fprintln(w, "\ndiff:    ", p.DiffShape.Note)
	fmt.Fprintln(w, "reverse: ", p.ReverseShape.Caveat)

	if len(p.Examples) > 0 {
		fmt.Fprintln(w, "\nexamples:")
		for _, ex := range p.Examples {
			fmt.Fprintf(w, "  %s\n", ex.Path)
			for _, line := range strings.Split(strings.TrimRight(ex.Excerpt, "\n"), "\n") {
				fmt.Fprintf(w, "    %s\n", line)
			}
		}
	}
}

func renderExplainNotFoundText(w io.Writer, p *explain.NotFoundPayload) {
	fmt.Fprintf(w, "not_found: %q\n", p.Noun)
	if p.Reason != "" {
		fmt.Fprintf(w, "  reason: %s\n", p.Reason)
	}
	if len(p.Candidates) > 0 {
		fmt.Fprintln(w, "  did you mean:")
		for _, cand := range p.Candidates {
			fmt.Fprintf(w, "    - %s (%s)\n", cand.ID, cand.Kind)
		}
	}
}
