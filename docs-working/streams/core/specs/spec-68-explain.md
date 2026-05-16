# Spec 68: `mooncake explain <noun>` + MCP `explain` tool

**Status:** Draft
**Stream:** core
**Promotes:** Brainstorm 2026-05-16, "Prompt 2" reframe; story
[`S-explain-spec`](../../../vision/brainstorm/2026-05-16-stories.md#s-explain-spec).
Synthesis line ~1144 lists this as the first concrete spec to fall out
of the brainstorm.
**Effort:** M (~3–5 days, rolled out in three waves: noun resolver +
CLI text, MCP tool wiring, `ops`-table migration).
**Prerequisites:** none. Reads `schema.json`
(`internal/config/validator.go:17 SchemaJSON()`), handler metadata
(`actions.ActionMetadata`), and the runlog
(`internal/runlog/runlog.go`). Adds one new persistence surface
(`~/.mooncake/ops.jsonl`) under the same JSONL discipline.

---

## Problem

An LLM agent — Cursor, Claude Code, Codex, Zed via MCP — that opens a
mooncake-managed repo today has to grep three nested search surfaces
to learn how a single verb works: `docs/`, `docs-working/`, the spec
that introduced the verb, the examples in `examples/`. None of those
surfaces are typed, none are addressable by noun, none ship a Diff or
Reverse shape the agent can pre-flight against. The data is in the
binary — `schema.json` is embedded, every handler returns
`ActionMetadata`, every run lands a typed record in
`~/.mooncake/runs.jsonl`. The agent just can't query it.

The same gap shows up for humans: `mooncake history` lists runs, but
the operator can't ask "what changed on `/etc/nginx/nginx.conf` in
the last week?" or "what handler did the `pkg.install` step in run
`r/4af2…` actually invoke, and what's its Reverse shape?" The kernel
knows; nothing renders.

This spec is a **rendering of the existing kernel** —
[`kernel.md`](../../../vision/kernel.md) §"What this enables" already
lists `mooncake explain <resource>` as derived from the audit trail.
This formalizes the noun set, the CLI surface, the MCP surface, and
the one schema decision that has to be made *now* because every
downstream rollback / replay / multi-run query depends on it:
**`op_id`**.

The gap is **the verb and a typed identity for operations**, not new
data.

## Goals

1. **Typed answers to typed nouns, served from the kernel.** A single
   verb — `mooncake explain <noun>` — returns the structured answer
   for four noun kinds: action verb, run id, resource, op id. Same
   data, two frontends (CLI text/JSON, MCP tool).
2. **MCP-first surface.** The MCP `explain` tool ships in the same
   PR wave as the CLI; agent-dev is the lighthouse audience per
   [`BOTTLENECK.md`](#) (when it ships) and brainstorm Synthesis
   §"Disagreement 2". The CLI text mode is the demo; the MCP tool
   is the product.
3. **An `op_id` schema that survives the three failure modes named
   below.** This is the one decision the preamble has to make. The
   implementation lands as a `ops.jsonl` migration in
   [`S-explain-impl`](../../../vision/brainstorm/2026-05-16-stories.md#s-explain-impl).
4. **Zero RAG. Zero embeddings. Zero third-party services.** Source
   of truth is `schema.json` + handler metadata + runlog. The "agent
   ramp" residue (decisions, drift findings, recent PRs) is the
   *next* spec's problem
   ([`S-facts-repo-index`](../../../vision/brainstorm/2026-05-16-stories.md#s-facts-repo-index)),
   not this one.
5. **One spec, not two.** `op_id` does not get its own spec; the
   schema choice is ten lines in the preamble below and the
   migration is two extra lines in `S-explain-impl`.

## Non-goals

- **Generic fact-provider plugin SDK.** The closed action set is the
  moat (`kernel.md` "What this is NOT"); a "noun-explain plugin"
  interface would break it by the second contributor. The noun set
  is closed at four and grows via the normal spec path.
- **RAG over `docs/`, `docs-working/`, git log, or PR history.** The
  residue is real (brainstorm §C.2) but lives in
  `S-facts-repo-index`, not here. If `explain` cannot answer a
  question from typed sources, it returns a typed "no match" — it
  does not fall through to keyword search.
- **A new ABI method on `Handler`.** `Metadata()` already returns
  enough to render the action noun. `Differ.Diff` and
  `Reverser.Reverse` are queried via the existing optional
  sub-interfaces. The kernel surface stays at four properties.
- **Rendering decisions for the typed payloads.** Spec 66
  (`spec-66-typed-plan-diff.md`) owns the per-kind rendering of
  `Diff`. `explain` consumes the same typed shapes; it does not
  reinvent them.
- **Editing or mutating anything.** `explain` is read-only across all
  noun kinds, including `<op-id>`. Replay/rollback are separate
  verbs.
- **`mooncake facts`.** The existing `mooncake facts` subcommand and
  `internal/explain` package (which today only displays system facts
  — CPU, memory, GPUs) is unrelated to this verb despite the name
  collision. See "Implementation notes" below for the cleanup.

## The `op_id` schema decision

**Position: `op_id` is a foreign-key into a separate `ops.jsonl`,
not a flat column on the run record.** Two extra lines of migration
today; saves a destructive change later.

Argument, condensed from Innovator's second pass (§α). The minimal
alternative — extend `runlog.Entry` with an `OpID` string column —
holds today *only because* ops and runs are 1:1 in mooncake. Three
shipments break the 1:1 and they are all on the near roadmap:

1. **`mooncake plan` produces an op with no run.** A plan-mode
   invocation generates a plan artifact and a typed Diff; nothing
   is mutated, so no runlog entry exists. The op happened; the run
   did not. Once `explain <op-id>` ships and an agent calls
   `inspect_plan` over MCP, the op needs an identity that does not
   have to route through a (nonexistent) run record. A column on
   the run table cannot represent a row that does not exist.

2. **`mooncake rollback` is one op that reverses N prior runs.**
   What's the `op_id` on the rollback's run record? Its own op, the
   op of the original failed apply, or the op of each reversed step?
   All three are useful for different queries — "show me my last
   rollback" / "show me every attempt to apply this plan" / "show me
   every reverse step ever executed against this resource". One
   correlation column is one answer; the real query surface is at
   least three.

3. **`mooncake replay <run-id>` is an op-on-an-op.** Replay-of-a-
   replay-of-an-apply is a chain. A flat correlation field holds one
   predecessor; the third replay loses the first.

The closer analogy is **Postgres WAL + `pg_stat_statements`**, not
jj's oplog. WAL is the transaction log (runs); `pg_stat_statements`
is the operation log (commands that triggered transactions). Same
database, two log surfaces, neither subsumes the other. Postgres did
not pre-split out of theoretical purity — it split because writing
statement metadata into the WAL would have made the WAL useless for
its actual job.

The schema, decided now:

```
~/.mooncake/runs.jsonl                  (existing — gains run_id, op_id)
~/.mooncake/ops.jsonl                   (new — append-only, JSONL)

ops.jsonl entry:
  {
    "ts":      RFC3339,
    "op_id":   "op/<22-char base32>",   // ULID-shaped, sortable
    "command": "apply" | "plan" | "rollback" | "replay" | ...,
    "args":    [<string>, ...],         // pre-redaction
    "actor":   "user:<name>" | "agent:<mcp-client-id>" | "ci",
    "parent":  "op/..." | null,         // chains replay-of-replay
    "config":  "<path-to-mooncake.yml>",
    "runs":    ["r/...", ...]           // populated as runs land
  }

runs.jsonl entry (additions only):
  {
    ...existing fields...,
    "run_id":  "r/<22-char base32>",    // surfaced; today implicit
    "op_id":   "op/..."                 // FK into ops.jsonl
  }
```

The "FK" is JSONL-shaped — no database, no constraints, just a
sortable opaque id that resolves by linear scan or by a simple
on-disk index added later if a user asks. The discipline matches the
rest of `~/.mooncake/`.

This spec writes the schema and the noun. `S-explain-impl` writes
the migration and the resolver.

## Reuse map — what's already in the tree

| Capability | Where | Status |
|---|---|---|
| Embedded JSON Schema | `internal/config/schema.json` + `SchemaJSON()` | ✓ |
| `ActionMetadata` (name / category / version / platforms / sudo / dryrun / events) | `internal/actions/handler.go:62` | ✓ |
| Per-action registry | `internal/actions.List()` | ✓ |
| `x-category` per verb in schema | `internal/config/schema.json` | ✓ |
| Typed `Diff` shape per handler | `internal/actions/handler_abi.go` + handler `diff.go` files | ✓ Spec-22 phase 4 |
| Typed `Reverse` shape per handler | `internal/actions/handler_abi.go` + handler `reverse.go` files | ✓ 11/13 priority handlers |
| Runlog (read + append) | `internal/runlog/runlog.go` | ✓ — needs `run_id`, `op_id` |
| MCP server + tool registration | `internal/mcp/server.go`, `internal/mcp/tools.go` | ✓ |
| Facts display (unrelated, name-collision only) | `internal/explain/` | ✓ — to be renamed; see notes |

Every byte `explain` returns is data the kernel already computes. The
work is the resolver, the JSON Schema for the MCP tool, and the
`ops.jsonl` migration.

## The noun set

Closed at four. Each noun returns a discriminated-union payload
keyed by `kind`. Same shape over CLI `--format json` and MCP.

### 1. `kind: "action"` — `<action-verb>` (e.g., `pkg.install`)

The agent's most common question: "how does this verb work?"

Returns:

```yaml
kind: action
name: pkg.install
metadata:                           # from ActionMetadata
  description: "Install OS packages via the platform package manager."
  category: package
  version: "1.0"
  supported_platforms: [linux, darwin]
  supports_dry_run: true
  supports_become: true
  requires_sudo: true
  implements_check: true
  emits_events: [pkg.installed, pkg.skipped]
schema:                             # extracted from schema.json $defs
  required: [name]
  properties:
    name: { type: string }
    state: { enum: [present, absent, latest] }
    ...
diff_shape:                         # typed Diff payload kind (spec-66)
  kind: package
  resource: "pkg:<manager>/<name>"
  before: { version: string|null, installed: bool }
  after:  { version: string|null, installed: bool }
reverse_shape:                      # what undoes this verb
  declared: true
  produces_step:
    action: pkg.install
    state: absent
  caveat: "Reverse uninstalls; does not restore the prior version
    unless ReverseData captured one."
examples:                           # from examples/, by lookup
  - path: "examples/pkg-install-postgres.yml"
    excerpt: |
      - pkg.install:
          name: postgresql
          state: present
spec_origin:
  - spec: "spec-22-phase-2"
    file: "docs-working/streams/core/specs/spec-22-handler-abi-phase-2.md"
```

Source of truth: `actions.Get(name).Metadata()`, the `$defs.<name>`
node in `schema.json`, `Differ`/`Reverser` type assertions on the
handler, and the `examples/` directory (matched by frontmatter or
by `action:` key inside YAML).

### 2. `kind: "run"` — `<run-id>` (e.g., `r/01HV5K…`)

The operator's most common question: "what did this run actually do?"

Returns:

```yaml
kind: run
run_id: r/01HV5K7QQX8MFN9G1KZQ6X3R7B
op_id:  op/01HV5K7QPC0M3WPZ6JKHWX0R1Y    # FK to ops.jsonl
ts:     2026-05-16T22:14:09Z
config: /home/me/repo/mooncake.yml
duration_ms: 4218
totals: { changed: 3, ok: 5, skipped: 1, failed: 0 }
steps:                              # per-step, in apply order
  - index: 1
    action: pkg.install
    resource: "pkg:apt/postgresql"
    result: changed
    diff:                           # typed Diff (spec-66 payload)
      kind: package
      before: { installed: false, version: null }
      after:  { installed: true,  version: "15.3-0ubuntu0.22.04.1" }
    reversible: true
  - index: 2
    action: file.write
    resource: "file:/etc/postgresql/15/main/postgresql.conf"
    result: changed
    diff: { kind: file, lines: <unified> }
    reversible: true
  - ...
caveats:
  irreversible_step_count: 0
```

Source of truth: enriched runlog entry. The current `runlog.Entry`
carries totals but not per-step typed Diff/Reverse — `S-explain-impl`
widens the entry to include the array. The widening is additive;
existing readers continue to read the old fields.

### 3. `kind: "resource"` — `<resource-handle>` (e.g.,
`file:/etc/nginx/nginx.conf`, `pkg:apt/postgresql`, `user:alice`)

The drift-investigation question: "what has this thing been through?"

Returns:

```yaml
kind: resource
resource: file:/etc/nginx/nginx.conf
history:                            # newest first, indexed by Diff.Resource
  - run_id: r/...
    op_id:  op/...
    ts:     2026-05-16T22:14:09Z
    action: file.write
    diff:   { kind: file, ... }     # the typed Diff that touched it
    result: changed
    reversed_in: null               # or op/... if a later op reversed it
  - run_id: r/...
    ...
```

The resource handle is canonical: every handler's typed `Diff`
populates the `Resource` field with a kind-prefixed string
(`file:<abs-path>`, `pkg:<manager>/<name>`, `user:<name>`,
`service:<unit>`, etc.). The set of prefixes is the existing handler
set; no new vocabulary.

Source of truth: linear scan over `runs.jsonl` filtering by
`steps[*].resource`. Linear is fine — a year of an operator's
personal use is ~10k entries. If a fleet member produces 100k+ this
becomes the place to add an on-disk index, and the JSONL shape stays
authoritative.

### 4. `kind: "op"` — `<op-id>` (e.g., `op/01HV5K…`)

The audit / replay question: "what command was this and what did it
produce?"

Returns:

```yaml
kind: op
op_id: op/01HV5K7QPC0M3WPZ6JKHWX0R1Y
ts:    2026-05-16T22:14:05Z
command: apply
args: ["-c", "infra/postgres.yml"]
actor: "user:aatsman"
parent: null                        # or op/... — chains for replay-of-replay
config: /home/me/repo/infra/postgres.yml
runs:                               # populated as runs land for this op
  - r/01HV5K7QQX8MFN9G1KZQ6X3R7B
plan_only: false                    # true means no run was produced
```

The three failure modes from the preamble all become tractable:

- **plan-with-no-run** → entry exists in `ops.jsonl` with
  `plan_only: true` and `runs: []`.
- **rollback-reverses-N-runs** → the rollback op carries its own
  `op_id`; each reverse step in `runs.jsonl` links *back* to the
  original run via `reversed_in: op/<rollback-op-id>` on the
  original step. Three queries, three different entry points, same
  data.
- **replay-of-replay** → `parent` is a chain pointer; `op` resolves
  recursively until `parent: null` (the original apply).

Source of truth: `ops.jsonl` (new).

## CLI surface

```
mooncake explain <noun>          [--format text|json|yaml]
                                 [--examples-limit N]

# Examples
mooncake explain pkg.install                   # action
mooncake explain r/01HV5K...                   # run
mooncake explain file:/etc/nginx/nginx.conf    # resource
mooncake explain op/01HV5K...                  # op
```

Noun-kind inference is by prefix: `r/` → run, `op/` → op,
`<prefix>:<rest>` → resource, otherwise → action verb. Ambiguous
input returns a typed error listing the candidate kinds.

Text mode renders the same data the JSON mode emits, formatted for a
terminal — categories grouped, longest field aligned, Diff/Reverse
sections collapsed to a one-line summary unless `--verbose`.

## MCP tool surface

One tool, registered alongside the existing tools in
`internal/mcp/tools.go`. The tool is read-only and safe to expose to
any agent (no `--read-only` filter strips it).

Tool name: `explain`.

```json
{
  "name": "explain",
  "description":
    "Look up typed information about a mooncake noun — an action verb (e.g. pkg.install), a run id, a resource handle (file:/path, pkg:apt/name, user:name, service:unit), or an operation id. Returns the typed schema, applicable examples, the Diff and Reverse shapes, and where it came from. Read-only.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "noun": {
        "type": "string",
        "description":
          "One of: an action verb (e.g. 'pkg.install'); a run id (e.g. 'r/01HV...'); a resource handle ('<kind>:<id>'); an op id (e.g. 'op/01HV...')."
      },
      "examples_limit": {
        "type": "integer",
        "minimum": 0,
        "maximum": 10,
        "default": 3
      }
    },
    "required": ["noun"],
    "additionalProperties": false
  },
  "outputSchema": {
    "oneOf": [
      { "$ref": "#/definitions/ExplainAction"   },
      { "$ref": "#/definitions/ExplainRun"      },
      { "$ref": "#/definitions/ExplainResource" },
      { "$ref": "#/definitions/ExplainOp"       },
      { "$ref": "#/definitions/ExplainNotFound" }
    ],
    "discriminator": { "propertyName": "kind" }
  }
}
```

The `outputSchema` definitions correspond 1:1 to the four noun
payloads above plus a typed not-found:

```json
{
  "ExplainNotFound": {
    "type": "object",
    "properties": {
      "kind":       { "const": "not_found" },
      "noun":       { "type": "string" },
      "candidates": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "kind": { "enum": ["action", "run", "resource", "op"] },
            "id":   { "type": "string" }
          }
        }
      }
    },
    "required": ["kind", "noun"]
  }
}
```

`not_found` is typed because that's how the agent learns "I gave you
something that doesn't resolve to any of the four kinds." It does
*not* fall through to keyword search (Non-goal: no RAG).

## Source of truth

| Noun kind | Reads |
|---|---|
| `action`   | `schema.json` (`SchemaJSON()`), `actions.List()` + per-action `Metadata()`, `Differ`/`Reverser` type assertions, `examples/*.yml` |
| `run`      | `~/.mooncake/runs.jsonl` (with `run_id` + `op_id` + per-step Diff/Reverse — added in `S-explain-impl`) |
| `resource` | linear scan over `runs.jsonl` filtered by step `Resource` |
| `op`       | `~/.mooncake/ops.jsonl` (new in `S-explain-impl`) |

No filesystem reads outside `~/.mooncake/` and the embedded schema.
No git. No PR list. No `docs/` markdown. The agent that asks
`explain` for the *why* of a non-goal gets a `not_found` with
candidates pointing at the action verbs that *are* in the closed set
— and the agent's next call (separate `S-facts-repo-index` MCP tool)
can do the residue.

## Implementation order

Each wave is a separate PR, reviewable independently. `S-explain-impl`
covers waves 1–3 and is the implementation story; this spec
specifies the contract.

| Wave | PR | What | Effort |
|---|---|---|---|
| 1 | spec-68-1 | New `internal/explain` resolver: `kind: action` only. Reads `SchemaJSON()` + `actions.List()` + `examples/`. CLI verb `mooncake explain <action>`. Existing `internal/explain` (facts display) renamed to `internal/factsfmt`; `mooncake facts` keeps working. | S |
| 2 | spec-68-2 | `ops.jsonl` schema added; `runlog.Entry` gains `RunID`, `OpID`; per-step typed Diff/Reverse arrays appended. Existing readers ignore unknown fields. Resolver covers `kind: run` + `kind: resource` + `kind: op`. | M |
| 3 | spec-68-3 | MCP tool `explain` registered. JSON Schema for input + output as above. Output examples wired into the MCP server test suite. | S |
| 4 | spec-68-4 | (Optional, deferred to first user ask) An on-disk index over `runs.jsonl` keyed by `resource`. Linear scan remains the fallback. | M |

The renames in wave 1 are mechanical: the existing
`internal/explain` package has one exported function (`DisplayFacts`)
called from one site (`cmd/mooncake.go:411`). One-PR sed-and-import
update, no semantic change.

## DONE criteria

After waves 1–3 land:

- `mooncake explain pkg.install` returns the typed `kind: action`
  payload for every action in `actions.List()`, in three output
  formats (text / json / yaml).
- `mooncake explain r/<run-id>` returns the typed `kind: run`
  payload for every entry in `runs.jsonl` written after the migration
  shipped.
- `mooncake explain file:/etc/<path>` returns the typed
  `kind: resource` payload (possibly empty `history: []` if the
  resource has never been touched).
- `mooncake explain op/<op-id>` returns the typed `kind: op` payload,
  including `plan_only: true` for plan-mode invocations and chained
  `parent` for replays.
- The MCP tool `explain` is registered, callable from Claude Code +
  Cursor, and round-trips the `oneOf` output schema correctly.
- `~/.mooncake/ops.jsonl` exists and is written by every `apply`,
  `plan`, `rollback`, `replay`, `inspect` invocation.
- `runs.jsonl` entries written after the migration include `run_id`
  + `op_id`. Pre-migration entries continue to read; they show
  `op_id: null` in `explain` output.
- The closed noun set is honored: `mooncake explain frobnicate` and
  `mooncake explain http://example.com/` both return a typed
  `not_found` with candidate suggestions, not a keyword-search
  fallback.

## Open questions

1. **`ops.jsonl` location for a fleet member.** Personal use lands at
   `~/.mooncake/ops.jsonl`. An `agentd` running under a service
   account writes where? Probably `$XDG_STATE_HOME/mooncake/ops.jsonl`
   or a config-overridable path. Defer the resolution to
   `S-explain-impl` wave 2 — it's a deployment concern, not a schema
   concern.

2. **`run_id` shape vs existing implicit identity.** The runlog today
   identifies entries by 1-based newest-first index (`runlog.At(i)`).
   That index is brittle across compactions and unsuitable as an
   `op_id` referent. The fix is to assign a sortable ULID-shaped
   `run_id` at append time and treat the integer index as a derived
   convenience. Belongs in wave 2; flag for migration testing.

3. **What about `transaction:` as a noun?** A transaction spans
   multiple steps and (per spec-30) carries its own LIFO compensation
   plan. Today it does not have a typed handle separate from the run
   it lives in. Three options: (a) `kind: run` already covers it
   because the transaction's steps are sequenced in the run payload;
   (b) add a fifth noun kind `transaction`; (c) defer until an agent
   asks. **My recommendation: (a) for now.** A `transaction` noun
   would be the first place the closed set bends, and the cost of
   re-bending later is much lower than the cost of opening the set
   speculatively.

4. **Examples lookup heuristic.** `examples/*.yml` is the obvious
   source for the per-action `examples` field. The lookup needs a
   rule: match by `action:` key inside YAML? by filename prefix?
   by a per-file frontmatter tag? Implementation decision in wave 1;
   the spec just guarantees ≥1 example is returned per action that
   has a verb in `examples/`.

5. **MCP output-schema enforcement.** The MCP spec allows
   `outputSchema` but tooling enforcement varies. We register the
   schema regardless; the agent client decides how strictly to
   validate. The Go side validates outgoing payloads against the
   schema in tests, not at runtime.

## Pairs with

- **[`kernel.md`](../../../vision/kernel.md)** — `mooncake explain
  <resource>` is listed under "What this enables — derived, not
  claimed." This spec is the *rendering*, not a new kernel column.
- **[spec-66](./spec-66-typed-plan-diff.md)** — the per-kind Diff
  payloads `explain` returns under `diff_shape:` and per-step
  `diff:` are exactly the spec-66 shapes. `explain` does not
  redefine them.
- **[spec-67](../../agent/specs/spec-67-mooncake-pilot.md)** — the
  pilot agent already calls MCP tools; once `explain` ships, the
  pilot's system-prompt builder (`internal/pilot/prompt.go`, and the
  in-flight `prompt_schema.go` from `S-pilot-schema-injection`) can
  drop ~80% of its hand-curated action descriptions and source them
  live from `explain`. Follow-up story, not in this spec.
- **[story `S-explain-impl`](../../../vision/brainstorm/2026-05-16-stories.md#s-explain-impl)** —
  implementation, including `ops.jsonl` migration, `runlog.Entry`
  widening, MCP tool registration, tests.
- **[story `S-facts-repo-index`](../../../vision/brainstorm/2026-05-16-stories.md#s-facts-repo-index)** —
  the residue. Anything `explain` cannot answer from typed sources
  is `facts.repo_index`'s problem. Order is fixed: `explain` ships
  first, the residue ships only if a real user asks for it.

## Receipts

- `internal/config/schema.json`: 24965 lines, embedded via
  `SchemaJSON()` (`internal/config/validator.go:17`). Every action
  has a `$defs` entry; `x-category` already groups them.
- `internal/actions/handler.go:62`: `ActionMetadata` carries
  description, category, version, platform support, sudo / dryrun
  / check flags, emitted events — everything the `action` noun
  needs in one struct.
- `internal/runlog/runlog.go`: 7-field `Entry` today
  (`ts`, `config`, `changed`, `ok`, `skipped`, `failed`,
  `duration_ms`). Needs `run_id` + `op_id` + per-step typed array.
  Migration is additive — old readers keep working.
- `internal/mcp/tools.go`, `internal/mcp/server.go`: existing tool
  registration surface. One new tool entry, no infrastructure
  changes.
- `internal/explain/explain.go`: existing package, displays system
  facts (CPU/memory/GPUs/storage). Name collision only; mechanical
  rename to `internal/factsfmt` in wave 1.

## Why this lives in core (not agent)

The MCP tool is the highest-fan-out surface, but the resolver is
kernel-shaped: it consumes `schema.json`, `ActionMetadata`, the
`Differ`/`Reverser` ABIs, and the runlog. The agent stream consumes
the *rendering*. Keeping the spec in core anchors the review on the
typed sources the resolver reads, not on the agent surface that
reads it.
