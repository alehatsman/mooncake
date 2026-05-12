# Epic: Agent Efficiency & Observable Runs

Brainstorm / working doc. Iterate here before moving to formal specs.

Core theme: make mooncake useful as a **system sense organ for AI agents** —
compact output, structured results, low token cost, and a direct agent interface.

---

## Epic 1: Observable Runs

Make every run self-documenting. Humans and agents can understand what happened
without reading raw logs.

### S1.1 Run Recap
Final line printed after every run:
```
RECAP  changed=12  ok=61  skipped=8  failed=0  duration=4m32s
```

### S1.2 Skip Reasons
When `when:` is false or `creates:` file exists, show reason instead of silence:
```
SKIP  Install OpenJDK (Debian/Ubuntu)  [when: apt_available]
SKIP  Install pyenv  [creates: ~/.pyenv/bin/pyenv]
```

### S1.3 Step Timing
Show duration only on steps that took >2s. Hidden otherwise to reduce noise.
```
✓  Update pacman mirrors  [47s]
✓  Install Python build deps  [12s]
```

### S1.4 Package Install Summary
After a package step, show which were new vs already present:
```
package  +neovim +fzf +ripgrep  (already present: git zsh tmux curl)
```
Package managers report this in their output — parse it rather than suppress it.

### S1.5 Changed vs OK Distinction
Visual separation between "ran and changed something" vs "ran, nothing needed":
- `✓` = ok, no change
- `~` or `+` = changed
- `✗` = failed
- `-` = skipped

---

## Epic 2: Compact Output Modes

Reduce noise for terminal users and AI agents reading stdout.

### S2.1 Quiet Mode (`--output quiet`)
Only errors and the recap line. Nothing else printed.

### S2.2 Agent Mode (`--output agent`)
JSONL stream — one JSON object per step event:
```jsonl
{"event":"step_start","name":"Install neovim","action":"package"}
{"event":"step_end","name":"Install neovim","changed":true,"duration_ms":3200}
{"event":"step_skip","name":"Install OpenJDK (Debian)","reason":"when: apt_available"}
{"event":"run_end","changed":3,"ok":58,"skipped":8,"failed":0,"duration_ms":274000}
```
Zero ANSI codes. Errors go to stderr as structured JSON.

### S2.3 Compact Mode
Default today is verbose. Compact mode collapses ok/skip to one dot per step,
expands on change or failure only:
```
......+......-......✗ Install fzf: exit code 1
```

### S2.4 Plan Diff Output
`mooncake plan --diff` — shows what would change vs current state, not just
what steps are in the plan.

---

## Epic 3: System Snapshot

`mooncake snapshot` — one command that dumps everything an agent needs to
understand a machine, in <500 tokens.

Goal: replace the "what is installed on this machine?" back-and-forth that
currently takes 5-10 tool calls and burns 2k+ tokens.

### S3.1 Snapshot Command
`mooncake snapshot` emits compact YAML/JSON of:
- os, distro, kernel, arch
- hardware: cpu model/cores, ram, disk usage
- installed dev tools + versions (see S3.3)
- running/failed services
- uptime, hostname, user

### S3.2 Token Budget
`--budget N` flag (default 400 tokens). Fields are prioritized; low-signal
fields (CPU flags, full disk list, GPU details) are dropped first to fit budget.
Output format optimized for token efficiency — abbreviations, no redundant keys.

### S3.3 Tool Inventory
Extend facts to detect and version-check installed dev tools:
- Languages: rust, node, python, java, go, ruby, php
- Runtimes: docker, podman, kubectl, helm
- Dev tools: git, nvim, tmux, fzf, ripgrep
- Shell: which shell, version

Cached with short TTL (facts already have caching infrastructure).

### S3.4 Service State
What systemd/launchd services are running vs stopped vs failed.
Compact summary: `running: sshd NetworkManager bluetooth  failed: thermald`

### S3.5 Snapshot Diff
`mooncake snapshot --diff <previous.json>` — what changed since last snapshot.
Useful for agents tracking drift between runs.

---

## Epic 4: Check Mode (Non-Destructive Audit)

Run a playbook without touching anything. See exactly what it would do and
what the current state is.

### S4.1 Check Mode (`--check`)
All actions report would-change/would-skip but make zero writes.
- Package action: queries installed state, reports what would be installed
- File action: diffs current vs desired content
- Shell action: shows command but does not run it

### S4.2 Check Output Format
Structured would-change list, parseable by agents. Same JSONL format as
agent mode (S2.2) but with `event: "would_change"` instead of actual changes.

### S4.3 Drift Detection
`mooncake check` against a saved desired-state plan — reports what has drifted.
Useful for periodic audits: "is this machine still in the expected state?"

### S4.4 Assert-Only Mode
`--assert-only` flag: skip all non-assert steps, run only assert steps.
Lets you use a playbook as a pure state-verification tool without side effects.

---

## Epic 5: Run History & Audit

Know what happened, when, and whether it worked.

