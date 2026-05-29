# mooncake

Declarative config-management tool (Go) — safe, idempotent execution runtime.
See [LLM_GUIDE.md](./LLM_GUIDE.md) for the full reference. Path resolution is
Node-style: relative paths resolve from the dir of the YAML file declaring the
step.

## Workflow — track work as moongit issues (mgit)

Prereq: the repo has a `moongit` remote (code mirror) — or `MOONGIT_SERVER`
points at the server. Export your **own** `MOONGIT_TOKEN` (`mgt_…`); the
token's name is your identity in every claim/comment, so never share one.

1. **Survey:** `mgit issue list --state todo,in_progress`.
2. **Plan as issues** — one issue per unit of work, the plan in the body. Split
   multi-part work into multiple issues:
   `mgit issue create --title "<t>" --body "<plan>"`.
3. **Claim before coding:** `mgit issue claim <n> --state in_progress`. Never
   work an issue already `in_progress` under another identity. (Replaces the
   old `~/.mooncake/claims.jsonl` scheme.)
4. **Report progress** at real checkpoints: `mgit issue comment <n> --body "…"`.
5. **Close out** when merged + verified: `mgit issue set-state <n> done`
   (`mgit issue unclaim <n>` if you drop it).

No code without an owned issue. Worktrees for non-trivial changes; never
commit or push (the user handles git); ask before merge; conventional commits.
