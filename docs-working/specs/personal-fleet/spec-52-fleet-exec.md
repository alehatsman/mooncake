# Spec 52: Fleet Exec — ad-hoc shell across peers

**Epic:** Personal Fleet — see [`epics/epic-personal-fleet.md`](../../epics/epic-personal-fleet.md).
**Status:** Draft
**Effort:** S (~2–3 days)
**Value:** High — closes the single biggest fleet DX gap noted in
[`clustermanagement/qol-features.md`](../../clustermanagement/qol-features.md).
Today, "what does `systemctl status sshd` say on every peer?" forces the
operator to author a one-step plan file with a `shell` action and apply
it. `fleet exec` collapses that workflow to one command. Operator's
`pssh` / `ansible -m shell` equivalent.
**Depends on:** spec-43 (transport + `/v1/runs`), spec-48 (multiplexer +
`--peer-filter`), spec-50 (extended filter keys `name=`, `os=`, `role=`).

---

## Problem

The kernel's `shell` action and the agentd `/v1/runs` endpoint are both
already in place. Streaming-back output through the multiplexer with
`[peer]` line prefixes is already done for `fleet apply`. What's missing
is the wrapper: a single command that takes a shell string, fans it out
across N peers, and surfaces the per-peer output + exit code with no
detour through a plan file on disk.

The pain is concrete:

```
# Today
$ cat > /tmp/check-sshd.yml <<'EOF'
- name: status
  shell: systemctl status sshd
EOF
$ mooncake fleet apply /tmp/check-sshd.yml --peer-filter os=linux

# What we want
$ mooncake fleet exec --peer-filter os=linux 'systemctl status sshd'
```

The agentd surface area required is **zero** if we synthesize an
in-memory one-step plan and submit it through the existing run
pipeline — every piece (auth, sync, streaming, exit-code reporting) is
already wired. This spec is mostly CLI surface and a tiny synthesized
plan helper.

---

## Goals

- **G1** `mooncake fleet exec '<cmd>'` runs `<cmd>` on every selected
  agentd peer in parallel, streaming `[peer]` -prefixed stdout/stderr
  back through the existing `fleet.Multiplexer`.
- **G2** Peer selection reuses `--peers`, `--peers-file`, `--peer-filter`
  (full spec-50 key set: `tag`, `name`, `os`, `role`), and `--parallel`
  with identical semantics to `fleet apply`. No new selection flags.
- **G3** Exit-code aggregation: the controller's exit code is the
  per-peer max (0 iff every peer succeeded; 1 if any non-zero exit;
  2 if any peer was unreachable or the submit failed). A trailing
  summary banner reports `N/M ok`, listing the failing peers by name.
- **G4** `--env KEY=VAL` (repeatable) forwards environment variables to
  the synthesized shell step. `--cwd <path>` sets the working directory
  on the peer. `--timeout <duration>` enforces a per-peer wall clock
  through the kernel's existing `Step.Timeout` field.
- **G5** `--become` (and `--ask-become-pass` once spec-47 lands the
  controller-side password prompt) maps onto the synthesized step's
  `become` field. Privilege escalation flows through the kernel's
  existing shell-action plumbing — no new code path.
- **G6** `--json` emits one JSONL record per peer
  (`{peer, run_id, exit_code, status, stdout, stderr, duration_ms}`)
  to stdout instead of the multiplexed prefix output. Intended for
  scripting; pairs with `jq`.

**Non-goals:**

- **stdin to the remote command.** Operators occasionally want
  `fleet exec 'tee /etc/foo' < local.txt`. Plumbing stdin through SSE
  multiplexed across N peers is non-trivial (which peer's prompt are
  we filling?) and rare. Defer to a follow-up; document the workaround
  (write a one-step plan with `shell.stdin:` or use `file.copy`).
- **Interactive TTY allocation** (`-t` / `ssh -t` analogue). No PTY.
  Anything that wants a terminal needs `ssh` or a real plan.
