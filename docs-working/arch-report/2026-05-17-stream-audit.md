# Mooncake — Stream Audit & Feature Completeness

**Date:** 2026-05-17
**Scope:** Stream-by-stream verification of what's shipped vs. what
the stream READMEs claim, plus a snapshot of in-flight work,
remaining gaps, and recommended next moves.
**Method:** Read each `docs-working/streams/<stream>/README.md`,
walk `internal/actions/` (78 packages) and `cmd/` (~50 files),
cross-check `~/.mooncake/claims.jsonl` and `git log` against the
claimed-shipped lists, surface drift and unrepresented work.

This is a *complement* to the arch-reports: those focus on
package structure, this focuses on shipped-feature coverage per
stream.

---

## 0. Headline

All four streams are **feature-complete on their stated v1 scope**.
The system is past "kernel works"; the current pressure is at the
**edges** (typed-diff renderers, AI-discovery surfaces, cross-
platform parity, fleet observability), not at the foundation.

| Stream | v1 scope | New specs drafted | Active work | Notable gaps |
|---|---|---|---|---|
| **core** | ✅ shipped | 3 (spec-65/66/68) | spec-66 w5, proposal-16 wave 3+, F-fix sweep | per-spec docs, reverse-capture rollout to 13 handlers |
| **fleet** | ✅ shipped end-to-end | 2 (spec-55/58) | shutdown+WoL (landed mid-day), darwin parity | spec-55/58 still drafted; enterprise hub deferred |
| **dx** | ✅ shipped | 0 | proposal-04 just merged | macOS preset coverage, "import dotfiles", marketplace |
| **agent** | ✅ load-bearing primitives shipped | 2 (spec-31/67) | proposal-01 MCP discovery merged 2026-05-17 | **Policy DSL, plan signing, quotas, egress, sandbox, replay — all un-specced** |

---

## 1. Core — kernel + handlers + planner + executor

### Verified shipped

- 78 action sub-packages under `internal/actions/`; all five
  families (file/text/pkg/os/git, plus container/wait/read/observe/
  repo) present.
- spec-22..38 + spec-59..64 receipts all match commits.
- Four-method handler ABI (`Permissions`/`Diff`/`Cost`/`Reverse`)
  wired through `internal/register`.
- R2.1c phase 2 (ReverseData wire round-trip) landed 2026-05-17
  at `5dd81b95` — closes the per-peer `Reverse()` gap.

### In-flight

- **spec-66** typed-plan-diff renderers — waves 2/3/4 merged,
  wave 5 (cron + mount) claimed 2026-05-17.
- **proposal-16** `http.request` — waves 1+2+3 merged; the
  `expect_json_keys` narrow assertion + `save_to` response sink
  shipped 2026-05-17 (`f897c37c`, `71c90dae`).
- **Cross-platform darwin extensions**: `os.group`, `os.service.Reverse`,
  `pkg.list / pkg.hold / pkg.upgrade` — all claimed/landed in the
  2026-05-17 work-day window. Also `pkg.hold` dnf support
  (`87e9d350`) for the RHEL trifecta.

### Open gaps

- **Per-spec docs in `docs-next/`.** Every shipped core spec
  carries a "docs phase pending" tail.
- **Reverse-capture rollout to refusing handlers.** spec-26 reverse-
  capture v1 shipped for `git.checkout` / `git.config`; the pattern
  is reusable for ~13 handlers (`os.*` family, `pkg.repo`, `pkg.hold`,
  `os.service`) that still return refusal stubs.

### Backlog snapshot (proposals/)

- **Audit-discipline batch (01–06)**: capability flags (05) and
  failed/error distinction (06) shipped; 01–04 partially absorbed
  by spec-66/68; 02/03 still open.
- **User-filed batch (07–10)**: **all four merged**.
- **Action-set batch (11–15)**: all unstarted (assert/heal, kv-
  state, process, watch, plan-recurse).
- **proposal-16** (`http.request`): ~85% shipped; only schema-validation
  variant `expect_json_schema` deferred.

---

## 2. Fleet — agentd + peer transport + multi-machine commands

### Verified shipped

