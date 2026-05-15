# Spec 61: `observe.logs` — Typed log inspection

**Status:** Draft (depends on spec-59)
**Epic:** E9 Modern Action Surface — bucket E9.4 (observability extensions)
**Effort:** M-L (1–2 weeks; sized by edge cases, not core mechanism)
**Value:** High for *real-world* deploy validation. "Did the service
start cleanly?" almost always means "are there error lines in its
log for the last 30 seconds?" — and today that's a 5-line `shell` +
`grep` + `tail` recipe that doesn't compose with `when:`.

**Design principles:** [`action-design-principles.md`](../../action-design-principles.md) + [`non-goals.md`](../../non-goals.md)

---

## Problem

A common deploy assertion shape:

> Restart nginx. Wait 5 seconds. Check the last 30 seconds of
> `/var/log/nginx/error.log` for `crit|emerg|alert`. If found, fail
> the step and surface the matching lines.

Today: `shell` + `tail` + `grep`, output captured into `stdout`,
parsed with template filters. Untyped, OS-specific, no plan-mode
preview, no redaction-awareness.

This spec adds `observe.logs` as the typed cousin of `wait.command`
(spec-29 ✅) — but specialized for "read recent log content and
match patterns" rather than "wait until a command exits zero."

Distinct from `wait.command` because the *output* is the interesting
part: how many matches, which lines, in what window. `wait` returns
boolean; `observe` returns structured matches.

---

## Goals

- **G1** `observe.logs` handler that reads a log source (file path,
  journalctl unit, container name) within a time/line window and
  returns typed match data.
- **G2** Three source modes:
  - `path: /var/log/...` — tail a file from a byte/line offset.
  - `journal_unit: nginx.service` — read systemd journal via
    `journalctl -u <unit> --since <window>` shell-out (Linux only).
  - `container: nginx-1` — `docker logs --since <window>` /
    `podman logs` (autodetected from the runtime fact).
- **G3** Pattern matching: a list of regex patterns, return per-
  pattern match count + sample matched lines (capped).
- **G4** Window: `since: 30s` (relative duration) or
  `from_line: 500` (absolute line offset). Default: last 60 seconds.
- **G5** spec-22 ABI: empty Diff, nil Reverse, `Cost{Risk:1, Reversible:true}`,
  `Permissions{ReadOnly:true, RequiredBinaries:[]}` (journalctl/docker
  declared only when those source modes are used).
- **G6** Compose with `when:` via `as:` capture — the canonical
  pattern is `observe.logs → when "matches > 0" → assert/alert/restart`.

**Out of scope:**

- Continuous log tailing as a subscription (that's a different
  primitive; see spec-63 streaming-observers).
- Structured log parsing (JSON log lines, key extraction) — first
  cut returns raw matched lines; structured parsing is a follow-up
  if real users hit it.
- Log shipping / aggregation across the fleet — that's spec-64
  territory.
- Replacing `assert.match` (no such action exists today;
  `assert` + captured stdout is the current pattern).

---

## Design

### Per-handler shape

```go
type LogObservation struct {
    Source      string          `json:"source"`        // path | journal_unit | container
    Window      string          `json:"window"`        // human-readable: "60s", "from line 500"
    LinesRead   int             `json:"lines_read"`
    Truncated   bool            `json:"truncated"`     // hit the line/byte cap
    Matches     []LogMatchGroup `json:"matches,omitempty"`
}

type LogMatchGroup struct {
    Pattern    string   `json:"pattern"`
    Count      int      `json:"count"`
    SampleLines []string `json:"sample_lines,omitempty"` // up to 5, head of match list
}
```

### YAML

```yaml
- name: restart nginx
  os.service: { name: nginx, state: restarted }

- name: check post-restart log
  observe.logs:
    journal_unit: nginx.service
    since: 30s
    patterns:
      - 'crit|emerg|alert'
      - '\[error\]'
      - 'connection refused'
    sample_lines: 5
  as: nginx_log

- name: roll back if errors appeared
  assert:
    that: "nginx_log.value.matches | map(attribute='count') | sum == 0"
    msg: "nginx logged errors after restart: {{ nginx_log.value.matches }}"
```

### Caps and safety

Logs can be huge. Hard caps to prevent runaway reads:

- `max_bytes: 1048576` (1 MiB) default.
- `max_lines: 10000` default.
- When either cap is hit, `Truncated: true`. Pattern matches still
  return what was found in the read window; downstream `assert`s
  should treat `Truncated && Matches==0` as "inconclusive," not "clean."

### Plan-mode

Per spec-59 default: deferred (`"observation deferred to apply mode"`).
`--inspect-real` actually reads. For path sources, plan-mode reading
is safe-by-default and might be enabled per-handler — open question.

---

## Key files

| File | Change |
|---|---|
| `internal/actions/observe_logs/handler.go` | New. |
| `internal/actions/observe_logs/source_file.go` | Path source. |
| `internal/actions/observe_logs/source_journal.go` | Linux-only journal source. |
| `internal/actions/observe_logs/source_container.go` | Docker/Podman source (autodetect). |
| `internal/config/config.go` | New Step field. |
| `internal/register/register.go` | Registration. |
| `examples/observability/post-restart-log-check.yml` | End-to-end example. |

---

## Phases

1. Foundation + file source. Simplest; exercises pattern matching
   and the caps.
2. Journal source (Linux).
3. Container source (Docker/Podman autodetect).
4. Docs + schema regen.

---

## Acceptance criteria

- The post-restart example matches errors in a controlled log file
  and fails the run cleanly.
- Caps hold: a 100MB log file doesn't OOM; `Truncated: true` flags
  honestly.
- Build / vet / lint / test green.

---

## Open questions

1. **Pattern language: regex or glob-ish?** Regex is more powerful
   but harder to write at 3am. Lean regex with a `match_mode: glob`
   opt-in escape hatch if anyone asks.
2. **Multiline matches?** A stack trace spans many lines. v1 is
   line-by-line; group-by-stack is a future enhancement.
3. **Filesystem permissions:** logs in `/var/log/` often need root.
   `Permissions{Sudo: true}` only when source needs it; let the
   author know via the standard preflight error.
4. **Path source on Windows:** Windows event log isn't a file — out
   of scope for v1. Document the limitation; a real Windows shape
   is its own follow-up.

---

## Cross-references

- [`spec-59-typed-observability.md`](./spec-59-typed-observability.md) — parent.
- [`spec-29-wait-primitives.md`](../done/spec-29-wait-primitives.md) — `wait.command` is the boolean cousin.
- [`spec-23-framework-primitives.md`](../done/spec-23-framework-primitives.md) — `!secret` redaction applies to log content automatically.
