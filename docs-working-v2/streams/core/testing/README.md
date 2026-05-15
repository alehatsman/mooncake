# Stream: core — Manual Test Plan

Tests for the typed action vocabulary, planner, executor, and four-method
handler ABI. This is the kernel: one box, no agentd.

> Live findings from the 2026-05-15 manual-tester pass are in
> `docs-working/analysis/findings-2026-05-15/`. Treat the "Open gaps"
> list there as live regression targets.

## What to test

| Surface | What "correct" looks like |
|---|---|
| **Idempotency** | Run twice → second run shows `ok=N changed=0 failed=0` and the file/state is byte-identical |
| **Dry-run safety** | `apply --dry-run` produces a plan but writes nothing to disk and no shell side effects |
| **Schema validation** | Unknown fields are rejected at validate-time; `mooncake validate` and `mooncake apply` agree |
| **Plan reproducibility** | `plan -o plan.json` then `apply --from-plan plan.json` is bit-exact even if source changes |
| **Action result shape** | Every action returns `{changed, failed, rc, status, stdout, stderr}` plus action-specific typed fields |
| **Error attribution** | Failures point at file:line, name the offending field/value, suggest a fix |
| **Compound semantics** | `try/catch/finally` and `transaction:` honor LIFO rollback and reverse markers |
| **Reactive triggers** | `on_change:` fires only when parent reports `changed=true`; skips otherwise |

## Test environment recipe

```bash
# Build a portable static binary
CGO_ENABLED=0 go build -ldflags='-s -w' -o out/mooncake-static ./cmd

# Run in clean docker — vanilla ubuntu:24.04 or alpine:3.21
docker run --rm \
  -v $PWD/out/mooncake-static:/usr/local/bin/mooncake:ro \
  -v $PWD/presets:/work/presets:ro \
  -v /tmp/test-playbook:/work:rw \
  -w /work \
  ubuntu:24.04 bash -c '
    apt-get update -qq && apt-get install -y -qq ca-certificates sudo
    mooncake apply -c cfg.yml
  '
```

**Why ca-certificates**: any `file.download:` step (and presets that
fetch checksums) needs them. Static binary doesn't include a CA bundle.

**Why sudo**: most presets use `as_user: root`. Even running *as*
root, current pre-flight calls `sudo` — fixed for `as_user: root`
specifically (MT-1), but `as_user: <other_user>` still needs sudo
(#81).

## Per-action test pattern (the 4-shot recipe)

For every action in `mooncake actions list`, run these four shots:

```bash
# Shot 1 — happy path (clean run)
mooncake step "<action>: { <minimal valid params> }"

# Shot 2 — idempotent re-run
mooncake step "<same step>"
# Expected: changed: false; operation: noop (or equivalent)

# Shot 3 — invalid field (regression test for #44)
mooncake step "<action>: { ..., bogus_field: 1 }"
# Expected via `apply`: validator error naming the field
# (via `step` today: silently accepted — see #83)

# Shot 4 — failure mode (whatever breaks the action)
mooncake step "<action with bad input>"
# Expected: failed: true; error: <descriptive>; rc != 0
```

This shape catches the most common regressions in one pass.

## Tricks & tips

1. **Use `--output-format json` for ground truth.** Text output is a
   friendly summary that drops a lot. The JSON channel surfaces
   `step.stdout` events, full `result` map, error details, and event
   ordering. Verify against JSON; *show* text.

2. **`mooncake step` ≠ `mooncake apply` on validation.** Step is
   lax: bypasses the schema validator and goes straight to the
   handler. Use `step` to probe the action's own pre-flight; use
   `apply` to probe schema strictness. They should agree but don't
   today (see #83).

3. **Generate schema before writing test playbooks.** Don't trust
   docs:
   ```bash
   mooncake schema generate --output schema.json
   awk '/^    "ACTION_NAME": \{/,/^    \},/' schema.json
   ```
   Field names drift fast (`cmd:` vs `command:`, `url:` vs `repo:`,
   `sha256:` vs `checksum:`). The schema is the source of truth.

4. **Idempotency triggers are subtle.** A step with `creates: /file`
   and `unless: test -f /file` should both work. But:
   - For `shell:`, both go inside the action: `shell: { cmd: ..., creates: ... }`
   - For step-level on file.write: was broken (#15), fixed (MT-77)
   - Run both **with the guard's file present** and **absent**.

5. **Always verify on disk, not just by recap.** Many bugs were
   "recap green, file wrong" (#2, #8, #14, #15, #80). The recap is
   the rendered story; truth is in `cat /path` or `sha256sum`.

6. **Test the failure shape explicitly.** When a step fails, check:
   - `result.failed` is `true` (some actions set `false` while
     populating `error:` — see #61)
   - `result.error` is descriptive (some are generic "command
     failed with exit code 1")
   - Recap counter agrees (`failed=N`)
   - Process exits with non-zero status

7. **for_each, when, and on_change need 3 cases each.**
   - `for_each` empty list, single item, many items
   - `when` true, false, undefined
   - `on_change` parent changed, parent ok (skip), parent failed

8. **Concurrent runs are safe at the state level.** Five parallel
   applies don't corrupt `~/.mooncake/runs.jsonl`. But concurrent
   applies to the same target file race at the FS level — that's
   the user's problem.

## Common pitfalls

- **Confusing JSON-Unicode escape with HTML escape.** `<` rendered
  as `<` in JSON is the correct Go json.Marshal behavior.
  `&lt;` would be wrong (HTML entity). Decode the JSON before
  judging — see #16's resolution.

- **`mooncake step` strips structured payloads pre-MT-22.** If a test
  needs `repo.search.results[]` or `observe.cpu.value.cores`, use
  `mooncake apply --output-format json` and parse `step.completed.result`.

- **Read.* actions wrap content in `.value`.** Templates need
  `{{ cfg.value.service.port }}`, not `{{ cfg.service.port }}`. Docs
  don't say this loudly.

- **Bare `{{ map }}` used to leak `<map[string]interface {} Value>`**
  (#70). If you see this, you're on an old binary — rebuild.

- **Test idempotency with a *guard* on the seed step.**
  Otherwise the seed re-runs every iteration and you're testing
  recovery-from-seed, not steady-state idempotency.

## How to file findings

Findings filed during testing should land at:

```
docs-working/analysis/findings-<DATE>/
  silent-success-bugs.md      ← "green recap, broken behavior"
  ssot-drift.md                ← validator/schema/docs out of sync
  template-engine.md           ← rendering / escaping / metrics-in-templates
  coverage-gaps.md             ← preset/tool/action gaps
  cli-and-friction.md          ← UX nits, error-message quality
  positive-keepers.md          ← features to feature; do-not-regress list
```

When the author lands a fix, update the original finding with a
`✅ FIXED (round N, commit <hash>)` block. Don't delete the original
report — future agents need the receipt.

## Concrete priority targets

If you have one hour, run these:

1. **Round-trip every action via `step` + verify result shape**
   (regression test for #22 / #83)
2. **Run `examples/loops/with-items.yml` and check `{{ item }}`
   substitution** (regression test for #8)
3. **Run `examples/transactions/rollback-demo.yml` and verify the
   `↺ Reverse:` markers + `reverted=N` recap counter** (regression
   test for #45)
4. **Generate a 5000-step playbook and verify <20s wall-clock**
   (regression test for kernel scaling)
5. **Apply twice; check all `changed_steps` go to 0 on the second
   run** (regression test for idempotency-at-recap)
