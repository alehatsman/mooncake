# Plan — sharpen mooncake into a provisioning / machine-management tool

> Date: 2026-09-07. Follows the vision refocus in `934083ac`
> (`docs-working/vision/goals.md`). Written after a full read-only review of the
> CLI surface, module system, task runner, action ABI, MCP surface, and a
> hands-on run of `init` / `plan` / `validate` / `task` / `doctor`.
>
> Premise: **no backward-compat burden.** Pre-1.0, single operator, no external
> users. Every break we're going to take, we take now.

---

## 0. Status (updated 2026-09-07, end of session)

| Phase | Issue | State |
|---|---|---|
| 0 — stop the bleeding | #171–#177 | **done** (`bcfe0819`, merged `ad1b5c2a`); issues closed 2026-09-07 after a board audit found them still at todo |
| 1 — one CLI shape | #179 | **done** (`341b5f6f`, merged `aad8d117`); closed 2026-09-07 |
| 3 — module system | #181 | **core done** (`430b37c4`, `c9fc7641`); #199–#201 open, 3.6/3.7 external |
| 2 — collapse state vocabulary | #180 | **partly done** (`74cbb58a`, `ab653a64`, `7626a858`, `1d44f9d7`); fleet fan-out left, post-1.0 |
| 4 — ABI honesty | #178 | not started; re-measured at 64 actions: 27 no Diff / 26 no Permissions / 31 no Reverse / 19 none |
| 5 — agent removal | #182 | **done** (`8bd89dff`, merged `652bf9ce..`); deleted from this repo, resurrect into `mooncake-agent` from history later |
| **1.0.0-pre master plan** | **#203** | coordinates all of the above plus the cleanup children #204–#209, see §4 |

Corrections to this document, found while executing it:

- **3.4 was already fixed.** #50 landed in PR #22 / `294b076f` before this plan
  was written; `specs/modules.md:87` carried a stale `[ ] DRIFT` box. Verified
  `294b076f` is an ancestor of main.
- **`mooncake step` must not be folded into `apply --step`** (Phase 1). It has
  an external consumer — moongit's CI runner execs `mooncake step '<yaml>'` —
  and it deliberately bypasses the planner, so it is not a duplicate path.
- **3.3 ships `mod tidy`/`verify`/`list`, not `mod init`.** Reading #46, that
  is a new-project Go skeleton scaffold gated on an external module publishing,
  unrelated to lock verification.
- **Phase 2's acceptance was optimistic.** `actions list` drops 4 rows and
  gains 2, not −4: `wait.file`/`wait.command` had no observe twin, so
  `observe.file`/`observe.command` were added rather than dropping capability.

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

## 3. Triage (2026-09-07, late session) — pipedream vs engineering

A second pass over the gap list, with one test: does a solo operator with a
personal fleet hit this on a Saturday. Anything that only makes sense with a
second user, a second module author, or an enterprise hub is not built.

### Cut (closed on moongit, reopen only against a concrete user)

| Idea | Why not | Issue |
|---|---|---|
| Fleet-wide desired state + reconcile loop | Kubernetes for five machines. `fleet apply` + cron'd `drift` covers it. | (never filed) |
| Capability-scoped tokens, expiry, per-caller policy | Enterprise identity layer. The real gap is two scopes, see below. | (never filed) |
| Component provides/requires contracts, collision detection | A type system for infrastructure; the hardest part of every tool that tried. Cross-module deps stay a hard no. | (never filed) |
| Approval as a run state over MCP | Harnesses already gate tool calls. A second gate inside mooncake duplicates the harness. | (never filed) |
| Sandboxed `plan --simulate` | Check paths are cheap. Solves a problem nobody has. | (never filed) |
| Runtime `plan:` sub-plan step | Composition primitive with no consumer. | #17 closed |
| `mcp_tool` action (kernel calls MCP) | The "runtime agents live inside" product. Goes with #182 if anywhere. | #12 closed |
| SDK `Session` handle | Sugar over a surface with one consumer. | #146 closed |
| `mod init` scaffold | Needs a second module author. | #46 closed |
| Fleet-level REST hub with plan-first enforcement | agentd already serves HTTP+SSE per peer; the hub is the unvalidated ring. | #152 closed |
| `mooncake watch` hot-reload | Dev-server pattern; playbooks are run, not watched. | #21 closed |

### Build (cheap, and a real user hits it)

