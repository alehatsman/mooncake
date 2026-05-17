# Proposal — `http.request`: kernel-honest HTTP action

**Status**: Partially shipped 2026-05-17 — Wave 1 (`cdec9bb3`), Wave 2 probe + reverse (`9f37718f`), Wave 3 `save_to` (`f897c37c`) and `expect_json_keys` (`71c90dae`). **Open**: `expect_json_schema` (full draft-07 file-path schema validation) — deferred pending a focused design conversation on the validator library + schema-loading rules. The narrow `expect_json_keys` (above) covers the most common assertion needs.
**Filed**: 2026-05-17 by aleh

---

## The user-facing ask, in one sentence

> Let me call an HTTP endpoint as a first-class step — with body, headers,
> auth, retries, response captured as facts — and have the kernel treat
> it the same way it treats `file` or `shell`: plan vs. apply, dry-run,
> diff, risk, idempotency.

## Why now

Mooncake has GET-shaped HTTP today through `file.download` (URL → file)
and `observe.http` / `wait.http` (probe & poll). None of them let an
agent or playbook **call** an HTTP API and consume the response as a
fact for downstream steps. Without that, every integration with a
webhook, JSON API, LLM endpoint, or notification service falls back to
`shell: curl` — losing dry-run, losing diff, losing risk classification,
losing secret redaction.

This is the action that turns Mooncake into agent infrastructure rather
than only config-management infrastructure (see brainstorm in session
log 2026-05-17). It is also the foundation `notify` and `llm` will sit
on top of as thin specializations — so getting the kernel treatment
right here pays off twice.

## Why this isn't `download` or `observe.http`

| Action          | Reads body? | Writes file? | Captures response as fact? | Allows POST/PUT/DELETE? |
|-----------------|:---:|:---:|:---:|:---:|
| `file.download` | ✓ (to file) | ✓ | — | — (GET hardcoded) |
| `observe.http`  | ✓ (capped sample) | — | ✓ (status + small body) | ✓ (read-shaped only) |
| `wait.http`     | ✓ | — | — (just status pass/fail) | ✓ (post-proposal-10) |
| **`http.request`** | ✓ (full) | optional (`save_to:`) | ✓ (full, JSON auto-parsed) | ✓ (all methods, idempotency-gated) |

`http.request` is the *general* primitive; the others are specializations
that exist for ergonomics and stay as-is.

## YAML surface

```yaml
- name: register webhook
  http.request:
    method: POST                          # default: GET
    url: "{{ api_base }}/hooks"
    headers:
      Content-Type: application/json
    auth:
      bearer: "{{ secrets.api_token }}"   # redacted in logs/diffs
    json: { event: "deploy", target: "{{ host }}" }

    # idempotency contract — exactly one required for POST/PATCH
    idempotency_key: "deploy-{{ run_id }}"
    # or:  creates_when: "facts.webhook_id == null"
    # or:  risk: high   (explicit ack)

    expect_status: [200, 201]
    retries: 3
    retry_on: [5xx, timeout]
    timeout: 10s
    register: hook                        # universal field

    # Wave 2 (deferred):
    # probe: { method: GET, url: ".../hooks/{{ run_id }}" }
    # reverse: { method: DELETE, url: ".../hooks/{{ hook.json.id }}" }

    # Wave 3 (deferred):
    # expect_json_schema: schemas/hook.json
    # save_to: /var/run/last-hook.json
```

## Body forms (one-of)

| Field | Shape | Sent as |
|---|---|---|
| `body:` | raw string (template-rendered) | bytes |
| `json:` | structured map/list | `application/json` + marshalled |
| `form:` | flat string map | `application/x-www-form-urlencoded` |
| `file:` | path to file | raw bytes from file |

`Content-Type` header takes precedence if the user sets it explicitly.

## Auth forms (one-of)

