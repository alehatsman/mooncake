# Positive Findings — Features To Feature, Do Not Regress

What works well and should be prominently featured in onboarding,
docs, and demos. Listed in roughly descending order of "biggest win".

---

## ★ `mooncake doctor` — best UX in the CLI

```
$ mooncake doctor
mooncake doctor — health check

Install
  ✓ mooncake dev
       /usr/local/bin/mooncake
  ℹ Go runtime: go1.26.3 linux/amd64
System
  ✓ os=linux arch=amd64 distribution=ubuntu package_manager=apt
       Use `mooncake facts` for the full list
State
  ℹ /root/.mooncake does not exist (will be created on first run)
       fix: run `mooncake apply` to auto-create, or `mooncake init` to scaffold a project
  ℹ no run history yet
       fix: run `mooncake apply` once and the log will be created
Preset search paths
  ⚠ no presets found in any search path
       fix: install the mooncake-presets package, or `mooncake presets update` to fetch from the remote registry
Tools
  ⚠ git not on PATH
       fix: install git — https://git-scm.com/downloads
       used by: git.* actions
  ⚠ fzf not on PATH
       fix: install fzf — https://github.com/junegunn/fzf
       used by: mooncake presets (interactive selector)
```

What's great:
- Sections (`Install`, `System`, `State`, `Preset search paths`, `Tools`)
- Status glyphs (`✓` `ℹ` `⚠`)
- **Specific fix suggestions with URLs**
- Says what's used for (`used by: git.* actions`)

This is the quality bar every other CLI surface should match.

---

## ★ `mooncake apply --dry-run` — the safety story working

```
$ mooncake apply -c /work/dry-run.yml --dry-run
Plan: dry-run.yml
Generated: 2026-05-15 16:43:36 on linux/amd64/ubuntu

↑ create marker file                                would create file (6 bytes)
    cost: risk 4 (routine) • reversible • 1 resource • 6 bytes
↑ shell side effect                                 would run: echo running > /tmp/dryrun-sideeffect

PLAN SUMMARY  would-change=2  ok=0  skipped=0  not-checkable=0  max-risk=4 (routine)
```

Verified zero side effects (no marker file, no shell side-effect file).
Each step gets a `risk N • reversible/irreversible • resource count •
bytes` annotation. Plan summary distinguishes `would-change` from `ok`
from `skipped` from `not-checkable`.

**Suggestion**: feature this prominently in onboarding and the
README. The safety story is real and demoable in two commands; users
should see it before they see anything else.

---

## ★ `mooncake init --non-interactive --template <name>` — clean scaffold

```
$ mooncake init --non-interactive --template empty
Created (template: empty):
  .gitignore
  mooncake.vars.yml
  mooncake.yml
  .mooncake/

Next:
  mooncake plan
  mooncake apply
  mooncake presets list
```

Generated playbook applies cleanly on first run. Templates: `empty`,
`dotfiles`, `server`, `agent-sandbox`.

---

## ★ `--output-format json` — production-quality JSONL surface

Per-event JSON: `run.started`, `step.started`, `step.stdout`,
`step.completed`, `run.completed`. Stable `step_id`. Full result blob
under `step.completed.data.result` with `changed/failed/skipped/rc/
stdout/stderr/status`.

**This is the AI-agent integration surface.** Should be promoted as
the "real" output channel; text is just a friendly summary. (See also
template-engine.md #36 — JSON channel captures shell stdout that
text drops.)

---

## ★ `mooncake plan` advanced flags — `--diff`, `--show-origins`, `--no-inspect`

`--diff` shows a unified diff of what the file would become:
```
↑ write                content differs (6 -> 7 bytes)
  --- /tmp/plan-target.txt
  +++ /tmp/plan-target.txt (proposed)
  @@ -1,1 +1,1 @@
  -after
  +before
```

`--show-origins` adds `file:line:col` under each step:
```
↑ write     content differs (6 -> 7 bytes)
    /work/cfg.yml:2:3
```

`--no-inspect` skips the per-step state check (faster, but plan can't
predict change vs. ok):
```
? write
PLAN SUMMARY  would-change=0  ok=0  skipped=0  not-checkable=1
```

`--format yaml` (and `--format json`) produces a round-trippable plan
with `when/unless_exists/creates/unless` metadata serialized.

