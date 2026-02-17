# Artifact System Examples

This directory contains comprehensive examples demonstrating the artifact capture and validation system for LLM agent loops.

## Overview

The artifact system provides:
- ✅ **Structured output capture** - Track all file changes with detailed metadata
- ✅ **Change budgets** - Enforce constraints on LLM modifications (max files, max lines)
- ✅ **Validation guardrails** - Ensure test coverage, path restrictions, file size limits
- ✅ **Rollback support** - Safe failure handling with automatic recovery
- ✅ **LLM feedback** - JSON/Markdown reports for agent iteration

## Files

### 1. `artifact-capture-example.yml`
**10 practical examples** covering:
- Basic artifact capture
- Content capture (before/after diffs)
- Validation with constraints
- Test coverage requirements
- Path restrictions (allowed/forbidden)
- Complete workflow with rollback
- Multi-iteration tracking
- Conditional validation
- LLM feedback generation

**Run examples:**
```bash
# Run a specific example (by step number)
mooncake run artifact-capture-example.yml --step 1

# Run all examples
mooncake run artifact-capture-example.yml

# Dry-run to preview
mooncake run artifact-capture-example.yml --dry-run
```

### 2. `llm-agent-workflow.yml`
**Production-ready LLM agent loop** with:
- ✅ Environment setup and configuration
- ✅ Pre-flight checks (git clean, baseline tests)
- ✅ Artifact capture of LLM changes
- ✅ Change budget validation
- ✅ Automated testing (lint, unit, integration)
- ✅ Security checks (audit, secrets detection)
- ✅ Rollback on failure
- ✅ Feedback generation for LLM
- ✅ Git integration (optional)

**Usage:**
```bash
# Run with default constraints
mooncake run llm-agent-workflow.yml \
  -e workspace=/tmp/my-workspace \
  -e max_files=15 \
  -e max_lines=500

# Enable auto-commit on success
mooncake run llm-agent-workflow.yml \
  -e auto_commit=true

# Skip cleanup for debugging
mooncake run llm-agent-workflow.yml \
  -e cleanup=false
```

## Quick Start

### Basic Capture

```yaml
- artifact_capture:
    name: "my-changes"
    steps:
      - file_replace:
          path: "config.yml"
          regex: "version: .*"
          replace: "version: 2.0"
```

**Output:**
- `./artifacts/my-changes/changes.json` - Structured metadata
- `./artifacts/my-changes/SUMMARY.md` - Human-readable summary

### Plan Embedding (New!)

**Embed the execution plan in artifacts for complete LLM context:**

```yaml
- artifact_capture:
    name: "llm-changes"
    embed_plan: true  # Includes full plan in artifact
    steps:
      - file_replace:
          path: "main.go"
          regex: "v1"
          replace: "v2"
```

**Benefits:**
- ✅ **Single-file context** - LLM gets plan + results in one JSON
- ✅ **Reproducibility** - Exact steps captured with initial variables
- ✅ **Audit trail** - Know what was executed, not just what changed
- ✅ **Debugging** - See plan and results together
- ✅ **Iteration** - Compare what LLM tried across attempts

**Artifact output with plan:**
```json
{
  "name": "llm-changes",
  "plan": {
    "step_count": 1,
    "steps": [
      {
        "name": "Update version",
        "file_replace": {
          "path": "main.go",
          "regex": "v1",
          "replace": "v2"
        }
      }
    ],
    "initial_vars": { ... }
  },
  "summary": { ... },
  "files": [ ... ]
}
```

**Configuration:**
- `embed_plan: true` - Always embed plan (default for plans ≤ 20 steps)
- `embed_plan: false` - Never embed (use `plan_summary` instead)
- `max_plan_steps: 20` - Don't embed if plan exceeds this (default: 20)

### Basic Validation

```yaml
- artifact_validate:
    artifact_file: "./artifacts/my-changes/changes.json"
    max_files: 10
    max_lines_changed: 200
    require_tests: true
```

**Behavior:**
- ✅ Passes if constraints met
- ❌ Fails step if any constraint violated
- Updates JSON with validation results

## Common Patterns

### Pattern 1: Capture + Validate

```yaml
- artifact_capture:
    name: "refactor"
    steps:
      - repo_apply_patchset:
          patchset_file: "llm.patch"

- artifact_validate:
    artifact_file: "./artifacts/refactor/changes.json"
    max_files: 20
```

### Pattern 2: Validate + Test + Rollback

```yaml
- artifact_capture:
    name: "changes"
    steps:
      - file_replace: ...

- artifact_validate:
    artifact_file: "./artifacts/changes/changes.json"
    max_lines_changed: 500
  register: validation

- shell:
    cmd: "npm test"
  register: tests
  when: validation is succeeded

- shell:
    cmd: "git restore ."
  when: tests is failed
```

### Pattern 3: LLM Feedback Loop

