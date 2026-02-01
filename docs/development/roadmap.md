# Mooncake — Detailed Feature Checklist (Dependency-Ordered)

## 0) Foundations (must ship first)

### 0.1 Config model + schema
- [ ] Define canonical internal structs:
  - [ ] `RunConfig` (root)
  - [ ] `Step` (union: exactly one action key)
  - [ ] `Action` variants (shell/file/template/include/include_vars/vars/assert/...)
  - [ ] `Common` fields: `name`, `tags[]`, `when`, `become`, `become_user`, `env`, `cwd`, `register`, `timeout`, `retries`, `retry_delay`, `changed_when`, `failed_when`
- [ ] JSON Schema (or CUE → JSON Schema) for:
  - [ ] root document
  - [ ] step union (`oneOf`)
  - [ ] per-action payloads
  - [ ] expression strings (typed as string but tagged as `expr`)
- [ ] Schema constraints:
  - [ ] exactly-one action key enforcement
  - [ ] forbid unknown fields (strict mode)
  - [ ] validate `tags` format, `timeout` format, paths non-empty
- [ ] YAML source mapping:
  - [ ] parse with node position retention
  - [ ] map validation errors to `file:line:col`
  - [ ] include include-chain context: `A.yml -> B.yml -> C.yml:line:col`
- [ ] Template pre-validation:
  - [ ] validate pongo2 syntax for any field marked templatable
  - [ ] surface template line/col + originating yaml path
- [ ] CLI:
  - [ ] `mooncake validate --config ... --vars ...`
  - [ ] exit codes: `0 ok`, `2 validation error`, `3 runtime error`

### 0.2 Deterministic plan compiler
- [ ] Plan IR types:
  - [ ] `Plan` (ordered steps)
  - [ ] `PlanStep` fields:
    - [ ] `id` (stable)
    - [ ] `origin` (file, line, col, include stack)
    - [ ] `name_resolved` (post-template)
    - [ ] `tags_effective`
    - [ ] `when_expr_resolved` (string)
    - [ ] `become_effective`
    - [ ] `action` (compiled action payload)
    - [ ] `rendered` (optional: dry-run string)
- [ ] Include expansion:
  - [ ] recursive includes
  - [ ] relative path base = directory of including file
  - [ ] cycle detection with chain display
- [ ] Vars layering (deterministic precedence):
  - [ ] CLI `--vars` (highest)
  - [ ] include_vars
  - [ ] config-local vars
  - [ ] facts (read-only)
- [ ] Loop expansion:
  - [ ] `with_items`: expand to N steps; each has stable id suffix (`stepid[i]`)
  - [ ] `with_filetree`: deterministic ordering (lexicographic path)
  - [ ] loop vars: `item`, `index`, `first`, `last`
- [ ] Tag filtering at plan stage:
  - [ ] if `--tags` set, steps without matching tags are excluded (or marked skipped; pick one and stay consistent)
  - [ ] dry-run and run show identical step indices/ids
- [ ] CLI:
  - [ ] `mooncake plan --format json|yaml`
  - [ ] `--show-origins` prints file:line:col per step

### 0.3 Execution semantics (idempotency + check mode)
- [ ] Core step result model:
  - [ ] statuses: `ok`, `changed`, `skipped`, `failed`
  - [ ] timings: start/end/duration
  - [ ] stdout/stderr capture policy (bounded)
  - [ ] register payload (structured)
- [ ] `--dry-run` (check-mode):
  - [ ] identical plan + identical evaluators
  - [ ] actions implement `Plan()` and `Apply()`:
    - [ ] `Plan()` computes diff/intent, no side effects
    - [ ] `Apply()` executes changes
  - [ ] dry-run prints `would_change` and reason
- [ ] Expression engine:
  - [ ] `when` boolean expression
  - [ ] `changed_when` boolean expression based on action result
  - [ ] `failed_when` boolean expression based on action result
  - [ ] type rules: missing var handling, nulls, strings/bools/numbers, map/list indexing
