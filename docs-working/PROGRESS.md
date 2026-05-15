# Mooncake — Streams Progress & Ideal-State Report

Generated from `VISION.md`, `ROADMAP.md`, and the freshest `docs-working/` state
(master @ `4e1fe89`, 2026-05-15, revision 11).

> **What changed since revision 10 — THE AGENT-SAFETY PIVOT IS SHIPPED.**
> Spec-22 phase 5 slices E (`09c315b`/`0170893`) + F (`757c431`/`c001475`)
> closed `Reverse()` across every priority handler (text family + pkg +
> download + unarchive + service). Spec-30 PR B (`15cdc79`/`dd097ea`)
> shipped the executor LIFO rollback state machine + on_rollback gating
> + the rollback-demo example. The README rewrite (`4f5f239`/`4e1fe89`)
> closed the marketing/reality gap that the brainstorm doc flagged as
> the strategic motivation. The demo claim — *"agent edits N files,
> Kth fails, mooncake auto-reverts"* — is now runnable via
> `examples/transactions/rollback-demo.yml`. 4 of the 5 success criteria
> in `next-priorities-2026-05.md` are met; only lighthouse-user
> acquisition (Path X in the post-pivot section) remains. The
> strategic constraint has shifted from **code** to **users**.

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
| **22** | **Extended Handler ABI (`Diff`/`Reverse`/`Cost`/`Permissions`)** | **🟡 in progress** — phases 1+2 ✅, 3 ✅ (`Permissions()` across 5/5 families), 4 ✅ (`Diff()` across 5/5 families + JSON plan-output wiring), **phase 5 slices A+B+C+D ✅** — `Reverse()` now covers `file.write` (create + modify), `file.write` link/hardlink modes, `file.copy`, and `file.template`. **The file family of `Reverse()` is largely complete.** Phase 5 slice E in flight in `worktree-spec-22-phase5e` (likely text family + pkg + os.service + file.download/unarchive). Phases 6–8 (`Cost`, planner/MCP wiring, docs) still draft. |

**Verdict**: very wide, and the ABI is finally landing. Action breadth no
longer the bottleneck — `Reverse()` is. Phase 3 needs to finish, then
phases 4–6 (Diff/Reverse/Cost) unblock spec-30.

### Stream 2 — Safe Agent Runtime  *(the defensible wedge)*

| Spec | Topic | State |
|---|---|---|
| 22 | Extended Handler ABI | 🟡 in progress (see Stream 1) — phases 3 + 4 + **phase 5 slices A–D ✅ (Reverse on the file family largely complete)**; slice E in flight; phase 6 (`Cost()`) + 7–8 (wiring + docs) still draft |
| 23 | Framework primitives (`on_change`, `try/catch/finally`, `!secret`) | **§1 (`on_change`) ✅**, **§3 (`!secret`) ✅** + plan-output redaction polish, **§2 (`try/catch/finally`) still drafted** — semantically overlaps with spec-30 transactions, design must align |
| 30 | `transaction:` blocks with auto-reverse | **PR A ✅** — transaction parser + plan-time reversibility check (`7c2c00e`/`e3276e0`). PR B+ (execution semantics + reverse-on-failure orchestration) presumably next in `worktree-spec-30-parser`. |

Plus a list of unwritten future specs in `streams.md`: policy DSL, plan
signing, per-action quotas, egress policy, sandbox mode, cost classifier,
deterministic replay.

**Verdict**: `Reverse()` is largely in the tree for the file family
(slices A–D), and **the `transaction:` parser is also in master**
(spec-30 PR A). What's left is slice E (`Reverse()` on the remaining
handlers) + spec-30 execution semantics + reverse-on-failure
orchestration. The agent-safety demo is one PR away from being a
falsifiable claim — *"agent edits 4 files, third fails, mooncake
auto-reverts the first two"* becomes executable the moment spec-30
PR B's executor lands.

### Stream 3 — Fleet & Cluster Management  *(the monetizable wedge)*

Personal Fleet (sub-stream): **12/14 PRs shipped end-to-end** as of
2026-05-15.

| Phase | PRs | State |
|---|---|---|
| **A** (one peer end-to-end) | 1–5 | ✅ all shipped |
| **B** (real fleet) | 6 multiplexer ✅, 7 status ✅, 8 logs/facts ✅, 9 native SSH driver ✅, 10 installer templates + 8-step bootstrap ✅, 11 bootstrap/pair ✅ (auto-flipped when PR 10 landed) | ✅ complete |
| **C** (polish) | 12 mDNS ⏳, 13 `fleet init` ⏳, 14 overlays/tags ✅ | 1/3 |

