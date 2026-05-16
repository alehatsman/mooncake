# Issue #9 — Killer demo: analysis + plan

**Source:** [#9 Build killer demo: AI-proposed infrastructure mutation with rehearsal, risk analysis, transaction rollback, and explainability](https://github.com/alehatsman/mooncake/issues/9)
**Analyzed against:** master @ `c6f6838` (2026-05-15)
**Companions:** [`../clustermanagement/issue-10-analysis.md`](../clustermanagement/issue-10-analysis.md) (historical-systems audit), [`../clustermanagement/issue-11-analysis.md`](../clustermanagement/issue-11-analysis.md) (cluster-mgmt capabilities)
**Author:** assistant, 2026-05-15

---

## 0. TL;DR

The demo proves *one* sentence: **"AI can propose infrastructure changes
without receiving unrestricted shell access, and without permanently
breaking the machine."**

Mooncake has more of the substrate than it looks. Of the 11 capabilities
the issue's capability matrix names, **6 already ship** (typed plan,
structured diff, permission model, health assertions, transactions,
rollback) and **3 ship in minimal form** (artifact history, AI
integration via MCP, risk per-step via `Coster`). The actually-missing
pieces are:

1. **Plan-level risk aggregation** (`mooncake risk <plan>`) — the
   per-step `Risk 1..10` band already exists; no roll-up yet.
2. **Rehearsal sandbox** (`mooncake rehearse <plan>`) — execute against
   a throwaway container/VM, run assertions, predict failure. Real
   new mechanism.
3. **Explainability index** (`mooncake explain <resource>`) — resource
   → run-history reverse index. The data exists in run-logs; the
   query path doesn't.
4. **The demo scenario itself** — a ~60-line YAML config + walkthrough
   that ties everything together.

This doc audits each demo phase against current code (§2), names the
hard parts (§3), proposes a 4-slice plan with explicit acceptance
criteria (§4), and details the slice-1 work plan so it's directly
shippable (§5). The anti-goals (§6) keep the demo small.

The strategic conclusion: **Slice 1 is mostly polish + glue around
already-shipped primitives.** It can land in a week and the demo is
already 80% real. Rehearsal (slice 3) is the genuinely new mechanism
and deserves its own design pass.

---

## 1. What the demo actually proves

The demo's emotional center is a **controlled failure**: a plan
executes, an assertion fails partway through, the system rolls back to
clean state, and the operator sees why. Not a happy-path provisioning
walkthrough — those are everywhere and prove nothing.

Three things the demo proves that nothing else in the infra-tools
world demonstrates:

1. **Typed mutation surface.** The AI doesn't get shell. It writes a
   plan. Mooncake compiles the plan, types every step, and refuses to
   execute anything that doesn't compile to a known action. This is
   the wedge — every other "AI for ops" demo is "Claude has SSH and
   bash."
2. **Risk-before-execution.** The AI submits a plan; Mooncake shows
   risk, permissions, affected resources, and reversibility *before*
   anyone touches reality. The human approval is informed, not blind.
3. **Honest rollback.** Failure happens. Mooncake walks back the
   transaction in LIFO order, calling each step's compensator. The
   final state is verifiable. No "partial success — please clean up
   manually" footer.

Everything else in the demo (CUDA, Triton, nginx, explain) is *staging*
for those three claims. Substitute Postgres for Triton and the wedge
is identical; the staging just needs to feel real.

---

## 2. Current-state audit, by demo phase

### Phase 1: Initial state

**The issue wants:** Linux box with CUDA 12.6, ollama, nginx, inference
API, all healthy.

**Today:**

- `examples/ollama/ollama-quick-start.yml` installs ollama with
  tinyllama and verifies via HTTP — fully working.
- nginx, systemd services, package management all have actions
  (`os_systemd`, `pkg_install`, `service`, `file.template`).
- The "inference API" is whatever ollama exposes on `:11434` — no
  separate service needed; saves one moving part.

**Gap.** Nothing material. A `demo/setup.yml` (60 lines, idempotent)
brings a fresh box to the initial state.

### Phase 2: User request → AI generates plan

**The issue wants:** Plain-English upgrade request → LLM emits a
Mooncake plan.

