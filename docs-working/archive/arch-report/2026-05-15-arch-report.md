# Mooncake — Architecture Report

**Date:** 2026-05-15
**Codebase:** `github.com/alehatsman/mooncake` @ master
**Scope:** Structural review — what the tree looks like today, what's
load-bearing, what's drifting, what to do about it.
**Grounded in:** `docs-working/ARCH_SNAPSHOT.md` (auto-generated package
metrics + gocyclo + goda cut), `docs-working/vision/`, `streams/`
READMEs, direct reads of the Handler ABI and the largest packages.

This is a fresh pass — not an incremental update to prior reviews.
Read top-to-bottom; cross-reference, not authority.

---

## 0. Executive summary

The tree is **structurally healthy at the kernel** and **silently
accreting at the edges**. The five-system mental model (Actions /
Presets / Planner / Executor / Facts) maps 1:1 to packages with the
expected stability properties: low-instability foundation packages
(`config`, `facts`, `events`, `security`, `expression`, `metrics`,
`registry`, `runlog`, `errors`, `winutil`, `fleet/transport` —
all at instability 0.00–0.06), a clean handler fan-out through
`internal/register` (single-importer, 65 action sub-packages), and an
opt-in extended ABI (`Differ` / `Reverser` / `Coster` / `Permitter`)
that grew without breaking the minimal `Handler` contract.

Four structural pressure points show up in the metrics:

1. **`cmd/` is 10,547 LOC** of mostly business logic (largest package
   by a wide margin), with three files above 800 LOC and the top
   cyclomatic functions concentrated there (`fleetApplyAction` = 49,
   `run` = 33). It's the only `instability=1.00` package and it's
   carrying orchestration that belongs in `internal/`.
2. **`internal/executor` has 36 files** (mostly tests, but 12
   non-test sources) covering eight runtime concerns in one
   package: dispatch, dry-run, plan-inspect, preflight, transaction,
   try/catch, secret resolution, scope, tag-check, result. 67 packages
   import it — every concern that lands here ripples.
3. **`internal/config/config.go` is 1866 LOC of struct definitions**
   (the `Step` union), now the single largest non-test file. Every
   new action field widens it. There's no plugin escape valve —
   that's deliberate (non-goal #2) but the file's growth rate is the
   tax.
4. **The handler set has grown to 65 packages**. Mean LOC per handler
   is ~600 with a long right tail (`service` 1466, `file` 2044,
   `tool` 1676, `package` 1216). Most are small enough to read in one
   sitting; a handful are themselves becoming mini-services.

Nothing here is critical. Three of the four are predictable
consequences of the project doing what it set out to do — ship more
typed actions, more CLI commands, more enterprise-shaped features at
the edges. The recommendation is to **carve seams now, before the
next big stream lands**, not to refactor existing code for its own
sake.

---

## 1. What the tree looks like (the actual model)

```
                ┌─────────────────────────────────────────────┐
                │           cmd/   (10,547 LOC)                │
                │   CLI surface + orchestration mixed         │
                │   instability=1.00, eff=36, aff=0           │
                └────────────┬────────────────────────────────┘
                             │
        ┌────────────────────┴────────────────────┐
        │                                          │
┌───────▼────────┐                       ┌────────▼────────┐
│ internal/      │                       │ internal/agentd │
│ executor       │                       │   (2,940 LOC)   │
│  (3,270 LOC)   │                       │   HTTP+SSE      │
│ aff=67, inst   │                       │ peer transport  │
│ =0.18          │                       │ aff=2, inst=0.78│
└───────┬────────┘                       └────────┬────────┘
        │                                         │
   ┌────┴──────┐                          ┌───────┴───────┐
   │ uses      │                          │ wires Run     │
   ▼           ▼                          ▼               ▼
┌──────────┐ ┌──────────┐            ┌──────────┐  ┌───────────┐
│ plan     │ │ effects  │            │ fleet/   │  │  mcp      │
│ (1,619)  │ │  (932)   │            │ transport│  │  (923)    │
│ inst=0.60│ │ inst=0.50│            │ (1,340)  │  │ inst=0.82 │
└────┬─────┘ └──────────┘            │ inst=0.00│  └───────────┘
     │                               └──────────┘
     ▼
┌──────────────────────────────────────────────────────────────┐
│             internal/register (1 importer)                   │
│  Imports all 65 internal/actions/* sub-packages              │
│  Goda cut: removing this prunes 64 packages, 1.0 MB,         │
│  30,322 LOC — the single biggest "leverage" package          │
└──────────────────────────────────────────────────────────────┘
                              │
            ┌─────────────────┼─────────────────┐
            ▼                 ▼                 ▼
     ┌────────────┐   ┌────────────┐   ┌────────────┐
     │ actions/   │   │ actions/   │   │ actions/   │
     │  file      │   │  shell     │   │ ... × 63   │
     │ (2,044)    │   │ (819)      │   │            │
     │ inst=0.28  │   │ inst=0.83  │   │ inst≈0.80+ │
     └────────────┘   └────────────┘   └────────────┘
                              │
              ┌───────────────┴───────────────┐
              ▼                               ▼
       ┌────────────────┐            ┌────────────────┐
       │ STABLE CORE    │            │ STABLE LEAVES  │
       │ (foundation)   │            │ (no out edges) │
       ├────────────────┤            ├────────────────┤
       │ config 0.01    │            │ facts 0.00     │
       │ actions 0.06   │            │ events 0.00    │
       │ template 0.12  │            │ security 0.00  │
       │ executor 0.18  │            │ expression 0.00│
       │ file/* 0.28    │            │ metrics 0.00   │
       │ logger 0.30    │            │ winutil 0.00   │
       │ fleet 0.33     │            │ registry 0.00  │
       │ fleet/discov.40│            │ runlog 0.00    │
       └────────────────┘            │ fleet/tx 0.00  │
                                     └────────────────┘
```

