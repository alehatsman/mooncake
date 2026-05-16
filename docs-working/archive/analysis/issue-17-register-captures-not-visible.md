# Bug — `register:` captured outputs invisible to subsequent template renders

**Tracking:** [#17](https://github.com/alehatsman/mooncake/issues/17)
**Surfaced:** 2026-05-15 during the post-master-sync fleet-command test
sweep.

## Repro — straight from `docs-next/examples/actions/shell.yml`

```yaml
- name: "Capture command output"
  shell: whoami
  register: current_user

- name: "Use captured output"
  shell: echo "Running as user $(whoami); rendered = '{{ current_user.stdout }}'" \
         > /tmp/register-probe.log
```

```sh
$ mooncake apply -c register.yml
~ Capture command output
▶ Use captured output
~ Use captured output
RECAP  ok=0  changed=2  skipped=0  failed=0  26ms

$ cat /tmp/register-probe.log
Running as user alehatsman; rendered = ''
```

`$(whoami)` (live shell exec) returns the user, but the rendered
`{{ current_user.stdout }}` template variable comes out empty. Every
namespace form tried renders empty:

| Template               | Rendered |
|------------------------|----------|
| `{{ r }}`              | `''`     |
| `{{ r.stdout }}`       | `''`     |
| `{{ r.result.stdout }}`| `''`     |
| `{{ registered.r }}`   | `''`     |
| `{{ result.stdout }}`  | `''`     |

The data captured by `register:` is somewhere — the shell step did
run and produce stdout — but it's not exposed to the renderer's
variable scope by any of the documented or guessable paths.

Verified on:
- Local `mooncake apply -c <plan>`
- `mooncake fleet apply <plan>` against linux/windows peers (same
  failure mode)

## Why this matters

`register:` is the load-bearing primitive for:
- LLM-agent workflows (capture artifact.capture metadata, branch on
  it). See `examples/llm-agent-workflow.yml` which uses `{{
  summary.stdout }}` and `{{ feedback.stdout }}` as central control
  flow.
- `observe.*` actions (spec-59) — the whole point is observing
  state and reacting. If the observation can't be read back into
  the next step's template, the typed-observability seed is
  un-consumable from the YAML side.
- `changed_when` / `failed_when` predicates that consult the prior
  step's stdout.
- spec-37 step output capture (just shipped) — surely meant to
  feed register.

If the bug is local-apply-only, this is regression on a
documented feature. If it's fleet-apply too (confirmed yes), the
fleet runtime never had it working.

## Root cause — hypotheses

Without diving into the renderer source:

1. **Render scope omission.** The executor stores the registered
   result somewhere (`ctx.RegisteredResults` or similar) but the
   template Renderer.GetVariables() doesn't merge it into the
   scope it hands to Pongo2.
2. **Pongo2 vs Go-reflect attribute access.** Maybe the stored
   value is `actions.Result` (an interface) and Pongo2's reflect
   dot-access doesn't see methods/fields correctly. (But empty
   string is the wrong failure mode for "not accessible" — Pongo2
   usually renders the literal text.)
3. **Spec-37 step output capture refactor regressed it.** Recent
   commits include `feat(spec-37): step output capture` (`actions/log` D3, etc.).
   If the executor now routes outputs through a different
   pipeline, the old register: wiring may have been disconnected.

## Test gap

There's no `register_e2e_test.go` in `internal/executor/` that
asserts `{{ name.stdout }}` is a non-empty string after a shell
step. The documented example in `docs-next/examples/actions/shell.yml`
isn't exercised by any test (it's documentation, not a fixture).

A high-value add: a smoke test that:
1. Runs `shell: whoami` with `register: x`
2. Runs `assert: that=x.stdout != ""`

Would catch this regression instantly.

## Workaround

For shell output, write to a file in step 1, read it back in
step 2 via `read.json` (when string) or via `shell: $(cat /tmp/x)`
inline. Ugly but works.

For observe.*, no workaround — the data structure is in-memory only.
Effectively means observe.* actions can only be used for
side-effect signalling (success/fail) until register: is fixed.

## Reproducer — single file

```yaml
# register-bug.yml — fails with `mooncake apply -c register-bug.yml`
- name: capture
  shell: whoami
  register: r

- name: probe
  shell:
    cmd: echo "stdout='{{ r.stdout }}'" >&2
```

Expected: stderr contains `stdout='alehatsman'`
Actual:   stderr contains `stdout=''`
