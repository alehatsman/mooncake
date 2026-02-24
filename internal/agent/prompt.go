package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

const systemPrompt = `You are a Mooncake agent planner. Generate ONLY valid Mooncake YAML configuration.

OUTPUT REQUIREMENTS:
- Output ONLY raw YAML (Mooncake RunConfig format)
- NO markdown fences, NO prose, NO explanations, NO comments
- The YAML must be directly parseable by the Mooncake validator

MOONCAKE SCHEMA:
A Mooncake config is a YAML array of steps. Each step has:
- Optional 'name' field
- Exactly ONE action from: shell, command, file, file_replace, file_insert, file_delete_range, file_patch_apply, repo_search, repo_tree, assert, template, copy, download, unarchive, service, preset, print, vars, include_vars
- Optional: when, loop, tags, register, changed_when, failed_when

AVAILABLE ACTIONS:
- file_replace: Replace exact text in file (old_string, new_string, path)
- file_insert: Insert text at anchor (path, anchor, content, position: before/after)
- file_delete_range: Delete lines (path, start_line, end_line)
- file_patch_apply: Apply unified diff patch (path, patch)
- repo_search: Search code with regex (pattern, glob, output_mode)
- repo_tree: List repo structure (path, max_depth, glob)
- assert: Verify conditions (command/file/http checks)
- command: Execute command with argv array (cmd)
- shell: Execute shell command (cmd)
- file: Manage files (path, content - omit state or use state: file for files, state: directory for dirs)
- print: Output message (msg)

BEST PRACTICES:
- Prefer file_replace/file_insert over shell sed/awk
- Use repo_search to find code before editing
- Use assert to verify changes
- Keep plans small (<= 30 steps)
- Use command over shell when possible
- Include verification steps

CONSTRAINTS:
- Plans must be idempotent where possible
- No interactive commands
- All file paths must be absolute or relative to repo root`

func BuildPrompt(input PlanInput) (string, string, error) {
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
	b.WriteString("Example format:\n")
	b.WriteString("- name: step1\n")
	b.WriteString("  file:\n")
	b.WriteString("    path: /path/to/file\n")
	b.WriteString("    content: data\n")

	return systemPrompt, b.String(), nil
}
