# Plan Compiler Demo

This directory demonstrates the deterministic plan compiler feature in mooncake.

## Overview

The plan compiler allows you to:
1. **Generate a deterministic execution plan** - Same config always produces identical plan
2. **Inspect before execution** - Review what will be executed before running
3. **Save and reuse plans** - Save plans to JSON/YAML for later execution
4. **Full traceability** - Every step tracks its origin file, line, and include chain

## Commands

### Generate a plan (text format)
```bash
mooncake plan -c main.yml --format text
```

### Generate a plan with origin tracking
```bash
mooncake plan -c main.yml --format text --show-origins
```

### Save plan to JSON
```bash
mooncake plan -c main.yml -o plan.json
```

### Save plan to YAML
```bash
mooncake plan -c main.yml -o plan.yaml
```

### Execute from saved plan
```bash
mooncake run --from-plan plan.json
```

### Filter by tags during planning
```bash
mooncake plan -c main.yml --tags install --format text
```

This marks non-matching steps as `skipped` in the plan.

## Features Demonstrated

### 1. Loop Expansion
The `with_items` directive expands into multiple steps:
- Each iteration gets `item`, `index`, `first`, `last` variables
- Loop context is preserved in the plan

### 2. Include Files
The `include` directive is expanded at plan-time:
- Includes are recursively resolved
- Cycle detection prevents infinite loops
- Include chain is tracked for each step

### 3. Tag Filtering
Steps can be filtered by tags:
- Non-matching steps are marked as `skipped`
- Skipped steps appear in plan but won't execute

### 4. When Conditions
`when` conditions are preserved in plan:
- Evaluated at runtime (may depend on step results)
- Not evaluated during planning

### 5. Deterministic Ordering
- File tree loops (`with_filetree`) are sorted alphabetically
- Same config always produces identical plan (except timestamp)
- Step IDs are sequential: step-0001, step-0002, etc.

### 6. Origin Tracking
Every step records:
- Source file path
- Line and column number
- Include chain (for included steps)

## Example Output

```
Plan: main.yml
Generated: 2026-02-04 12:00:00
Steps: 6

[1] Setup step (ID: step-0001)
    Action: shell
    Tags: setup

[2] Install curl (ID: step-0002)
    Action: shell
    Tags: install
    Loop: with_items[0] (first=true, last=false)

[3] Install git (ID: step-0003)
    Action: shell
    Tags: install
    Loop: with_items[1] (first=false, last=false)

[4] Install vim (ID: step-0004)
    Action: shell
    Tags: install
    Loop: with_items[2] (first=false, last=true)

[5] Common configuration (ID: step-0005)
    Action: shell
    Tags: common
    Origin: /path/to/common.yml:1:1
    Chain: /path/to/main.yml:19

[6] Final step (ID: step-0006)
    Action: shell
```

## JSON Plan Structure

Plans saved as JSON have this structure:

```json
{
  "version": "1.0",
  "generated_at": "2026-02-04T12:00:00Z",
  "root_file": "main.yml",
  "steps": [
    {
      "id": "step-0001",
      "origin": {
        "file": "/path/to/main.yml",
        "line": 10,
        "column": 3
      },
      "name": "Step name",
      "tags": ["install"],
      "when": "env == 'production'",
      "skipped": false,
      "action": {
        "type": "shell",
        "data": {
          "command": "echo hello"
        }
      },
      "loop_context": {
        "type": "with_items",
        "item": "curl",
        "index": 0,
        "first": true,
        "last": false,
        "loop_expression": "packages"
      }
    }
  ],
  "initial_vars": {
    "env": "production"
  },
  "tags": ["install"]
}
```

## Benefits

1. **Predictability** - Know exactly what will run before execution
2. **Debugging** - Trace any step back to its source
3. **CI/CD** - Generate plans in CI, review, then execute
4. **Compliance** - Audit trail of what was planned vs executed
5. **Testing** - Verify plan structure without execution
6. **Reproducibility** - Same plan file ensures identical execution
