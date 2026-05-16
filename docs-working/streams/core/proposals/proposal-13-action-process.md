# Proposal 13: `process` action — supervised long-running processes (distinct from `service`)

**Status:** Draft proposal (brainstorm-stage)
**Effort:** S (~3 days)
**Value:** Medium-High — fills the gap between "fire-and-forget
`shell:`" and "OS-managed `service:`" for the ad-hoc long-running
processes agent loops and pilot-agent runtimes spawn.

---

## Problem

Mooncake has two existing primitives for "do work":

- `shell:` — fire a command, wait for it to exit. No supervision.
- `service:` — manage an OS-installed unit (systemd, launchd,
  Windows services). Heavy: requires a unit file, root, package
  install.

There's a middle category that has no good answer today:

- Run a local LLM server (`ollama serve`, `vllm serve`) as part of
  a plan and let downstream steps depend on it.
- Spawn a Python helper that listens on a unix socket for the rest
  of the plan to consume.
- Run a pilot-agent process that should be alive while the maintain
  loop is running and reaped when it exits.
- Run a long-running scraper / watcher / replay tool that an agent
  manages declaratively.

`shell:` won't do — these processes need to outlive the step that
started them. `service:` won't do — they're per-plan, per-user, not
something to install into systemd.

## Proposal

A new core action: `process`. Concept is "userland systemd for plan
lifetime" — supervised, restartable, scoped to the mooncake apply.

### Step shape

```yaml
- process:
    name: ollama
    command: ["ollama", "serve"]
    env:
      OLLAMA_HOST: "127.0.0.1:11434"
    cwd: /tmp
    state: running        # running | stopped | restarted | absent
    health:
      port_open: 11434
      timeout: 30s
    restart:
      on_exit: always     # never | on_failure | always
      max_in_window: 5
      window: 60s
    log:
      stdout: ~/.mooncake/processes/ollama.out
      stderr: ~/.mooncake/processes/ollama.err
      rotate: 10M
```

### Lifecycle scope

Each process is tracked in `~/.mooncake/processes/` with a pid file,
its config, and its log paths. State is per-user, not system-wide
(no root required for the common case).

- **state: running** — start if not running; wait for health gate;
  fail step if health doesn't pass within `timeout`.
- **state: restarted** — kill (if running), then start. Always
  changes.
- **state: stopped** — kill if running, leave pid file removed.
- **state: absent** — stop + delete log + delete pid file.

### Health gates

Reuses the `assert` predicate vocabulary (proposal-11): `port_open`,
`http_ok`, `process_running`, `file_exists`. Default for "is it up"
is `process_running` against the pid.

### Cleanup

Two cleanup modes:

- **`scope: plan`** — process is killed when the plan exits
  (`apply` finishes). Good for "spin up an LLM, do five steps,
  spin down".
- **`scope: session`** (default) — process survives `apply`,
  managed declaratively across runs. A subsequent plan with
  `state: absent` reaps it.

### Why this is not `service:`

`service:` manages OS-installed long-running daemons. `process:` is
"my plan needs a sidecar for the next 5 minutes". They look similar
but the lifecycle ownership is different:

- service: owned by the OS, mooncake just toggles state.
- process: owned by mooncake, started/stopped by the plan.

A pilot-agent plan that wants to "start ollama, run 4 prompts,
stop ollama" has no good shape in `service:` (you'd have to install
a unit file, run as root, register, deregister). `process` is the
natural shape.

## Use modes

- **Pilot agents with local models** — a plan starts `ollama serve`,
  uses it across many `mcp_tool` calls (proposal-07), stops it on
  exit.
- **LAN fleet sidecars** — each node runs an `agentd` companion
  process started by a plan, no systemd unit required.
- **Self-healing observers** — a watch process started under
  `process:` whose pid is asserted by an `assert` (proposal-11).
- **Integration tests** — start a temporary HTTP mock, run plan,
  stop mock.

## What this doesn't address

- **Cross-host process supervision** — fleet stream territory.
- **Resource limits (cpu / mem caps)** — defer; add when there's a
  user need. cgroup v2 integration is a separate lift.
- **TTY / interactive process** — out of scope.
- **Replacement for `systemd` / `launchd`** — explicitly not.
  `process` is the lightweight "my plan needs a sidecar"
  primitive.

## Field-budget impact

Zero universal fields. `process:` is a new step type, fully
self-contained.

## Pairs with

- **Core proposal-11** (`assert` + `heal`) — assert process health,
  heal by restart.
- **Agent proposal-07** (`mcp_tool`) — pilot agents that use local
  models start the model under `process:`.
- **Fleet stream** — future "fleet process" for spawning the same
  sidecar across N peers, layered on this single-node design.
