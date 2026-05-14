# Mooncake — Streams Progress & Ideal-State Report

Generated from `VISION.md`, `ROADMAP.md`, and the freshest `docs-working/` state
(master @ `b019805`, 2026-05-15, revision 8).

> **What changed since revision 7**: **DX bundle shipped** (`bd61319`)
> — **spec-51 (local-apply overlay parity, `4d6b2a1`)** and a **simple
> `mooncake fleet discover` (spec-45 simple, `f49930b`)** that probes
> hosts from `peers.toml` + `~/.ssh/config` against `/v1/version`
> without mDNS or any new daemon work. Spec-50 (extended filter keys)
> wasn't in the bundle — left for another round. Plus a **`!secret`
> plan-output redaction polish** (`b019805`) — `!secret` refs are now
> redacted in plan output by default, closing a gap in the spec-23 §3
> surface. **The single biggest in-flight move is
> `worktree-spec-22-phase5`** — **`Reverse()`**, the primitive between
> today and the spec-30 `transaction:` demo (still no commits in that
> worktree; agent is in deep exploratory mode). Idle/cleanup-pending:
> `dx-bundle` and `plan-redact-polish` (both already merged into
> master). The earlier `spec-22-phase4f` and `tier2-secrets` worktrees
> are gone — phase 4f folded into the JSON-Diff follow-up (`bb082a1`);
> tier2-secrets stalled or was absorbed elsewhere.

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
| **22** | **Extended Handler ABI (`Diff`/`Reverse`/`Cost`/`Permissions`)** | **🟡 in progress** — phases 1+2 ✅, 3a–3d ✅ (`Permissions()`), 4a–4e ✅ (`Diff()` across all 5/5 priority handler families), plus phase 4 follow-up (`bb082a1`) wiring `Diff()` into JSON plan output. **Phases 3 + 4 fully complete.** **Phase 5 (`Reverse()`) in flight in `worktree-spec-22-phase5`** — the headline primitive that unblocks spec-30 `transaction:` blocks. Phases 6–8 (`Cost`, planner/MCP wiring, docs) still draft. |

**Verdict**: very wide, and the ABI is finally landing. Action breadth no
longer the bottleneck — `Reverse()` is. Phase 3 needs to finish, then
phases 4–6 (Diff/Reverse/Cost) unblock spec-30.

### Stream 2 — Safe Agent Runtime  *(the defensible wedge)*

| Spec | Topic | State |
|---|---|---|
| 22 | Extended Handler ABI | 🟡 in progress (see Stream 1) — phases 3 + 4 complete; **phase 5 (`Reverse()`) in flight in `worktree-spec-22-phase5`**; phase 6 (`Cost()`) and 7–8 (wiring + docs) still draft |
| 23 | Framework primitives (`on_change`, `try/catch/finally`, `!secret`) | **§1 (`on_change`) ✅**, **§3 (`!secret`) ✅**, **§2 (`try/catch/finally`) still drafted** — semantically overlaps with spec-30 transactions, design must align |
| 30 | `transaction:` blocks with auto-reverse | drafted, blocked on 22 phase 5 (`Reverse()`) |

Plus a list of unwritten future specs in `streams.md`: policy DSL, plan
signing, per-action quotas, egress policy, sandbox mode, cost classifier,
deterministic replay.

**Verdict**: the ABI contract is in the tree; `Permissions()` and
`Diff()` are both fully declared across the priority handler families;
executor preflights sudo + required-binaries today; `Diff()` provides
structural deltas a UI or LLM can branch on without parsing prose.
Stream 2 now has two of three spec-23 sections shipped (`on_change` +
`!secret`). **`Reverse()` (phase 5) is the last big gate** for the
spec-30 `transaction:` demo — the headline agent-safety claim. After
that: `Cost()`, planner/MCP wiring, and the demo assembly itself.

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
| 50 | Extended filter keys (`os=`, `name=`, `role=`) for `--peer-filter`/`--step-filter` | Draft. Generalises spec-48's `tag=`-only DSL |
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

