# Bug — `observe.http` silently follows redirects with no opt-out

**Tracking:** [#18](https://github.com/alehatsman/mooncake/issues/18)
**Surfaced:** 2026-05-15 during the post-master-sync network-focused test
sweep.

## Repro

```sh
$ mooncake fleet observe http --url 'http://github.com' --expect-status 301
PEER     STATUS   FOUND  STATUS_CODE  LATENCY_MS  REACHABLE
main_pc  success  true   200          58          true

$ mooncake fleet observe http --url 'http://github.com' --expect-status 200
PEER     STATUS   FOUND  STATUS_CODE  LATENCY_MS  REACHABLE
main_pc  success  true   200          33          true
```

The first invocation asks "is `http://github.com` returning 301?".
GitHub *does* return 301 (HTTP→HTTPS redirect), but `observe.http`
silently follows the redirect, sees the 200 at the final URL, and
reports `STATUS_CODE=200` / `FOUND=false`.

The operator has no way to:
- Detect whether a redirect actually happened
- Verify the redirect status (301 vs 302 vs 307)
- Verify the redirect target (`Location:` header)
- Disable redirect following for the "is the redirect itself working"
  test case

## Why this matters

Redirect verification is a common operational check:
- "Is my HTTP→HTTPS redirect still in place after the cert renewal?"
  → needs to see the 301, not the eventual 200.
- "Did the old URL still redirect to the new path after the
  migration?" → needs the `Location:` header.
- "Did the load balancer's health endpoint accidentally start
  returning a redirect?" → needs to detect the redirect status.

`curl -I` (HEAD with no follow) is what most operators reach for to
answer these questions today. `observe.http` should be able to do
the same.

## Fix

Add two CLI flags + corresponding fields on `config.ObserveHTTP`:

```
--follow-redirects=N    # max redirects to follow (default 10; 0 = no follow)
--capture-redirect-chain # include intermediate {status, location} hops in result
```

YAML shape:

```yaml
- observe.http:
    url: http://example.com/old-path
    follow_redirects: 0          # don't follow
    expect_status: 301
    capture_header: [Location]
```

The CLI default of 10 matches Go's `http.Client.CheckRedirect`
default; 0 disables following. Setting 0 + `--expect-status 301`
gives the canonical "is the redirect working" probe.

Bonus: `capture_redirect_chain: true` records each `{status, url,
location}` hop so the operator can verify the *whole* chain (e.g.
`http://example.com → http://www.example.com → https://www.example.com/`).

## Related observation

`observe.http`'s `--expect-body` substring match is also missing
(tried during this same test sweep — flag doesn't exist). That's
arguably a separate enhancement and worth its own slot.

## Workaround

Drop to raw shell:

```yaml
- shell:
    cmd: |
      curl -sI http://github.com | head -1   # see the literal 301 line
```

Loses the typed result + cross-peer table view that `observe.http`
provides.
