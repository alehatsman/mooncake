# Mooncake — Streams Progress & Ideal-State Report

Generated from `VISION.md`, `ROADMAP.md`, and the freshest `docs-working/` state
(master @ `2ee98e7`, 2026-05-15, revision 14).

> **What changed since revision 13**: two more Stream-1 specs landed.
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

> **What changed since revision 12**: a large catch-up rev — eight
> meaningful landings on top of rev12, plus three new drafted specs.
> **Spec-22 phase 7 shipped** (`92b58d8`/`1d43a48`) — `Diff`, `Cost`,
> and `Permissions` are now surfaced through the MCP `check_plan` and
> `run_plan` tool responses, so agents consume the four-method ABI
> without parsing prose. Only phase 8 (docs) remains on spec-22.
> **Spec-23 §2 `try/catch/finally`** (`f598238`/`7b4d62a`) — compound
> steps landed, closing the last drafted section of spec-23 and the
> semantic-overlap design question with spec-30 transactions. **Fleet
> doctor matured into a real diagnostic loop**: per-peer probe ladder
> with curated hints (`81be15b`), per-peer last-seen persistence
> (`16e54d2`), transport-error classification (`00c2c48`), optional
> SSH-fallback diagnostic for unreachable peers (`57cc10e`), and final
> wiring of the SSH fallback into the doctor ladder (`35c9897`). The
> "peer is unreachable" question now has structured answers.
> **`mooncake fleet upgrade` Windows path** (`fac72cc`/`7f5ac24`) +
> cross-OS guard (`72c6a2d`/`9c423c2`) — the upgrade story is now
> Linux + Windows. **Architecture snapshot tooling + commit-gate
> hooks + idempotent docs** (`c259d50`/`208b1e5`) — infrastructure for
> keeping `docs-next/generated/` in sync with code. **Spec-56**
> (Windows fleet bootstrap, `56203fd`), **spec-57** (`windows_firewall_rule`
> + `windows_scheduled_task` actions, `c6f6838`), and **spec-58**
> (fleet-drift, `d963c25`) are newly drafted. The latter is the
> highest-leverage candidate from GitHub **issue #11** — a 20-item
> cluster-management brainstorm — mapped against the codebase in
> [`clustermanagement/issue-11-analysis.md`](clustermanagement/issue-11-analysis.md).
> The strategic constraint stays where rev11/rev12 put it — at
> **users**, not code.

---

## 1. Streams progress

Mooncake organises work into five parallel streams. Snapshot below.

### Stream 1 — Action Surface  *(kernel completeness)*

The typed mutation vocabulary. Ships everywhere.

| Spec | Topic | State |
|---|---|---|
| 24 | `pkg.*` (install/remove/repo/hold/upgrade/list) | P1–P6 shipped (P6: ABI hooks across pkg_repo / pkg_hold / pkg_list / pkg_upgrade). P7 (docs) pending |
| 25 | `text.line` · `text.patch.{ini,json,yaml}` | P1–P4 shipped, P5 (ABI hooks) waits on spec-22 |
| 26 | `git.clone` (+creds/submodules) · `git.checkout` · `git.config` | P1–P5 shipped (P5: spec-22 ABI hooks across all three handlers + new `ResourceGit` kind). P6 (docs) pending |
| 27 | `os.user` · `os.group` · `os.ssh_key` | P1–P4 shipped (P4: spec-22 ABI hooks across all three). P5 (docs) pending |
| 28 | `os.cron` · `os.sysctl` · `os.systemd` · `os.mount` · `os.firewall` | P1–P6 shipped (ufw driver only for firewall; P6: spec-22 ABI hooks across all five handlers) |
| 17 | Batched packages + templated `names` | shipped |
| 37 | Step output capture (collision + plan-mode) | ✅ shipped (`901e013`/`2ee98e7`). `CaptureInPlan` capability + `for_each`-aware collision warning + new plan-mode capture path; framework-only, zero new YAML surface |
| 38 | `read.json` / `read.yaml` | ✅ shipped (`8549c33`/`2ee98e7`). Tier-1 read-only actions; `CaptureInPlan: true`; `query:` (pathquery), `max_bytes:`, `redact:`; read.yaml rejects multi-document files |
| 32 | Collapse step action dispatch | not started |
| **22** | **Extended Handler ABI (`Diff`/`Reverse`/`Cost`/`Permissions`)** | **🟡 in progress** — phases 1+2 ✅, **3 ✅** (`Permissions()` across 5/5 families), **4 ✅** (`Diff()` across 5/5 families + JSON plan-output wiring), **5 ✅** (`Reverse()` across the full priority handler set: file family + text family + pkg + os.service + download + unarchive, slices A–F all merged), **6 ✅** (`Cost()` across all 15 handlers + plan JSON + recap-line surface, `6469608`/`7d382f5`), **7 ✅** (`92b58d8`/`1d43a48` — `Diff`/`Cost`/`Permissions` wired into MCP `check_plan` and `run_plan` responses so agents consume structural deltas + cost + permission requirements directly). Only phase **8** (docs) remains on spec-22. |

