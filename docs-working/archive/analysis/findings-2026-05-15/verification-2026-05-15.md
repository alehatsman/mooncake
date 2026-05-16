# Fix Verification — 2026-05-15

After the initial 43 findings were filed, the project author landed fixes
for 9 of them (commits prefixed `MT-N fix` or `fix(...): MT-N`). I
rebuilt the static binary from `HEAD` and re-ran each repro. Results
below.

## Verification status

| # | Original sev | Status | Commit | Notes |
|---|---|---|---|---|
| **#1** | HIGH | ✅ **FIXED** | `81dc50e` | `as_user: root` now short-circuits when euid=0; jq preset succeeds on vanilla ubuntu with no sudo |
| **#2** | HIGH | 🟡 **partial** | `fa189ae` | Nested form (`shell: { cmd:..., creates:... }`) reports `skipped` correctly; step-level form (`shell: ..., creates: ...` as sibling) still reports `changed` |
| **#4** | MEDIUM | 🟡 **partial** | `362d288` | Line number now anchors to the failing step (was always line 1); vocabulary listing still narrow (related to #27) |
| **#5** | LOW | ✅ **FIXED** | `4bf28b4` / `17e5649` | Shell stdout now surfaces at `-l debug` with `\|`-prefixed indent under the step |
| **#6** | LOW | ✅ **FIXED** | `1be1d1f` / `61985d7` | `LLM_GUIDE.md` action vocabulary refreshed |
| **#8** | CRITICAL | ✅ **FIXED** | `e8d1fc6` / `3073dd9` | `for_each` now binds `{{ item }}` to each slice element; step names render correctly |
| **#12** | LOW | ✅ **FIXED** | `7007130` / `16a29dd` | `metrics -q --format json` now returns proper JSON object instead of `key=value` lines |
| **#13** | LOW | ✅ **FIXED** | `158d1bd` / `796ed2f` | Broken `package:` suggested_step no longer fires for unrelated errors |
| **#14** | CRITICAL | ✅ **FIXED** (caveat) | `a09e12e` / `6a88b02` | `file.download: checksum: ...` now verifies before rename; bad checksum → clean error, file does NOT land |

**Both CRITICAL bugs fixed.** Two HIGH/MEDIUM partial. Excellent
turnaround.

---

## ⚠ New finding from verification — silent unknown-field acceptance

The MT-14 fix works **only** with the schema-correct `checksum:` field.
My original #14 repro used `sha256:` (which I assumed was the field
name). Re-test:

```yaml
- file.download:
    url: https://github.com/jqlang/jq/releases/download/jq-1.7.1/jq-linux-amd64
    dest: /tmp/dl/jq-bad
    sha256: "0000...0000"   # ← unknown field
    mode: "0755"
```

```
$ mooncake apply -c bad-sum.yml
▶ download with WRONG sha256
~ download with WRONG sha256
RECAP  ok=0  changed=1  skipped=0  failed=0  194ms

$ ls /tmp/dl/
jq-bad   ← file landed, no verification, no warning
```

**This is a different bug from MT-14**: the validator silently accepts
unknown fields. `sha256:` is not in the file.download schema, but it's
accepted without warning, and the protection a user *thought* they
were getting silently doesn't apply.

**Severity**: MEDIUM (security-adjacent — same shape as the original
#14, but now the user-facing trigger is "typed wrong field name"
instead of "verify ran after rename")

**Where it should live**: this connects directly to **#27 (validator
drift)**. The schema generator's output has `additionalProperties:
false` on each action's properties block. But the validator that
runs at `apply` time accepts arbitrary extra fields. Wiring the
validator to honor the generated schema (the fix for #27) closes
this one too.

**Filed as**: new finding #44 — see updated
[`silent-success-bugs.md`](./silent-success-bugs.md).

---

## #2 — what still doesn't work

The fix added `Creates` / `Unless` fields to `ShellAction`. The
**nested form** works:

```yaml
- name: shell with creates guard (NESTED — works)
  shell:
    cmd: touch /tmp/once.flag
    creates: /tmp/once.flag
```

Run 2:
```
- shell with creates guard (NESTED — works) [creates: /tmp/once.flag]
RECAP  ok=1  changed=0  skipped=1  failed=0
```

But the **step-level form** still reports `changed`:

```yaml
- name: shell with creates guard (STEP-LEVEL — still broken)
  shell: touch /tmp/once.flag
  creates: /tmp/once.flag
```

Run 2:
```
~ shell with creates guard (STEP-LEVEL — still broken)
RECAP  ok=2  changed=2  skipped=0  failed=0
```

Most examples and documentation use the step-level form (since it
parallels Ansible). The fix only addressed half the surface. Either:
- (a) Honor `creates:` / `unless:` at step-level too (use the same
  short-circuit path).
- (b) Deprecate step-level for shell entirely, document the nested
  form as canonical, fix all examples.

(a) is the smaller change.

---

## #4 — what still doesn't work

The fix correctly anchors the line number to the failing step. But:

- The error still lists the short allowed-action vocabulary (related
  to #27).
- `file.template` with `content:` produces "Step must have exactly
  one action", but `file.template` IS in the vocabulary — the real
  error is that `content` isn't a valid property of `file.template`.
  The validator falls back to the generic "no recognized action"
  message instead of "unknown property `content` for `file.template`".

Repro:
```
$ mooncake validate -c bad.yml
Error: bad.yml
  Line 11: Step must have exactly one action (shell, cmd, file.write, file.template, ...)
    dest: /tmp/x
    (in step: third step - bad file.template needs src not content)
```

The step uses `file.template:` (a valid action), but the validator
treats it as if no recognized action is present. Same SSOT-drift
class as #27.

---

## Severity rollup post-fixes

| Severity | Original | After fixes | Δ |
|---|---:|---:|---|
| CRITICAL | 2 | **0** | 🎉 −2 |
| HIGH | 7 | 5 (one is #2 partial) | −2 |
| MEDIUM | 9 | 8 (added #44, removed #4, #12) | −1 |
| LOW | 19 | 14 (#5, #6, #13 closed; #2/#4 dropped to partials) | −5 |
| **Total open** | **43** | **34** | **−9** |
| Positive keepers | 12 | 14 (+ MT-1/2/8/14 fixes themselves) | +2 |

## What to fix next (post-verification)

Three highest-ROI fixes still open:
1. **#15** — unify `creates:` / `unless:` honor across all action handlers (also closes #2's step-level gap)
2. **#27 via #35** — wire validator to `mooncake schema generate`'s output (closes #4 vocabulary issue, closes #44 unknown-field acceptance, refines #29's vocabulary list quality)
3. **#40** — let `tool github-release` install bare binaries
