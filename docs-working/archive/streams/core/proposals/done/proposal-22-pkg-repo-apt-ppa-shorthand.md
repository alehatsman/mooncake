# Request — `pkg.repo apt:` `ppa:` shorthand

**Status**: **Shipped** 2026-05-27 (`0fa0d32e`) — `pkg.repo: apt: { ppa: <owner>/<repo> }` derives the launchpad URI + binary keyring from `keyserver.ubuntu.com`; `DistributionCodename` fact added. Default-pinned by fingerprint. Adjacent bugfix: `shared.MaybeDearmor` now handles ASCII-armored `gpg_key_url`. Docker e2e verified on ubuntu:22.04 with neovim-ppa/unstable.
**Filed**: 2026-05-17 by aleh (from main_pc / WSL, while revisiting dotfiles `nvim/ubuntu_install.yml`)
**Related**: existing pkg.repo apt driver. This is a UX shorthand, not a new driver.

---

## The user-facing ask, in one sentence

> Let me write `pkg.repo: apt: { ppa: neovim-ppa/unstable }` and have
> the driver figure out the launchpad URI + keyserver fetch, instead
> of either spelling out the explicit URL + key URL or falling back to
> a `shell: add-apt-repository ppa:...` step.

## Why it matters today

`add-apt-repository ppa:...` is Ubuntu's canonical way to add a PPA.
It collapses three operations into one CLI call:

1. Fetch the launchpad signing key for that PPA from
   `keyserver.ubuntu.com` over HKP.
2. Write a DEB822 source list (`<owner>-ubuntu-<ppa>-<suite>.sources`)
   pointing at `http://ppa.launchpad.net/<owner>/<ppa>/ubuntu`.
3. `apt-get update`.

The existing `pkg.repo apt:` driver covers (2) and (3) cleanly and
covers (1) for repos that publish keys over HTTPS, but **not** for
launchpad PPAs whose keys live on keyservers. The launchpad fallback
is awkward enough that today's choice is between:

- Spelling out the URI and a `https://keyserver.ubuntu.com/pks/lookup?op=get&search=0x<fpr>`
  URL with `gpg_check: false`, OR
- Keeping the action as a `shell: add-apt-repository ppa:...` step.

Both work; neither matches the one-line declarative shape mooncake is
trying to reach for in this surface area.

## Proposed shape

Add a `ppa:` field to `PkgRepoApt`. When set, the driver derives
URI, suites, components, and the keyring URL from launchpad's
well-known shape:

```yaml
- pkg.repo:
    name: neovim-ppa
    state: present
    apt:
      ppa: neovim-ppa/unstable
      # Optional overrides if launchpad's conventions don't fit:
      # suites: ["{{ ubuntu_codename | default:'jammy' }}"]
      # components: [main]
```

Derivation rules:

- `uri` ← `http://ppa.launchpad.net/<owner>/<ppa>/ubuntu`
- `suites` ← `[{{ ubuntu_codename }}]` (auto-detected from facts;
  overrideable)
- `components` ← `[main]`
- `gpg_key_url` ← `https://api.launchpad.net/devel/~<owner>/+archive/ubuntu/<ppa>/signing_key_fingerprint`
  used to *discover* the fingerprint, then fetch the actual key from
  `https://keyserver.ubuntu.com/pks/lookup?op=get&search=0x<fpr>`.
  Alternative: fetch the fingerprint from
  `https://api.launchpad.net/1.0/~<owner>/+archive/ubuntu/<ppa>` (the
  REST API surfaces `signing_key_fingerprint` directly).
- `gpg_key_fingerprint` ← discovered fingerprint, so `gpg_check: true`
  is the default (security upgrade over the curl-based shell).

`ppa:` and `uri:` are mutually exclusive at Validate time.

## Implementation notes

- The launchpad REST API at `https://api.launchpad.net/1.0/~<owner>/+archive/ubuntu/<ppa>`
  returns JSON including `"signing_key_fingerprint"`. One extra HTTP
  call at apply time discovers the fingerprint; the existing keyring
  fetch logic then pulls the key from `keyserver.ubuntu.com` and
  pins it. No new deps.
- `add-apt-repository` parses the PPA shorthand identically; we can
  shell out to it as a fallback if reimplementing the launchpad
  resolution feels heavy. Downside: it adds a `software-properties-common`
  dep on the host.
- For pkg.repo `state: absent`, the same `ppa:` shorthand resolves to
  the same DEB822 path and is removed without contacting launchpad.

## Sites unblocked (alehatsman/dotfiles)

1 shell in `components/nvim/ubuntu_install.yml`:

- Add neovim PPA (currently `shell: add-apt-repository ppa:neovim-ppa/unstable -y`)

Plus any future Ubuntu PPA (the `longsleep/ubuntu-golang-backports`
PPA is already on main_pc, added out-of-band — this same shorthand
would let it move into a `pkg.repo` step).