| Field | Effect |
|---|---|
| `auth.bearer: "tok"` | adds `Authorization: Bearer tok` |
| `auth.basic: {user, pass}` | adds `Authorization: Basic <b64>` |
| `auth.header: {name, value}` | adds arbitrary header |

All three flow through the secret-redaction filter for logs/diffs/events.

## Kernel concerns

| Concern | Behavior |
|---|---|
| **Plan mode (Wave 1)** | GET/HEAD/OPTIONS: execute, capture facts, `WouldChange=false`. POST/PUT/PATCH/DELETE: skip network, emit `"would <METHOD> <url> body=<hash>"`, `WouldChange=true`. |
| **Plan mode (Wave 2)** | If `probe:` set, run probe GET in plan to evaluate `creates_when` predicate. |
| **Idempotency (Wave 1)** | Method drives default. POST/PATCH require *exactly one* of: `idempotency_key:`, `creates_when:`, `risk: high`. Validate() rejects otherwise — fail loud rather than ship a footgun. |
| **Diff** | `method url` + sorted header keys (auth values redacted) + body hash. On apply: append `→ status_code, response.hash, duration_ms`. |
| **Reverse (Wave 2)** | Default not-reversible; set `ReverseData=nil` and audit "network call — no rollback." If `reverse:` block set, store rendered compensation request. |
| **Permissions** | `Network=true` always; `FilesystemWrite=[save_to]` if set; `Sudo=false`; `become:` rejected by Validate. |
| **Cost** | `CategoryNetwork`. Per-host duration EMA (same store as `download`). |
| **Risk** | GET/HEAD/OPTIONS=low, PUT/DELETE=medium, POST/PATCH=high. User override allowed. |
| **Secrets** | `auth.*` + standard auth headers (`Authorization`, `Cookie`, `X-*-Token`, `X-*-Key`) redacted in logs/diffs/events. Body redaction opt-in via `redact_body: true`. |
| **Facts** | `{status_code, headers, body, json, duration_ms, redirected_from}` exposed under `register:` name. `json` auto-parsed when Content-Type matches. |
| **Events** | `EventHTTPRequested{method, url_host, status, duration_ms, dry_run}` — host only, no body, no full URL (privacy). |
| **Become** | Rejected. HTTP-with-sudo is nonsense. |

## Why this is kernel-honest

1. **No universal-field addition** — reuses `register/retries/timeout/when/risk/tags`. Soft-cap holds at 36/40.
2. **Idempotency forced into the schema** — not advice; not docs; Validate refuses to ship a POST without one of three explicit safeguards. That's the moat.
3. **Plan ≠ apply is non-trivial** — Wave 1 short-circuits write methods; Wave 2 GET-probes give "would I actually do anything?" semantics no LangChain/n8n offers.
4. **Composes** — `wait_http` → `http.request` → `assert` → `register` already works; no new orchestration primitives.
5. **`download` survives** — stays as the URL→file+checksum specialization. `http.request` could subsume it later; not now.

## Implementation waves

| Wave | Scope | Status |
|---|---|---|
| **1** | Step field, handler, Validate (idempotency contract), Permissions, Run (plan/apply branch), body/auth/headers/method/retries/timeout/expect_status, register, redaction, events, tests | In flight 2026-05-17 |
| **2** | Plan-mode GET probes, reverse compensation | Deferred |
| **3** | `expect_json_schema`, `save_to` (writes response body, adds FilesystemWrite) | Deferred |
| **4** | docs/actions/http_request.md | Deferred |

## Open questions

- **Streaming responses (SSE, chunked).** `llm` will want this. Wave 1 buffers; revisit when `llm` lands.
- **Connection reuse across steps.** Each step gets a fresh `http.Client` today (mirrors `download`). Could add a per-run shared pool if chatty agent flows show measurable cost.
- **Cookie jar / session.** Punted. If wanted, `http.session` later.

## Effort estimate

Wave 1: ~800–1200 LOC handler + ~600 LOC tests. Single worktree, single PR.
Waves 2–4: sequenced follow-ups, each smaller than Wave 1.
