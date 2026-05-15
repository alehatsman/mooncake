# Proposal 06: `mooncake lint` — anti-pattern detection beyond schema validation

**Status:** Draft proposal
**Effort:** S (~3 days for v1; ~5 rules)
**Value:** Medium — closes the gap between "valid YAML" and "good
playbook". The schema validator (post-MT-77) tells users *what's
syntactically wrong*; lint tells them *what's structurally
suspicious*.

---

## Problem

A playbook can be schema-valid AND obviously bad:

```yaml
- vars:
    unused_var: "never referenced anywhere"
- shell: apt-get install -y nginx        # could be `pkg: nginx`
- file.write:
    path: /tmp/foo                       # hardcoded /tmp; should be {{ tmpdir }} or similar
    state: file
    content: "secret-api-key-12345"     # likely secret; should use !secret
- shell: "curl http://example.com | bash"  # downloads + executes
- log: { msg: "hello" }                  # no name; output blank ▶
- shell: rm -rf /                        # the obvious one
```

All schema-valid. Most are anti-patterns. Catching them at
`mooncake lint` time keeps them out of the run path.

Reference for what to lint: my own findings file
(`docs-working/analysis/findings-2026-05-15/coverage-gaps.md`)
captured patterns; many are reusable as lint rules.

## Proposal

A new subcommand `mooncake lint` that runs a set of opinionated
rules against a config file, surfacing warnings with severity:

```
$ mooncake lint -c mooncake.yml

mooncake.yml:1:1   warning  L001  step has no `name:` (renders as blank in output)
mooncake.yml:3:3   warning  L007  `shell: apt-get install ...` — use `pkg:` for idempotency
mooncake.yml:7:14  warning  L011  hardcoded `/tmp/` path — prefer `{{ tmpdir }}` or absolute project path
mooncake.yml:9:14  error    L022  string starting with `sk-` looks like an API key — use `!secret`
mooncake.yml:11:3  error    L030  `curl URL | bash` pattern — pipe-to-shell is unsafe; use `file.download` + `assert: file_sha256:`
mooncake.yml:13:1  warning  L001  step has no `name:`

6 issues found (2 errors, 4 warnings)
```

Exit codes:
- `0` — no issues
- `1` — warnings only
- `2` — errors

## Initial rule set (v1)

Pick rules that are concrete and false-positive-low:

| ID | Severity | Rule | Receipt |
|---|---|---|---|
| L001 | warning | step has no `name:` | proposal-01 pain |
| L002 | warning | `name:` longer than 60 chars | renders awkwardly |
| L003 | warning | `vars:` declares key not used in any subsequent step | dead code |
| L007 | warning | `shell: apt-get install` / `pacman -S` / `brew install` — use `pkg:` | better idempotency story |
| L008 | warning | `shell: systemctl ...` — use `os.service` | typed action exists |
| L011 | warning | hardcoded `/tmp/X` path — prefer `{{ user_home }}` or named project dir | reproducibility |
| L012 | info | `shell: cd X && ...` — use `as_user:` or step `working_dir:` if available | clarity |
| L015 | warning | `file.write` with literal `mode: "0777"` | overly permissive |
| L020 | error | step-level `creates:` on `file.write` (pre-MT-77, kept as compat alert) | rejected; or use action-level guard |
| L022 | error | string content matches `sk-` / `ghp_` / `xoxb-` / pasted PEM | likely secret; use `!secret` |
| L030 | error | `shell:` content matches `curl.*\|\s*bash` / `wget.*\|\s*sh` | supply-chain risk; use file.download + assert |
| L040 | info | unreachable step (always `when: false`) | dead code |

Each rule has a doc page in `docs-next/lint/<rule-id>.md` with
rationale, examples of bad vs. good, and how to disable.

## Configuration

`.mooncake-lint.toml` at project root:
```toml
[rules]
L011 = "warning"    # downgrade from default
L030 = "off"        # bypass (e.g., this is a known curl-bash for getting bootstrap)
L999 = "error"      # custom rule loaded from a plugin
```

CLI overrides:
```bash
mooncake lint --disable L011         # one-off
mooncake lint --severity-min error   # only errors, skip warnings
mooncake lint -c playbook.yml --format json  # machine-readable
```

## API

| Command | Behavior |
|---|---|
| `mooncake lint` | Lint default config |
| `mooncake lint -c <path>` | Lint specific file |
| `mooncake lint --format json` | JSON output |
| `mooncake lint --disable L0XX` | Skip a specific rule |
| `mooncake lint --severity-min {info\|warning\|error}` | Filter |
| `mooncake lint --explain L030` | Print the doc page for a rule |

## Integration points

1. **CI:** `mooncake lint && mooncake plan && mooncake apply` is the
   recommended pipeline. Lint is the cheapest of the three.
2. **Pre-commit hook:** Document a shell snippet in
   `docs/development/pre-commit-hook.md`.
3. **Editor integration:** If the IDE has a YAML language server,
   they can shell out to `mooncake lint --format json` on save.
4. **`mooncake watch`:** (proposal-03) — pre-run lint before each
   plan re-render.

## Receipts

Patterns I literally saw in my own ad-hoc test playbooks across 59
iterations (most schema-valid but anti-pattern):

| Pattern I wrote | Lint rule that would catch it |
|---|---|
| Unnamed steps everywhere | L001 |
| `shell: touch /tmp/x` instead of `file.write:` | L005 (new: prefer typed action over shell) |
| Hardcoded /tmp/mooncake-tests/... in dozens of tests | L011 |
| `creates:` at step level (pre-MT-77) | L020 |
| `vars:` with keys that I forgot to use | L003 |

## Implementation sketch

`internal/lint/`:
- `rules.go` — rule registry
- `rules/<id>.go` — one file per rule, implements `Rule` interface
- `report.go` — formatter (text + JSON)
- Hook into the same plan-load path; lint runs *after* validate but
  *before* apply

```go
type Rule interface {
    ID() string
    DefaultSeverity() Severity
    Check(plan *Plan) []Finding
    Doc() string
}
```

## What this doesn't address

- **Cross-file analysis** (lint rule that says "this preset's
  state: parameter isn't used by any step that calls it") —
  separate, harder.
- **Performance hints** (e.g., "this for_each over 1000 items
  should use a batched approach") — semantic; later.
- **Style** (vs lint) — `mooncake fmt` (auto-format playbooks)
  would be a sibling proposal. Out of scope here.

## Why dx, not core

Lint rules are *opinions about good practice*. Core has invariants
(things that MUST be true). DX has guidance (things that SHOULD be
true). Keep the distinction; route rule violations through the
formatters/recipes from proposal-02 / proposal-05.
