# Proposal 06: Permissions as a contract — declare allowed permissions, plan rejected if exceeded

**Status:** Draft proposal
**Effort:** M (~5 days; design + handler propagation)
**Value:** High — this is the "Docker for AI agents" pitch made
concrete. Today agents can theoretically run any action; the safety
story is "the human reads the plan". A declared permissions contract
moves it from human-watchful to machine-enforceable.

---

## Problem

Today's "Docker for AI agents" framing rests on:
- **Typed mutation surface** (no shell escape needed for common ops)
- **`transaction:` rollback** (LIFO Reverse on failure)
- **`!secret` typed refs** (redaction)
- **`Permissions/Diff/Cost/Reverse` ABI methods** (spec-22)

What's missing: **the agent (or its operator) can't declare "this
plan is allowed to do X but not Y" and have the server enforce it**.

Today's `Permissions()` returns what an action touches:
```
file.write { path: /etc/foo.conf } → permissions: filesystem_write(/etc/foo.conf)
pkg { name: nginx }                → permissions: pkg_install, network_download
shell                              → permissions: shell_exec (full escape hatch)
```

MCP's `run_plan` returns:
```jsonc
{
  "requires": {
    "filesystem_write": ["/etc/foo.conf"],
    "pkg_install": ["nginx"],
    "network_download": ["package_manager"]
  }
}
```

That's a **summary**, not a **contract**. The plan still runs
regardless of whether the agent's caller (a user, a CI policy)
intended to allow those permissions.

## Proposal

Three pieces:

### 1. Declared allowed-permissions on `run_plan`

```jsonc
// Request
{"method": "tools/call", "params": {"name": "run_plan", "arguments": {
   "config": "/work/cfg.yml",
   "allow_permissions": {
     "filesystem_write": ["/etc/foo.conf"],   // exact-path allowlist
     "filesystem_write_prefix": ["/var/log/"], // prefix allowlist
     "pkg_install": true,                      // unconstrained
     "shell_exec": false,                      // explicit denial
     "network_download": ["github.com"]        // hostname allowlist
   }
}}}

// Response (if plan exceeds permissions):
{
  "error": {
    "code": -32000,
    "message": "plan requires permissions not in allow_permissions",
    "data": {
      "denied": [
        {"permission": "filesystem_write", "value": "/etc/passwd", "step": "step-0007"},
        {"permission": "shell_exec", "value": "rm -rf /var/cache", "step": "step-0009"}
      ],
      "allowed_but_unused": [
        {"permission": "filesystem_write_prefix", "value": "/var/log/"}
      ]
    }
  }
}
```

Plan rejected **at plan-time, before any step runs**.

### 2. Default-deny policy mode

```jsonc
{"name": "run_plan", "arguments": {
   "config": "/work/cfg.yml",
   "policy": "default_deny",         // every permission must be in allow_permissions
   "allow_permissions": {...}
}}}
```

Or:
```jsonc
{"name": "run_plan", "arguments": {
   "config": "/work/cfg.yml",
   "policy": "explicit_only"          // synonym for default_deny
}}}
```

vs. today's implicit:
```jsonc
{"name": "run_plan", "arguments": {
   "config": "/work/cfg.yml",
   "policy": "default_allow"          // current behavior; runs anything
}}}
```

Default for new MCP clients SHOULD be `default_deny`. Existing
agents that don't pass `allow_permissions` keep working at
`default_allow` for one release cycle, with a warning logged.

### 3. Permission categories

A small taxonomy of permission "kinds":

| Kind | Examples |
|---|---|
| `filesystem_read` | reading config/secrets |
| `filesystem_write` | file.write, file.copy, file.template |
| `filesystem_write_prefix` | wildcard glob form |
| `filesystem_delete` | rm / file.write state: absent |
| `pkg_install` | pkg, pkg.* |
| `pkg_remove` | pkg state: absent |
| `system_user` | os.user/os.group |
| `system_service` | os.service/os.systemd |
| `system_firewall` | os.firewall, os.sysctl |
| `system_cron` | os.cron |
| `network_download` | file.download, git.clone, pkg manager |
| `network_egress` | observe.http, !secret with HTTP backend |
| `shell_exec` | shell, cmd (the escape hatch) |
| `fleet_apply` | controller initiates fleet apply |
| `fleet_kill` | controller cancels fleet runs |

