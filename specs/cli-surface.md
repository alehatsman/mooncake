---
id: cli-surface
status: draft
owners: [aleh]
covers:
  - "cmd/mooncake.go"
  - "cmd/kernel/apply.go"
  - "cmd/kernel/plan.go"
  - "cmd/kernel/state.go"
  - "cmd/dev/*.go"
---

# CLI Surface (command tree shape)

## Intent
This spec owns the *shape* of `mooncake`'s command tree — which verbs exist at
the top level, how they group, and which are hidden — not what any individual
command does. Per-command behavior stays with the spec that owns it
(execution-engine, task-runner, modules, fleet, agentd, introspection-tooling).

The tree grew to 26 flat top-level commands with no tiers, six ways to execute
something, four separately-implemented vocabularies for reading machine state,
and maintainer-only build tooling sitting next to `apply`. This spec collapses
that to one categorised tree with exactly one preview path and one state noun.

There are no deprecation aliases. Pre-1.0, no external users; the break is
taken once, in one commit, with `tasks.yml` and CI moving in the same change.

## Behavior

### Grouping
- WHEN `mooncake --help` renders, every command SHALL declare a `Category` and
  SHALL be listed under it: `Run`, `Inspect`, `Fleet`, `Manage`, `Daemon`,
  `Agent`, and (hidden) the `dev` tree.
- WHERE a command is maintainer-only tooling — documentation generation, schema
  export, cross-compilation — it SHALL live under `mooncake dev <verb>` and
  `dev` SHALL be `Hidden` so it does not appear in the top-level command list;
  it SHALL remain fully functional when typed, and `mooncake dev --help` SHALL
  list its subcommands.

### One preview path
- WHEN a preview is requested, `mooncake plan` SHALL be the only way to get one.
  `apply --dry-run` SHALL be removed, not aliased.
- WHERE `--dry-run` is passed to `apply`, it SHALL be rejected with a usage
  error naming `mooncake plan`, matching how `task` already rejects it.

### Inline step execution — `step` is retained

The original Phase 1 sketch folded `mooncake step` into `apply --step`. That
was wrong on two counts, both found by checking consumers before writing code:

1. **It has an external consumer.** moongit's CI runner (`internal/ci/ci.go`)
   execs `mooncake step '<yaml>'` for every CI step, and this repo's own
   `ci/Dockerfile` exists to put that binary on the runner's PATH. Removing
   the verb breaks mooncake's own CI, in another repo, with no alias to catch
   it.
2. **It is not a duplicate path.** `step` deliberately bypasses the planner —
   it builds a minimal `ExecutionContext` and calls `DispatchStepAction`
   directly — and emits a *flat per-step* JSON payload
   (`RegisteredResult.ToMap()` plus `action`/`error`/`duration_ms`). `apply
   --output-format json` emits a run-level event stream. They are different
   contracts for different callers, not two spellings of one thing.

- `mooncake step` SHALL be retained, in the `Run` category.
- Its flat JSON payload SHALL remain the contract, since a CI runner parses it.
- It SHALL keep `--dry-run`, which selects `actions.ModePlan` for the single
  dispatch. This is the one surviving `--dry-run` in the tree: `step` has no
  `plan` counterpart to steer users toward, so the "one preview path" rule
  does not reach it.

### One state noun
- WHEN machine state is read, `mooncake state` SHALL be the single entry point,
  replacing the separate `facts`, `metrics`, and `snapshot` commands.
- WHEN `mooncake state` runs with no subcommand, it SHALL print the composite
  machine snapshot, accepting `--budget`, `--diff`, and `--save` as the retired
  `snapshot` command did.
- WHEN `mooncake state facts` runs, it SHALL print host facts, accepting
  `--query` (repeatable dot-path).
- WHEN `mooncake state metrics` runs, it SHALL print live metrics, accepting
  `--query`, `--fields`, `--refresh`, and `--output`.
- WHERE any `state` view runs, `--format text|json` SHALL select the rendering,
  and the JSON shape SHALL be unchanged from the command it replaced so existing
  consumers only change the verb.

### Machine-readable output
- WHERE a command reads rather than mutates, it SHALL accept `--format
  text|json` and emit a stable JSON document under `json`.
- This SHALL hold for the six read commands that lacked it: `drift inspect`,
  `tool list`, `mod cache list`, `cron list`, `vault list`, and
  `vault recipients list`.
- `actions list`, `runs list`, and `history list` already carried `--format`;
  an earlier audit over-counted them as gaps by grepping the wrong help level.
- WHEN `--format json` is given, the command SHALL write only the JSON document
  to stdout; human-oriented headers, colour, and progress SHALL go to stderr or
  be suppressed, so the stream parses without filtering.

### Removals
- `mooncake query` SHALL be removed. It read a value from a JSON/YAML file by
  dotted path — a job `jq`/`yq` already do at the shell, and one the
  `read.json` / `read.yaml` actions already do inside a playbook. The
  `internal/pathquery` engine those actions share SHALL be retained.

## Non-goals
- What `apply`, `plan`, `task`, `fleet`, `mod`, `vault`, or `agentd` *do* —
  owned by their own specs. This spec only says where they sit and how they are
  spelled.
- The `mooncake agent` command's eventual removal — it keeps a top-level slot
  under the `Agent` category until the split to its own repo lands.
- Collapsing the `observe.*` / `wait.*` action families or `fleet observe`,
  which is the other half of the state-vocabulary problem and needs the handler
  work first.
- Any change to plan/apply semantics, exit codes, or event streams.

## Checklist
- [ ] Every top-level command declares a `Category`; help renders tiers.
- [ ] `dev` parent command, `Hidden`, carrying `docs` / `schema` / `selfbuild`.
- [x] `apply --dry-run` removed; passing it is a usage error naming `plan`.
- [ ] `step` retained in the `Run` tier with its flat JSON contract and
      `--dry-run` intact — moongit's CI runner execs it.
- [ ] `mooncake state` composite snapshot with `--budget` / `--diff` / `--save`.
- [ ] `state facts` (`--query`) and `state metrics`
      (`--query`/`--fields`/`--refresh`/`--output`); `facts`, `metrics`,
      `snapshot` top-level commands removed.
- [ ] `--format json` on `drift inspect`, `tool list`, `mod cache list`,
      `cron list`, `vault list`, `vault recipients list`.
- [x] `mooncake query` removed; `internal/pathquery` retained for `read.*`.
- [ ] `tasks.yml`, `mgitci.yml`, and `scripts/` updated to the new spellings in
      the same commit — no aliases means no grace period.
- [ ] A test asserts the top-level command set and their categories, so the
      tree can't silently regrow.
