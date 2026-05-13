# Epic: Read & Report — Closing the Observation Gap

> Status: brainstorming / proposed epic. Iterate here before promoting buckets
> to formal specs under `docs-working/specs/action-surface/`.

**Stream:** 1 — Action Surface (see [streams.md](../streams.md))
**Design principles:** [action-design-principles.md](../action-design-principles.md)

---

## The thesis

> **Mooncake has rich vocabulary for changing a system and an impoverished
> vocabulary for looking at one.** Today the answer to "what's in this
> file?" is `shell: cat foo.json | jq .key`. That bypasses every guarantee
> mooncake exists to provide: no typed result, no structured event, no plan
> preview, no agent surface, no secret redaction.

This epic closes that gap with **four** changes — two new actions, one
framework feature, one CLI verb. No new namespaces, no query DSL, no
sugar layer. Reality-checked from a larger draft that grew its own
ecosystem; this is the version that earns its keep.

---

## Why now

1. **Spec-25 builds the write side** (`text.patch.{json,yaml,ini}`).
   Without a matching read side, every agent loop still falls back to
   `shell:` for "now read it back to verify". That defeats the
   audit-by-default story from `VISION.md` §2.
2. **The agent-developer wedge (VISION §4.2)** is "Docker for AI agents".
   A sandboxed agent that can only mutate through mooncake also needs to
   *observe* through mooncake. Otherwise we force shell access back in.
3. **The agent-efficiency epic shipped 15 specs** for compact, observable
   *mutation* runs. The next agent UX win is *observation* — letting the
   agent surface a finding into the run log without a shell pipeline.

---

## Today's gap, honestly

| Need | Today | Verdict |
|---|---|---|
| Parse a JSON / YAML file in a plan | `shell: jq …` / `shell: yq …` | Real gap |
| Capture step output for downstream steps | possible via vars but awkward | Framework gap (`register:`) |
| Show a labeled value or small map in the log | `log: { msg: "..." }` free-text only | Real gap |
| One-shot "what does this file say at this path" | shell out | CLI gap |
| Read CSV / TOML / INI / .env / HTTP in a plan | shell out | **Not a real gap.** Punt. |
| jq-style power querying | `shell: jq` | **Not a core gap.** Tier-2 plugin if ever. |
| Rendering tables / trees / diffs in the log | not possible | **Niche.** Snapshot covers trees; spec-35 covers diffs. |

The four-deliverable scope below targets the three "real gap" rows.

---

## The four deliverables

### D1 — `read.json` and `read.yaml`

Two new tier-1 actions. Same shape, two formats.

```yaml
- read.json:
    path: ./package.json
    query: version       # optional; dotted path (a.b.c, a[0])
    redact:              # optional; regex list applied to value before publishing
      - "token|secret"
  register: pkg_version
```

Semantics:
- Read the file, parse, optionally extract `query` subtree, optionally
  redact, publish as the step's typed output.
- Bounded: default `max_bytes: 1MB`. Larger files fail with a clear
  error suggesting explicit limits.
- Read-only by contract: `idempotent: true`, `reversible: true` (no-op),
  no `FilesystemWrite` permissions. Always reports `ok` (never `changed`).
- Typed output in JSON Schema → MCP server surfaces real return shapes.

The `query:` field stays tiny — same dotted-path subset as
`text.patch.json` (spec-25). If a user wants jq, they run jq via `shell:`
and we don't apologize.

`read.yaml` is byte-for-byte the same surface against YAML input
(`gopkg.in/yaml.v3`).

**What we are *not* shipping:** `read.toml`, `read.ini`, `read.env`,
`read.csv`, `read.http`, `read.command`, `read.lines`. Each can be added
later if real demand appears. Today none of them is asked for in
practice.

### D2 — `register:` step field

Framework change: any step gets a top-level `register: <name>` field
that publishes its primary output to a named variable. Drops the
"primary output → vars" plumbing from a two-step chore to one line.

```yaml
- read.json:
    path: ./package.json
    query: version
  register: app_version

- log:
    msg: "deploying app v{{ app_version }}"
```

Works on every step that has a typed output, not just `read.*`. So `cmd`,
`shell`, `repo.search`, `repo.tree`, `tool` all get capture-into-vars for
free.

Resolves the "captured value flow" need without adding a separate `vars.set`
ceremony to every reader. The name is borrowed from Ansible deliberately —
it's the term the audience already knows.

### D3 — `log:` extended with structured data

Today `log:` takes `msg:` (free text). Extend it to accept an optional
`data:` (any value) and `format:` (`kv` | `json`).

```yaml
- log:
    title: "Service config"           # optional, used as header by kv
    data:
      port: "{{ config.service.port }}"
      tls:  "{{ config.service.tls }}"
      logs: "{{ config.service.log_level }}"
    format: kv
```

Renders in the terminal as a multi-line aligned KV block. For
`format: json`, pretty-prints with 2-space indent. Token-bounded by
default (`budget: 400` tokens) so agents don't burn context on huge
payloads — gracefully truncates with `… (truncated)`.

Backwards-compatible — pre-existing `log: { msg: ... }` keeps working
exactly as before; new fields are additive.

The JSONL event design (does this go on `step_end.outputs` or get a new
`event: observe` line in the agent stream?) is left for the spec to
decide. The epic does not pre-commit.

### D4 — `mooncake query` CLI verb (+ MCP tool)

One-shot inspection outside `mooncake apply`. The honest answer to "an
agent wants to look at a file" — no plan, no apply, just a query.