Original 14-PR plan 14/14 closed, plus three operational features
delivered outside the plan: `fleet apply <machine>` (ordered phases),
`fleet upgrade` (Linux + Windows), `fleet doctor` per-peer probe
ladder. Two-peer WSL + Windows testbed validated `running` /
`failed` / `unreachable` states.

### In-flight / landed 2026-05-17

- **`fleet shutdown` + `fleet up`** (WoL). Auto-MAC capture +
  explicit `fleet mac-refresh`. Landed `3a6d3dbf`.
- **R2.1c phase 2** (ReverseData over the wire) — `5dd81b95`.
  Closes the per-peer `Reverse()` gap end-to-end.

### Open gaps

- **spec-55** (`fleet doctor` fan-out) and **spec-58** (`fleet drift`)
  still drafted — both high-leverage. Drift is the "single feature
  that would turn config-management into a fleet operating system"
  per the stream README, and nobody is working it.
- **macOS agentd** functional but preset coverage uneven vs. Linux.
- **Enterprise hub**: intentionally deferred. Not a regression.

---

## 3. DX — `init`, `doctor`, `history`, error quality

### Verified shipped

spec-39..42 all map to commits. `mooncake doctor`'s ~16 checks
present in `cmd/doctor.go`. `mooncake init` template scaffolding
present in `cmd/init.go`. The agent-dx Makefile targets
(`build-pkg`, `test-pkg`, `sym`, `doc`, `refs`, `callers`, `impl`,
`budget-status`) all present.

### In-flight / landed 2026-05-17

- **proposal-04** `mooncake actions show <name>` merged
  (`b806505b`); F047 follow-up (descriptions + enums enrichment)
  merged (`533c15e8`).

### Open gaps (no doc drift)

- "Import existing dotfiles" — no spec.
- `mooncake share <preset>` / marketplace — depends on core
  spec-65 (modules) landing first.
- macOS preset coverage (same item as fleet).
- First-run "what now?" affordances — partial.

### Backlog snapshot (proposals/)

01 (step-name defaults), 02 (output middle ground), 03 (`mooncake
watch`), 05 (error recipes), 06 (`mooncake lint`) — all unstarted.

---

## 4. Agent — MCP server, agent loop, agent-safety primitives

### Verified shipped

spec-10 MCP, spec-18 agentd, spec-22 four-method ABI MCP-wired,
spec-23 (`on_change` / `!secret` / `try/catch`), spec-30
transactions w/ LIFO rollback. `examples/transactions/rollback-
demo.yml` exists and is runnable.

### In-flight / landed 2026-05-17

- **agent proposal-01** — `list_actions` / `describe_action` /
  `list_presets` MCP tools — merged `82e9668a`. Closes the agent
  capability-discovery gap.

### Drafted, not started

- **spec-31** (tier-2 plugin model — `notify.*`, `container.compose`,
  `k8s.apply`, `db.postgres.*`).
- **spec-67** (`mooncake pilot` — in-binary LLM-driven plan
  generation, provider-portable). Drafted 2026-05-16.

### The real story for this stream

The following items are **all un-specced** and constitute the
entire "agent safety wedge" that VISION.md positions as the
defensible product:

- Policy DSL (`deny: agent.touches("/etc/passwd")`).
- Plan signing (Sigstore-style).
- Per-action quotas.
- Egress policy.
- Sandbox mode (agent loses shell entirely).
- Deterministic replay (`mooncake replay <run-id>`).
- Cost/risk classifier on top of `Cost()`.

Three of the README's "four typed-end-to-end" guarantees — plan,
snapshot, reverse — are demoable today. The fourth (replay) is the
last open piece. **This is the stream with the most strategic
upside and the thinnest spec coverage.**

---

## 5. Cross-cutting issues

### Doc drift on stream READMEs (resolved 2026-05-17)

Both core and fleet READMEs were stale by 1–2 days at the start
of this audit:

- `streams/core/README.md` listed "Active specs: None" while
  three specs existed (spec-65/66/68). Fixed in `f08ddfb4`.