- [ ] `shell` idempotency:
  - [ ] `creates: <path>` → skip if exists
  - [ ] `unless: <command>` → run only if unless returns non-zero
  - [ ] `changed_when` override (default: changed if rc==0; or default changed=true; choose explicit contract)
- [ ] Retries:
  - [ ] `retries: N`
  - [ ] `retry_delay: duration`
  - [ ] retry on failure only unless configured

### 0.4 Sudo / privilege escalation
- [ ] Input methods:
  - [ ] `--ask-become-pass` (prompt no-echo)
  - [ ] `--sudo-pass-file` (0600)
  - [ ] `SUDO_ASKPASS` support (optional)
- [ ] Security:
  - [ ] forbid plaintext `--sudo-pass` by default (or warn + require explicit insecure flag)
  - [ ] redact password in logs/events
- [ ] Become implementation:
  - [ ] Linux/macOS: `sudo -S` / askpass
  - [ ] Windows: explicit not supported or use `runas` (define scope)
- [ ] Per-step become:
  - [ ] `become: true|false`
  - [ ] `become_user` (optional; linux/mac only)

---

## 1) Core Engine UX / Observability

### 1.1 Event stream + presentation
- [ ] JSON event schema:
  - [ ] `run.started`, `plan.built`, `step.started`, `step.stdout`, `step.stderr`, `step.finished`, `run.finished`
- [ ] TUI consumes events only
- [ ] `--raw` consumes same events (no alternate code path)
- [ ] `--log-format json|text`
- [ ] `--log-level debug|info|warn|error`
- [ ] Output truncation rules:
  - [ ] cap stdout/stderr per step (bytes + lines)
  - [ ] store full output to artifacts dir optionally

### 1.2 Run artifacts
- [ ] `--artifacts-dir` default `.mooncake/runs/<timestamp>`
- [ ] Write:
  - [ ] `plan.json`
  - [ ] `facts.json`
  - [ ] `results.json` (per step)
  - [ ] `stdout.log` / `stderr.log` (optional)
  - [ ] `diff.json` (changed files)
- [ ] Deterministic naming + stable machine-readable format

---

## 2) File System Actions (detailed)

### 2.1 `file` action (expand into sub-modes)
Define `file:` as a structured union.

#### 2.1.1 Ensure directory
- [ ] `file: { state: directory, path, mode?, owner?, group? }`
- [ ] Idempotent:
  - [ ] create if missing
  - [ ] chmod/chown only if differs
- [ ] Dry-run shows which attributes would change
- [ ] Recursive option:
  - [ ] `recurse: true` for mode/owner/group on tree (explicit)

#### 2.1.2 Ensure file
- [ ] `file: { state: touch, path, mode?, owner?, group? }`
- [ ] Create empty if missing
- [ ] Update metadata only if differs

#### 2.1.3 Remove path
- [ ] `file: { state: absent, path, force?: bool }`
- [ ] Safety:
  - [ ] refuse empty path
  - [ ] refuse `/` unless `--i-accept-danger`
  - [ ] optional `allow_glob`
- [ ] Idempotent: ok if already absent

#### 2.1.4 Symlink
- [ ] `file: { state: link, src, dest, force?: bool }`
- [ ] Behavior:
  - [ ] create symlink if missing
  - [ ] if dest exists and not link:
    - [ ] fail unless `force: true` (then replace)
  - [ ] if link points elsewhere:
    - [ ] replace (counts as changed)
- [ ] Windows:
  - [ ] define behavior (requires admin or developer mode); if unsupported → explicit error

#### 2.1.5 Hardlink (optional)
- [ ] `file: { state: hardlink, src, dest, force?: bool }`

#### 2.1.6 Permissions-only / ownership-only operations
- [ ] `file: { state: perms, path, mode?, owner?, group?, recurse? }`

#### 2.1.7 Copy (separate `copy` action recommended)
If you keep in `file`, make it explicit:
- [ ] `copy: { src, dest, mode?, owner?, group?, force?, backup?, checksum? }`
- [ ] Preserve:
  - [ ] optionally preserve times
  - [ ] optionally preserve mode