- **SSH-transport peers in v1.** Peers with `transport = "ssh-bootstrap"`
  (spec-44) are skipped with a banner. Re-evaluate when bootstrap-SSH
  peers become a regular operating mode rather than a one-shot install
  channel.
- **`--dry-run`.** Exec is ad-hoc by definition; previewing a shell
  command without running it would just be `echo`. The kernel shell
  handler's `SupportsDryRun=true` is honored if `--dry-run` is passed,
  but the helptext discourages it ("for exec, dry-run prints the
  rendered command without executing — use `echo $CMD` instead").
- **Plan-dir sync.** No `--plan-dir`, no `--vars-file`. The synthesized
  plan lives entirely in memory on the controller and only `plan_path`
  is needed daemon-side — see Design §"Plan synthesis".

---

## Reuse map

**Reused (no changes):**

- `internal/fleet/transport/agentd.go` — `Client.Submit` (POST
  `/v1/runs`), `Client.Stream` (SSE), `Client.GetVersion`,
  `Client.GetRun`. The submitRequest JSON shape is unchanged.
- `internal/agentd/runs_handler.go` — daemon accepts the synthesized
  plan as-is; it's just a normal one-step shell plan.
- `internal/agentd/store.go` — `SubmitReq` / `Run` records unchanged.
- `internal/fleet/multiplex.go` — `NewMultiplexer`, `Drain`, `Banner`,
  `formatEvent` (especially the `step.stdout` / `step.stderr` /
  `step.failed` cases — they already render the shell output we'll
  emit).
- `internal/actions/shell/handler.go` — the shell action handler on the
  daemon side does the actual work, including `become`, `Env`, `Cwd`,
  `Timeout`, retry. No handler change.
- spec-48's `parseFilterFlags` + spec-50's `peerMatchesFilters` /
  `validatePeerFilterKeys` / `newPeerOSCache`. Same selector pipeline
  as `fleet apply`.

**Extended:**

- `cmd/fleet.go` — register `fleetExecCommand()`; fits next to
  `fleetApplyCommand()` in the `Subcommands` slice. Shares
  `selectPeers`, the filter-flag parsers, and the SIGINT handler shape.

**New:**

| Component | Location |
|---|---|
| `fleet exec` CLI command + action | `cmd/fleet_exec.go` |
| Synthesized one-step plan writer (writes to a tempfile, returns abs path) | `internal/fleet/exec/plan.go` |
| Per-peer exec orchestrator (parallel fan-out, JSONL or multiplex emit) | `internal/fleet/exec/run.go` |

Exec deliberately bypasses `internal/fleet/apply.go` rather than wedging
into `ApplyOptions`. The Apply path is plan-dir-centric (walk → sync →
submit); Exec has no plan-dir to walk and the daemon's run-submit
handler refuses a non-absolute `plan_path` (`runs_handler.go:51`). The
cleanest design is a parallel `Exec` orchestrator that writes a tiny
synthesized plan into a per-peer scratch dir under the peer's
`SyncedRoot`, then submits that path. Details below.

---

## Design

### Plan synthesis

Each peer receives a freshly synthesized one-step plan written into its
own scratch scope under the peer's `SyncedRoot`. The plan file content
is generated on the controller from the CLI flags:

```yaml
# Synthesized by `mooncake fleet exec` — do not edit.
- name: fleet-exec
  shell:
    cmd: |
      systemctl status sshd
  # become / env / cwd / timeout populated from flags
```

Each peer's scratch directory:

```
<peer-SyncedRoot>/exec/<controller-id>/<ulid>/exec.yml
```

The ULID ensures concurrent `fleet exec` invocations don't collide. The
scope segment `exec/<controller-id>` keeps exec runs separate from the
plan-dir scopes that `fleet apply` writes (`apply/<controller-id>/...`),
so neither path's garbage collection can touch the other's files.

To get the file onto the peer, reuse the existing transport `Put`
(`PUT /v1/files`) — same code path `fleet.SyncTo` uses, but with a
synthetic one-entry sync list. The `scope` query param goes to
`exec/<controller-id>`, the `path` to `<ulid>/exec.yml`. The daemon
stores the file under its synced root exactly as it does for plan-dir
syncs.

After the run terminates, the controller could clean up the scratch
file. Keep it simple in v1: leave the file in place, let the daemon's
existing GC story (spec-43 §"sync GC", currently TODO) handle it. The
files are tiny (< 1 KiB each) so this isn't urgent.

### Submit shape

```go
// Per peer, after Put:
client.Submit(ctx, transport.SubmitRequest{
    PlanPath: <peer-SyncedRoot>/exec/<controller-id>/<ulid>/exec.yml,
    BaseDir:  <peer-SyncedRoot>/exec/<controller-id>/<ulid>/,
    // No VarsFiles, Tags, Names, or Goal.
})
```

The daemon then runs the step exactly like any other run. Events flow
through the existing SSE stream; `step.stdout` and `step.stderr`
already get formatted with the right indentation by
`multiplex.go:formatEvent`.

### CLI shape

```
mooncake fleet exec [flags] -- <command...>
mooncake fleet exec [flags] '<command string>'
```

Two argument forms:

1. **Single string** (Bourne-shell-style):
   `mooncake fleet exec 'systemctl status sshd'`
   The whole arg becomes the `shell.cmd` field as-is; the daemon's
   interpreter (bash on Unix, powershell on Windows) parses it.

2. **`--` separator** (argv form for quoting safety):
   `mooncake fleet exec --peer-filter os=linux -- ls -la /tmp`
   Args after `--` are joined with single spaces and assigned to
   `shell.cmd`. We do NOT pass them as `command.argv` (which would
   bypass the shell) — exec is explicitly the "I want shell" entry
   point. The shape `command` (no shell) is what `fleet apply` with a
   plan is for.

Quoting is the operator's responsibility. Document the surprises:
single-quotes pass through; backslash-escapes are shell-dependent;
`$VAR` is expanded by the *peer's* shell, not the controller's. This
matches pssh / ansible-shell conventions.

### Flags

```
--peers <names>            comma-separated (alias for --peer-filter name=...)
--peers-file <path>        override peers.toml location
--peer-filter <k=v>        repeatable; same DSL as fleet apply
--parallel <n>             max in-flight peers, 0=unbounded (default 0)
--env KEY=VAL              repeatable; forwarded to shell step env
--cwd <path>               working directory on the peer
--timeout <duration>       per-peer wall clock (e.g. 30s, 2m); default unset
--become                   run with sudo (Unix) / require admin (Windows)
--ask-become-pass          (deferred to spec-47 password plumbing)
--shell <interpreter>      override default interpreter (bash, zsh, pwsh, ...)
--no-color                 disable ANSI prefix coloring
--json                     emit JSONL records instead of prefixed output
```

The flag set deliberately mirrors `fleet apply` where overlap makes
sense; new flags (`--env`, `--cwd`, `--timeout`, `--become`,
`--shell`) all map to existing fields on the kernel's `ShellAction` and
`Step` types. No new daemon-side validation.

### Output: multiplexed mode (default)

Reuses `fleet.NewMultiplexer` exactly as `fleet apply` does. The
synthesized step emits `step.started`, then a stream of `step.stdout` /
`step.stderr` events, then either `step.completed` or `step.failed`,
then `run.completed`. The multiplexer's `formatEvent` already knows how
to render all of these.

Example session:

```
$ mooncake fleet exec --peer-filter os=linux 'systemctl is-active sshd'
fleet exec: 3 peer(s), command = "systemctl is-active sshd"
[laptop  ] ▶ run started
[desktop1] ▶ run started
[macbook ] – skipped (transport ssh-bootstrap not supported by exec)
[laptop  ]   ▸ fleet-exec
[desktop1]   ▸ fleet-exec
[laptop  ]       active
[laptop  ]     ✔ fleet-exec
[laptop  ] ✔ run complete success: 1/1 changed, 0 failed, 0 skipped (12ms)
[desktop1]       active
[desktop1]     ✔ fleet-exec
[desktop1] ✔ run complete success: 1/1 changed, 0 failed, 0 skipped (8ms)
fleet exec: 2/2 ok
```

Failure case (non-zero exit on one peer):

```
[laptop  ]       active
[desktop1]       inactive
[desktop1]     ✗ fleet-exec: exit status 3
[desktop1] ✔ run complete failed: 0/1 changed, 1 failed, 0 skipped (15ms)
[laptop  ]     ✔ fleet-exec
[laptop  ] ✔ run complete success: 1/1 changed, 0 failed, 0 skipped (12ms)
fleet exec: 1/2 ok — failed on desktop1
$ echo $?
1
```

### Output: `--json` mode

One JSONL record per peer, emitted in order of completion. Multiplexer
is skipped; the stream consumer collects events per peer, builds the
record, prints when the run terminates.

```json
{"peer":"laptop","run_id":"01JFXY...","status":"success","exit_code":0,"stdout":"active\n","stderr":"","duration_ms":12}
{"peer":"desktop1","run_id":"01JFXZ...","status":"failed","exit_code":3,"stdout":"inactive\n","stderr":"","duration_ms":15}
```

Notes:

- `stdout` / `stderr` are accumulated from `step.stdout` / `step.stderr`
  event lines (joined with `\n`). Bounded at 1 MiB per stream; truncate
  beyond that and set `stdout_truncated: true` so the operator notices
  rather than silently losing trailing bytes.
- `exit_code` is parsed out of `step.completed` / `step.failed` event
  data when the kernel surfaces it. Today the kernel's shell handler
  attaches the exit code on `step.failed` (`error_message` field); the
  spec assumes it also lands on `step.completed` data — verify during
  implementation; if missing, surface as a tiny kernel patch
  (~10 LOC: include `exit_code` in `StepCompletedData`).

### SIGINT handling

Same shape as `fleet apply`:

1. First `^C` cancels the local context — the SSE streams close, the
   multiplexer prints a banner ("⚠ ^C closes the log stream only —
   remote runs continue. See `mooncake fleet logs <host>` to reattach").
2. Second `^C` hard-exits with 130.

Remote `shell` runs are NOT canceled by ^C in v1 — the agentd run keeps
going. Acceptable: most exec commands are sub-second, and the
"close-the-stream-only" semantics are familiar from `fleet apply`. If
operators ask, follow-up could add a `--cancel-on-ctrl-c` flag that
POSTs a hypothetical `/v1/runs/{id}/cancel` (not in the daemon today).

### Exit-code aggregation

| Per-peer outcome | Aggregate effect |
|---|---|
| Run completed, all steps success | 0 |
| Run completed, step failed (non-zero shell exit) | 1 |
| Peer unreachable / submit failed | 2 |
| Mix of above | max of the above |

The summary banner names every non-zero peer:

```
fleet exec: 4/5 ok — failed on macbook (exit 2); unreachable: vps-1
```

---

## Implementation outline

### Task 1 — `internal/fleet/exec/plan.go`

1. `Synthesize(opts SynthOptions) ([]byte, error)` returns the YAML
   bytes of a one-step plan. Inputs: `Cmd`, `Env`, `Cwd`, `Timeout`,
   `Become`, `Interpreter`.
2. Marshal via the existing `config.Step` / `ShellAction` types so the
   YAML is guaranteed to round-trip through the kernel loader. No
   hand-rolled string templating.
3. Test: every flag combination produces a plan the kernel
   `config.LoadPlan` accepts without error.

### Task 2 — `internal/fleet/exec/run.go`

1. `Exec(ctx, opts ExecOptions) ([]PeerOutcome, error)` runs the
   per-peer cycle:
   - Marshal plan bytes (once; shared across peers).
   - For each peer: `GetVersion` to learn `SyncedRoot`; mint a ULID
     scratch dir; `Put` the plan; `Submit`; `Stream`.
   - Accumulate stdout/stderr and exit code into `PeerOutcome`.
   - Emit to either the `Events` channel (multiplex mode) or the
     internal accumulator (json mode); identical shape choice to
     `fleet.Apply`.
2. Reuse `fleet.SyncTo`? No — `SyncTo` walks a plan-dir, and we only
   have one file. Call `Client.Put` directly with the one file.
3. Test (unit): plan bytes are well-formed YAML and contain expected
   fields. Test (integration): exec against a real agentd; failure
   path exits non-zero.

### Task 3 — `cmd/fleet_exec.go`

1. `fleetExecCommand()` returns a `*cli.Command` with the flags above.
2. `fleetExecAction(c)`:
   - Parse the command arg (single string vs `--`-separated argv;
     reject zero-arg case with helptext).
   - Load `peers.toml`; apply `--peers` + `--peer-filter` (same code
     as `fleet apply`: `selectPeers`, `parseFilterFlags`,
     `validatePeerFilterKeys`, `peerMatchesFilters`, `newPeerOSCache`).
   - Filter out non-agentd transports; banner the skipped peers.
   - Build `exec.SynthOptions` from flags.
   - Dispatch to `runExecMultiplex(...)` or `runExecJSON(...)`.
3. Register in `fleetCommand().Subcommands` between
   `fleetApplyCommand()` and `fleetStatusCommand()`.

### Task 4 — Multiplex driver

1. Mirror `runApplyPhase` (`cmd/fleet_machine.go`) but call the
   `exec.Exec` orchestrator instead of `fleet.Apply`.
2. SIGINT handler identical to `fleetApplyAction`'s.
3. Exit-code aggregation per the table above.

### Task 5 — JSON driver

1. Same fan-out, but each per-peer goroutine accumulates events into
   a `PeerOutcome` struct (no multiplexer); on terminal status,
   serialize via `json.Marshal` and emit one line under a mutex to
   stdout.
2. Aggregate exit code is computed after all peers complete.

### Task 6 — Tests

1. Unit: plan synthesis (Task 1) round-trips through `config.LoadPlan`
   for every flag combination.
2. Integration: `cmd/fleet_exec_integration_test.go` against a real
   agentd; assert exit codes for success / failure / unreachable;
   assert `--json` output is valid JSONL; assert `--peer-filter os=`
   selects correctly.
3. Quoting smoke test: `fleet exec -- echo 'hello world'` produces
   `hello world` on each peer.
4. Helptext snapshot test: flag list stable.

### Task 7 — Docs

1. Add a `fleet exec` row to `docs-working/clustermanagement/qol-features.md`
   striking it from the "Tier 1 — small, daily-use" list.
2. README snippet under the personal-fleet section: the
   `systemctl is-active sshd` example.
3. PROGRESS.md follow-up after merge.

---

## Open questions

1. **Synthesize-as-plan vs new `/v1/exec` endpoint.** This spec leans
   hard on plan synthesis: zero daemon work, identical event stream as
   apply, free reuse of `become` / `env` / `timeout`. The cost is the
   per-peer scratch file write (one `PUT /v1/files` round trip per peer,
   ~1 KiB body). The alternative — a new `POST /v1/exec` endpoint that
   takes `{cmd, env, cwd, timeout, become}` and runs the shell action
   inline without a plan file — is faster by exactly one round trip and
   spares the scratch file, but adds an endpoint that duplicates the
   plan-submit machinery (queue, hub, persistence, idempotency). **Lean:
   synthesize-as-plan.** The latency cost is one HTTP PUT on the LAN
   (sub-millisecond on agentd) and the moving-parts win is substantial.
2. **`exit_code` event-data exposure.** Today the kernel surfaces a
   shell step's exit code only on `step.failed` (via the error
   message). For `--json` to produce a useful `exit_code` field on
   success, the kernel needs to attach `exit_code` to
   `StepCompletedData` too. Verify during implementation. If it's not
   there, this spec is a coupled ~10-LOC kernel change (touches
   `internal/events` + the shell handler's `Execute` return). Lean:
   accept the coupling — it's a one-field addition and useful beyond
   exec.
3. **Stdin support.** Genuinely useful for `fleet exec 'tee /etc/foo'`,
   but plumbing it through SSE-multiplexed N-peer fan-out is awkward
   (do all peers get the same stdin? Is stdin closed after the prompt?).
   Punt to v2. Operators can use `shell.stdin:` in a plan today.
4. **Scratch file cleanup.** The synthesized plans accumulate at
   `<SyncedRoot>/exec/<controller-id>/<ulid>/exec.yml`. At ~1 KiB each
   and bounded by run history, this is fine for a personal fleet. But
   should the daemon GC `exec/` more aggressively than `apply/`? Lean
   no — same retention policy is simpler.
5. **SSH-transport peers.** Skipped in v1 with a banner. The
   bootstrap-SSH transport (spec-44) was designed for installation, not
   ongoing operations, and growing it into a parallel command-runner
   path duplicates agentd's job. If users start hitting this, the right
   answer is "run `mooncake fleet bootstrap` and convert the peer to
   agentd", not "extend exec to ride the bootstrap SSH transport".
6. **`--shell <interpreter>`.** The kernel's `ShellAction.Interpreter`
   exists and works. Worth exposing as a CLI flag in v1? Lean yes —
   trivial to plumb, and `fleet exec --shell zsh 'echo $ZSH_VERSION'`
   is a natural request.
7. **Timeout semantics.** `--timeout 30s` enforces a per-peer wall
   clock via `Step.Timeout`. On expiry, the kernel kills the process
   and the step fails. Should we ALSO cap the controller-side
   `Client.Stream` deadline? Lean no — the kernel's step kill produces
   a `step.failed` event with `error_message: "timeout"`, and the run
   completes normally. Adding a second-layer controller deadline just
   risks double-reporting.
8. **`--ask-become-pass`.** Listed in flags but explicitly punted until
   spec-47's controller-side password prompt lands. Document the
   deferral in helptext.

---

## Success criteria

After this spec lands:

1. `mooncake fleet exec 'systemctl is-active sshd'` runs on every
   agentd peer in `peers.toml`, prints `[peer] active` (or the failure
   line) on stdout, exits 0 iff every peer succeeded.
2. `mooncake fleet exec --peer-filter os=linux 'uname -r'` selects
   only Linux peers using the spec-50 evaluator — no peer-side
   bookkeeping required.
3. `mooncake fleet exec --json -- df -h /` produces one JSONL record
   per peer, each containing `peer`, `exit_code`, `stdout`, `stderr`,
   `duration_ms`. The exit code on the controller equals the max of
   the per-peer exit codes.
4. `mooncake fleet exec --become 'systemctl restart sshd'` runs with
   sudo via the kernel's existing `become` plumbing — no new daemon
   code path.
5. ^C closes the log stream and prints the "remote runs continue"
   banner identical to `fleet apply`. Second ^C hard-exits 130.
6. Helptext makes the "this is shell, not argv" choice explicit:
   `--` arguments are joined and passed as a shell string; `$VAR` is
   expanded by the peer's shell, not the controller's.
7. Adding `fleet exec` does not regress any `fleet apply`,
   `fleet status`, or `fleet upgrade` test. Shared helpers
   (`selectPeers`, `parseFilterFlags`, etc.) keep their signatures.
