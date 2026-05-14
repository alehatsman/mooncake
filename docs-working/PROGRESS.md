# Mooncake — Streams Progress & Ideal-State Report

Generated from `VISION.md`, `ROADMAP.md`, and the freshest `docs-working/` state
(master @ `4078878`, 2026-05-14).

---

## 1. Streams progress

Mooncake organises work into five parallel streams. Snapshot below.

### Stream 1 — Action Surface  *(kernel completeness)*

The typed mutation vocabulary. Ships everywhere.

| Spec | Topic | State |
|---|---|---|
| 24 | `pkg.*` (install/remove/repo/hold/upgrade/list) | P1–P5 shipped, P6 (ABI hooks) blocked on spec-22 |
| 25 | `text.line` · `text.patch.{ini,json,yaml}` | P1–P4 shipped, P5 blocked on spec-22 |
| 26 | `git.clone` (+creds/submodules) · `git.checkout` · `git.config` | P1–P4 shipped |
| 27 | `os.user` · `os.group` · `os.ssh_key` | P1–P3 shipped |
| 28 | `os.cron` · `os.sysctl` · `os.systemd` · `os.mount` · `os.firewall` | P1–P5 shipped (ufw only) |
| 17 | Batched packages + templated `names` | shipped |
| 37 | Step output capture (collision + plan-mode) | drafted |
| 38 | `read.json` / `read.yaml` | drafted; depends on 37 |
| 32 | Collapse step action dispatch | not started |
| **22** | **Extended Handler ABI (`Diff`/`Reverse`/`Cost`/`Permissions`)** | **not started — blocks every spec above's final phase AND all of Stream 2** |

**Verdict**: very wide. The "final phase" of every action is waiting on
spec-22. Adding more action breadth without 22 buys nothing strategic.

### Stream 2 — Safe Agent Runtime  *(the defensible wedge)*

| Spec | Topic | State |
|---|---|---|
| 23 | Framework primitives (`on_change`, `try/catch/finally`, `!secret`) | drafted, blocked on 22 |
| 30 | `transaction:` blocks with auto-reverse | drafted, blocked on 22 |

Plus a list of unwritten future specs in `streams.md`: policy DSL, plan
signing, per-action quotas, egress policy, sandbox mode, cost classifier,
deterministic replay.

**Verdict**: ZERO code yet. Confirmed by grep — no `transaction:`, `try:`,
`!secret`, or extended ABI in source. This is the stream the README's
marketing makes promises about.

### Stream 3 — Fleet & Cluster Management  *(the monetizable wedge)*

Personal Fleet (sub-stream): **8/14 PRs shipped end-to-end** as of
2026-05-14.

| Phase | PRs | State |
|---|---|---|
| **A** (one peer end-to-end) | 1–5 | ✅ all shipped |
| **B** (real fleet) | 6 multiplexer ✅, 7 status ✅, 8 logs/facts ⏳, 9 SSH driver ⏳, 10 systemd/launchd templates ⏳, 11 bootstrap/pair 🟡 lite | half done |
| **C** (polish) | 12 mDNS ⏳, 13 `fleet init` ⏳, 14 overlays/tags 🟡 in flight | barely started |

Sidecars merged this cycle: **spec-49 agentd-on-Windows** (TCP-only mode,
SSE race fixes) and a fleet polish PR (output + peer-filter UX + Windows
config paths).

**Verified against a real WSL + Windows two-peer testbed** including health
states. This isn't slideware.

Enterprise sub-stream (C1–C5 hub epics): **zero specs**, deferred. Per
`next-priorities`, intentionally not now.

**Verdict**: the bulk of recent activity. Closest stream to "lovable v1"
for its target audience.

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
`next-priorities`. The major friction items (no `init`, no default config,
broken README quickstart) are closed.

**Verdict**: the gap from "kernel-only, hand-write YAML" to "Mooncake feels
like a real tool" is closed.

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
today. The README front-page can describe this honestly; the gap is polish
(preset sharing UX) not capability.

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
`fleet bootstrap user@new-box` adds a new machine in 60s. No hub, no SaaS,
peer-to-peer over LAN.

