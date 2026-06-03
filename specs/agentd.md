---
id: agentd
status: draft
owners: [aleh]
covers:
  - cmd/agentd/**
  - internal/agentd/**
  - internal/runlog/**
  - internal/jsonllog/**
---

# Agentd — host daemon: run lifecycle + event streaming

## Intent

`agentd` is the per-host daemon that accepts mooncake plans over a small `/v1/*`
HTTP surface, queues and executes them one at a time, persists a file-backed
record of every run, and streams structured events live (SSE) and to disk
(JSONL). It is the server half of the fleet protocol: the `fleet`/`runs`
controller is the client. It listens on a perm-gated unix socket and/or a
bearer-auth-gated TCP address.

## Behavior

- WHEN a client POSTs `/v1/runs` with a validated plan path, agentd allocates a
  ULID run id, writes a `queued` `record.json`, touches `events.jsonl`, and
  enqueues the run; submission returns immediately.
- WHILE running, a single FIFO worker goroutine executes one run at a time,
  chdir'ing to the run's `base_dir` so path resolution matches the submitter's
  intent; concurrency is intentionally absent in v1.
- WHEN a run starts/finishes, the worker transitions the record through
  `queued → running → success|failed` and stamps started/finished timestamps and
  any error.
- WHEN the daemon executes a run it persists the kernel result to `result.json`
  via atomic temp-file+rename, served by `GET /v1/runs/{id}/result`.
- WHEN events are produced, the per-run sink encodes each once with a monotonic
  `seq`, appends it to `events.jsonl`, and broadcasts identical bytes to an
  in-memory hub; the sink drains, flushes, and fsyncs before the terminal record
  is written so a `success` record is never observed ahead of its event tail.
- WHEN a client GETs `/v1/runs/{id}/events` it replays `events.jsonl` then tails
  the live hub; IF a subscriber falls behind, the hub drops messages (never
  blocks the broadcaster) and the client backfills the seq gap via
  `Last-Event-ID` on reconnect.
- WHEN the daemon restarts, startup reconcile marks any run left `queued`/
  `running` by a previous PID as `interrupted` so stale runs never appear live.
- WHEN the daemon receives SIGTERM/SIGINT it cancels the shared run context so the
  in-flight apply abandons at the next step boundary, drains the worker, and
  removes the unix socket.
- WHERE a TCP listener is bound, every request requires a bearer token and
  agentd MAY advertise itself over mDNS; WHERE only the unix socket is bound,
  filesystem perms (0600) are the sole gate.
- WHEN agentd starts and the socket path already exists it probes it: a live
  daemon there is an error, a stale file is removed.
- WHEN a client calls the control surface, agentd serves `GET /v1/health`,
  `/v1/version`, `/v1/facts`, `/v1/metrics`, `POST /v1/mcp`, run endpoints,
  `PUT/HEAD /v1/files`, and self-management (`/v1/self/{binary,replace,mac,
  shutdown}`).
- WHEN every run record is finalized, a compact line is also appended to the
  shared `~/.mooncake/runs.jsonl` runlog (via `jsonllog`), readable newest-first
  for `explain`/history.
- WHEN a controller issues `fleet kill <peer> <run-id>` it SHOULD cancel that
  specific in-flight run; agentd v1 exposes no per-run cancel endpoint, only the
  whole-daemon drain (#25).

## Non-goals

- No concurrency: v1 runs plans strictly one at a time; parallel applies are out
  of scope.
- Not a scheduler or queue broker — submission is fire-and-forget FIFO, no
  priorities, retries, or cron.
- No central coordination; agentd knows nothing of other peers.

## Checklist

- [x] `POST /v1/runs` submit: validate path, ULID id, `queued` record, enqueue (`internal/agentd/{runs_handler,store}.go`)
- [x] Single-goroutine FIFO worker with `base_dir` chdir (`internal/agentd/worker.go`)
- [x] Run state machine `queued→running→success|failed` + atomic record writes (`internal/agentd/store.go`)
- [x] `result.json` kernel result, atomic write, `GET /v1/runs/{id}/result` (`internal/agentd/worker.go`, `runs_handler.go`)
- [x] Per-run JSONL sink: seq, async drain, fsync-before-terminal (`internal/agentd/jsonl_sink.go`)
- [x] In-memory SSE hub: broadcast, drop-on-slow, eager registration (`internal/agentd/sse_hub.go`)
- [x] `GET /v1/runs/{id}/events` replay-then-tail with `Last-Event-ID` backfill (`internal/agentd/runs_handler.go`)
- [x] Startup reconcile of prior-daemon `queued`/`running` → `interrupted` (`internal/agentd/store.go`)
- [x] Graceful shutdown: cancel run ctx, drain worker, remove socket (`internal/agentd/{server,worker}.go`)
- [x] Dual listeners: perm-gated unix socket + bearer-auth TCP; stale-socket claim (`internal/agentd/{server,middleware}.go`)
- [x] Optional mDNS advertise when TCP-bound (`internal/agentd/server.go`)
- [x] Control surface: health/version/facts/metrics/mcp/files/self-* (`internal/agentd/{server,handlers,files_handler,self_*}.go`)
- [x] Shared append-only runlog `runs.jsonl` (`internal/runlog/**`, `internal/jsonllog/**`)
- [x] `mooncake agentd run|bootstrap` + `mooncake runs` CLI (`cmd/agentd/**`)
- [ ] Per-run cancel endpoint to back `fleet kill` (#25)
