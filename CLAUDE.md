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

---

## Architecture soft caps

Three review-time prompts (not CI gates) that keep the architecture
self-policing as the project grows. When a PR crosses one of these
thresholds, the reviewer asks the question; nothing auto-blocks.

Grounded in the 2026-05-15 arch report
(`docs-working/arch-report/2026-05-15-arch-report.md`).

### 1. Handler LOC > 1,500 → split

Reason: handlers that grow past ~1,500 LOC are almost always
cross-platform multiplexers (file, service, package, os_mount) that
accreted per-OS branches into one file. Past the cap, split into
per-OS sub-packages (`internal/actions/<name>/{linux,darwin,windows}`)
or into sibling action types.

### 2. `internal/config.Step` universal-field count > 40 → flag

Reason: every universal field on `Step` is a concept every step type
must ignore or honor. The closed action set is the kernel's moat
(see `docs-working/vision/kernel.md`); the cost of that is a
monotonically-growing `config.go`. Today's count is 36 (run
`task budget-status`). Past 40, the field has become a tag everyone
has to ignore, and the question "why does *every* step need this?"
stops having a good answer.

### 3. `gocyclo` > 35 in any non-test function → refactor on next touch

Reason: gocyclo > 35 means six or more independent decision branches
in one function — almost always a CLI handler doing business logic
that belongs in an `internal/` package. The cap doesn't force a
refactor; it surfaces the smell on the next PR that touches the
function.

### Today's known violations (tracked, not blocking)

Run `task budget-status` for the current source-of-truth list.
Pinning the list inline here has drifted within a sprint every time
we've tried it (see code-review finding F002); the script is the
authoritative source.

These are documented, not hidden. New violations should be
explicitly defended in the PR description that lands them.

See `docs-working/arch-report/2026-05-15-refactoring-plan.md` §2
(R0.4) for the rationale; this is the project's first formal soft-cap
policy.