**What's shipped**: agentd with TCP listener + bearer auth + SSE hub +
sandboxed file sync + `/v1/files` PUT/HEAD endpoints ✅, controller-side
multiplexed `fleet apply` ✅, **`fleet status`** with `--json` ✅, parallel
multi-peer multiplexer with `^C` banner ✅, `peers.toml` + `controller_id`
✅, `fleet bootstrap` / `fleet pair` **lite** (shell-out to `ssh`/`scp`) 🟡,
Windows agentd ✅.

**Gap**: `fleet logs` + `fleet facts` ⏳ (already referenced by the
multiplexer's `^C` banner — dishonest forward-reference today). Native SSH
driver + systemd/launchd installer templates ⏳ (would promote bootstrap
from 🟡 to ✅). mDNS discovery ⏳. `fleet init` interactive flow ⏳.
Per-host overlays + tag selectors 🟡 (in flight, known flag-name collision
on `--tag`).

**Distance to ideal**: ~70% to the v1 "Friday-evening demo" success
criteria from the epic. Phase A done; Phase B half done; Phase C barely
started. `fleet apply` + `fleet status` work end-to-end against a real
two-peer testbed — that's the headline. The remaining gaps are
well-understood and scoped.

**Notable**: this is the only ideal-state where Mooncake's wedge is in
active, sequential motion.

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

**Gap — and this is the strategic gap of the whole project**:

- **Extended handler ABI (`Diff`/`Reverse`/`Cost`/`Permissions`)**: not started. Spec-22.
- **`transaction:` blocks**: not started. Spec-30, depends on spec-22. The headline demo.
- **`try/catch/finally`, `on_change:`, `!secret`**: not started. Spec-23, depends on spec-22.
- **Policy DSL** (`deny:` patterns): not specced.
- **Plan signing** (Sigstore-style): not specced.
- **Per-action quotas + egress policy**: not specced.
- **Sandbox mode** (agent loses shell entirely): not specced.
- **Deterministic replay**: implicit via run audit but no `replay` command.
- **Cost / risk classifier**: not specced.

**Distance to ideal**: ~30%. The agent *interface* (MCP, agent loop,
structured I/O) is real. The agent *safety* primitives — the things the
VISION makes its loudest claims about — are not yet implemented.
`next-priorities-2026-05.md` calls this out explicitly: **"The work that
ships doesn't serve the story being told."**

---

## 3. The honest strategic picture

Mooncake has built the **kernel** (Stream 1: production-quality) and the
**fleet runtime** (Stream 3: 8/14 PRs in, live-tested) extremely well. It
has built the **DX funnel** (Stream 4: shipped). It has **not** built the
**agent safety layer** (Stream 2: the marketing wedge). Stream 5 is parked.

The internal recommendation in `analysis/next-priorities-2026-05.md` is
**finish-then-pivot**:

1. **Finish** (2–3 weeks left): close Phase B of personal-fleet — PR 8
   (`fleet logs`/`facts`), PR 9/10 (real bootstrap), PR 14 (overlays/tags
   — currently in flight). Plus DX R7–R10.
2. **Pivot to Path A**: write spec-22 against spec-30 as the real
   consumer, implement `Reverse()` on the three biggest handlers
   (`file.write`, `text.line`, `pkg.install`), ship `transaction:` blocks,
   add `Permissions` preflight, tiny policy v0.
3. **Land one lighthouse user** during the pivot.

The unfair-advantage statement the VISION leaves open (§13.10) gets
answered by the pivot: **"plan + snapshot + reverse + deterministic
replay, all typed."** If you can demo *"agent edited 4 files, third
failed, mooncake auto-reverted the first two,"* the agent-safety pitch
becomes falsifiable instead of aspirational.

The strategic question is what happens *after* in-flight fleet work
finishes: more action breadth (drift continues) or commit to the ABI
pivot (positioning gets earned).
