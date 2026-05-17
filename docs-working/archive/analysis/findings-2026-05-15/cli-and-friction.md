# CLI Nits + Minimal-Container Friction

DX papercuts, error-message quality, and the "drop the binary into a
fresh docker container" story.

---

## #1 — `as_user: root` invokes `sudo` even when uid=0 — HIGH

**Repro**: apply any preset using `as_user: root` (the `jq` preset
does) in vanilla `ubuntu:24.04` or `alpine:3.21`:

```
▶ Install jq (Linux)
  Installing packages: jq
failed to install packages [jq]: exec: "sudo": executable file not found in $PATH
✗ Install jq (Linux)
```

**Why HIGH**: official base images don't ship sudo. Every preset
that hardcodes `as_user: root` (canonical pattern for system installs)
is unusable in CI containers, minimal images, and most Docker
quickstarts — the audience the "Docker for AI agents" framing
attracts.

**Fix**: when the current process is already uid=0, `as_user: root`
should short-circuit and run directly, not invoke `sudo`. Check is
`os.Geteuid() == 0`.

**Scope confirmation**: `pkg:` directly (without `as_user: root`)
works on ubuntu/alpine/debian with no sudo — proving the executor's
general "run-as-root" path is correct. The bug is localized to
the `as_user: root` handler.

---

## #21 — Static binary in vanilla `ubuntu:24.04` fails TLS without `ca-certificates` — LOW (packaging)

**Repro**: `mooncake apply` with any `file.download:` step in fresh
`ubuntu:24.04` with no ca-certificates:

```
failed to download: ... tls: failed to verify certificate: x509:
  certificate signed by unknown authority
```

Same minimal-image story as #1. A user following "Docker for AI
agents" and pulling `ubuntu:24.04` to test mooncake hits TLS errors
on first download.

**Options**:
- (a) Make the official `Dockerfile` (alpine 3.21 with
  ca-certificates) the prominent quickstart; drop docs suggesting
  "drop the binary into ubuntu:24.04".
- (b) Bundle Mozilla's CA list in the binary (rooted CA).
- (c) `mooncake doctor` adds a "ca-certificates missing" check.

---

## #11 — `snapshot --diff` only diffs hw + curated tool list — LOW (surface gap)

**Repro**:
```
$ mooncake snapshot --format json --save snap1.json
$ apt-get install -y jq
$ mooncake snapshot --diff snap1.json
hw:
  ~ ram_free_mb          25451 → 25454
```

New tool (`jq`) installed between snapshots does not surface — `tools:`
map only includes a curated allowlist (bash version in this case).

**Suggestion**: this is probably an `observe.*` story (spec-59), not
a snapshot fix per se. Worth noting as input to that spec.

**Round 35 verification**: confirmed the `tools:` map is a *curated
dev-tools list*. After `apt-get install jq`, jq does NOT appear; but
after `apt-get install golang`, `go: "1.22.2"` does. So bash/go/likely
python/node/etc. are detected by name; arbitrary CLIs aren't. This
is reasonable for runtime context but the docs should say so —
otherwise users assume snapshot is a `mooncake-aware inventory`.

```json
"tools": {
  "bash": "5.2.21",
  "go": "1.22.2"
}
```

JSON diff form is clean:
```json
{
  "hw": {
    "ram_free_mb": {"from": 23132, "to": 23065}
  }
}
```

---

## #12 — `metrics -q <key> --format json` silently ignores `--format` — LOW

**Repro**:
```
$ mooncake metrics --format json -q cpu_usage_pct -q memory_used_pct
cpu_usage_pct=3.08
memory_used_pct=17.41
```

`key=value` lines instead of JSON. `--fields` *does* honor `--format
json`. Either `-q` should respect `--format` too, or the help text
should call out that `-q` is text-only.

---

## #13 — Error-hint `suggested_step` emits invalid YAML — LOW

**Repro**: any failing shell command triggering the "is not installed"
heuristic. Example:

```json
{
  "step": "<[]interface",
  "action": "shell",
  "stderr": "bash: line 1: lt: command not found\n...",
  "hint": "bash: is not installed",
  "suggested_step": "package:\n  name: bash:\n  state: present"
}
```

