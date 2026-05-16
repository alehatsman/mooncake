# Mooncake

[![CI](https://github.com/alehatsman/mooncake/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/alehatsman/mooncake/actions/workflows/ci.yml)
[![Security](https://github.com/alehatsman/mooncake/actions/workflows/security.yml/badge.svg?branch=master)](https://github.com/alehatsman/mooncake/actions/workflows/security.yml)
[![codecov](https://codecov.io/gh/alehatsman/mooncake/branch/master/graph/badge.svg)](https://codecov.io/gh/alehatsman/mooncake)

**A safe execution layer for AI-driven system configuration.**

Mooncake is a single Go binary that turns YAML intent into typed,
idempotent system mutations — with auto-revert on failure, secrets
that never leak into logs, and a clean ABI an AI agent can call
without losing your machine.

```yaml
- name: deploy a new app config atomically
  transaction:
    - file.write:
        path: /etc/myapp/config.yml
        content: !secret env:APP_CONFIG
    - os.service:
        name: myapp
        state: restarted
  on_rollback:
    - log: "deploy failed; previous config restored"
```

If `os.service: restarted` fails, the `file.write` is automatically
reverted via the handler's `Reverse()` method and the `on_rollback`
notification fires. The system ends up byte-identical to its
pre-transaction state.

## Who it's for

- **AI agent developers** — give your agent a typed action vocabulary,
  dry-run validation, auto-revert on failure, and structured event
  output. Your agent can't escape the typed ABI; every mutation is
  observable and reversible.
- **Solo developers** — manage dotfiles + dev box + a personal fleet
  of 1–10 machines from one terminal, peer-to-peer, no hub.
- **Platform engineers** — declarative state with `--dry-run`,
  structured Diff output per action, run audit log, secret redaction.

## Quick start

```bash
go install github.com/alehatsman/mooncake@latest

# Scaffold a project.
mooncake init --template dotfiles

# Preview changes (typed Diff per action, no side effects).
mooncake apply --dry-run

# Apply.
mooncake apply
```

`mooncake apply` (and `plan`, `validate`) auto-discover
`./mooncake.yml` or `./mooncake/main.yml`, so you rarely need `-c`.

## Agent-safety features

Every claim here links to a working example you can run:

- **`transaction:` blocks with auto-revert.** Group N steps; on any
  failure, previously-completed steps run their `Reverse()` in LIFO
  order. Filesystem byte-identical to pre-transaction state. Run
  [`examples/transactions/rollback-demo.yml`](examples/transactions/rollback-demo.yml)
  to see the rollback fire.

- **Typed secret references that don't leak.** `!secret env:APP_TOKEN`
  resolves at apply time, is added to the redaction denylist, never
  appears in plan output, run logs, or `step.stdout`. Three built-in
  providers: `env:`, `file:`, `stdin:` (interactive). Try
  [`examples/secrets/env-secret.yml`](examples/secrets/env-secret.yml).

- **Reactive triggers (`on_change:`).** A step's `on_change:` children
  run only when the parent reported `changed=true`. The standard
  config-then-reload pattern without Ansible's handler magic. Try
  [`examples/triggers/on-change-config-reload.yml`](examples/triggers/on-change-config-reload.yml).

- **Structural diffs in plan mode.** `mooncake plan --diff` returns
  machine-readable `Diff` records per step — what file content
  changed, what package version, what service state — not just prose
  output. Used by the MCP server and any AI agent driving Mooncake to
  decide whether to proceed.

- **Permission preflight.** Every handler declares its
  `Permissions()` (sudo? network egress? specific binary?). The
  executor refuses to dispatch a step that needs sudo when the run
  isn't elevated, surfacing the requirement at plan time instead of
  as `EACCES` mid-run.

These primitives compose. A `transaction:` of `!secret`-bearing
`file.write` steps, with `on_change:` to restart services on config
change, all dry-runnable, with the secret never appearing in plan
JSON — is one short YAML file.

## What you can do

The full action surface (40+ typed actions). Highlights:

| Action | Purpose |
|---|---|
| `file.write` · `file.template` · `file.copy` · `file.download` | File-content management with Reverse() on every shape |
| `text.line` · `text.replace` · `text.patch.{ini,json,yaml}` | Surgical edits to existing files, all reversible |
| `pkg` | Cross-platform package install/remove (apt, dnf, brew, pacman) |
| `os.service` · `os.systemd` · `os.cron` · `os.sysctl` · `os.mount` · `os.firewall` | System-level resources |
| `os.user` · `os.group` · `os.ssh_key` | Identity management |
| `git.clone` · `git.checkout` · `git.config` | Repository setup with credentials + submodules |
| `container.image` · `container` | Container build / run |
| `repo.search` · `repo.tree` · `repo.patch` | Code-scoped operations for agents |
| `wait.{port,http,file,command}` | Synchronization primitives |
| `shell` · `cmd` · `assert` · `log` | Escape hatches + control |

See the full [actions reference](https://mooncake.alehatsman.com/guide/config/actions/).

**Auto-detected facts**: `{{os}}`, `{{arch}}`, `{{cpu_cores}}`,
`{{memory_total_mb}}`, `{{distribution}}`, `{{package_manager}}`.
Run `mooncake facts` to see all.

**Control flow**: `when:`, `with_items:`, `with_filetree:`, `tags:`,
`as_user:`, `--peer-filter tag=os=darwin` for fleet apply.

## Personal fleet

Drive plans across the machines you already own — no hub, no SaaS,
peer-to-peer.

```bash
# Bring a fresh box into the fleet (8-step bootstrap: SSH, install,
# systemd/launchd unit, start, verify, pair).
mooncake fleet bootstrap aleh@new-laptop

# Apply a plan across every peer.
mooncake fleet apply ~/dotfiles/config.yml

# Apply only to darwin peers.
mooncake fleet apply ~/dotfiles/config.yml --peer-filter tag=os=darwin

# Live status table.
mooncake fleet status

# Reattach to in-flight runs across the fleet.
mooncake fleet logs --all
```

The fleet plumbing is real `golang.org/x/crypto/ssh` + SFTP for
bootstrap, agentd over HTTP+SSE for everyday transport. See
[`docs-working/epics/done/epic-personal-fleet.md`](docs-working/epics/done/epic-personal-fleet.md)
for the design rationale.

## Comparison

| Capability | Mooncake | Ansible | Shell scripts |
|---|---|---|---|
| Single-binary install | ✓ | Python + modules | n/a |
| Idempotent typed actions | ✓ (40+ with Reverse) | ✓ (untyped) | ✗ |
| Dry-run with structural diffs | ✓ `plan --diff` | partial (check mode) | ✗ |
| **Transactions with auto-revert** | ✓ `transaction:` | ✗ | ✗ |
| **Secret refs that don't leak** | ✓ `!secret env:KEY` | partial (Vault module) | ✗ |
| Reactive triggers without registry | ✓ `on_change:` | partial (`notify:` + handlers) | ✗ |
| Cross-platform single config | Linux + macOS + Windows | Limited Windows | OS-specific |
| Designed for AI agent use | ✓ typed ABI + MCP server | ✗ untyped | ✗ unsafe |
| Personal-fleet peer-to-peer | ✓ `fleet apply` | hub-style (AWX) | n/a |

Mooncake isn't trying to replace Ansible at enterprise scale — it
ships the primitives Ansible doesn't have (typed Reverse, transaction
blocks, typed secrets, agent-safe ABI) while staying a single binary.

## Documentation

**[Full documentation](https://mooncake.alehatsman.com)** — guides,
action reference, AI specification.

Quick links:
- [Core concepts](https://mooncake.alehatsman.com/guide/core-concepts/)
- [Actions reference](https://mooncake.alehatsman.com/guide/config/actions/)
- [Complete reference](https://mooncake.alehatsman.com/guide/config/reference/)
- [AI / LLM specification](https://mooncake.alehatsman.com/ai-specification/)
- [Presets](https://mooncake.alehatsman.com/guide/presets/) — parameterized YAML workflows (in-tree library being retired; module-system replacement in design — see `docs-working/vision/sharing_and_modules.md`)

### Local examples

The [`examples/`](examples/) directory has a curated learning path —
see [`examples/README.md`](examples/README.md) for the ordered tour.
Notable demos for the agent-safety story:

- [`examples/transactions/rollback-demo.yml`](examples/transactions/rollback-demo.yml) — deliberate failure shows auto-revert in action
- [`examples/secrets/env-secret.yml`](examples/secrets/env-secret.yml) — `!secret env:APP_TOKEN` with redaction
- [`examples/triggers/on-change-config-reload.yml`](examples/triggers/on-change-config-reload.yml) — reactive reload pattern

```bash
git clone https://github.com/alehatsman/mooncake.git
cd mooncake

mooncake apply --config examples/hello-world/config.yml
cat examples/README.md  # ordered learning path
```

## Testing

Tested across Linux (Ubuntu, Debian, Alpine, Fedora, Arch), macOS
(Intel + Apple Silicon), and Windows Server.
See [testing docs](docs-next/testing/README.md).

## Contributing

Contributions welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).

- [Report bugs](https://github.com/alehatsman/mooncake/issues)
- [Request features](https://github.com/alehatsman/mooncake/issues)
- [Roadmap](docs-next/development/roadmap.md)

## License

MIT — Copyright (c) 2026 Aleh Atsman. See [LICENSE](LICENSE).

---

**[Read the full documentation](https://mooncake.alehatsman.com)** for
detailed guides, examples, and reference materials.
