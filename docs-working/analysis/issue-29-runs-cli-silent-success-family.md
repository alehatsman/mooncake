# Bug — `mooncake runs apply/get/follow` exit 0 on errors that should be non-zero (silent-success family)

**Surfaced:** 2026-05-15 during tick-10 of the autonomous test loop —
`mooncake runs` (local agentd client) sweep.

## TL;DR

Four exit-code bugs in one subcommand family, all members of the
recurring "silent success" pattern. The deepest:

```bash
$ mooncake runs apply -c /tmp/badyaml.yml   # planner-stage failure
Run 01KRPP85K5VWY4C78XY77FF086 submitted; streaming events...

$ echo $?
0
$ mooncake runs get 01KRPP85K5VWY4C78XY77FF086 | jq -r .status
failed
```

The run failed; the CLI reported success. A CI/script `if mooncake runs
apply ...; then echo "deployed"; fi` is broken.

## Repros

Each command run against a freshly-started per-user agentd
(`mooncake agentd` in another shell).

### A. `runs apply` exits 0 when run fails pre-streaming

```yaml
# /tmp/badyaml.yml
this is not: valid yaml: [
```

```
$ mooncake runs apply -c /tmp/badyaml.yml
Run 01KRPP85K5VWY4C78XY77FF086 submitted; streaming events...

$ echo $?
0
```

The agentd-side `record.json` says:

```json
{
  "id": "01KRPP85K5VWY4C78XY77FF086",
  "status": "failed",
  "error": "planner setup failed: failed to build plan: failed to read config: yaml: mapping values are not allowed in this context",
  ...
}
```

…and the SSE event stream `events.jsonl` is **empty** — no
`run.completed`, no `run.failed`, nothing. The client streams 0
events, exits 0, and never asks "did the run actually succeed?"

For comparison, a *step-stage* failure (after the executor opens
the stream) works correctly:

```
$ mooncake runs apply -c /tmp/runs-fail.yml   # shell step "exit 7"
Run 01KRPP8QJTF18MWQ40TW4BRQH6 submitted; streaming events...

▶ will-fail
✗ will-fail
  command failed with exit code 7

RECAP  ok=0  changed=0  skipped=0  failed=1  1313ms  ✗ command failed with exit code 7

$ echo $?
1
```

→ `events.jsonl` has `run.completed` with `success: false`. Client
reads it, exits 1.

The whole gap is: pre-executor failures emit no stream events, only a
`record.json` final state, and the client trusts the empty stream.

Same surface for `runs apply -c /missing/file.yml`:

```
$ mooncake runs apply -c /tmp/does-not-exist.yml
2026/05/15 22:46:34 submit run: HTTP 400: {
  "error": "plan_path_not_found",
  "message": "stat /tmp/does-not-exist.yml: no such file or directory"
}
$ echo $?
0    ← HTTP 400 → exit 0
```

The agentd correctly rejected with HTTP 400 — the client logged the
body and exited 0 anyway.

### B. `runs get` / `runs follow` exit 0 on all error paths

```
# missing positional arg
$ mooncake runs get
2026/05/15 22:46:07 usage: mooncake runs get <run_id>
$ echo $?
0

$ mooncake runs follow
2026/05/15 22:46:16 usage: mooncake runs follow <run_id>
$ echo $?
0
```

```
# malformed run id (length check fails)
$ mooncake runs get does-not-exist
{
  "error": "invalid_run_id",
  "message": "invalid run id length: 14"
}
$ echo $?
0
```

```
# well-formed but non-existent run id
$ mooncake runs get 01KRPP706S7VP5CXRCAVWAKJJX
{
  "error": "run_not_found",
  "message": "01KRPP706S7VP5CXRCAVWAKJJX"
}
$ echo $?
0

$ mooncake runs follow 01KRPP706S7VP5CXRCAVWAKJJX
$ echo $?
0    ← silent. No output. No error. Exit 0.
```

The follow case is the worst — *no output at all* for an unknown ID.
A CI script:

```bash
RUN=$(mooncake runs apply -c plan.yml | awk '/Run /{print $2}')
mooncake runs follow $RUN && echo "deployed"
```

…will print "deployed" even if `$RUN` is empty / wrong / typo'd.

### C. Help text doesn't show the required positional arg