(Defined in `internal/actions/permissions/`. Spec-22 already
defines the `Permissions()` method; this proposal makes its
output a contract.)

## Use cases

| Scenario | Permission declaration |
|---|---|
| Agent updates only a single file | `{"filesystem_write": ["/etc/specific.conf"]}` |
| CI for a deploy pipeline; can install packages but no shell | `{"filesystem_write_prefix": ["/etc/myapp/", "/var/log/myapp/"], "pkg_install": true, "shell_exec": false}` |
| Read-only ops (observability dashboards) | `{"filesystem_read": true, "network_egress": true}` (everything else denied) |
| `mooncake fleet apply` with auto-approval policy | Caller declares allowed permissions; agentd enforces |

## Integration with !secret

Once a plan references a `!secret` typed ref, agents should be
able to declare what they're allowed to read:

```jsonc
"allow_permissions": {
  ...,
  "secret_read": ["env:DATABASE_URL", "vault:prod/api_key"]
}
```

If the plan references `!secret env:SOMETHING_ELSE`, plan rejected.

## Implementation

Permission check runs **at plan-compile time**, before executor
starts:

```go
// internal/plan/permissions.go
func (p *Plan) CheckAgainst(allow PermissionSet, policy Policy) error {
    required := p.AggregatePermissions()  // sums Permissions() per step
    return policy.Enforce(required, allow)
}
```

`AggregatePermissions()` is already what `requires:` in run_plan
returns. Just feed it into a policy check.

The policy itself is a small DSL:
- Exact match (`filesystem_write` for `/etc/foo.conf` allows that
  path exactly)
- Prefix match (`filesystem_write_prefix` matches any path under it)
- Boolean (allow all of this category, allow nothing)
- Glob (`/var/log/*.log`)

## API consistency

`check_plan` returns the same `denied:` list when run with
`policy: default_deny` and insufficient `allow_permissions` — so
agents can ask "what would this plan need?" before deciding what
to allow.

`diff_plan` (proposal-04) returns the typed diff regardless of
permissions — diffs are read-only.

## Receipts

- The MCP `requires:` field in `run_plan` already exists. It's
  ~80% of the data needed for enforcement; just isn't checked.
- spec-22 declared `Permissions()` per handler. The output isn't
  used as a contract today.
- Audit findings: no specific receipt because the audit only
  exercised single-user / single-controller flows. The pain
  surfaces when an agent runs at scale (or autonomously) — at
  which point the safety story has to hold without human review.

## Why this lives in agent

The MCP server is the agent integration point. The permissions
contract is a property of the agent's call to the server.
Implementing it requires:

1. Aggregate `Permissions()` per step (already done for `requires:`)
2. Enforce against declared allow set (new)
3. Reject at plan-time (new)
4. Document the permission taxonomy (new)

Each piece is small. Together they're the load-bearing safety
feature for agent-driven mooncake.

## What this doesn't address

- **Per-action quotas** (max N packages installed per run, max M
  files written) — out of scope; defer to a future
  `quota` block in plan.
- **Egress policy** (network destinations allowlist) — covered
  by `network_download` / `network_egress` permission categories
  but the actual destination matching is per-action-handler work.
- **Plan signing** (verifying a plan came from a trusted source) —
  separate, larger spec.
- **Permission delegation** — if an MCP client passes a plan to
  another MCP client, who owns the allow set? Defer.

## Pairs with

- **Proposal 04** (diff_plan) — agents see typed diff + permission
  summary, then approve or refine the allow set
- **Core proposal-05** (capability flags) — `Permissions()` is one
  of the four ABI methods this surfaces
- **Spec-23 §3** (!secret) — `secret_read` permission is the
  agent-side hook
- **Fleet proposal-04** (global flags) — `--allow-permissions
  <file>` could be a fleet apply flag too, with the same JSON shape
