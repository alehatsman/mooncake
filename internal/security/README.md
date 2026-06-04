# Secrets

Mooncake resolves secrets at apply time through a provider registry. A secret
ref is written in YAML as a `!secret <provider>:<path>` tagged scalar — the tag
is rewritten to an in-memory sentinel marker during parsing, so the plaintext
value never touches the plan or the filesystem until the step actually runs.

```yaml
steps:
  - name: push image
    shell: docker login -u ci -p {{ token }}
    vars:
      token: !secret env:REGISTRY_TOKEN      # from environment
      cert:  !secret file:/etc/tls/ca.pem   # from a file
      pw:    !secret vault:db/password       # Age-encrypted, committed to repo
```

## Built-in providers

| Provider | Format | Where the value comes from |
|----------|--------|----------------------------|
| `env`    | `!secret env:VAR_NAME` | Process environment variable |
| `file`   | `!secret file:/path/to/file` | File contents (trailing newline stripped) |
| `stdin`  | `!secret stdin:key` | Interactive prompt (TTY only, per-apply cache) |
| `vault`  | `!secret vault:path/to/name` | Age-encrypted `.age` file in vault directory |

### env

```yaml
password: !secret env:DB_PASSWORD
```

The variable must be set and non-empty; an empty export (`export FOO=`) is
treated as unset.

### file

```yaml
cert: !secret file:~/.config/mooncake/ca.pem
cert: !secret file:/etc/secrets/api.key
```

Tilde (`~`) expands to `$HOME`. No other expansion is performed. Paths should
be absolute for portability across hosts.

### stdin

```yaml
password: !secret stdin:postgres-root
```

Prompts once per named key: `Enter secret for stdin:postgres-root:`. Multiple
refs with the same key share one prompt. Refuses to hang in non-TTY environments
(CI / piped input) — use `file:` or `vault:` for unattended runs.

### vault

Secrets encrypted at rest with [Age](https://age-encryption.org). Safe to
commit to the config repo — each `.age` file is unreadable without the private
key. Intended for dotfiles repos and fleet configs where secrets must be
versioned alongside the config that uses them.

## Vault quick start

```sh
# 1. Generate an identity (private key) — once per machine.
mooncake vault init
# → writes ~/.config/mooncake/vault-identity.txt
# → prints your public key (AGE1...)

# 2. Register yourself as a recipient.
mooncake vault recipients add $(mooncake vault pubkey) --name $(hostname)

# 3. Add a secret (prompts echo-off).
mooncake vault add db/password

# 4. Commit the encrypted file and recipients list.
git add vault/ && git commit -m "vault: add db/password"
```

Reference the secret in any step:

```yaml
steps:
  - name: configure db
    shell: psql -c "ALTER USER app PASSWORD '{{ pw }}'"
    vars:
      pw: !secret vault:db/password
```

## Vault on a new machine

Each collaborator generates their own identity. The repo admin adds their
public key and reruns `rekey` — all existing secrets are re-encrypted for the
new recipient without touching the plaintext.

```sh
# new machine
mooncake vault init
mooncake vault pubkey   # prints AGE1xyz...

# admin (anywhere with the repo checked out)
mooncake vault recipients add AGE1xyz... --name workstation
mooncake vault rekey    # re-encrypts every .age for all recipients
git add vault/ && git commit -m "vault: add workstation"
git push

# new machine: pull + apply works immediately
git pull && mooncake apply config.yml
```

## Managing recipients

```sh
mooncake vault recipients list           # show all registered pubkeys
mooncake vault recipients add AGE1...    # add a key
mooncake vault recipients remove AGE1... # revoke a key
mooncake vault rekey                     # apply list changes to all secrets
mooncake vault rekey --dry-run           # preview without writing
```

`vault/recipients.txt` is a plain text file — one `AGE1...` per line, optional
`# name` comment on the line above. Commit it alongside the `.age` files so
every clone has the full recipient list.

## Vault directory layout

```
vault/
  recipients.txt        # committable recipient list
  db/
    password.age        # encrypted secret
    replica-key.age
  api/
    token.age
```

Default location: `~/.config/mooncake/vault/`  
Override: `MOONCAKE_VAULT_DIR=./vault` (relative paths work — set them per
project in the shell session or a `.envrc`).

## Environment variables

| Variable | Default | Effect |
|----------|---------|--------|
| `MOONCAKE_VAULT_IDENTITY` | `~/.config/mooncake/vault-identity.txt` | Path to the Age private key |
| `MOONCAKE_VAULT_DIR` | `~/.config/mooncake/vault/` | Path to the vault directory |

Per-project override pattern (e.g. in `.envrc`):

```sh
export MOONCAKE_VAULT_DIR="$(git rev-parse --show-toplevel)/vault"
```

## How resolution works

1. The YAML decoder recognises the `!secret` tag and rewrites the scalar to a
   sentinel marker string. The marker flows through plan compilation unchanged —
   `mooncake plan` prints `"!secret vault:db/password"`, never the plaintext.
2. At apply time, `internal/secrets/resolver` walks each step's action fields
   via reflection. Any sentinel marker is resolved through the provider registry.
3. The resolved value is added to the run's `Redactor` denylist so it cannot
   appear in events, runlog, or step stdout.
4. Plan mode never resolves — markers stay in the plan and are safe to inspect
   or transmit.

## CLI reference

- [`mooncake vault init`](../cli/vault_init.md) — generate identity
- [`mooncake vault add`](../cli/vault_add.md) — add or update a secret
- [`mooncake vault list`](../cli/vault_list.md) — list secrets
- [`mooncake vault pubkey`](../cli/vault_pubkey.md) — print own public key
- [`mooncake vault rekey`](../cli/vault_rekey.md) — re-encrypt for current recipients
- [`mooncake vault recipients add`](../cli/vault_recipients_add.md) — add a recipient
- [`mooncake vault recipients list`](../cli/vault_recipients_list.md) — list recipients
- [`mooncake vault recipients remove`](../cli/vault_recipients_remove.md) — remove a recipient
