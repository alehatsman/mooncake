# Architecture Delta — post-refactor-plan

**Date:** 2026-05-16 (late)
**Codebase:** `github.com/alehatsman/mooncake` @ `6f4cde0`
**Predecessor:** [`2026-05-16-refactor-plan-complete.md`](./2026-05-16-refactor-plan-complete.md)
**Scope:** Three batches that landed after the plan-complete bookmark:
R2.1c (daemon KernelResult wire), agent-dx tooling, side-findings
(test isolation + soft-cap refresh).

---

## 0. Summary

The one open checkpoint criterion from the plan-complete doc —
`internal/mcp imports internal/apply directly` — is **still open**.
Everything else that moved this week was either completing the wire
protocol or tooling/hygiene.

**What closed:**
- R2.1c: `apply.KernelResult` now round-trips over the agentd wire
  protocol. `fleet.FleetKernelResult.Reverse()` composes against typed
  Steps from each peer instead of returning `ErrPerPeerKernelResultNotWired`.
- agent-dx: `AGENT.md` + focused Makefile targets (sub-second
  per-package feedback, gopls structural lookups, `budget-status`).
- side-findings: GIT_* env isolation for 5 test packages +
  `fleet/discovery` mDNS disable; CLAUDE.md soft-cap numbers corrected.

**What's still open (forward-looking, same as plan-complete §4):**
1. MCP → `internal/apply` direct import (the one remaining ✗ criterion)
2. Spec-66 typed plan diffs
3. R0.1-followup (Reverser interface)
4. `explain.DisplayFacts` gocyclo 53
5. `copy.Execute` gocyclo 41

---

## 1. R2.1c — daemon KernelResult wire (`2d058cb2`)

The R2.1b bookmark noted: "FleetKernelResult.Peers carries KernelResult
placeholders; the wire isn't there yet." R2.1c closes that gap.

### What moved

**`internal/agentd`** (+~150 production LOC, +90 test LOC):
- `worker.go` — `executeRun` passes `*executor.RunCapture` via
  `StartConfig.Capture`; a new `daemonSummarySink` subscribes to
  `run.completed` and writes `<run_dir>/result.json` after the events
  stream flushes.
- `runs_handler.go` — new `GET /v1/runs/{id}/result` endpoint. Returns
  the persisted `result.json`; 404 `result_not_ready` while
  `writeResult` is still running (narrow race window).
- `store.go` — `Store.ResultPath()` accessor.
- `result_test.go` — E2E test: submit plan, wait terminal, GET result,
  assert Plan/Steps/Summary fields are populated.

**`internal/fleet/transport`** (+70 production LOC, +~100 test LOC):
- `agentd.go` — `Client.GetRunResult(ctx, runID) → (*apply.KernelResult, error)`.
  `ErrRunResultNotReady` sentinel for the race window. 16 MiB body cap
  (vs 1 MiB for trivial endpoints).
- `integration_test.go` — wire-level E2E: real agentd over TCP, submit
  + stream + `GetRunResult`, assert `*apply.KernelResult` round-trips
  with `Plan/Steps/Summary` populated.

**`internal/fleet`** (+38 LOC net after orchestrator refactor):
- `apply.go` — `ApplyResult` gains a `KernelResult` field. `Apply()`
  calls `GetRunResult` after SSE stream terminates; `ErrRunResultNotReady`
  skipped silently, other errors emit a banner but don't fail the apply.
- `apply_phase.go` — `ApplyPhaseOutcome.PerPeer []ApplyResult` surfaces
  typed per-peer results to `RunApplyPhase` callers.
- `orchestrator.go` — maps `PerPeer` into `PeerResult`, feeding
  `FleetKernelResult.Peers` with typed kernel tail per peer.

### Known gap: ReverseData (R2.1c phase 2)

