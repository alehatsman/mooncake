# Mooncake — Streams Progress & Ideal-State Report

Generated from `VISION.md`, `ROADMAP.md`, and the freshest `docs-working/` state
(master @ `401ab95`, 2026-05-15, revision 15).

> **What changed since revision 14**: a wide quality-and-coverage round
> rather than a single new feature. Headlines:
>
> 1. **Personal-fleet original 14-PR plan is now 14/14 ✅.** PR 13
>    interactive `fleet init` shipped (`a43db9c`/`f3d64c9`); rev-14
>    inconsistencies between "14/14 (epic moved to done)" and "Phase C
>    2/3 with PR13 ⏳" are reconciled to **14/14, Phase C complete**.
> 2. **Four shipped fleet specs moved from `specs/personal-fleet/` to
>    `specs/done/`**: spec-45 fleet-discovery, spec-52 fleet-exec
>    (`c597854`/`7e855b0`), spec-53 fleet-watch (`ad34ac0`/`e569ad3`),
>    spec-54 fleet-ps (`528530b`/`df3d4dd`). The only remaining drafted
>    personal-fleet items are spec-55 (fleet-doctor fan-out, single-host
>    ladder already shipped) and spec-58 (fleet-drift).
> 3. **Massive manual-test fix campaign — ~40 issues closed in the round
>    37 of `docs(working)` commits.** Critical: `file.download` sha256
>    verified before rename (MT-14); `for_each` iterates slice elements,
>    not `reflect.Value` repr (MT-8); shell timeout kills the process
>    group (#16); plan-dir walker skips non-regular files (#15);
>    `failed_when` stops fabricating exit code 1 on clean exit 0 (#21).
>    GitHub issues closed: **#13, #14, #15, #16, #18, #19, #20**. Plus
>    ~30 MT-xx fixes (creates:/unless: honored on every action, retry
>    backoff strategies, strict YAML decode, presets-list/runs-list
>    `--format json`, JSON-RPC notification suppression, …). The
>    findings live under `analysis/findings-2026-05-15/` and the manual
>    tester's `verification-2026-05-15.md` tracks fix status per
>    finding.
> 4. **Action-surface ABI rollout is functionally complete on the
>    priority set**: spec-26 phase 5 (git.* ABI hooks, `c3d67ab`/`e4b6d3d`),
>    spec-27 phase 4 (os identity, `4cd6037`), spec-28 phase 6 (os
>    scheduling, `3a7b49d`/`eb93572`). All five action families
>    (file, text, pkg, os.service, os identity, os scheduling, git)
>    now declare `Permissions`/`Diff`/`Cost`/`Reverse`. The remaining
>    work is per-spec docs, not method declarations.
> 5. **Spec-26 reverse-capture v1 shipped** (`419b127`/`a5da5c0`) —
>    `git.checkout` and `git.config` replace `Reverse()` refusal stubs
>    with real apply-time state capture (typed `*ReverseInfo` on
>    `Result.ReverseData` → inverse Step). This unblocks the same
>    pattern for 13 other handlers that still refuse (os.* family,
>    pkg.repo, pkg.hold, os.service).
>
> Strategic position is unchanged from rev 14: the kernel is **fully
> wired**, the four-method ABI is **declared on every priority handler
> and consumed through MCP**, the rollback demo is **runnable code**,
> and the personal-fleet flagship is **closed**. The headline gap
> remains *users* (lighthouse adoption), not engineering.

> **What changed since revision 13** *(carried from rev 14)*: two more Stream-1 specs landed.
> **Spec-37 (Step Output Capture)** shipped (`901e013`/`2ee98e7`) — a
> `CaptureInPlan` capability on `ActionMetadata` lets plan-mode bind
> `as:` results into `Scope.Results` for opted-in actions (mutation
> actions still stay invisible to vars during plan). The three
> executor write sites + a new plan-mode capture path all funnel
> through one `captureResult` helper that also emits a collision
> warning on `as:` overwrite — for_each-sibling iterations are
> exempt via `step.Origin`-keyed detection. Dry-run wording:
> "Would register result as:" → "Would capture result as:".
> **Spec-38 (`read.json` + `read.yaml`)** shipped (`8549c33`/`2ee98e7`) —
> two tier-1 read-only actions that close the gap forcing every
> "read a value from a file" flow to shell out to `jq`/`yq`. Both
> set `CaptureInPlan: true` so plan-mode runs publish the value into
> the run scope; downstream `when: pkg.found` clauses branch
> correctly in plan preview. New `internal/pathquery/` package
> (dotted + integer-index path subset, reserved for future spec-25);
> `security.Redactor` gains `AddPattern` + `RedactValue` deep
> walker. New surface: `read.json` / `read.yaml` with `path`,
> `query`, `max_bytes` (default 4 MiB), `redact:` regex list.
> read.yaml rejects multi-document files at parse time. The
> shell-out-to-jq pattern that breaks completely in sandboxed
> agent mode now has a typed, redactable, plan-mode-friendly
> alternative. The strategic constraint stays where rev11–rev13
> put it — at **users**, not code.

> **Older revs (compressed)**: rev 13 landed spec-22 phase 7 (MCP wiring),
> spec-23 §2 try/catch/finally, fleet doctor probe ladder + SSH fallback,
> fleet upgrade Windows path, and drafted specs 56/57/58.

---

## 1. Streams progress

Mooncake organises work into five parallel streams. Snapshot below.

### Stream 1 — Action Surface  *(kernel completeness)*

The typed mutation vocabulary. Ships everywhere.

| Spec | Topic | State |
|---|---|---|
| 22 | Extended Handler ABI (`Diff`/`Reverse`/`Cost`/`Permissions`) | ✅ phases 1–7 shipped; MCP surfaces all four methods. Phase 8 (docs) outstanding |
| 24–28 | `pkg.*` · `text.*` · `git.*` · `os.user`/`group`/`ssh_key` · `os.cron`/`systemd`/`sysctl`/`mount`/`firewall` | ✅ all priority families shipped with four-method ABI. Per-spec docs phase outstanding |
| 17 | Batched packages + templated `names` | ✅ shipped |
| 37 | Step output capture (collision + plan-mode) | ✅ shipped (`901e013`/`2ee98e7`) — `CaptureInPlan` capability + `for_each`-aware collision warning |
| 38 | `read.json` / `read.yaml` | ✅ shipped (`8549c33`/`2ee98e7`) — typed readers, `CaptureInPlan: true`, `query:` / `max_bytes:` / `redact:` |
| 32 | Collapse step action dispatch | not started — structural refactor |

**Verdict**: action breadth and the four-method ABI are both **complete on the priority set and wired through MCP**. What remains is per-spec docs and the spec-32 dispatch refactor (not a feature).

### Stream 2 — Safe Agent Runtime  *(the defensible wedge)*

| Spec | Topic | State |
|---|---|---|
| 22 | Extended Handler ABI | 🟡 in progress (see Stream 1) — phases 3 + 4 + 5 + 6 + **7 ✅** (`92b58d8`/`1d43a48` — `Diff`/`Cost`/`Permissions` surfaced through MCP `check_plan`/`run_plan`). Only phase 8 (docs) remains. |
| 23 | Framework primitives (`on_change`, `try/catch/finally`, `!secret`) | **✅ all three sections shipped**. §1 `on_change` ✅, §3 `!secret` ✅ + plan-output redaction polish, **§2 `try/catch/finally` ✅** (`f598238`/`7b4d62a` — compound steps, semantic overlap with spec-30 transactions resolved). |
| 30 | `transaction:` blocks with auto-reverse | **PR A ✅** (parser + plan-time reversibility check, `7c2c00e`/`e3276e0`) + **PR B ✅** (executor + LIFO rollback + on_rollback gating, `15cdc79`/`dd097ea`). **The agent-safety demo is runnable** via `examples/transactions/rollback-demo.yml`. |

Plus a list of unwritten future specs in `streams.md`: policy DSL, plan
signing, per-action quotas, egress policy, sandbox mode, cost classifier,
deterministic replay.

**Verdict**: spec-22 is now 7/8 — the four-method ABI is **wired
through MCP**, not just declared. spec-23 is **fully shipped** —
`try/catch/finally` (§2) closed the last drafted section.
`transaction:` blocks (spec-30) work end-to-end and the rollback
demo is runnable. **Stream 2's drafted-spec backlog is empty**;
what remains lives in the un-specced future list (policy DSL,
plan signing, per-action quotas, egress policy, sandbox mode,
deterministic replay) and in phase 8 of spec-22 (docs).

### Stream 3 — Fleet & Cluster Management  *(the monetizable wedge)*

Personal Fleet (sub-stream): **🎉 14/14 PRs shipped end-to-end** as of
2026-05-15. Epic moved to
[`epics/done/epic-personal-fleet.md`](epics/done/epic-personal-fleet.md);
this stream's flagship is closed.

| Phase | PRs | State |
|---|---|---|
| **A** (one peer end-to-end) | 1–5 | ✅ all shipped |
| **B** (real fleet) | 6 multiplexer ✅, 7 status ✅, 8 logs/facts ✅, 9 native SSH driver ✅, 10 installer templates + 8-step bootstrap ✅, 11 bootstrap/pair ✅ | ✅ complete |
| **C** (polish) | 12 mDNS ✅ (advertise + browse), **13 `fleet init` ✅** (`a43db9c`/`f3d64c9` — interactive flow), 14 overlays/tags ✅ | ✅ **complete** |
| **Post-plan QoL** | `fleet exec` (spec-52, `7e855b0`), `fleet ps` (spec-54, `df3d4dd`), `fleet watch` (spec-53, `e569ad3`), `fleet upgrade`, `fleet doctor` ladder, `fleet apply <machine>`, mDNS slice | ✅ shipped on top |

**Beyond the original 14-PR plan** (drafted from real-world use):

| Spec | Topic | State |
|---|---|---|
| 45 / 50 / 51 / 52 / 53 / 54 | discover + mDNS / extended filter keys / local-apply overlay / fleet exec / watch / ps | ✅ all shipped; moved to `specs/done/` |
| — | `fleet apply <machine>` (ordered multi-peer) · `fleet upgrade` (Linux + Windows) · `fleet doctor` probe ladder + SSH fallback | ✅ all shipped |
| 49 | agentd on Windows (TCP-only, SSE race fixes) | ✅ shipped |
| 55 | fleet-doctor fan-out wrapper | 📝 drafted; single-host ladder already shipped |
| 56 | Windows fleet bootstrap | 📝 drafted (`56203fd`) |
| **58** | **Fleet drift** — periodic `InspectPlan` + `/v1/drift` + `fleet drift` + per-machine `drift:` policy | **📝 drafted (`d963c25`)** — highest-leverage candidate from GitHub issue #11 |

**Verified against a real WSL + Windows two-peer testbed.** Enterprise hub
sub-stream (C1–C5): zero specs, intentionally deferred.

**Verdict**: the stream is **done for v1 + has a forward backlog**.
v1 is closed: mDNS discovery, `apply <machine>`, fleet upgrade
(Linux + Windows), and a real `fleet doctor` ladder all shipped.
Phase C closed: interactive `fleet init` shipped (`a43db9c`/`f3d64c9`).
The drafted-spec backlog narrows to **three items** (55 fleet-doctor
fan-out, 56 Windows fleet bootstrap, 58 fleet-drift) — specs 52, 53,
54 shipped end-to-end and moved to `specs/done/`. **spec-58 fleet-drift**
is the strategically heaviest of the three and would turn Mooncake from
"config management tool" into "fleet operating system" — issue #11
explicitly ranks drift detection as the highest-value operational
feature.

### Stream 4 — Developer Experience  *(the funnel)*

The DX audit drove four spec batches:

| Spec | Topic | State |
|---|---|---|
| 39 | `mooncake init` + auto-discovery | ✅ shipped |
| 40 | Default config discovery + `--dry-run` alias | ✅ shipped |
| 41 | `mooncake doctor` (16 health checks) | ✅ shipped |
| 42 | Examples index + `history` + `presets recommend` | ✅ shipped |

DX-audit items R7–R10 (history-show, doctor extensions, recommend
polish, first-run tip) — partly done; the rest listed as untouched in
`next-priorities`.

**Verdict**: the gap from "kernel-only, hand-write YAML" to "Mooncake feels
like a real tool" is closed, and the user-filed request that was the
last open DX item is now in master. Next DX increment is whatever
operator pain surfaces next from real use.

### Stream 5 — Ecosystem  *(plugins, marketplace, integrations)*

| Spec | Topic | State |
|---|---|---|
| 31 | Tier-2 plugin model (`notify.*` proof) | drafted |
| — | Tier-2 secret provider plugin (Vault/age/1Password) | 🟡 in flight in `worktree-tier2-secrets` — building on the just-shipped `!secret` env provider (spec-23 §3) |

**Verdict**: the long-standing "explicitly deferred" line is no longer accurate — a tier-2 secret-provider plugin opened in a worktree. The `!secret` env provider gave Stream 5 a natural first hook to plug into, and the project picked it up. spec-31's `notify.*` proof is still drafted.

---

## 2. Where Mooncake stands against four ideal-state visions

### A. Personal dotfiles management

**Ideal**: `mooncake init dotfiles` scaffolds, you `git push` it, on a new
box `curl … | mooncake apply <repo>` bootstraps everything — packages,
configs, services, shell. `mooncake plan` shows drift; `mooncake apply`
makes it boring. Sharing a preset is a one-liner.

**What's shipped**: `mooncake init` ✅, default config discovery ✅,
`mooncake plan` with `--diff` ✅, `mooncake apply` ✅, `mooncake doctor` ✅,
`mooncake history` ✅, `mooncake presets recommend` ✅, **330+ built-in
presets** ✅, snapshot/diff ✅, structured errors ✅, run history JSONL ✅.

**Gap**: `mooncake share <preset>` / marketplace doesn't exist (Stream 5,
deferred). One-line bootstrap-from-URL (`curl | mooncake apply <repo>`)
isn't explicit — `install.sh` ships, but the "pull config + apply" loop is
documented as DIY. **No "import existing dotfiles" command** to ease
migration.

**Distance to ideal**: ~95% there. The story is real and self-consistent
today. The gap is polish (preset sharing UX) not capability.

### B. Personal computer provisioning (single new machine)

**Ideal**: pick up a fresh laptop / VM / WSL / Mac. Run one command. End
up with: dotfiles + dev tools + packages + services + drift detection + an
audit trail. Works on Linux, macOS, Windows (WSL or native).

**What's shipped**: `install.sh` single-binary ✅, full action surface for
`pkg`/`file`/`text`/`service`/`user`/`group`/`cron`/`sysctl`/`systemd`/`mount`/`firewall` ✅,
**Windows native support** (spec-49) ✅, idempotent re-runs ✅, snapshot for
compliance/audit ✅, check mode (`mooncake plan`) ✅, `mooncake history` ✅.

**Gap**: macOS preset coverage smaller than Linux. Windows is fresh;
corners likely. No "agent sandbox" template even though the DX audit
drafted one. The `unarchive`/`download` actions exist but
the `disk-partition-action.md` exploration moved to `deferred/`.

**Distance to ideal**: ~85%. The kernel can do this; coverage is uneven
across OSes. macOS especially is "works but presets thinner."

### C. Multi-device provisioning on local network  *(personal fleet)*

**Ideal**: `mooncake fleet apply config.yml` from any box, applies to all
your boxes, interleaved logs scroll past, `fleet status` shows health,
`fleet bootstrap user@new-box` adds a new machine in 60s, per-host
overlays land naturally. No hub, no SaaS, peer-to-peer over LAN.

**What's shipped**: agentd with TCP listener + bearer auth + SSE hub +
sandboxed file sync + `/v1/files` PUT/HEAD endpoints ✅, controller-side
multiplexed `fleet apply` ✅, **`fleet status`** with `--json` ✅, **`fleet
logs` + `fleet facts`** ✅, **`mooncake fleet discover`** (spec-45 simple
— probes `peers.toml` + `~/.ssh/config` against `/v1/version`,
table-rendered) ✅, parallel multi-peer multiplexer with `^C` banner ✅,
`peers.toml` + `controller_id` ✅, **native SSH driver** (crypto/ssh +
pkg/sftp, ssh-agent → IdentityFiles → clear-error auth chain,
known_hosts verification) ✅, **full `mooncake fleet bootstrap`** with
spec-44 8-step orchestration, embedded systemd unit + launchd plist,
two-stage SFTP install, daemon-reload + 10s `/v1/version` startup probe,
idempotent re-bootstrap via version-match short-circuit ✅, **per-host
overlays + tag selectors** ✅, **local-apply overlay parity** (spec-51
— `mooncake apply` now auto-loads `vars/by-host/<hostname>.yml` like
`fleet apply` does) ✅, Windows agentd ✅.

**Gap**: nothing from the original 14-PR plan. The v1 success
criteria are all in master. Outstanding personal-fleet items are the
post-v1 drafted backlog (55 fleet-doctor fan-out, 56 Windows fleet
bootstrap, 58 fleet-drift) and `fleet apply <machine>` polish.

**Distance to ideal**: ~99% to the v1 "Friday-evening demo" success
criteria from the epic. **Phases A + B + C all complete**; the
interactive `fleet init` (PR13) shipped on 2026-05-15. `fleet apply`
+ `fleet status` + `fleet logs` +
`fleet discover` (with mDNS) + per-host overlays + native SSH + full
bootstrap + local-apply overlay parity + extended filter keys +
ordered-phase `fleet apply <machine>` + `fleet upgrade` all work
end-to-end against the real WSL + Windows testbed.

**Notable**: continued highest-velocity stream. Cycle landings:
spec-22 phase 6 (Cost) reshapes the agent-safety story; `fleet
upgrade` closes a day-2-ops gap; the fleet polish bundle (apply
`<machine>` + mDNS) closes the last two long-standing personal-fleet
asks. The "Friday-evening demo" success criteria are essentially
all met.

### D. Secure AI execution layer (base for agent harnesses)

**Ideal** (from VISION §7): an LLM agent has no shell, no raw file API.
Only the Mooncake typed ABI. Every mutation is dry-runnable, mediated,
reversible, audited. Agent can declare intent ("install postgres, create
user, create db") as a `transaction:` block — if step 3 fails, steps 1+2
auto-revert. Policy DSL says `deny: agent.touches("/etc/passwd")`. Plans
are signed; daemon refuses unsigned ones in prod. Per-action quotas +
egress policy. Deterministic replay for debugging. The MCP server exposes
all of this as agent tools.

**What's shipped**: MCP server + agent loop, structured JSONL +
errors, plan-mode with content diffs, snapshot/diff, run audit,
SSE event stream, secret redaction, four-method ABI (`Permissions` +
`Diff` + `Reverse` + `Cost`) declared across priority handlers
**and wired through MCP** (spec-22 phase 7), spec-23 `on_change` /
`!secret` / `try/catch/finally`, spec-30 `transaction:` with LIFO
rollback (demo: `examples/transactions/rollback-demo.yml`).

**Gap**: spec-22 phase 8 (docs). Policy DSL (`deny:` patterns), plan
signing, per-action quotas, egress policy, sandbox mode, deterministic
replay, and a risk-scoring layer on top of `Cost()` — none specced.

**Distance to ideal**: ~80%, up from ~75%. The four-method ABI is
fully declared (Permissions + Diff + Reverse + Cost), `transaction:`
blocks execute with LIFO rollback, and the rollback demo is real. The
agent-safety pitch on the README is now backed by runnable code. What
remains is wiring (phase 7), docs (phase 8), and the policy/quota/
sandbox/signing layers — all incremental on top of the working
foundation.

---

## 3. The honest strategic picture

Mooncake has built the **kernel** (Stream 1: production-quality), the
**fleet runtime** (Stream 3: **14/14, Phase C complete**, plus three
operational features beyond the original plan — apply `<machine>` +
fleet upgrade (Linux + Windows) + fleet doctor probe ladder — and
three shipped Tier-1 QoL specs (52 exec, 53 watch, 54 ps)), the
**DX funnel** (Stream 4:
shipped), and the **agent safety layer** (Stream 2: spec-22 phases
3-7 all done, MCP wires `Diff`/`Cost`/`Permissions`; spec-23 **all
three sections shipped**; spec-30 PRs A + B in master — the rollback
demo runs). The four-method ABI is declared **and consumed through
MCP**, the rollback demo is real code, and `try/catch/finally` is
no longer drafted.

The rev-13 top-5 from `analysis/top-5-priorities-2026-05.md` (spec-22,
spec-30, fleet PR 8/9/10, spec-23) is fully closed; the rev-15
changelog block and the compressed older-revs line at the top hold
the per-item detail. The
remaining forward-looking bet is **spec-58 fleet-drift** — drafted
but not implemented — which would extend the agent-safety story
from single-machine (transactions + rollback) to fleet-scale (drift
detection + opt-in reapply/revert).

The unfair-advantage statement from VISION §13.10 — *"plan + snapshot
+ reverse + deterministic replay, all typed"* — is mostly load-
bearing now. Three of four are in master and demoable. Deterministic
replay is the last open piece on that line.

The strategic constraint stays where rev11–rev12 put it — at
**users**, not code. Code is shipping faster than the lighthouse-user
funnel can absorb. The next bottleneck is adoption, not engineering.
