# Streams — Test Plan Index

Each stream owns a `testing/` directory with a hands-on manual test
plan. Start here when you're a new agent / contributor / QA pass.

## Test plans

| Stream | Test plan | Scope |
|---|---|---|
| [core](./core/testing/README.md) | Kernel: action handlers, planner, executor, schema validation, idempotency | One box, no daemon |
| [fleet](./fleet/testing/README.md) | agentd, peer transport, `mooncake fleet` subcommands | Multi-machine |
| [dx](./dx/testing/README.md) | `init`, `doctor`, `history`, error message quality, first-run UX | The CLI front door |
| [agent](./agent/testing/README.md) | MCP server, transactions, on_change, !secret, agent loop | "Docker for AI agents" |

## Stream dependencies

```
agent  ─┐
fleet  ─┼──► core   (everything stacks on the kernel)
dx     ─┘
```

If `core` tests fail, fix those before running the other streams —
broken kernel cascades.

## Conventions

- **Findings live in `docs-working/analysis/findings-<DATE>/`.** Split
  by *area* (silent-success-bugs, ssot-drift, template-engine,
  coverage-gaps, cli-and-friction, positive-keepers) — not by stream.
  A test for stream X often surfaces a bug in stream Y.
- **Severity tags**: CRITICAL / HIGH / MEDIUM / LOW. CRITICAL is "lies
  about success" or "silently loses data". Use sparingly.
- **Mark fixed findings with `✅ FIXED (round N, commit <hash>)`** — keep
  the original report intact above the fix banner so future agents see
  the receipt.
- **Mark partial fixes with `🟡 partial`**. Spell out what's still open.

## The single rule for testing

> **Verify on disk, not by recap.**

The biggest class of bugs found in 2026-05 (5 of the 7 HIGH+ findings)
shared the same shape: `failed=0` in the recap, but the action didn't
actually do what it claimed. The kernel got better at distinguishing
these; the convention is to never trust the rendered story.

Check `cat /path` (or `sha256sum`, or `getent passwd`, or `git
rev-parse HEAD`) after every test. Recap is the gloss; truth is the
filesystem.

## Quickstart for a new agent

If you have **5 minutes**: run the "concrete priority targets" list at
the bottom of `core/testing/README.md`. Five canary tests for the
kernel.

If you have **1 hour**: pick one stream's full test plan and execute
section by section. File findings as you go.

If you have **a day**: run all four streams' "1 hour" target lists,
plus the cross-cutting concerns (concurrency, unicode, performance,
SIGINT — see `core/testing/README.md` § Tricks).
