# Agent framework — retired direction

> **Retired 2026-09-07.** This page used to be the source-of-truth doc
> for "mooncake as a substrate other agents build on" — external
> consumers (moongit, a planned `openclaw`) registering their own
> typed Go handlers, a four-layer cognitive architecture (L1 registry
> / L2 kernel / L3 grounding / L4 reasoning), and a fully-offline
> general-purpose agent built on top. Cut because it had **zero
> validated adoption**, and its one real test failed in a way that's
> worth recording so it doesn't get re-proposed unexamined.

## Why this was cut

The plan called for a lighthouse consumer to prove the shape: moongit
would register a `moongit.issue` typed action pack into mooncake's
registry, so a moongit-driven agent mutated the issue tracker through
mooncake's typed, reversible, audited ABI instead of shell (moongit
issue #107). It was built up to and then closed:

> "Closing as redundant. moongit now exposes its full surface over
> MCP. Typed moongit.\* mooncake actions are the long way around — #12
> (mcp_tool) gives the same coverage generically once it lands."

That's the opposite integration direction from the one this doc
proposed: moongit became an MCP **server** that mooncake (or anything)
calls *into*, rather than a consumer that imports mooncake and
registers actions *into* it — which is exactly the `mcp__moongit__*`
tool surface used to manage this repo's issues and PRs today. Given
the one real attempt went the other way, and neither the openclaw
reference agent nor the L3/L4 grounding split it existed to serve was
ever built, the whole external-framework narrative was speculative
from the start and is not a direction mooncake is pursuing. Mooncake's
focus is system configuration, provisioning, and management — not
building or hosting other people's agents.

## What's still real and stays

Two refactors landed on the way to this idea, and both are worth
keeping on their own merits — as internal hygiene and a legitimate
embedding capability, not as steps toward a framework:

- **Registry-as-dependency** (#105) — the action registry is an
  injectable `*Registry` instead of a package global, threaded through
  the executor, planner, and agent loop. Cleaner DI, easier testing.
- **Public `sdk/` facade** (#106) — a Go program can `import
  "github.com/alehatsman/mooncake/sdk"` and call `ApplyConfig` /
  `ApplySteps` / `ApplyBytes` to run a plan **in-process**, without
  shelling out to the CLI. Useful on its own for a test harness or a
  custom tool that wants provisioning as a library call — independent
  of any agent story.

The typed `Handler` ABI (`Diff`/`Reverse`/`Cost`/`Permissions`, see
[`kernel.md`](./kernel.md)) remains how *mooncake's own action set*
grows, built-in, through the normal spec path. Registering a
compile-time custom handler from an external Go binary is still
technically possible — the registry API doesn't forbid it — but it
is not a pursued direction and nothing currently exercises it.

## See also

- [`kernel.md`](./kernel.md) — what mooncake *is*.
- [`goals.md`](./goals.md) — the layered product surface; the
  standalone "AI agent developer" audience and the enterprise-hub
  ring were demoted the same day as this page, same reasoning
  (unvalidated, no real user).
- [`sharing_and_modules.md`](./sharing_and_modules.md) — the *config*
  sharing story (Git-native modules) — unaffected by this page; a
  separate, still-live direction.