**Gap**:

- **`Reverse()`**: **🟡 in flight in `worktree-spec-22-phase5`**. Spec-22 phase 5. **The headline primitive — the last gate between today and the agent-safety demo.**
- **`Cost()`**: not implemented yet. Spec-22 phase 6.
- **Planner / MCP wiring of `Diff` + `Cost`**: not implemented yet. Spec-22 phase 7.
- **`transaction:` blocks**: not started. Spec-30, needs `Reverse()`. The killer demo.
- **`try/catch/finally`**: still drafted (spec-23 §2). Overlaps semantically with spec-30 transactions; design must align.
- **Policy DSL** (`deny:` patterns): not specced. Hooks now exist via `Permissions`.
- **Plan signing** (Sigstore-style): not specced.
- **Per-action quotas + egress policy**: not specced.
- **Sandbox mode** (agent loses shell entirely): not specced.
- **Deterministic replay**: implicit via run audit but no `replay` command.
- **Cost / risk classifier**: not specced.

**Distance to ideal**: ~65%, up from ~60%. `Diff` is now wired into JSON
plan output (so agents can branch on structural deltas without parsing
prose). **`Reverse()` is in flight** in `worktree-spec-22-phase5` —
which means the project is actively pulling the last load-bearing
primitive into the tree. Once it ships and spec-30 `transaction:`
follows, the README's agent-safety claim becomes a falsifiable demo.

---

## 3. The honest strategic picture

Mooncake has built the **kernel** (Stream 1: production-quality), the
**fleet runtime** (Stream 3: 12/14, **Phase B complete**, plus the
simple `fleet discover` bonus from this cycle, live-tested), and the
**DX funnel** (Stream 4: shipped). The **agent safety layer** is the
primary track: spec-22 phases 3 and 4 are both fully shipped
(`Permissions()` and `Diff()` across all 5/5 priority handler
families, with `Diff` now wired into JSON plan output), spec-23 §1
(`on_change`) and §3 (`!secret`) are live in master, and **`Reverse()`
(phase 5) is in flight right now** in `worktree-spec-22-phase5`. The
strategic gap is no longer "does any agent-safety code exist" — it's
**"how soon does spec-30 ship after `Reverse()` lands."**

`analysis/top-5-priorities-2026-05.md` (filed 2026-05-14) named the
ordering. As of rev8 the picture is:

1. **Spec-22** — phases 1+2 ✅, phase 3 ✅ (Permissions across all 5/5
   families), phase 4 ✅ (Diff across all 5/5 families + JSON plan
   output wiring), **phase 5 (Reverse) 🟡 in flight**, phases 6–8 still
   draft.
2. **Spec-30** — `transaction:` blocks. The killer demo. Starts the
   moment `Reverse()` works on `file.write`.
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
(personal-fleet close-out) is effectively done — only Phase C polish
(mDNS auto-advertise, interactive `fleet init`), spec-50 (extended
filter keys), and the `mooncake apply <machine>` request remain. The
pivot has fully happened and Track A is **mid-flight at speed**: phases
3 and 4 of spec-22 are done, spec-23 §1+§3 are done, ten merges have
landed across three sessions, and **`Reverse()` is in flight right
now**. Once it lands and spec-30 `transaction:` blocks ship on top of
it, the agent-safety pitch becomes a falsifiable demo: *"agent edits 4
files, third fails, mooncake auto-reverts the first two."*

The unfair-advantage statement the VISION leaves open (§13.10) gets
answered when `transaction:` ships: **"plan + snapshot + reverse +
deterministic replay, all typed."** Once `Reverse()` lands, the answer
to that question becomes load-bearing.

The strategic question is no longer *"will the pivot happen"* or *"how
soon does `Reverse()` start"* — it's *"how soon does spec-30 ship after
`Reverse()` merges."*