```
$ mooncake runs get --help
NAME:
   mooncake runs get - Print the JSON record for one run

USAGE:
   mooncake runs get [command options]    ← no <run_id> shown

OPTIONS:
   --system    (default: false)
   --help, -h  show help
```

Same for `runs follow`. The required positional only surfaces in the
runtime usage message, which itself goes to stderr and exits 0 (see B).

### D. `runs list --format text` silently outputs JSON

```
$ mooncake runs list --help | grep format
   --format value, -f value  Output format (currently only 'json' is supported) (default: "json")
```

```
$ mooncake runs list --format text
{
  "runs": [
    ...
  ]
}
$ echo $?
0
```

The help text *says* only json is supported, but the command silently
falls back without warning when the user passes `text`. Either accept
`text` (and render a text table) or reject explicitly:

```
$ mooncake runs list --format text
Error: --format text not supported; use --format json
```

## Why this matters

`mooncake runs` is the daemon-side counterpart to `mooncake apply`.
Everything in the agentd direction goes through it — including (per
spec-58 and the fleet runbook) future GitOps-style "have I converged?"
checks. Silent-success here means:

1. **CI green when deploy fails**. The badyaml case is the clearest
   — operator's pre-flight syntax check fails on the agent side, CI
   sees green, broken plan goes to production.
2. **Scripted retry logic doesn't trigger**. `until mooncake runs apply
   -c plan.yml; do sleep 10; done` exits the loop on first try because
   exit was 0 even though the run failed.
3. **MCP / agent integrations consume wrong status**. The MCP server
   exposes runs to LLM clients; if the client only checks exit code
   (the normal pattern), it sees success when the run failed.

This is the same silent-success family already tracked in
`docs-working/analysis/findings-2026-05-15/silent-success-bugs.md` and
issues #21 (failed_when fabricates exit code), #26 (git.clone silent
noop), #27 (artifact.capture drops fields). The pattern recurs because
each command-line path makes its own decisions about "do we have
enough signal to declare failure?" without a single chokepoint.

## Fix

### For A — runs apply

After the SSE stream closes, fetch `GET /v1/runs/{id}` and check
`record.status`. If `failed` and no `run.completed` was emitted,
print the `record.error` and exit non-zero. Reference:
`cmd/runs.go:applyRunAction` (the function that opens the stream).

Or, on the agentd side: emit a synthetic `run.failed` event when the
planner fails so the client doesn't need a second round-trip. The
event payload would mirror `record.error`.

### For B — runs get / follow

In each command's `Action` func, when the HTTP response carries an
`error` field, exit non-zero:

```go
type errResp struct {
    Error   string `json:"error"`
    Message string `json:"message"`
}
if er.Error != "" {
    fmt.Fprintln(os.Stderr, body)
    os.Exit(1)
}
```

For the "no positional arg" case, the `usage:` message should also
exit 1 (today it uses `log.Println` which doesn't terminate).

### For C — help text

Add `ArgsUsage: "<run_id>"` to the `urfave/cli` command definitions
so the USAGE line in `--help` shows the positional.

### For D — runs list --format text

If `text` is documented as unsupported, reject it explicitly with
`return cli.Exit("--format text not supported", 2)`. Better: implement
a text-table renderer (the JSON shape is small — id, status, started,
duration would tabulate cleanly).

## Test gap

`cmd/runs_test.go` (if it exists) likely covers happy-path apply
+ list + get + follow. Add three negative tests:

```go
// pre-stream agentd failure → exit non-zero
result := runCLI(t, "runs", "apply", "-c", invalidYAMLFile)
require.NotEqual(t, 0, result.ExitCode)

// unknown run id → exit non-zero  
result = runCLI(t, "runs", "get", "01KRPP706S7VP5CXRCAVWAKJJX")
require.NotEqual(t, 0, result.ExitCode)

// missing positional → exit non-zero
result = runCLI(t, "runs", "get")
require.NotEqual(t, 0, result.ExitCode)
```

## Workaround

Operators have to write this dance instead of trusting `apply` exit:

```bash
RUN=$(mooncake runs apply -c plan.yml | awk '/Run /{print $2}')
STATUS=$(mooncake runs get "$RUN" | jq -r .status)
[[ "$STATUS" == "success" ]] || { echo "run $RUN failed"; exit 1; }
```

Brittle (parses stdout, races with `apply`'s background streaming,
needs jq). Operators familiar with the silent-success family will
already do this; everyone else gets bitten.
