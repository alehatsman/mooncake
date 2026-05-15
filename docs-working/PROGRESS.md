# Mooncake — Streams Progress & Ideal-State Report

Generated from `VISION.md`, `ROADMAP.md`, and the freshest `docs-working/` state
(master @ `beb495e`, 2026-05-15, revision 12).

> **What changed since revision 11**: three more landings since the
> post-pivot README rewrite, all building on the demo. **Spec-22 phase 6
> shipped** (`6469608`/`7d382f5`) — `Cost()` declarations across all 15
> handlers plus JSON-plan-output and recap-line surface. With phases 3 +
> 4 + 5 + 6 now all green, the four-method ABI (`Permissions`/`Diff`/
> `Reverse`/`Cost`) is **fully declared** across the priority handler
> set; phases 7–8 (planner/MCP wiring + docs) are the only remaining
> spec-22 work. **`mooncake fleet upgrade`** (`534044b`/`96d3bfb`) lets
> the controller push a new agentd binary to Linux peers (self-replace)
> without re-running bootstrap — closes a long-standing operational
> gap. **Fleet polish bundle** (`35f21a9`+`70476f6`/`beb495e`) lands the
> `mooncake apply <machine>` ordered-multi-peer UX from the
> long-standing user request — drives Windows+WSL boxes through
> phase-prefixed sequential applies via `machines/<name>/fleet.yml` —
> *and* finishes spec-45 by adding the mDNS slice (daemon advertise on
> `_mooncake._tcp.local` + controller browse merged into `fleet
> discover`). Personal Fleet is now **13/14 PRs**; only the interactive
> `fleet init` flow remains in Phase C polish. **No active worktrees**:
> dx-bundle, fleet-polish, and spec-22-phase6 are all idle/cleanup-
> pending. The strategic constraint stays where rev11 put it — at
> **users**, not code.

---

## 1. Streams progress

Mooncake organises work into five parallel streams. Snapshot below.

### Stream 1 — Action Surface  *(kernel completeness)*

The typed mutation vocabulary. Ships everywhere.

| Spec | Topic | State |
|---|---|---|
| 24 | `pkg.*` (install/remove/repo/hold/upgrade/list) | P1–P5 shipped, P6 (ABI hooks) waits on spec-22 phase 3 |
| 25 | `text.line` · `text.patch.{ini,json,yaml}` | P1–P4 shipped, P5 (ABI hooks) waits on spec-22 |
| 26 | `git.clone` (+creds/submodules) · `git.checkout` · `git.config` | P1–P4 shipped |
| 27 | `os.user` · `os.group` · `os.ssh_key` | P1–P3 shipped |
| 28 | `os.cron` · `os.sysctl` · `os.systemd` · `os.mount` · `os.firewall` | P1–P5 shipped (ufw only) |
| 17 | Batched packages + templated `names` | shipped |
| 37 | Step output capture (collision + plan-mode) | drafted |
| 38 | `read.json` / `read.yaml` | drafted; depends on 37 |
| 32 | Collapse step action dispatch | not started |
| **22** | **Extended Handler ABI (`Diff`/`Reverse`/`Cost`/`Permissions`)** | **🟡 in progress** — phases 1+2 ✅, **3 ✅** (`Permissions()` across 5/5 families), **4 ✅** (`Diff()` across 5/5 families + JSON plan-output wiring), **5 ✅** (`Reverse()` across the full priority handler set: file family + text family + pkg + os.service + download + unarchive, slices A–F all merged), **6 ✅** (`Cost()` across all 15 handlers + plan JSON + recap-line surface, `6469608`/`7d382f5`). Phases **7–8** (planner/MCP wiring + docs) are the only remaining spec-22 work. |

**Verdict**: the four-method ABI contract (`Permissions`/`Diff`/`Reverse`/`Cost`) is **fully declared** across the priority handler set. Action breadth no longer the bottleneck; ABI breadth no longer the bottleneck either. What's left is wiring (phase 7: surface `Diff`+`Cost` through the planner and MCP server so agents can consume them) and docs (phase 8).