- [ ] Large files: stream copy, atomic write temp + rename

#### 2.1.8 Sync (separate `sync` action)
- [ ] `sync: { src, dest, delete?: bool, exclude?: [], checksum?: bool }`
- [ ] Implementation:
  - [ ] prefer native `rsync` if present else Go copy-tree
  - [ ] deterministic ordering

### 2.2 `template` action
- [ ] `template: { src, dest, mode?, owner?, group?, backup? }`
- [ ] Features:
  - [ ] atomic write: render → temp file → diff → rename
  - [ ] change detection via content hash
  - [ ] optional `newline: lf|crlf`
- [ ] Template validation pre-run (Phase 0)

### 2.3 `unarchive` action
- [ ] `unarchive: { src, dest, strip_components?, creates?, mode? }`
- [ ] Supported:
  - [ ] `.tar`, `.tar.gz`, `.tgz`, `.zip`
- [ ] Idempotent:
  - [ ] if `creates` exists → skip
- [ ] Safety:
  - [ ] prevent path traversal (`../`) entries

### 2.4 `download` action
- [ ] `download: { url, dest, sha256?, mode?, owner?, group?, timeout_s?, retries?, headers? }`
- [ ] Features:
  - [ ] resume (optional)
  - [ ] ETag/If-Modified-Since (optional)
- [ ] Idempotent:
  - [ ] if sha256 matches → skip
  - [ ] else download to temp, verify, rename

---

## 3) Process Actions

### 3.1 `shell` action (structured)
- [ ] `shell: { cmd, interpreter?: "bash"|"sh"|"pwsh"|"cmd", env?, cwd?, stdin?, timeout_s?, capture?: bool }`
- [ ] Prefer `exec.Command` without shell when `argv` provided:
  - [ ] allow `command: { argv: ["git","clone",...], ... }` as safer alternative
- [ ] Quoting rules documented
- [ ] Exit code handling:
  - [ ] `rc` always captured
  - [ ] `failed_when` overrides rc logic
- [ ] Streaming output events optional:
  - [ ] emit stdout/stderr chunks for TUI

---

## 4) Service Management (`systemd` / launchd / Windows)

### 4.1 `systemd` action (Linux)
- [ ] `systemd: { name, state?: started|stopped|restarted|reloaded, enabled?: bool, daemon_reload?: bool }`
- [ ] Unit file management (optional but high value):
  - [ ] `systemd: { unit: { dest: "/etc/systemd/system/<name>.service", src_template?: ..., content?: ... } }`
  - [ ] `dropin:` support:
    - [ ] `dropin: { name: "10-mooncake.conf", content?, src_template? }`
    - [ ] writes to `/etc/systemd/system/<name>.service.d/<dropin>.conf`
- [ ] Environment directives via drop-in:
  - [ ] `Environment=K=V` lines
  - [ ] `EnvironmentFile=/etc/<...>` option
- [ ] Common directives checklist to support in templates/docs (not parsed by Mooncake, but validated as file ops):
  - [ ] `[Unit] After=`, `Wants=`, `Requires=`
  - [ ] `[Service] ExecStart=`, `WorkingDirectory=`, `User=`, `Group=`
  - [ ] `[Service] Environment=`, `EnvironmentFile=`
  - [ ] `[Service] Restart=`, `RestartSec=`, `TimeoutStartSec=`
  - [ ] `[Install] WantedBy=multi-user.target`
- [ ] Verification:
  - [ ] `systemctl is-active`, `is-enabled`, `status` capture
- [ ] Idempotent:
  - [ ] only `daemon-reload` when unit/dropin changed

### 4.2 `launchd` action (macOS) — optional but aligned with cross-platform
- [ ] `launchd: { label, plist_src_template|plist_content, state?: loaded|unloaded, enabled?: bool }`
- [ ] Paths:
  - [ ] user agents: `~/Library/LaunchAgents`
  - [ ] system daemons: `/Library/LaunchDaemons` (requires sudo)
