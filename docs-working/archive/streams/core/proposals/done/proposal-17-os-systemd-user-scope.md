# Request — `os.systemd`: user-scope (`systemctl --user`) support

**Status**: Draft proposal
**Filed**: 2026-05-17 by aleh (from main_pc / WSL, while migrating dotfiles `mcsearch` component)
**Related**: spec-69 phase 5a (which migrated the SYSTEM-scope handlers; user-scope was explicitly out of scope)

---

## The user-facing ask, in one sentence

> Let me write `os.systemd: scope: user name: foo.service ...` so my
> per-user services (mcsearch tunnel + watch instances on every fleet
> machine) stop being 30 lines of `shell: systemctl --user ...`
> boilerplate.

## Why it matters today

`os.systemd` writes to `/etc/systemd/system/` and shells to bare
`systemctl daemon-reload / enable / start`. The action assumes system
scope by construction. Per-user services need:

- a different unit destination dir (`~/.config/systemd/user/`)
- the `--user` flag on every `systemctl` invocation
- *no* sudo escalation (the user owns the unit + the systemd
  session)

The mcsearch component in alehatsman/dotfiles has 4 user-scope
shells today, all forced because `os.systemd` doesn't cover them:

```yaml
- name: Enable and start mcsearch-tunnel.service
  shell: |
    systemctl --user daemon-reload
    systemctl --user enable --now mcsearch-tunnel.service
  when: mcsearch_role == 'client' and os == 'linux'
  tags: [mcsearch, tunnel]
```

The tunnel.service file itself is also a `file.template:` instead
of an `os.systemd:` block because that action only writes
system-scope units. So a single conceptual operation ("declare
this user-scope service exists, enable it, start it") needs
file.template + shell as a pair.

## Templated instances — a sibling sub-feature

mcsearch also uses a *templated* user unit (`mcsearch-watch@.service`)
with one instance per watched project. The reconciliation loop
("enable every instance in `mcsearch_watch_paths`, disable any
instance not in that list") is the single longest shell block in
the dotfiles:

```yaml
- name: Sync mcsearch-watch instances against mcsearch_watch_paths
  shell: |
    set -euo pipefail
    declared='{{ mcsearch_watch_paths }}'
    if [[ -z "${declared// /}" ]]; then ...
    declare -a paths; read -ra paths <<<"$declared"
    for p in "${paths[@]}"; do
      esc=$(systemd-escape "$p")
      systemctl --user enable --now "mcsearch-watch@${esc}.service"
    done
    # ...prune stale instances...
```

A natural `os.systemd: scope: user` shape would handle
single-instance units. A *separate* `instances:` field (or new
action) would handle the reconciliation case:

```yaml
- os.systemd:
    scope: user
    name: mcsearch-watch@.service
    instances:
      from: "{{ mcsearch_watch_paths | split }}"
      escape: systemd                  # systemd-escape each item
    state: enabled-running
```

That second piece is its own design challenge; if it lands as a
follow-up to `scope: user` that's fine.

## Proposed shape (single-instance)

Add a `Scope` field to `OsSystemd`. Default `"system"`; `"user"`
flips three things:

```go
// internal/actions/os_systemd/handler.go
if r.scope == scopeUser {
    dir = filepath.Join(os.Getenv("HOME"), ".config/systemd/user")
    sudoNeeded = false
    systemctlArgs = []string{"--user"}
}
```

- Unit file written under `$HOME/.config/systemd/user/<name>` instead
  of `/etc/systemd/system/<name>`.
- `systemctl daemon-reload / enable / disable / start / stop / is-*`
  all take the `--user` flag.
- `Sudo: false` in `Permissions()` — the user owns their own
  systemd session.

Idempotency surface unchanged: byte-identical unit content + the
existing observed enabled/active state. Reverse path mirrors the
system-scope flow.

## Adjacency

- macOS user agents (launchd plists under `~/Library/LaunchAgents/`)
  are the parallel surface. Worth filing as a sibling proposal once
  this is shipped; today there's no clean home for them either, and
  mcsearch shells out to `launchctl unload / load` on darwin for
  the same reason it shells out to `systemctl --user` on linux.

## Sites unblocked (alehatsman/dotfiles)

5 shells, all in `components/mcsearch/`:

- Enable + start mcsearch-tunnel.service (linux client path)
- Deploy + daemon-reload mcsearch-watch@.service template
- Restart any running mcsearch-watch@* (try-restart loop)
- Sync watch instances reconciliation loop (needs the templated
  `instances:` sub-feature)
- Plus future per-user service patterns that aren't on the table
  today specifically because they'd be shells anyway
