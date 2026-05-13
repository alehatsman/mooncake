# Spec 27: `os.*` Identity — user, group, ssh_key

**Status:** Draft
**Epic:** E9 Modern Action Surface — bucket E9.3
**Effort:** M (1 week)
**Value:** High. "Set up a server" is the canonical Mooncake use case
and today it's an ugly mix of `shell: useradd`, `shell: usermod`,
manual `~/.ssh/authorized_keys` editing. First-class identity actions
fix this once.

**Design principles:** `docs-working/action-design-principles.md`

---

## Problem

Today's server-setup playbook is a pile of shells:

```yaml
- shell: |
    id deploy || useradd -m -s /bin/bash deploy
    usermod -aG sudo deploy
    mkdir -p /home/deploy/.ssh
    echo "{{ deploy_pubkey }}" >> /home/deploy/.ssh/authorized_keys
    chown -R deploy:deploy /home/deploy/.ssh
    chmod 700 /home/deploy/.ssh
    chmod 600 /home/deploy/.ssh/authorized_keys
```

Not idempotent (the `echo >>` appends every run), no diff, ugly.

---

## Goals

- **G1** `os.user` — declarative user. Create, modify, remove.
- **G2** `os.group` — declarative group.
- **G3** `os.ssh_key` — authorized_keys management. Idempotent at the
  per-key level.
- **G4** All three implement spec-22 hooks.

**Out of scope:**

- `os.sudoers` — separate action (or fold into `os.user`'s
  `sudoer: true` shorthand). Decide during implementation.
- LDAP / SSSD users — Tier-2 (`identity.ldap.*`).
- Password management — only via `!secret`. No plaintext.

---

## Design

### `os.user`

```yaml
- os.user:
    name: deploy
    state: present
    uid: 1500                # optional
    gid: 1500                # optional; or `group: deploy`
    shell: /bin/bash
    home: /home/deploy
    create_home: true
    groups: [sudo, docker]   # supplementary
    append_groups: true      # default true; false replaces
    comment: "Deploy user"
    password: !secret env:DEPLOY_PWHASH  # crypt(3) hash, never plaintext
    expires: never           # YYYY-MM-DD or "never"
    locked: false
```

Idempotency: each managed field is read via `getent passwd` / `chage`
and only modified if drift detected. The full read+write cycle is
recorded for `Diff`.

`state: absent` removes the user. Optional `remove_home: true` deletes
the home directory.

### `os.group`

```yaml
- os.group:
    name: deploy
    state: present
    gid: 1500
    system: false
```

### `os.ssh_key`

```yaml
- os.ssh_key:
    user: deploy
    key: "ssh-ed25519 AAAA... deploy@laptop"
    state: present
    comment: "deploy@laptop"   # optional alternate identifier
    options: [no-port-forwarding, command="/usr/local/bin/restricted.sh"]
    path: ~deploy/.ssh/authorized_keys  # default: auto
```

Idempotency: the `key` is parsed (algo + base64 + comment). Lookup by
algo+base64 (comment is descriptive). Updating options on a matched
key replaces the line; adding the same key with different options is
a change.

Multi-key form:

```yaml
- os.ssh_key:
    user: deploy
    keys:
      - "ssh-ed25519 AAAA... laptop"
      - "ssh-rsa AAAA... yubikey"
    state: present
    exclusive: false       # if true, removes any keys not in this list
```

Authorized_keys file is created with mode 0600, parent .ssh dir with
0700, both owned by the target user.

### Cross-cutting

- **Permissions:** `os.user` and `os.group` declare `Sudo: true` and
  `RequiredBinaries: [useradd|usermod|userdel|getent]` (system-
  dependent). `os.ssh_key` declares `Sudo: true` iff target user's
  home isn't the current user.
- **Cost:** `os.user` create `Risk: 5`, modify `Risk: 4`,
  delete `Risk: 7`. `os.ssh_key` `Risk: 4` (security-relevant).
- **Diff:** field-level before/after. For `os.user`, the diff lists
  changed fields (`shell: /bin/sh → /bin/bash`).
- **Reverse:**
  - `os.user` create ↔ remove. Modify reverses by restoring snapshotted
    fields.
  - `os.ssh_key` add ↔ remove.
  - Deleting a user with `remove_home: true` is declared
    `reversible: false` (data loss).

---

## Key files

| File | Change |
|---|---|
| `internal/actions/os_user/`, `internal/actions/os_group/`, `internal/actions/os_ssh_key/` | New handlers. |
| `internal/config/config.go` | New Step fields: `OsUser`, `OsGroup`, `OsSSHKey`. |
| `internal/register/register.go` | Register three. |
| `internal/security/secrets.go` | Already supports `!secret`. No change. |
| `internal/config/schema.json` etc. | Regenerate. |
| Tests | Per-action; integration test against a Docker matrix. |

---

## Tasks (phased)

1. **Phase 1** — `os.user` create/modify/remove. Field-level idempotency.
2. **Phase 2** — `os.group`.
3. **Phase 3** — `os.ssh_key` single-key and multi-key forms.
4. **Phase 4** — spec-22 hooks across all three.
5. **Phase 5** — Docs + examples + presets for "standard deploy
   user".

---

## Acceptance criteria

- `os.user` setting `groups: [sudo, docker]` on an existing user with
  `groups: [sudo]` reports `changed: true`; second run `false`.
- `os.ssh_key` with `state: present` and the same key twice in a row
  produces byte-identical `authorized_keys`.
- `os.user` with `password: !secret env:HASH` writes the hash; the
  literal value never appears in events / runlogs.
- `os.ssh_key` with `exclusive: true` removes pre-existing keys not in
  the supplied set.
- All three implement spec-22 hooks.
- Build / vet / lint / test green.

---

## Open questions

1. **Should `os.user` support `system: true` to create system users
   (uid < 1000, no home)?** Yes; mirror `useradd -r`.
2. **Should `os.sudoers` be a separate action or fold in?** Likely
   separate (`os.sudoers`) because the file format is subtle and
   `visudo` validation matters. Spec it later.
3. **Locale: on macOS, `useradd` doesn't exist; we have `dscl`.**
   Implement a macOS driver inside `os.user` or punt with a clear
   "unsupported on darwin" error? Probably implement — Mooncake users
   on macOS will want this.
4. **What's the semantic of `os.user` removing a user that has running
   processes?** Default: error. Add `force: true` to override.
5. **Should `os.ssh_key` accept a URL (`https://github.com/alice.keys`)?**
   Tempting. Probably yes via `keys_url:` as a separate field.
