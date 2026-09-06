# Plan — sharpen mooncake into a provisioning / machine-management tool

> Date: 2026-09-07. Follows the vision refocus in `934083ac`
> (`docs-working/vision/goals.md`). Written after a full read-only review of the
> CLI surface, module system, task runner, action ABI, MCP surface, and a
> hands-on run of `init` / `plan` / `validate` / `task` / `doctor`.
>
> Premise: **no backward-compat burden.** Pre-1.0, single operator, no external
> users. Every break we're going to take, we take now.

---

## 1. Where we actually are

| Area | Prod LOC | State |
|---|---:|---|
| Action kernel (`internal/actions`) | 43,707 | Real. 66 actions, idempotent, plan/diff |
| config + plan + executor + apply | 14,729 | Real. transactions, try/catch, heal, on_change |
| fleet + agentd | 18,968 | Real. Daily use, P2P, 22 subcommands |
| kernel support (facts/template/security/…) | 12,982 | Real |
| docs + schema tooling | 4,758 | Maintainer-only, ships in the user binary |
| SDK facade (`sdk/`) | 4,024 | **One consumer: `examples/notify/webhook.go`** |
| LLM agent loop (`internal/agent`) | 4,314 | Real, but a second product; consumer is moongit |
| MCP (`internal/mcp`) | 1,939 | 19 tools, 4 of them dead or redundant |
| modules (`internal/modules` + `cmd/mod`) | 804 | ~20% of its own design doc |
| presets (`internal/presets`) | 491 | Retired concept, still load-bearing code |

Engine: good. Product shape around it: not. The vision was cut back to
solo-dev provisioning + personal fleet; the binary still carries the surface of
the three-audience ambition. And the first five minutes of a new user's life are
broken.

## 2. Decisions taken (2026-09-07)

1. **`internal/agent` splits to its own repo.** `mooncake-agent` becomes a
   separate binary importing mooncake as a library. Reopen moongit #110.
2. **One noun: "module".** `internal/presets` → component/module naming
   unified, one `use:` resolution path, "preset" gone from code, doctor, MCP,
   docs.
3. **Full CLI regroup, no deprecation aliases.**

---

## Phase 0 — stop the bleeding

No spec needed. All of it is a broken promise already in the tree.

