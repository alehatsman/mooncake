# Proposal 02: Output middle ground — `--output-format readable` (default) showing more, the right amount

**Status:** Draft proposal
**Effort:** S (~2 days, single PR plus tests)
**Value:** High — fixes the dominant "I can't see what happened"
complaint without breaking anything. Touches every `apply` user.

---

## Problem

Today `mooncake apply` has two output modes:

| Mode | Detail | Use |
|---|---|---|
| **text** (default) | Step glyphs (`▶ ~ ✓ ✗ -`), recap, run-completion message | Humans glancing |
| **json** | Newline-delimited JSON: `run.started`, `step.started`, `step.stdout`, `step.completed`, `run.completed` | Machines |

Neither serves the dominant case: **a human iterating on a playbook**.

What text mode drops:
- Shell stdout (`echo "Hello"` runs but its output is invisible — finding #5)
- Step IDs (no way to cross-reference with history / artifacts)
- Action types for unnamed steps (proposal-01)
- Per-step duration
- Per-step risk / reversibility metadata (`plan` shows this; `apply` doesn't)
- `result.error` content for soft failures (only the top-line message)

What JSON mode adds (often too much):
- Every `step.stdout` line as a separate event
- Per-event timestamps
- Full result map per step (most fields zero/empty for `log:`)
- Verbose for a human watching a 10-step playbook

Receipts:
- Finding #5: shell stdout missing in text mode entirely (now
  surfaces at `-l debug` per MT-5, but default-level still drops it)
- Finding #36: JSON channel is richer than text; that hint should
  flow back into the text formatter
- Finding #55: agentd-streamed runs hide step names too
- 50+ test iterations where I had to grep `step.completed` in JSON
  to see the real outcome

## Proposal

A third mode `--output-format readable` (becomes the new default),
inheriting text's brevity but surfacing the missing signal:

```
▶ step-0001  shell echo "Hello from Mooncake!"        2ms
  | Hello from Mooncake!
~ step-0002  file.write /tmp/marker.txt (12 bytes)    1ms
~ step-0003  shell echo "OS: $(uname -s)"             3ms
  | OS: Linux
  | Arch: x86_64

RECAP  ok=0  changed=3  skipped=0  failed=0  6ms  ★ first run
```

Changes from current `text`:
1. **Step ID column** (`step-0001`) — visible reference for history/artifacts
2. **Synthesized label** when name is missing (proposal-01)
3. **Per-step duration** right-aligned
4. **Captured stdout** indented under the step with `|` prefix (already what `-l debug` does — promote to default)
5. **Bytes / count summary** for file actions (already in plan output, propagate to apply)
6. **Recap on one line** instead of multi-line

Add a `--output-format compact` mode for the current minimal output
(scripts that grep `RECAP failed=`).

Keep `--output-format json` unchanged.

## Decision tree

```
Want machine-readable JSONL?           --output-format json
Want a one-line per step + recap?      --output-format compact
Anything else (default)?               --output-format readable
```

## API

| Flag | Behavior |
|---|---|
| `--output-format readable` | New default. Replaces current text mode. |
| `--output-format compact` | Current text mode (the bare `▶ ~ ✓`). |
| `--output-format json` | Unchanged. |
| `-l debug` | Adds the existing debug prefix lines on top of readable. |

`mooncake.yml.lock` or shell env `MOONCAKE_OUTPUT_FORMAT=compact`
to pin a project default. (Already a half-pattern via `MOONCAKE_HOST`.)

## Receipts

What I actually did during testing:
- For 30+ iterations, I ran `--output-format json` and piped to
  `jq` or `grep print.message` to see what really happened.
- Several findings (#5, #36, #55) trace to "stdout invisible in
  text mode".
- Finding #2 / #15 / #80 ("silent green recap") would have been
  caught faster with per-step duration + bytes — a 0-byte
  "changed" file.write step stands out vs. a real one.

## What this also unlocks

- **History `show <N>` can mirror this format**. Today
  `history show 1` is identical to `history` (no detail). Reusing
  the readable formatter inside `history show` gives users a
  per-step replay for free.
- **Artifact `stdout.log` already prefixes `[step-NNNN]`**. Same
  shape; share the formatter.
- **Plan output can match.** Today `plan` and `apply` use different
  formatters; unifying would let `plan --diff` and `apply --diff`
  feel like the same surface.

## Implementation sketch

A renderer family in `internal/render/`:
- `render/compact.go` — current text
- `render/readable.go` — new default
- `render/json.go` — existing
- `render/history.go` — calls into readable for `history show`

Each consumes the `events.jsonl` stream (already produced
internally per artifact bundle finding) and emits the chosen
format.

## What this doesn't address

- **TTY-only animation** (`--tui` mode) is orthogonal. Keep it.
- **Localization** of labels — same caveat as proposal-01.
- **`--output-format json` PascalCase keys in `validate`**
  (finding #69) — separate cleanup, not on the apply path.

## Risk

This **changes the default**. Existing scripts grepping the text
output (e.g., CI scripts that look for `RECAP ... failed=`) keep
working — `RECAP` stays. But screenscrapers that depend on the
exact step-row format break. Mitigation:
- Pre-announce in release notes
- Provide `MOONCAKE_OUTPUT_FORMAT=compact` for one release cycle
- Default flips to `readable` in next minor

Given how few users have automated against text output (most use
`--output-format json` already, per the JSON channel's
existence), this risk is low.
