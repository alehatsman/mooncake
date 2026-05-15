# Spec 59: Typed Observability — `observe.*` Action Family

**Status:** 🟢 **Seed + extensions shipped.** Phases 1–3 + 5 complete.
All four seed handlers landed (`observe.port` `27d61a7`,
`observe.process` / `observe.service` / `observe.http` `b26acbf`).
Spec-60 (cpu/memory/disk), spec-62 (gpu), spec-61 (logs), and
spec-64 (`fleet observe` cross-peer fan-out) all shipped on top.
**Nine typed observers** now on master: port, process, http,
service, cpu, memory, disk, gpu, logs — covering every observation
shape named in the issue-8/10/11 analysis docs. The agent loop
(observe → reason → act) closes end-to-end via spec-37 `as:` capture
+ `when:` branching, exercised by four examples in
`examples/observability/`. Phase 5 doc page lives at
`docs-next/guide/config/observability.md`. Only Phase 6
(`--inspect-real` plan-mode opt-in CLI flag) remains.
**Epic:** E9 Modern Action Surface — bucket E9.4
**Effort:** M (4 seed handlers, ~2–3 weeks)
**Value:** **Highest unbuilt strategic bet.** Mutation has a typed ABI
(spec-22 ✅); observation doesn't. Today's only options are `shell` +
parse free-form output (fragile, untyped, no redaction, no Diff) or
the static facts collector (whole-box, not per-step). Closing this
gap is the substrate three drafted features (`spec-58` drift,
`fleet explain`, workload placement) all already lean on.

**Design principles:** `docs-working/action-design-principles.md` + `docs-working/non-goals.md`

**Analysis trail:**
- [`analysis/issue-8-changegraph-analysis.md`](../../analysis/issue-8-changegraph-analysis.md) §7 — names `observe.*` as the load-bearing new primitive.
- [`clustermanagement/agentic-interface-brainstorm.md`](../../clustermanagement/agentic-interface-brainstorm.md) §1 — calls it strategic bet #1: *"It's the missing half of the ABI — mutation is solved, observation isn't. It unblocks almost every other ambitious idea on this list."*
- [`clustermanagement/issue-10-analysis.md`](../../clustermanagement/issue-10-analysis.md) §3 — calls this out as missing-from-issue-#11.
- [`clustermanagement/issue-11-analysis.md`](../../clustermanagement/issue-11-analysis.md) §11/§14 — `explain` and `placement` both depend on typed observation as substrate.

---

## Problem

Mooncake has a typed contract for *mutating* the world:

```go
type Handler interface {
    Metadata() ActionMetadata
    Validate(step *config.Step) error
    Run(ctx Context, step *config.Step) (Result, error)
}
// + spec-22's Diff / Reverse / Cost / Permissions sub-interfaces
```

Every mutation declares structure (`Diff`), reversibility (`Reverse`),
blast radius (`Cost`), and capability requirements (`Permissions`).
An agent (or a human) can reason about a plan before it runs.

**There is no equivalent for reading state.** Today, an author who
needs to know "is port 8080 listening?" or "did this endpoint return
200?" has three options:

1. **Shell out and parse** — `shell: "ss -tnlp | grep :8080"` and
   read `result.stdout`. Untyped, brittle, OS-specific, no redaction
   awareness, no Diff (Mooncake can't tell you what "would" be
   observed in plan mode).
2. **Static facts** — `internal/facts/` collects ~50 things at run
   start. Useful for "what distro is this?" but not for "is *this
   specific port* open *right now* during this step?". Facts are
   whole-box, not per-step.
3. **`wait.*`** (spec-29 ✅) — `wait.port` polls until a port opens
   or a timeout fires. It's the *polling* cousin of observation, not
   the single-shot read. `wait.port` is "block until true";
   `observe.port` would be "tell me now, and let the next step
   branch on the answer."

Every downstream feature that wants to *react to* current state hits
this gap:

- **Drift detection (spec-58, drafted)** wants periodic typed reads
  of `is service X running? what version? is the config file in
  shape Y?` against the last applied plan. Currently has to fall
  through to `inspect` mode (re-run the whole plan in dry mode) for
  every check, which is heavy and indirect.
