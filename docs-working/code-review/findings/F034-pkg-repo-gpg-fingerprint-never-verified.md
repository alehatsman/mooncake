---
id: F034
title: pkg.repo's gpg_key_fingerprint is required at validate-time but NEVER verified against the fetched GPG key — silent integrity bypass
severity: bug
package: internal/actions/pkg_repo
file: internal/actions/pkg_repo/handler.go
lines: 151-153, 251, 307-308, 426-437, 482-500
status: open
---

## What

The `pkg.repo` action requires users to supply a
`gpg_key_fingerprint` when `gpg_check: true` (the default):

```go
// handler.go:151-153 (Validate)
if r.Apt.GPGKeyURL != "" && gpgCheckEnabled(r.Apt) && strings.TrimSpace(r.Apt.GPGKeyFingerprint) == "" {
    return fmt.Errorf("pkg.repo.apt: gpg_key_fingerprint is required when gpg_check is true (set gpg_check: false to opt out)")
}
```

The error message **strongly implies the fingerprint will be
checked** ("gpg_check is true"). A user reading this and complying
expects pinned-key verification.

`grep` for downstream uses of the rendered fingerprint:

```
$ grep -nE 'gpgKeyFingerprint' internal/actions/pkg_repo/handler.go
251:	gpgKeyFingerprint string
307:	if out.gpgKeyFingerprint, err = render(r.Apt.GPGKeyFingerprint); err != nil {
308:		return out, fmt.Errorf("pkg.repo: render gpg_key_fingerprint: %w", err)
```

**The field is captured and rendered. Then it's never used.**

The actual fetch + write flow (lines 426-437):

```go
if plan.keyringPath != "" {
    if err := os.MkdirAll(apt.keyringsDir, 0o755); err != nil {
        return fmt.Errorf("pkg.repo.apt: mkdir keyrings: %w", err)
    }
    body, err := fetchKey(r.gpgKeyURL)             // ← HTTP fetch
    if err != nil {
        return fmt.Errorf("pkg.repo.apt: fetch gpg key: %w", err)
    }
    if err := writeAtomic(plan.keyringPath, body, 0o644); err != nil {  // ← straight to disk
        return fmt.Errorf("pkg.repo.apt: write keyring: %w", err)
    }
}
```

No fingerprint comparison anywhere. Whatever bytes `fetchKey`
returns get written to `/etc/apt/keyrings/<name>.gpg` and apt
then trusts packages signed by that key.

`fetchKey` is `httpFetchKey` (line 482):

```go
func httpFetchKey(url string) ([]byte, error) {
    // #nosec G107 -- URL comes from user-supplied YAML.
    resp, err := http.Get(url)
    // ... return body unverified
}
```

`http.Get` accepts both `http://` and `https://`. No TLS pinning,
no scheme check, no integrity check, no length cap. A MITM on a
plain-HTTP key URL trivially substitutes their own key; an
attacker who can hijack DNS for an HTTPS key URL with a coerced
or compromised CA does the same.

## Why this is a bug, not a smell

This is **silent security theater on the apt key trust chain**:

1. Validate says "fingerprint required for security".
2. User supplies a fingerprint.
3. Handler writes apparent-confidence YAML.
4. **The fingerprint is never actually checked.**
5. Apt then trusts the unverified key for every package install
   from that repo for the lifetime of the system.

The end result is **strictly worse** than the no-fingerprint
opt-out (`gpg_check: false`) because:

- The opt-out at least tells the operator "I'm trusting the
  fetch transport blindly."
- The fingerprint-required path tells the operator "you're
  pinned" — but isn't.

For a config-management tool whose threat model includes
"controller pushes signed playbooks to fleet of peers," this
breaks the integrity chain that the rest of mooncake works hard
to maintain (file SHA verification in tool/fetch, atomic writes,
sudo-only key paths, etc.). The pkg_repo handler is the weakest
link.

## Reproduction (conceptual)

```yaml
- pkg.repo:
    name: docker
    apt:
      uri: https://download.docker.com/linux/debian
      suites: [bookworm]
      gpg_key_url: http://attacker.example/docker.gpg
      gpg_key_fingerprint: 9DC858229FC7DD38854AE2D88D81803C0EBFCD88
```

