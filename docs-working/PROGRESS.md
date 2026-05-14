# Mooncake — Streams Progress & Ideal-State Report

Generated from `VISION.md`, `ROADMAP.md`, and the freshest `docs-working/` state
(master @ `9666e40`, 2026-05-15, revision 4).

> **What changed since revision 2**: **Phase B of personal-fleet is fully
> complete.** PR 9 (native SSH driver via `crypto/ssh` + `pkg/sftp` with
> ssh-agent → IdentityFiles auth chain + `known_hosts` verification) and
> PR 10 (full spec-44 §88 8-step bootstrap orchestration with embedded
> systemd unit + launchd plist, two-stage SFTP install, daemon-reload +
> startup probe, idempotent re-bootstrap via version-match short-circuit)
> both shipped. PR 11 flipped from 🟡 lite → ✅ full automatically. Two
> bug fixes also landed: tag-filter UX (flags after positional, opt-in
> tag semantics) and `step.failed` error rendering + `git.clone`
> divergent-state guard. Personal-fleet now **12/14** (only PR 12 mDNS
> and PR 13 `fleet init` remain — both Phase C polish). Spec-22 phase 3c
> (`Permissions()` across the text.\* family) also shipped (`6779638`)
> — **phase 3 is now complete**; phase 4 (`Diff()`) is the next gate
> for the agent-safety wedge.

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
| **22** | **Extended Handler ABI (`Diff`/`Reverse`/`Cost`/`Permissions`)** | **🟡 in progress** — phases 1+2 ✅ (types + sub-interfaces + safe defaults), 3a ✅ (`file.write` + executor preflight), 3b ✅ (full file family), 3c ✅ (text.* family). **Phase 3 (`Permissions()`) complete**. Phases 4–8 (Diff, Reverse, Cost, planner/MCP wiring, docs) still draft. |

**Verdict**: very wide, and the ABI is finally landing. Action breadth no
longer the bottleneck — `Reverse()` is. Phase 3 needs to finish, then
phases 4–6 (Diff/Reverse/Cost) unblock spec-30.

### Stream 2 — Safe Agent Runtime  *(the defensible wedge)*

| Spec | Topic | State |
|---|---|---|
| 22 | Extended Handler ABI | 🟡 in progress (see Stream 1) — the dependency that gates everything below |
| 23 | Framework primitives (`on_change`, `try/catch/finally`, `!secret`) | drafted, blocked on 22 |
| 30 | `transaction:` blocks with auto-reverse | drafted, blocked on 22 |

Plus a list of unwritten future specs in `streams.md`: policy DSL, plan
signing, per-action quotas, egress policy, sandbox mode, cost classifier,
deterministic replay.

**Verdict**: the ABI contract is in the tree and `Permissions()` preflight
runs in the executor today. Half of phase 3 is done. `Diff` / `Reverse` /
`Cost` are still spec, not code — they are the gating work for the
`transaction:` demo. Stream 2 has moved from "zero code" to "scaffolded."

### Stream 3 — Fleet & Cluster Management  *(the monetizable wedge)*

Personal Fleet (sub-stream): **12/14 PRs shipped end-to-end** as of
2026-05-15.

| Phase | PRs | State |
|---|---|---|
| **A** (one peer end-to-end) | 1–5 | ✅ all shipped |
| **B** (real fleet) | 6 multiplexer ✅, 7 status ✅, 8 logs/facts ✅, 9 native SSH driver ✅, 10 installer templates + 8-step bootstrap ✅, 11 bootstrap/pair ✅ (auto-flipped when PR 10 landed) | ✅ complete |
| **C** (polish) | 12 mDNS ⏳, 13 `fleet init` ⏳, 14 overlays/tags ✅ | 1/3 |

Sidecars merged this cycle: **spec-49 agentd-on-Windows** (TCP-only mode,
SSE race fixes), a fleet polish PR (output + peer-filter UX + Windows
config paths), and a `--plan-dir` flag.

**Verified against a real WSL + Windows two-peer testbed** including
`running`/`failed`/`unreachable` health states. This isn't slideware.

Enterprise sub-stream (C1–C5 hub epics): **zero specs**, deferred. Per
`next-priorities`, intentionally not now.

**Verdict**: the bulk of recent activity. Closest stream to "lovable v1"
for its target audience. The remaining gap (real bootstrap + mDNS +
`fleet init`) is well-scoped.

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

