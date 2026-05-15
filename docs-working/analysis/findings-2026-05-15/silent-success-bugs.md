# Silent-Success Bugs

The dominant pattern in this audit. The recap reports `failed=0` /
green / "changed=N", but the action either did nothing useful or did
something unverified. **The largest cluster of findings.**

> Start fixes from this file. These bugs evade CI because CI sees
> green. They surface in production.

**Post-MT-fix status** (see [`verification-2026-05-15.md`](./verification-2026-05-15.md)):
- ✅ **CLOSED**: #8 (for_each), #14 (file.download sha256 verify before rename)
- 🟡 **PARTIAL**: #2 (nested form fixed; step-level still broken)
- **NEW**: #44 (file.download silently accepts unknown fields),
  #45 (transaction recap miscounts reverts),
  #46 (file.unarchive not idempotent)

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

## #15 — `creates:` and `unless:` are silently ignored on `file.write` — HIGH

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

## #22 — `mooncake step` truncates action-specific structured output — HIGH (AI-agent UX)

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

## #24 — `artifact.capture` reports 0 file changes across all action types — HIGH

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

## #28 — `failed_when: false` does not suppress assertion failure — MEDIUM (semantics)

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

## #40 — `tool: backend: github-release` always tries to extract asset as archive — HIGH

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

## #44 — `file.download` silently accepts unknown fields (e.g. `sha256:`) — MEDIUM (security-adjacent)

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

| # | Sev | Status | Action / Surface | Fix size |
|---|---|---|---|---|
| 8 | CRITICAL | ✅ FIXED | `for_each` | landed `e8d1fc6` |
| 14 | CRITICAL | ✅ FIXED | `file.download checksum:` | landed `a09e12e` |
| 15 | HIGH | open | `creates:`/`unless:` on `file.write` | small — extend exec wrapper |
| 2 | HIGH | 🟡 partial | shell guard recap mark | nested form fixed; step-level still |
| 22 | HIGH | open | `mooncake step` JSON shape | small — emit full result map |
| 24 | HIGH | open | `artifact.capture` | medium — file-change tracking |
| 28 | MEDIUM | open | `failed_when:` on assert | small — route through wrapper |
| 40 | HIGH | open | `tool github-release` bare-binary | small — filename heuristic |
| 44 | MEDIUM | open | `file.download` unknown-field acceptance | closes with #27 |
| 45 | MEDIUM | open | `transaction:` recap miscounts reverted steps | renderer + counter |
| 46 | MEDIUM | open | `file.unarchive` not idempotent | check dest contents before extract |

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