This is genuinely impressive for an agent / fleet workflow:
- `plan --diff` for human review
- `plan --show-origins` for IDE click-through
- `plan -o plan.json` to capture for later replay
- `plan --no-inspect` for fast static expansion when state is trusted

---

## ★ Saved plans + `--from-plan` — strong safety property

```
$ mooncake plan -o plan.json
$ vim mooncake.yml          # change something
$ mooncake apply --from-plan plan.json
2026/05/15 16:48:38 refusing to apply stale plan: plan input files
have changed since the plan was built (use --allow-stale to override)
```

Plan JSON includes `input_files_hash`. Default refuses stale plans.
`--allow-stale` honors plan-time values: replayed plan wrote `v1`,
not the current source's `v2`. Confirming the plan is truly
self-contained.

**Suggestion**: this is the secret superpower no one talks about.
"Plan once, apply many times, get exactly the planned result" is a
fleet/auditing story.

---

## ★ `mooncake mcp` stdio server — real AI-agent integration

Exposes:
- `get_facts` — system facts
- `get_snapshot` — compact state
- `fact_query` — single fact by key
- `get_metrics` — live metrics
- `run_plan` — apply a config file
- `check_plan` — dry-run a config file

Clean JSON-RPC 2.0 over stdio. (Two cosmetic warts — see
`cli-and-friction.md` #25, #26.)

This is the production-quality integration point.

---

## ★ `mooncake schema generate` + `mooncake docs generate`

The action registry is **self-describing and codegen-able**.

```
$ mooncake schema generate --output schema.json
$ grep -o '"[a-z]*\.[a-z_.]*": *{' schema.json | sort -u | wc -l
44
```

Schema supports `--format json|yaml|openapi|typescript`. Docs has
sections `platform-matrix`, `capabilities`, `action-summary`,
`action-properties`, `preset-examples`, `schema`, `all`.

**The infrastructure to close the SSOT-drift findings already
exists** (see [`ssot-drift.md`](./ssot-drift.md)). Wire validator +
docs to these generators.

---

## ★ `mooncake actions list` — the honest action inventory

```
ACTION          CATEGORY   PLATFORMS                 SUDO     CHECK
----------------------------------------------------------------------
artifact.capture system     all                       no       no
... (44+ rows) ...
```

`SUDO` and `CHECK` columns. The only complete view of what's
actually registered. Should be the source of truth for
`LLM_GUIDE.md`'s action list.

---

## ★ `--artifacts-dir` produces a clean run bundle

```
$ mooncake apply --artifacts-dir /tmp/artifacts --capture-full-output
$ ls /tmp/artifacts/runs/20260515-182225-ff1bf0/
events.jsonl    JSONL event stream (richest)
plan.json       full plan with input_files_hash for audit
facts.json      point-in-time facts snapshot (good for repro)
stdout.log      "[step-NNNN] line" prefixed captured output
stderr.log      same shape for stderr
```

Run-bundle directory naming `<timestamp>-<hash>` is sortable and unique.
The `stdout.log` format with `[step-NNNN] line` prefix is *exactly*
what the default-renderer is missing per #5 — that hint should
flow back into the text formatter. This artifact directory is what
fleet replay / forensic debugging will be built on; keep it.

---

## ★ `mooncake history` — clean run audit trail

```
$ mooncake history list
#3   2026-05-15 17:05 UTC  config=config.yml  changed=3  ok=0  failed=0  0.0s
#2   2026-05-15 17:05 UTC  config=config.yml  changed=3  ok=0  failed=0  0.0s
#1   2026-05-15 17:05 UTC  config=config.yml  changed=3  ok=0  failed=0  0.0s

$ mooncake history list --format json
[ ... newest-first array of run summaries ... ]
```

Stored in `~/.mooncake/runs.jsonl` (append-only). Solid foundation
for fleet-level run dashboards.

---

## ★ `vars.load` and `--vars` use last-write-wins layering

```
# base.yml:     app: web, port: 8080, shared: from-base
# override.yml: port: 9090, extra: from-override

$ mooncake apply --vars base.yml --vars override.yml
result: app=web port=9090 shared=from-base extra=from-override
```

Both forms produce identical results:
```yaml
- vars.load: base.yml
- vars.load: override.yml
```
```
$ mooncake apply -v base.yml -v override.yml
```

Last-load-wins, missing keys preserve earlier values. Same semantics
as Ansible's `--extra-vars` chain. Clean.

---

## ★ `import:` handles circular imports + template-resolved paths

```
# a.yml: imports b.yml; b.yml: imports a.yml
$ mooncake apply -c a.yml
include cycle detected: /work/a.yml
Chain: /work/a.yml:1 -> /work/b.yml:1

# self.yml: imports itself
$ mooncake apply -c self.yml
include cycle detected: /work/self.yml
Chain: /work/self.yml:1

# dyn.yml: import: "{{ module }}.yml" with module=nope
$ mooncake apply -c dyn.yml
failed to read included config "/work/nope.yml": ... no such file or directory
```

Three positives:
- Cycle detection works (both 2-step and self-loop)
- The `Chain:` trace shows the import path with file:line
- `import: "{{ module }}.yml"` template-substitutes before resolving
  the path — dynamic imports work

Don't break this. The Chain output is the right shape for debugging
include trees.

---

## ★ Multi-file configs / `import:` / `vars.load` work cleanly

```yaml
# mooncake.yml
- vars.load: vars/main.yml
- import: tasks/inner.yml
```

Vars propagate into included files. Paths in `import:` resolve
relative to the including file. The "path resolution in presets"
story in `LLM_GUIDE.md` works as advertised.

---

## ★ `assert: file_sha256:` works correctly — reuse for `file.download`

```
✗ assert sha256 (intentionally wrong)
  assertion failed (file_sha256): expected sha256:0000…,
  got sha256:0f8f… (file: /tmp/assert-test.txt)
```

Clean error, correct semantics, file path included. **This is the
same verification path that `file.download: sha256:` should be using**
— see [`silent-success-bugs.md` #14](./silent-success-bugs.md#14).
The verification logic is solid; only the wiring in `file.download`
is broken.

---

## ★ `os.user` works cleanly

```
$ mooncake step "os.user: { name: alice, state: present }"
{"changed": true, "action": "os.user", "duration_ms": 68}
$ id alice
uid=1001(alice) gid=1001(alice) groups=1001(alice)
```

Properly creates with default group. No surprises. Don't break this.

---

## ★ `mooncake step` for `shell:` — clean agent primitive

```
$ mooncake step "shell: echo hi"
{
  "changed": true,
  "action": "shell",
  "stdout": "hi\n",
  "duration_ms": 1
}
```

`stdout`/`stderr`/`changed`/`duration_ms`/`error` all surfaced.
Almost a clean agent-primitive surface — only blemish is the
truncated structured-data return for typed actions (see
[`silent-success-bugs.md` #22](./silent-success-bugs.md#22)).

---

## ★ `pkg:` auto-detects apt/apk/etc. and is idempotent

```
=== ubuntu (apt) ===     ~ install jq     RECAP changed=1  failed=0
=== alpine (apk) ===     ~ install jq     RECAP changed=1  failed=0
=== debian:slim ===      ~ install jq     RECAP changed=1  failed=0
```

Run 2 on alpine: `ok=1 changed=0 skipped=0 failed=0`. Correctly
idempotent.

This is what `as_user: root` should be doing under the hood (see
[`cli-and-friction.md` #1](./cli-and-friction.md#1)).

---

## ★ `mooncake agentd` HTTP daemon — production quality

```
$ mooncake agentd --bind 127.0.0.1:7878 --no-mdns
{"time":"...","level":"INFO","msg":"agentd listening","socket":"/tmp/.../agentd.sock","bind":"127.0.0.1:7878","state_dir":"/root/.local/state/mooncake/agentd","token_file":"/root/.config/mooncake/agentd.token",...}

$ TOK=$(cat /root/.config/mooncake/agentd.token)
$ curl -H "Authorization: Bearer $TOK" http://127.0.0.1:7878/v1/health
{"status": "ok"}
$ curl -H "Authorization: Bearer $TOK" http://127.0.0.1:7878/v1/facts | head
{"apk_available": false, "apt_available": true, ...}
```

What's great:
- Bearer-token auth from auto-generated token file
- Structured JSON request logs with stable `request_id` per request:
  `{time, level: "INFO", msg: "http", request_id, method, path, status, bytes, duration_ms}`
- mDNS advertisement, configurable (--no-mdns)
- TCP bind, Unix socket, both, or socket-only
- `--system` mode for proper /run/mooncake / /var/lib/mooncake paths
- Endpoints: `/v1/health`, `/v1/facts`, `/v1/metrics` (and presumably the run/plan endpoints)

This is the substrate for `mooncake fleet` to talk to peers. Solid.

---

## ★ `mooncake fleet` actionable error messages

Without peers configured:
```
$ mooncake fleet status
fleet status: no peers configured. Edit /root/.config/mooncake/peers.toml or run `mooncake fleet bootstrap` / `mooncake fleet pair`.

$ mooncake fleet discover
no peers configured in peers.toml; no usable hosts in ~/.ssh/config
hint: `mooncake fleet bootstrap <user@host>` to add the first peer
```

Every "empty" state has an actionable next step. Matches the
`mooncake doctor` quality bar.

---

## ★ `observe.*` family — typed observation with consistent schema

9 actions: `observe.{cpu,disk,gpu,http,logs,memory,port,process,service}`.

Consistent result shape:
```json
{
  "as_of": "<timestamp>",
  "found": true,
  "value": { ...action-specific structured data... },
  "failed": false,
  "status": "ok"
}
```

`observe.port`:  `{host, local_addr, open: bool, port, protocol}`
`observe.process`: `{args, pid, pids, running}`
`observe.http`: `{latency_ms, method, reachable, status_code, url}`
`observe.cpu`: `{cores, idle_pct, load_15m, load_1m, load_5m, usage_pct}`
`observe.disk`: `{free_bytes, inodes_total, inodes_used, path, total_bytes, used_bytes}`
`observe.memory`: `{available_bytes, free_bytes, swap_total_bytes, swap_used_bytes, total_bytes, used_bytes}`
`observe.logs`: `{identifier, lines_read, matches: [{count, pattern, sample_lines}], source, truncated, window}`
`observe.gpu`: requires nvidia-smi; clean error if missing

This is the typed-observation primitive spec-59 promised. Live, fast,
schema-stable. The agent-loop read-side that the rest of the action
surface needs.

`observe.logs` deserves special note — `patterns:` takes a list of
regex strings, returns per-pattern `{count, sample_lines}`, includes
a `window` field for time-bounding. Multi-pattern in one call avoids
N round trips for "did any of these errors happen?" probes.

---

## ★ `text.line` — properly idempotent

```yaml
- text.line: { path: /tmp/file, line: "baz=3", state: present }
```

Run 2: `changed: false` — line already present, no duplicate
appended. Gold standard for "ensure line exists" semantics; other
text.* actions should mirror this idempotency story (see #47).

---

## ★ `on_change:` hooks fire only on real changes — clean reactive triggers

```yaml
- file.write:
    path: /tmp/oc-target.txt
    content: "v1\n"
  on_change:
    - file.write: { path: /tmp/oc-reacted.txt, content: "fired\n" }
    - shell: echo reacted >> /tmp/oc-shell.log
```

Run 1 (parent changes):
```
~ write a file (will change)
~ react to change
~ also reacts
RECAP  changed=3
```

Run 2 (parent no-op):
```
✓ write a file (will change)
- react to change [on_change: parent step-0001 did not change]
- also reacts [on_change: parent step-0001 did not change]
RECAP  ok=1  changed=0  skipped=2
```

Beautifully clean: parent's `changed=false` → children skipped with
descriptive reason. The exact reactive-triggers semantics the spec
promised. Keep.

---

## ★ `file.copy` properly idempotent (content + mode aware)

```
$ echo "src" > /tmp/src.txt
$ mooncake step "file.copy: { src: /tmp/src.txt, dest: /tmp/dst.txt, mode: \"0644\" }"
{"changed": true}
$ mooncake step "file.copy: { src: /tmp/src.txt, dest: /tmp/dst.txt, mode: \"0644\" }"
{"changed": false}    ← no-op
$ echo "src2" > /tmp/src.txt
$ mooncake step "file.copy: { src: /tmp/src.txt, dest: /tmp/dst.txt, mode: \"0644\" }"
{"changed": true}     ← detects content change
```

Properly compares content + mode. Don't regress.

---

## ★ `pkg.hold` / `pkg.upgrade` / `pkg.repo` — typed package management

```
$ mooncake step "pkg.hold: { name: bash, state: held }"
{"changed": true, "manager": "apt", "holding": ["bash"], "unholding": null, "targets": ["bash"]}

$ mooncake step "pkg.hold: { name: bash, state: held }"   # run 2
{"changed": false, "manager": "apt", "holding": null, "unholding": null, "targets": ["bash"]}

$ mooncake step --dry-run "pkg.upgrade: { names: [bash] }"
{"changed": false, "manager": "apt", "attempted": ["bash"], "autoremove": false}

$ mooncake step "pkg.repo: { name: docker, apt: {...}, state: absent }"
{"changed": false, "operation": "noop", "name": "docker", "manager": "apt"}
```

All three:
- Idempotent (`changed: false` on no-op)
- Return distinct `holding/unholding/attempted/operation` signals (better than just a bool)
- Honest `manager: "apt"` so callers know which path ran

Same `operation:` signal pattern as `os.group` — solid convention,
extend to all stateful actions.

---

## ★ `os.cron` / `os.sysctl` / `os.ssh_key` — clean file-based system management

```
$ mooncake step "os.cron: { name: backup, schedule: \"0 3 * * *\", command: /usr/local/bin/backup.sh, user: root, state: present }"
{"changed": true, "operation": "create", "name": "backup", "path": "/etc/cron.d/backup"}

$ mooncake step "os.cron: ..."   # run 2
{"changed": false, "operation": "noop", "name": "backup", "path": "/etc/cron.d/backup"}

$ mooncake step "os.sysctl: { name: vm.swappiness, value: 10, state: present }"
{"changed": true, "operation": "create", "name": "vm.swappiness", "path": "/etc/sysctl.d/99-mooncake.conf"}

$ mooncake step "os.ssh_key: { user: root, key: \"ssh-ed25519 ...\", state: present }"
{"changed": true, "added": 1, "removed": 0, "path": "/root/.ssh/authorized_keys"}
```

Each:
- Writes a file under the right system dir (`/etc/cron.d/`, `/etc/sysctl.d/`, `~/.ssh/authorized_keys`) — file-managed not crontab/sysctl-runtime-modified
- Surfaces `path:` so users know where the change landed
- Idempotent (`operation: noop` or `added: 0` on no-op)
- `added/removed` counters mirror pkg.hold's `holding/unholding` pattern — a per-action "what changed" enum is the convention

Worth promoting as a portable Mooncake convention: every stateful
action should surface `operation:` (or equivalent counts) so callers
can audit without parsing log strings.

---

## ★ `os.group` clean lifecycle with `operation:` signal

```json
{"action": "os.group", "changed": true, "name": "testers", "operation": "create"}
{"action": "os.group", "changed": false, "name": "testers", "operation": "noop"}
{"action": "os.group", "changed": true, "name": "testers", "operation": "remove"}
```

The `operation: create|noop|remove` field is a clear signal that
**other actions could adopt**. Right now most actions surface this
only through the `changed` boolean + log lines; an explicit
`operation:` enum is friendlier for agents and dashboards.

---

## ★ `when:` expression engine is comprehensive

Tested combinations that all evaluate correctly:
- `when: debug_mode` (simple bool)
- `when: count > 1` (numeric comparison)
- `when: role == "web"` (string equality)
- `when: debug_mode and count > 0` (AND)
- `when: role == "api" or count == 3` (OR)
- `when: not (role == "api")` (NOT with parens)
- `when: os == "linux"` (fact comparison)

Skipped step shows the failed expression: `- should NOT run [when: role == "api"]`.

---

## ★ `text.delete_range` validation error format — best in the CLI

```
{
  "error": "validation failed for text.delete_range action: start_anchor is required\n\nThe 'text.delete_range' action: Delete text between start and end anchor patterns in files\n\nRequired parameters:\n  - end_anchor: string\n  - path: string\n  - start_anchor: string ← MISSING\n\nOptional parameters:\n  - backup: boolean\n  - inclusive: boolean\n  - regex: boolean\n"
}
```

Lists required/optional params with types and `← MISSING` annotation
on the offending one. **Port this template to every action's
validation error path.** Currently only some actions hit this quality
bar.

---

## Don't regress

Anything in this file is a feature that works today. Every refactor
that touches these surfaces should add a test that protects the
behavior here.