```yaml
- artifact_capture:
    name: "llm-attempt"
    format: "json"
    steps: [...]

- shell:
    cmd: |
      jq .summary ./artifacts/llm-attempt/changes.json > feedback.json
  register: feedback

- print:
    msg: "LLM Feedback: {{ feedback.stdout }}"
```

## Artifact Output Structure

```
artifacts/
  my-changes/
    changes.json       # Full metadata
    SUMMARY.md         # Human-readable summary
```

### changes.json Schema

```json
{
  "name": "my-changes",
  "capture_time": "2026-02-17T12:00:00Z",
  "summary": {
    "total_files": 5,
    "total_lines_added": 120,
    "total_lines_removed": 80,
    "total_lines_changed": 200,
    "files_created": 1,
    "files_updated": 4,
    "files_by_language": {"go": 3, "yaml": 2},
    "files_by_type": {"code": 3, "config": 2},
    "top_changed_files": [
      {"path": "main.go", "lines_changed": 100}
    ]
  },
  "files": [
    {
      "path": "main.go",
      "operation": "updated",
      "lines_added": 80,
      "lines_removed": 60,
      "language": "go",
      "file_type": "code",
      "size_after": 12450
    }
  ],
  "validated": true,
  "validation_pass": true,
  "violations": []
}
```

## Validation Constraints

| Constraint | Description | Example |
|------------|-------------|---------|
| `max_files` | Maximum files changed | `max_files: 10` |
| `max_lines_changed` | Maximum total lines | `max_lines_changed: 500` |
| `max_file_size` | Max individual file size (bytes) | `max_file_size: 1048576` |
| `require_tests` | Code changes → test changes required | `require_tests: true` |
| `allowed_paths` | Only allow these globs | `["src/**/*.js"]` |
| `forbidden_paths` | Never allow these globs | `["node_modules/**"]` |

## Glob Patterns

Supported wildcards:
- `*` - Match any characters (single path component)
- `**` - Match any path components (recursive)

Examples:
- `src/**/*.go` - All .go files under src/
- `**/*.test.js` - All test files anywhere
- `config/*.yml` - Config files in config/ only

## Integration with LLMs

### 1. LLM Generates Patchset

```python
# LLM generates changes
patch = llm.generate_patch(prompt)
with open('/tmp/workspace/llm.patch', 'w') as f:
    f.write(patch)
```

### 2. Mooncake Executes Safely

```bash
mooncake run llm-agent-workflow.yml \
  -e workspace=/tmp/workspace
```

### 3. LLM Receives Feedback

```python
# Read feedback
with open('/tmp/workspace/feedback.json') as f:
    feedback = json.load(f)

if feedback['status'] == 'rejected':
    # Retry with adjustments
    prompt = f"Previous attempt failed: {feedback['failures']}"
    patch = llm.generate_patch(prompt)
```

## Best Practices

### ✅ DO
- Set conservative change budgets (start small)
- Always require test coverage for code changes
- Use forbidden_paths to protect critical files
- Capture artifacts for audit trail
- Generate feedback for LLM iteration

### ❌ DON'T
- Don't skip validation in production
- Don't allow changes to node_modules/, dist/, etc.
- Don't capture content for large files (use max_diff_size)
- Don't auto-commit without review
- Don't ignore test failures

## Troubleshooting

### Validation Fails: "Too many files changed"

```yaml
# Reduce scope of changes
max_files: 5  # More restrictive
```

### Validation Fails: "Code changed without test changes"

```yaml
# Ensure test files are modified
require_tests: true

# Or disable if only config changes
require_tests: false
```

### Artifact capture shows 0 files

**Cause:** Steps didn't modify any files

**Fix:** Check that steps are actually executing (use `--dry-run` to debug)

### Path restrictions not working

**Cause:** Glob pattern too restrictive

**Fix:**
```yaml
# ❌ Too restrictive
allowed_paths: ["src/utils/format.js"]

# ✅ Better
allowed_paths: ["src/**/*.js"]
```

## Advanced Usage

### Multi-Stage Validation

```yaml
# Stage 1: Lenient (exploration)
- artifact_validate:
    artifact_file: "./artifacts/explore/changes.json"
    max_files: 50

# Stage 2: Strict (production)
- artifact_validate:
    artifact_file: "./artifacts/final/changes.json"
    max_files: 10
    require_tests: true
```

### Comparative Analysis

```yaml
# Compare two artifact captures
- shell:
    cmd: |
      diff <(jq .summary ./artifacts/v1/changes.json) \
           <(jq .summary ./artifacts/v2/changes.json)
```

### Custom Metrics

```yaml
# Extract custom metrics from artifact
- shell:
    cmd: |
      jq '{
        complexity: (.summary.total_lines_changed / .summary.total_files),
        test_ratio: (.summary.test_files_count / .summary.code_files_count)
      }' ./artifacts/my-changes/changes.json
```

## See Also

- [Action Reference](../docs/guide/config/actions.md#artifact_capture)
- [LLM Agent Guide](../docs/guide/llm-agents.md)
- [Preset System](../docs/guide/presets.md)