**Today:**

- MCP server (`internal/mcp/server.go`) exposes `run_plan`,
  `check_plan`, `get_facts`, `get_snapshot`, `get_metrics`,
  `fact_query`. The LLM can already read system state and run
  arbitrary plan text.
- Claude/Cursor can write Mooncake YAML — the schema is in
  `internal/config/schema.json`.

**Gap.** No `propose_plan` MCP tool — the LLM has `run_plan` (execute)
but no "render this plan for human approval" half. That's slice 4
work, but it's small.

### Phase 3: Plan output (risk, changes, affected, rollback availability)

**The issue wants:**

```
Plan generated.
Risk: 8/10
Changes: upgrade CUDA, restart Docker, install Triton, modify nginx, open port 8001
Affected services: ollama, inference-api, nginx
Rollback: available for 7/8 steps
Requires: sudo, network
```

**Today:**

- `Plan.Inspections[].Cost.Risk` — per-step risk 1..10 already exists
  (spec-22 phase 6, `internal/actions/handler_abi.go:147`).
- `Plan.Inspections[].Diff` — typed per-step diff for handlers that
  implement `Differ` (file, template, package, service).
- `Plan.Inspections[].Cost` — blast-radius estimate per step.
- `PermissionSet` per step via `Permitter` — declares `RequiresSudo`,
  `RequiresNetwork`, `Binaries`, `WritesPaths`, etc.
- `internal/mcp/tools.go:aggregatePermissions(p *plan.Plan)` already
  rolls permissions up plan-wide for MCP consumers.

**Gap.** Three small renderers, all reading from already-populated data:

- A `mooncake risk <plan>` CLI command that aggregates
  `Inspections[].Cost.Risk` plan-wide (max? mean? weighted? — design
  question, see §3.1) and prints the rollup.
- A `mooncake plan --risk` flag that adds the risk band to standard
  `mooncake plan` output.
- An "affected services" extractor — walks `Inspections[].Diff` for
  `Resource.Kind == ResourceService` references, dedupes. ~30 LOC.

Reversibility-per-step is already there too: `Cost.Reversible bool`
exists on every inspection, and the transaction planner already
refuses irreversible steps unless `allow_irreversible: true` (spec-30).

### Phase 4: Rehearsal

**The issue wants:**

```bash
mooncake rehearse deploy.yml
```

Runs plan in a sandbox (container/namespace/VM/Lima/WSL), executes
assertions, reports predicted operational failures.

**Today:** **Nothing.** There is no sandbox path. `--check` (spec-15)
is dry-run, not sandboxed-execute.

**Gap.** Real new mechanism. The fact that this is the only missing
strategic piece means it deserves its own spec (§3.2). The cheap-but-
real first version: ephemeral Docker container, mount plan + presets
in, run `mooncake apply` inside, capture exit code + events.jsonl,
return a delta. The expensive-but-complete version: ephemeral VM
(Lima on macOS, qemu on Linux, WSL on Windows) for the steps that
container can't simulate (kernel-level, systemd-as-pid-1 properly).

### Phase 5: Real execution with failure

**The issue wants:** Execute the plan; one assertion fails; surface
the failure.

**Today:**

- Wait primitives (`wait_command`, `wait_file`, `wait_http`,
  `wait_port`) + `assert` action cover the assertion vocabulary.
- `examples/transactions/rollback-demo.yml` already demonstrates
  controlled failure mid-transaction.
- Run-log + events.jsonl persist the failure with structured fields.

**Gap.** Nothing for execution; the gap is on the *demo content* side
— writing the deploy.yml so a specific assertion fails realistically
(e.g. `assert: http://localhost:8001/health == 200` against a Triton
deploy whose ENV is wrong).

### Phase 6: Automatic rollback

**The issue wants:** Reverse traversal — restore nginx config, remove
Triton, restore Docker, restart old services.

**Today:**

- spec-30 transactions already do this. The existing rollback-demo.yml
  performs LIFO `Reverse()` walk + `on_rollback` notify.
- Per-step `Reverser` is implemented for file/template/package/service
  (the demo-relevant action set).

