# Proposal 02: `mooncake fleet kill <run-id>` — cancel an in-flight run from the controller

**Status:** Draft proposal
**Effort:** S (~2 days for the agentd half; the CLI is trivial)
**Value:** High — the missing answer to `fleet ps` saying "run
01KRP... running on gpu-box-1". Today the only escape is `ssh`.

---

## Problem

`fleet ps` happily shows in-flight runs:

```
$ mooncake fleet ps
PEER         RUN_ID                       AGE   STATUS    NAME
gpu-box-1    01KRPK1126M8B63M63SKYQVCM5   38s   running   apt-upgrade
main_pc      01KRPK117ABCDEFGHIJK2345678  12s   running   site.yml
```

A run on the wrong peer. A run that's wedged. A run that should
have been aborted before the user realized. There's no answer
from the controller. Today:

```bash
ssh gpu-box-1
sudo systemctl stop mooncake-agentd
# ... wait, that kills agentd entirely, not just this run
sudo journalctl -u mooncake-agentd
# find PID, kill -9, hope nothing got half-written
sudo systemctl start mooncake-agentd
```

`mooncake apply` itself doesn't exit cleanly on SIGINT (#87), so
the controller side might be wedged too. Two interruption gaps in
one workflow.

This is a hole in the fleet contract. Every container orchestrator
(docker, k8s, nomad) has a `kill` verb. Mooncake's fleet doesn't.

## Proposal

A new subcommand:

```bash
mooncake fleet kill <peer> <run-id>
```

Behavior:
1. POST `/v1/runs/<id>/cancel` to the named peer's agentd.
2. agentd:
   a. Sends SIGINT to the run's process group
   b. Waits up to `--grace 5s` for clean shutdown (current step's
      cleanup, transaction rollback if applicable)
   c. SIGKILL if grace expires
   d. Marks the run terminal with status=`cancelled` in
      `runs.jsonl`
3. Streams events back so the controller sees the cancellation
   propagating in real time

```
$ mooncake fleet kill gpu-box-1 01KRPK11
[gpu-box-1] cancelling run 01KRPK1126M8B63M63SKYQVCM5...
[gpu-box-1]   ↺ Reverse: step-0004 apt-get upgrade -y (timeout=5s)
[gpu-box-1]   ↺ Reverse: step-0003 apt-get update
[gpu-box-1] ✗ run cancelled at step 4/8 (2 reverted, 0 failed)
fleet kill: 1/1 cancelled in 1.4s
```

Cancellation should compose with `transaction:` rollback (already
exists in core), so half-applied state has a clear story:

- Steps inside a `transaction:` that were committed before cancel →
  reversed via existing LIFO path
- Steps outside a transaction → left as-is, marked cancelled
- The run's `on_rollback:` block (if defined) fires

## Variants

| Command | Behavior |
|---|---|
| `fleet kill <peer> <run-id>` | Cancel one run by ULID prefix |
| `fleet kill <peer>` | Cancel ALL in-flight runs on a peer (prompts) |
| `fleet kill --peer tag=staging` | Cancel in-flight on every matching peer (prompts) |
| `fleet kill <peer> <run-id> --force` | Skip grace period; SIGKILL immediately |
| `fleet kill <peer> <run-id> --no-rollback` | Skip transaction reverse; just exit (dangerous, opt-in) |

Default: prompt before killing more than one run at once.

## API on agentd

```
POST /v1/runs/<id>/cancel
   { grace_seconds: 5, no_rollback: false }
→ 202 Accepted   { ack: true, run_id: ... }
→ 404 Not Found  { error: "run not found" }
→ 409 Conflict   { error: "run already terminal", status: "success" }
```

Cancellation is async; the controller polls or streams events to
confirm.

## Receipts

Pain points from manual testing:
- Round 49 (SIGINT testing): `mooncake apply` doesn't exit on
  SIGINT (#87). Same root cause as missing cancellation.
- Round 30: I ran a long fleet exec and wanted to cancel mid-run
  to debug; had to wait for the 30s sleep to complete.
- Hypothetical: imagine running `fleet apply` against 10 peers and
  realizing on peer 3 that you have a bug in the playbook. Today:
  every peer runs the bad playbook to completion. With kill: stop
  the fleet, fix the bug, redeploy.

## Why this lives in fleet, not core

agentd already maintains the run lifecycle (it's the runner). The
cancellation primitive is a *daemon* feature. The CLI surface is
just a thin client. Fleet owns the daemon → fleet owns this.

The kernel `mooncake apply` SIGINT story (#87) is a separate
issue but mechanically similar. Fixing kill at the daemon
unblocks the controller-side fix too: the controller can SIGINT
itself, then ask each peer to cancel its in-flight run.

## What this doesn't address

- **Recovery / replay after cancellation**. Today a cancelled run
  leaves whatever state it left. Future work: `fleet replay <run-id>`
  picks up from the last committed step. Out of scope here.
- **Cancellation reasons / audit**. v1 just emits "cancelled by
  <user>@<host>" in the run record. Operator-attributed reasons
  ("oncall paged us about this") can come later.
- **Cancelling queued runs** (status=queued, not yet started). v1
  scope is in-flight only; if agentd has a queue, queued runs
  should also be cancellable.

## Implementation sketch

agentd side (Go):
```go
// internal/agentd/cancel.go
func (s *Server) cancelRun(ctx context.Context, runID string, opts CancelOpts) error {
    run := s.runs.Get(runID)
    if run == nil { return ErrRunNotFound }
    if run.Terminal() { return ErrAlreadyTerminal }

    run.Cancel(ctx, opts.Grace, opts.NoRollback)  // signals worker goroutine
    return nil
}
```

The worker goroutine reads run.cancelCh in its step loop, runs
rollback if applicable, marks terminal, emits cancel events to
SSE subscribers. Cleanly integrates with the existing run state
machine.

Client side (`cmd/fleet_kill.go`): POST + tail SSE, render
`[peer]`-prefixed events same way as `fleet logs`. ~50 LOC.