| Work | Why | Issue |
|---|---|---|
| ABI honesty | The README lies about half the actions. Correctness, not a feature. | #178 |
| Two agentd token scopes (read / write) | One leaked observe token is root on every peer. One afternoon. | #198 |
| `index.yml` `mooncake_version` constraint | The first shared module breaks on the first schema change. | #199 |
| `mod outdated` / `mod update` | The pin is write-once. | #200 |
| `mod verify --require-lock` | CI trusts the tag when the lock is missing. Ten lines. | #201 |
| Cancel + streaming | A run an agent cannot stop is a run it should not start. | #25, #7 |
| The agent split | Every cut idea above came from the second product still living in the tree. | #182 |

Already shipped, dropped from the wish list: durable plan with a content hash.
`plan --save` + `apply --from-plan` + `input_files_hash` + stale-plan
refusal (`internal/plan/plan.go:24`) is that story.

### Deferred (labelled `deferred` on moongit; real, not now)

- Deterministic replay (#10) — right idea, needs a bounded journal first (#176).
- Fleet drift (#2) — fan-out over per-host `drift` after #180; not a reconcile loop.
- Module discovery / `share` (#38) — after #199–#201, and after a second author.
- One event schema across CLI and agentd — do it when a concrete consumer breaks.
- Fleet plan preview (per-peer would-change) — falls out of drift.

### Decisions recorded, no build

- SSH-transport peers stay bootstrap + diagnostics only (`specs/fleet.md:72`).
  The half-support banners across exec/ps/watch/logs are the cost of that
  decision; not filing a feature to change it.
- `MODULES.md` and `internal/modules/README.md` overlap; fold one into the
  other when Phase 3 closes, not before.

---

## 4. 1.0.0-pre master plan (2026-09-07, third pass) — moongit #203

A review of the two passes above against the tree at `c2d1bc7e` found the
engine work landed and the *residue* not: Phase 0/1 issues still open on the
board, the second product still in the tree, strategy docs still pitching the
cut audiences, tracked junk at the repo root, and specs describing removed
commands. #203 is the epic that closes that gap; its definition of done is
"one product, clean tree, honest surface, no dead code, green gate,
releasable".

| Workstream | Issue | Kind |
|---|---|---|
| B — delete the second product (`internal/agent`, `sdk/`, `examples/notify`, agent-evals) | #182 (#5 closed) | **done** (`8bd89dff`, PR #46) — resurrect from history when `mooncake-agent` exists |
| C — `artifact.*` / `repo.*`: five actions built for the cut agent-developer audience, none with any ABI property | #204 | **done** (`652bf9ce`, PR #48) |
| D — tree hygiene: `executor_coverage.out`, `github-actions.yml`, `test-repo-ops.yml`, `wait-for-clean.sh`, `mooncake_codes.txt`, `install`, root `llms.txt`, `scripts/migrate`, two orphan scripts, `.codecov.yml`/`.gitleaks.toml` (found while verifying — orphaned by the same cut), `.github/workflows/*.disabled` (kept `release.yml` only, fixed its stale Go pin + `master`→`main` reference) | #205 | **done** (`ccc7bf1e`, PR #49) |
| E — retire `VISION.md`, `BOTTLENECK.md`, brainstorm/, positioning.md; fix README/AGENT.md/examples/ROADMAP/CONTRIBUTING/RELEASING dead references | #206 | **done** (`998b79a1`, PR #51) |
| F — specs still describe `mooncake query` / `mooncake facts` | #207 | **done** (`16eecae6`, PR #50) |
| G — 29 `deadcode` hits, 14 action packages with v1 names, 247 `//nolint` | #208 | **PR #52 up, moongit CI running** |
| H — ABI honesty | #178 | engineering |
| I / J — modules hardening, agentd token scopes | #199–#201, #198 | engineering |
| K — open bugs | #188 #189 #191 #196 #202 | engineering |
| L — release readiness: tag → goreleaser → `install.sh`, `latest` moving tag, CHANGELOG | #209 | last |

Out of scope for the tag: Phase 2 remainder (#180), deferred (#2 #10 #38),
cancel/stream (#25 #7), the structural-debt files.

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
have none of the three.** (Re-measured 2026-09-07 after the `wait.*` collapse:
64 actions, 27 / 26 / 31, 19 with none — current numbers live on #178.) The gaps are on load-bearing actions —
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

Revised after the §3 triage (Phases 0, 1 and the core of 3 are done):

1. #178 ABI honesty.
2. The four cheap ones: #198 token scopes, #199 version constraint,
   #200 `mod update`, #201 `--require-lock`.
3. #25 cancel, #7 streaming.
4. #182 the split.
5. Phase 2 (state collapse) and the deferred list, in that order, only when
   something concrete asks for them.