**Gap.** Nothing mechanical. The demo just needs to wrap its risky
steps in a `transaction:` block. The polish is making the rollback
output *legible* — the existing CLI shows `↺ Reverse: ...` but a demo
benefits from a clearer final summary ("Transaction failed. System
restored.").

That summary is ~50 LOC in the executor's terminal output layer.

### Phase 7: Explainability

**The issue wants:**

```bash
$ mooncake explain triton
Introduced by: run-145
Reason: local inference serving upgrade
Rolled back because: inference-api health assertion failed
Affected: docker, nginx, cuda-runtime
```

**Today:** **Nothing.** Run-logs exist (`internal/agentd/store.go`,
`internal/runlog/`) and they have all the data (every step's
`Resource.Identifier`, status, run-id, timestamp) — but there is no
reverse index keyed by resource. `mooncake runs ls` shows runs; no
command shows "what touched this resource?"

**Gap.** Index + renderer. The index can be derived on demand from
existing JSONL run-logs (~200 LOC walking
`~/.mooncake/runs/*/events.jsonl`, filtering by `Diff.Resource.Identifier`).
For a v0 demo the on-demand scan is fast enough (a few hundred runs at
most). A persistent index can come later.

### Capability matrix (issue's table) — reality check

| Capability | Issue checked | Reality | Gap |
|---|---|---|---|
| typed plan | yes | ✅ shipped (spec-16) | none |
| structured diff | yes | ✅ shipped (spec-22 Differ) | none |
| permission model | yes | ✅ shipped (spec-22 Permitter) | none |
| risk scoring | yes | 🟡 per-step shipped; no plan rollup | trivial aggregator |
| rehearsal environment | yes | ❌ not started | **real new spec** |
| health assertions | yes | ✅ shipped (assert + wait_*) | none |
| transactions | yes | ✅ shipped (spec-30) | none |
| rollback | yes | ✅ shipped (LIFO Reverse) | output polish |
| explainability | yes | ❌ not started | reverse index + cmd |
| AI integration | yes | 🟡 MCP shipped; no propose flow | small additions |
| artifact history | yes | 🟡 local runs/; no resource index | covered by explain |

Six of eleven capabilities are *done*. The demo is closer than the
issue's "build everything" framing implies.

---

## 3. The hard parts

Three places where engineering judgment matters more than rote
implementation.

### 3.1 Plan-level risk aggregation

The `Cost.Risk` band is per-step. Plan-level risk requires picking an
aggregation function, and the choice signals the operator's mental
model.

**Options:**

1. **Max.** The riskiest step is the plan's risk. Honest about
   worst-case, ignores cumulative effect.
2. **Sum, capped at 10.** Approximates "more risky steps = more risky
   plan." Saturates quickly and loses signal at the top end.
3. **Weighted by reversibility.** `Risk_step * (1 + irreversible_weight)` —
   irreversible steps weight 2x. Models "irreversible high-risk is
   especially bad."
4. **Two-number rollup.** Report both: max-risk + irreversible-step
   count. ("Risk 8/10, 2 irreversible steps.") Most informative; not
   a single number.

**Recommendation: option 4** (two-number rollup), surfaced in CLI as:

```
Risk: 8/10 (1 step), Irreversible: 2/8 steps
```

Reasoning: the single-number framing is the historical mistake (CVSS
scores, etc.). Two numbers force the operator to look at both
dimensions. Six lines of code; significantly more informative.

The MCP version returns a struct: `{ max_risk, irreversible_count,
total_steps, by_step: [...] }` so the LLM can reason structurally.

### 3.2 Rehearsal sandbox (the actually new mechanism)

The honest design question: **what does "sandbox" mean across the
platforms Mooncake supports?**

- Linux: Docker container (fast, cheap) covers ~70% of action surface.
  Doesn't cover: kernel modules, sysctl that affects host kernel,
  systemd-as-PID-1, hardware (CUDA driver), bind-mounted host paths.
  For those, ephemeral qemu/firecracker VM.
- macOS: Lima (Linux VM on macOS). Already a project dep candidate.
  Can't simulate macOS-specific actions (LaunchAgents, macOS pkg
  manager).
- Windows: WSL. Same trade-off — Linux actions yes, native Windows
  actions no.