**Post-PR-14 follow-up specs** (not in original 14-PR plan, drafted from real-world use):

| Spec | Topic | State |
|---|---|---|
| 50 | Extended filter keys (`os=`, `name=`, `role=`) for `--peer-filter` / `name=` for `--step-filter` | ✅ shipped (`57686d1`/`e445a64`). Generalises spec-48's `tag=`-only DSL |
| 51 | Local-apply overlay parity — `mooncake apply` auto-loads `vars/by-host/<hostname>.yml` | ✅ shipped (`4d6b2a1`) — DX bundle |
| 45 simple | `mooncake fleet discover` — probe `peers.toml` + `~/.ssh/config` against `/v1/version` | ✅ shipped (`f49930b`) — DX bundle. Pragmatic subset of spec-45 (no mDNS, no `fleet init`, no daemon changes) |

Sidecars merged this cycle: **spec-49 agentd-on-Windows** (TCP-only mode,
SSE race fixes), a fleet polish PR (output + peer-filter UX + Windows
config paths), and a `--plan-dir` flag.

**Verified against a real WSL + Windows two-peer testbed** including
`running`/`failed`/`unreachable` health states. This isn't slideware.

Enterprise sub-stream (C1–C5 hub epics): **zero specs**, deferred. Per
`next-priorities`, intentionally not now.

**Verdict**: closest stream to "lovable v1" for its target audience.
**`mooncake fleet discover` (the simple form from spec-45)** just
shipped — probes hosts from `peers.toml` + `~/.ssh/config` against
`/v1/version` and prints a status table; no mDNS, no `fleet init`
ceremony, no new daemon work. The remaining gap (mDNS + interactive
`fleet init`) is pure polish and explicitly deferred.

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

**New work filed**: `requests/request-apply-machine-multi-peer.md` — a user
request for `mooncake apply <machine>` (ordered Windows+WSL multi-peer
apply with phase prefixing and fail-fast). Workaround exists as a per-repo
script; the ask is to ship it upstream. Not yet specced.

**Verdict**: the gap from "kernel-only, hand-write YAML" to "Mooncake feels
like a real tool" is closed. Next DX increment is the "one machine,
ordered phases" UX from the new request doc.

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

**Gap**: mDNS auto-advertise ⏳ (the simple discover doesn't need it,
but the larger spec-45 still does). Interactive `fleet init` flow ⏳.
The ordered-phase `mooncake apply <machine>` UX (Windows+WSL) ⏳ —
filed as a user request. **Spec-50** (extended filter keys: `os=`,
`name=`, `role=`) drafted but not yet shipped.

**Distance to ideal**: ~95% to the v1 "Friday-evening demo" success
criteria from the epic. **Phase A and Phase B both complete**; Phase C
1/3 plus the simple-discover bonus. `fleet apply` + `fleet status` +
`fleet logs` + `fleet discover` + per-host overlays + native SSH + full
bootstrap + local-apply overlay parity all work end-to-end against the
real WSL + Windows testbed.

**Notable**: continued highest-velocity stream — three more PRs in this
cycle (spec-51, spec-45-simple, the DX bundle merge). The "Friday-
evening demo" success criteria are essentially met sans the polish
items (mDNS auto-advertise and the interactive `fleet init`).

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

**What also just shipped (rev10)**:

- **`Reverse()` across the file family** ✅ (spec-22 phase 5 slices A+B+C+D) — `file.write` create + modify + link/hardlink modes, `file.copy`, `file.template` all have a working `Reverse()` that produces the inverse Step. **Slice E in flight** for the remaining handlers.
- **`transaction:` parser + plan-time reversibility check** ✅ (spec-30 PR A) — the planner now understands `transaction:` blocks and refuses to plan one containing irreversible steps unless `allow_irreversible: true` is set. **PR B+ (execution semantics + reverse-on-failure orchestration)** is presumably next.

**Gap**:

- **`Reverse()` across remaining handlers** (text family, pkg, os.service, file.download, file.unarchive): in flight in `worktree-spec-22-phase5e`. Once landed, all 5/5 priority handler families have `Reverse()`.
- **Spec-30 PR B+** (transaction execution + reverse-on-failure orchestration): not in master yet. **The single biggest gap between today and the agent-safety demo.**
- **`Cost()`**: not implemented yet. Spec-22 phase 6.
- **Planner / MCP wiring of `Diff` + `Cost`**: not implemented yet. Spec-22 phase 7.
- **`transaction:` blocks**: **🟡 parser in flight** (`worktree-spec-30-parser`). Unblocked by phase 5 slice A. Execution semantics + reverse-on-failure orchestration still ahead.
- **`try/catch/finally`**: still drafted (spec-23 §2). Overlaps semantically with spec-30 transactions; design must align.
- **Policy DSL** (`deny:` patterns): not specced. Hooks now exist via `Permissions`.
- **Plan signing** (Sigstore-style): not specced.
- **Per-action quotas + egress policy**: not specced.
- **Sandbox mode** (agent loses shell entirely): not specced.
- **Deterministic replay**: implicit via run audit but no `replay` command.
- **Cost / risk classifier**: not specced.

**Distance to ideal**: ~75%, up from ~70%. `Reverse()` is now shipped
across most of the file family (slices A–D), and **the `transaction:`
parser is in master** with plan-time reversibility checking. The
agent-safety demo is no longer "waiting on Reverse"; it's **waiting
on spec-30 PR B (the executor)** — one PR away.

---

## 3. The honest strategic picture

Mooncake has built the **kernel** (Stream 1: production-quality), the
**fleet runtime** (Stream 3: 12/14, **Phase B complete**, plus the
simple `fleet discover` bonus + spec-50 extended filter keys,
live-tested), and the **DX funnel** (Stream 4: shipped). The **agent
safety layer** is the primary track: spec-22 phases 3 and 4 are fully
shipped, phase 5 slices A–D are in master (Reverse across the file
family), spec-23 §1 and §3 are live, and **spec-30 PR A — the
`transaction:` parser — just shipped** (`e3276e0`). The
`transaction:` keyword is now a real thing the planner understands,
with a plan-time reversibility check that refuses irreversible
contents without explicit opt-in.

`analysis/top-5-priorities-2026-05.md` (filed 2026-05-14) named the
ordering. As of rev10 the picture is:

1. **Spec-22** — phases 1+2 ✅, phase 3 ✅, phase 4 ✅, **phase 5
   slices A–D ✅** (Reverse on file family), slice E 🟡 in flight
   (remaining handlers), phases 6–8 still draft.
2. **Spec-30** — `transaction:` blocks. **PR A ✅** (parser + plan-time
   reversibility check). **PR B+** (execution semantics +
   reverse-on-failure orchestration) presumably next in
   `worktree-spec-30-parser`. **One PR from the agent-safety demo.**
3. **Personal-fleet PR 8** — `fleet logs` + `fleet facts`. ✅ shipped.
4. **Personal-fleet PR 9 + PR 10** — native SSH driver + systemd/launchd
   installer. **Both ✅ shipped.** PR 11 auto-promoted from 🟡 lite to ✅.
5. **Spec-23** — framework primitives. §1 (`on_change`) ✅, §3
   (`!secret`) ✅, §2 (`try/catch/finally`) drafted.

Plus three bonuses delivered outside the top-5: **spec-51 (local-apply
overlay parity) ✅**, **spec-45 simple (`mooncake fleet discover`) ✅**
(the small DX win proposed mid-conversation; landed as the smallest
useful subset of spec-45), and the **DX bundle merge** that packaged
them.

`next-priorities-2026-05.md` recommends **finish-then-pivot**. Track B
(personal-fleet close-out) is effectively done — spec-50 just shipped
too, so only Phase C polish (mDNS auto-advertise, interactive `fleet
init`) and the `mooncake apply <machine>` request remain. Track A is
**demoable-soon**: 13+ merges across five sessions; spec-22 phases
1–4 + 5A–D ✅; spec-23 §1 + §3 ✅; spec-30 PR A ✅. **All that
remains for the killer demo is spec-30 PR B (executor) + the
remaining `Reverse()` handlers (slice E).** When those ship the
agent-safety pitch becomes a falsifiable demo: *"agent edits 4 files,
third fails, mooncake auto-reverts the first two."*

The unfair-advantage statement the VISION leaves open (§13.10) is
**now load-bearing**: *"plan + snapshot + reverse + deterministic
replay, all typed."* Reverse is mostly in the tree; transactions
parse; the rest is incremental.

The strategic question keeps collapsing: now it's just **"how soon
does spec-30 PR B (the executor) ship."** When it does, mooncake has
the headline agent-safety demo on its README — falsifiable, not
aspirational.