`Result.ReverseData` and `Result.Detail` are `json:"-"`. After a
wire round-trip, handlers that depend on `ReverseData` (git.checkout,
git.config, os.ssh_key, os.mount, pkg.repo, os.service, os.firewall,
os.systemd) will see `ReverseData=nil` and surface their refusal path.

Round-tripping `ReverseData` requires a per-handler type registry with
a discriminator on the wire — tracked as R2.1c phase 2. Purely additive
once the registry mechanism is in place.

---

## 2. agent-dx — LLM-agent tooling (`ff257432`)

Not structural, but directly affects the agent-assist feedback loop.

**`AGENT.md`** — single-screen cheat sheet: focused commands, gopls
lookups, soft-cap rules, where things live. Discoverable via
`make agent-help`.

**Makefile targets added:**

| Target | What it does |
|---|---|
| `make build-pkg PKG=...` | Compile one package |
| `make test-pkg PKG=...` | `-race -count=1` for one package |
| `make test-fn FN=... PKG=...` | Targeted `-run` |
| `make lint-pkg PKG=...` | golangci-lint for one package |
| `make check-pkg PKG=...` | build + test + lint, one package |
| `make sym Q=...` | `gopls workspace_symbol` |
| `make doc SYM=...` | `go doc` |
| `make refs LOC=...` | `gopls references` |
| `make callers LOC=...` | `gopls call hierarchy` |
| `make impl LOC=...` | `gopls implementations` |
| `make budget-status` | Soft-cap report (LOC/gocyclo/Step fields) |

**`scripts/budget-status.sh`** — parses `internal/config/config.go`
for universal Step fields (those without an `action:` tag), runs
gocyclo, counts handler LOC. Feeds the CLAUDE.md soft-cap policy.

---

## 3. side-findings — test isolation + soft-cap refresh (`453727c9`)

**GIT_* env isolation:**
- 5 new `main_test.go` files (`git_checkout`, `git_clone`, `git_config`,
  `agent`, `agentd`) unset all `GIT_*` vars at `TestMain` startup.
- `internal/snapshot/minimal.go` — `gitCleanEnv()` helper filters
  `os.Environ()` of `GIT_*` before spawning git subprocesses.
  Prevents hook-invoked runs from corrupting test fixtures.
- `internal/fleet/discovery` — `TestAggregate_*` tests now pass
  `mdns: false` to avoid picking up real LAN peers during CI.

**CLAUDE.md soft-cap refresh:**
- Corrected Step universal-field count from "~25" to 36 (actual).
- Known-violations list updated: dropped `fleetApplyAction` and
  `os_systemd.computePlan` (both now under cap); added
  `explain.DisplayFacts` (gocyclo 53) and `executor.ExecuteStep`
  (gocyclo 37); bumped `internal/actions/service` to 1,607 LOC;
  added `internal/actions/package` (within 20% of cap).
- Added pointer to `make budget-status` so the doc doesn't drift again.

---

## 4. Structural metrics (unchanged since plan-complete)

Package LOC, coupling, and the gocyclo top-5 are unchanged from the
plan-complete snapshot. R2.1c added ~220 production LOC to agentd +
fleet/transport but both packages are leaves (instability 0.80 /
0.17) with no new import edges.

The one new import edge: `internal/fleet/transport` now imports
`internal/apply` (to type the `GetRunResult` return). This is
architecturally correct: transport is a pure leaf, and pulling in
the apply kernel shape is how the typed wire gets its type.

---

## 5. Next

Same priority order as plan-complete §4:

1. **MCP → `internal/apply` import.** The last ✗ criterion.
   `internal/mcp` still goes through `executor.Start`; it should call
   `apply.NewRunner(cfg).Run(ctx)` directly and surface
   `*apply.KernelResult` in the MCP response.
2. **Spec-66 typed plan diffs.** Build on `KernelResult` + handler
   `Differ` payloads (computed, currently discarded in check_plan).
3. **R2.1c phase 2** (ReverseData registry). Purely additive; unblocked
   once someone needs per-peer `Reverse()` to work through the wire.