The shape is correct. Everything below `cmd/` and `agentd/` is either
properly stable (foundation) or properly unstable (handler leaves).
There are no inverted-instability surprises — no `aff=20, eff=0` "god
data" sitting in the wrong place.

What `instability=0.00` packages have in common (and what makes them
load-bearing):

| Pkg | LOC | aff | Role |
|---|---:|---:|---|
| `events` | 631 | 40 | The wire format every emit/consume crosses |
| `security` | 710 | 13 | `become`/sudo policy boundary |
| `facts` | 1864 | 11 | OS / hardware inventory consumed by `when:`/`changed_when:` |
| `metrics` | 1276 | 6 | Live daemon metrics → template scope |
| `expression` | 708 | 6 | `when:` / `failed_when:` / `changed_when:` predicate engine |
| `winutil` | 1205 | 3 | Windows-specific syscalls (scheduled tasks, firewall) |
| `fleet/transport` | 1340 | 5 | Peer transport (HTTP, SSH) |
| `runlog` | — | — | Append-only audit log |
| `registry` | 820 | 1 | (a misnomer — this is the *preset* registry, not the action one) |

`events` with 40 afferent and 0 efferent is the strongest single
architecture signal in the tree: a *narrow data shape with broad
consumption* is exactly the thing that lets the kernel grow without
the consumers having to coordinate.

---

## 2. What's working (signed, dated, evidence-based)

### 2.1 The Handler ABI is genuinely extensible

`internal/actions/handler_abi.go` adds **four opt-in sub-interfaces**
(`Differ` / `Reverser` / `Coster` / `Permitter`) on top of the
required `Handler` (`Metadata` / `Validate` / `Run`). Each comes with
a `Resolve*` default-helper so old handlers don't break.

Concrete evidence the ABI is being used as intended:

- **`Reverser` count is growing organically.** The `reverse-capture
  v4` merge (just before this report) added real `Reverse()` to
  `os.ssh_key`, `os.mount`, `pkg.repo` — the third such batch. The
  pattern (capture pre-state into `result.ReverseData` in apply mode,
  build inverse `Step` from that data) is now in 8+ handlers.
- **`Differ` is wired through to `mooncake plan --diff`.** Test:
  `internal/executor/inspect_test.go::TestInspectPlan_RoutesDiff`.
- **`Permitter` drives the `--dry-run --permissions` output** without
  any handler being forced to declare permissions if it doesn't have
  them (default-resolver returns `PermissionSet{}`).

This is the textbook "evolve the contract without breaking existing
implementors" pattern, and it's holding.

### 2.2 Plan / Execute separation is a real boundary

`internal/plan` is at instability 0.60 (9 efferent, 6 afferent) and
`internal/executor` is at 0.18 (15 efferent, 67 afferent). The
asymmetry is exactly what you want: planning is upstream of execution,
not a peer.

The plan-mode dispatch contract (`dispatchRunner` calls
`handler.Run(ctx with Mode=ModePlan)`) means **every action's
prediction logic is in the same function as its apply logic**. There
is no parallel "checker" tree to drift. This was the architectural
bet behind spec-16; the bet has paid off — `mooncake plan` is now
checkable for every typed action without a separate code path.

