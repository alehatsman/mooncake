---
id: F025
title: fleet.peerDiff doesn't diff Roles or SSH fields — Upsert silently changes them without surfacing
severity: bug
package: internal/fleet
file: internal/fleet/peers.go
lines: 230-245
status: open
---

## What

`peerDiff` (`peers.go:230`) builds the human-readable summary
returned by `Upsert()` when an existing peer entry is replaced:

```go
func peerDiff(old, newer Peer) []string {
    var out []string
    if old.Addr != newer.Addr {
        out = append(out, fmt.Sprintf("addr: %s → %s", old.Addr, newer.Addr))
    }
    if old.Transport != newer.Transport {
        out = append(out, fmt.Sprintf("transport: %s → %s", old.Transport, newer.Transport))
    }
    if old.Token != newer.Token {
        out = append(out, "token: (rotated)")
    }
    if !stringSlicesEqual(old.Tags, newer.Tags) {
        out = append(out, fmt.Sprintf("tags: %v → %v", old.Tags, newer.Tags))
    }
    return out
}
```

`Peer` (`peers.go:42-59`) has **6 user-facing fields** plus
identity (`Name`):

| Field | Diffed? |
|---|---|
| `Name` | n/a (key) |
| `Addr` | yes |
| `Transport` | yes |
| `Token` | yes (redacted) |
| `Tags` | yes |
| **`Roles`** | **no** — spec-50 addition silently misses |
| **`SSH`** | **no** — later addition silently misses |

## Why it's a bug, not a smell

`Upsert` returns `diff` so the caller (`cmd/fleet.go`'s
`bootstrap` and `add-peer` commands) can print "updated peer X:
addr: ... → ..." to the user. The user reads that as "here's
everything that changed."

A `fleet bootstrap` run that re-registers an existing host with
new roles or a new SSH fallback **shows nothing** in the diff —
diff is empty, looks like a no-op, the user moves on. But the
peer's roles/ssh did change on disk. Surprise behavior when the
user later runs `--peer-filter role=primary` and gets a
different set than expected.

Concrete repro:

```sh
# initial bootstrap
mooncake fleet bootstrap user@host --tags=db
# adds peer entry { Name: host, Tags: [db] }

# user manually edits peers.toml to add Roles: [primary, db-replica]
# OR a later spec-50-aware bootstrap auto-fills roles

# re-bootstrap (e.g. after a host reinstall)
mooncake fleet bootstrap user@host --tags=db
# Upsert called; peerDiff returns [] (only Roles differs, which isn't checked)
# user sees: "peer host updated"  — but no detail
# Roles silently reset to [] because the new Peer struct has empty Roles
```

The silent-revert is the load-bearing risk: roles get **dropped**
on a re-bootstrap if the caller doesn't preserve them, AND the
diff doesn't tell the user.

## Suggested fix

Add the two missing diff branches:

```go
func peerDiff(old, newer Peer) []string {
    var out []string
    if old.Addr != newer.Addr {
        out = append(out, fmt.Sprintf("addr: %s → %s", old.Addr, newer.Addr))
    }
    if old.Transport != newer.Transport {
        out = append(out, fmt.Sprintf("transport: %s → %s", old.Transport, newer.Transport))
    }
    if old.Token != newer.Token {
        out = append(out, "token: (rotated)")
    }
    if !stringSlicesEqual(old.Tags, newer.Tags) {
        out = append(out, fmt.Sprintf("tags: %v → %v", old.Tags, newer.Tags))
    }
    if !stringSlicesEqual(old.Roles, newer.Roles) {
        out = append(out, fmt.Sprintf("roles: %v → %v", old.Roles, newer.Roles))
    }
    if old.SSH != newer.SSH {
        out = append(out, fmt.Sprintf("ssh: %s → %s", old.SSH, newer.SSH))
    }
    return out
}
```

Add a test that asserts every non-Name field appears in
peerDiff when changed — that's the regression guard against the
class of "field added, diff forgot to update."

## Adjacent observation

`Upsert` writes `cfg.Peers[idx] = p` (line 222) — full
replacement, not field-by-field merge. So a caller that builds a
new Peer with **only** Addr+Transport+Token+Tags fields filled
(Roles/SSH empty) silently clears any pre-existing Roles/SSH on
the on-disk entry. That's a separate bug from F025: even if the
diff surfaces the change, the user probably didn't intend to
clear roles by re-bootstrapping.

The fix shape:

- Either preserve Roles/SSH from old when newer has them empty
  (sticky-on-empty merge), and document the policy in Upsert's
  doc-comment.
- Or require callers to load the existing peer first and merge
  explicitly.

The cmd-side (`cmd/fleet.go` bootstrap path) is the right place
to make that call, since the cmd knows whether the user passed
`--clear-roles` or similar. F025's diff fix gives the user a
chance to *notice* the silent clear; the merge policy is a
follow-up that prevents it.

## Verification

- Add `TestUpsert_DiffSurfacesRolesAndSSH`: load an existing
  peer with Roles + SSH set, Upsert with the same Name and
  different Roles + SSH, assert the diff slice contains
  "roles:" and "ssh:" entries.
- `go test ./internal/fleet/...`
- Manual: re-bootstrap an existing peer with roles set, check
  the cmd-side prints a non-empty diff.

## References

- Spec-50 — added Roles to Peer.
- `peers.go:42-59` — Peer struct with field comments.
- `cmd/fleet.go` — caller that consumes peerDiff (verify the
  output formatting will pick up the new lines correctly).
