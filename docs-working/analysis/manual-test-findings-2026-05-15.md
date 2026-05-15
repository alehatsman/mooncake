# Manual Test Findings — 2026-05-15

Working doc. Filed by an LLM acting as a manual tester: built the binary, ran
four scripted scenarios inside `ubuntu:24.04` and `alpine:3.21` containers.
Each finding below is an independent issue worth filing — listed in
descending severity.

> **TL;DR**: dry-run and validate-time error messages are good and should be
> kept. The two highest-impact bugs are (1) `as_user: root` always shelling
> out to `sudo` even when uid=0, which makes presets unusable in minimal
> containers, and (2) `shell` step guards (`creates:` / `unless:`) being
> honored at the FS level but mis-counted as `changed` instead of `skipped`
> in the recap — which silently breaks the idempotency story.

---

## Repro environment

```
Binary:     CGO_ENABLED=0 go build -ldflags='-s -w' -o out/mooncake-static ./cmd
Containers: ubuntu:24.04, alpine:3.21
Mount:      -v $PWD/out/mooncake-static:/usr/local/bin/mooncake:ro
            -v $PWD/presets:/work/presets:ro
            -v <playbook>:/work/mooncake.yml:ro
            -w /work
```

The playbooks and the exact docker invocations are in `/tmp/mooncake-tests/`
on the test host.

---

## What worked well (do not regress)

These are explicitly called out so a future refactor doesn't trade them away:

1. **`mooncake facts` in a container** — correct OS (`ubuntu 24.04`), kernel,
   package manager (`apt`), CPU model, storage, NICs. Clean table layout.
2. **`file.write` idempotency** — second `apply` shows `✓ ok=2`, no rewrite.
3. **`mooncake apply --dry-run`** — verified zero side effects (no marker
   file, no shell-side-effect file created), produced a per-step
   `risk N (routine) • reversible • N resource • N bytes` annotation and a
   summary. This is the safety story working as advertised. **Headline UX
   win.**
4. **Validate-time failures** — broken Jinja filter is caught with file +
   line + column (`Line 5 ... Col 25 near 'nope'`); missing `import:` reports
   the full resolved absolute path; failing `assert` reports expected vs
   actual exit code.
5. **`jq` preset on Ubuntu** — works end-to-end, second run reports `ok=3
   changed=1 skipped=6` (mostly correct), once `sudo` is present.

---

## Findings

### 1. `as_user: root` invokes `sudo` even when uid=0 — SEVERITY: HIGH

**Repro**: apply any preset using `as_user: root` (`presets/jq` does) in a
vanilla `ubuntu:24.04` or `alpine:3.21` container.

```
▶ Install jq (Linux)
  Installing packages: jq
failed to install packages [jq]: exec: "sudo": executable file not found in $PATH
✗ Install jq (Linux)
RECAP  ok=0  changed=1  skipped=1  failed=2  71ms
```

**Why it matters**: official base images don't ship sudo. Every preset that
hardcodes `as_user: root` (the canonical pattern for system installs) is
unusable in CI containers, minimal images, and most Docker quickstarts —
which is exactly the audience the "Docker for AI agents" framing is meant to
attract.

**Fix**: when the current process is already uid=0, `as_user: root` should
short-circuit and run the step directly, not invoke `sudo`. The check is
`os.Geteuid() == 0`.

**Surface area**: every preset using `as_user: root` (grep finds many),
plus the executor path that builds the `sudo` command.

---

### 2. `shell` with `creates:` / `unless:` reports `changed` instead of `skipped` — SEVERITY: HIGH (silent correctness)

**Repro**: `/tmp/mooncake-tests/idem/mooncake.yml`. Run twice.

Run 2 output:
```
▶ shell with creates guard
~ shell with creates guard
▶ shell with unless guard
~ shell with unless guard
RECAP  ok=2  changed=2  skipped=0  failed=0
```

**Observed**: the recap shows `changed=2` and the line marker is `~`.

**Expected**: `skipped=2` and the line marker should be `-` (or whatever the
skipped glyph is). `file.write` correctly distinguishes ok / changed /
skipped in the same playbook.

