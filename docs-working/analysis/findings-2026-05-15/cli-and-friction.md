# CLI Nits + Minimal-Container Friction

DX papercuts, error-message quality, and the "drop the binary into a
fresh docker container" story.

---

## #1 — `as_user: root` invokes `sudo` even when uid=0 — HIGH

**Repro**: apply any preset using `as_user: root` (the `jq` preset
does) in vanilla `ubuntu:24.04` or `alpine:3.21`:

```
▶ Install jq (Linux)
  Installing packages: jq
failed to install packages [jq]: exec: "sudo": executable file not found in $PATH
✗ Install jq (Linux)
```

**Why HIGH**: official base images don't ship sudo. Every preset
that hardcodes `as_user: root` (canonical pattern for system installs)
is unusable in CI containers, minimal images, and most Docker
quickstarts — the audience the "Docker for AI agents" framing
attracts.

**Fix**: when the current process is already uid=0, `as_user: root`
should short-circuit and run directly, not invoke `sudo`. Check is
`os.Geteuid() == 0`.

**Scope confirmation**: `pkg:` directly (without `as_user: root`)
works on ubuntu/alpine/debian with no sudo — proving the executor's
general "run-as-root" path is correct. The bug is localized to
the `as_user: root` handler.

---

## #21 — Static binary in vanilla `ubuntu:24.04` fails TLS without `ca-certificates` — LOW (packaging)

**Repro**: `mooncake apply` with any `file.download:` step in fresh
`ubuntu:24.04` with no ca-certificates:

```
failed to download: ... tls: failed to verify certificate: x509:
  certificate signed by unknown authority
```

Same minimal-image story as #1. A user following "Docker for AI
agents" and pulling `ubuntu:24.04` to test mooncake hits TLS errors
on first download.

**Options**:
- (a) Make the official `Dockerfile` (alpine 3.21 with
  ca-certificates) the prominent quickstart; drop docs suggesting
  "drop the binary into ubuntu:24.04".
- (b) Bundle Mozilla's CA list in the binary (rooted CA).
- (c) `mooncake doctor` adds a "ca-certificates missing" check.

---

## #11 — `snapshot --diff` only diffs hw + curated tool list — LOW (surface gap)

**Repro**:
```
$ mooncake snapshot --format json --save snap1.json
$ apt-get install -y jq
$ mooncake snapshot --diff snap1.json
hw:
  ~ ram_free_mb          25451 → 25454
```

New tool (`jq`) installed between snapshots does not surface — `tools:`
map only includes a curated allowlist (bash version in this case).

**Suggestion**: this is probably an `observe.*` story (spec-59), not
a snapshot fix per se. Worth noting as input to that spec.

---

## #12 — `metrics -q <key> --format json` silently ignores `--format` — LOW

**Repro**:
```
$ mooncake metrics --format json -q cpu_usage_pct -q memory_used_pct
cpu_usage_pct=3.08
memory_used_pct=17.41
```

`key=value` lines instead of JSON. `--fields` *does* honor `--format
json`. Either `-q` should respect `--format` too, or the help text
should call out that `-q` is text-only.

---

## #13 — Error-hint `suggested_step` emits invalid YAML — LOW

**Repro**: any failing shell command triggering the "is not installed"
heuristic. Example:

```json
{
  "step": "<[]interface",
  "action": "shell",
  "stderr": "bash: line 1: lt: command not found\n...",
  "hint": "bash: is not installed",
  "suggested_step": "package:\n  name: bash:\n  state: present"
}
```

Two bugs in one hint:
1. `package:` is not in the schema's vocabulary — the correct action
   is `pkg:`.
2. The package name comes out as `bash:` (trailing colon retained
   from stderr parsing).

Also, the hint fired on a bash error that wasn't about missing bash
— it was a cascading failure from #8's broken `for_each` substitution.
Separate issue, but it shows the heuristic also misclassifies errors.

---

## #19 — `--tags <typo>` silently runs only untagged steps — LOW (UX)

**Repro**:
```
$ mooncake apply --tags deplly   # typo of 'deploy'
... only the untagged steps run, with no warning ...
RECAP  ok=1 changed=0 skipped=3 failed=0
```

A typo silently degrades to "only untagged steps run". User sees
green (`failed=0`) but intended deploy steps never executed.

**Fix**: when no step matches any requested tags, exit nonzero with
`no steps matched tags: deplly. Did you mean: deploy?` (fuzzy
suggestion from the tag inventory).

---

## #42 — `wait.command` attempt counter off — LOW

**Repro**:
```
$ mooncake step "wait.command: { cmd: 'test -f /tmp/x', timeout: 1s, interval: 200ms }"
{"error": "wait.command timeout after 1s (1 attempts); last exit 1"}
```

"1 attempts" with `interval: 200ms` and `timeout: 1s` — should be ~5
attempts. Either interval parsing failed silently or counter is
off-by-(N-1). Low severity; the wait/timeout behavior is correct.

---

## #25 — MCP server replies to `notifications/initialized` with malformed `{"jsonrpc":""}` — LOW (protocol)

**Repro**:
```
> {"jsonrpc":"2.0","method":"notifications/initialized"}
< {"jsonrpc":""}
```

JSON-RPC 2.0 says notifications MUST NOT receive responses.
Empty-version `jsonrpc` is also invalid. Strict MCP clients could
refuse to talk further.

**Fix**: handle notifications without emitting any response.

---

## #26 — MCP server `run_plan` error has duplicated prefix — LOW (cosmetic)

```
> {"method":"tools/call","params":{"name":"run_plan","arguments":{"config":"/tmp/no-such-file.yml"}}}
< {"error":{"code":-32000,"message":"failed to read config: failed to read config: open /tmp/no-such-file.yml: no such file or directory"}}
```

`failed to read config:` appears twice. Two layers of wrapping both
adding the same prefix.

---

## #20 — Bad-checksum + missing parent dir produces misleading "rename failed" error — LOW

**Repro**: see [`silent-success-bugs.md` #14](./silent-success-bugs.md#14)
for context. When `/tmp/dl/` doesn't exist:

```
failed to move file: rename /tmp/mooncake-download-2352616758 /tmp/dl/jq-bad:
  no such file or directory
```

Real error is checksum mismatch + missing parent dir; surface message
is just the rename failure. Same root cause as #14. Fixing #14 fixes
this too.

---

## Summary table

| # | Sev | Area | Fix |
|---|---|---|---|
| 1 | HIGH | `as_user: root` always sudoes | uid=0 short-circuit |
| 21 | LOW | TLS in vanilla ubuntu container | docs prefer alpine + doctor check |
| 11 | LOW | snapshot tool inventory | input to spec-59 |
| 12 | LOW | `metrics -q --format json` | respect `--format` |
| 13 | LOW | error-hint invalid YAML | use `pkg:`, strip trailing `:` |
| 19 | LOW | `--tags <typo>` silent fail | warn + fuzzy suggestion |
| 42 | LOW | `wait.command` attempt counter | counter accuracy |
| 25 | LOW | MCP notification reply | don't respond to notifications |
| 26 | LOW | MCP error prefix dup | trim outer wrapper |
| 20 | LOW | bad-checksum + missing parent | subsumed by #14 |
