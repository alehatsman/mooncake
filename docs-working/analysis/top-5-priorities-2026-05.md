# Top-5 Priorities — 2026-05-14

## Context

Mooncake has shipped a high-quality kernel and most of Phase A+B of the personal-fleet runtime (8/14 PRs). But the work that ships doesn't match the marketing: the README pitches *"safe execution runtime for AI-driven configuration"*, while the foundation that turns the kernel into an actual agent-safety story — **spec-22 (Extended Handler ABI: `Diff`/`Reverse`/`Cost`/`Permissions`)** — is unstarted, and every Stream-2 spec downstream of it is blocked.

Three independent analysis docs converge on the same diagnosis:

- **VISION.md §13.10** leaves the unfair-advantage question open; the candidate answer is "tight coupling of plan + snapshot + rollback + deterministic agent replay" — none of which exists yet without spec-22.
- **streams.md** names spec-22 explicitly as "the infrastructure layer that all others depend on"; blocks the final phase of every Stream-1 spec and all of Stream 2.
- **next-priorities-2026-05.md** recommends "finish-then-pivot" — close out personal-fleet Phase B (2–3 weeks), then commit to Path A (spec-22 → spec-30 → transactions demo).
- **PROGRESS.md** quantifies the gap: "Secure AI execution" is ~30% complete; personal-fleet is ~70%.

This plan picks the smallest set of priorities that, if shipped together, would (a) close personal-fleet to the "Friday-evening demo" state and (b) unblock the agent-safety wedge so the README stops over-promising.

What gets cut on purpose: new Stream-1 action breadth (specs 24/25/26/27/28), enterprise hub specs, mDNS discovery (PR12/13), DX polish R7–R10. Per the gap-analysis filter: nothing new ships until Path A lands.

---

## The 5 priorities

### 1. Spec-22 — Extended Handler ABI ⭐ strategic blocker

**What:** Add `Diff() (Patch, error)`, `Reverse(Patch) (Action, error)`, `Cost() Cost`, `Permissions() []Capability` to the Handler interface. Wire them into at least the three highest-leverage handlers (file family, `text.line`, `pkg.install`).

**Why it's #1:** Every analysis doc and the streams.md "what to work on next" section names spec-22 as the bottleneck. It's the prerequisite for spec-30 (transactions), the killer agent-safety demo, and the final phase of every Stream-1 spec. Without it, every "agent safely changes a file and we can revert it" claim in the README is unproven.

**Sequencing note (per streams.md):** Land spec-22 *against* spec-30's transactions as the concrete consumer, not abstractly. Reverse-correctness for `pkg.install` is genuinely tricky (was-it-already-installed?) — that question should drive ABI design, not be resolved after.

**Effort:** L (per the streams.md framing — multiple weeks). Critical files: `internal/actions/handler.go`, `internal/actions/file/*`, `internal/actions/text_*`, `internal/actions/package/*`, `internal/snapshot/*`.

**Done when:** Three handlers implement the new interface; `mooncake plan --diff` shows structural diffs (not text); a single `Reverse()` call on a captured `Patch` produces an Action that undoes the original.

---

### 2. Spec-30 — `transaction:` blocks (reverse-on-failure)

**What:** A `transaction:` step block that runs N sub-steps; on any failure, calls `Reverse()` on each previously-completed step in reverse order and reports the audit trail.

**Why it's #2:** This is the demo the agent-safety pitch is missing — *"agent does 4 file edits, third fails, mooncake auto-reverts the first two."* The streams.md author calls this "the unfair-advantage demo." Pairs with spec-22; can't ship without it but should drive its design.

**Effort:** L. New planner + executor support, ~600 LOC. Critical files: `internal/plan/*`, `internal/executor/*`, plus three real example transactions in `examples/`.

**Done when:** A three-step `transaction:` block deliberately fails on step 3 and the run.completed event shows two `step.reversed` records. One README example replaces hedged marketing.

---

### 3. Personal-fleet PR 8 — `fleet logs` + `fleet facts`

**What:** `mooncake fleet logs <host>` (reattach to latest run via SSE), `mooncake fleet logs --all` (multiplex across all peers), `mooncake fleet facts <host>` + `--query <key>` (fan-out facts comparison).

**Why it's #3:** PR6's ^C banner already references `mooncake fleet logs <host>` as a forward-looking promise — it's currently dishonest, the command doesn't exist. Spec-46 has the design; PR7 already established the renderer + multiplexer reuse pattern; SSE consumer code is already in `internal/fleet/transport/sse.go`. Smallest path to closing Phase B's "real fleet" demo. Independent of spec-22.

**Effort:** S (~300 LOC + tests). Critical files: `cmd/fleet_logs.go` (new), `cmd/fleet_facts.go` (new), reuse `internal/fleet/multiplex.go` and `internal/fleet/transport/agentd.go:Stream`. Add `GetFacts` per-key lookup helper.

**Done when:** `fleet logs --all` reattaches to in-flight runs on N peers with multiplexed output. `fleet facts --query go_version` prints a tabular comparison across peers. Both have unit + integration tests.

---

### 4. Personal-fleet PR 9 + PR 10 — native SSH driver + systemd/launchd installer

**What:** Replace the shell-out `ssh`/`scp` calls in `internal/fleet/transport/ssh.go` with `golang.org/x/crypto/ssh`. Add `internal/fleet/bootstrap.go` embedded systemd-unit + launchd-plist templates, 8-step install sequence with rollback per spec-44's table.