**Why it matters**: `creates:` and `unless:` are the documented idempotency
escape hatch for `shell`. If they're reported as `changed`, users running
`mooncake apply` in a watch loop, in CI, or in a fleet drift check will see
constant noise that contradicts the "second run = no changes" guarantee.
The actual side effect is suppressed (mtime of `once.flag` did not move,
content of `unless.txt` did not change) — so this is **reporting**, not
behavior. That makes it a silent correctness bug, easy to ship without
noticing.

**Fix**: in the shell action handler, when the `creates`/`unless` precheck
short-circuits, set the result status to `skipped`, not `changed`.

---

### 3. `jq` preset only handles apt + brew, README claims four package managers — SEVERITY: MEDIUM (broken promise)

**Repro**:
```
docker run --rm -v $PWD/out/mooncake-static:/usr/local/bin/mooncake \
  -v $PWD/presets:/work/presets -v /tmp/mooncake-tests/preset-jq/mooncake.yml:/work/mooncake.yml \
  -w /work alpine:3.21 sh -c 'apk add --no-cache sudo bash && mooncake apply'
```

Output:
```
- Install jq (macOS) [when: ... brew_available ...]
- Install jq (Linux) [when: ... apt_available ...]
▶ Verify installation with assert
✗ Verify installation with assert
RECAP  ok=0  changed=1  skipped=2  failed=2  70ms
```

**Why it matters**: `presets/jq/README.md` lists "Linux (apt, dnf, yum,
pacman)" as supported. The actual `tasks/install.yml` only has `apt` and
`brew` branches — apk/dnf/yum/pacman silently skip, then the verify-assert
fails. That's the worst failure shape: "install" appears to succeed (no
package errors), then the assert kicks in with a confusing message.

