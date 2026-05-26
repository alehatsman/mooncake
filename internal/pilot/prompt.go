package pilot

import (
	"encoding/json"
	"fmt"
	"strings"
)

const promptPreamble = `You are a Mooncake agent planner. Generate ONLY valid Mooncake YAML configuration.

OUTPUT REQUIREMENTS:
- Output ONLY raw YAML (Mooncake RunConfig format)
- NO markdown fences, NO prose, NO explanations, NO comments
- The YAML must be directly parseable by the Mooncake validator`

const promptBestPractices = `BEST PRACTICES:
- Prefer typed text/file actions over shell sed/awk
- Use search/tree actions to discover code before editing
- Use assert to verify changes
- Keep plans small (<= 30 steps)
- Prefer cmd (typed argv) over shell where possible
- Include verification steps`

const promptConstraints = `CONSTRAINTS:
- Plans must be idempotent where possible
- No interactive commands
- All file paths must be absolute or relative to repo root`

// buildSystemPrompt assembles the system prompt by combining the static
// preamble + best-practices + constraints with a schema-derived action
// vocabulary chunk and a style-specific TASK STYLE block. The
// vocabulary is generated from internal/config/schema.json so new
// actions surface without editing this file (spec-67 §12.2). The
// style fragment is the only per-style difference (spec-67 §12.3).
func buildSystemPrompt(style Style) (string, error) {
	chunk, err := BuildSchemaChunk()
	if err != nil {
		return "", fmt.Errorf("build schema chunk: %w", err)
	}
	var b strings.Builder
	b.WriteString(promptPreamble)
	b.WriteString("\n\n")
	b.WriteString(chunk)
	b.WriteString("\n")
	b.WriteString(promptBestPractices)
	b.WriteString("\n\n")
	b.WriteString(promptConstraints)
	b.WriteString("\n\n")
	b.WriteString(selectStyleFragment(style))
	return b.String(), nil
}

func BuildPrompt(input PlanInput) (string, string, error) {
	systemPrompt, err := buildSystemPrompt(input.Style)
	if err != nil {
		return "", "", err
	}

	var b strings.Builder

	b.WriteString("GOAL:\n")
	b.WriteString(input.Goal)
	b.WriteString("\n\n")

	b.WriteString("REPOSITORY SNAPSHOT:\n")
	var snapshot map[string]interface{}
	if err := json.Unmarshal(input.Snapshot, &snapshot); err == nil {
		snapshotJSON, _ := json.MarshalIndent(snapshot, "", "  ")
		b.Write(snapshotJSON)
	} else {
		b.Write(input.Snapshot)
	}
	b.WriteString("\n\n")

	if input.LastIteration != nil {
		b.WriteString("LAST ITERATION:\n")
		b.WriteString(fmt.Sprintf("- Iteration: %d\n", input.LastIteration.Iteration))
		b.WriteString(fmt.Sprintf("- Status: %s\n", input.LastIteration.Status))
		b.WriteString(fmt.Sprintf("- Plan Hash: %s\n", input.LastIteration.PlanHash))
		if len(input.LastIteration.ChangedFiles) > 0 {
			b.WriteString(fmt.Sprintf("- Changed Files: %d\n", len(input.LastIteration.ChangedFiles)))
			for i, f := range input.LastIteration.ChangedFiles {
				if i >= 10 {
					b.WriteString(fmt.Sprintf("  ... and %d more\n", len(input.LastIteration.ChangedFiles)-10))
					break
				}
				b.WriteString(fmt.Sprintf("  - %s\n", f))
			}
		}
		if input.LastIteration.ErrorMessage != "" {
			b.WriteString("- Error:\n")
			errorLines := strings.Split(input.LastIteration.ErrorMessage, "\n")
			for i, line := range errorLines {
				if i >= 20 {
					b.WriteString("  ... (truncated)\n")
					break
				}
				b.WriteString(fmt.Sprintf("  %s\n", line))
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("Generate a Mooncake YAML plan to accomplish the goal.\n")
	b.WriteString("Output ONLY a YAML array of steps (starting with -), no other text.\n")

	return systemPrompt, b.String(), nil
}
