# Proposal 01: Step name defaults — render action+content when `name:` is omitted

**Status:** Draft proposal
**Effort:** XS (~half a day, single PR)
**Value:** Medium — the most visible papercut in the CLI; every
unnamed step prints as `▶ ` with nothing after the glyph, costing
the user a glance every time they scan output.

---

## Problem

When a step has no `name:`, the renderer emits `▶ ` (and `~` / `✓` /
`✗`) with **an empty body**. The user has to count steps mentally
and cross-reference with the YAML file to know what's running.

Receipts from the 2026-05-15 manual-test pass (59 iterations):

```
$ mooncake apply -c throwaway.yml
▶ 
~ 
▶ 
✓ 
▶ 
✗ 
  no matching process

RECAP  ok=1  changed=1  skipped=0  failed=1
```

Three completely opaque steps. Throwaway YAML (testing, debugging,
LLM-generated configs) very often has no `name:` because the user
hasn't reached the "polish" pass. The output is unreadable until
they add names.

The artifact bundle already does this right: `stdout.log` uses
`[step-NNNN] line` as the line prefix. Same idea for the human
text formatter.

Also observed during fleet exec testing:
```
[local] submitted run 01KRPK1126M8B63M63SKYQVCM5
[local]   ▸ fleet-exec        ← server-side step name shipped along
[local]       from-peer
[local]     ✔ fleet-exec
```

Fleet remote-exec defaults the step name to `fleet-exec`. The same
generative default could apply locally.

## Proposal

When `name:` is unset, render a synthesized label using
`<action-type>` + a brief key parameter:

| Step | Synthesized label |
|---|---|
| `- shell: echo hi` | `shell: echo hi` |
| `- shell: { cmd: "long command...", as: ... }` | `shell: long command…` (truncate at column 60) |
| `- file.write: { path: /tmp/x.txt, ... }` | `file.write /tmp/x.txt` |
| `- file.download: { url: https://..., dest: /opt/foo }` | `file.download → /opt/foo` |
| `- pkg: { name: jq }` | `pkg jq` |
| `- log: { msg: "hello" }` | `log: hello` |
| `- assert: { command: ... }` | `assert command` |
| `- transaction: [...]` | `transaction (3 children)` |
| `- try: [...]` | `try (2 children, catch+finally)` |
| `- vars: { a: 1, b: 2 }` | `vars (2 keys)` |
| `- import: tasks/x.yml` | `import tasks/x.yml` |

Rule of thumb:
- **Identity action** (action type + the value being acted on)
- **Truncate** at column ~60 with `…`
- **Use `(N children)` for compound actions** (transaction/try)
- **Use a colon** before short content (`log:`, `shell:`); a space
  before paths (`file.write /tmp/x`).
- **Never** synthesize a name longer than the rendered glyph row's
  budget.

## API

No CLI flag. Always-on, transparent default. If the user provides
`name:`, that wins. If not, the renderer synthesizes.

The synthesized label is also written to the JSON channel as
`step.completed.data.name` if `name:` was unset — agents reading
JSON should see the same string the human renderer prints.

## Receipts (where I felt the pain)

In the 2026-05-15 audit, ~30% of test playbooks I wrote omitted
`name:` because they were one-shot probes. Every one produced
output with blank step rows. Reading the recap, I'd then have to
re-open the YAML to figure out which step changed/failed.

Specific examples from findings:
- **Round 4** (broken-template / missing-include) — three failure
  scenarios, all without names. The error pointed at "in step:
  ensure /tmp/idem dir" — a name from a *previous* edit (the file
  had been rewritten). With synthesized labels, the error attribution
  could fall back to the action+content of the actual current step.
- **Round 15** (`mooncake runs apply`) — agentd-streamed output also
  has blank-name steps: `[local]   ▸ ` with nothing after.
  Filed as #55.
- **Round 41** (large playbooks) — 5000-step playbook auto-generated
  via `for i in $(seq 1 5000)`; literally every step lacks a name.
  Output is illegible without scrolling back to the YAML.

## Risks / non-goals

- **Don't reformat user-provided names.** If `name: "Install Foo"`
  exists, leave it alone — even if it's long.
- **Don't add this to plan JSON.** The synthesized label is a *render*
  artifact, not a stored one. JSON plan files should keep `name:
  ""` when the user wrote no name.
- **No localization yet** — synthesized labels are English-only.
  Future i18n can map action types to localized verbs.

## Implementation sketch

A new helper in the renderer:
```go
func (r *TextRenderer) stepLabel(step *plan.Step) string {
    if step.Name != "" {
        return step.Name
    }
    return synthesizeName(step.ActionType, step.RawAction)
}
```

`synthesizeName` is a small per-action-type map of formatters,
defaulting to `<action-type>` when no formatter is registered.
Lives in `internal/text/labels.go` (or similar). Reachable from
both the run renderer and the plan renderer.

## What this doesn't address

- Step IDs (`step-0001`) are already in the JSON channel; they
  should also appear in the text channel for cross-reference (e.g.
  in error attribution and history `show`). Separate proposal —
  see [proposal-02](./proposal-02-output-middle-ground.md).