**Fix**: add `apk_available`, `dnf_available`, `yum_available`,
`pacman_available` facts (if they don't already exist), and corresponding
branches in `presets/jq/tasks/install.yml`. Or honestly downscope the README
to "apt, brew" and file a backlog issue for the other managers.

**Wider issue**: this is almost certainly not specific to jq — every preset
authored against this pattern has the same risk. Worth a one-time audit
across `presets/*/tasks/install.yml`.

---

### 4. Validator anchors all errors to "Line 1" of the first step — SEVERITY: MEDIUM (DX)

**Repro**: any playbook where a non-first step uses an action with an
invalid sub-key. Example: `file.template` with `content:` (which the schema
rejects in favor of `src:`).

```
Error: /work/mooncake.yml
  Line 1: Step must have exactly one action (shell, cmd, file.write, ...)
    - vars:
    (in step: ensure /tmp/idem dir)
```

The `(in step: ...)` hint still pointed at the **previous** edit's step name
even after the file was rewritten — and the line number `1` was just wrong;
the real problem was at line 17 (`file.template: content:`).

**Why it matters**: bisection took ~4 round trips of editing pieces out of
the file. A user without LLM patience would file a different bug entirely.

**Fix**: report the line of the offending step, not line 1; report the
offending sub-key when it exists, not the entire allowed-action vocabulary.

---

### 5. Shell stdout is hidden by default, even at `-l debug` — SEVERITY: LOW (surprise)

**Repro**: `examples/hello-world/config.yml` does `shell: echo "Hello from
Mooncake!"`. At default log level you only see step markers; at `-l debug`
you see the rendered command but not its stdout.

**Why it matters**: the very first action in the very first canonical
example is `echo "Hello from Mooncake!"` — a user reasonably expects to see
"Hello from Mooncake!" printed. Discovering that you need
`as: greeting` + a `log: "{{ greeting.stdout }}"` step is a five-minute
onboarding tax.

**Options**:
- (a) Make `--show-output` / `-o` a top-level flag on `apply`.
- (b) Auto-stream stdout for `shell` at `-l debug` (matches user
  expectation of "debug shows me everything").
- (c) Document it explicitly in the hello-world README so the surprise lands
  before the user thinks the binary is broken.

(a) + (c) together is the smallest fix.

---

### 6. `LLM_GUIDE.md` actions list is out of date — SEVERITY: LOW (doc drift)

**Repro**: `LLM_GUIDE.md` line 102 says

> 13 actions: print, vars, shell, command, include_vars, file, template,
> copy, download, unarchive, assert, preset, service

The validator's allowed-action vocabulary is:

> shell, cmd, file.write, file.template, file.copy, file.download,
> file.unarchive, text.replace, text.insert, text.delete_range, text.patch,
> os.service, pkg, repo.search, repo.tree, repo.patch, artifact.capture,
> artifact.validate, assert, use, log, import, vars, vars.load, or wait

These barely overlap. `print`/`include_vars`/`preset`/`command` no longer
exist; `cmd`/`pkg`/`use`/`log`/`import`/`text.*`/`repo.*`/`artifact.*` are
not mentioned.

**Why it matters**: the guide's tagline is "for LLMs"; an LLM following it
verbatim writes a broken playbook. I did, on the first try.

**Fix**: regenerate the action list from
`internal/config/schema.json` or `internal/register/register.go`. Better: a
single-source-of-truth section that the docs build pulls from.

---

### 7. `examples/variables-and-facts/config.yml` is broken — SEVERITY: LOW (example rot)

**Repro**:
```
$ mooncake apply -c examples/variables-and-facts/config.yml
planner setup failed: failed to build plan: failed to read config:
failed to read config: yaml: line 47: mapping values are not allowed in this context
```

**Why it matters**: it's one of the directories shown in `examples/README.md`
as a starting point. New users will copy from it.

**Fix**: read line 47, fix the YAML (likely an unquoted `:` inside a value).

---

---

## Continued exploration (round 2)

Additional surfaces probed after the first round; new findings below pick up
numbering from #8.

### 8. `for_each` does not iterate — `{{ item }}` resolves to the Go reflect.Value repr of the slice — SEVERITY: CRITICAL

**Repro**: the upstream example, untouched.

```
$ docker run --rm -v $PWD/out/mooncake-static:/usr/local/bin/mooncake:ro \
    -v $PWD/examples/loops:/work:ro -w /work ubuntu:24.04 \
    mooncake apply -c /work/with-items.yml
```

Output:
```
▶ <[]interface
~ <[]interface
▶ {}
~ {}
▶ Value>
~ Value>
... (repeats 3× for the 3 for_each blocks)
RECAP  ok=0  changed=9  skipped=0  failed=0
```

And the plan view confirms it:
```
↑ Install package      would run: echo "Installing <[]interface"
↑ Install package      would run: echo "Installing {}"
↑ Install package      would run: echo "Installing Value>"
```

**What's happening**: `{{ item }}` inside `for_each` does NOT bind to each
slice element. It is bound to `fmt.Sprintf("%v", reflectValueOfTheWholeSlice)`,
which renders as `<[]interface {} Value>`. That string is then **tokenized
on whitespace** into 3 fragments — `<[]interface`, `{}`, `Value>` — and each
iteration receives one fragment as `item`.

**Two distinct bugs stack here**:
1. `{{ item }}` is bound to the slice's `reflect.Value.String()` instead of
   to each element.
2. The `apply` view replaces the step's `name:` with the bad item value
   entirely (plan view preserves `name:` correctly — only `apply`'s
   renderer clobbers it).

**Why CRITICAL**: `for_each` is THE primary iteration primitive in a
config-management language. It's used in the canonical loops example, in
`presets/*/tasks/install.yml`, in `examples/file-insert-example.yml`, etc.
Every preset that loops over packages, users, or paths is silently
miscompiling. The fact that it lands as 9 "changed" rows with a green
recap (`failed=0`) means CI passes while doing nothing useful — worst
possible failure shape.

Surprising it shipped — likely a regression from a recent template-engine
refactor; the example files are still in tree but the rendering changed
underneath them.

**Where to look**: the for-each expansion in `internal/plan/planner.go`
(see `expandInclude` neighbors) and the variable binding into the per-step
`ExpansionContext.Variables`. Likely `item` is being set to the slice
value, not the element value, inside the iteration loop.

---

### 9. `pkg:` direct usage **is** sudo-clean — confirms #1 is scoped to `as_user: root`

**Repro**: `/tmp/mooncake-tests/pkg/mooncake.yml` runs against
ubuntu/alpine/debian-slim with **no sudo**:

```
=== ubuntu (apt) ===        ~ install jq via pkg     RECAP  changed=1  failed=0
=== alpine (apk) ===        ~ install jq via pkg     RECAP  changed=1  failed=0
=== debian:slim (apt) ===   ~ install jq via pkg     RECAP  changed=1  failed=0
```

Idempotency on alpine round 2: `ok=1 changed=0 skipped=0 failed=0`. Correct.

**Implication for #1**: the bug is not in the executor's general
"run-as-root" path — `pkg` is already smart enough to skip sudo when uid=0.
The fix is localized to the `as_user: root` handler. That should make #1
cheaper to fix.

---

### 10. Saved plans + `--from-plan` work well, including staleness — keep this

**Repro**: `mooncake plan -o plan.json` produces a JSON plan with:

```json
{
  "version": "1.0",
  "input_files_hash": "dd417db007...",
  "steps": [{ "name": "write file", "file.write": {...}, "id": "step-0001", ...}]
}
```

After modifying `mooncake.yml`, replaying:

```
$ mooncake apply --from-plan plan.json
2026/05/15 16:48:38 refusing to apply stale plan: plan input files have
changed since the plan was built (use --allow-stale to override)
```

`--allow-stale` honors plan-time values: replayed plan wrote `v1`, not the
current source's `v2`. Strong safety property. No regression here.

**Suggestion**: feature this prominently in onboarding (`docs-working/
analysis/dx-audit-2026-05.md` mentions promoting `plan`; this is the
follow-on demo).

---

### 11. `snapshot --diff` only diffs hardware/curated tool list — SEVERITY: LOW (surface gap)

**Repro**:

```
$ mooncake snapshot --format json --save snap1.json
$ apt-get install -y jq
$ mooncake snapshot --diff snap1.json
hw:
  ~ ram_free_mb          25451 → 25454
```

A new tool (`jq`) installed between snapshots does not surface — the
`tools:` map only includes a curated allowlist (bash version, in this
case). For "Docker for AI agents", the agent wants to ask "what's
installed?" — the snapshot is the right surface and it's underbuilt.

**Suggestion**: this is probably an `observe.*` story (spec-59), not a
snapshot fix per se. Worth noting as input to that spec.

---

### 12. `metrics -q <key> --format json` silently ignores `--format` — SEVERITY: LOW

**Repro**:

```
$ mooncake metrics --format json -q cpu_usage_pct -q memory_used_pct
cpu_usage_pct=3.08
memory_used_pct=17.41
```

`key=value` lines instead of JSON. `--fields` *does* honor `--format json`.
Either `-q` should respect `--format` too, or the help text should call out
that `-q` is text-only.

---

### 13. Error-message "suggested step" hint emits invalid YAML — SEVERITY: LOW

**Repro**: any failing shell command that triggers the "is not installed"
heuristic. Example output:

```json
{
  "event": "step_error",
  "step": "<[]interface",
  "action": "shell",
  "stderr": "bash: line 1: lt: command not found\n...",
  "hint": "bash: is not installed",
  "suggested_step": "package:\n  name: bash:\n  state: present"
}
```

Two bugs in one hint:
1. The action name `package:` is not in the schema's vocabulary — the
   correct action is `pkg:`.
2. The package name comes out as `bash:` (trailing colon retained from
   stderr parsing), so the suggestion is doubly wrong.

Also, the hint fired on a bash error that wasn't actually about missing
bash — it was a cascading failure from #8's broken `for_each` substitution.
That's a separate issue, but it shows the heuristic also misclassifies
errors.

---

## Updated suggested filing

| # | Severity | Where it should land |
|---|---|---|
| 1 | HIGH | `as_user: root` short-circuit when uid=0 — small focused fix in executor |
| 2 | HIGH | shell action `creates:`/`unless:` should report `skipped` not `changed` |
| 3 | MEDIUM | one-time preset audit — apk/dnf/yum/pacman coverage |
| 4 | MEDIUM | validator error line/column accuracy |
| 5 | LOW | shell stdout visibility on first run |
| 6 | LOW | `LLM_GUIDE.md` action list regeneration |
| 7 | LOW | `examples/variables-and-facts/config.yml` fix |
| **8** | **CRITICAL** | `for_each` `{{ item }}` substitution — likely `internal/plan/planner.go` |
| 9 | (informational) | scoped #1 — pkg is fine, as_user is the bug |
| 10 | (informational) | saved-plan staleness — keep, feature it |
| 11 | LOW | snapshot tool inventory expansion (input to spec-59) |
| 12 | LOW | `metrics -q` ignores `--format json` |
| 13 | LOW | error-hint `suggested_step` emits invalid action name (`package:` vs `pkg:`) |

#8 is the standout — almost certainly the highest-impact single bug found
in this session, and probably a recent regression. Worth triaging before
anything else. If `for_each` is silently broken, every preset that loops
is silently broken, which is a much bigger blast radius than any of the
others.

---

## Continued exploration (round 3)

Six more surfaces probed: text.* actions, file.download checksum,
init/doctor, tag filtering, JSON output, template engine, and vars
overlays. Numbering continues from #13.

### 14. `file.download` silently ignores `sha256:` — SEVERITY: CRITICAL (security)

**Repro**: `/tmp/mooncake-tests/download/bad-sum.yml` — declares
`sha256: "0000…0000"` for a known-good jq binary URL.

```
$ docker run --rm -v $PWD/out/mooncake-static:/usr/local/bin/mooncake \
    -v /tmp/mooncake-tests/download:/work:rw -w /work ubuntu:24.04 bash -c '
    mkdir -p /tmp/dl
    mooncake apply -c bad-sum.yml -l debug'
```

Output:
```
▶ download with WRONG sha256
  Downloading: https://github.com/.../jq-linux-amd64 -> /tmp/dl/jq-bad
Failed to close temp file: close /tmp/...-2571834238: file already closed
Failed to remove temp file: ... no such file or directory
~ download with WRONG sha256
RECAP  ok=0  changed=1  skipped=0  failed=0  199ms
```

`ls /tmp/dl/jq-bad` → file present, 2,319,424 bytes.
`sha256sum /tmp/dl/jq-bad` → `5942c9b…` (the real jq hash), **not**
`0000…` (the declared one).

**Why CRITICAL**: `sha256:` exists exactly to defend against URL tampering
/ swap / MITM. If it's silently bypassed, every preset that downloads a
binary with checksum is providing only the illusion of safety.
`docs/LLM_GUIDE.md` recommends "Binary download + checksum" as
installation tier 3 — the recommendation pattern itself is broken.

**Likely cause**: the debug log says `Failed to close temp file ... file
already closed` and `Failed to remove temp file ... no such file or
directory`. Suggests there's verification logic that runs after the
download, but the order is wrong: the file is moved to the destination
**before** the verify step, and when verify fails it tries to clean up a
temp file that no longer exists. Net effect: bad bytes land on disk,
verify silently swallows the cleanup error, action reports success.

**Edge case observed**: when the destination *parent dir* doesn't exist
yet, the rename fails first and the verify-then-cleanup path masks the
checksum mismatch as `failed to move file: rename ... no such file or
directory`. Different failure shape, same root cause.

**Suggested fix order**:
1. Verify checksum on the temp file **before** rename.
2. On mismatch, return a clear `ChecksumMismatchError { expected, got,
   url }` — not a generic rename failure.
3. Treat the cleanup errors (close / remove) as bugs in cleanup, not
   as part of the user-facing success/failure boolean.

---

### 15. `creates:` and `unless:` are silently ignored on `file.write` — SEVERITY: HIGH (correctness)

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

Same with `unless: test -f /tmp/text/guarded.txt` — file is clobbered
anyway.

**Why HIGH**: `creates:` is the documented idempotency primitive. Users
write `creates: /opt/app/installed` to guard a download-and-install
step. With `file.write`, the guard does nothing and the step always
re-runs. (Side effect of this: in my round-2 idem.yml test, what
looked like a text.replace reporting bug was actually file.write's
seed step ignoring `creates:` and rewriting the file every run.
text.replace was correctly responding to the freshly-seeded content.)

**Likely cause**: `creates:` / `unless:` are honored by the executor's
generic shell pre-check path, but file actions short-circuit through
their own state-check (file content equality) without consulting the
step's `creates:` / `unless:` keys. Should be honored uniformly.

**Wider implication**: re-check all non-shell actions for `creates:` /
`unless:` honor. `text.replace`, `text.insert`, `pkg`, `file.copy`,
etc. — every action with custom state logic likely has the same gap.

---

### 16. Template engine HTML-escapes by default in `log: msg:` — SEVERITY: MEDIUM (correctness/DX)

**Repro**:

```yaml
- log:
    msg: "host_specific={{ undefined | default:'<unset>' }}"
```

JSON output:
```json
{"type":"print.message","data":{"message":"host_specific=&lt;unset&gt;"}}
```

The `<` and `>` rendered as `&lt;` `&gt;`. This appears in both terminal
output and the JSON `print.message` event payload — so it's happening
at render time, not at display time.

**Why it matters**: `log:` is the canonical "show me a value" surface.
Users debugging will quote literal angle brackets (e.g. `"<no value>"`,
`"# <changes>"`), or render YAML/HTML, and see escaped output that
doesn't match the input. The JSON event channel also leaks the
escape, which is wrong for any downstream consumer that isn't a
browser.

**Likely cause**: pongo2 / template engine has `autoescape: true` on
by default. For Mooncake's surface that should be `false` (or scoped
to actions that actually emit to HTML, of which there are none).

