# Stream: dx — Manual Test Plan

Tests for the developer-experience surface: `init`, `doctor`,
`history`, `presets recommend`, default config discovery, error
message quality. The first-five-minutes story.

> **Audience for this stream**: a new user who just typed
> `brew install mooncake` (or `curl install.sh`). They have not read
> the docs. Test the path *they* will take, not the path you know works.

## What to test

| Surface | What "correct" looks like |
|---|---|
| **`mooncake init`** | Scaffolds `mooncake.yml`, `mooncake.vars.yml`, `.gitignore`, `.mooncake/` — generated playbook applies cleanly on first run |
| **`mooncake init --non-interactive --template <name>`** | Works in CI; no prompts; templates: `empty`, `dotfiles`, `server`, `agent-sandbox` |
| **`mooncake doctor`** | Sections (Install / System / State / Presets / Tools) with ✓ / ℹ / ⚠ glyphs and **actionable fix suggestions with URLs** |
| **`mooncake history list`** | Newest-first numbered list; `--format json` is valid JSONL; `show <N>` returns one entry |
| **`mooncake apply` with no config** | Friendly error: "no mooncake config found in /; searched ./mooncake.yml ./mooncake/main.yml" with scaffold suggestion |
| **Validator errors** | File:line attribution; quoted step source; vocabulary lists 60+ actions (closes #27); "renamed-field" hint for typos |
| **Plan output** | `--diff` shows unified diff; `--show-origins` adds file:line:col; `--no-inspect` skips per-step state |
| **`mooncake apply --dry-run`** | Per-step `risk N • reversible • N resource • N bytes` annotation; PLAN SUMMARY at bottom; **zero side effects** |
| **First-run hint** | After successful first apply: "★ First run — nice. Try `mooncake plan` ..." |

## Test environment recipe

```bash
CGO_ENABLED=0 go build -ldflags='-s -w' -o out/mooncake-static ./cmd

# Fresh container = first-time-user simulation
docker run --rm -it \
  -v $PWD/out/mooncake-static:/usr/local/bin/mooncake:ro \
  -w /home/test \
  ubuntu:24.04 bash
```

**Why `-it`**: many DX surfaces probe for TTY (init's interactive
mode, ask-become-pass). Test both with and without `-it`.

## Test scenarios

### 1. Onboarding cold start (10 min)

Imagine a new user. Run these in order:

```bash
mooncake apply                   # no config — friendly error?
mooncake doctor                  # what's the state of this box?
mooncake init                    # interactive: needs TTY
mooncake init --non-interactive --template empty
mooncake apply                   # generated playbook should just work
mooncake history                 # one entry, my first run
mooncake apply                   # second run should be 100% idempotent
```

**Verify each output**:
- `apply` no-config error names BOTH searched paths
- `doctor` shows fix URLs for missing tools
- `init` complains about no-TTY clearly (don't silently fail)
- Generated `mooncake.yml` is valid and applies
- `history` is empty before first run; one entry after
- Second `apply` shows all `ok` (no `changed`)

### 2. Error message taste test (15 min)

A *good* error tells the user WHAT failed, WHY, and HOW to fix it.

Trigger each of these and rate the error 1–5 stars:

```bash
# Bad action name
echo '- nonsense_action: { x: 1 }' > cfg.yml
mooncake apply -c cfg.yml
# Expected: "unknown field nonsense_action (likely a typo or a renamed
#   field — see docs-next/guide/config/actions.md)"
#   plus the action vocabulary

# Top-level dict instead of list
cat > cfg.yml <<'EOF'
log:
  msg: hi
EOF
# Current: confusing "unknown field log" — known issue #72

# Missing required field
echo '- text.delete_range: { path: /tmp/x }' > cfg.yml
mooncake apply -c cfg.yml
# Expected (gold standard):
#   "validation failed: ... start_anchor is required
#    Required parameters:
#      - end_anchor: string
#      - path: string
#      - start_anchor: string ← MISSING
#    Optional parameters: ..."

# Bad YAML (tabs)
printf -- '- log:\n\tmsg: hi\n' > cfg.yml
mooncake apply -c cfg.yml
# Expected: yaml: line N: found character that cannot start any token

# Wrong file path
mooncake apply -c /no/such/file.yml
# Expected: clear "file not found" with path

# Stale plan
mooncake plan -o plan.json
mv mooncake.yml mooncake.yml.bak
mooncake apply --from-plan plan.json
# Expected: "refusing to apply stale plan: plan input files have
#   changed since the plan was built (use --allow-stale to override)"
```

Anything below 4 stars is a finding to file.

### 3. `mooncake doctor` exhaustive (15 min)

Doctor's job is to diagnose without being asked. Test each rung:

```bash
# Fresh box
mooncake doctor

# Verify these rungs:
# ✓ mooncake dev installed at /usr/local/bin/mooncake
# ℹ Go runtime: ...
# ✓ os=linux arch=amd64 distribution=ubuntu package_manager=apt
# ℹ /root/.mooncake does not exist (will be created on first run)
#       fix: run mooncake apply ...
# ℹ no run history yet
# ⚠ no presets found in any search path
#       fix: install the mooncake-presets package ...
# ⚠ git not on PATH
#       fix: install git — https://git-scm.com/downloads
#       used by: git.* actions
# ⚠ fzf not on PATH
#       fix: install fzf — ...
#       used by: mooncake presets (interactive selector)
# ⚠ sudo not on PATH
```

Each rung should:
- Use the right glyph (✓ ℹ ⚠) — not ✗ for not-yet-set-up
- Have a `fix:` line when actionable
- Have a `used by:` line when the missing tool's use is non-obvious
- Have URLs for external tools

**Known gap (#71)**: doctor reports `disk-space probe unsupported on this OS` on Linux — `mooncake facts` works on the same machine. Fix doctor's probe to use the same code path.

### 4. `apply --dry-run` safety (5 min)

```bash
cat > cfg.yml <<'EOF'
- file.write: { path: /tmp/dry-marker, content: "v1" }
- shell: echo running > /tmp/dry-side-effect
EOF

rm -f /tmp/dry-*
mooncake apply -c cfg.yml --dry-run
# Verify:
# - "would create file (3 bytes)" / "would run: echo ..." per step
# - "risk N (routine) • reversible • 1 resource • N bytes" cost line
# - PLAN SUMMARY with would-change=2 ok=0 skipped=0 failed=0
ls /tmp/dry-marker /tmp/dry-side-effect 2>&1
# Expected: BOTH absent — zero side effects
```

Regression target. If `--dry-run` ever writes anything, that's a
P0 fix (this is the safety story the README promises).

### 5. `apply --output-format json` channel completeness (5 min)

```bash
mooncake apply -c examples/hello-world/config.yml --output-format json | jq -c '{type, step_name: .data.name}'
```

Should see this sequence:
```json
{"type": "run.started"}
{"type": "plan.loaded"}
{"type": "step.started", "step_name": "Print hello message"}
{"type": "step.stdout"}                       ← stdout line
{"type": "step.completed", "step_name": "..."}
{"type": "run.completed"}
```

Verify `step.stdout` events appear (regression test for #5). Verify
`step.completed.data.result` has full `{changed, failed, rc, stdout,
stderr, status, duration_ms}`.

## Tricks & tips

1. **Always test the no-TTY path.** Most users run mooncake in CI.
   `init` should refuse interactive mode cleanly with a hint; doctor
   should produce its output regardless; `--ask-become-pass` without
   TTY should error early (currently doesn't — #85).

2. **Use a *fresh* container per test.** Past state in
   `~/.mooncake/` (runs.jsonl, agentd.token) leaks across runs and
   confounds DX tests. `docker run --rm` is your friend.

3. **Test on alpine too.** Tools-missing branches (`apt_available:
   false`, `apk_available: true`) hit different code paths. Many
   bugs surface only on the platform a doctor's check claims
   "supported".

4. **The first-run star tip is brittle.** It's printed verbatim on
   first `apply`. If you regress it, users miss the "next steps"
   prompt. Test on a fresh `~/.mooncake/`.

5. **Validate error-message taste with a non-mooncake user.** If
   you can't get a teammate, read it aloud. Errors that
   reference internal terminology (`expansion context`, `cell ABI`)
   are unhelpful; errors that name a file/line/key are.

6. **Doctor is the right template** for every diagnostic surface.
   Copy its glyph + section + fix-line shape into validator
   errors, fleet doctor, agentd startup logs. Consistency >
   cleverness.

## Common pitfalls

- **Don't confuse `--format` and `--output-format`.** Most commands
  use `--format`; `apply` and `runs apply` use `--output-format`
  (#68). Bind a habit: when in doubt, `--help | grep -E '(--format|--output)'`.

- **The "renamed field" hint** in validator errors is part of MT-77.
  Don't break it during refactors — it converts mystery into action.

- **`mooncake init` non-interactive REQUIRES `--template`**. There's
  no default. The error message says so, but it's an easy miss.

## How to file findings

Same convention as the other streams. DX bugs almost always belong
in `cli-and-friction.md` (error messages, flag interaction, hints).

## Concrete priority targets

If you have one hour:

1. **Cold-start onboarding** — every command from `apply` (no
   config) through `init` → first `apply` → `history`
2. **Error message taste test** — six trigger conditions, rate each
3. **`doctor` on a vanilla container** — verify all sections, all
   fix hints, no false "unsupported" claims
4. **`--dry-run` zero-side-effect verification** — regression for
   the safety story
5. **`--output-format json` event completeness** — regression for
   the JSON channel as the "real" API