### Stream 2 — Safe Agent Runtime  *(the defensible wedge)*

| Spec | Topic | State |
|---|---|---|
| 22 | Extended Handler ABI | 🟡 in progress (see Stream 1) — phases 3 + 4 + 5 + **6 ✅** (Cost across all 15 handlers); only phases 7 (planner/MCP wiring) + 8 (docs) remain |
| 23 | Framework primitives (`on_change`, `try/catch/finally`, `!secret`) | **§1 (`on_change`) ✅**, **§3 (`!secret`) ✅** + plan-output redaction polish, **§2 (`try/catch/finally`) still drafted** — semantically overlaps with spec-30 transactions, design must align |
| 30 | `transaction:` blocks with auto-reverse | **PR A ✅** (parser + plan-time reversibility check, `7c2c00e`/`e3276e0`) + **PR B ✅** (executor + LIFO rollback + on_rollback gating, `15cdc79`/`dd097ea`). **The agent-safety demo is runnable** via `examples/transactions/rollback-demo.yml`. |

Plus a list of unwritten future specs in `streams.md`: policy DSL, plan
signing, per-action quotas, egress policy, sandbox mode, cost classifier,
deterministic replay.

**Verdict**: the ABI is closed in shape (4/4 methods across the
priority set; phase 6 just shipped `Cost()`). `transaction:` blocks
work end-to-end. **The headline demo is no longer aspirational** — the
README's auto-revert claim runs from a real example. What remains for
spec-22 is wiring (phase 7) — surface `Diff` + `Cost` through the
planner and MCP so agents and UIs consume the structural deltas
without parsing prose.

### Stream 3 — Fleet & Cluster Management  *(the monetizable wedge)*

Personal Fleet (sub-stream): **13/14 PRs shipped end-to-end** as of
2026-05-15.

| Phase | PRs | State |
|---|---|---|
| **A** (one peer end-to-end) | 1–5 | ✅ all shipped |
| **B** (real fleet) | 6 multiplexer ✅, 7 status ✅, 8 logs/facts ✅, 9 native SSH driver ✅, 10 installer templates + 8-step bootstrap ✅, 11 bootstrap/pair ✅ | ✅ complete |
| **C** (polish) | **12 mDNS ✅** (advertise + browse), 13 `fleet init` ⏳ (interactive flow), 14 overlays/tags ✅ | 2/3 |

**Post-PR-14 follow-up specs** (not in original 14-PR plan, drafted from real-world use):

| Spec | Topic | State |
|---|---|---|
| 50 | Extended filter keys (`os=`, `name=`, `role=`) for `--peer-filter` / `name=` for `--step-filter` | ✅ shipped (`57686d1`/`e445a64`). Generalises spec-48's `tag=`-only DSL |
| 51 | Local-apply overlay parity — `mooncake apply` auto-loads `vars/by-host/<hostname>.yml` | ✅ shipped (`4d6b2a1`) — DX bundle |
| 45 simple | `mooncake fleet discover` — probe `peers.toml` + `~/.ssh/config` against `/v1/version` | ✅ shipped (`f49930b`) — DX bundle. Now augmented by the mDNS slice (`70476f6`) so discover also picks up `_mooncake._tcp.local` responders on the LAN |
| 45 mDNS | Daemon `_mooncake._tcp.local` advertise + controller browse | ✅ shipped (`70476f6`/`beb495e`). agentd advertises on TCP bind, `fleet discover` merges responders. `--no-mdns` / `--name` flags on agentd, `--no-mdns` / `--mdns-timeout` on discover |
| — | `mooncake fleet apply <machine>` — ordered multi-peer apply via `machines/<name>/fleet.yml` | ✅ shipped (`35f21a9`/`beb495e`). Closes `requests/request-apply-machine-multi-peer.md`. Phases run sequentially with fail-fast; manifests live in the dotfiles repo |
| — | `mooncake fleet upgrade` — push new agentd binary fleet-wide | ✅ shipped (`534044b`/`96d3bfb`). Self-replace for Linux peers; no re-bootstrap needed |