Two bugs in one hint:
1. `package:` is not in the schema's vocabulary — the correct action
   is `pkg:`.
2. The package name comes out as `bash:` (trailing colon retained
   from stderr parsing).

Also, the hint fired on a bash error that wasn't about missing bash
— it was a cascading failure from #8's broken `for_each` substitution.
Separate issue, but it shows the heuristic also misclassifies errors.

---

## #19 — `--tags <typo>` silently runs only untagged steps — LOW (UX)

**Repro**:
```
$ mooncake apply --tags deplly   # typo of 'deploy'
... only the untagged steps run, with no warning ...
RECAP  ok=1 changed=0 skipped=3 failed=0
```

A typo silently degrades to "only untagged steps run". User sees
green (`failed=0`) but intended deploy steps never executed.

**Fix**: when no step matches any requested tags, exit nonzero with
`no steps matched tags: deplly. Did you mean: deploy?` (fuzzy
suggestion from the tag inventory).

---

## #42 — `wait.command` attempt counter off — LOW

**Repro**:
```
$ mooncake step "wait.command: { cmd: 'test -f /tmp/x', timeout: 1s, interval: 200ms }"
{"error": "wait.command timeout after 1s (1 attempts); last exit 1"}
```

"1 attempts" with `interval: 200ms` and `timeout: 1s` — should be ~5
attempts. Either interval parsing failed silently or counter is
off-by-(N-1). Low severity; the wait/timeout behavior is correct.

---

## #25 — MCP server replies to `notifications/initialized` with malformed `{"jsonrpc":""}` — LOW (protocol)

**Repro**:
```
> {"jsonrpc":"2.0","method":"notifications/initialized"}
< {"jsonrpc":""}
```

JSON-RPC 2.0 says notifications MUST NOT receive responses.
Empty-version `jsonrpc` is also invalid. Strict MCP clients could
refuse to talk further.

**Fix**: handle notifications without emitting any response.

---

## #26 — MCP server `run_plan` error has duplicated prefix — LOW (cosmetic)

```
> {"method":"tools/call","params":{"name":"run_plan","arguments":{"config":"/tmp/no-such-file.yml"}}}
< {"error":{"code":-32000,"message":"failed to read config: failed to read config: open /tmp/no-such-file.yml: no such file or directory"}}
```

`failed to read config:` appears twice. Two layers of wrapping both
adding the same prefix.

---

## #20 — Bad-checksum + missing parent dir produces misleading "rename failed" error — LOW

