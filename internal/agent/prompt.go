package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alehatsman/mooncake/internal/executor"
)

const promptPreamble = `You are a Mooncake agent planner. Generate ONLY a valid Mooncake configuration.

OUTPUT REQUIREMENTS:
- Output ONLY a compact JSON array of steps (Mooncake RunConfig format), no other text
- NO markdown fences, NO prose, NO explanations, NO comments
- The JSON must be directly parseable by the Mooncake validator
- Example: [{"name":"create greeting","cmd":{"argv":["sh","-c","echo hello > /tmp/greeting.txt"]}}]`

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
//
// policy, when non-nil and non-zero, injects the PERMISSIONS CONTRACT
// block (#11) right after the action vocabulary, so the model reads the
// full action list and then the constraint on which subset it may use.
// A nil/zero policy keeps the prompt byte-identical to the pre-policy
// shape.
func buildSystemPrompt(style Style, policy *executor.Policy) (string, error) {
	chunk, err := BuildSchemaChunk()
	if err != nil {
		return "", fmt.Errorf("build schema chunk: %w", err)
	}
	var b strings.Builder
	b.WriteString(promptPreamble)
	b.WriteString("\n\n")
	b.WriteString(chunk)
	b.WriteString("\n")
	if contract := renderPolicyContract(policy); contract != "" {
		b.WriteString(contract)
		b.WriteString("\n\n")
	}
	b.WriteString(promptBestPractices)
	b.WriteString("\n\n")
	b.WriteString(promptConstraints)
	b.WriteString("\n\n")
	b.WriteString(selectStyleFragment(style))
	return b.String(), nil
}

func BuildPrompt(input PlanInput) (string, string, error) {
	systemPrompt, err := buildSystemPrompt(input.Style, input.Policy)
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
		// Surface the step that actually broke the plan — name, action, exit
		// code, and its stderr tail. This is the most actionable feedback on
		// a failure: without it the planner sees only the top-level error
		// string and re-proposes the same failing step (#71). The directive
		// tells it to FIX or REPLACE that step rather than reproduce the plan.
		if fs := input.LastIteration.FailedStep; fs != nil {
			b.WriteString(fmt.Sprintf("- Failed Step: %q (action: %s, exit code: %d)\n", fs.Name, fs.Action, fs.ExitCode))
			if fs.Stderr != "" {
				b.WriteString("  stderr (last 4 KB):\n")
				b.WriteString("  ```\n")
				for _, line := range strings.Split(strings.TrimRight(fs.Stderr, "\n"), "\n") {
					b.WriteString("  ")
					b.WriteString(line)
					b.WriteString("\n")
				}
				b.WriteString("  ```\n")
			}
			b.WriteString("  This step FAILED. Do NOT re-propose the same step unchanged — fix it, take a different approach, or omit it.\n")
		}
		// Per-step outcomes for every action type — closes the
		// non-cmd/shell signal gap. file.write / pkg.install /
		// os.service etc. produce no stdout but still need to confirm
		// to the model "yes, this step ran and did X". Each summary
		// is already truncated and newline-collapsed at capture time
		// (see output_capture.go summarizeStep); printed verbatim
		// here in step.completed order.
		if len(input.LastIteration.StepSummaries) > 0 {
			b.WriteString("- Step Outcomes:\n")
			for _, line := range input.LastIteration.StepSummaries {
				b.WriteString("  - ")
				b.WriteString(line)
				b.WriteString("\n")
			}
		}
		// Surface the captured stdout from the last cmd/shell step so
		// the model can decide whether the goal is already answered.
		// Without this block step-style agent loops re-propose the same
		// diagnostic step forever — the LLM has no way to see the
		// command's output. Already 4 KiB-tail-truncated at capture
		// time (see output_capture.go); printed verbatim here.
		if input.LastIteration.LastStepStdout != "" {
			b.WriteString("- Last Step Stdout (last 4 KB):\n")
			b.WriteString("```\n")
			b.WriteString(input.LastIteration.LastStepStdout)
			if !strings.HasSuffix(input.LastIteration.LastStepStdout, "\n") {
				b.WriteString("\n")
			}
			b.WriteString("```\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("Generate a Mooncake plan to accomplish the goal.\n")
	b.WriteString("Output ONLY a compact JSON array of steps, no other text.\n")

	return systemPrompt, b.String(), nil
}