### 2.3 The non-goals are load-bearing in the tree

`docs-working/vision/non_goals.md` declares seven explicit refusals
(no DSL evolution, no plugin marketplace, no `loop_control`, no Git-
as-database, no CRD/operator, no ACID claims, no enterprise hub in
v1). For each, you can point at a *thing that didn't get built* and
see the saving:

| Non-goal | Cost it saved |
|---|---|
| No DSL evolution | `internal/template` is **504 LOC** at instability 0.12. There is no template-AST package, no custom-filter registry, no `loop_control`. Compare to Ansible's Jinja layer. |
| No plugin marketplace | `internal/register` is **64 LOC**. There is no dlopen, no plugin manifest, no plugin ABI versioning. |
| `transaction:` is SAGA, not ACID | `internal/executor/transaction.go` is small enough to read in one sitting (planner-rejected nested transactions, LIFO rollback). No journal, no two-phase commit, no recovery state machine. |
| No enterprise hub | `internal/fleet` is **3,303 LOC**. There is no central controller, no per-host CRD reconciliation, no admission webhook. |

This is the rare case where the strategy doc and the code agree.

### 2.4 Architecture has its own feedback loop

`scripts/arch-snapshot.sh` regenerates `ARCH_SNAPSHOT.md` with package
metrics, gocyclo hotspots, internal import edges, and goda's "cut"
analysis (what removing a package would prune). This was added
2026-04 and it's how *this report* exists. Most projects don't have a
structural feedback loop. The fact that yours does means review can
be repeated mechanically without re-reading every file.

### 2.5 The post-2026-05-15 bug-fix sweep removed a silent-failure class

Seven `bug`-labeled issues closed in 24 hours (#16, #17, #20, #21,
#23, #26, #27, #28, #29 + the MT-series). All but two were members
of the same family: **the CLI/handler completed without surfacing a
mismatch between declared intent and actual outcome**. The fixes
followed a consistent pattern: detect the divergence, return an
error, recap line goes red, exit code goes non-zero.

The pattern is now codified well enough that #28 (`schema validate`
silent drift) and #29 (`runs` CLI silent exit 0) were caught and
fixed in single PRs. **Silent success is the bug pattern this
codebase is now organised against** — that's a sign the team's mental
model is reaching consensus.

---

## 3. Structural pressure points

### 3.1 Pressure: `cmd/` carries application services it shouldn't

**Evidence:**

| File | LOC | gocyclo top function |
|---|---:|---|
| `cmd/presets.go` | 1,516 | `collectParameters` = 29; `executePresetInstall` = 28 |
| `cmd/mooncake.go` | 1,456 | `run` = 33; `formatPlanText` = 26 |
| `cmd/fleet.go` | 897 | `fleetApplyAction` = 49 |
| `cmd/fleet_doctor.go` | 557 | — |
| `cmd/fleet_observe.go` | 522 | — |
| `cmd/runs.go` | 378 | `streamRunEvents` = 25 |

`fleetApplyAction` at cyclomatic complexity **49** is the deepest
business-logic function in the project. Reading it: it does peer
filtering, plan-snapshot upload, ordered phase rollout, per-peer SSE
fan-in, recap aggregation, and exit-code computation — about six
responsibilities that would naturally split into a
`fleet.Orchestrator` struct with one method per phase.

`run` (cmd/mooncake.go:236) at gocyclo **33** is the top-level
`apply` dispatcher. It's doing config resolution, vars layering, tag
filtering, plan building, plan-or-execute selection, artifacts
writing, and run audit — none of which is "CLI parse → call → render."

**Why it matters now (not later):**

- Today `cmd` is unimportable (`instability=1.00`, no internal
  package imports it). If anyone ever wants a second frontend — a
  TUI, an MCP server entry that doesn't bounce through HTTP, an SDK
  call — the answer is "no" because the orchestration lives in
  `cmd/`.
- The MCP server already exists (`internal/mcp`, 923 LOC) and it
  *re-implements* parts of the apply path because it can't import
  the cmd-level orchestration. That's the duplication symptom.