Sidecars merged earlier: **spec-49 agentd-on-Windows** (TCP-only mode,
SSE race fixes), a fleet polish PR (output + peer-filter UX + Windows
config paths), and a `--plan-dir` flag.

**Verified against a real WSL + Windows two-peer testbed** including
`running`/`failed`/`unreachable` health states. This isn't slideware.

Enterprise sub-stream (C1–C5 hub epics): **zero specs**, deferred. Per
`next-priorities`, intentionally not now.

**Verdict**: the stream is essentially **done for v1**. mDNS auto-
advertise just shipped (so `mooncake fleet discover` finds boxes on
the LAN with zero peers.toml setup); the long-standing `mooncake
apply <machine>` Windows+WSL ordered-phase request just shipped; and
`mooncake fleet upgrade` makes day-2 ops painless. The only Phase C
item left is the interactive `fleet init` flow (PR13) — pure operator
UX, not a capability gap.

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

**Recently closed**: `requests/request-apply-machine-multi-peer.md` —
`mooncake fleet apply <machine>` shipped (`35f21a9`/`beb495e`) reading
`machines/<name>/fleet.yml` and running ordered phases with fail-fast.
The wrapper script every multi-peer dotfiles repo was reinventing
moves upstream.

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
`disk-partition-action.md` is a loose spec, not done.

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

**Gap**: Interactive `fleet init` flow ⏳ (PR13 — the only Phase C
polish item left). That's it. Everything else from the v1 success
criteria is in master.

**Distance to ideal**: ~98% to the v1 "Friday-evening demo" success
criteria from the epic. **Phase A and Phase B complete**; **Phase C
2/3** (mDNS now ✅). `fleet apply` + `fleet status` + `fleet logs` +
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

**What's shipped**:

- **MCP server** with `run_step` / `get_facts` / `get_snapshot` /
  `check_plan` / `run_plan` ✅ (`internal/mcp/`)
- **Agent loop** for iterate-until-done (`internal/agent/`) ✅
- **Structured JSONL output** + structured errors with suggested fixes ✅
- **`mooncake plan` dry-run** with content diffs ✅
- **Snapshot + diff** at file/system level ✅
- **Run audit trail** (JSONL with run IDs, ULID-ordered) ✅
- **agentd async submit + SSE event stream** ✅
- **Secret redaction** ✅
- **Extended Handler ABI contract** ✅ (spec-22 phases 1+2)
- **`Permissions()` declaration + executor preflight** ✅ across **5/5
  priority handler families**: file (`file.write`, `file.template`,
  `file.copy`, `file.download`, `file.unarchive`), text (`text.line`,
  `text.patch.{ini,json,yaml}`), `pkg`, `os.service`, plus executor
  preflight — sudo and required-binary checks fail fast with typed
  errors agents can catch. **Spec-22 phase 3 fully complete.**
- **`Diff()` declaration** ✅ across all 5/5 priority handler families
  (phases 4a–4e, including `file.download` + `file.unarchive`) plus the
  phase 4 follow-up wiring `Diff()` into JSON plan output (`bb082a1`).
  Structural deltas a UI or LLM can branch on without parsing prose.
  **Spec-22 phase 4 fully complete.**
- **`on_change:` reactive triggers** ✅ (spec-23 §1) — a step re-runs
  when a watched step changes a thing.
- **`!secret` tag + env provider** ✅ (spec-23 §3) — secrets pulled
  from env, masked in logs, and now also **redacted from plan output
  by default** (`b019805` polish), never written to disk.

**What's shipped since rev11**:

- **`Reverse()` across the full priority handler set** ✅ — slices E (text family, `0170893`) + F (categoricals: download, pkg, unarchive, os.service, `c001475`) closed the rollout.
- **Spec-22 phase 6: `Cost()` on all 15 handlers** ✅ — `6469608`/`7d382f5`. Plan output and recap line now surface per-step cost classifications. Last piece of the four-method ABI.
- **`transaction:` executor + LIFO rollback** ✅ — spec-30 PR B, `15cdc79`/`dd097ea`. The agent-safety demo runs from `examples/transactions/rollback-demo.yml`.

