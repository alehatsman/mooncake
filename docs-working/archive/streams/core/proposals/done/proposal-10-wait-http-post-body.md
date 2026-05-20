# Request — `wait_http`: POST + JSON body support

**Status**: Shipped 2026-05-17 (commit `bb25104a`, merged `89362060`)
**Filed**: 2026-05-16 by aleh

---

## The user-facing ask, in one sentence

> Let me poll `POST /v1/embeddings` with a JSON body to confirm an
> embedding server is up, the same way I can poll a GET today.

## Why it matters today

`wait_http` is the right primitive for "wait until this service is
ready" — it polls an HTTP endpoint, returns success once the status
matches. It's currently GET-only.

For services with **no health endpoint**, the only way to confirm
readiness is to issue the actual request shape they handle. The
embedding service in `mcsearch/server.yml` (Qwen3-Embedding via vLLM)
exposes `POST /v1/embeddings` but no `GET /healthz`; readiness means
"can answer a real embedding request, not just bind a port."

## Concrete current usage

```yaml
# components/mcsearch/server.yml — today
- name: Wait for embedding endpoint to be ready
  shell: |
    for i in $(seq 1 60); do
      response=$(curl -s -o /dev/null -w "%{http_code}" \
        -X POST -H 'Content-Type: application/json' \
        -d '{"input":"ping","model":"{{ mcsearch_embed_model }}"}' \
        http://127.0.0.1:{{ mcsearch_remote_port }}/v1/embeddings 2>/dev/null)
      if [ "$response" = "200" ]; then
        echo "embedding endpoint ready after ${i}s"
        exit 0
      fi
      sleep 1
    done
    echo "endpoint did not come up in 60s"
    exit 1
  timeout: 90s
  changed_when: false
  tags: [mcsearch, server, verify]
```

What it should look like:

```yaml
- name: Wait for embedding endpoint to be ready
  wait_http:
    url: http://127.0.0.1:{{ mcsearch_remote_port }}/v1/embeddings
    method: POST
    headers:
      Content-Type: application/json
    body: |
      {"input": "ping", "model": "{{ mcsearch_embed_model }}"}
    status: 200
    timeout: 60s
    interval: 1s
  tags: [mcsearch, server, verify]
```

## Design notes

- **Body forms.** Two natural shapes:
  (a) `body:` as a raw string (most flexible — user formats the JSON
  themselves and templates work),
  (b) `body_json:` as a structured map that the action serializes.
  (a) is simpler and matches what `download:` and `pkg.repo` do for
  inline content.
- **Methods.** POST is the most common; PUT/PATCH/DELETE round out the
  set. GET stays default.
- **Status matching.** Already supports `status:` (single code) — extend
  to `statuses: [200, 201]` or a range expression if useful, but POST
  endpoints typically have a single happy code.
- **Idempotency / side-effects.** Worth a docs note: polling a POST
  *will* hit the handler each iteration. The embedding case above runs
  inference each poll. The user is expected to know what they're
  poking — but a `count: 1` (single shot, no retry) option could be
  useful for "I just want to validate POST works, once."

## Effort estimate

Small. The existing client already supports POST in Go's `net/http`;
plumbing method/headers/body through the schema is mostly schema +
serialization work.