- Every CLI flag-handling bug closed in the recent sweep (#28 strict-
  default mismatch, #29 silent exit 0) had a "fix lives in cmd"
  shape — which is fine for flag plumbing but worrying for the
  business logic underneath.

**Recommendation:**

Extract one orchestrator per top-level subcommand into `internal/`:

| `cmd` function | Extract to |
|---|---|
| `fleetApplyAction` | `internal/fleet/orchestrator.{Plan,Apply}` |
| `run` (apply path) | `internal/apply/runner.Run` |
| `executePresetInstall` | `internal/presets/installer.Install` |
| `streamRunEvents` | `internal/agentd/client.StreamEvents` |
| `fleetUpgradeAction` | `internal/fleet/upgrade.Orchestrator.Run` |

Each extraction is **localised** — moves the body, keeps the cmd file
as flag-parse → orchestrator-construct → orchestrator-call → render.
None of this requires interface changes. Recommend starting with
`fleet.Orchestrator` (highest gocyclo, clearest seam) as a
proof-of-pattern, then propagating.

**Cost of not doing it:** the next stream that wants to call the
fleet-apply path from somewhere other than the CLI (MCP, SDK, agent
loop) duplicates 897 LOC.

### 3.2 Pressure: `internal/executor` is being treated as a layer

**Evidence:**

`internal/executor` has 36 files (12 non-test). The non-test files
each own a *distinct runtime concern*:

```
executor.go     (1,310 LOC) — dispatch, step lifecycle, recap stats
context.go                 — ExecutionContext, RunServices
result.go                  — Result, RegisteredResult, capture API
scope.go                   — VariableScope, register/As wiring
secret_resolve.go          — !secret typed-ref resolution
preflight.go               — pre-apply permission checks
inspect.go                 — plan-mode collector + StepInspection
dryrun.go                  — dry-run mode shims (some now legacy)
transaction.go             — spec-30 transaction state machine
trycatch.go                — spec-23 §2 try/catch/finally compound
tag_check.go               — --tags / --skip-tags filter
errors.go                  — typed step errors
```

Top gocyclo in the package: `ExecuteStep` = **37**. Second:
`walkAndResolveSecrets` = 23.

The package's `instability=0.18` is healthy ("controller layer
that's mostly imported"). But its *file structure* says it's being
treated as a layer ("everything that runs at execute time goes
here"). 67 packages import it. Every new concern that lands here
ripples to all of them.

This is architecture-by-accretion. Each individual file is
defensible. The aggregate is a package that's hard to *not* import
when you need anything from the runtime.

**Recommendation:**

Promote sibling sub-systems to peer packages where they have a real
seam. Specifically:

| Move | New package | Rationale |
|---|---|---|
| `transaction.go` + `trycatch.go` | `internal/control` | Compound step state machines — distinct from leaf-step dispatch. Currently 0 callers outside executor; can be promoted without external churn. |
| `secret_resolve.go` | `internal/secrets/resolver` | !secret refs are a pre-execute walk; cleaner as a service the executor calls. Also unlocks reuse from MCP / agent loop without dragging in 3,270 LOC of executor. |
| `preflight.go` | `internal/preflight` | Permission-aggregation pass is a separate phase already. |
| `tag_check.go` | `internal/plan/filter` | Tag filtering is a *plan-time* decision (per spec-32). It lives in executor for historical reasons. Belongs on the plan side. |

After the moves, `internal/executor` shrinks toward its actual
responsibility: **dispatch + step lifecycle + result capture**.
Target: under 1,500 non-test LOC.

This is not urgent. It becomes urgent the moment a fifth concern
lands in the package — e.g. "policy DSL evaluation" or "deterministic
replay state." Catching it before then keeps the move tractable.

### 3.3 Pressure: `internal/config/config.go` is the Step union, and it grows monotonically

**Evidence:**

`internal/config/config.go` is **1,866 LOC** — now the single
largest non-test file in the repo. It's mostly the `Step` struct: a
discriminated union with one pointer field per action type
(`FileWrite *File`, `Shell *Shell`, `GitClone *GitClone`, etc.) plus
the universal fields (`Name`, `Tags`, `As`, `When`, `OnChange`,
`Transaction`, `Try`/`Catch`/`Finally`, `ForEach`, ...).

Every new typed action adds one pointer field. Every new framework
primitive (transaction, try, on_change) adds one or more universal
fields. The file's growth rate over the last 60 days is roughly
linear with the number of shipped specs.

**Tension:**

- Adding actions is exactly what the project wants to keep doing.
- Non-goal #2 (no plugin marketplace) means there is no escape valve
  — every action ships in-tree.
- The schema generator (`internal/schemagen`) reflects over this
  struct, so the registry and the YAML schema stay in lockstep "for
  free". Splitting the struct breaks that.

**Why this is not really a smell yet:**

`internal/config` has **76 afferent packages** at instability **0.01**.
That's the right shape for a "shared data definition" package.
Touching this file is a global event by design; the cost of every
edit is intentional friction (`tools/check-schema.sh` already gates
this in pre-commit).

What would make it a smell:

- A second `Step` shape (e.g. a v2 schema). Today everything is in
  one type and the file is honest about that.
- Universal-field accretion. Current universal-field count is ~25;
  each new one is a global concept. Past ~40 the field has become a
  "tag everyone has to ignore."

**Recommendation:**

Watch, don't act. But:

- **Add a guard for universal-field growth.** Whenever a new
  universal field lands, the PR should explain "why does every step
  type need this?" Make it a thing the reviewer asks.
- **Resist temptation to split.** Splitting `Step` into per-action
  files saves nothing — the field count is the same, the import
  graph just gets noisier. The 1,866 LOC is the cost of the closed-
  action-set bet. The bet is right; pay the cost.

### 3.4 Pressure: handler proliferation produces a long fat tail

**Evidence:**

65 action packages under `internal/actions/`. Mean LOC ≈ 600. Top of
the distribution:

| Handler | LOC | Notes |
|---|---:|---|
| `file` | 2,044 | Multi-state (`file`, `directory`, `absent`, `link`, `hardlink`, `touch`, `perms`) — six sub-handlers in one package |
| `tool` | 1,676 | Tool discovery + version pinning |
| `service` | 1,466 | Cross-platform service abstraction (systemd / launchd / Windows SCM) |
| `package` | 1,216 | Cross-distro package manager dispatch |
| `text_patch_json` | 1,047 | RFC 6902 JSON patch with type-aware diff |
| `os_mount` | 1,071 | fstab + Windows mount + mac diskutil |
| `assert` | 1,004 | Cross-cutting predicates (file/http/expression/...) |
| `os_systemd` | 908 | systemd unit lifecycle |

The mean is fine. The top of the distribution has a pattern:
**handlers that are themselves cross-platform multiplexers** (file,
service, package, os_mount) accrete logic per OS branch and end up
mini-services.

**Concrete cyclo evidence:** `service.Run` = 27, `os_systemd.computePlan`
= 34, `os_systemd.applyPlan` = 28, `copy.Execute` = 41, `copy.Run` = 27.

**Recommendation:**

Don't refactor what's working. But establish a soft cap:

- **Handler > 1,500 LOC is a smell.** Either:
  - The handler is a cross-platform multiplexer → extract per-OS
    sub-packages (`internal/actions/service/{linux,darwin,windows}`).
    `service` is the cleanest candidate.
  - The handler is doing too many things → split into separate
    action types (the `file` package already does this *internally*
    via `state:` dispatch; one could argue `file.write` / `file.mkdir`
    / `file.remove` / `file.touch` as siblings would be cleaner
    today). Cost-benefit unclear — the current shape works.

For now: gocyclo > 35 in any handler is the trigger. Two functions
hit that today (`copy.Execute` 41, `os_systemd.computePlan` 34); both
are localised enough to refactor when next touched.

---

## 4. Drift watch

Things that are fine now but should be re-examined in 6 months.

### 4.1 `internal/agentd` carries HTTP routing + business logic

`internal/agentd` is 2,940 LOC at instability **0.78** with only 2
afferent. That's "leaf service" shape, which is correct. But inside
the package, the HTTP handlers do real orchestration (run submission,
SSE multiplexing, event-log streaming). If the daemon ever needs a
second transport (gRPC, unix-socket-IPC for an SDK), the HTTP layer
is glued to the orchestration.

**Watch:** if a non-HTTP entry point gets proposed, the agentd
package needs a service split first.

### 4.2 Two `registry` packages

- `internal/registry` (820 LOC, aff=1) — the **preset** registry
  (`mooncake presets list/install/recommend`).
- `internal/register` (64 LOC, aff=1) — the **action handler**
  registry (the side-effect-imports pattern).

The names are sibling-similar; the contents are entirely different
domains. Recommend renaming `internal/registry` → `internal/presets/
registry` or `internal/presets/library` next time it's touched.

### 4.3 `internal/effects` is at 50% instability with 2 callers

932 LOC, eff=2, aff=2. That's a mid-band package: not a foundation,
not a leaf. The split between `effects.Performer` (the interface the
file handler calls) and `effects/default.go` (the unsudoed
implementation) plus the sudo path scattered across the file handler
is mildly leaky.

**Watch:** if a third effect kind (besides the unsudoed default and
sudo wrapping) gets proposed — Windows-elevated, remote-via-agentd —
the package needs a stronger boundary first.

### 4.4 `cmd` doesn't import `cmd_test.go` — but `cmd_test.go` is 2,746 LOC

That's the largest test file in the repo. It exists because `cmd/` is
the only place where the end-to-end CLI flow is wired up. As the cmd-
extraction work in §3.1 happens, this file naturally shrinks. If it
*doesn't* shrink after extractions, that's a sign the extractions
left their orchestration entry point in cmd.

### 4.5 The actions package is the second-most imported foundation (72 aff)

`internal/actions` (the interface definitions, not the handlers) has
72 afferent at instability 0.06. That's a `config`-like position but
with eff=5 (vs. `config`'s eff=1). The 5 efferent edges
(`config`, `events`, `expression`, `logger`, `template`) are all
legitimate. Watch that count: above 8 means actions is starting to
pull in implementation-y dependencies.

---

## 5. Recommendations, ordered by leverage

1. **Extract `fleet.Orchestrator` from `cmd/fleet.go`.** Highest-
   leverage move. Unlocks (a) MCP/SDK callers, (b) testability
   without spawning a subprocess, (c) the next fleet stream (drift
   detection, scheduled apply) without piling on cmd. **Estimated
   blast radius:** medium — single file moves, no interface changes,
   no behavior change. **Risk:** low.

2. **Promote `transaction.go` + `trycatch.go` → `internal/control`.**
   Drops two well-bounded files out of executor. Sets the precedent
   for the executor split. No external callers — pure internal move.
   **Blast radius:** small. **Risk:** very low.

3. **Move `tag_check.go` to `internal/plan/filter`.** Tag filtering
   is a plan-time concept that landed in executor for historical
   reasons. Fixing the location is the kind of thing that compounds
   into "executor is a layer, not a controller." **Blast radius:**
   small. **Risk:** very low.

4. **Carve `apply.Runner` out of `cmd.run`.** Same shape as #1 for
   the local-apply path. The MCP server already wants to call this;
   today it can't. **Blast radius:** medium (largest function in
   cmd/mooncake.go). **Risk:** low.

5. **Document the soft caps in the repo guide.** "Handler > 1,500
   LOC is a smell. Universal Step field count > 40 is a smell.
   gocyclo > 35 is a refactor trigger." These are review-time
   prompts, not hard CI gates. Adding them to `CONTRIBUTING.md` or
   `LLM_GUIDE.md` makes the architecture self-policing as the
   project grows.

6. **Watch-only items from §4.** No action until a trigger event.

The first four moves total ~3,500 LOC of mechanical extraction, no
interface changes, and unlock structural seams the next streams will
need anyway. They are pure-debt-reduction work and can be done in
parallel by separate hands.

---

## 6. What this report intentionally does not cover

- **Performance.** This is a structural review. CPU/memory hotspots
  are a separate pass.
- **Test architecture.** The test/source ratio is healthy (the
  largest test files mirror the largest source files), but a
  dedicated review of fixtures / golden-file strategy / parallel
  flakiness would be its own report.
- **Security model.** The `become` / sudo gating in `internal/security`
  is at instability 0.00 and that's correct, but the threat model
  (especially MCP-driven mutation) needs its own architectural pass
  before the agent stream graduates from drafted to specced.
- **Documentation architecture.** `docs-next/` is in a known state of
  transition (mass `register:` → `as:` rot, ~50 stale examples). Not
  a code-architecture concern.
- **Module-system design** (the just-merged `presets → module`
  pivot). Too fresh to evaluate; revisit after the first signed-module
  spec lands.

---

## 7. The one-paragraph version

The kernel is in good shape — typed actions with an opt-in extended
ABI, a clean plan/execute boundary, a stable foundation that 65
handlers fan out from cleanly. The growth pressure is all at the
edges: `cmd/` is carrying application services that belong below it,
`internal/executor` is being treated as a layer rather than a
controller, and the closed action set has produced a `config.go`
that grows monotonically (which is the cost the team has chosen to
pay). The first move worth making is extracting `fleet.Orchestrator`
from `cmd/fleet.go` — it's the highest-cyclo function in the project,
it's the seam every future fleet caller needs anyway, and the work
is mechanical. Everything else can wait for a trigger.
