# Request — `pkg.install`: brew cask support (`--cask`)

**Status**: Shipped — merged 2026-05-18
**Filed**: 2026-05-17 by aleh
**Related**: shipped [`proposal-07-pkg-aur-support`](done/proposal-07-pkg-aur-support.md), shipped [`proposal-08-pkg-brew-taps-and-tolerant-rc`](done/proposal-08-pkg-brew-taps-and-tolerant-rc.md). Casks are the third major slice of the Homebrew surface; the first two already landed.

---

## The user-facing ask, in one sentence

> Let me write `pkg.install: manager: brew names: [docker, slack, ...] cask: true` instead of `shell: brew install --cask {{ cask_apps | join: ' ' }}` in my macOS dotfiles.

## Why it matters today

Brew is two ecosystems behind the same CLI:

- **Formulae** (CLI tools, libraries): `brew install jq` →
  `pkg.install: manager: brew names: [jq]` ✓ supported today.
- **Casks** (GUI apps, fonts, signed binaries): `brew install --cask docker` →
  no clean home in `pkg.install`. Has to be a shell.

The dotfiles `platforms/macos/packages.yml` has the formula list
migrated to `pkg.install: manager: brew` (per proposal-07/08).
The cask list right next to it is still:

```yaml
- name: Make sure all the cask apps are present
  shell: |
    brew install --cask {{ cask_apps | join: ' ' }}
  failed_when: result.rc not in [0, 1]
  timeout: 30m
```

Same shape, same retry needs, same idempotency surface — just
`--cask` added to the brew command.

A second site: `components/google-cloud/index.yml` darwin path:

```yaml
- name: Install google-cloud-sdk (brew cask, macOS)
  shell: brew install --cask google-cloud-sdk
  when: os == 'darwin'
```

## Proposed shape

Add a `cask` boolean to `Pkg` (and the batch-template path). When
`manager: brew` and `cask: true`:

- Build the command as `brew install --cask <names>` /
  `brew uninstall --cask <names>` for state=present/absent.
- The idempotency check switches from `brew list` to
  `brew list --cask` (or `brew info --cask --json=v2`).
- Reverse path mirrors: uninstall via `--cask`.

```yaml
- pkg.install:
    manager: brew
    cask: true
    state: present
    names:
      - docker
      - slack
      - visual-studio-code
      - font-jetbrains-mono-nerd-font
```

## Why a flag, not a new manager string

`manager: brew-cask` was considered. Rejected because:

- Casks and formulae are the same Homebrew installation, share the
  same prefix, share the tap mechanism (proposal-08). They differ
  only at the install verb (`--cask` flag) and the list query.
- Treating cask as a separate `manager:` would imply they could
  coexist on hosts without Homebrew, which they can't.
- The flag shape lets users declare one bag of formulae and one
  bag of casks in the same machine vars without redundant
  manager: brew on every step.

## Edge case worth flagging

`brew install --cask <name>` returns non-zero (rc=1) for "already
installed" exactly like brew tap returned non-zero for "already
tapped" (proposal-08). Suspect the same fix applies — treat rc=1
plus the right stderr substring as a no-op rather than an error.

## Sites unblocked (alehatsman/dotfiles)

- `platforms/macos/packages.yml` — 14 casks in one shell
- `components/google-cloud/index.yml` — google-cloud-sdk darwin
  install (currently a one-line shell)

Combined with proposal-07 (yay) + proposal-08 (taps) already
shipped, this closes the third leg of the "no shell-outs for
Homebrew" goal.
