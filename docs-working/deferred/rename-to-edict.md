# Rename: mooncake → edict

**Status**: Deferred — revisit when ready

## Decision

Rename the project from `mooncake` to **Edict** for enterprise positioning.

## Why Edict

- An edict is an authoritative proclamation of desired state — matches declarative config semantics exactly
- No conflicts in DevOps, CLI tooling, or infrastructure space (verified May 2026)
- Enterprise/governance connotation resonates with compliance-focused buyers
- 2 syllables, unambiguous pronunciation, memorable
- `edict apply config.yml` reads naturally

## Rejected alternatives (and why)

| Name | Reason rejected |
|---|---|
| Pragma | PRAGMA = Cardano blockchain org (pragma.builders) |
| Praxis | Active Ruby API framework (github.com/praxis/praxis) |
| Sentinel | HashiCorp Sentinel + Microsoft Sentinel + Alibaba Sentinel |
| Idem | VMware built an Idem idempotent config tool |
| Enact | Samsung's Enact.js framework |
| Fiat | Car brand owns domain space |

## What renaming will touch

- Binary name: `mooncake` → `edict`
- Go module: `github.com/alehatsman/mooncake` → `github.com/alehatsman/edict`
- GitHub repo rename
- Install paths: `/usr/local/bin/mooncake` → `/usr/local/bin/edict`
- Config/preset search paths: `~/.mooncake/` → `~/.edict/`
- Documentation site URL
- README, CLAUDE.md, LLM_GUIDE.md references
