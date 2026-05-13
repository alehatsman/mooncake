# Spec 13: Single-Step Execution

**Epic:** E6 Agent-Native Interface (S6.2)  
**Effort:** XS (1–2h)  
**Value:** Medium — agents can test one action and inspect the result before committing to a full plan

---

## Problem

Running a full config to test one action is too heavy. Agents and power users want
to fire a single inline step and get back a structured result without writing a
config file.

---

## Goal

```
mooncake step 'shell: {cmd: "echo hello"}'
mooncake step 'package: {name: git, state: present}'
mooncake step 'file: {path: /tmp/test.txt, content: "hello"}'
```

Executes the step inline, prints JSON result to stdout:

```json
{"changed": false, "action": "package", "stdout": "", "stderr": "", "duration_ms": 42}
```

On failure:
```json
{"changed": false, "action": "shell", "error": "exit code 1", "stdout": "", "stderr": "command not found", "duration_ms": 10}
```

Exit code mirrors step success (0 = ok, 1 = failed).

---

## Implementation

### `cmd/step.go`

New `stepCommand()`:

1. Take first positional arg as inline YAML string
2. Parse as `config.Step` (same parser used by config loader)
3. Build a minimal executor context (no full config needed)
4. Run the single step via `executor.DispatchStepAction`
5. Marshal result to JSON, print to stdout
6. Exit 0 on success, 1 on failure

Use `--become` and `--dry-run` flags (same as run command).

### `cmd/mooncake.go`

Register `stepCommand()` in Commands list.

---

## Result JSON Schema

```json
{
  "changed": bool,
  "action": string,
  "stdout": string,       // trimmed, empty if none
  "stderr": string,       // trimmed, empty if none
  "error": string,        // omitempty
  "duration_ms": int64
}
```

---

## Acceptance Criteria

1. `mooncake step 'shell: {cmd: "echo hi"}'` prints JSON with `changed: true`.
2. `mooncake step 'package: {name: git}'` reports `changed: false` if already installed.
3. A failing step prints JSON with `error` field and exits 1.
4. `--dry-run` flag passes through to the action handler.
5. Unknown action type exits 1 with error JSON.
6. `mooncake --help` lists `step` subcommand.
