# Mooncake agent eval harness

Eval harness for the `mooncake agent` (née `mooncake agent`) loop. See
spec-67 §14 for the full design.

The prompt is the moat. This harness exists so that prompt iteration
does not silently regress plan quality.

## What it does

For each `(goal, snapshot, assertions)` tuple in `goals/`:

1. Load the snapshot referenced by `snapshot:` from `snapshots/`.
2. Build a system+user prompt via `internal/agent.BuildPrompt` (the
   same call path the live agent loop uses).
3. Call `internal/agent/llm.NewClient().GeneratePlan(...)` against the
   provider/model named in the goal file.
4. Parse the returned YAML plan.
5. Run the assertions listed in the goal file against the parsed plan.
6. Report pass / fail per assertion, per goal, plus a summary line.

## Running locally

Real LLM calls cost real money. The runner refuses to start unless
you opt in:

```
MOONCAKE_AGENT_EVAL=1 task agent-evals
```

Or directly:

```
MOONCAKE_AGENT_EVAL=1 go run ./testing-next/agent-evals/
```

A valid provider must be configured: either `claude` on `$PATH` (the
CLI client takes precedence) or `CLAUDE_API_KEY` set in the
environment (HTTP client fallback). See `internal/agent/llm/client.go`.

### Dry-run

`-dry-run` parses every goal file, snapshot, and assertion string
without contacting any LLM. Use it to validate the harness shape in
CI on PRs that did not opt into a paid run:

```
go run ./testing-next/agent-evals/ -dry-run
```

### Filter to one goal

```
MOONCAKE_AGENT_EVAL=1 go run ./testing-next/agent-evals/ -only 001-create-service-and-verify
```

## Layout

```
testing-next/agent-evals/
├── goals/         # (goal, snapshot, assertions) YAML tuples
├── snapshots/     # canned snapshots fed to the prompt (JSON)
├── assertions/    # assertion grammar + checks
├── run.go         # the runner (gated on MOONCAKE_AGENT_EVAL=1)
└── README.md      # this file
```

## Assertion grammar

Each assertion is a single string. Supported forms:

| Form | Meaning |
|---|---|
| `schema_valid` | Plan parses + validates against `internal/config/schema.json`. |
| `contains_step <action>` | At least one step uses `<action>` (e.g. `shell`, `file_replace`). |
| `contains_step_with <action> <field>=<substring>` | At least one `<action>` step whose `<field>` contains `<substring>`. |
| `step_count <= N` | Plan has at most N steps. |
| `step_count >= N` | Plan has at least N steps. |
| `no_step <action>` | Plan does not use `<action>` (e.g. block `shell` where structured actions exist). |

Unknown forms fail to parse — that is intentional, so a typo in a
goal file surfaces immediately instead of silently passing.

## CI hook

`.github/workflows/agent-evals.yml.disabled` runs the harness on PRs
labeled `needs-agent-eval`. The workflow is currently `.disabled` to
match the project's other workflow files; enable by renaming when CI
is wired up.

## Adding a goal

1. Drop a new `goals/NNN-<slug>.yml` file (next free number).
2. Reference an existing `snapshots/<name>.json` or add a new one.
3. Pick a small set of assertions — 3–6 is the sweet spot. Aim for
   assertions that catch obvious regressions, not exhaustive plan
   structure. The LLM has room to choose; assertions police the
   floor.
4. Run `-dry-run` to confirm parse, then a real run to confirm pass.

## Cost guard

The starter set is ~5 goals. At Claude Sonnet pricing as of 2026-05,
a full sweep is single-digit cents. Keep it that way: do not let
this directory grow into a regression-test dumping ground.