---

### 17. Live metrics not available in apply-time templates (despite docs) — SEVERITY: MEDIUM (doc drift / feature)

**Repro**:

```yaml
- log:
    msg: "load1={{ load_avg_1m }}"
```

Output: `load1=` (empty).

**Why it matters**: `LLM_GUIDE.md` line 130 explicitly says

> Use in templates: `{{ cpu_usage_pct }}`, `when: load_avg_1m < 4`

But `load_avg_1m` resolves to empty in `log: msg:` rendering. Either
the docs lie or the metrics-in-template wiring is not connected.

Two reasonable resolutions:
- (a) Wire metrics into the template variable map (matches the docs).
- (b) Restrict templates to facts and document that explicitly; provide
  an inline call like `{{ metric('load_avg_1m') }}` if you want
  on-demand sampling.

(b) is arguably better — apply-time template rendering of live metrics
would sample once per template and have unclear caching semantics; an
explicit accessor is clearer. But pick one and make docs match.

---

### 18. Jinja2-style filter args `default('x')` don't parse — SEVERITY: LOW (DX surprise)

**Repro**:

```yaml
- log: { msg: "{{ undefined | default('fallback') }}" }
```

Errors:
```
failed to render message: [Error (where: parser) in <string> | Line 1
Col 31 near '('] '}}' expected
```

