---
id: introspection-tooling
status: draft
owners: [aleh]
covers:
  - "internal/doctor/*.go"
  - "internal/explain/*.go"
  - "internal/docgen/*.go"
  - "internal/schemagen/*.go"
  - "internal/queryio/*.go"
  - "internal/pathquery/*.go"
  - "cmd/doctor/*.go"
  - "cmd/docs/*.go"
  - "cmd/schema/*.go"
  - "cmd/query/*.go"
  - "cmd/kernel/explain.go"
---

# Introspection & DX Tooling

## Intent
The introspection surface is the read-only developer-experience layer that
reports on, explains, and exports mooncake's own state and metadata without
mutating anything. `doctor` audits the install/system/state/project; `explain`
answers typed questions about a noun (action verb, run, resource, op); `docs`
emits action documentation from the live registry; `schema` exports JSON
Schema / OpenAPI / TypeScript from the same metadata so editors and validators
stay in sync; `query` extracts a value from a JSON/YAML file by dotted path with
agent-friendly exit codes. All of these derive their output from the action
registry, embedded schema, and on-disk state — never from the network.

## Behavior
- WHEN `mooncake doctor` runs, it SHALL execute a fixed catalogue of checks
  (install, system, state, presets, tools, project, services) and report each as
  OK / info / warning / error with a concrete fix hint, in deterministic order.
- WHERE `--section` is given it SHALL restrict to those sections (repeatable);
  `--skip-project` SHALL omit the cwd project checks; `--no-color` (or
  `NO_COLOR`) SHALL disable colour.
- WHEN `doctor` finishes, it SHALL exit 0 on success-or-warnings, 1 on errors,
  and — only under `--strict` — 1 on warnings; `--format json` SHALL emit the
  `Report` JSON contract (counts, checks, timing).
- WHEN `mooncake explain <noun>` runs, it SHALL resolve the noun to a typed
  discriminated-union `Result` (`action`/`run`/`resource`/`op`/`not_found`) and
  render it as text/json/yaml via `--format`.
- WHERE the noun is an action verb, it SHALL return action metadata, schema, and
  up to `--examples-limit` (0–10, default 3) usage excerpts drawn from the
  in-tree `examples/`; a `not_found` result SHALL exit non-zero on the CLI.
- WHEN `mooncake docs generate --section <s>` runs, it SHALL emit a single
  markdown section (platform-matrix, capabilities, action-summary,
  action-properties, preset-examples, schema, all) to `--output` or stdout, and
  SHALL skip the write when the existing file is byte-equal ignoring the
  generated header.
- WHEN `--section all-into-dir` runs, `--output` SHALL be a required directory
  and the generator SHALL write a per-topic file tree (action cards, schema,
  properties, matrices, preset examples) ready for MkDocs / llms.txt.
- WHEN `mooncake schema generate` runs, it SHALL emit JSON Schema (default),
  YAML, OpenAPI 3.0, or TypeScript `.d.ts` from the action registry and Go
  structs, honouring `--extensions`/`--examples`/`--strict`.
- WHEN `mooncake schema validate --schema <f>` runs, it SHALL regenerate the
  current schema with the same `--strict`/`--extensions` settings, byte-compare
  via the same writer, and exit 1 when out of date.
- WHEN `mooncake query <file> <path>` runs, it SHALL auto-detect format from
  extension (override `--as`), validate the dotted/bracketed path, read within
  `--max-bytes` (default 4 MiB), and print scalars raw / structured values as
  JSON (`--pretty` indents).
- WHEN `query` resolves, it SHALL exit 0 on a found value, 1 on a path miss
  (file parsed, key absent), and 2 on an unreadable/oversize/parse error.

## Non-goals
- Any state mutation — every command here is read-only / generate-to-output;
  apply/plan/rollback are owned by the execution-engine spec.
- The action registry, metadata, and embedded schema that these tools read —
  owned by the actions / config-model specs.
- `explain` wave-2 run/resource/op resolution and the MCP `explain` tool wiring
  (declared but not the CLI surface here).
- General dotted-path semantics shared with `read.json`/`read.yaml` action
  handlers (this spec covers only the `query` CLI over that path engine).

## Checklist
- [x] `doctor` fixed check catalogue across 7 sections with OK/info/warn/error +
  fix hints; deterministic order.
- [x] `doctor` `--section`/`--skip-project`/`--no-color`/`--strict`; exit-code
  contract; `--format json` Report.
- [x] `explain` typed union result; text/json/yaml; action metadata + schema +
  `examples/` excerpts (`--examples-limit` 0–10); `not_found` exits non-zero.
- [x] `docs generate` single-section markdown (idempotent header-aware write) +
  `all-into-dir` MkDocs/llms tree.
- [x] `schema generate` json/yaml/openapi/typescript; `--extensions`/`--examples`/
  `--strict`.
- [x] `schema validate` regenerate-and-byte-compare with matching flags; exit 1
  out of date.
- [x] `query` format auto-detect (`--as`), path validation, `--max-bytes` bound,
  scalar-raw / JSON (`--pretty`) output.
- [x] `query` agent exit codes: 0 found, 1 path-miss, 2 read/parse error.
