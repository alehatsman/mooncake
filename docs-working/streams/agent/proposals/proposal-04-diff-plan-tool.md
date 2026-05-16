# Proposal 04: `diff_plan` MCP tool — typed pre-execution diff for the agent's reasoning step

**Status:** Draft proposal
**Effort:** S (~2 days; reuses core proposal-04's typed diff)
**Value:** High — `check_plan` today returns plan structure but
not the *intent* of what would change. Agents reviewing a plan
before approval need a typed-diff surface, not raw plan JSON.

---

## Problem

Today the agent flow for "what would this plan do?":

```
agent → mcp.tools/call check_plan {config: "/work/cfg.yml"}
mcp → agent: {
  "changed": 0, "ok": 0, "failed": 0, "skipped": 0,
  "requires": {"filesystem_write": ["/tmp/x.txt"]},
  "steps": [{"name": "greet"}, {"name": "write"}, {"name": "verify"}]
}
```

This is plan structure: which steps will run, what permissions they
need. What's missing: **what would the system actually look like
afterward?**

The agent has to:
1. Parse the plan
2. For each step, derive what would change
3. Recombine into a summary

That's work the kernel can do (and `mooncake plan --diff` already
does). It's just not exposed via MCP.

The fix: expose the typed diff (per core proposal-04) as an MCP
tool.

## Proposal

A new MCP tool `diff_plan`:

```jsonc
// Request
{"method": "tools/call", "params": {"name": "diff_plan", "arguments": {
   "config": "/work/cfg.yml",
   "format": "typed"            // typed | unified | both
}}}

// Response: structured typed diff per step
{
  "plan_id": "01KRPK...",
  "input_files_hash": "abc123...",
  "summary": {
    "files": {"created": 2, "modified": 1, "deleted": 0},
    "packages": {"installed": 1, "removed": 0, "upgraded": 0},
    "users": {"created": 1, "modified": 0, "deleted": 0},
    "services": {"enabled": 1, "started": 1},
    "totals": {"would_change": 5, "ok": 1, "skipped": 0}
  },
  "steps": [
    {
      "step_id": "step-0001",
      "name": "write config",
      "action": "file.write",
      "diff": {
        "kind": "file",
        "path": "/etc/nginx/conf.d/site.conf",
        "from": null,
        "to_bytes": 256,
        "to_mode": "0644",
        "unified_diff": "--- /etc/...\n+++ /etc/... (proposed)\n@@ ..."
      }
    },
    {
      "step_id": "step-0002",
      "name": "install nginx",
      "action": "pkg",
      "diff": {
        "kind": "package",
        "manager": "apt",
        "package": "nginx",
        "from": {"installed": false},
        "to": {"version": "1.24.0-1ubuntu1.1"}
      }
    },
    {
      "step_id": "step-0003",
      "name": "enable nginx",
      "action": "os.service",
      "diff": {
        "kind": "service",
        "service": "nginx",
        "from": {"state": "inactive", "enabled": false},
        "to": {"state": "active", "enabled": true}
      }
    }
  ]
}
```

The `summary:` rollup gives the agent an at-a-glance "this plan
touches 2 new files, installs 1 package, creates 1 user". The
`steps:` array gives the per-step typed diff (the same data core
proposal-04 surfaces in `plan --diff`).

## Why typed, not just unified

Agents reasoning about plans benefit from category-grained diffs:

- "Would this plan install any packages?" → check
  `summary.packages.installed > 0`
- "Are any irreversible operations being attempted?" → check each
  step's `action` against the capability matrix (core proposal-05)
- "What's the rollback story?" → walk the steps and check
  `capabilities.reverse` per action

The unified-diff form is good for files. The typed form is good
for everything else. Offer both:

```jsonc
{"name": "diff_plan", "arguments": {"config": "...", "format": "typed"}}    // typed only
{"name": "diff_plan", "arguments": {"config": "...", "format": "unified"}}  // unified text
{"name": "diff_plan", "arguments": {"config": "...", "format": "both"}}     // both, in the JSON
```

## Why `diff_plan`, not extend `check_plan`

`check_plan` today is "dry-run the plan and tell me the outcome
counters". `diff_plan` is "tell me the typed deltas". Different
questions, different shapes. Keeping them separate:

- `check_plan` answers "would this succeed?" (validation + outcome)
- `diff_plan` answers "what would change?" (typed transitions)

Both are useful; both should exist; both should be cheap (neither
mutates).

## Compose with run_plan

```jsonc
// Agent workflow:
// 1. Check the plan would succeed
{"name": "check_plan", "arguments": {"config": "/work/cfg.yml"}}

// 2. Get the typed diff
{"name": "diff_plan", "arguments": {"config": "/work/cfg.yml"}}

// 3. Show the diff to a user (or evaluate against a policy)
// 4. If approved:
{"name": "run_plan", "arguments": {"config": "/work/cfg.yml", "stream": true}}
```

The order is: validate → diff → apply. Each step is a separate
MCP call so the agent can checkpoint.

## Implementation

`diff_plan` is structurally similar to `check_plan`:

```go
// internal/mcp/tools/diff_plan.go
func (s *Server) diffPlan(args DiffPlanArgs) (*DiffPlanResult, error) {
    plan, err := planner.Compile(args.Config)
    if err != nil { return nil, err }

    diffs, err := plan.GenerateDiff(s.ctx)  // calls handler.Diff() per step
    if err != nil { return nil, err }

    return &DiffPlanResult{
        PlanID:      plan.ID,
        InputHash:   plan.InputFilesHash,
        Summary:     rollupSummary(diffs),
        Steps:       diffs,
    }, nil
}
```

`plan.GenerateDiff` already exists per core proposal-04 (typed
plan diff). The MCP binding is a thin wrapper.

## Receipts

- **#54** (MCP run_plan counters) — fixed; but `requires` is
  pre-execution permission summary, not a change preview. Users
  asked "what changes?" — `diff_plan` answers.
- **Core proposal-04** — typed plan diff is the data source.
  Exposing it via MCP is the obvious followup.

## What this doesn't address

- **Policy enforcement** — diff_plan returns "what would change";
  agent decides if it's allowed. Future proposal: `policy_check`
  tool that runs the diff against a declared policy.
- **Drift comparison** — "diff between desired (plan) and current
  (host)" — that's `mooncake fleet drift` (spec-58). Compose: drift
  uses diff_plan internally + observe.*.
- **Resource cost estimates** — "this plan will pull 280MB of
  packages and take ~2 min". core proposal-05's `Cost` capability
  surfaces it; could be added to summary later.

## Pairs with

- **Core proposal-04** (typed plan diffs) — provides the data
- **Proposal 01** (`describe_action`) — the action's `Diff` shape
  documented per action
- **Proposal 06** (permissions as contract) — `requires:` from
  check_plan and typed diff from diff_plan together drive the
  agent's pre-approval gate