**Repro**: see [`silent-success-bugs.md` #14](./silent-success-bugs.md#14)
for context. When `/tmp/dl/` doesn't exist:

```
failed to move file: rename /tmp/mooncake-download-2352616758 /tmp/dl/jq-bad:
  no such file or directory
```

Real error is checksum mismatch + missing parent dir; surface message
is just the rename failure. Same root cause as #14. Fixing #14 fixes
this too.

---

## Summary table

**Status rollup as of 2026-05-18: all CLI/friction findings ✅ FIXED
except #11 (curated tools list — design decision, not a bug).**

| # | Sev | Status | Area | Fix |
|---|---|---|---|---|
| 1 | HIGH | ✅ FIXED | `as_user: root` always sudoes | uid=0 short-circuit `81dc50e` |
| 21 | LOW | ✅ FIXED | TLS in vanilla ubuntu container | doctor flags missing ca-certificates `49e41406` |
| 11 | LOW | (design decision) | snapshot tool inventory | curated dev-tools list is intentional; doc clarification only |
| 12 | LOW | ✅ FIXED | `metrics -q --format json` | proper JSON object `7007130` / `16a29dd` |
| 13 | LOW | ✅ FIXED | error-hint invalid YAML | `suggested_step` uses `pkg:`, clean cmd name `796ed2f7` |
| 19 | LOW | ✅ FIXED | `--tags <typo>` silent fail | errors on zero-match tag filter `2ad89456` |
| 42 | LOW | ✅ FIXED | `wait.command` attempt counter | `interval:` alias for `poll_interval:` `ff975066` |
| 25 | LOW | ✅ FIXED | MCP notification reply | suppress responses to notifications `8b688526` |
| 26 | LOW | ✅ FIXED | MCP error prefix dup | strip duplicate prefix `f5b59e0f` |
| 20 | LOW | ✅ FIXED | bad-checksum + missing parent | subsumed by #14 fix |
| 53 | LOW | ✅ FIXED | `--artifacts-dir` results.json | flush publisher before close `b0b4b9b2` |
| 55 | LOW | ✅ FIXED | `runs apply` empty step names | falls back to action name `8ca42a89` |
| 56 | LOW | ✅ FIXED | `runs list --format json` | JSON output added `db5648b1` / `9dfa978d` |
| 57 | LOW | ✅ FIXED | `runs <subcommand>` error format | suggests nearest command `68c474f7` |
| 58 | LOW | ✅ FIXED | `--skip-tags` exclusion flag | added `3efa25da` |
| 59 | LOW | ✅ FIXED | fuzzy hint on `--tags` typo | `closestTag` (Levenshtein) suggests nearest match — `internal/plan/filter/tags.go:103-123`; covered by `TestMT19_UnmatchedTagsError/typo errors with suggestion` |
| 61 | MEDIUM | ✅ FIXED | `error:` populated + `failed:false` | `step` sets failed=true on handler error `e516b220`; observe.process slice `37a6ddef` |
| 65 | LOW | ✅ FIXED | `--max-plan-age` boundary | sub-second precision in error `8dc0145d` |
| 66 | LOW | ✅ FIXED | bad CLI flag dumps help | suppress help on flag parse error `e9c9f342` |
| 68 | LOW | ✅ FIXED | `--output-format` vs `--format` | aliased on `apply` `45b30779` + `validate` `8d0cd9c0` |
| 69 | LOW | ✅ FIXED | `validate --format json` PascalCase | snake_case keys `f221f332` |
| 71 | LOW | ✅ FIXED | doctor disk-space probe | walks to existing ancestor `e7834f40` |
| 72 | LOW | ✅ FIXED | top-level dict YAML | step-shape hint `cf184c9f` |
| 73 | LOW | ✅ FIXED | empty config file | clear error message `3c08fe1f` |
| 74 | LOW | ✅ FIXED | `--facts-json` PascalCase | snake_case keys `77de8908` |
| 75 | LOW | ✅ FIXED | `plan -o plan.yaml` null fields | omit empty union members `16d4329c` |
| 76 | LOW | ✅ FIXED | `--capture-full-output` silent no-op | hard-errors without `--artifacts-dir` `6366b2d` |
| 77 | LOW | ✅ FIXED | validator empty-vocabulary error | unknown-field message instead `2d1c09e7`; `creates:`/`unless:` universal `50768578` |
| 78 | LOW | ✅ FIXED | `peers.toml` TOML error | array-of-tables hint `fd478912` |
| 81 | LOW | ✅ FIXED | `as_user` generic errors | targeted errors when can't escalate `355d48ba` |
| 85 | LOW | ✅ FIXED | `--ask-become-pass` no TTY | actionable error `f01b10cc` |
| 86 | LOW | ✅ FIXED | `--max-output-lines` silently ignored | require `--artifacts-dir` `3cf6e2aa` |
| 87 | MEDIUM | ✅ FIXED | `apply` doesn't exit on SIGINT | proper signal handling `7b55547d` |

---

## #53 — `--artifacts-dir` produces no `results.json` despite CLI help — LOW (doc/feature drift)

**Repro**:
```
$ mooncake apply -c hello-world/config.yml --artifacts-dir /tmp/artifacts --capture-full-output
$ ls /tmp/artifacts/runs/*/
events.jsonl  facts.json  plan.json  stderr.log  stdout.log
```

But `mooncake apply --help` says:
- `--max-output-bytes value  Max bytes of output per step in results.json (default: 1048576)`
- `--max-output-lines value  Max lines of output per step in results.json (default: 1000)`

Both flags reference a `results.json` that is never written.

**Fix**: either generate `results.json` (a per-step result summary makes sense
alongside the raw `events.jsonl`) or drop the references from `--max-output-*`
help text.

(The rest of the bundle is good — `stdout.log` is prefixed `[step-0001] line`
which is exactly what the default-renderer is missing per #5. Worth promoting.)

---

## #55 — `mooncake runs apply` streams empty step names — LOW

**Repro** (agentd running on 127.0.0.1:7878):
```yaml
# /tmp/r/cfg.yml — no name: on steps
- log: { msg: "hi from daemon" }
- file.write: { path: /tmp/from-daemon.txt, content: "yes" }
```

```
$ mooncake runs apply -c /tmp/r/cfg.yml
Run 01KRPEH07KHKNGN8KVT8F7M4DV submitted; streaming events...

▶ 
✓ 
▶ 
~ 

RECAP  ok=2  changed=1
```

Step markers print but with empty names. Local `mooncake apply` on the
same config also renders empty names, but at least falls back to the
action name in some places. The runs-streaming channel should
fall back to action name (`shell`, `log`, `file.write`) when `name:`
is omitted, like the artifact `stdout.log` does (`[step-0001]`).

(File lands correctly; this is purely rendering UX.)

---

## #56 — `mooncake runs list --format json` errors — LOW

```
$ mooncake runs list --format json
flag provided but not defined: -format
```

Same gap as #38 (`presets list --format json`). The runs-list command
on the daemon side returns JSON natively (`/v1/runs`); the CLI side
should accept `--format json` to forward that response verbatim.

---

## #72 — Top-level dict YAML reports confusing "unknown field log" — LOW (DX)

**Repro**:
```yaml
# /work/dict.yml — user forgot the leading dash; this is a dict not a list
log:
  msg: hi
```

```
$ mooncake apply -c /work/dict.yml
Error: /work/dict.yml
  Line 1: unknown field `log` (likely a typo or a renamed field — see docs-next/guide/config/actions.md)
```

But `log` IS a valid action name. The real problem is that the file is
a top-level dict instead of `[{log: ...}]` list. The "renamed/typo"
hint sends users down the wrong path.

**Fix**: detect top-level dict and emit a specific hint:
```
Error: /work/dict.yml expected a list of steps; got an object.
  Add a leading `- ` to each step:
  - log:
      msg: hi
```

---

## #73 — Empty config file says only "EOF" — LOW (DX)

**Repro**:
```
$ > /work/empty.yml
$ mooncake apply -c /work/empty.yml
planner setup failed: failed to build plan: failed to read config: EOF
```

Same for whitespace-only files. Bare "EOF" is the yaml library's
verbatim error. Could surface a specific message:
```
config file is empty: /work/empty.yml — expected a list of steps
```

---

## #86 — `--max-output-lines` and `--max-output-bytes` are silently ignored — LOW (DX)

**Repro**:
```yaml
- shell: "for i in $(seq 1 100); do echo line $i; done"
  as: out
```

```
$ mooncake apply -c c.yml --output-format json --max-output-lines 5
# step.completed.result.stdout contains ALL 100 lines:
"stdout": "line 1\nline 2\n...line 100\n"
```

Per `mooncake apply --help`:
- `--max-output-bytes value   Max bytes of output per step in results.json (default: 1048576)`
- `--max-output-lines value   Max lines of output per step in results.json (default: 1000)`

Both flags reference `results.json` (which doesn't exist per #53)
AND don't actually limit the stdout returned in `step.completed.result.stdout`.

**Why LOW**: in practice 1MB / 1000-line limits are fine for most
runs, but the documented flags don't actually do what they say.

**Fix**: either implement output truncation in `result.stdout`/
`result.stderr` honoring these flags, or drop them from `--help`.

---

## ★ Mooncake scales linearly — 5000 steps in 16s (~307 steps/sec)

```
$ {for i in $(seq 1 5000); do echo "- log: { msg: \"step-$i\" }"; done} > big.yml
$ time mooncake apply -c big.yml --output-format json | wc -l
15003
elapsed=16258ms
```

500 steps: 2.8s (~180 steps/sec startup-dominated)
5000 steps: 16.3s (~307 steps/sec steady state)

JSON event stream: 5000 × (step.started + print.message + step.completed) + 3
overhead = 15003 lines, all properly serialized. No memory spike
observed. Linear scaling.

For comparison, this is in the same ballpark as Ansible-on-localhost
for a similar 5000-step playbook. Good engineering.

---

## #87 — `mooncake apply` doesn't exit on SIGINT (Ctrl-C) mid-run — MEDIUM

**Repro**:
```yaml
- shell: "touch /tmp/before && sleep 30 && touch /tmp/after"
```

```
$ mooncake apply -c slow.yml &
$ MOON_PID=$!
$ sleep 2
$ kill -INT $MOON_PID
$ sleep 3
$ test -f /tmp/before && echo before-yes      # yes (child started)
$ test -f /tmp/after && echo after-yes         # no (sleep killed)
$ kill -0 $MOON_PID && echo "STILL ALIVE"      # ← STILL ALIVE
```

Behavior:
- The shell child (sleep) is killed (good — SIGINT propagates to
  the process group)
- But `mooncake apply` itself stays alive forever
- No "interrupted" line in `~/.mooncake/runs.jsonl`
- User has to `kill -KILL` to terminate

This makes interactive Ctrl-C unusable for long-running runs. Users
hitting Ctrl-C expect either:
- (a) immediate exit with non-zero status + history entry showing
  "interrupted at step N"
- (b) prompt to confirm (Ctrl-C again to force)

Right now: silent hang.

**Fix**: install a SIGINT/SIGTERM handler that:
1. Cancels the current step's context (kills its child processes)
2. Records the run as interrupted in `~/.mooncake/runs.jsonl`
3. Exits with code 130 (SIGINT) or 143 (SIGTERM) — standard convention

(Found while testing concurrency / agentd lifecycle. Worth fixing
before fleet exec gets used in production — orphan children +
zombie agentd state are the same root cause.)

---

## #85 — `--ask-become-pass` without TTY emits `inappropriate ioctl for device` — LOW

**Repro**:
```
$ mooncake apply --ask-become-pass -c cfg.yml <&-     # no TTY
BECOME password:
sudo password setup failed: failed to resolve password: failed to read password: inappropriate ioctl for device
```

The Go `term.ReadPassword(0)` call returns this generic OS-level
error when stdin isn't a TTY. The wrapper prints a prompt
("BECOME password: ") even though the read will immediately fail.

**Fix**: detect at startup whether stdin is a TTY before prompting.
If not:
```
--ask-become-pass requires an interactive terminal.
  Use --sudo-pass-file <path> (0600 permissions) instead.
```

(Same template as the `mooncake init` non-TTY hint from finding round 11.)

---

## #81 — `as_user: <name>` errors are generic when sudo is missing or user doesn't exist — LOW

**Repro** (running as root, sudo not installed):
```yaml
- shell: id
  as_user: alice
```
```
$ mooncake apply -c cfg.yml
command failed with exit code 1
```

```yaml
- shell: id
  as_user: nobodyxyz   # nonexistent user
```
```
$ mooncake apply -c cfg.yml
command failed with exit code 1
```

Both errors are identical and generic. The user has no way to tell:
- "sudo isn't installed" vs.
- "the named user doesn't exist" vs.
- "the shell command itself failed"

**Compare with MT-1 fix**: `as_user: root` correctly short-circuits
when uid=0. The same pre-flight check should look at sudo
availability and `getent passwd <user>` before invoking, and emit
specific errors:
```
as_user: alice requires sudo; sudo not on PATH
  fix: apt-get install sudo (or run as the target user directly)

as_user: nobodyxyz: user does not exist on this host
  fix: check spelling, or os.user: { name: nobodyxyz, state: present } first
```

Same template as `mooncake doctor` already uses for missing tools.

---

## #78 — `peers.toml` TOML parse error is unhelpful for the wrong-but-plausible form — LOW (DX)

**Repro** — natural mistake when writing `~/.config/mooncake/peers.toml`:
```toml
[peers.local]                    # dotted-key form (wrong)
addr = "127.0.0.1:7878"
token = "..."
```

```
$ mooncake fleet status
parse /root/.config/mooncake/peers.toml: toml: cannot store a table in a slice
```

The error is technically accurate but doesn't tell the user the right
form. The correct shape is:
```toml
[[peers]]                        # array-of-tables (correct)
name = "local"
addr = "127.0.0.1:7878"
token = "..."
```

**Fix**: catch this specific TOML error and emit:
```
peers.toml: expected `[[peers]]` array-of-tables. Did you mean:
  [[peers]]
  name = "local"
  ...
(See `mooncake fleet --help` for the schema.)
```

(`mooncake fleet init` interactive flow might already do this right;
this matters for users editing peers.toml by hand.)

---

## #77 — Validator error "Step must have exactly one action ()" with EMPTY action list — LOW (regression of #27 vocabulary)

**Repro**:
```yaml
- file.write:
    path: /tmp/guarded.txt
    content: "v1\n"
  creates: /tmp/guarded.txt    # step-level creates: no longer allowed?
```

```
$ mooncake apply -c cfg.yml
Line 1: Step must have exactly one action ()
  - file.write:
```

The error message has an EMPTY parenthesized list `()` where it
should list the valid action vocabulary (per #27 fix that landed
showing 62 action names). For this specific case `creates:` at
step-level + `file.write:` at step-level evidently makes the
validator see "more than one action" — but instead of saying that,
it lists an empty enum.

Probably an off-by-one in the vocabulary-printing path when the
"too many actions" branch fires instead of the "no action" branch.

**Fix**: when the error is "too many actions", say so, and list which
keys looked like actions. When it's "unknown action", list the
vocabulary.

(Aside: step-level `creates:` on `file.write` used to silently no-op
per the original #15. Now it errors at validate time — a different
kind of fix. Marking #15 as ✅ resolved by removal-of-silent-bypass.)

---

## #76 — ✅ FIXED (verified round 32, commit `6366b2d`)

```
$ mooncake apply -c c.yml --capture-full-output
--capture-full-output requires --artifacts-dir (the captured logs need a directory to land in)
```

Clean hard-error matching the help text. No more silent no-op.

### Original report (now resolved)

## #76 (original) — `--capture-full-output` silently no-ops without `--artifacts-dir` — LOW (DX)

**Repro**:
```
$ mooncake apply -c hello-world/config.yml --capture-full-output
# runs normally, no warning, no capture (since --artifacts-dir is unset)
```

Per `mooncake apply --help`:
> `--capture-full-output  Capture full stdout/stderr to artifacts (requires --artifacts-dir)`

The help text says "requires --artifacts-dir" but supplying
`--capture-full-output` alone doesn't error — it just silently does
nothing. Users who set the flag expecting capture get no warning
that they forgot the partner flag.

**Fix**: either warn at startup ("--capture-full-output ignored
because --artifacts-dir is not set"), or hard-error matching the
help text's "requires".

(Reverse case: `--artifacts-dir` without `--capture-full-output` is
valid — it just emits the JSONL/plan/facts but no captured
stdout/stderr. That's already correct behavior.)

---

## ★ `--tui --output-format json` is properly mutually-exclusive

```
$ mooncake apply --tui --output-format json
--output-format json cannot be combined with --tui
```

Clean error. Right kind of validation for incompatible flag pairs.

---

## #74 — `--facts-json` output uses PascalCase keys — LOW (style, extends #69)

**Repro**:
```
$ mooncake apply --facts-json /work/facts.json
$ head /work/facts.json
{
  "OS": "linux",
  "Arch": "amd64",
  "Hostname": "5d49de35216d",
  "Username": "root",
  "UserHome": "/root",
  ...
}
```

PascalCase, same outlier as `validate --format json` (#69). Every
other JSON surface uses snake_case (`os`, `hostname`, `user_home`).

**Fix**: switch to snake_case JSON tags on the FactsSet struct.
Probably comes from `json:"OS,..."` tags being absent and Go's
default-PascalCase falling through.

---

## #75 — `plan -o plan.yaml` serializes all union-member null fields per step — LOW (noise)

**Repro**:
```
$ mooncake plan -c cfg.yml -o /work/plan.yaml
$ head -25 /work/plan.yaml
steps:
  - name: ""
    when: ""
    unless_exists: null
    unless_command: null
    creates: null
    unless: null
    file.write: null         ← all 60+ action types serialized with null
    file.template: null
    file.copy: null
    file.download: null
    file.unarchive: null
    text.line: null
    text.replace: null
    text.insert: null
    ...
```

Every step gets ~60 lines of `<action>: null` for action types it
doesn't use. The JSON form (default) omits these (`omitempty`). Only
YAML serialization keeps them.

**Why LOW**: plans round-trip correctly (`apply --from-plan
plan.yaml` works). Just noisy and harder to inspect.

**Fix**: add `,omitempty` to YAML tags on action union fields, or use
inline maps for the action body.

---

## #71 — `mooncake doctor` reports "disk-space probe unsupported on this OS" on Linux — LOW (false negative)

**Repro**:
```
$ mooncake doctor
...
State
  ℹ /root/.mooncake does not exist (will be created on first run)
  ℹ no run history yet
  ℹ disk-space probe unsupported on this OS    ← Linux ubuntu:24.04
```

But `mooncake facts` and `observe.disk` both successfully report disk
space on the same machine. `mooncake doctor` has its own probe that
returns "unsupported" on Linux containers — must use a different
codepath than facts/observe.

**Fix**: route doctor's disk check through the same path as
`observe.disk` / `facts.disk_free_gb` (both work on this OS). Or
mark it `ℹ` (info, expected) on cgroup-limited environments.

(Cosmetic — `ℹ` is informational, not an error.)

---

## ★ `mooncake snapshot --budget N` — token-budget summary for LLM consumption

```
$ mooncake snapshot --budget 100      # ~400 char budget (1 token ≈ 4 chars)
os: linux/ubuntu amd64  kernel: ... host: ...
hw: 32 cores ...  ram: ... disk: ...
uptime: 5h 40m

tools:
  bash:    5.2.21

$ mooncake snapshot --budget 50       # ~200 char budget
os: linux/ubuntu amd64  kernel: ...
hw: 32 cores ...
uptime: 5h 40m
# tools: section dropped to fit budget
```

This is a thoughtful surface for LLM agents that need to pack
system context into a small prompt. Drops the most-expensive sections
first. Worth promoting in agent-integration docs.

---

## #68 — `validate --format` vs `apply --output-format` — naming inconsistency — LOW (DX)

**Repro**:
```
$ mooncake apply --output-format json     # works
$ mooncake validate --output-format json
Incorrect Usage: flag provided but not defined: -output-format

$ mooncake validate --format json          # works
```

Same concept (output format), different flag name across commands.
`metrics`, `snapshot`, `history list`, `plan`, `presets list`,
`schema generate`, `validate` all use `--format`. Only `apply` (and
`runs apply`) uses `--output-format`.

**Fix**: standardize on `--format` everywhere; alias `--output-format`
on `apply` for back-compat.

---

## #69 — `validate --format json` uses PascalCase field names — LOW (style)

**Repro**:
```
$ mooncake validate -c bad.yml --format json
{
  "valid": false,
  "diagnostics": [
    {
      "FilePath": "/work/bad.yml",       ← PascalCase
      "Line": 1,                         ← PascalCase
      "Column": 1,
      "Message": "...",
      "Severity": "error"
    }
  ]
}
```

Every other JSON surface (`apply --output-format json`, `metrics
--format json`, `step` result, MCP responses) uses snake_case
(`step_id`, `duration_ms`, `as_of`). validate is the outlier.

**Fix**: emit `file_path`, `line`, `column`, `message`, `severity`.

---

## #65 — `--max-plan-age` boundary is inclusive (rejects at exactly N) — LOW

**Repro**:
```
$ mooncake plan -o /work/plan.json
$ mooncake apply --from-plan /work/plan.json --max-plan-age 1s
refusing to apply stale plan: plan is 1s old; --max-plan-age is 1s
```

Plan is exactly 1s old; `--max-plan-age 1s` rejects. Most users would
expect `age > max_age` (strict), not `age >= max_age` (inclusive).
"Older than 1 second" should mean strictly older.

**Fix**: change `>=` to `>` in the comparison; document boundary.

---

## #66 — Bad CLI flag value dumps help text before error — LOW

**Repro**:
```
$ mooncake apply --max-plan-age garbage
... (dumps full --help output) ...
2026/05/15 19:02:18 invalid value "garbage" for flag -max-plan-age: parse error
```

The error itself is clean (`invalid value ... parse error`), but it's
preceded by the entire help output. urfave/cli default behavior.
Hides the actual error and floods the terminal.

**Fix**: configure urfave/cli to suppress help-on-flag-parse-error, or
post-process to surface the error first.

---

## #61 — Recurring pattern: `error:` populated but `failed: false` — MEDIUM (consistency)

Observed across multiple actions:

```
observe.process (no matching process):
  {"failed": false, "error": "no matching process", "found": false}

wait.http (timeout):
  {"failed": false, "success": false, "error": "wait.http timeout after 1.001s..."}

os.mount (mount failure):
  {"failed": false, "error": "os.mount: mount /tmp/tmpfs: exit status 32: ...", "operation": "create"}

os.firewall (iptables denied):
  {"failed": false, "error": "os.firewall: read current rules: Permission denied..."}
```

**Why MEDIUM**: callers checking `result.failed` see `false` and assume
success — but the `error:` field is populated with a real diagnostic.
Three possible meanings, all confusing:
- "We did our best; the actual side effect failed but we didn't fail the step" (os.mount)
- "Observation completed; the answer is 'not present'" (observe.process)
- "Timeout is not a failure" (wait.http)

These should be distinct. **Recommended convention**:
- `failed: true` when the action couldn't do its job (os.mount couldn't mount; os.firewall couldn't read).
- `failed: false, found: false` for observe actions returning "not present".
- `failed: false, success: false, error: ""` for wait actions hitting timeout — with `error` empty (timeout is the answer, not an error).

Today, agents have to parse the `error` string to distinguish. That's
fragile.

---

## #58 — No `--skip-tags` exclusion flag — LOW (DX gap)

**Repro**:
```
$ mooncake apply --skip-tags a
flag provided but not defined: -skip-tags
```

`--tags X` includes, `--skip-tags X` excludes — Ansible's standard
pattern. Mooncake supports only inclusion. The "run everything except
slow tests" workflow needs a workaround (tag every step `default` and
exclude the slow tag, etc.).

**Fix**: add `--skip-tags <list>` paralleling `--tags`. Both can
coexist (`--tags deploy --skip-tags slow` = include deploy, exclude slow).

---

## #59 — ✅ FIXED (already implemented at MT-19 landing, verified 2026-05-18)

`closestTag` (Levenshtein) runs against the plan's tag inventory and
appends `Did you mean: <best>?` when the typo is within distance
threshold. Verified at `internal/plan/filter/tags.go:103-123`; covered
by `TestMT19_UnmatchedTagsError/typo errors with suggestion` which
asserts `Did you mean: deploy` for input `deplly`.

### Original report (now resolved)

**Repro** (post-MT-19):
```
$ mooncake apply --tags depply
tags setup failed: no steps matched tags: depply (available: a, b)
```

The "available: a, b" list is helpful but doesn't suggest the closest
typo correction. The mooncake-doctor and validator both use fuzzy
suggestions; this should too:
```
tags setup failed: no steps matched tags: depply (did you mean: deploy?). available: a, b, deploy
```

(Tiny issue; MT-19 itself is a clear win — the error tells you
exactly what's wrong now.)

---

## #57 — `runs` subcommand error format inconsistent — LOW

```
$ mooncake runs submit /path/to/cfg
No help topic for 'submit'
```

`submit` is a guess — the real subcommand is `apply`. But the CLI
reports "No help topic" instead of "unknown subcommand: submit (try
apply / follow / get / list)". The `gh` CLI style ("unknown command
'submit', did you mean 'apply'?") would be a better template.