Works with the Pongo2 colon syntax: `default:'fallback'`.

**Why it matters**: every Ansible/Jinja2 tutorial in the world uses
`default('x')`. Users will type it first, get a cryptic
"`}}` expected" error pointing at the open paren, and conclude the
template engine is broken. The fix is either:
- (a) Accept both forms.
- (b) Document the colon syntax loudly in the templates doc.
- (c) Improve the error message: "filter argument syntax is `|
  filter:value`; got `(`. Did you mean `default:'fallback'`?".

---

### 19. `--tags <typo>` silently runs only untagged steps — SEVERITY: LOW (UX)

**Repro**:

```
$ mooncake apply --tags deplly   # typo of 'deploy'
... only the untagged steps run, with no warning ...
RECAP  ok=1 changed=0 skipped=3 failed=0
```

**Why it matters**: a typo in a CI invocation silently degrades to
"only untagged steps". The user sees green (`failed=0`) but the
intended deploy steps never ran. This is the same shape as the for_each
bug — green CI, broken behavior.

**Fix**: when no step matches any of the requested tags, exit nonzero
with `no steps matched tags: deplly. Did you mean: deploy?` (fuzzy
suggestion from the tag inventory).

---

### 20. Bad-checksum download leaks "rename failed: no such file" when parent dir is missing — SEVERITY: LOW (error-message clarity)

See #14 for context. When `/tmp/dl/` doesn't exist:

```
failed to move file: rename /tmp/mooncake-download-2352616758 /tmp/dl/jq-bad:
  no such file or directory
```

The real error is checksum mismatch + missing parent dir; the surface
message is just the rename failure. Same root cause as #14 (verify-after-
move ordering). Fixing #14 fixes this too.

---

### 21. Static binary in vanilla `ubuntu:24.04` fails TLS without `ca-certificates` — SEVERITY: LOW (packaging)

**Repro**: `mooncake apply` with any `file.download:` step in a fresh
`ubuntu:24.04` container with no `ca-certificates` installed:

```
failed to download: ... tls: failed to verify certificate: x509:
  certificate signed by unknown authority
```

**Why it matters**: this is the same minimal-image story as #1. A user
following "Docker for AI agents" framing and pulling
`ubuntu:24.04` to test mooncake will hit TLS errors on the first
download. Same kind of friction.

**Options**:
- (a) Make the official `Dockerfile` (already uses alpine 3.21 with
  ca-certificates installed) the prominent quickstart; remove
  any docs that suggest "drop the binary into ubuntu:24.04".
- (b) Ship a tiny static-with-rooted-CA bundle (bundle Mozilla's CA
  list in the binary).
- (c) `mooncake doctor` already warns about missing tools — add a
  "ca-certificates not on PATH" check.

---

## What worked well (round 3)

Three more keepers worth not regressing:

- **`mooncake doctor`** — sections, ✓/ℹ/⚠ glyphs, **specific fix
  suggestions with URLs**. The best UX in the whole CLI. Should be
  promoted prominently.
- **`mooncake init --non-interactive --template empty`** — clean
  scaffold (`.gitignore` + `mooncake.vars.yml` + `mooncake.yml` +
  `.mooncake/`), the generated playbook applies cleanly on first run.
- **`--output-format json`** — newline-delimited JSON, `run.started`
  → `step.started` → `step.completed` → `run.completed`, stable
  `step_id`, full result blob. Production-quality LLM/programmatic
  surface.

---

## Updated suggested filing (round 3)

| # | Severity | Where it should land |
|---|---|---|
| **14** | **CRITICAL/security** | `file.download` — verify checksum BEFORE rename; emit `ChecksumMismatchError` |
| **15** | **HIGH** | unify `creates:` / `unless:` honor across all action handlers |
| 16 | MEDIUM | disable template autoescape (or scope it) |
| 17 | MEDIUM | metrics-in-templates: wire it or update docs |
| 18 | LOW | accept `default('x')` filter args, or improve parse error |
| 19 | LOW | `--tags <typo>` should warn, with fuzzy suggestion |
| 20 | LOW | (subsumed by #14) |
| 21 | LOW | `mooncake doctor` adds ca-certificates check; docs prefer alpine quickstart |

---

## Severity rollup (across all rounds)

| Severity | Findings | Notes |
|---|---|---|
| **CRITICAL** | #8, #14 | for_each broken; sha256 bypassed |
| **HIGH** | #1, #2, #15 | as_user/sudo; shell guard recap; file.write guards |
| **MEDIUM** | #3, #4, #16, #17 | preset coverage; validator UX; HTML escape; metrics-in-templates |
| **LOW** | #5, #6, #7, #11, #12, #13, #18, #19, #20, #21 | DX surprises and doc drift |
| **(info — keep)** | #10, +doctor, +init non-interactive, +JSON output | features to feature |

The top three (#8 for_each, #14 sha256, #15 guard honor) all share a
theme: **silent correctness failure that returns success**. CI passes,
the recap is green, but the action either did nothing useful or did
something unverified. That class of bug is the most dangerous because
nobody learns about it from feedback — they learn about it from
production incident reports. Worth fixing first.
