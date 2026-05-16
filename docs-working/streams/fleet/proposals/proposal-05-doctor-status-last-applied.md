# Proposal 05: `fleet doctor --all` + `fleet status` shows last-applied — connect the daily questions

**Status:** Draft proposal
**Effort:** S (~2 days)
**Value:** High — the two questions every fleet operator wakes up
asking ("is anything broken?" and "when did each box last
converge?") deserve one-command answers.

---

## Problem

Today the operator's daily check is several commands:

```
$ mooncake fleet status
HOST       ADDR              ACCESSIBLE  RUNNING  OS             MOONCAKE  QUEUE  LAST RUN
main_pc    192.168.1.5:7878  yes         no       linux (amd64)  0.2.0     0      —
laptop     192.168.1.6:7878  yes         no       linux (amd64)  0.2.0     0      —
gpu-box-1  192.168.1.7:7878  no          —        —              —         —      —
```

Three questions remain unanswered:

1. **What's wrong with `gpu-box-1`?** → run `fleet doctor gpu-box-1`.
   Once per unreachable peer.
2. **When did each peer last successfully apply?** → not in this
   table. The "LAST RUN" column shows `—` if no run happened in
   this controller's history; the peer might have applied via
   another controller or via `mooncake apply` locally on the
   peer.
3. **Has anyone drifted since their last apply?** → spec-58, not
   shipped yet.

The first one (run doctor) is a per-peer dance. The second is
information that **agentd already knows** — `runs.jsonl` has the
last applied timestamp + outcome. Just hasn't been surfaced.

## Proposal A: `fleet doctor --all`

Multi-peer doctor:

```bash
$ mooncake fleet doctor --all
main_pc       → 192.168.1.5:7878
  ✓ resolve   ✓ tcp   ✓ http   ✓ auth   ✓ facts        → healthy
laptop        → 192.168.1.6:7878
  ✓ resolve   ✓ tcp   ✓ http   ✓ auth   ✓ facts        → healthy
gpu-box-1     → 192.168.1.7:7878
  ✓ resolve   ✗ tcp   —      —      —                → unreachable (TCP refused)
    → why: port 7878 not accepting connections on 192.168.1.7
    → fix: ssh in, check `systemctl status mooncake-agentd`

fleet doctor: 2/3 healthy, 1 unreachable
```

Parallel probe with per-peer rung table. Fails-fast per peer
(stops at first red rung). The output condenses one peer per
two lines unless something fails (then the failing rung
expands to show diagnostics).

| Flag | Behavior |
|---|---|
| `fleet doctor --all` | All peers in peers.toml |
| `fleet doctor --peer tag=prod` | Filtered (uses proposal-01 selector) |
| `fleet doctor <peer>` | Single peer (today's behavior — keeps positional shortcut) |
| `fleet doctor --json --all` | JSONL per peer |

Exit code: non-zero if any peer is unhealthy. Useful in CI.

## Proposal B: `fleet status` shows last applied

Today's `fleet status` table:

```
HOST       STATE      OS             MOONCAKE  QUEUE  LAST RUN
main_pc    ok         linux (amd64)  0.2.0     0      —
```

Proposed addition: a `LAST APPLIED` column showing the most recent
successful run's timestamp, age, and outcome:

```
HOST       STATE      OS             VER    LAST APPLIED         DRIFT?
main_pc    ok         linux (amd64)  0.2.0  4h ago (✓ 7 changed) clean
laptop     ok         linux (amd64)  0.2.0  3d ago (✓ 12 changed) clean
gpu-box-1  unreach.   —              —      —                    —
db-1       ok         linux (amd64)  0.2.0  21d ago (✓ 4 changed) ⚠ stale (>7d)
```

Fields:
- **LAST APPLIED**: humanized age + result of latest run with
  `status: success` (skips queued / cancelled / failed). Comes
  from agentd's `runs.jsonl`.
- **DRIFT?**: `clean` (last apply was recent, kernel-side fast
  drift probe says no), `⚠ stale (>Nd)` (haven't applied in a
  while), `⚠ drift (N changes)` (drift detected — needs spec-58
  to populate, but column is reserved now).

The `MOONCAKE` column rename to `VER` keeps row width
manageable. `QUEUE` is dropped from the default view (mostly 0;
still in `--wide`).

`fleet status --wide` shows the full set (QUEUE, LAST RUN
ULID, agentd uptime, etc.).

## Why these two together

The daily morning question is *"is the fleet healthy and
in-sync?"*. Today the answer requires:

```
fleet status                       # accessibility
fleet doctor <each unreachable>   # diagnose
fleet ps --all                    # is anyone running something I forgot?
fleet facts <peer> --query last_apply_time  # last applied... wait, fact doesn't exist
```

After this proposal:

```
fleet doctor --all     # everything reachable + healthy?
fleet status           # what's the state? when did each apply?
```

Two commands. One for "is everyone there?", one for "are they in
shape?".

## API on agentd

`fleet status` already calls `/v1/version`, `/v1/runs`, `/v1/facts`
per peer. The "LAST APPLIED" field comes from `/v1/runs` filtered
by `status=success` — already implementable client-side. No new
endpoint needed.

`fleet doctor --all` is also client-side: same probe ladder
parallelized across peers. ~50 LOC.

The DRIFT? column needs spec-58 (drift detection) to populate
real data. v1 can ship with a placeholder ("—") and the column
becomes meaningful when drift lands.

## Receipts

From the manual-tester audit:
- Round 30: After `fleet status` showed `1/2 accessible, 1 unreachable`,
  the natural next step was `fleet doctor <unreachable>`. Three
  commands of friction; should be one.
- The "LAST RUN" column today shows `—` for peers I had just `fleet
  exec`-ed seconds ago — it's not populated unless we *applied*
  something. Confusing naming.
- Drift detection (spec-58) is identified as the highest-value
  unbuilt fleet feature; this proposal reserves UI real estate so
  it has a column to display in when it ships.

## What this doesn't address

- **Per-peer last-applied is on agentd, not the controller**. If
  the same peer is owned by two controllers, each sees its own
  history. Mooncake is single-controller-per-peer-by-design
  (peer.toml is the inventory). Document this assumption.
- **Drift-detection-itself** is spec-58. This proposal just
  reserves the column.
- **Reachability heat-map / dashboard** — a TUI / web UI is out
  of scope.

## Pairs with

- **proposal-04 (global flags)** — `fleet doctor --all` should
  honor the same `--peer`, `--parallel`, `--timeout` as the
  rest of the surface.
- **spec-58 (fleet drift)** — populates the DRIFT? column.
- **proposal-02 (fleet kill)** — the "is anyone running something
  I forgot?" question gets a `fleet ps` answer; if the answer is
  bad, `fleet kill` provides remediation.