**Verdict**: nothing started; explicitly deferred per `next-priorities`.

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
logs` + `fleet facts`** ✅, parallel multi-peer multiplexer with `^C`
banner ✅, `peers.toml` + `controller_id` ✅, **native SSH driver** (crypto/ssh
+ pkg/sftp, ssh-agent → IdentityFiles → clear-error auth chain,
known_hosts verification) ✅, **full `mooncake fleet bootstrap`** with
spec-44 8-step orchestration, embedded systemd unit + launchd plist,
two-stage SFTP install, daemon-reload + 10s `/v1/version` startup probe,
idempotent re-bootstrap via version-match short-circuit ✅, **per-host
overlays + tag selectors** ✅, Windows agentd ✅.

**Gap**: mDNS discovery ⏳. `fleet init` interactive flow ⏳. The
ordered-phase `mooncake apply <machine>` UX (Windows+WSL) ⏳ — filed as
a user request.

**Distance to ideal**: ~92% to the v1 "Friday-evening demo" success
criteria from the epic. **Phase A and Phase B both complete**; Phase C
1/3. `fleet apply` + `fleet status` + `fleet logs` + per-host overlays +
native SSH + full bootstrap (`mooncake fleet bootstrap user@new-box` now
actually does the 60-second story the epic promises) all work end-to-end
against the real WSL + Windows testbed.

**Notable**: this is the stream with the most velocity right now — six
PRs and two bug fixes landed since the previous report. The "Friday-
evening demo" success criteria are essentially met sans mDNS auto-discovery
and the interactive `fleet init`.

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
- **`Permissions()` declaration + executor preflight** ✅ on the full file
  family (`file.write`, `file.template`, `file.copy`, `file.download`,
  `file.unarchive`) — sudo and required-binary checks fail fast with
  typed errors agents can catch
- **`Permissions()` on text.\* family** ✅ (spec-22 phase 3c) — phase 3 complete

**Gap — the strategic gap of the whole project**:

- **`Diff()`**: not implemented yet. Spec-22 phase 4. Structural deltas a UI / LLM can branch on.
- **`Reverse()`**: not implemented yet. Spec-22 phases 5–6. The headline primitive.
- **`Cost()`**: not implemented yet. Spec-22 phase 6.
- **`transaction:` blocks**: not started. Spec-30, needs `Reverse()`. The killer demo.
- **`try/catch/finally`, `on_change:`, `!secret`**: not started. Spec-23. `on_change` + `!secret` are independent of spec-22 and can ship in parallel.
- **Policy DSL** (`deny:` patterns): not specced. Hooks now exist via `Permissions`.
- **Plan signing** (Sigstore-style): not specced.
- **Per-action quotas + egress policy**: not specced.
- **Sandbox mode** (agent loses shell entirely): not specced.
- **Deterministic replay**: implicit via run audit but no `replay` command.
- **Cost / risk classifier**: not specced.

**Distance to ideal**: ~40%, up from ~30%. The agent *interface* is real,
the ABI contract is in the tree, and `Permissions` preflight is live for
the biggest handler family. The agent *safety* primitives that turn the
README's marketing into a demoable claim — `Diff` / `Reverse` /
`transaction:` — are still pending. **The next single biggest leverage
move in the whole codebase is shipping `Reverse()` on `file.write`.**

---

## 3. The honest strategic picture

Mooncake has built the **kernel** (Stream 1: production-quality), the
**fleet runtime** (Stream 3: 12/14, **Phase B complete**, live-tested),
and the **DX funnel** (Stream 4: shipped). It has *started* the **agent
safety layer** — phase 3 of spec-22 is fully in master (`Permissions()`
across the file and text families). The strategic gap is no longer "does any agent-safety
code exist" but "how fast does `Reverse()` + `transaction:` ship after
`Permissions` finishes."

`analysis/top-5-priorities-2026-05.md` (filed 2026-05-14) names the
ordering explicitly:

1. **Spec-22** (the strategic blocker) — phases 1+2+3a+3b shipped; 3c in
   flight; 4–8 still draft.
2. **Spec-30** — `transaction:` blocks. The killer demo. Starts the
   moment `Reverse()` works on `file.write`.
3. **Personal-fleet PR 8** — `fleet logs` + `fleet facts`. ✅ shipped.
4. **Personal-fleet PR 9 + PR 10** — native SSH driver + systemd/launchd
   installer. **Both ✅ shipped 2026-05-15.** PR 11 (`fleet bootstrap`
   CLI) auto-promoted from 🟡 lite to ✅ full.
5. **Spec-23** — framework primitives. `on_change` + `!secret` are
   parallelisable with spec-22 work.

`next-priorities-2026-05.md` recommends **finish-then-pivot**. With PRs
8, 9, 10, 11, and 14 all in master, **Track B (personal-fleet close-out)
is effectively done** — only Phase C polish (mDNS, `fleet init`) and
the new `mooncake apply <machine>` user request remain, all of which the
analysis docs explicitly de-prioritise. The recommendation collapses to
**"pivot now."** Track A (agent-safety) is the only meaningful work
ahead; phase 3 just closed, so phase 4 (`Diff()`) is the next move.

The unfair-advantage statement the VISION leaves open (§13.10) gets
answered when `transaction:` ships: **"plan + snapshot + reverse +
deterministic replay, all typed."** *"Agent edited 4 files, third failed,
mooncake auto-reverted the first two"* becomes a falsifiable claim once
spec-22 phase 5 (`Reverse()`) and spec-30 land.

The strategic question is no longer *"will the pivot happen"* — it's
*"how soon does `Reverse()` get to `file.write`."*
