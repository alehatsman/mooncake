# Spec 29: `wait.*` Primitives — port / http / file / command

**Status:** Draft
**Epic:** E9 Modern Action Surface — bucket E9.3
**Effort:** S (3 days)
**Value:** Medium-high. Agents need to chain steps reliably. "Start
the service, then wait for the port to open, then run the migration"
is the canonical orchestration. Today's polymorphic `wait` action
works but its shape is opaque; the per-domain split mirrors the
modern action surface convention and makes Diff/Cost natural.

**Design principles:** `docs-working/action-design-principles.md`

---

## Problem

Today's `wait` action handles all four cases inside one polymorphic
schema:

```yaml
- wait:
    port: 5432           # one of port / http / file / command
    host: localhost
    timeout: 60
```

Issues:
- Schema is implicitly disjoint (only one of `port`/`http`/`file`/
  `command` makes sense at a time); JSON Schema doesn't express that
  well without a deep `oneOf`.
- `Diff` / `Cost` / `Permissions` differ per kind (e.g. `wait.http`
  needs Network).
- Search/docs are easier with namespaced names.

The fix: split into four typed actions; deprecate the polymorphic
`wait` action.

---

## Goals

- **G1** `wait.port` — wait for a TCP port to accept connections.
- **G2** `wait.http` — wait for an HTTP endpoint to return a 2xx (or
  custom predicate).
- **G3** `wait.file` — wait for a file/dir path to exist, or to
  contain a specific string.
- **G4** `wait.command` — wait for a command to exit 0 (or custom
  predicate).
- **G5** All four implement spec-22 hooks.
- **G6** Keep today's `wait` action working for one release with a
  deprecation hint; remove in the next minor.

**Out of scope:**

- DNS resolution as a wait kind (`wait.dns`) — defer.
- Wait-for-multiple-of (any/all of N conditions) — orchestrate via
  `for_each` over multiple wait steps.

---

## Design

### Common shape

All four share:

```yaml
timeout: 60s               # default
poll_interval: 1s          # default
retry: { attempts, delay, backoff }  # universal retry block (spec-21)
```

### `wait.port`

```yaml
- wait.port:
    host: localhost        # default localhost
    port: 5432
    timeout: 30s
```

Implementation: `net.DialTimeout("tcp", host:port, poll_interval)`
loop until success or `timeout` elapsed.

### `wait.http`

```yaml
- wait.http:
    url: http://localhost:8080/health
    status:
      - 200
      - 204                # accepted statuses
    body_contains: "ok"    # optional
    headers:
      Authorization: !secret env:API_TOKEN
    timeout: 60s
```

Implementation: HTTP GET (configurable to POST/HEAD); check status
+ optional body substring + optional JSON path predicate.

### `wait.file`

```yaml
- wait.file:
    path: /var/run/myapp.pid
    contains: "READY"      # optional substring
    timeout: 30s
```

### `wait.command`

```yaml
- wait.command:
    cmd: "pg_isready -U postgres"
    timeout: 60s
    expect_exit: 0
```

---

## Cross-cutting

- **Permissions:** `wait.http` declares `Network: true`. `wait.port`
  declares `Network: true` iff `host != localhost`. `wait.file`
  declares no permissions. `wait.command` declares `RequiredBinaries`
  if the command's argv[0] is known.
- **Cost:** all `Risk: 1` (read-only). `Resources: 1`.
- **Diff:** waits never "change" state — they assert eventual state.
  Diff is `{Operation: noop}`.
- **Reverse:** trivially no-op (no side effect to undo).

---

## Key files

| File | Change |
|---|---|
| `internal/actions/wait_port/`, `wait_http/`, `wait_file/`, `wait_command/` | New handlers. |
| `internal/actions/wait/handler.go` | Existing wait action: print deprecation warning on use; route to the new actions internally for one release. |
| `internal/config/config.go` | New Step fields. |
| `internal/register/register.go` | Register four. |
| `internal/config/schema.json` etc. | Regenerate. |

---

## Tasks (phased)

1. **Phase 1** — Four new actions, each independent. ~1 day each.
2. **Phase 2** — Deprecation path on existing `wait` action.
3. **Phase 3** — spec-22 hooks (trivial for waits).
4. **Phase 4** — Docs + examples.

---

## Acceptance criteria

- `wait.port` succeeds when the port opens within `timeout`; fails
  with a clear error after `timeout` elapses.
- `wait.http` with `body_contains: "ok"` polls until the substring
  appears or times out.
- `wait.file` returns success the moment the file appears.
- `wait.command` returns success the first time the command exits 0.
- All four trivially pass spec-22 hooks (Diff: noop, Reverse: nil).
- Build / vet / lint / test green.

---

## Open questions

1. **Should `wait.http` support a JMESPath predicate on JSON body?**
   Probably yes — common need. Add `json_predicate: "status == 'ready'"`.
2. **Default timeout — 30s, 60s, or 5m?** Lean 60s; agents in CI
   typically need <60s reactions.
3. **`wait.port` over UDP?** Tempting; defer until asked.
4. **Should waits emit a progress event every poll, or stay silent
   until done?** Silent default; `--verbose` opt-in.
