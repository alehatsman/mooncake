# Request — macOS launchd user-agent surface (parallel to user-scope `os.systemd`)

**Status**: Draft proposal
**Filed**: 2026-05-17 by aleh
**Related**: [proposal-17-os-systemd-user-scope](proposal-17-os-systemd-user-scope.md). This is the macOS half of the same per-user-service story.

---

## The user-facing ask, in one sentence

> Let me declare a user `~/Library/LaunchAgents/<label>.plist` and its `launchctl bootstrap/bootout` lifecycle as data, the same way the linux side will declare user-scope systemd units once proposal-17 lands.

## Why it matters today

mcsearch's client-side tunnel ships as a per-user service that
keeps an SSH `-L` forward open to the embedding host. On linux
clients that's a `systemctl --user enable --now
mcsearch-tunnel.service`. On macOS clients it's a launchd plist
+ `launchctl unload / load`. Both are shells today:

```yaml
- name: Load mcsearch-tunnel launchd plist (mac)
  shell: |
    plist=~/Library/LaunchAgents/com.alehatsman.mcsearch-tunnel.plist
    launchctl unload "$plist" 2>/dev/null || true
    launchctl load "$plist"
  when: mcsearch_role == 'client' and os == 'darwin'
  tags: [mcsearch, tunnel]
```

`os.service` exists and supports darwin per its docstring, but
only at the system-launchd-daemon scope. User-agent scope
(`~/Library/LaunchAgents/`, no root, `launchctl bootstrap
gui/$(id -u)`) isn't covered.

## Proposed shape

Two options, in rough order of preference:

**Option A — extend `os.service` with `scope:`**, mirroring
proposal-17's approach for `os.systemd`:

```yaml
- os.service:
    scope: user                  # default: system
    name: com.alehatsman.mcsearch-tunnel
    state: started
    enabled: true                # bootstrap into gui/<uid> domain
    unit:
      content: <plist xml>       # or dest: path
```

- `scope: user` on darwin writes to `~/Library/LaunchAgents/`,
  runs `launchctl bootstrap/bootout gui/$(id -u) <plist>`,
  doesn't elevate.
- `scope: system` keeps today's behavior (system launchd daemons
  under `/Library/LaunchDaemons/`, root required).

**Option B — separate `os.launchd` action** mirroring `os.systemd`'s
shape. More cleanly parallel to user-scope `os.systemd`; downside
is doubling the per-OS-namespace surface.

I'd lean toward **A** because it preserves the "one action, one
service contract" promise the existing `os.service` docstring
makes ("Supports systemd (Linux), launchd (macOS), and Windows
services"). User-scope is just a missing dimension of that
contract, not a new action.

## Implementation notes

- `launchctl unload | load` is the legacy interface, deprecated
  in favor of `launchctl bootout | bootstrap gui/<uid> <plist>`
  since macOS 10.10. The action should use the modern form.
- Idempotency: a launchd job is bootstrapped iff `launchctl print
  gui/<uid>/<label>` returns 0. Use that as the state probe,
  parallel to `systemctl --user is-active`.
- The plist file write itself reuses the `Performer.WriteFile`
  path with `Become: false` (user owns `~/Library/LaunchAgents/`).
  No special infrastructure needed beyond the dispatcher knowing
  to take the launchd path on darwin + user scope.

## Sites unblocked (alehatsman/dotfiles)

1 shell in `components/mcsearch/index.yml`:

- Load mcsearch-tunnel launchd plist (mac, client path)

The corresponding linux shell ("Enable and start
mcsearch-tunnel.service") is covered by proposal-17 — these two
should land together so the mcsearch component reaches "fully
declarative on both platforms" rather than "declarative on linux,
shell on mac."
