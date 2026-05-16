# Stream: agent — Manual Test Plan

Tests for the MCP server, the agent loop, `transaction:` rollback,
`!secret` typed refs, `on_change:` triggers, `try/catch/finally`. The
"Docker for AI agents" safety story has to be runnable here.

> If an LLM-driven agent invokes mooncake and it does the wrong
> thing, the safety guarantees come from this stream. Test the
> contract you're advertising.

## What to test

| Surface | What "correct" looks like |
|---|---|
| **MCP server (stdio)** | Initialize → tools/list → tools/call all work; notifications get no response (#25); errors use proper JSON-RPC codes |
| **MCP `run_plan` / `check_plan`** | Returns real counters per step; ULID run ID embedded; `requires:` permission summary; full step list (#54) |
| **MCP `get_facts` / `get_snapshot`** | Schemas stable across versions; PII-safe (hostnames OK, no auth tokens leaked) |
| **MCP `get_metrics`** | Honors `fields:` filter, `refresh: true` bypasses TTL cache |
| **transaction: success path** | All children commit; recap `changed=N`, no `reverted` count |
| **transaction: failure + rollback** | LIFO Reverse() on completed children; `on_rollback:` fires; recap shows `reverted=N`, text formatter prints `↺ Reverse:` markers (#45) |
| **try/catch/finally** | catch fires only on try failure; finally always runs; skipped-due-to-try-failed steps show `[try-block already failed]` reason |
| **on_change: hooks** | Parent changed → all children fire; parent ok → all children skip with `[on_change: parent step-N did not change]` |
| **!secret typed refs** | Auto-redacted in logs/history/artifacts; resolved from 3 providers (env, file, vault) |
| **Plan reproducibility** | `plan -o plan.json` + `apply --from-plan` is bit-exact; stale-plan refusal is default |

## Test environment recipe

```bash
CGO_ENABLED=0 go build -ldflags='-s -w' -o out/mooncake-static ./cmd

# MCP server speaks stdio
docker run --rm -i \
  -v $PWD/out/mooncake-static:/usr/local/bin/mooncake:ro \
  -v /tmp/agent-test:/work:rw \
  -w /work \
  ubuntu:24.04 mooncake mcp <<'JSONRPC'
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_facts","arguments":{}}}
JSONRPC
```

## Test scenarios

### 1. MCP protocol contract (15 min)

```bash
# 1a. initialize handshake
echo '{"jsonrpc":"2.0","id":1,"method":"initialize", ...}' | mooncake mcp
# Expected: { result: { protocolVersion: "2024-11-05", capabilities: {tools: {}}, serverInfo: {name: "mooncake", version: "0.2.0"} } }

# 1b. notifications/initialized — MUST NOT receive a response
# (per JSON-RPC 2.0; was bug #25 — verify still fixed)

# 1c. tools/list
# Expected tools: get_facts, get_snapshot, fact_query, get_metrics,
#                 run_plan, check_plan, run_step (if shipped)

# 1d. tools/call with invalid arguments
# Expected: error.code = -32602 (Invalid params) or -32000 (server error)

# 1e. tools/call with unknown tool name
# Expected: error.code = -32601 (Method not found)
```

Every step above is a regression target — protocol breakage will
break every agent client.

### 2. MCP run_plan / check_plan (10 min)

```yaml
# /work/test.yml
- log: { msg: greet }
- file.write: { path: /tmp/mcp-target.txt, content: hi }
- assert: { file: { path: /tmp/mcp-target.txt, exists: true } }
```

```bash
# Submit via MCP
echo '{...tools/call run_plan with config: /work/test.yml ...}' | mooncake mcp
# Expected result.content[0].text (JSON-stringified):
# {
#   "ok": 2, "changed": 1, "failed": 0, "skipped": 0,
#   "requires": {"filesystem_write": ["/tmp/mcp-target.txt"]},
#   "steps": [
#     {"name": "greet"},
#     {"name": "write", "changed": true},
#     {"name": "verify"}
#   ]
# }
```

Verify:
- Counters reflect reality (regression test for #54)
- `requires:` lists the actual paths the run will touch
- Every step in the playbook appears in `steps`
- File `/tmp/mcp-target.txt` actually exists after the call

### 3. check_plan must NOT execute side effects (5 min)

```bash
rm -f /tmp/mcp-target.txt
echo '{...tools/call check_plan ...}' | mooncake mcp
# Expected: no /tmp/mcp-target.txt on disk
# Known issue: assert against not-yet-applied state errors —
# check_plan should virtualize the file existence or skip assertions
```

### 4. transaction: success + rollback (10 min)

Pre-shipped examples cover both:

```bash
# Success
mooncake apply -c examples/transactions/file-create-trio.yml
# Verify all 3 files exist after; recap changed=3

# Failure + rollback
rm -f /tmp/mc-rollback-demo-*
mooncake apply -c examples/transactions/rollback-demo.yml
# Verify:
# - First 2 file.writes appear in text output with ~ changed
# - Third file.write fails (ENOTDIR on /dev/null/foo)
# - ↺ Reverse: markers print for the first 2 (LIFO)
# - on_rollback: notify fires; marker file written
# - Files a and b are GONE on disk; only marker remains
# - Recap shows: failed=1 reverted=2 (and the marker step changed=1)
```

### 5. try/catch/finally (10 min)

```bash
# try succeeds → catch skipped, finally runs
cat > /work/t1.yml <<EOF
- try:
    - log: { msg: try-step }
  catch:
    - log: { msg: should-NOT-fire }
  finally:
    - log: { msg: finally-fired }
EOF
mooncake apply -c /work/t1.yml --output-format json | jq '.data.message' | grep -v null
# Expected output: "try-step", "finally-fired"
# (NOT "should-NOT-fire")

# try fails → catch fires, finally runs
cat > /work/t2.yml <<EOF
- try:
    - log: { msg: try-before }
    - shell: exit 1
    - log: { msg: should-NOT-fire }
  catch:
    - log: { msg: caught-it }
  finally:
    - log: { msg: finally-fires }
EOF
mooncake apply -c /work/t2.yml
# Verify:
# - "should-NOT-fire" never prints
# - "[try-block already failed]" shows on the skipped step
# - "caught-it" + "finally-fires" both print
# - run exits non-zero (caught != recovered)
```

### 6. on_change: hooks (5 min)

```bash
cat > /work/oc.yml <<EOF
- file.write:
    path: /tmp/oc-target.txt
    content: "v1\n"
  on_change:
    - log: { msg: reacted }
EOF

# First run — parent changes, hook fires
mooncake apply -c /work/oc.yml
# Expected: ~ file.write; ~ reacted

# Second run — parent no-op, hook skipped
mooncake apply -c /work/oc.yml
# Expected: ✓ file.write; - reacted [on_change: parent step-0001 did not change]
```

### 7. Saved plan replay (10 min)

```bash
# Capture plan
mooncake plan -c /work/cfg.yml -o /work/plan.json

# Modify source
sed -i 's/v1/v2/' /work/cfg.yml

# Replay: should refuse (input changed)
mooncake apply --from-plan /work/plan.json
# Expected: "refusing to apply stale plan: plan input files have changed
#   since the plan was built (use --allow-stale to override)"

# Force replay
mooncake apply --from-plan /work/plan.json --allow-stale
# Expected: writes v1 (plan-time value), NOT v2 (current source)
```

This is the auditing story working. Don't break this.

## Tricks & tips

1. **MCP responses wrap content in `content[].text`**, JSON-stringified.
   You have to JSON-decode twice to inspect:
   ```bash
   ... | jq -r '.result.content[0].text' | jq .
   ```

2. **Notifications MUST not get responses.** If `notifications/initialized`
   triggers a `{"jsonrpc":""}` reply, that's a regression of #25.

3. **agent-loop testing needs the `run_plan` tool.** A real agent
   doesn't shell out; it round-trips JSON-RPC. Build a test client
   that scripts the conversation:
   ```bash
   # Sequence: initialize → list tools → check_plan → run_plan → get_facts
   ```
   Or use any MCP test harness from the Anthropic SDK ecosystem.

4. **For transaction tests, prefer the upstream examples.** They've
   been the canary for spec-30 since landing. If
   `examples/transactions/rollback-demo.yml` works, the executor's
   reverse path is healthy.

5. **Always verify `↺ Reverse:` markers in text output AND
   `reverted=N` in the recap.** Each catches a different bug —
   the marker shows the text formatter sees the event; the recap
   counter shows the event reached the metrics layer. Both must
   work for the auditing story to be honest.

6. **on_change skip messages are spec-23.** The exact format is:
   `- <step-name> [on_change: parent <parent-id> did not change]`.
   Brittle to format changes; pin it in golden tests.

7. **!secret testing**: don't shell-out a real secret. Use the
   `env:` provider with a sentinel value and grep the artifact
   bundle for the sentinel — should never appear (redaction is
   the contract).

## Common pitfalls

- **Nested `try:` is rejected at plan-time** (per #67). If your
  test uses `try: { try: ... }`, expect "nested try: blocks are
  not supported in v1" with a hint about `continue_on_error:`.

- **`transaction:` inside `try:` works**; `try:` inside
  `transaction:` is rejected with `set allow_irreversible: true to
  override`. Asymmetric on purpose.

- **`check_plan` runs assertions on not-yet-applied state.** Assertions
  in a plan that depend on prior file.write may fail during
  check_plan even though run_plan would succeed. Either filter
  asserts in check_plan or virtualize them.

- **MCP run_plan's `requires:` is a *forecast*, not a commitment.**
  The actual touched paths are in `events.jsonl` after execution.
  Treat `requires` as a permission-prompt input, not a final ACL.

## How to file findings

Findings affecting agent safety are highest priority. File under:
```
docs-working/analysis/findings-<DATE>/silent-success-bugs.md
```

Any finding where `failed: false` but the action did the wrong thing
is a P0 — this stream's whole pitch is "the agent can trust the
result". File loudly.

## Concrete priority targets

If you have one hour:

1. **MCP protocol contract** — initialize/list/call/notifications,
   verify every shape
2. **run_plan + on-disk verification** — the agent loop's primary
   read-then-write surface
3. **transaction rollback end-to-end** — recap counters, text
   markers, on_rollback firing, on-disk reverted state
4. **on_change second-run skip semantics** — regression for #45's
   companion path
5. **`--from-plan --allow-stale` semantics** — plan-time values
   win over current source (the audit story)