**Why it's #4:** The currently-shipped "lite" bootstrap uses `nohup mooncake agentd ...` with no service unit — the daemon dies on logout. PR 11 is marked 🟡 in implementation-order.md *because* PR 9 + PR 10 didn't ship. Closes the bootstrap story end-to-end; lets `mooncake fleet bootstrap user@host` actually be the 60-second story the epic promises.

**Effort:** M-L (~850 LOC total). Critical files: `internal/fleet/transport/ssh.go` (rewrite), `internal/fleet/bootstrap.go` (extend), new `init/mooncake-agentd.service` + `init/com.mooncake.agentd.plist` embedded via `//go:embed`. Bootstrap tests (the current `bootstrap_test.go` doesn't exist — see gap-analysis B1).

**Done when:** `mooncake fleet bootstrap aleh@fresh-box` SCPs the binary, installs the platform-appropriate service unit, starts the daemon, and the new peer survives a reboot. Integration test against an alpine+sshd container in CI.

---

### 5. Spec-23 — Framework primitives (`on_change`, `try`/`catch`/`finally`, `!secret`)

**What:** Three independent primitives: reactive triggers (`on_change` re-runs a step when another step changes a thing), structured error handling (`try`/`catch`/`finally` blocks), secrets that mask in logs (`!secret` tag).

**Why it's #5:** Stream 2's other half. `on_change` and `!secret` are **independent of spec-22** so they can ship in parallel with the spec-22 work. `try`/`catch`/`finally` overlaps semantically with transactions but is a different shape (don't conflate). Together with spec-22 + spec-30, this completes the "agent-friendly mutation surface" the README pitches.

**Effort:** M (~500 LOC). Critical files: `internal/plan/*` (parser), `internal/executor/*` (`on_change` re-run logic, `try`/`catch` flow), `internal/security/*` (secrets masking).

**Done when:** A plan with `on_change: file.write -> shell` re-runs the shell only when the file changed. A `try`/`catch` block catches a deliberate failure and runs the `catch` branch. `!secret` values appear as `***` in `step.stdout` events.

---

## Parallel grouping

Two independent tracks. Each track has sequential pieces inside.

### Track A — Agent-safety wedge (the strategic priority)

```
spec-22 ──→ spec-30 (transactions demo)
   │
   └────── spec-23 (parts independent of spec-22, mostly parallel)
```

- **Spec-22 first** (gating) — but with spec-30's transaction shape used as the concrete design driver
- **Spec-30** starts the moment spec-22's `Reverse()` works on `file.write` (don't wait for all handlers)
- **Spec-23**: ship `on_change` + `!secret` in parallel with spec-22 (they don't touch the Handler interface). Land `try`/`catch`/`finally` after spec-30 (semantic overlap with transactions; design must align)

Track A is the "marketing-vs-reality" close-out. ~3–6 weeks one-engineer; faster with the parallel `on_change`/`!secret` start.

### Track B — Personal-fleet close-out (parallel to Track A)

```
PR 8 (fleet logs + facts)    │ ── both independent of each other
PR 9 + PR 10 (real bootstrap) │
```

- **PR 8** and **PR 9/10** are independent — pick either order, or split between two contributors.
- Both are **independent of Track A** — different code paths, different concerns. Run in parallel with Track A on a separate worktree.
- After both land, the personal-fleet epic flips from "8/14 with 🟡 partials" to "11/14 with Phase B fully ✅" — the Friday-evening demo from the epic.

Track B is ~2–3 weeks total if split, ~3–4 if one engineer does both.

### Already in flight (don't restart, just integrate)

- **PR 14 (spec-48 — overlays + peer-filter/step-filter)** — landing in `worktree-pr14-overlays-tags` per the other agent. Has the `--tag` name-collision question that I flagged in the gap analysis; resolve before merging.

### Explicit "not in this top-5"

These are valid work but lower-leverage right now per the analysis docs:

- **Stream 1 breadth** (specs 24/25/26/27/28) — wait until spec-22 lands; their final phases all depend on it
- **mDNS / `fleet init`** (PR 12, PR 13) — pure polish; defer until a real user complains about hand-editing peers.toml
- **Enterprise hub / cluster epic** — no paying users
- **WASM plugin model (spec-31)** — premature
- **DX polish R7–R10** — already 85% shipped; finish off after Track A demos

---

## Verification (per priority, end-to-end)

| # | How to know it's done |
|---|---|
| 1 | `mooncake plan --diff config.yml` prints structural diffs for `file.write` / `text.line` / `pkg.install`. A unit test feeds a `Patch` to `Reverse()` and asserts the resulting Action undoes the original. |
| 2 | Run `examples/transaction-revert.yml` (new), fail step 3 deliberately, see `step.reversed` events for steps 1 and 2 in `runs/<id>/events.jsonl`. README's agent-safety paragraph maps 1:1 to this demo. |
| 3 | With 2 live agentds + 1 slow apply in flight, `mooncake fleet logs --all` shows interleaved live output. `mooncake fleet facts --query go_version` prints a 3-row comparison table. New tests cover both flags. |
| 4 | `mooncake fleet bootstrap aleh@fresh-box`: SCP binary, register service unit, start daemon, register peer, exit 0. Reboot the box; agentd comes back up automatically. Bootstrap integration test passes against an alpine+sshd container. |
| 5 | Plan with `on_change: file.write -> shell` re-runs shell only on file-write change. `!secret` value in `vars/secrets.yml` appears as `***` in run events. `try`/`catch`/`finally` runs the catch branch on a deliberate failure. |

When all five are shipped, every README claim is falsifiable. That is the stopping condition for "Path A is done" per the next-priorities doc.
