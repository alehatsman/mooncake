# Spec 43: `fleet bootstrap` UX + Pairing

**Epic:** Personal Fleet — see [`epics/epic-personal-fleet.md`](../../epics/epic-personal-fleet.md), sub-epic P5.
**Status:** Draft
**Effort:** S (~2–3 days)
**Value:** High — the "add a new box in 60 seconds" demo moment. This spec
is thin: it stitches spec-44 (SSH bootstrap transport) and spec-45 (discovery)
together behind a polished CLI surface.
**Depends on:** spec-44 (SSH driver), spec-45 (discovery), spec-43 (peers.toml).

---

## Problem

Specs 40 and 41 give us the parts:

- spec-44 can install mooncake on a fresh box via SSH and bring up agentd.
- spec-45 can list candidates (mDNS + SSH config + peers.toml).

What's missing is the **one-command flow** that goes from "I just got a
new box" to "it's part of my fleet, I just ran `fleet apply` against it."
This spec ships that flow plus the matching `pair` command for already-
provisioned peers.

---

## Goals

- **G1** `mooncake fleet bootstrap <user@host>` runs the full
  install-and-pair sequence (delegates to spec-44), then prints a
  ready-to-run "Try: mooncake fleet apply ..." hint.
- **G2** `mooncake fleet pair <addr> [--token-via ssh|stdin|file]` adds a
  peer that is ALREADY running agentd. Three token paths:
  - `ssh user@host` — SSH in, `cat` the token file, write to peers.toml.
  - `stdin` (default) — operator pastes.
  - `file <path>` — read from a local file (CI use).
- **G3** Both commands are idempotent on the controller: existing
  `[[peers]]` entries are replaced (with a diff line printed) rather than
  duplicated.
- **G4** Bootstrap optionally consumes a name selected from `fleet init`'s
  output, so the UX flows naturally from "discover" to "add".

**Non-goals:**

- Decommission / uninstall. Separate spec if needed.
- Bulk bootstrap (`fleet bootstrap-many` from a file). Out of scope; the
  user can loop in their shell.
- Re-pair (rotate token). Adjacent spec; flag in open questions.

---

## Reuse map

Everything that does the actual work is in spec-44 and spec-45. This spec
is mostly CLI plumbing.

**Reused:**

- spec-44 SSH driver and orchestrated install (`internal/fleet/bootstrap.go`).
- spec-45 SSH config parser (for resolving `user@host` aliases).
- spec-43 `peers.toml` writer.
- spec-45 candidate aggregator (for `--from-init` mode).

**New:**

| Component | Location |
|---|---|
| `mooncake fleet bootstrap` CLI | `cmd/fleet.go` |
| `mooncake fleet pair` CLI | `cmd/fleet.go` |
| Token-pull-over-SSH helper | `internal/fleet/transport/ssh.go` (extends spec-44) |
| Peers.toml upsert helper | `internal/fleet/peers.go` (extends spec-43) |

---

## CLI shapes

### `bootstrap`

```
mooncake fleet bootstrap <user@host> [flags]
  --name <id>         Peer name in peers.toml (default: hostname from <user@host>)
  --tags <a,b,c>      Tags to set on the new peer
  --port <p>          agentd bind port on target (default 7878)
  --binary <path>     Source binary (default: controller's $0)
  --upgrade           Allow replacing a different version of mooncake
  --dry-run           Show what would happen, change nothing
```

Calls spec-44 `Bootstrap()` then upserts the peer in `peers.toml`.
Output is the spec-44 progress line series plus a trailing pair of
banner lines.

### `pair`

```
mooncake fleet pair <addr> [flags]
  --name <id>             Peer name (default: hostname half of addr)
  --tags <a,b,c>
  --token-via stdin       (default) Prompt for token.
  --token-via ssh:<u@h>   SSH in to <u@h>, cat the token file.
  --token-via file:<p>    Read token from local path.
```

`pair` does NOT install anything. It writes the entry to `peers.toml` after
verifying the token via `GET /v1/version` (returns 200 only when the
bearer matches). If verification fails, nothing is written.

```
$ mooncake fleet pair macbook.lan:7878 --token-via ssh:aleh@macbook.lan --name macbook --tags darwin
[macbook.lan] ssh aleh@macbook.lan: connecting…   ✓
[macbook.lan] reading /etc/mooncake/agentd.token…  ✓
[macbook.lan] verifying token…                     ✓ mooncake 0.9.0
wrote ~/.config/mooncake/peers.toml (added macbook)
✓ macbook paired. Try: mooncake fleet status
```

### Composability with `init`

`mooncake fleet init` (spec-45) presents candidates; selecting an mDNS
candidate with `--bootstrap` for an SSH-only candidate dispatches to
`fleet bootstrap`. Operator can also drive these one-by-one manually if
they prefer.

---

## Idempotency rules

For both commands, when the peer name already exists in `peers.toml`:

1. Compute a "diff" between the new and existing entries (addr, tags,
   token).
2. If identical: print "✓ already paired, no changes." Exit 0.
3. If different: print a one-line diff per changed field, ask Y/n (unless
   `--yes`), and replace.

The replace operation reads peers.toml into memory, swaps the entry,
writes via temp + rename. Other entries untouched.

---

## Tasks

### Task 1 — `pair` command

1. New subcommand in `cmd/fleet.go`.
2. Parse `--token-via`. For `ssh:`: use spec-44 SSH driver,
   `Run("cat <token_path>")`, take the trimmed stdout.
3. Verify by calling `GET /v1/version` with `Authorization: Bearer <token>`.
4. Upsert into peers.toml.

### Task 2 — `bootstrap` command

1. New subcommand in `cmd/fleet.go`.
2. Delegates the eight-step sequence to spec-44 `Bootstrap()`.
3. On success: reads back the token from spec-44's return value, upserts
   into peers.toml.
4. On failure: prints the failure mode from spec-44's documented table,
   exits with non-zero.

### Task 3 — Peers.toml upsert helper

1. Extend `internal/fleet/peers.go`:
   ```go
   func Upsert(path string, p Peer) (added bool, diff []string, err error)
   ```
   `diff` is a list of "field X: old → new" lines when an existing entry
   was replaced.

### Task 4 — Tests

1. `pair` against a fake agentd accepting one token and rejecting all
   others; assert peer is written only on the matching token.
2. `bootstrap` integration: alpine container + sshd; assert peer appears
   in peers.toml after success.
3. Upsert: round-trip a TOML file with comments and ordering preserved
   (use a comment-preserving TOML writer; fall back to documented-loss if
   that's hard).

---

## Open questions

1. **Token rotation.** A separate `mooncake fleet rotate-token <peer>`
   command that triggers the daemon to mint a new token and re-pairs is
   probably worth its own micro-spec. Out of scope here.
2. **Pair without verification.** `--no-verify` for the case where the
   peer is firewalled off from the controller's network during pairing
   (e.g. add an entry now, use it from a different controller location
   later). Lean: don't ship it; the verify step is too useful.
3. **TOML comment preservation.** If we use a roundtrip-preserving TOML
   library (`pelletier/go-toml/v2` does this), comments survive upserts.
   If we use stdlib JSON-like encode, they don't. Lean to preservation.
4. **What does `--dry-run` show for bootstrap?** Same step lines as the
   real run, but each one prefixed with `[would]`. Skip actual SSH
   writes; do the SSH connect (so auth issues surface) and the platform
   detection (so unsupported-platform refuses early), then stop.