- [ ] `launchctl bootstrap/bootout` support

### 4.3 Windows service action (later)
- [ ] `win_service: { name, state, start_mode }` (PowerShell `Set-Service`, `Start-Service`)

---

## 5) Assertions / Verification (first-class)

### 5.1 `assert` action (union)
- [ ] `assert: { command: "...", rc?: 0, stdout_contains?: "...", stdout_regex?: "...", timeout_s? }`
- [ ] `assert: { file: { path, exists?: bool, mode?: "0644", owner?: "...", group?: "...", sha256?: "..." } }`
- [ ] `assert: { http: { url, method?: GET|POST, status?: 200, jsonpath?: "...", equals?: any, timeout_s? } }`
- [ ] Result:
  - [ ] never “changed”
  - [ ] fail with precise mismatch diff

---

## 6) Facts (structured, immutable)

### 6.1 Facts collection
- [ ] `facts` run once per execution (cached)
- [ ] OS:
  - [ ] `os.name`, `os.version`, `kernel`, `arch`
- [ ] CPU:
  - [ ] model, cores, flags (AVX etc)
- [ ] Memory:
  - [ ] total, free, swap total/free
- [ ] Disk:
  - [ ] mounts, fs type, size/free
- [ ] Network:
  - [ ] interfaces, default route, DNS
- [ ] GPU (NVIDIA):
  - [ ] `gpu.count`, `gpu.model[]`, `gpu.driver_version`, `gpu.cuda_version` (from `nvidia-smi`)
- [ ] Toolchain probes (optional):
  - [ ] `docker.version`, `git.version`, `python.version`, `go.version`

### 6.2 CLI
- [ ] `mooncake facts --json`
- [ ] `--facts-json <path>` emit during run

---

## 7) ML Adoption Modules (after foundations)

### 7.1 `ollama` action/module
- [ ] `ollama: { state: present|absent, host?, models_dir?, service?: bool }`
- [ ] Install:
  - [ ] Linux/macOS installer strategy
- [ ] Service:
  - [ ] systemd drop-in for env vars (`OLLAMA_HOST`, `OLLAMA_MODELS`, `OLLAMA_DEBUG`)
- [ ] Model pull:
  - [ ] `ollama: { pull: ["llama3.1:8b", ...] }`
- [ ] Healthcheck:
  - [ ] `assert.http` to `/api/tags`
- [ ] Facts emitted:
  - [ ] `ollama.endpoint`, `ollama.models[]`

### 7.2 Container runtime
- [ ] `docker: { state: present, version_pin? }`
- [ ] `nvidia_container_toolkit: { state: present }`
- [ ] Optional: `apptainer: { state: present }`

### 7.3 Python env
- [ ] `uv: { state: present, version_pin?, cache_dir? }`
- [ ] `micromamba: { state: present, root_prefix?, envs_dir? }`
- [ ] `python_env: { backend: uv|micromamba, name, spec: pyproject|requirements|env_yml }`

---

## 8) Safety rails (needed before “yolo” ideas)

### 8.1 Dangerous ops gating
- [ ] Global allow/deny lists for:
  - [ ] `shell` commands (pattern match)
  - [ ] file deletes outside workspace
- [ ] Require explicit confirmation flags:
  - [ ] deleting `/`, modifying boot configs, driver reinstall, etc.
- [ ] Redaction:
  - [ ] mark vars as secret → never print in logs/events/artifacts

---

## 9) Detailed CLI checklist
- [ ] `mooncake run --config ... --vars ... --tags ... --dry-run`
- [ ] `mooncake plan --config ... --format json|yaml`
- [ ] `mooncake validate --config ...`
- [ ] `mooncake facts --json`
- [ ] `mooncake explain` (later)
- [ ] `mooncake doctor` (later)

---

## 10) Cross-platform policy (explicit scope)
- [ ] Define per-action availability matrix:
  - [ ] Linux/macOS/Windows support per action
- [ ] For unsupported:
  - [ ] fail at validation/plan-time with actionable message