| # | Fix | Evidence |
|---|---|---|
| 0.1 | `init --template server` emits `become: true`, a field `validate` rejects | `internal/scaffold` templates; spec-21 collapsed become→`as_user` |
| 0.2 | All four templates end with `echo "Browse: ./presets/…"` — dir doesn't exist | every `init` template |
| 0.3 | `mooncake.vars.yml` is scaffolded + cited in errors, never auto-loaded | `cmd/kernel/unresolved.go:74` |
| 0.4 | ~20 flags render garbled help (backticks in `Usage:` are urfave's value placeholder) | `cmd/kernel/apply.go:69`, `cmd/kernel/plan.go:90`, `cmd/fleet/status.go:28` + ~17 more |
| 0.5 | Unknown-field error points at archived `docs-next/`; no did-you-mean | `internal/config/reader.go:355` |
| 0.6 | `doctor` reports a "Preset search paths" section and `used by: steps with become: true` | `internal/doctor/registry.go`, `checks_*.go` |
| 0.7 | `~/.mooncake/runs.jsonl` unbounded (9.0 MB / 24,903 lines here), `ops.jsonl` 5.4 MB, no rotation or `gc` | `internal/runlog`, `internal/statedir` |
| 0.8 | Delete `init --template agent-sandbox` (sells the cut audience) | `internal/scaffold` |
| 0.9 | README says "40+ typed actions"; real count is 66, of which 42 have `Reverse` | `README.md:101` |

Exit: a fresh `mooncake init && mooncake plan && mooncake apply` works on all
four templates with no dead references and no garbled help.

---

## Phase 1 — one shape for the CLI

**Spec first.** 26 flat top-level commands, no tiers, six execution entry points.

### Target shape

```
run      apply · plan · task
inspect  state · history · explain · actions · drift
fleet    (22 subcommands, unchanged for now)
manage   mod · tool · vault · cron · doctor · init
daemon   agentd · runs · mcp
dev      docs · schema · selfbuild        (hidden from top-level help)
```

### Concrete moves

- **`apply --dry-run` deleted.** Its own help says "sugar for `mooncake plan`".
  One preview path.
- **`facts` + `snapshot` + `metrics` collapse into `mooncake state`**, with
  `--format json` and selector flags. Three commands, one concept.
- **`mooncake query` deleted.** It reads a JSON/YAML file by dotted path. That
  is `jq`/`yq`, and `read.json`/`read.yaml` already cover the in-playbook case.
- **`docs` / `schema` / `selfbuild` move behind `mooncake dev`**, hidden.
  Maintainer tooling should not be in a user's `--help`.
- **`--format json` on every read command.** Currently missing on
  `actions list`, `drift inspect`, `tool list`, `runs list`, `mod cache list`,
  `cron list`, `vault list` — an agent has to screen-scrape all seven.
- **`step` folded into `apply --step '<yaml>'`.** One inline-execution path.

Exit: `mooncake --help` fits on a screen; every read command emits JSON.

---

## Phase 2 — collapse the state vocabulary

Four separate implementations of "read machine state" exist today:

1. `mooncake facts` / `snapshot` / `metrics`
2. actions `observe.{cpu,memory,disk,gpu,port,process,http,service,logs}` (9)
3. actions `wait.{port,http,file,command}` (4) — a poll loop around #2
4. `fleet observe` (8 subcommands) + `fleet top`

### Target

- `observe.*` is the single typed family.
- `wait.X` dies; `observe.X` grows a `wait:` modifier
  (`wait: {for: 30s, until: open}`). Removes 4 actions and a duplicated
  poll/backoff implementation.
- `fleet observe` becomes fan-out over the same handlers, not a parallel
  implementation (`cmd/fleet/observe.go`, 466 LOC).
- `mooncake state` (Phase 1) reads through the same handlers.

Exit: one place implements "is port 5432 open", reachable from a playbook, the
CLI, and the fleet.

---

## Phase 3 — finish the module system

This is what makes the provisioning story *shareable*, and it is the last 5% of
`goals.md §1`. Currently shipped: fetch, cache, `index.yml`, resolve,
`mod add`, `mod cache`. Everything else in
`docs-working/vision/sharing_and_modules.md` is unbuilt.

| # | Work |
|---|---|
| 3.1 | Lockfile — **name it `mooncake.modules.lock`**; `mooncake.lock` is already claimed by the `tool` action (`internal/lockfile/lockfile.go:24`) |
| 3.2 | `h1:sha256` checksums over the resolved module tree; verify on load |
| 3.3 | `mod tidy` · `mod list` · `mod verify` · `mod init` (moongit #46) |
| 3.4 | Fix DRIFT #50 — a module component's relative `file.copy` `dest:` with a subdir resolves against the module cache dir, not the invocation dir (`specs/modules.md:87`) |
| 3.5 | Retire "preset" — `internal/presets` renamed, doctor section removed, `list_presets` MCP tool deleted, `parameters:` alias dropped |
| 3.6 | Seed `github.com/mooncake-modules/*` with the modules actually in daily use |
| 3.7 | `mooncake share` (moongit #38) once 3.1–3.3 land |

Exit: `mooncake mod add … && mooncake mod verify` round-trips with a committed
lockfile, and the word "preset" appears nowhere in the binary.

---

## Phase 4 — ABI honesty

Of 66 actions: **31 have no `Diff`, 30 no `Permissions`, 24 no `Reverse`, 23
have none of the three.** The gaps are on load-bearing actions —
`container`, `container.image`, `tool`, all four `windows.*`, `repo.*`,
`artifact.*`.

Two acceptable outcomes, per action:

- **Implement** `Diff` + `Permissions` — priority: `container`,
  `container.image`, `tool`, `windows.registry`.
- **Declare untyped** — `shell`, `cmd`, `process` are escape hatches by design.
  Mark them explicitly in `actions list` and in the docs, and stop implying the
  four-property ABI is universal.

Also: `observe.*` actions currently report `Reverse: yes`. A read-only observer
should not claim reversibility — audit that.

Exit: `actions list` tells the truth, and the README's ABI claim matches it.

---

## Phase 5 — split the second product

Per decision (2), `internal/agent` (4,314 LOC, 3 LLM provider clients,
prompt schema, confirm gate, iteration store, control channel) moves to its own
repo as `mooncake-agent`, importing mooncake as a library.

- Reopen moongit #110.
- This is the SDK's justification: `sdk/` (4,024 LOC) currently has one
  consumer, `examples/notify/webhook.go`. After the split it has a real one.
  If the split stalls, the SDK is a deletion candidate.
- `mooncake agent` leaves the CLI. `specs/agent-loop.md` moves with the code.

---

## Cut list

- `init --template agent-sandbox`
- MCP tools: `list_presets` (dead), `read_file` / `grep_files` / `glob_files`
  (every agent harness has these natively; pure context tax)
- `mooncake query`
- the `wait.*` action family (4)
- `apply --dry-run`, `mooncake step` (folded)
- `sdk/` — conditional on Phase 5 stalling

## MCP / AX backlog (already filed on moongit)

#169 execution ID + attach · #7 stream `run_plan` events · #8 `cancel_plan` ·
#9 `diff_plan` · #10 `list_runs`/`get_run`/`replay_run`. Deterministic replay is
the last leg of the unfair-advantage line in `goals.md:147`.

## Structural debt (not scheduled; watch it)

`internal/config/config.go` 2,617 · `internal/executor/executor.go` 1,968 ·
`internal/plan/planner.go` 1,619 · `cmd/fleet` 5,559 LOC of logic in the CLI
layer (instability 0.86). Per `feedback_no_refactor_loop`: one refactor at a
time, on request — not a program.

---

## Order of work

Phase 0 first and alone — it is all broken promises, and none of it needs a
spec. Then 1 → 3 → 2 → 4, with 5 running in parallel whenever moongit can take
the migration. Phase 3 before 2 because module distribution is the validated
user-facing gap; the observe/wait collapse is internal hygiene.