**Verdict**: the four-method ABI contract (`Permissions`/`Diff`/`Reverse`/`Cost`) is **fully declared** across the priority handler set **and wired through MCP**. Action breadth no longer the bottleneck; ABI breadth no longer the bottleneck either; consumer-side wiring no longer the bottleneck either. What's left is phase 8 (docs).

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
| **C** (polish) | **12 mDNS ✅** (advertise + browse), **13 `fleet init` ✅** (`f3d64c9` — interactive flow), 14 overlays/tags ✅ | ✅ complete |
| **Post-plan QoL** | `fleet exec` (spec-52, `7e855b0`), `fleet ps` (spec-54, `df3d4dd`), `fleet watch` (spec-53, `e569ad3`), `fleet upgrade`, `fleet doctor` ladder, `fleet apply <machine>`, mDNS slice | ✅ shipped on top |

**Post-PR-14 follow-up specs** (not in original 14-PR plan, drafted from real-world use):

| Spec | Topic | State |
|---|---|---|
| 50 | Extended filter keys (`os=`, `name=`, `role=`) for `--peer-filter` / `name=` for `--step-filter` | ✅ shipped (`57686d1`/`e445a64`). Generalises spec-48's `tag=`-only DSL |
| 51 | Local-apply overlay parity — `mooncake apply` auto-loads `vars/by-host/<hostname>.yml` | ✅ shipped (`4d6b2a1`) — DX bundle |
| 45 simple | `mooncake fleet discover` — probe `peers.toml` + `~/.ssh/config` against `/v1/version` | ✅ shipped (`f49930b`) — DX bundle. Now augmented by the mDNS slice (`70476f6`) so discover also picks up `_mooncake._tcp.local` responders on the LAN |
| 45 mDNS | Daemon `_mooncake._tcp.local` advertise + controller browse | ✅ shipped (`70476f6`/`beb495e`). agentd advertises on TCP bind, `fleet discover` merges responders. `--no-mdns` / `--name` flags on agentd, `--no-mdns` / `--mdns-timeout` on discover |
| — | `mooncake fleet apply <machine>` — ordered multi-peer apply via `machines/<name>/fleet.yml` | ✅ shipped (`35f21a9`/`beb495e`). Closes `requests/request-apply-machine-multi-peer.md`. Phases run sequentially with fail-fast; manifests live in the dotfiles repo |
| — | `mooncake fleet upgrade` — push new agentd binary fleet-wide | ✅ shipped Linux (`534044b`/`96d3bfb`), **Windows** (`fac72cc`/`7f5ac24`), and cross-OS guard (`72c6a2d`/`9c423c2`). Self-replace via MoveFile + scheduled task on Windows; no re-bootstrap needed on either OS |
| — | `mooncake fleet doctor` — per-peer probe ladder + SSH fallback | ✅ shipped as a real diagnostic loop (`81be15b` ladder + `16e54d2` last-seen + `00c2c48` error classification + `57cc10e` SSH fallback diag + `35c9897` ladder wiring). "Peer unreachable" now has structured answers, including a fallback channel for misconfigured agentds |
| 52–55 | Tier-1 fleet QoL — `exec` / `watch` / `ps` / `doctor`-fleetwide | 📝 drafted (`e71b57e`). Brainstormed in `clustermanagement/qol-features.md` |
| 56 | Windows fleet bootstrap | 📝 drafted (`56203fd`) |
| **58** | **Fleet drift** — periodic `InspectPlan` loop + `/v1/drift` + `mooncake fleet drift` + per-machine `drift:` policy block | **📝 drafted (`d963c25`)**. Highest-leverage candidate from GitHub issue #11; see [`clustermanagement/issue-11-analysis.md`](clustermanagement/issue-11-analysis.md) for the full 20-item map |