- **`mooncake explain <resource>`** (issue #8 §4, issue #11 §11)
  wants typed before/after data per (host, resource). Diff payloads
  give the *change-side*; observation gives the *current-side*.
- **Workload placement** (issue #11 §14) wants typed queries against
  fleet state: "find peers where `observe.gpu.memory_bytes >= 24GB`."
  Today this has to ride on facts (whole-box) or shell parsing.
- **Agent loop in general** — every AI-agent runtime pattern in
  industry is `observe → reason → propose → execute → observe`. We
  ship the middle three; the bookends are missing the typed shape.

The fix is to add observation as a first-class action family with
the same ABI discipline as mutation, starting with the four
highest-leverage handlers and growing one at a time.

---

## Goals

- **G1** Define an `ObserveResult` standard payload shape (typed
  `Value` per handler + universal fields: `Found`, `AsOf`, `Error`).
- **G2** Ship four seed `observe.*` handlers covering the highest-
  value coverage area:
  - `observe.port` — TCP/UDP port state (open? listener? pid?)
  - `observe.process` — process state (running? pid? args? memory?)
  - `observe.http` — HTTP GET result (status, body sample, latency)
  - `observe.service` — systemd/launchd service state (active, enabled, sub-state)
- **G3** All four implement the full spec-22 ABI: `Permissions()`
  declares `ReadOnly: true` + any required binaries / network;
  `Diff()` returns empty (no mutation); `Reverse()` returns nil (no
  inverse needed); `Cost()` returns `{Risk: 1, Reversible: true}`.
- **G4** Plan mode is *safe by default* — `observe.*` actions return
  a synthetic "would observe" placeholder result. Opt-in flag
  `inspect.real: true` (or `--inspect-real` CLI) lets plan mode
  actually execute the read for richer preview. (Mirrors the
  spec-15/16 inspect-mode design.)
- **G5** Compose cleanly with spec-37 step-output capture: `as:` on
  an `observe.*` step exposes the typed result to downstream
  templates (`{{ ngx.value.open }}`, `{{ db.value.pid }}`).
- **G6** Compose with `when:` and `on_change:` so a typical pattern
  works without ceremony:

  ```yaml
  - observe.port: { host: localhost, port: 80 }
    as: nginx
  - os.service:
      name: nginx
      state: restarted
    when: "not nginx.value.open"
  ```

**Out of scope (each captured in its own spec, in order of leverage):**

- [`spec-60`](./spec-60-observe-system-resources.md) —
  `observe.cpu` / `observe.memory` / `observe.disk`. Cohesion
  question with `internal/metrics/`; share data via a refactored
  collector, separate surface.
- [`spec-61`](./spec-61-observe-logs.md) — `observe.logs`. Tailing,
  pattern matching, windowing. Three source modes (file / journald
  unit / container).
- [`spec-62`](./spec-62-observe-gpu.md) — `observe.gpu`. NVIDIA /
  Apple / best-effort AMD. Shares collector with spec-60.
- [`spec-63`](./spec-63-observe-streaming.md) — streaming /
  subscription mode. **Deliberately deferred**: the right home for
  "watch state and react" is the drift loop (spec-58), not the plan
  executor. Spec exists to record the boundary.
- [`spec-64`](../personal-fleet/spec-64-fleet-observe.md) —
  `fleet observe.<kind>`. Cross-peer fan-out; Stream 3 territory.
- `observe.diff` — the inverse of spec-22's `Diff`: given a file or
  config target, what's the delta against current state? Useful but
  big; needs its own spec when a consumer surfaces.

---

## Design

### `ObserveResult` standard payload

Shared shape across all observe handlers:

```go
// In internal/actions/observe.go (new shared package).
type ObserveResult struct {
    // Found is true if the resource being observed exists / is
    // reachable. The handler decides what "found" means:
    //   - observe.port: port is bound by some listener (regardless of
    //     whether we could identify the listener).
    //   - observe.process: at least one process matched the selector.
    //   - observe.http: the GET completed (any 1xx-5xx); transport
    //     failures (DNS, refused) set Found=false.
    //   - observe.service: the service exists in the init system.
    Found bool `json:"found"`

    // Value is the typed payload. Each handler defines its own struct
    // (see "Per-handler shapes" below). Stored as `any` in this
    // generic envelope; the schema for each handler's Value is
    // declared in its ActionMetadata.OutputSchema (a new field).
    Value any `json:"value,omitempty"`

    // AsOf is the timestamp the observation was taken. Set by the
    // handler, not by the caller; "as of when was this true?" is part
    // of the answer.
    AsOf time.Time `json:"as_of"`

    // Error carries a user-facing error message when Found=false
    // because the observation itself failed (DNS error, permission
    // denied, network unreachable). When Found=false because the
    // observed resource genuinely doesn't exist, Error is empty.
    Error string `json:"error,omitempty"`
}
```

This shape is what `as: <name>` exposes to downstream templates.
Authors write `{{ nginx.value.open }}`, not `{{ nginx.open }}`, so
the envelope is always visible and the typed payload stays nested.

### Per-handler shapes

Each handler declares its own `Value` struct. The starter set:

```go
// observe.port
type PortObservation struct {
    Open      bool   `json:"open"`              // listener bound to (host, port)
    Protocol  string `json:"protocol,omitempty"` // "tcp" | "udp" (default tcp)
    Listener  string `json:"listener,omitempty"` // process name if discoverable
    Pid       int    `json:"pid,omitempty"`     // 0 if unknown / not permitted
    LocalAddr string `json:"local_addr,omitempty"`
}

// observe.process
type ProcessObservation struct {
    Running   bool     `json:"running"`
    Pid       int      `json:"pid,omitempty"`
    Pids      []int    `json:"pids,omitempty"`     // when multiple match
    Args      []string `json:"args,omitempty"`     // argv of first match
    User      string   `json:"user,omitempty"`
    StartedAt string   `json:"started_at,omitempty"` // RFC3339
}

// observe.http
type HTTPObservation struct {
    StatusCode int               `json:"status_code"`
    Reachable  bool              `json:"reachable"` // transport-level
    LatencyMs  int64             `json:"latency_ms"`
    Headers    map[string]string `json:"headers,omitempty"` // selected, see Validate
    BodySample string            `json:"body_sample,omitempty"` // truncated to 2048 bytes
}

// observe.service
type ServiceObservation struct {
    Exists     bool   `json:"exists"`
    Active     bool   `json:"active"`     // currently running
    Enabled    bool   `json:"enabled"`    // starts at boot
    SubState   string `json:"sub_state,omitempty"` // systemd's sub-state
    Manager    string `json:"manager,omitempty"`   // "systemd" | "launchd" | "sysv"
}
```

### YAML shapes

```yaml
# Single observation — capture via spec-37 `as:` for downstream use
- observe.port:
    host: localhost
    port: 8080
    protocol: tcp           # default tcp
  as: app

- observe.process:
    name: nginx             # exact match against process name
    # or:
    # pattern: '^nginx:'    # regex against full argv
  as: ngx

- observe.http:
    url: https://example.internal/health
    timeout: 3s
    expect_status: 200      # optional — sets Found=false if mismatched
    capture_headers: [Server, X-Request-ID]
  as: health

- observe.service:
    name: nginx
  as: svc

# Condition + reactive pattern (closes the agent loop)
- name: ensure nginx is up
  os.service: { name: nginx, state: restarted }
  when: "not health.value.reachable or health.value.status_code != 200"
```

### Handler interface — reuse, don't fork

`observe.*` handlers implement the **existing** `Handler` interface
plus the spec-22 sub-interfaces. No new top-level interface.
Rationale (per non-goals.md "Borrow vocabulary, not implementation"):

- `Run()` performs the observation, returns a `Result` whose
  `OutputData` field carries the `ObserveResult`. The executor
  treats this as a normal `Result` for capture/`as:` purposes.
- `Permissions()` declares `Network` (observe.http) or
  `RequiredBinaries: [ss, ps, systemctl]` (observe.port,
  observe.process, observe.service) and the new bit `ReadOnly: true`.
  The `ReadOnly` bit lets policy gates (future) treat the entire
  family as "no mutation possible."
- `Diff()` returns `Diff{Resource: ..., Operation: "observe"}` with
  empty Before/After. Authors of `--diff structural` get a single
  line acknowledging the observation; no false sense of mutation.
- `Reverse()` returns `(nil, nil)` — there is nothing to undo.
- `Cost()` returns `{Risk: 1, Resources: 1, Bytes: 0, Reversible: true}`.

### Plan-mode behavior

By default, plan mode does **not** execute the observation. Each
handler returns a synthetic placeholder result with
`Found: false, Error: "observation deferred to apply mode"`. The
plan output's text formatter renders these as:

```
~ observe nginx port :80         (deferred to apply)
```

This matches the spec-15/16 doctrine: plan mode is structurally
predictive, side-effect-free. Network reads to arbitrary hosts in
plan mode would surprise people.

Opt-in for richer preview: a CLI flag `--inspect-real` (or per-step
`inspect: { real: true }`) tells the executor to actually run the
observation in plan mode. `observe.http` to a public health endpoint
is a sensible thing to do in plan mode if the author asks for it.

This dual behavior is the same pattern wait.* already uses (it's a
no-op in plan mode unless explicitly opted in).

### Composition contract

- **With spec-37 `as:`** — the captured value is the full `ObserveResult`
  envelope. Templates address as `{{ name.value.field }}`.
- **With `when:`** — standard predicate evaluation; `when: "ngx.value.open"`
  works without ceremony.
- **With `on_change:`** — observations can declare
  `changed_when: "value.open == false"` so the on_change trigger fires
  when the *observation* shifts state. (Note: `changed_when` is
  evaluated *after* the observation; "changed" here means "the predicate
  was true," not "the underlying state shifted since last run.")
- **With `wait.*`** — `wait.X` and `observe.X` share the predicate
  schemas where possible (e.g. `wait.http`'s `expect_status: 200`
  and `observe.http`'s `expect_status: 200` mean the same thing).
  Authors who internalize one schema pick up the other for free.

---

## Key files

| File | Change |
|---|---|
| `internal/actions/observe.go` | New shared package with `ObserveResult`, common helpers, `OutputSchema` metadata field. |
| `internal/actions/observe_port/handler.go` | New. Uses `net.Listen` probe + `/proc/net/tcp` (Linux) / `lsof` fallback for listener identification. |
| `internal/actions/observe_process/handler.go` | New. Reads `/proc` on Linux + `ps` fallback elsewhere. |
| `internal/actions/observe_http/handler.go` | New. `net/http` client with timeout, header capture, body truncation. |
| `internal/actions/observe_service/handler.go` | New. `systemctl is-active` / `launchctl print` per OS. |
| `internal/config/config.go` | Four new Step action fields: `ObservePort`, `ObserveProcess`, `ObserveHTTP`, `ObserveService`. Validation. |
| `internal/register/register.go` | Four new handler registrations. |
| `internal/schemagen/generator.go` | Schema entries for the four actions; regenerate `schema.json`. |
| `examples/observability/` | New directory. `check-app-health.yml`, `port-driven-restart.yml`, `service-state-snapshot.yml`. |

---

## Phases

1. **Phase 1 — Foundation + `observe.port`.** Land the
   `ObserveResult` shape, the `OutputSchema` metadata addition, and
   the first handler. Smallest blast radius, exercises the whole
   plumbing including spec-37 capture composition.
2. **Phase 2 — `observe.process` + `observe.service`.** Same shape,
   different OS-specific implementations. Wire systemd / launchd /
   sysv detection from existing `internal/facts/` plumbing.
3. **Phase 3 — `observe.http`.** Network handler. Permissions
   declare `Network`; preflight refuses if `Permissions.AllowNetwork=false`
   is ever introduced (forward-compat for the policy DSL).
4. **Phase 4 — Composition tests + examples.** End-to-end:
   `observe → when → mutate` patterns for nginx restart-on-port-down,
   service-state-driven config rewrite, http-health-driven action.
5. **Phase 5 — Docs.** New page `docs-next/guide/observability.md`.
   Update `docs-next/guide/actions.md` family table. Regenerate
   `schema.json`, run `make schema-check` + `make docs-check`.
6. **Phase 6 — `--inspect-real` flag.** Final phase: plan-mode opt-in
   that actually runs the observation. Independent of the per-handler
   work; can land later if Phase 4 closes the demo loop without it.

---

## Acceptance criteria

- `examples/observability/check-app-health.yml` runs an `observe.http`,
  captures via `as: health`, and conditionally restarts a service based
  on `health.value.status_code`. Apply succeeds; the captured value is
  in the run log.
- `mooncake plan` on the above example renders the observe step as
  `(deferred to apply)` and the conditional step as `(unknown — depends
  on observation)`. No network calls in plan mode.
- `mooncake plan --inspect-real` on the same example actually runs the
  observation and predicts the conditional correctly.
- All four handlers expose Diff/Cost/Permissions/Reverse per spec-22.
  `mooncake plan --format json` surfaces the (empty) Diff +
  `cost: { risk: 1, reversible: true }` per observe step.
- MCP `check_plan` / `run_plan` responses include the typed
  `ObserveResult` for each observe step that ran. The agent SDK can
  branch on `result.value.open` without parsing prose.
- Build / vet / lint / test green. Schema regenerated. Docs check green.

---

## Open questions

1. **Naming: `observe.*` vs `probe.*` vs `query.*`?** All three
   analysis docs use `observe`. systemd uses `probe` for hardware
   discovery; "query" implies a structured-data store. Stick with
   `observe`.
2. **Output schema declaration: where?** Two options:
   - **(a)** Extend `ActionMetadata` with an `OutputSchema` field
     populated from Go reflection on the handler's `Value` type.
     Pros: introspectable from CLI, MCP, docs. Cons: reflection
     plumbing.
   - **(b)** Document the shape in `docs-next/guide/observability.md`,
     no machine-readable schema.

   **Lean (a)** — it's the future input to a real-world MCP "what can
   I observe and what do I get back?" tool. But (a) can land in
   Phase 5 without blocking the seed handlers.
3. **`observe.port` Pid discovery without root?** On Linux,
   `/proc/net/tcp` shows the socket but mapping inode→pid requires
   walking `/proc/*/fd`, which needs to read other processes' files.
   Without root we'll often return `Pid: 0`. Acceptable — the field is
   `omitempty`. Document the limitation.
4. **Plan-mode `--inspect-real` scope:** is it global ("run all
   observations for real") or per-step (`inspect: { real: true }`)?
   Probably both — the per-step opt-in is the more useful default; a
   global flag is convenient for development. Both are cheap to add.
5. **Should `changed_when` default to "observation result differs from
   prior run"?** That would make every `observe.*` step into an
   implicit drift sensor. Tempting but couples this spec to spec-58.
   **Defer** — explicit `changed_when` for now; revisit once spec-58
   ships and we see real usage patterns.
6. **Redaction:** `observe.http` body and headers can contain
   secrets. Reuse the spec-23 §3 `!secret` redaction denylist? Yes —
   the executor's redactor already runs over `Result.OutputData`.
   Document that authors capturing sensitive HTTP responses should
   redact before `as:`-capture, or rely on automatic denylist matching.

---

## Cross-references

- [`spec-22-extended-handler-abi.md`](./spec-22-extended-handler-abi.md)
  — the typed-mutation ABI this spec mirrors. Every `observe.*`
  handler implements the four spec-22 sub-interfaces with the
  no-mutation specialization.
- [`spec-29-wait-primitives.md`](../done/spec-29-wait-primitives.md)
  — the polling cousin. Schemas align where applicable
  (`expect_status`, `host`/`port`/`url`/`timeout`).
- [`spec-37-step-output-capture.md`](../done/spec-37-step-output-capture.md)
  — the capture mechanism that makes `as: nginx` →
  `{{ nginx.value.open }}` work.
- [`spec-58-fleet-drift.md`](../personal-fleet/spec-58-fleet-drift.md)
  — drafted drift loop. First major consumer of `observe.*`; should
  use these handlers instead of re-running `inspect` for every check.
- [`../../analysis/issue-8-changegraph-analysis.md`](../../analysis/issue-8-changegraph-analysis.md)
  §7 — the "observe.* is the load-bearing new primitive" argument.
- [`../../clustermanagement/agentic-interface-brainstorm.md`](../../clustermanagement/agentic-interface-brainstorm.md)
  §1 — strategic-bet framing.
- [`../../non-goals.md`](../../non-goals.md) — observation must not
  become telemetry sprawl. No high-cardinality metric collection,
  no streaming subscriptions in v1, no policy DSL on top.
