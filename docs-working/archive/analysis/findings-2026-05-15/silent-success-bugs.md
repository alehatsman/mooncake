# Silent-Success Bugs

The dominant pattern in this audit. The recap reports `failed=0` /
green / "changed=N", but the action either did nothing useful or did
something unverified. **The largest cluster of findings.**

> Start fixes from this file. These bugs evade CI because CI sees
> green. They surface in production.

**Post-2026-05-17 status: all entries in this file are ✅ FIXED.** See the
[summary table](#summary-table) at the bottom for per-finding commit refs.
Original reports kept below each ✅ banner for context.

---

## #8 — `for_each` does not iterate; `{{ item }}` resolves to the Go reflect.Value of the slice — CRITICAL

**Repro**: the upstream example, untouched.

```
$ docker run --rm -v $PWD/out/mooncake-static:/usr/local/bin/mooncake:ro \
    -v $PWD/examples/loops:/work:ro -w /work ubuntu:24.04 \
    mooncake apply -c /work/with-items.yml
▶ <[]interface
~ <[]interface
▶ {}
~ {}
▶ Value>
~ Value>
... (repeats 3× for the 3 for_each blocks)
RECAP  ok=0  changed=9  skipped=0  failed=0
```

Plan view confirms:
```
↑ Install package      would run: echo "Installing <[]interface"
↑ Install package      would run: echo "Installing {}"
↑ Install package      would run: echo "Installing Value>"
```

**What's happening**: `{{ item }}` inside `for_each` does NOT bind to
each slice element. It is bound to `fmt.Sprintf("%v",
reflectValueOfTheWholeSlice)`, which renders as `<[]interface {}
Value>`. That string is then **tokenized on whitespace** into 3
fragments, and each iteration receives one fragment.

Two distinct sub-bugs stack:
1. `{{ item }}` binds to the slice value, not each element.
2. The `apply` text view replaces the step's `name:` with the bad
   item value entirely (plan view preserves `name:` correctly — only
   `apply`'s renderer clobbers it).

**Why CRITICAL**: `for_each` is THE primary iteration primitive in a
config-management language. Every preset that loops (over packages,
users, paths) is silently miscompiling. Recap shows `failed=0` so CI
passes while doing nothing useful.

**Where to look**: `internal/plan/planner.go` for-each expansion;
the variable binding into per-step `ExpansionContext.Variables`.
Likely `item` is set to the slice value rather than the element.

---

## #14 — `file.download` silently ignores `sha256:` — CRITICAL (security)

**Repro**: declare `sha256: "0000…0000"` for a known-good binary URL.

```
$ docker run --rm -v $PWD/out/mooncake-static:/usr/local/bin/mooncake:ro \
    -v /tmp/mooncake-tests/download:/work:rw -w /work ubuntu:24.04 bash -c '
    mkdir -p /tmp/dl
    mooncake apply -c bad-sum.yml -l debug'
▶ download with WRONG sha256
  Downloading: https://github.com/.../jq-linux-amd64 -> /tmp/dl/jq-bad
Failed to close temp file: close /tmp/...-2571834238: file already closed
Failed to remove temp file: ... no such file or directory
~ download with WRONG sha256
RECAP  ok=0  changed=1  skipped=0  failed=0  199ms
```

`ls /tmp/dl/jq-bad` → file present (2,319,424 bytes).
`sha256sum /tmp/dl/jq-bad` → `5942c9b…` (real jq hash), **not**
`0000…` (declared).

**Why CRITICAL**: `sha256:` defends against URL tampering / MITM /
swap. Silently bypassed → every preset that downloads a binary with
checksum is providing illusion of safety. `LLM_GUIDE.md` recommends
"Binary download + checksum" as install tier 3 — the recommendation
pattern itself is broken.

**Likely cause**: verify step runs AFTER rename. When verify fails,
it tries to clean up a temp file that no longer exists ("file already
closed" / "no such file or directory" in debug logs). Net effect: bad
bytes land, cleanup error swallowed, action returns success.

**Edge case observed**: when destination *parent dir* doesn't exist,
rename fails first and the failure is masked as `failed to move file:
rename ... no such file or directory`. Different shape, same root
cause.

**Fix order**:
1. Verify checksum on the temp file **before** rename.
2. On mismatch, return `ChecksumMismatchError { expected, got, url }`
   — not a generic rename failure.
3. Treat cleanup errors as bugs in cleanup, not user-facing
   success/failure.

**Reuse opportunity**: `assert: file_sha256:` already does correct
sha256 verification (see [`positive-keepers.md`](./positive-keepers.md)).
Same logic should be called from `file.download` before rename.

---

## #15 — ✅ RESOLVED (round 29): silently-ignored → now validator-rejected

Step-level `creates:` / `unless:` on `file.write` are now rejected at
validate time (the validator complains "exactly one action" — see
[`cli-and-friction.md` #77](./cli-and-friction.md#77) for that
error-message bug). Either way: no more silent bypass. Users learn
about the issue immediately.

The remaining open question is *what's the right syntax for guarded
file.write*. Options:
- Nest creates under shell: only (status quo for shell; doesn't help file.write)
- Add `state: file_if_absent` enum value to file.write (idempotent by
  construction)
- Step-level `creates:` for all actions (the original promise)

Pick one; document it.

---

### Original report (now resolved)

**Repro**:
```yaml
- file.write:
    path: /tmp/text/guarded.txt
    state: file
    content: "v1\n"
  creates: /tmp/text/guarded.txt
```

```
$ echo "v0-already-here" > /tmp/text/guarded.txt
$ mooncake apply -c creates-fw.yml
$ cat /tmp/text/guarded.txt
v1
```

Same with `unless: test -f /tmp/text/guarded.txt` — file clobbered
anyway.

**Why HIGH**: `creates:` is the documented idempotency primitive.
Users write `creates: /opt/app/installed` to guard a download-and-
install step. For `file.write` the guard does nothing — the step
always re-runs.

**Likely cause**: `creates:` / `unless:` honored by the executor's
shell pre-check path, but file actions short-circuit through their
own state-check without consulting the step's `creates:` / `unless:`
keys.

**Wider implication**: re-check all non-shell actions for `creates:`
/ `unless:` honor. `text.replace`, `text.insert`, `pkg`, `file.copy`,
etc. — every action with custom state logic likely has the same gap.

---

## #2 — `shell` with `creates:` / `unless:` reports `changed` instead of `skipped` — HIGH

**Repro**: run a playbook twice where the shell step has
`creates: /tmp/once.flag`.

Run 2 output:
```
▶ shell with creates guard
~ shell with creates guard
▶ shell with unless guard
~ shell with unless guard
RECAP  ok=2  changed=2  skipped=0  failed=0
```

The guard IS honored at the FS level — mtime of `once.flag` doesn't
move, content of `unless.txt` unchanged. But the recap miscounts and
the line marker is `~ changed`, not `- skipped`.

**Why HIGH (silent correctness)**: `creates:` and `unless:` are the
documented idempotency escape hatch. If they're reported as `changed`,
fleet drift checks and CI watch loops see constant noise that
contradicts the "second run = no changes" guarantee.

**Fix**: in shell action handler, when `creates`/`unless` precheck
short-circuits, set result status to `skipped`, not `changed`.

---

## #22 — ✅ FIXED (commit `c6e327e`, verified 2026-05-16)

`mooncake step` now emits `RegisteredResult.ToMap()` — the same shape
`apply --output-format json` exposes under `result.*`:

```
$ mkdir -p /tmp/mt22-probe && echo "hello world" > /tmp/mt22-probe/file.txt
$ mooncake step "repo.search: { path: /tmp/mt22-probe, pattern: hello }"
{
  "action": "repo.search",
  "changed": false,
  "duration_ms": 0,
  "failed": false,
  "rc": 0,
  "results": [
    {"file": "file.txt", "line": 1, "column": 1,
     "match": "hello", "context": "hello world"}
  ],
  "skipped": false,
  "stderr": "",
  "stdout": "",
  "total_files": 1,
  "total_matches": 1
}
```

Test coverage in `cmd/step_test.go`:
`TestBuildStepJSON_SurfacesActionDataMap` (the headline regression),
`TestBuildStepJSON_ShellShapePreserved` (shell still works),
`TestBuildStepJSON_PopulatesActionEvenOnNilResult`,
`TestBuildStepJSON_OmitsErrorWhenNil`,
`TestBuildStepJSON_DataDoesNotShadowSharedScalars`. Don't regress.

### Original report (now resolved)

**Repro**:
```
$ echo "hello" > /tmp/probe.txt
$ mooncake step "repo.search: { path: /tmp, pattern: hello }"
{
  "changed": false,
  "action": "repo.search",
  "duration_ms": 0
}
```

But via `apply --output-format json` the same action returns:
```json
{
  "results":[{"file":"probe.txt","line":1,"column":1,"match":"hello",
              "context":"hello"}],
  "total_files":1,
  "total_matches":1,
  ...
}
```

**Why HIGH**: `mooncake step`'s help text says "Execute a single
inline step and return JSON result". But for typed actions
(`repo.search`, `repo.tree`, `text.replace`, etc.) the structured
payload is dropped. An agent calling `step` for `repo.search` gets
no way to know what was found. `shell` actions DO surface `stdout` —
so the schema is shell-special-cased, not action-generic.

**Fix**: `mooncake step`'s JSON should include the full action result
map, not a manually-selected subset. Whatever `apply --output-format
json` emits under `result.*` should be available verbatim in `step`'s
output.

---

## #24 — ✅ FIXED (commit `ee125177`, verified 2026-05-17)

`artifact.capture` now records file changes from `file.write`, `text.replace`,
and friends; tracker-after-flush contract pinned in `8c2c69f2`. `initial_vars`
noise (CPU flags etc.) trimmed.

### Original report (now resolved)

## #24 (original) — `artifact.capture` reports 0 file changes across all action types — HIGH

**Repro 1** — file.write inside artifact.capture:

```yaml
- artifact.capture:
    name: test
    output_dir: /tmp/artifact-out
    format: both
    capture_content: true
    steps:
    - file.write:
        path: /tmp/artifact-target/config.txt
        state: file
        content: "v1\n"
```

Output:
```
Captured 0 file changes
Artifact capture complete: 0 files changed
```

SUMMARY.md: `Total Files: 0, Files Created: 0` — even though
`cat /tmp/artifact-target/config.txt` → `v1`.

**Repro 2** — text.replace inside artifact.capture: same result. 0
changes recorded, real text.replace executed successfully.

**Why HIGH**: entire purpose of `artifact.capture` is to record what
the wrapped steps changed — the AI-agent feedback loop. Recording
zero changes when changes happened defeats the purpose. From
`examples/artifact-capture-example.yml`: "for LLM agent loops" — the
loop has no signal.

**Also observed**: `initial_vars` dump in `changes.json` includes the
entire CPU flags list (100+ items per artifact). Should be cleaner —
either omit `initial_vars` entirely or have an opt-in
`capture_facts` flag.

---

## #28 — ✅ FIXED (commit `bcaf9392`, verified 2026-05-17)

`assert` results now route through the executor wrapper that respects
`failed_when:`. `failed_when: false` correctly suppresses a failing
assertion; recap counts as `ok`, not `failed`.

### Original report (now resolved)

## #28 (original) — `failed_when: false` does not suppress assertion failure — MEDIUM (semantics)

**Repro**:
```yaml
- name: assert sha256 (intentionally wrong)
  assert:
    file_sha256:
      path: /tmp/assert-test.txt
      checksum: "0000…0000"
  failed_when: false
```

Recap: `ok=5 changed=1 skipped=0 failed=1`. The step is still counted
as failed despite the explicit override.

**Why MEDIUM**: `failed_when:` is the documented escape hatch for
"this step might fail but I don't want it to gate the run". If
`failed_when: false` doesn't suppress, the escape hatch doesn't exist
for assert. Probably the assert handler short-circuits with its own
failure path before the executor's `failed_when` is evaluated.

**Fix**: assert (and any other action that hardcodes failure) should
write its result through the standard executor wrapper that respects
`failed_when`.

---

## #40 — ✅ FIXED (commit `cb6b21bb`, verified 2026-05-17)

`tool: backend: github-release` (and `archive-url`) now infer bare-binary vs
archive from the asset filename; bare binaries are renamed + chmod +x and
installed without extraction. Closes the jq / hadolint / kind class of cases.

### Original report (now resolved)

## #40 (original) — `tool: backend: github-release` always tries to extract asset as archive — HIGH

**Repro** (after supplying `tag: "jq-1.7.1"` to work around #39):

```
extract: unsupported archive format for /root/.local/share/mooncake/tools/mooncake-tool-1040481812.bin (supported: .tar.gz, .tgz, .tar, .zip)
```

`jq-linux-amd64` IS the binary itself — no archive. There's no
`extract: false` / `is_binary: true` in the schema.

**Why HIGH**: many GitHub releases ship bare binaries: jq, hadolint,
kind, kubectl, k9s, gh, mc, ripgrep prebuilt, etc. Without
bare-binary support, `tool: backend: github-release` is effectively
useless for the most common case.

**Fix**: infer from asset filename (no `.tar.gz`/`.zip` extension →
treat as binary, rename + chmod +x). Or add explicit `extract: false`.

---

---

## #44 — `file.download` silently accepts unknown fields (e.g. `sha256:`) — ✅ FIXED (round 24)

Per `mooncake validate` and `mooncake apply` both rejecting unknown
fields now:
```
$ mooncake apply -c bad-sum.yml
Error: /work/bad-sum.yml
  Line 5: unknown field `sha256` (likely a typo or a renamed field
    — see docs-next/guide/config/actions.md)
```

File does NOT land. The `additionalProperties: false` schema is now
enforced. This was the umbrella for many "silently doesn't work"
reports — closing it raises confidence in every subsequent finding.

Original report retained below for context:

---

### Original report (now resolved)

**Repro** (post-MT-14 fix):

```yaml
- file.download:
    url: https://github.com/jqlang/jq/releases/download/jq-1.7.1/jq-linux-amd64
    dest: /tmp/dl/jq-bad
    sha256: "0000...0000"        # ← unknown field, not in schema
    mode: "0755"
```

```
$ mooncake apply -c bad-sum.yml
~ download with WRONG sha256
RECAP  ok=0  changed=1  skipped=0  failed=0
$ ls /tmp/dl/
jq-bad
```

The schema defines `file.download` properties as `checksum`, `dest`,
`url`, etc. — `sha256:` is not a valid field. But the validator
accepts it without warning. The user *thinks* they declared a
checksum; they didn't; the verification path never runs.

**Why MEDIUM (security-adjacent)**: same shape as #14, but the
trigger is "user typed the wrong field name". A defense-in-depth
problem: even after MT-14 fixed the verify-before-rename ordering,
unknown-field acceptance reopens the same hole through a different
door. Users may have been writing `sha256:` because `LLM_GUIDE.md`
referenced sha256 verification.

**Fix**: this is a symptom of [#27](./ssot-drift.md#27). The
`mooncake schema generate` output already has `additionalProperties:
false` on each action's properties block. Wire the validator to
honor it. Same fix closes #44 and refines #4.

---

## Summary table

**Status rollup as of 2026-05-17: all silent-success findings ✅ FIXED.**

| # | Sev | Status | Action / Surface | Fix |
|---|---|---|---|---|
| 8 | CRITICAL | ✅ FIXED | `for_each` | landed `e8d1fc6` |
| 14 | CRITICAL | ✅ FIXED | `file.download checksum:` | landed `a09e12e` |
| 15 | HIGH | ✅ FIXED | `creates:`/`unless:` on `file.write` | step-level now universal field; validator rejects misplaced (commit `50768578` + `2d1c09e7` for MT-77) |
| 2 | HIGH | ✅ FIXED | shell guard recap mark | nested form `fa189ae5`; step-level closed by MT-15/MT-77 (regression test `f3bb10f2`) |
| 22 | HIGH | ✅ FIXED | `mooncake step` JSON shape | landed `c6e327e` (full result map) |
| 24 | HIGH | ✅ FIXED | `artifact.capture` file-change tracking | landed `ee125177`; tracker-after-flush pinned `8c2c69f2` |
| 28 | MEDIUM | ✅ FIXED | `failed_when:` on assert | landed `bcaf9392` |
| 40 | HIGH | ✅ FIXED | `tool github-release` bare-binary | landed `cb6b21bb` |
| 44 | MEDIUM | ✅ FIXED | `file.download` unknown-field acceptance | closed by #27 validator fix; verified round 24 |
| 45 | MEDIUM | ✅ FIXED | `transaction:` recap shows `reverted=N`, `↺ Reverse:` markers visible | landed before round 18 |
| 46 | MEDIUM | ✅ FIXED | `file.unarchive` not idempotent | landed `1d374c9` |
| 47 | MEDIUM | ✅ FIXED | `text.*` no-match is idempotent success | landed `439c3265` |
| 48 | MEDIUM | ✅ FIXED | `retry:` honors `failed_when:false` | landed `efc7efd0` |
| 49 | LOW | ✅ FIXED (doc) | `attempts:N` → N+1 total executions | doc clarification `55961982` |
| 54 | HIGH | ✅ FIXED | MCP `run_plan` returns real counters | verified round 32 |
| 62 | MEDIUM | ✅ FIXED | `retry: backoff:` honored | landed `80c3468d` |
| 67 | MEDIUM | ✅ FIXED | nested `try:` blocks rejected with hint | landed `cc7ef39` (round 42) |
| 70 | MEDIUM | ✅ FIXED | bare `{{ map }}` JSON-marshals | landed `295ba2a` (round 44) |
| 80 | MEDIUM | ✅ FIXED | `text.patch` failed-hunk diagnostics | landed `e1322d1f` |
| 83 | MEDIUM | ✅ FIXED | `mooncake step` strict-decode | landed `4e6997e` (KnownFields decode) |
| 84 | MEDIUM | ✅ FIXED | `text.insert` content-aware idempotency | landed `fc7b9df3` |
| 87 | MEDIUM | ✅ FIXED | `mooncake apply` exits on SIGINT/SIGTERM | landed `7b55547d` |

---

## #54 — ✅ FIXED (verified round 32)

MCP run_plan now returns real counters and all step entries:
```json
{
  "ok": 2,
  "changed": 1,
  "failed": 0,
  "skipped": 0,
  "requires": { "filesystem_write": ["/tmp/mcp-target.txt"] },
  "steps": [
    {"name": "greet"},
    {"name": "write", "changed": true},
    {"name": "verify"}
  ]
}
```

The agent loop now has the per-step signal it needs.

### Original report (now resolved)

## #54 (original) — MCP `run_plan` executes but returns all-zero counters and 1-step truncated result — HIGH (agent-loop)

**Repro**: 4-step playbook (`vars`, `log`, `file.write`, `assert`) submitted via MCP `run_plan`:

```yaml
- vars: { name: alice }
- name: greet
  log: { msg: "hi {{ name }}" }
- name: write
  file.write: { path: /tmp/mcp-target.txt, state: file, content: "from mcp\n" }
- name: verify
  assert: { file: { path: /tmp/mcp-target.txt, exists: true } }
```

MCP response:
```json
{
  "changed": 0, "ok": 0, "failed": 0, "skipped": 0,
  "duration_ms": 0,
  "steps": [{"name": "greet"}],
  "requires": {"filesystem_write": ["/tmp/mcp-target.txt"]}
}
```

But on disk: `/tmp/mcp-target.txt` exists with content `from mcp`. **The
plan ran**; the result counters lie. Only one of the four step names is
in `steps`, and every counter is zero.

**Why HIGH**: this is `mooncake mcp` exposing `run_plan` as the
canonical agent-integration entry point. An agent calling
`run_plan` to apply changes to a host gets back `{ok:0, changed:0,
failed:0}` regardless of what actually happened — so it can't tell
whether the run succeeded, partially succeeded, or did anything at
all. Same shape as #22 (step result truncation) but at the MCP layer.

The `requires:` field — `{filesystem_write: ["/tmp/mcp-target.txt"]}` —
is actually a nice pre-execution permission summary. Keep that. But
the post-execution counters need to reflect reality.

**Also observed in same test**: MCP `check_plan` (dry-run) appears to
*execute the assert step* even though no preceding step has produced
the file. Test playbook above failed `check_plan` with `assertion
failed (file): expected file exists, got file does not exist` — but
the write step (dry-run) hadn't created the file. So `check_plan`
is mixing real-execution of asserts into a plan that's supposed to
be side-effect-free preview. Either asserts shouldn't run in
check_plan, or the plan should virtualize the future filesystem
state for assert evaluation.

**Fix**:
1. `run_plan` result counters and `steps` list should reflect all
   plan steps and their actual outcomes.
2. `check_plan` should NOT evaluate asserts that depend on
   not-yet-applied state, or should virtualize the planned changes
   when doing so.

---

## #45 — `transaction:` recap miscounts reverted steps as changed — MEDIUM

**Repro**: `examples/transactions/rollback-demo.yml` — three file.writes
in a transaction; third writes to `/dev/null/cannot-exist-here` which
fails; first two are rolled back.

```
▶ create rollback demo a
~ create rollback demo a
▶ create rollback demo b
~ create rollback demo b
▶ create rollback demo c
✗ create rollback demo c
▶ notify rollback occurred
~ notify rollback occurred
RECAP  ok=0  changed=3  skipped=0  failed=1
```

Post-rollback state:
```
$ ls /tmp/mc-rollback-demo-*
/tmp/mc-rollback-demo-marker
```

Files a and b are GONE (rollback worked). But the recap claims
`changed=3` — counting the reverted files as changes. The text
formatter shows them as `~ changed` even though their changes were
reverted by the transaction's LIFO Reverse() pass.

The README example claims expected output is:
```
↺ Reverse: create rollback demo b   (file deleted)
↺ Reverse: create rollback demo a   (file deleted)
```

But no `↺ Reverse:` markers appear in actual output. The rollback
happens silently from the user's perspective; only the missing files
afterward reveal that anything was undone.

**Why MEDIUM**: same shape as #2 — semantics work, recap lies. A
fleet operator watching `changed_steps` from JSON output sees `3
changed` and thinks 3 files persisted; really, only 1 did (the
on_rollback marker).

**Fix**: when a transaction's child is reverted, set the step's
result `status` to `reverted` (or `skipped`) rather than `changed`.
Also surface the `↺ Reverse: <step>` lines the README promises.

---

## #46 — `file.unarchive` is not idempotent — MEDIUM

**Repro**:

```
$ tar czf /tmp/bundle.tar.gz src/
$ rm -rf /tmp/dest && mkdir /tmp/dest
$ mooncake step "file.unarchive: { src: /tmp/bundle.tar.gz, dest: /tmp/dest }"
{"changed": true, "action": "file.unarchive", "duration_ms": 0}

$ mooncake step "file.unarchive: { src: /tmp/bundle.tar.gz, dest: /tmp/dest }"
{"changed": true, "action": "file.unarchive", "duration_ms": 0}
```

Second run reports `changed: true` despite the destination already
containing the same files. Either:
- the action doesn't check existing files at all (always re-extracts),
- or the check is broken.

**Why MEDIUM**: many install patterns are "download archive, unarchive
to /opt". Idempotent re-runs are a core mooncake promise; this
breaks it.

**Fix**: before extracting, walk the archive's entries and check
whether each target file exists with matching content (or matching
mode + size as a cheap proxy). Skip if all entries already match.

---

## #47 — ✅ FIXED (commit `439c3265`, verified 2026-05-17)

`text.replace`, `text.delete_range`, `text.insert` now treat no-match as
idempotent success (`changed: false`), not a hard failure. Keeps the
idempotency promise across re-runs.

### Original report (now resolved)

## #47 (original) — `text.*` actions fail-loud on no-match instead of treating as idempotent — MEDIUM

**Repro**:

```
$ echo "before" > /tmp/tr.txt
$ mooncake step "text.replace: { path: /tmp/tr.txt, pattern: \"neverthere\", replace: \"x\" }"
{"error": "no matches found for pattern: neverthere"}
RECAP failed=1
```

Same shape for `text.delete_range` (anchor not found), `text.insert`
(anchor not found).

**Why MEDIUM**: this defeats idempotency at the action level. A
common second-run scenario for a config-management playbook:
- Run 1: pattern matches, change applied
- Run 2: pattern no longer matches (because run 1 changed it), action fails

The workaround `failed_when: false` exists but interacts badly with
retry (#48) and with `assert`-class actions (#28).

**Fix options**:
- (a) Add a `must_match: false` flag (opt-in) for "no match = idempotent success".
- (b) Change default: no-match = success, opt-in `must_match: true` for strict.

(b) matches the idempotency promise better but is a behavior change.

---

## #48 — ✅ FIXED (commit `efc7efd0`, verified 2026-05-17)

`retry:` runs all attempts before `failed_when:` is evaluated. `retry: 3,
failed_when: false` now means "try up to 3 times to succeed; if all fail,
mask the result", matching the intuitive read.

### Original report (now resolved)

## #48 (original) — `retry:` doesn't trigger when `failed_when: false` is set — MEDIUM

**Repro**:

```yaml
- shell: exit 1
  retry: { attempts: 3, delay: 200ms }
  failed_when: false
```

Debug output:
```
▶ retry-fail-3-times
  Executing: exit 1          ← only 1 attempt
~ retry-fail-3-times         ← reported "changed" (failed_when masked failure)
elapsed=92ms                 ← no 600ms retry delay
```

Without `failed_when: false`:
```
  Executing: exit 1
  Waiting 200ms before retry...
  Retry attempt 1/3
  Waiting 200ms before retry...
  Retry attempt 2/3
  Waiting 200ms before retry...
  Retry attempt 3/3
✗ retry-actually-fails  (command failed after 4 attempts)
elapsed=606ms
```

**Why MEDIUM**: `retry:` and `failed_when:` are both step-level
keys. Users reasonably read `retry: 3, failed_when: false` as
"try 3 times, but don't fail the run no matter what". The actual
order is `failed_when` first → if "not failed", no retry. So
retry never runs.

**Fix**: evaluate order should be **retry → failed_when**. Try N
times to get success; if all retries fail, *then* apply
`failed_when` to mask the final result.

---

## #70 — ✅ FIXED (round 44, commit `295ba2a`)

Bare `{{ map }}` now JSON-marshals:
```
$ mooncake apply -c <read.json + log {{ cfg.value }}>
raw value = {"a":1,"b":2}      ← clean JSON
                               ← previously: <map[string]interface {} Value>
```

Both top-level and nested map/slice variables render as JSON when
stringified directly. Drilling still works via `.value.X.Y`. Don't
regress.

### Original report (now resolved)

## #70 (original) — Direct `{{ map_var }}` stringify leaks Go reflect.Value; need `cfg.value.X` to access JSON content — MEDIUM (DX + display)

**Original report was wrong** — after auditing across action types,
the bug is narrower than feared.

**What actually happens**:
```yaml
- read.json: { path: /work/data.json }   # {"service":{"name":"web","port":8080}}
  as: cfg
- log: { msg: "via .value: {{ cfg.value.service.name }}" }
- log: { msg: "raw cfg = {{ cfg }}" }
```

Output:
```
via .value: web              ← drilling WORKS via .value.X.Y
raw cfg = <map[string]interface {} Value>   ← top-level stringify broken
```

So:
- ✅ `read.json` returns `{value: {...}}` — same wrapping as `observe.*` actions
- ✅ `cfg.value.service.name` drills correctly → `"web"`
- ❌ `{{ cfg }}` (direct stringify) produces `<map[string]interface {} Value>` (Go reflect.Value String())
- ⚠️ JSON numbers stringify as floats with `.0` suffix: `port=8080.000000`, `replicas=3.000000` — cosmetic

**Audit across action types** (round 21):
- `read.json as:` — drilling via `.value` works; top-level stringify broken
- `repo.search as: r` — `r.total_matches=2` ✓, `r.results.0.file` empty (array index access via `.0` broken)
- `observe.cpu as: c` — `c.value.cores=32` ✓, fields accessible
- `pkg.list as: p` — `p.count=92` ✓, `p.manager=apt` ✓
- `shell as: s` — `s.stdout`, `s.rc` ✓

So **most `as:` paths work**; what's broken is:
1. **Direct stringify of a non-scalar variable** → reflect.Value repr
2. **Array-index access** `r.results.0.file` returns empty (could be Pongo2 syntax — try `r.results.0|attr:"file"` or `r.results[0].file`)

**Documentation needed**: every typed-result action result has a
`.value` wrapper (or a flat top-level for actions returning multiple
named fields like `pkg.list`). Users following `{{ cfg.service.name }}`
silently get empty.

**Fix**:
1. When stringifying a map/slice variable directly, marshal to
   JSON/YAML instead of falling through to Go reflect.Value String().
2. Document the `.value` wrapper in templates docs (every `as:`-capable
   action's result page should show the template path).
3. Support array indexing `.0`, `.1` consistently (currently `.results.0` returns empty for repo.search).

Severity downgraded from HIGH → MEDIUM since the data IS accessible
once users learn `.value`; the original report's "all empty" was due
to my missing `.value`.

---

## #83 — ✅ FIXED (commit `4e6997e`, verified 2026-05-16)

`mooncake step` now runs the inline YAML through
`yaml.NewDecoder(...).KnownFields(true)` — the same strict-decode
posture `mooncake apply` uses since MT-44. Unknown fields anywhere
in the step (top-level or nested action struct) abort parse with a
diagnostic that names the field and the type:

```
$ mooncake step "wait.command: { cmd: \"exit 42\", expected_exit: 42 }"
2026/05/16 ... failed to parse step YAML: yaml: unmarshal errors:
  line 1: field expected_exit not found in type config.WaitCommand
# exit 1
```

Canonical field still works:
```
$ mooncake step "wait.command: { cmd: \"exit 42\", expect_exit: 42 }"
{"action": "wait.command", "success": true, "last_exit": 42, ...}
```

Step now has the same safety story as YAML-file authors using
`apply`. Three regressions tests cover the headline repro, an
unknown top-level field, and the positive canonical case. Don't
regress.

### Original report (now resolved)

**Repro**:
```
$ mooncake step "wait.command: { cmd: \"exit 42\", expected_exit: 42 }"
{"action": "wait.command", "error": "wait.command timeout after 1s; last exit 42"}
# step ran — unknown field expected_exit (correct is expect_exit) was silently ignored
```

vs.

```
$ echo "- wait.command: { cmd: 'exit 42', expected_exit: 42 }" > /work/c.yml
$ mooncake apply -c /work/c.yml
Error: /work/c.yml
  Line 1: unknown field `expected_exit` ...
```

`apply` (via validate) properly rejects unknown fields per #44.
`step` doesn't — it goes straight to the action handler, which only
validates its own required fields. So the user gets:
- `apply`: clear "unknown field X" error → fix and retry
- `step`: silent ignore + timeout → confusing failure → user doesn't
  realize the field was misspelled

**Why MEDIUM**: same shape as the umbrella #44 issue, but in the
agent-facing entry point. Agents calling `step` for one-shot
operations have a strictly weaker safety story than YAML-file
authors using `apply`.

**Fix**: `mooncake step` should run the inline YAML through the same
schema validator that `mooncake validate` / `apply` use, before
dispatching to the handler.

---

## #84 — ✅ FIXED (commit `fc7b9df3`, verified 2026-05-17)

`text.insert` now checks whether the exact content already appears in the
expected position before inserting. Run 2 of the same step → `changed: false,
operation: noop`. Same convention as `text.line`. Regression test at
`internal/actions/file_insert/handler_test.go:214` (MT-84).

### Original report (now resolved)

## #84 (original) — `text.insert` is not idempotent — duplicates content on re-run — MEDIUM

**Repro**:
```yaml
- text.insert:
    path: /work/conf.txt
    anchor: "[server]"
    position: after
    content: "host=localhost"
```

```
$ mooncake step <above>
{"changed": true, "insertions": 1}

$ mooncake step <above>      # exact same step, run 2
{"changed": true, "insertions": 1}     # ← duplicated insert!
```

File after run 2:
```
[server]
host=localhost
host=localhost      ← duplicate
port=8080
```

`text.line` correctly handles "line already exists" — see
[`positive-keepers.md`](./positive-keepers.md). `text.insert` doesn't:
it just inserts the content at every matching anchor, blindly.

**Why MEDIUM**: idempotency is a core mooncake promise. Users have
to wrap `text.insert` in `unless: grep -q 'host=localhost' /work/conf.txt`
to make it safe. That's brittle and easy to forget.

**Fix**: before inserting, check if the exact content already appears
in the expected position (immediately after/before the anchor).
If yes, return `changed: false, operation: noop`. Same convention
as text.line.

(Wider observation: every text.* action should either honor `unless:`
universally OR implement content-aware idempotency. Pick one model
and apply consistently.)

---

## #80 — ✅ FIXED (commit `e1322d1f`, verified 2026-05-17)

`text.patch` now surfaces hunk counters (`applied_hunks`, `failed_hunks`,
`total_hunks`) on every result. When no hunks apply, the action returns
a clear error naming the drift; agents have the per-hunk signal they need.

### Original report (now resolved)

## #80 (original) — `text.patch` silently no-ops on broken hunks (no error, no signal) — MEDIUM

**Repro**:
```
$ echo -e "alpha\nbeta\ngamma" > /work/in.txt
$ mooncake step "text.patch: {
    path: /work/in.txt,
    patch: \"--- a/in.txt\n+++ b/in.txt\n@@ -50,3 +50,3 @@\n alpha\n-beta\n+BETA\n gamma\n\"
  }"
{
  "action": "text.patch",
  "changed": false,
  "failed": false,
  "rc": 0
  # ← no failed_hunks, no error, no diagnostic
}
```

The patch targets line 50 in a 3-line file. The patch can't apply.
But the action returns clean success: `changed: false, failed: false`.
No `failed_hunks` count, no error message, no clue.

Compare to a VALID patch result:
```json
{"applied_hunks": 1, "failed_hunks": 0, "total_hunks": 1}
```

The valid case surfaces hunk counters. The broken case omits them
entirely — agents have no signal that the patch was rejected.

**Why MEDIUM**: this is a *bunch* of silent-success in one place.
LLM-driven code refactors via `repo.patch` / `text.patch` rely on the
"did my change land?" signal. With this bug, the agent thinks the
patch succeeded and moves on — the file is unchanged.

**Fix**: when the patch couldn't apply any hunk, return
`failed_hunks > 0, total_hunks > 0, changed: false, error: "no
hunks matched (target file may have drifted)"`. Or: return
`failed: true` if all hunks failed.

(The idempotent case — patch already applied — should *also* be
distinguished from this. Today both return identical `changed:
false, failed: false`. They mean different things and should look
different.)

---

## #67 — ✅ FIXED (round 42, commit `cc7ef39`)

Now rejected at plan time with a clear error and an actionable hint:
```
planner setup failed: failed to build plan: try block <unnamed>:
  try child 0 (unknown): nested try: blocks are not supported in v1.
  Hint: to swallow a single inner step failure, set continue_on_error: true
  on that step instead of wrapping it in another try:.
```

Pointing at `continue_on_error: true` as the substitute is exactly
the right DX move — names the v1 alternative instead of just
"unsupported". Don't regress.

### Original report (now resolved)

## #67 (original) — Nested `try:` blocks fail with "no handler registered for action type: unknown" — MEDIUM

**Repro**:
```yaml
- try:
    - log: { msg: outer-before }
    - try:                          # ← nested try
        - shell: exit 1
      catch:
        - log: { msg: inner-catch }
    - log: { msg: outer-after }
  catch:
    - log: { msg: outer-catch }
```

Output:
```
▶ 
✓ 
no handler registered for action type: unknown
▶ 
✗ 
RECAP  ok=1  changed=0  skipped=0  failed=1
```

Inner `try:` is treated as `action: unknown`. The executor doesn't
recognize compound actions (`try`, `transaction`) when they appear
inside another compound block.

**Why MEDIUM**: realistic use case — "atomically update config, but
if the migration sub-step fails, run its own cleanup, while the
outer block still handles other failures". With nested try broken,
users must flatten control flow.

**Workaround for now**: only top-level `try:` works; refactor nested
into sequential blocks at the top level.

**Fix**: register `try` and `transaction` action types in the inner
expansion path too, not just at the top level.

---

## #62 — ✅ FIXED (verified round 32, commit `80c3468`)

`retry: backoff: exponential` now produces 100→200→400ms delays:
```
Waiting 100ms before retry (backoff=exponential)...
Waiting 200ms before retry (backoff=exponential)...
Waiting 400ms before retry (backoff=exponential)...
```
`backoff: linear` produces 100→200→300ms. `backoff:` label is now
shown in the debug message — nice touch.

### Original report (now resolved)

## #62 (original) — `retry: backoff: linear|exponential` is silently ignored; delay stays fixed — MEDIUM

**Repro**:
```yaml
- shell: exit 1
  retry: { attempts: 3, delay: 100ms, backoff: exponential }
```

Debug output (3 retries):
```
  Waiting 100ms before retry...
  Retry attempt 1/3
  Waiting 100ms before retry...
  Retry attempt 2/3
  Waiting 100ms before retry...
  Retry attempt 3/3
```

`backoff: linear` produces the same output. Total elapsed is identical
(~1.5s for both) — well below what linear (100+200+300=600ms wait) or
exponential (100+200+400=700ms wait) would produce.

Also: `backoff: nonsense` is silently accepted (per #44 unknown-field
acceptance), no warning or error.

**Why MEDIUM**: external API integrations rely on backoff to avoid
hammering a stuck dependency. With fixed delay only, mooncake's
retry creates the exact thundering-herd problem backoff is supposed
to prevent.

**Fix**:
1. Honor `backoff: linear` → delay × attempt_n
2. Honor `backoff: exponential` → delay × 2^attempt_n
3. Validate `backoff:` against an enum at parse time (closes the #44 hole here)
4. Document defaults: `backoff: fixed` (status quo) explicitly named

---

## #49 — `retry: { attempts: N }` runs N+1 attempts, not N — LOW (semantic clarity)

**Repro** (from #48 above):

```
attempts: 3 → "Retry attempt 1/3", "2/3", "3/3" → "command failed after 4 attempts"
```

`attempts:` means "number of *retries* after the initial try", giving
a total of N+1 executions. Common interpretation but documentation
should say so explicitly. Ansible's `until: ... retries:` uses the
same semantics; ours doesn't say.

**Fix**: clarify docs, or rename to `max_retries:` / `extra_attempts:`.