Sidecars merged earlier: **spec-49 agentd-on-Windows** (TCP-only mode,
SSE race fixes), a fleet polish PR (output + peer-filter UX + Windows
config paths), and a `--plan-dir` flag.

**Verified against a real WSL + Windows two-peer testbed** including
`running`/`failed`/`unreachable` health states. This isn't slideware.

Enterprise sub-stream (C1–C5 hub epics): **zero specs**, deferred. Per
`next-priorities`, intentionally not now.

**Verdict**: the stream is **done for v1 + has a forward backlog**.
v1 is closed: mDNS discovery, `apply <machine>`, fleet upgrade
(Linux + Windows), and a real `fleet doctor` ladder all shipped.
Phase C still has the interactive `fleet init` UX left, pure polish.
The drafted-spec backlog is now seven items (52, 53, 54, 55, 56, 58
+ the issue-11 brainstorm); **spec-58 fleet-drift** is the
strategically heaviest of the seven and would turn Mooncake from
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
with mDNS landed, plus follow-up specs and three operational features —
apply `<machine>` + fleet upgrade (Linux+Windows) + fleet doctor probe
ladder — beyond the original plan), the **DX funnel** (Stream 4:
shipped), and the **agent safety layer** (Stream 2: spec-22 phases
3-7 all done, MCP wires `Diff`/`Cost`/`Permissions`; spec-23 **all
three sections shipped**; spec-30 PRs A + B in master — the rollback
demo runs). The four-method ABI is declared **and consumed through
MCP**, the rollback demo is real code, and `try/catch/finally` is
no longer drafted.

`analysis/top-5-priorities-2026-05.md` (filed 2026-05-14) named the
ordering. As of rev13 the picture is:

1. **Spec-22** — phases 1+2 ✅, 3 ✅, 4 ✅, 5 ✅, **6 ✅**, **7 ✅**
   (`Diff`/`Cost`/`Permissions` surfaced through MCP `check_plan` and
   `run_plan`). Only phase 8 (docs) remains.
2. **Spec-30** — `transaction:` blocks with auto-reverse. **PR A + PR
   B both ✅** — parser, plan-time reversibility check, executor, LIFO
   rollback, on_rollback gating, runnable demo. The headline claim is
   no longer aspirational.
3. **Personal-fleet PR 8** — `fleet logs` + `fleet facts`. ✅ shipped.
4. **Personal-fleet PR 9 + PR 10** — native SSH driver + systemd/launchd
   installer. ✅ both shipped. PR 11 auto-promoted to ✅.
5. **Spec-23** — framework primitives. **All three sections shipped**:
   §1 `on_change` ✅, §3 `!secret` ✅, **§2 `try/catch/finally` ✅**
   (`f598238`/`7b4d62a`).

Plus the bonuses delivered outside the top-5: **spec-51 (local-apply
overlay parity) ✅**, **spec-45 simple + mDNS slice ✅**, **spec-50
(extended filter keys) ✅**, **`mooncake fleet apply <machine>` ✅**,
**`mooncake fleet upgrade` (Linux + Windows) ✅**, and **`mooncake
fleet doctor` probe ladder + SSH fallback ✅**. New drafted-spec
backlog since rev12: **spec-56** (Windows fleet bootstrap),
**spec-57** (Windows firewall + scheduled task actions), and
**spec-58 fleet-drift** — the latter picked from a 20-item issue #11
brainstorm as the highest-leverage cluster-management gap.

`next-priorities-2026-05.md` recommends **finish-then-pivot**. Track B
(personal-fleet close-out) is *done* — Phase C 2/3 with only the
interactive `fleet init` ⏳ left, and that's pure UX polish. Track A
(agent-safety) shipped its headline demo via spec-30 PR B **and** the
MCP wiring via spec-22 phase 7; the spec-22 backlog is down to docs
(phase 8). The policy/quota/signing layers remain un-specced. The
next forward-looking bet is **spec-58 fleet-drift** — drafted but not
implemented — which would extend the agent-safety story from
single-machine (transactions + rollback) to fleet-scale (drift
detection + opt-in reapply/revert).

The unfair-advantage statement from VISION §13.10 — *"plan + snapshot
+ reverse + deterministic replay, all typed"* — is mostly load-
bearing now. Three of four are in master and demoable. Deterministic
replay is the last open piece on that line.

The strategic constraint stays where rev11–rev12 put it — at
**users**, not code. Code is shipping faster than the lighthouse-user
funnel can absorb. The next bottleneck is adoption, not engineering.
