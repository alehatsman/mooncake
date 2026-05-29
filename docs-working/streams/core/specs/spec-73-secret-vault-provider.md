# Spec 73: `vault:` secret provider — encrypted-at-rest secrets in the repo

**Epic:** Secrets — extends the `!secret` typed-reference system from
spec-23 §3 with a provider that lets secrets live *inside* a config
repo, encrypted at rest, instead of only as out-of-band `env:`/`file:`
material the operator has to place by hand.
**Status:** Draft
**Effort:** M (~3–4 days incl. the `mooncake vault` CLI)
**Value:** High for the dotfiles/fleet use case — today every secret
(sudo password, dex tokens, service basic-auth creds) lives in a
gitignored `0600` file the operator copies to each host manually. There
is no way to version a secret, no way to share it across the fleet
through the same git pull that ships everything else, and no audit of
when it changed. A `vault:` provider closes that gap without weakening
the plan-time resolution + redaction guarantees already in master.
**Depends on:** spec-23 §3 (`!secret` resolver + provider registry,
shipped). Composes with F037 (plan-time marker expansion) unchanged.

---

## Problem

Mooncake resolves `!secret <provider>:<path>` references at plan time
and redacts the values everywhere downstream (logs, diffs, events,
plan JSON re-render). Two providers ship today, both registered in
`internal/security/`:

- `env:KEY`  — `EnvProvider`, `internal/security/secrets.go:145`
- `file:PATH` — `FileProvider`, `internal/security/secrets_file.go`

Both assume the secret already exists *outside* the repo: an exported
env var, or a file the operator placed (typically `0600`, gitignored).
That is the right default for a single secret on a single box. It does
not scale to a fleet:

- **No versioning.** The secret can't live in git, so a rotation is an
  untracked, un-reviewable side-channel change on every host.
- **Manual fan-out.** Adding a host means re-placing every secret file
  by hand. The whole point of `mooncake apply` over an SSH fleet is to
  *not* do that.
- **No single source of truth.** `git pull` ships the config but not
  the secrets it references, so the two drift.

Concrete motivating case (real, 2026-05-29): the `moongit` dotfiles
component needs an HTTP Basic password for its web UI + git endpoint.
The current workaround is a component-seeded `~/.config/moongit/
moongit.secret.env` (`0600`, created once, never overwritten, outside
the repo). It works, but the password can't be versioned or fanned out
— exactly the gap above. With this spec it becomes:

```yaml
# in the committed component, no out-of-band file:
MOONGIT_BASIC_PASS: !secret vault:moongit/basic_pass
```

---

## Design

### 1. `vault:` provider

A new `VaultProvider` registered alongside the existing two:

```go
// internal/security/secrets_vault.go
func init() { DefaultRegistry.Register("vault", VaultProvider{}) }

type VaultProvider struct{}

// Resolve("moongit/basic_pass") decrypts the vault store, looks up the
// dotted/slashed key, returns the plaintext. Same signature as every
// other provider (internal/security/secrets.go:49), so the resolver,
// redactor, and F037 plan-time expansion need zero changes.
func (VaultProvider) Resolve(path string) (string, error)
```

Resolution flow (unchanged from `env:`/`file:`): the planner expands
`!secret vault:KEY` markers into resolved-then-redacted values before
any handler runs; the redactor masks the plaintext in all event/log/
diff output by value, so a vault secret is no more exposed than an
`env:` one is today.

### 2. Vault store format

One encrypted file per repo, default `secrets.vault` at the config
root (overridable via `MOONCAKE_VAULT` / `--vault`). Plaintext is a
flat YAML map of `key: value`; ciphertext is the encrypted blob.

**Crypto: age** (`filippo.io/age`), not a hand-rolled scheme.
Rationale matching the project's boring-tech bias:

- single small, audited Go dependency; no external binary required
  (unlike shelling out to `sops`/`gpg`).
- supports both passphrase (`scrypt`) and X25519 recipient keys, so a
  fleet can encrypt to each host/operator's age public key — the
  natural fit for "decrypt on N machines."
- armored output is git-diff-friendly (text, not binary).

Key discovery for decryption, first hit wins:

1. `--vault-key <path>` / `MOONCAKE_VAULT_KEY` (age identity file)
2. `~/.config/mooncake/age.key`
3. `MOONCAKE_VAULT_PASSPHRASE` (passphrase mode, for CI)
4. interactive passphrase prompt (TTY only)

### 3. `mooncake vault` CLI

Authoring must never write plaintext to disk:

| Command | Behavior |
|---|---|
| `mooncake vault init` | create `secrets.vault` + generate `age.key` if absent; print the public recipient |
| `mooncake vault edit` | decrypt → `$EDITOR` on a `0600` tmpfile in tmpfs → re-encrypt → shred tmpfile |
| `mooncake vault set KEY [-]` | set one key from arg/stdin without opening an editor |
| `mooncake vault get KEY` | print one plaintext value (TTY-guarded; for debugging) |
| `mooncake vault keys` | list keys (never values) |
| `mooncake vault rekey --add-recipient <agepub>` | re-encrypt for an added/removed fleet recipient |

### 4. Fleet integration

`mooncake apply` over the fleet already ships the repo to each peer.
With per-host age recipients, each peer decrypts locally with its own
identity key — the plaintext never crosses the wire and is never
written to peer disk (resolved in-memory at plan time). `vault rekey`
is how a new host joins.

---

## Scope

**In:**
- `VaultProvider` + registration; age-backed store read path.
- `mooncake vault {init,edit,set,get,keys,rekey}`.
- Key discovery chain; passphrase + recipient-key modes.
- Redaction parity test: a `vault:` value is masked everywhere an
  `env:` value is.
- Docs page + one `examples/secrets/` playbook.

**Out (separate specs if wanted):**
- Remote KMS/Vault-server backends (AWS KMS, HashiCorp Vault) as
  additional providers — the registry already allows them; this spec
  is the local-encrypted-file case only.
- Automatic rotation / TTLs.
- Per-key access policy (ties into the agent-safety policy DSL).

---

## Alternatives considered

- **Shell out to `sops`.** Works today with zero mooncake changes (a
  `shell:` step decrypts to a `file:`-backed secret). Rejected as the
  *native* answer because it adds an external binary to every host's
  dependency chain and leaves a plaintext file on disk mid-apply —
  the very thing the in-memory resolver avoids. Fine as a stopgap.
- **Keep the gitignored `0600` file pattern** (status quo). Correct
  for one secret on one box; does not version or fan out. This spec
  exists precisely for the fleet case it can't serve.
- **Hand-rolled AES.** Rejected — never roll crypto; age is the
  boring, audited choice.

---

## Testing

- Unit: `VaultProvider.Resolve` happy path, missing key, wrong key,
  corrupt blob, missing identity → typed errors with the secret path
  redacted (mirror `secrets_file_test.go`).
- Resolver integration: `!secret vault:k` end-to-end through the
  planner with redaction asserted in event output.
- CLI: `init`→`set`→`edit`→`rekey` round-trip; `edit` leaves no
  plaintext tmpfile behind (assert tmpdir clean on exit).
- Negative: `vault get` refused on non-TTY without an explicit flag.

---

## Open questions

- One vault file per repo, or namespaced multi-file (`secrets/*.vault`)
  for large fleets? Start single-file; the provider path already
  carries a `/`-separated key so namespacing can be additive later.
- Should `apply` fail closed when a referenced vault key is missing,
  or fall through to other providers? Proposed: fail closed with the
  key path (redacted) in the error — same as `file:` not-found today.