- `streams/fleet/README.md` listed R2.1c phase 2 as an open gap
  while it had merged the same day (`5dd81b95`). Fixed in
  `f08ddfb4`.

The README index in `streams/` is the contract; it needs a
refresh after every merge wave.

### Manual-test debt (closed 2026-05-17)

The 43-finding manual-test queue at
`docs-working/archive/analysis/findings-2026-05-15/` is fully
closed. Original three highest-ROI fixes (`#15` creates/unless
unification, `#27/#35` validator-schema wiring, `#40` bare-binary
github-release install) all shipped.

### Code-review queue (1 open as of this audit)

`docs-working/code-review/` shipped 46 findings (F001–F047, F036
skipped) — all `status: done`. **F048** filed during the audit:
fleet `machine.go` parses `fleet.yml` with non-strict YAML, while
the main plan loader uses `KnownFields(true)` — silent unknown-
field acceptance class, same shape as F044.

### Architecture soft-caps

Run `make budget-status` for the live list. As of this audit:

- `service` package at 1844 LOC (was 1514 at audit start — grew
  during the day from `os.service.Reverse` darwin extension).
  **Over the 1500 soft cap by ~23%.**
- `pkg_repo` at 1507 LOC. Just over the cap.
- `file`, `http_request`, `os_user`, `tool` all within 20% of the
  cap.
- `internal/config.Step` universal-field count: 36 (cap 40).

---

## 6. Recommended next moves

In rough order of leverage.

1. **Cheap wins (low effort, high signal).**
   - Refresh stale stream READMEs after every merge wave (done
     today; needs to become a habit).
   - F048 fix: `yaml.NewDecoder + KnownFields(true)` in
     `internal/fleet/machine.go`. One-line change.

2. **High-leverage spec to schedule: spec-58 fleet drift.**
   The fleet stream README calls it "the single feature that
   would turn Mooncake from config management tool into fleet
   operating system." Zero claims activity. Drafted only.

3. **Strategic gap: pick one un-specced agent-safety item.**
   Policy DSL or deterministic replay are the two with the
   highest "this is what makes Mooncake defensible" payoff.
   The agent stream has the least spec coverage of any stream
   *and* the biggest strategic upside per VISION.md.

4. **`service` package split.** Now ~23% over the 1500 soft cap,
   growing fast with cross-platform extensions. Split into
   `internal/actions/service/{linux,darwin,windows}` per
   CLAUDE.md §1. Will become the next blocked-by-cap PR if not
   addressed.

5. **Continue the code-review cold-read sweep.** 11 packages
   still unread (see `docs-working/code-review/TODO.md`
   "Still to review"). F048 came out of the first one
   (`internal/fleet/machine.go`, 140 LOC).

---

## 7. What this audit intentionally does not cover

- **Package-level structural analysis.** That's the arch-report's
  job. See `2026-05-16-refactor-plan-complete.md` for the latest
  pass.
- **Test coverage.** Item 11 on the code-review queue.
- **Performance.** Not benchmarked here.
- **Documentation completeness.** Per-spec docs in `docs-next/`
  are a tracked gap but not enumerated.
- **Marketing / positioning.** See `VISION.md` and
  `docs-working/positioning.md`.

---

## 8. Addendum — what shifted within 2026-05-17

The audit was performed mid-day 2026-05-17. By end of day the
following had landed and are reflected above:

- `pkg.hold` dnf support (`87e9d350`) — RHEL pkg.* trifecta complete.
- `os.service.Reverse` darwin (`edf71b5d`).
- `pkg.upgrade` darwin via brew (`aae079fd`).
- `fleet shutdown` + `fleet up` (WoL) (`3a6d3dbf`).
- `http.request` `expect_json_keys` (`71c90dae`) and `save_to`
  (`f897c37c`).
- Stream README refreshes (`f08ddfb4`).
- Code-review TODO restructure (`012caba1`).
- Test-suite fzf-hang fix (`0884f4fc`).
- F048 filed (`0e64665f`).

Future audits should re-baseline against `git log --since=2026-05-17`
before drawing conclusions about what's "in-flight" — the project's
delivery cadence is currently 8–12 merges per work-day.
