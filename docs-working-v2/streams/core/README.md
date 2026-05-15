# Stream: core

The typed mutation vocabulary. The planner. The executor. The
four-method handler ABI (`Permissions` / `Diff` / `Cost` / `Reverse`).
Facts. Snapshot. Everything below the daemon, the MCP server, and the
CLI's outer subcommands.

If you can run it as `mooncake apply -c plan.yml` on one machine with
no agentd involved, it lives here.

## Scope

| In | Out |
|---|---|
| Action handlers (`file.*`, `text.*`, `pkg.*`, `os.*`, `git.*`, `wait.*`, `read.*`, `observe.*`) | The MCP server and agent loop (see [agent](../agent/README.md)) |
| The handler ABI and its four methods | Fleet transport, multi-peer apply (see [fleet](../fleet/README.md)) |
| Planner (include resolution, for_each expansion, plan compilation) | `mooncake init` and other onboarding UX (see [dx](../dx/README.md)) |
| Executor (idempotency, secret resolution, retry, on_change, transactions) | |
| Facts subsystem | |
| Snapshot + structural diff | |
| Run audit log (JSONL events, structured errors, redaction) | |
| Schema generation (`schema.json` and validator) | |

## State

**Feature-complete on the priority handler set.** All five action
families ship with the four-method ABI. The kernel is what the rest of
the project stands on; it ships in master and is not the bottleneck.

Recent shipped specs (see commit history for the full receipts):

- spec-22 — Extended Handler ABI (phases 1–7)
- spec-23 — Framework primitives (`on_change`, `!secret`,
  `try/catch/finally`)
- spec-24–28 — `pkg.*`, `text.*`, `git.*`, `os.user`/`os.group`/
  `os.ssh_key`, `os.cron`/`os.systemd`/`os.sysctl`/`os.mount`/
  `os.firewall`
- spec-30 — `transaction:` with LIFO rollback
- spec-32 — step-action-dispatch refactor (reflection + plan tags)
- spec-33/34 — execution-context split, typed variable scope
- spec-35 — plan-diff
- spec-36 — Windows support (shell/pkg/file)
- spec-37/38 — step output capture + `read.json`/`read.yaml`
- spec-59–64 — typed observability (`observe.*` family)

## Active specs

None.

## Open gaps

- **Per-spec docs in `docs-next/`.** Every shipped spec above carries a
  "docs phase pending" tail. The work belongs in the canonical docs
  tree, not this folder.
- **Reverse-capture rollout to refusing handlers.** spec-26 reverse-
  capture v1 shipped for `git.checkout` / `git.config`; the pattern is
  reusable for ~13 other handlers (os.* family, pkg.repo, pkg.hold,
  os.service) that still return refusal stubs. Not blocking; ship as
  per-handler PRs when motivated.

## Design principles for new actions

Before adding any action, read
[`docs-working/action-design-principles.md`](../../../docs-working/action-design-principles.md).
The 11 rules summarized:

1. Modern naming (dot-namespaced, no legacy compat names).
2. Idempotent or explicitly declared not.
3. Plannable (structured diff in plan mode).
4. Snapshot-aware (declares what resource it touches).
5. Reversible by default, irreversible by exception.
6. Typed with JSON Schema.
7. Single responsibility.
8. Secure by default (redaction).
9. Cross-platform unless meaningfully OS-specific.
10. Composable through `outputs`.
11. Stable error taxonomy.

## Cross-stream dependencies

- [agent](../agent/README.md) consumes Core's typed Diff/Cost/
  Permissions through MCP. New ABI methods on Core handlers
  immediately become available to agents.
- [fleet](../fleet/README.md) ships Core plans across peers; agentd
  hosts the same executor that runs local applies.
- [dx](../dx/README.md) reads Core's run audit + facts to drive
  `mooncake doctor` and `mooncake history`.