Today: the handler fetches `http://attacker.example/docker.gpg`
over plain HTTP, writes whatever it served as the trusted key,
and the fingerprint is ignored. Apt then trusts the attacker's
key for any package from `download.docker.com`. The mismatch
between the fingerprint operator supplied and the key actually
written is invisible.

## Suggested fix

After fetching but before writing:

```go
body, err := fetchKey(r.gpgKeyURL)
if err != nil {
    return fmt.Errorf("pkg.repo.apt: fetch gpg key: %w", err)
}

// Verify the fetched key matches the operator's pinned fingerprint.
// gpgCheckEnabled was checked at Validate; if it's false the
// fingerprint is empty here and we skip verification (operator opted out).
if r.gpgKeyFingerprint != "" {
    got, vErr := gpgFingerprintOf(body)
    if vErr != nil {
        return fmt.Errorf("pkg.repo.apt: parse fetched gpg key: %w", vErr)
    }
    want := normalizeFingerprint(r.gpgKeyFingerprint)
    if normalizeFingerprint(got) != want {
        return fmt.Errorf(
            "pkg.repo.apt: fetched gpg key fingerprint %s does not match pinned %s (key url: %s)",
            got, want, r.gpgKeyURL,
        )
    }
}

if err := writeAtomic(plan.keyringPath, body, 0o644); err != nil {
    return fmt.Errorf("pkg.repo.apt: write keyring: %w", err)
}
```

`gpgFingerprintOf` parses the key and emits its V4 fingerprint
hex string. Standard library `golang.org/x/crypto/openpgp`
(deprecated but still functional) or a small wrapper around
shelling to `gpg --show-keys --with-fingerprint --with-colons`.

`normalizeFingerprint` upper-cases + strips spaces so users can
write the fingerprint with or without `0x`, with or without
spaces every 4 chars — matches `gpg --fingerprint` output.

## Adjacent observations

### (a) httpFetchKey is unbounded (overlaps F012)

`io.ReadAll(resp.Body)` (line 492) with no size cap. A malicious
server could OOM the daemon by sending GB of garbage as a "GPG
key." Realistic GPG keys are < 100 KB. Cap at e.g. 256 KB:

```go
body, err := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
```

### (b) Plain-HTTP key URLs should be refused

Even with fingerprint verification, allowing `http://` lulls
operators into thinking transport doesn't matter. After F034's
fix, the fingerprint mismatch fails loudly — but a paranoid
operator config can pin to a particular key fingerprint and
still ship a vulnerable playbook because the URL scheme is
operator-facing trust. Add a Validate check:

```go
if r.Apt.GPGKeyURL != "" && !strings.HasPrefix(r.Apt.GPGKeyURL, "https://") {
    return fmt.Errorf("pkg.repo.apt: gpg_key_url must use https:// (got %s)", r.Apt.GPGKeyURL)
}
```

With an opt-out flag (`allow_insecure_key_url: true`) for
operators who explicitly want it.

### (c) No context plumbing — same F012 family

`httpFetchKey` doesn't take a context. Ctrl-C / apply timeout
can't cancel a stuck GPG key fetch. Consistent with F012's
cross-cutting fix.

## Verification

- Add `TestRun_FingerprintMismatchRefuses`: stub `fetchKey` to
  return a known key, supply a deliberately-wrong fingerprint,
  assert Run returns an error and **does NOT** write
  `keyringPath`. Pre-fix the test fails (write happens).
- Add `TestRun_FingerprintMatchSucceeds`: round-trip the
  expected fingerprint of the stubbed key, assert keyring is
  written.
- `go test ./internal/actions/pkg_repo/...`
- Manual: deliberately craft a config with a wrong fingerprint
  against a real apt repo; mooncake should refuse instead of
  writing a key that won't validate.

## References

- The `gpg_check: false` opt-out (line 152) — currently the
  *only* way to skip the fingerprint requirement; should remain.
- `internal/actions/tool/fetch.go` — does sha256 verification
  correctly for similar downloaded artifacts. The pattern is in
  the codebase; just not applied here.
- F012 (HTTP no-timeout cross-cutting) — `httpFetchKey` is on
  the F012 list.