**Design principle.** *Don't promise full simulation.* The sandbox
returns one of three verdicts per step:

- **`simulated`** — step ran in sandbox, assertion passed.
- **`unsimulatable`** — step cannot run meaningfully in sandbox
  (kernel module, hardware probe, etc.). Reported as `skipped`,
  *not* counted as success.
- **`failed-in-sandbox`** — step ran, assertion or step itself
  failed.

The rehearsal output is then:

```
Rehearsal complete.
8 steps total
6 ✓ simulated
1 ⊘ unsimulatable (cuda-driver-install — sandbox cannot test)
1 ✗ failed-in-sandbox (assert http://localhost:8001/health)

Recommended action:
  - Resolve sandbox failure before applying to real machine.
  - Manually verify unsimulatable step.
```

The `unsimulatable` honesty is the differentiator. Terraform's `plan`
implicitly claims success; rehearsal explicitly admits its limits.

**Spec shape.** A new spec — call it `spec-59-rehearse.md` — covers:

1. `Sandbox` interface (Linux container, Linux VM, macOS-Lima, WSL).
2. `Handler.Sandboxable() SandboxCapability` — opt-in per-handler
   declaration: `Full | PartialNoSideEffects | NotSimulatable`.
   Default = `Full` (assume container works).
3. `mooncake rehearse <plan>` CLI: pick sandbox impl, run plan, emit
   verdict.
4. Cleanup guarantee: sandbox is torn down on success or failure.

Effort estimate: **2-3 weeks** for v0 Linux-container sandbox + 3
opt-out handlers (kernel-module, sysctl, hardware) + cleanup harness.
VM-based sandboxing is a follow-up.

### 3.3 Explainability index

The `mooncake explain triton` query is "walk all run-logs, find any
event where `Diff.Resource.Identifier` matched or `Step.Args.name`
matched, build a chronological story."

**Design question.** Build a persistent index, or scan on demand?

**Recommendation: scan on demand for v0.** A typical personal-fleet
user has < 500 runs of < 30 steps each = 15K events. JSONL filter +
match completes in < 100ms. A persistent index is premature
optimization.

The interesting design question is *what's the match key?* Options:

- **Identifier match** (path / package name / service unit). Works
  for typed handlers; fails for `shell` actions which don't tag their
  resources.