```
mooncake query ./package.json version
mooncake query ./config.yml service.port
mooncake query ./mooncake.lock 'tool[0].name'
```

- Auto-detects format from extension (`.json` / `.yml` / `.yaml`),
  override via `--as json|yaml`.
- Same parsing path and same `path:` subset as `read.json` / `read.yaml`
  — implementation shared.
- Output to stdout: scalar values raw, structured values as compact JSON
  (or pretty with `--pretty`).
- Exit code: 0 on found, 1 on path-miss, 2 on parse error.

The MCP server (spec-10) exposes this as a tool (`query_file`) so
Claude/Cursor agents can call it directly without subprocess wrapping.

This is what most of the agent demand actually is: not a plan with a
single read step, but a quick "let me see what's there" lookup. Splitting
it into a CLI verb keeps the action surface small.

---

## What this is *not*

- **A query language.** No jq, no JMESPath, no Rego, no `query.*`
  namespace, no filter/map/regex actions. The path subset matches
  spec-25 and stops there. If jq's surface is genuinely needed, it
  ships as a tier-2 plugin (`query.jq` per spec-31), not core.
- **A reporting / dashboard layer.** `log:` writes to the execution log.
  Pretty plots, web dashboards, fleet rollups belong to Stream 3 (Fleet
  & Cluster Management).
- **A back door for shell.** No `read.command`. If you need to run a
  command and capture its output, that's `cmd: + register:`. The audit
  log treats it as a mutation step, which is the honest accounting.
- **Format completionism.** No `read.toml`, `read.ini`, `read.env`,
  `read.csv`, `read.http`, `read.lines` in v1. Each can land later on
  demonstrated demand. Speculative breadth is the trap; depth in JSON +
  YAML is the win.

---

## Cross-cutting rules

(Per [action-design-principles.md](../action-design-principles.md).)

1. **Read-only by contract.** `read.json` and `read.yaml` declare
   `idempotent: true`, `reversible: true` (no-op), no `FilesystemWrite`.
   Audit log shows them as `ok` always.
2. **Plan mode = execute mode.** Reads happen in plan mode too (cheap);
   the published value is suppressed from registering into vars so plan
   stays side-effect-free at the vars-context level.
3. **Secret redaction is mandatory.** Both readers respect
   `internal/security/redact.go`, plus the per-step `redact:` regex
   list for files known to contain secrets.
4. **Bounded output.** Default `max_bytes: 1MB`. Larger inputs surface a
   structured error with a hint. Refusing to load a 5GB JSON file is a
   feature.
5. **Typed outputs in JSON Schema.** Both readers' outputs are typed in
   their action schema. MCP server (spec-10) and any agent SDK get
   typed return values, not bags of bytes.

---

## Sequencing

One spec per deliverable. D1 and D2 are interdependent — `read.json` is
much less useful without `register:`, and `register:` wants a real
consumer to land alongside it. Ship them in the same PR or back-to-back.

| Spec | Deliverable | Effort | Notes |
|---|---|---|---|
| spec-R1 | D1 `read.json` + `read.yaml` | S | Same handler shape; share parser plumbing. |
| spec-R2 | D2 `register:` step field | S | Framework change; planner + executor + schema. |
| spec-R3 | D3 `log:` extended with `data:` + `format:` | S | Additive; keep backcompat with existing `log: msg:`. |
| spec-R4 | D4 `mooncake query` CLI + MCP tool | S | Reuses the parser from R1. |

All four are small. The whole epic is ~2 weeks of focused work.

**Recommended order:** R2 → R1 → R3 → R4. `register:` lands first so R1
has a clean publishing target. R1 next so R3 has something real to log.
R4 last because it's a thin CLI wrapper over machinery R1 already built.

---

## Open questions

1. **Agent JSONL event design (D3).** New `event: observe` line vs
   payload on `step_end`? Deferred to the R3 spec. Don't pre-commit.
2. **`register:` collisions.** What happens if two steps `register:` the
   same name? Probably: last-write-wins, with a warning on the second
   write. Confirm in R2.
3. **`read.*` in plan mode.** Does the read actually execute during
   `mooncake plan`? Default yes — the read is cheap and side-effect-free,
   and showing the value in plan output is valuable. Confirm in R1.
4. **`query` CLI exit codes (D4).** Path-miss → 1 vs 0-with-empty-stdout?
   Agents prefer non-zero on miss; shell pipelines sometimes prefer 0.
   Pick non-zero (1) for safety — agents are the primary audience.
5. **`max_bytes` default.** 1MB is a guess. May need to be lower for
   agents (token cost) or higher for `package-lock.json`-shaped files
   (which are routinely >5MB). Confirm in R1.

---

## How this lands in the broader product

**For solo developers (VISION §4.1):** `mooncake query ./package.json version`
replaces `jq` in the dotfiles bootstrap script. `log: data:` makes the
end-of-run summary a clean KV block instead of cobbled-together prose.

**For AI-agent developers (VISION §4.2):** the MCP `query_file` tool
plus typed `read.*` returns mean an agent can observe a system without
ever calling `shell`. Combined with the existing typed mutation surface,
that's the full "agent has no shell access" story Stream 2 has been
building toward.

**For platform teams (VISION §4.3):** mostly out of scope for this
epic. The hub layer cares about fleet observations, not per-step in-plan
reads. This epic is foundation work that lets fleet plans publish
typed observations later.

---

*This doc is meant to be edited. Cross out what's wrong, expand what's
underdeveloped, fork sections into their own specs under
`docs-working/specs/action-surface/` when they're ready to move from
"idea" to "plan."*
