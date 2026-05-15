# Mooncake Project Guide

**This project uses a universal LLM guide for all AI assistants.**

## 📖 Full Documentation

See **[LLM_GUIDE.md](./LLM_GUIDE.md)** for the complete project reference guide.

---

## Quick Context

**Mooncake** = Declarative config management tool (Go). "Docker for AI agents" - safe execution runtime with idempotency guarantees.

**Critical Rules**:
- ❌ **NEVER commit or push code** - user handles all git operations
- ✅ **Use a git worktree for implementation work** — any code change, spec implementation, or multi-PR feature must happen in a worktree, not on the active branch in the main checkout. Create one with `git worktree add ../mooncake-<branch> -b worktree-<branch>` and `cd` there before editing. When spawning sub-agents via the Agent tool for implementation, pass `isolation: "worktree"`. Doc-only edits (`docs-working/`, `docs-next/`, README tweaks), analysis writeups, and read-only investigation can stay on the current branch.
- ✅ **Claim work in `~/.mooncake/claims.jsonl` before starting.** JSONL append-only file outside any worktree, so all agents see the same state regardless of branch. Before touching a spec or task, check the latest line: `grep '"task":"spec-N"' ~/.mooncake/claims.jsonl | tail -1`. If empty or last status is `done`/`abandoned`, append a `claimed` line. If `claimed`/`in-progress` from a different worktree, STOP and tell the user. Statuses: `claimed` → `in-progress` → `done` | `abandoned`. Schema: `{ts, task, status, worktree, branch, note?, pr?}`. `task` is a spec slug like `spec-54` or a freeform label like `arch-snapshot`. Append `in-progress` when you start editing, `done` when merged, `abandoned` if you pivot.
- ✅ Focus on path resolution in presets (see LLM_GUIDE.md section)
- ✅ Follow definitive style guide: `docs/presets/definitive-style-guide.md`
- ✅ No over-engineering - minimal, focused solutions only

**Key Confusion Point**: Path resolution in presets
- Relative paths resolve from **including file's directory**
- Preset includes use `preset.BaseDir`
- See LLM_GUIDE.md "Path Expansion Summary" for details

**Architecture**: 5 core systems (Actions, Presets, Planner, Executor, Facts)
**Status**: Production-ready, 13 actions migrated ✅

For complete details, examples, and patterns → **[LLM_GUIDE.md](./LLM_GUIDE.md)**