### S5.1 Run Log
Append-only JSONL at `~/.mooncake/runs.jsonl`:
```jsonl
{"ts":"2026-05-12T10:30:00Z","config":"main.yml","tags":[],"changed":12,"ok":61,"skipped":8,"failed":0,"duration_ms":274000}
```

### S5.2 Last Run Summary
`mooncake last` prints the recap from the most recent run. Quick sanity check.

### S5.3 Step-Level Result Cache
Cache which steps changed/ok last time. Next run can annotate:
`✓  Install neovim  (unchanged since 2026-05-11)`

### S5.4 Agent Context Injection
When mooncake is invoked by an agent, it can prepend last-run summary to
agent context automatically. Agent knows what state the machine was in before
it starts making decisions.

---

## Epic 6: Agent-Native Interface

Mooncake as a tool AI agents call directly, not just run as a subprocess.

### S6.1 MCP Server Mode
`mooncake mcp` — expose mooncake as an MCP server with tools:
- `run_step` — execute a single inline step, return structured result
- `get_facts` — return system facts as JSON
- `get_snapshot` — return compact system snapshot (see E3)
- `check_plan` — dry-run a config, return would-change list
- `run_plan` — run a config file, stream JSONL events

This makes mooncake directly callable from Claude, Cursor, or any MCP client
without shell subprocess wrapping.

### S6.2 Single-Step Execution
`mooncake step 'package: {name: git, state: present}'` — run one inline step
from the command line, return JSON result. Useful for agents that want to
perform one action and check the result before proceeding.

### S6.3 Fact Query
```
mooncake facts --query rust.installed    → true
mooncake facts --query go.version        → 1.26.3
mooncake facts --query memory.free_mb    → 4096
```
Dot-path queries into the facts tree. Returns scalar values for easy scripting.

### S6.4 Explain Mode
`mooncake explain config.yml` — returns a token-compact natural language summary
of what a playbook does, without running it. For agent pre-flight: "before I
run this, what will it do?"

### S6.5 Structured Error Messages
Errors returned as structured JSON rather than prose strings:
```json
{"step": "Install pyenv", "action": "shell", "error": "exit code 1",
 "stdout": "...", "stderr": "curl: command not found",
 "hint": "curl is required", "suggested_fix": "package: {name: curl, state: present}"}
```
Agents can act on `suggested_fix` directly without parsing prose.

---

## Priority / Sequencing

| Epic | Value | Effort | Notes |
|------|-------|--------|-------|
| E1 Observable Runs | High | Low | Start here — immediate UX win |
| E2 Output Modes | High | Low | S2.2 agent mode is critical path for E6 |
| E3 Snapshot | High | Medium | Unique angle — nothing else does this |
| E6 Agent Interface | Very High | High | S6.1 MCP + S6.3 fact query are highest ROI |
| E4 Check Mode | Medium | High | Useful but not blocking |
| E5 Run History | Medium | Medium | Nice to have |

**Recommended order:** E1 → E2 (agent mode) → E3 → E6 (MCP) → E4 → E5

The E3 + E6 combination is the most differentiated thing in this space:
mooncake becomes the system sense organ for any AI agent running on a machine.

---

## Implementation Status

### Batch 1 (complete)
- spec-01 — S1.1 Run Recap ✅
- spec-02 — S1.2 Skip Reasons ✅
- spec-03 — S2.2 Agent JSONL Output ✅
- spec-04 — S3.1–S3.4 Snapshot Command ✅
- spec-05 — S6.3 Fact Query ✅

### Batch 2 (complete)
- spec-06 — S2.1 Quiet Mode ✅
- spec-07 — S1.3 Step Timing + S1.5 Changed/OK Distinction ✅
- spec-08 — S5.1 Run Log + S5.2 Last Run Summary ✅
- spec-09 — S6.5 Structured Error Messages ✅
- spec-10 — S6.1 MCP Server Mode ✅

### Preset Registry
- spec-11 — Community preset registry with remote fetch and multiple sources ✅

---

All 11 specs shipped and verified on Arch Linux (x1, 2026-05-12).

### Batch 3 (complete)
- spec-12 — S1.4 Package Install Summary ✅
- spec-13 — S6.2 Single-Step Execution ✅
- spec-14 — S3.5 Snapshot Diff ✅
- spec-15 — S4.1 Check Mode ✅

All 15 specs shipped and verified on Arch Linux (x1, 2026-05-12). Released as v0.4.0.

### Fixes (post-v0.4.0)
- fix: `version` variable wired so `-ldflags` injection takes effect (was hardcoded `0.2.0`)
- fix: `state: file` / `hardlink` / `perms` missing from JSON schema enum — caused any config mixing `package` + `file` steps to fail validation. Released as v0.4.1 binary on x1.

### Remaining from epic (not yet specced)
- S2.3 Compact Mode (dot-per-step progress)
- S2.4 Plan Diff (`mooncake plan --diff`)
- S4.2–S4.4 Check output format / drift detection / assert-only
- S5.3 Step-Level Result Cache
- S5.4 Agent Context Injection
- S6.4 Explain Mode (`mooncake explain config.yml`)
