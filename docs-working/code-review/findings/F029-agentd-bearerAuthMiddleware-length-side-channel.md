---
id: F029
title: agentd.bearerAuthMiddleware leaks Authorization-header length via timing — subtle.ConstantTimeCompare short-circuits on length mismatch
severity: risk
package: internal/agentd
file: internal/agentd/middleware.go
lines: 97-110
status: done
resolved_by: worktree-fix-f029
---

## What

The TCP listener's auth middleware uses `subtle.ConstantTimeCompare`:

```go
func bearerAuthMiddleware(token string) func(http.Handler) http.Handler {
    expected := []byte("Bearer " + token)
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            got := []byte(r.Header.Get("Authorization"))
            if subtle.ConstantTimeCompare(got, expected) != 1 {
                w.Header().Set("WWW-Authenticate", `Bearer realm="mooncake-agentd"`)
                writeError(w, http.StatusUnauthorized, "unauthorized", "bearer token required")
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

The comment on line 92-94 claims:

> Constant-time compare keeps the response time independent of where
> the mismatch occurs in the string.

That's true **only when both slices have the same length**. From
the Go stdlib docs for `subtle.ConstantTimeCompare`:

> ConstantTimeCompare returns 1 if the two slices, x and y, have equal
> contents and 0 otherwise. The time taken is a function of the length
> of the slices and **is independent of the contents**. **If the
> lengths of x and y do not match, it returns 0 immediately.**

So:
- Same length as `expected`, different content → comparison runs over
  the full length (constant-time within that length).
- Different length → returns 0 instantly without comparing.

An attacker probing `Authorization: Bearer <guess>` can measure
response time to learn whether their guess matches the expected
length. The token length is small information (probably 32 hex
chars = "Bearer "+32 = 39 bytes total) but it's a side channel
the comment claims doesn't exist.

## Why it's `risk` (not `bug`)

Token *length* leakage is much less serious than token *content*
leakage. The agentd token is a randomly-generated 32+ char string —
even knowing the exact length doesn't help an attacker brute-force
it. So the practical exploitability is low.

But the comment explicitly claims constant-time behavior the
implementation doesn't provide. Anyone reading the code (or
adopting this pattern elsewhere) inherits the wrong mental model.
And the fix is a 3-line change.

## Suggested fix

The standard pattern: hash both inputs to a fixed-size digest
before comparing. SHA-256 is fine here — the cost is in the
microseconds range.

```go
import (
    "crypto/sha256"
    "crypto/subtle"
)

func bearerAuthMiddleware(token string) func(http.Handler) http.Handler {
    expectedHash := sha256.Sum256([]byte("Bearer " + token))
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            got := r.Header.Get("Authorization")
            gotHash := sha256.Sum256([]byte(got))
            if subtle.ConstantTimeCompare(gotHash[:], expectedHash[:]) != 1 {
                w.Header().Set("WWW-Authenticate", `Bearer realm="mooncake-agentd"`)
                writeError(w, http.StatusUnauthorized, "unauthorized", "bearer token required")
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

`gotHash` and `expectedHash` are both 32 bytes regardless of the
attacker's input length, so the `ConstantTimeCompare` runs in
constant time for every request.

Alternative shape (no hashing, length-guarded):

```go
// Cheaper, less obvious. Pads `got` to len(expected) before the
// compare so the length-check short-circuit doesn't fire.
got := []byte(r.Header.Get("Authorization"))
buf := make([]byte, len(expected))
copy(buf, got)
ok := subtle.ConstantTimeCompare(buf, expected) == 1
// Also need to verify len(got) == len(expected) so a shorter
// `got` doesn't get padded into a false match.
ok = ok && len(got) == len(expected)
```

The hash version is more idiomatic and harder to get wrong. Both
preserve the "Bearer " prefix check (any header not starting with
"Bearer " hashes to a different digest from the expected hash).

## Adjacent observation — empty header

`r.Header.Get("Authorization")` returns `""` when the header is
absent. The empty string compares to "Bearer <token>" → length
mismatch → fast rejection. That's the same length-side-channel
issue but practically irrelevant (the header presence is already
observable from the request itself).

## Verification

- Add `TestBearerAuthMiddleware_TimingConstant` that measures
  median wall time over N requests with:
  (a) absent header
  (b) wrong-length header
  (c) right-length wrong-content header
  Pre-fix, (a) and (b) should both be significantly faster than
  (c). Post-fix, all three should be within noise of each other.
- Manual: tcpdump / Wireshark before and after, observe response
  timing under load. Today: (b) is sub-µs; (c) is ~3-5 µs for a
  32-char token. After fix: both ~30 µs (SHA-256 of "Bearer "+32
  bytes).

## Why the existing comment is dangerous

Future devs reviewing this code see "constant-time compare" and
move on. A copy-paste of this pattern into a more
length-sensitive context (e.g. comparing API keys of varying
formats) would silently leak more meaningful information. The
comment teaches an incorrect mental model — the fix is at least
as much about updating the docstring as about the bytes.

## References

- Go stdlib `crypto/subtle.ConstantTimeCompare` docs.
- The bytes/strings of the agentd bearer token are minted by
  `mooncake fleet bootstrap` and persisted in peers.toml on the
  controller side. Length is implementation-defined; today it's
  32 hex chars (from `crypto/rand`).