**Gap**:

- **Planner / MCP wiring of `Diff` + `Cost`** ⏳ — spec-22 phase 7. The methods are declared on every handler; the planner and MCP server still need to *surface* them so agent consumers can branch on structural deltas and predicted cost without parsing prose.
- **Spec-22 phase 8** ⏳ — docs.
- **`try/catch/finally`** — still drafted (spec-23 §2). Overlaps semantically with spec-30 transactions; design must align before code.
- **Policy DSL** (`deny:` patterns): not specced. Hooks now exist via `Permissions`.
- **Plan signing** (Sigstore-style): not specced.
- **Per-action quotas + egress policy**: not specced.
- **Sandbox mode** (agent loses shell entirely): not specced.
- **Deterministic replay**: implicit via run audit but no `replay` command.
- **Cost / risk classifier on top of `Cost()`**: not specced (the per-handler `Cost()` provides the input; an aggregation/risk-scoring layer is the next piece).

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
**fleet runtime** (Stream 3: **13/14**, Phase B complete, Phase C 2/3
with mDNS just landed, plus four follow-up specs and two operational
features — apply `<machine>` + fleet upgrade — beyond the original
plan), the **DX funnel** (Stream 4: shipped), and the **agent safety
layer** (Stream 2: spec-22 phases 3 + 4 + 5 + **6** all done; spec-23
§1 + §3 live; spec-30 PRs A + B in master — the rollback demo runs).
The four-method ABI is declared and the killer transaction demo is
real code.

`analysis/top-5-priorities-2026-05.md` (filed 2026-05-14) named the
ordering. As of rev12 the picture is:

1. **Spec-22** — phases 1+2 ✅, 3 ✅, 4 ✅, 5 ✅ (Reverse across the
   full priority handler set), **6 ✅** (Cost on all 15 handlers +
   plan/recap surface). Phases 7 (wiring) + 8 (docs) remain.
2. **Spec-30** — `transaction:` blocks with auto-reverse. **PR A + PR
   B both ✅** — parser, plan-time reversibility check, executor, LIFO
   rollback, on_rollback gating, runnable demo. The headline claim is
   no longer aspirational.
3. **Personal-fleet PR 8** — `fleet logs` + `fleet facts`. ✅ shipped.
4. **Personal-fleet PR 9 + PR 10** — native SSH driver + systemd/launchd
   installer. ✅ both shipped. PR 11 auto-promoted to ✅.
5. **Spec-23** — framework primitives. §1 (`on_change`) ✅, §3
   (`!secret`) ✅, §2 (`try/catch/finally`) drafted.

Plus the bonuses delivered outside the top-5: **spec-51 (local-apply
overlay parity) ✅**, **spec-45 simple + mDNS slice ✅**, **spec-50
(extended filter keys) ✅**, **`mooncake fleet apply <machine>` ✅**,
and **`mooncake fleet upgrade` ✅**.

`next-priorities-2026-05.md` recommends **finish-then-pivot**. Track B
(personal-fleet close-out) is *done* — Phase C 2/3 with only the
interactive `fleet init` ⏳ left, and that's pure UX polish. Track A
(agent-safety) shipped its headline demo via spec-30 PR B; what
remains is wiring (spec-22 phase 7 — surface `Diff` and `Cost`
through the planner and MCP) and the policy/quota/signing layers
that are still un-specced.

The unfair-advantage statement from VISION §13.10 — *"plan + snapshot
+ reverse + deterministic replay, all typed"* — is mostly load-
bearing now. Three of four are in master and demoable. Deterministic
replay is the last open piece on that line.

The strategic constraint stays where rev11 put it — at **users**, not
code. Code is shipping faster than the lighthouse-user funnel can
absorb. The next bottleneck is adoption, not engineering.