- **Name fuzzy match** (the step's `name:` field). Cheap but
  ambiguous; "install triton" matches even if the actual install
  happened via `pkg.install: { name: triton }`.
- **Composite.** Match by Identifier first; fall back to fuzzy name
  match; tag the response with the match type.

Recommendation: composite, with the response tagged. The CLI shows:

```
$ mooncake explain triton

Resource: triton (matched by package identifier)

Introduced by:
  run-145 (2026-05-12 14:32) — author: aleh
  step: install triton inference server
  reason: "local inference serving upgrade"
  diff: package install, 0 → 2.42.0

Rolled back at:
  run-145 (2026-05-12 14:38) — assertion failed
  cause: assert http://localhost:8001/health returned 502
  cleanup: package removed via Reverse()

Currently:
  not installed (last verified: 2026-05-12 14:38)

Affected (in same transaction):
  - docker (restarted)
  - nginx (config updated, then reverted)
```

The "Currently:" line requires a live facts check. Acceptable cost on
an explain call.

---

## 4. Plan: 4 slices, ordered

The issue's own slice breakdown is fine — it's reordered slightly
here to put *shippable demo* first and *unique mechanism* second.

### Slice 1 — Minimal viable demo (week 1)

**Goal.** A working `examples/agent-changegraph/deploy.yml` that
demonstrates the failure-and-rollback arc. Uses *only* already-shipped
primitives.

**Deliverables:**

1. `examples/agent-changegraph/` directory with:
   - `setup.yml` (initial state: nginx + ollama running, idempotent).
   - `deploy.yml` (60-80 lines, transaction with intentional failure).
   - `README.md` (the demo walkthrough script).
2. New CLI: `mooncake risk <plan>` — plan-level risk rollup
   (two-number format per §3.1). ~80 LOC.
3. New `mooncake plan --risk` flag — adds risk band + reversibility
   summary to standard plan output. ~40 LOC.
4. Rollback summary polish: end-of-run line "Transaction failed.
   System restored." or "Transaction failed. ROLLBACK FAILED — manual
   intervention required." ~50 LOC in `internal/executor/`.

**Acceptance:**

- `mooncake plan examples/agent-changegraph/deploy.yml` shows risk
  + reversibility + affected services.
- `mooncake apply examples/agent-changegraph/deploy.yml` runs forward,
  fails on the planted assertion, rolls back cleanly, exits non-zero.
- After rollback, `setup.yml` state is restored byte-for-byte.
- The demo's README can be read by someone unfamiliar with Mooncake
  and they understand the wedge.

**Effort:** 1 week. The biggest chunk is writing a *good* demo
scenario; the code adds are small.

### Slice 2 — Risk + permissions + explain (week 2-3)

**Goal.** The pre-execution intelligence layer: an operator can read
a plan and know what they're about to break, before applying.

**Deliverables:**

1. `mooncake explain <resource>` CLI — on-demand scan of run-logs,
   composite identifier+name match, structured output per §3.3.
   ~250 LOC + tests.
2. `mooncake plan --json` already exists; add `risk_summary` and
   `affected_resources` blocks to the JSON payload so MCP consumers
   can render them. ~60 LOC.
3. New MCP tool: `propose_plan(yaml, reason)` — *renders* (not
   executes) a plan with full risk/diff/permission output, ready for
   human approval. Returns the same payload as `mooncake plan --json
   --risk`. ~100 LOC.
4. New MCP tool: `explain_resource(name)` — backs `mooncake explain`
   for LLM consumers.

**Acceptance:**

- After the slice-1 demo runs (and rolls back), `mooncake explain
  triton` produces a chronological story tagged with the rollback
  reason.
- Claude/Cursor (or any MCP client) can call `propose_plan` with a
  YAML blob and get back a structured risk + diff + reversibility
  payload — *without executing anything*.
- A second demo script ("AI proposes plan → human approves → apply")
  in `examples/agent-changegraph/with-mcp.md`.

**Effort:** 1-2 weeks.

### Slice 3 — Rehearsal sandbox (week 4-5)

**Goal.** Predict failure before touching real reality.

**Deliverables:**

1. `spec-59-rehearse.md` — design doc covering Sandbox interface,
   Sandboxable handler opt-in, lifecycle, cleanup.
2. Linux-container sandbox impl (Docker-shelling for v0; libcontainer
   later if Docker dep is too heavy).
3. `Handler.Sandboxable()` declarations for the demo-relevant action
   set: full for file/template/shell-with-no-host-side-effect; partial
   for service/pkg; unsimulatable for kernel-module, sysctl,
   hardware-probe actions.
4. `mooncake rehearse <plan>` CLI with verdict output per §3.2.
5. Demo update: `examples/agent-changegraph/README.md` includes a
   `rehearse → fail → fix → apply` arc.

**Acceptance:**

- `mooncake rehearse examples/agent-changegraph/deploy.yml` spins up
  a container, applies the plan inside, runs the planted assertion,
  emits a `failed-in-sandbox` verdict, tears down the container.
- A handler marked `NotSimulatable` produces a `⊘ unsimulatable`
  verdict in the rehearsal report — not a silent success.
- The sandbox always tears down, even on Mooncake panic (tested via
  test that injects a panic mid-rehearsal).

**Effort:** 2-3 weeks. The genuine new spec.

### Slice 4 — Full MCP loop + recorded demo (week 6)

**Goal.** End-to-end agentic flow: LLM proposes, Mooncake gates,
human approves, Mooncake applies (or rolls back).

**Deliverables:**

1. New MCP tool: `apply_approved(run_token)` — applies a plan that
   was previously proposed and returned a token. Token-gating prevents
   the LLM from re-applying without re-presenting. ~150 LOC.
2. New MCP tool: `rehearse_plan(yaml)` — front-ends `mooncake rehearse`
   for LLM consumers.
3. Demo recording (asciinema or video): the 5-minute "LLM proposes
   upgrade → Mooncake shows risk → human approves → apply fails →
   rollback → explain" arc.
4. `docs-next/guide/agentic-demo.md` — public-facing writeup.

**Acceptance:**

- An LLM can chain `propose_plan → rehearse_plan → apply_approved`
  through MCP without any other shell access.
- The recorded demo is < 5 minutes and shows all 11 capabilities from
  the issue's matrix.
- The demo is reproducible from a fresh box via `make demo`.

**Effort:** 1 week.

---

## 5. Slice-1 implementation detail (the directly-shippable part)

Slice 1 is mostly content (the demo YAML) wrapped around two small
features. Detailed here so it can be picked up directly.

### 5.1 The demo scenario

**Realistic-enough-to-trust narrative.** The user has a working ollama
serving local models. They (or their LLM) want to add a *second*
service on the same box: a fictional "inference dashboard" exposing
ollama stats via nginx, with a healthcheck endpoint. The plan:

1. Install a fictional `inference-dashboard` package (use a real
   small npm/pip package or just `file.write` a python script — keep
   it self-contained).
2. Render nginx config to reverse-proxy `/dashboard` to the new
   service.
3. Reload nginx.
4. Start the dashboard service.
5. Open firewall port (planted failure: assert health endpoint).

**The planted failure.** The dashboard service starts but the health
endpoint returns 502 because — in the demo — the nginx config is
*deliberately* missing a `proxy_set_header Host` directive that
triggers the dashboard's host-validation. The `assert
http://localhost:8080/dashboard/health == 200` step fails.

This failure is realistic (host-header misconfig is a common nginx
gotcha), reversible (rollback restores nginx config + stops the
service), and produces a satisfying recovery story.

**File structure:**

```
examples/agent-changegraph/
├── README.md                    # walkthrough script
├── setup.yml                    # initial: nginx + ollama, idempotent
├── deploy.yml                   # the demo plan (transaction + assertion)
└── presets/
    └── inference-dashboard/
        ├── preset.yml
        ├── dashboard.py.j2      # the fictional service
        └── nginx.conf.j2        # the planted-buggy reverse-proxy config
```

**`deploy.yml` (target shape, ~70 lines):**

```yaml
version: "1.0"

steps:
  - name: install + deploy inference dashboard
    transaction:
      - name: install dashboard service
        use:
          name: inference-dashboard
          with:
            port: 8080
            backend: http://localhost:11434

      - name: render nginx reverse-proxy
        file.template:
          src: presets/inference-dashboard/nginx.conf.j2
          dest: /etc/nginx/sites-enabled/dashboard.conf

      - name: reload nginx
        service:
          name: nginx
          state: reloaded

      - name: assert dashboard healthy
        assert:
          shell: curl -fsS http://localhost:8080/dashboard/health
          expect_exit: 0

    on_rollback:
      - name: record rollback reason
        file.write:
          path: /tmp/mooncake-rollback-marker
          content: |
            Dashboard deploy rolled back at {{ now }}.
            Cause: health assertion failed.
            See: mooncake explain inference-dashboard
```

The `inference-dashboard` preset's nginx.conf.j2 is intentionally
missing `proxy_set_header Host $host;` — the demo's planted bug.

### 5.2 `mooncake risk` command — implementation

**Location:** `cmd/risk.go` (new), shelling out via `internal/plan/`.

**Logic:**

```go
func runRisk(cmd *cobra.Command, args []string) error {
    p, err := plan.BuildPlanFromFile(args[0])
    if err != nil { return err }

    var maxRisk int
    var irreversibleCount int
    var totalSteps int

    for _, ins := range p.Inspections {
        if ins.Skipped { continue }
        totalSteps++
        if ins.Cost != nil {
            if ins.Cost.Risk > maxRisk { maxRisk = ins.Cost.Risk }
            if !ins.Cost.Reversible { irreversibleCount++ }
        }
    }

    perms := mcp.AggregatePermissions(p)   // already exists

    fmt.Printf("Risk:          %d/10\n", maxRisk)
    fmt.Printf("Irreversible:  %d/%d steps\n", irreversibleCount, totalSteps)
    fmt.Printf("Requires sudo: %v\n", perms.Sudo)
    fmt.Printf("Network:       %v\n", perms.Network)
    return nil
}
```

Reuses `mcp.aggregatePermissions` — already plan-wide, already wired.

### 5.3 `--risk` flag on `mooncake plan`

In `cmd/plan.go`, add `--risk bool` flag. When true, the textual
plan-output is suffixed with the rollup block from §5.2 plus a per-
step risk column in the existing plan table.

Pseudocode:

```go
if flagRisk {
    // ... existing plan output ...
    printRiskSummary(p)
    for _, ins := range p.Inspections {
        if ins.Cost != nil && ins.Cost.Risk >= 5 {
            fmt.Printf("  [risk %d/10] %s\n", ins.Cost.Risk, ins.StepID)
        }
    }
}
```

### 5.4 Rollback summary line

In `internal/executor/` (find the transaction-completion log point —
likely `internal/executor/transaction.go` or the equivalent), after
rollback completes:

```go
if rollbackErr != nil {
    log.Printf("Transaction failed. ROLLBACK FAILED — manual intervention required.")
    log.Printf("  Last successful step: %s", lastReverseStep)
    log.Printf("  Failed reverse: %s — %v", failedReverseStep, rollbackErr)
} else {
    log.Printf("Transaction failed. System restored.")
    log.Printf("  Reverted %d step(s).", reversedCount)
}
```

Honest failure-mode reporting per §2.11 of the historical-systems
analysis: never claim rollback succeeded when it didn't.

### 5.5 Slice-1 verification checklist

Before declaring slice 1 done:

- [ ] `examples/agent-changegraph/setup.yml` is idempotent (run twice
      → zero changes second time).
- [ ] `examples/agent-changegraph/deploy.yml` runs to assertion
      failure on first apply.
- [ ] Rollback restores the byte-for-byte pre-deploy state (verify
      with `mooncake snapshot diff before after`).
- [ ] `mooncake risk examples/agent-changegraph/deploy.yml` produces
      the two-number rollup with > 0 irreversible.
- [ ] `mooncake plan --risk` output is legible.
- [ ] The README walkthrough is followable by someone who has
      cloned Mooncake but never used it.
- [ ] All new code has unit tests; the demo scenario has at least
      one integration test that runs the full apply → fail → rollback
      arc.

---

## 6. Anti-goals

The demo dies the moment any of these creep in:

1. **No Kubernetes.** No "deploy this to a k3s cluster." The demo is
   local-first or it isn't the demo.
2. **No real cloud APIs.** No AWS / GCP / Azure provider calls. The
   demo runs offline on a fresh laptop.
3. **No multi-agent orchestration.** One LLM, one plan, one human.
   "Agents negotiating" is a different demo.
4. **No giant YAML.** The deploy.yml stays under 80 lines. If it
   grows, the scenario is wrong.
5. **No SaaS control plane.** No backend service, no signup, no
   dashboard hosted anywhere. Mooncake binary + local box.
6. **No CUDA / GPU dependency.** The issue mentions CUDA + Triton
   for *flavor*; the real demo substitutes a fictional service so
   it runs on any laptop. CUDA reappears as a "scale-up" example
   in docs, not in the canonical demo.
7. **No success-only narrative.** The failure-and-rollback arc is
   load-bearing. A demo that succeeds the first time proves nothing.
8. **No "AI does everything."** The human gate is the wedge. The
   demo must show the approval step explicitly — otherwise it's
   "AI has shell" with extra steps.

The anti-goals are the demo's *quality control*. Anyone proposing
additions checks against this list first.

---

## 7. Dependency on issue #8 (ChangeGraph)

The historical-systems analysis (#10 §4) argues ChangeGraph (#8) is
the highest-leverage missing piece because everything else reads
from it. **Does the killer demo depend on ChangeGraph?**

**No, but it's strictly better with it.**

Without ChangeGraph, slice 1's demo works fine — the transaction is
linear and `Plan.Steps[]` is enough to drive Reverse() in LIFO order.

ChangeGraph would add three things the demo could show:

- **Explanation graph traversal.** `mooncake explain inference-dashboard`
  could walk the dependency graph and surface "depends on nginx,
  ollama; was rolled back via reverse-walk through {step-3, step-2,
  step-1}." Without the graph, explain is chronological-only.
- **Smarter risk scoring.** "This step touches a service three other
  steps depend on" is graph-shape data. The two-number rollup in
  §3.1 is a good v0 without it.
- **Rehearsal ordering.** A graph-aware sandbox can run independent
  steps in parallel inside the sandbox. Linear is fine for v0.

**Recommendation.** Slice 1-2 ship without ChangeGraph. Slice 3
(rehearsal) and slice 4 (MCP loop) ship without it too. ChangeGraph
arrives in parallel; the demo upgrades to use it once #8 lands.

The killer demo is the *forcing function* that proves ChangeGraph's
value, not a feature gated on it.

---

## 8. Suggested rollout

Sequence, in priority order:

1. **Slice 1 (week 1)** — minimal demo + risk command. This is the
   shippable thing. The wedge is proven from this slice alone.
2. **Slice 2 (week 2-3)** — explain + propose_plan MCP tool. Adds
   the agentic surface.
3. **Slice 3 (week 4-5)** — rehearsal sandbox (spec-59). The
   genuinely new mechanism.
4. **Slice 4 (week 6)** — full MCP loop + recorded demo.

Total: 6 weeks for the full arc. Slice 1 alone (1 week) is enough to
demo internally and validate the framing.

**Stop conditions.** If slice 1 doesn't land cleanly in a week, the
substrate has a gap the audit missed — *fix the substrate*, don't
work around it in the demo content. If rehearsal (slice 3) drags
past 3 weeks, ship slices 1+2+4 and defer rehearsal to a v2 — the
demo without rehearsal still proves the wedge.

**Visible artifact at each slice.** Each slice should produce something
demoable in isolation:

- Slice 1: `mooncake apply deploy.yml` shows fail+rollback.
- Slice 2: `mooncake explain inference-dashboard` shows provenance.
- Slice 3: `mooncake rehearse deploy.yml` shows predicted failure.
- Slice 4: an asciinema of an LLM driving the full loop.

---

## 9. Open questions

Issues that need a decision before slice work starts:

1. **Risk aggregation.** Two-number (recommended) vs. single-number
   vs. structured-only. Affects CLI design.
2. **Explain match keys.** Identifier-only vs. composite (recommended).
   Affects index design.
3. **Sandbox runtime.** Docker (cheap, requires docker installed) vs.
   buildah-style rootless container (more setup, no daemon) vs.
   Lima (cross-platform, heavier). For v0, **recommend Docker** with
   a clear "requires Docker" prerequisite — keeps slice-3 cheap.
4. **The "fictional service" content.** The demo's
   `inference-dashboard` should be small, real-feeling, and not
   require external dependencies. A 50-line Python `http.server`
   that hits ollama's API and adds host-validation is plausible.
   Worth prototyping in slice 1.
5. **Where slice 4's recorded demo lives.** Hosted asciinema?
   YouTube? GitHub-rendered SVG? Affects what the README links to.

---

## 10. References

- Issue [#9 killer demo](https://github.com/alehatsman/mooncake/issues/9) — source spec.
- [`../clustermanagement/issue-10-analysis.md`](../clustermanagement/issue-10-analysis.md) — historical-systems audit; argues ChangeGraph is the highest-leverage missing piece, and that the wedge ("AI safety substrate") is what this demo proves.
- [`../clustermanagement/issue-11-analysis.md`](../clustermanagement/issue-11-analysis.md) — fleet capabilities; the demo runs single-node, fleet is out of scope.
- [`../specs/done/spec-30-transactions.md`](../specs/done/spec-30-transactions.md) — already done; explicitly calls itself "Killer demo. Headline feature."
- [`../specs/done/spec-22-extended-handler-abi.md`](../specs/done/spec-22-extended-handler-abi.md) (search for `spec-22` if filename differs) — the Differ/Reverser/Coster/Permitter ABI that powers the typed-plan surface.
- [`../../examples/transactions/rollback-demo.yml`](../../examples/transactions/rollback-demo.yml) — existing transaction example; slice 1's demo derives from this shape.
- Issue [#8 ChangeGraph](https://github.com/alehatsman/mooncake/issues/8) — strictly-better substrate; not a blocker for this demo.
