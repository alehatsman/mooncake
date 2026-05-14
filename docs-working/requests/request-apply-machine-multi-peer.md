# Request — `mooncake apply <machine>`: ordered multi-peer apply for a single conceptual machine

**Status**: User request, not yet specced
**Filed**: 2026-05-14 by aleh (from the controller side, sitting at x1)
**Related**: [`epics/epic-personal-fleet.md`](../epics/epic-personal-fleet.md) — this is a concrete deliverable for Stream 4 DX.
**Affects**: `fleet apply` ergonomics, controller-side UX, `peers.toml` shape.

---

## The user-facing ask, in one sentence

> From my laptop (x1), I want to run `mooncake apply main_pc` and have it
> sync my Windows host first, then the WSL Ubuntu side — one command, one
> terminal, one interleaved log stream, fail-fast on phase failure.

## Why this isn't just "run two `fleet apply` commands"

It can be — and that's the workaround I'm using today (see "Today's
workaround" below). But it doesn't scale to the mental model I actually
have. In my head, **`main_pc` is one machine**, not two peers that
happen to share hardware. Same for `mini_pc`. The fact that one box
exposes two agentd daemons (Windows on `:7879`, WSL on `:7878`) is an
implementation detail of how WSL works — it leaks into the UX every
time I want to apply a config.

Concretely, a personal-fleet user shouldn't have to:

- Remember the order (Windows first, then WSL — because `.wslconfig`
  changes need to land before WSL restarts, and Windows-host firewall
  rules need to exist before WSL services try to bind ports).
- Remember the two peer names (`main_pc-win` vs `main_pc`).
- Remember which plan file applies to which peer
  (`platforms/windows/bootstrap.yml` vs `machines/main_pc/index.yml`).
- Write a personal wrapper script in *every* dotfiles repo that uses
  mooncake on a Windows+WSL box.

That last one is the smell. The wrapper is the same shape every time;
this belongs upstream.

## Today's workaround

`scripts/sync.sh` in my dotfiles, called as `make main_pc-sync`:

```bash
#!/usr/bin/env bash
set -euo pipefail
MACHINE="${1:?usage: $0 <machine>}"
VARS=(--vars-file shared/variables.yml --vars-file "machines/${MACHINE}/vars.yml")

mooncake fleet apply --peers "${MACHINE}-win" --plan-dir . "${VARS[@]}" \
  platforms/windows/bootstrap.yml

mooncake fleet apply --peers "${MACHINE}" --plan-dir . "${VARS[@]}" \
  "machines/${MACHINE}/index.yml"
```

Works fine. Two issues with it as the long-term answer:

1. Every multi-peer dotfiles repo will reinvent this. The protocol is
   identical (host bootstrap → WSL apply). It should be a built-in.
2. There's no clean way to introspect "is the fleet healthy for this
   machine?" — `mooncake fleet status` lists peers, not machines. I'd
   want to ask "is main_pc reachable end-to-end?" and have it check
   both `main_pc-win` and `main_pc`.

## What success looks like

```text
$ mooncake apply main_pc
[main_pc-win]  → applying platforms/windows/bootstrap.yml (phase 1/2)
[main_pc-win]  ▶ Disable Windows Fast Startup
[main_pc-win]  ~ Disable Windows Fast Startup
[main_pc-win]  ▶ Deploy .wslconfig to the Windows user home
[main_pc-win]  ~ Deploy .wslconfig to the Windows user home
…
[main_pc-win]  RECAP ok=2 changed=16 failed=0  7s
[main_pc]      → applying machines/main_pc/index.yml (phase 2/2)
[main_pc]      ▶ Deploy /etc/wsl.conf
[main_pc]      ~ Deploy /etc/wsl.conf
…
[main_pc]      RECAP ok=4 changed=23 failed=0  34s
apply main_pc: 2/2 phases ok, 39 changed total, 41s
```

Single command. Phase-prefixed logs. Fail-fast: if `main_pc-win` fails,
`main_pc` is not attempted. Exit code is the worst phase's exit code.

For a single-peer machine like `mac` or `x1`, the same command degrades
naturally to one phase — no change required to the user's invocation.

## Shape of the underlying concept

A "machine" is a named bundle of *ordered phases*, each phase being
`{peer, plan, vars, tags}`. Two reasonable places to declare it:

**(a) A new manifest file per machine.** Lives next to vars:

```yaml
# machines/main_pc/fleet.yml
phases:
  - name: windows-host
    peer: main_pc-win
    plan: ../../platforms/windows/bootstrap.yml
  - name: wsl
    peer: main_pc
    plan: ./index.yml
```

`mooncake apply main_pc` looks up `machines/main_pc/fleet.yml`,
resolves vars from `machines/main_pc/vars.yml` + `shared/variables.yml`
by convention, runs phases in order. Falls back to "single phase using
the local agentd or peer of same name" when no manifest is present.

**(b) Extending `peers.toml` with machine groupings.** Each peer
declares which machine it belongs to and which phase order it runs in:

```toml
[[peers]]
name      = "main_pc-win"
machine   = "main_pc"
phase     = 1
plan      = "platforms/windows/bootstrap.yml"
addr      = "192.168.1.68:7879"
…

[[peers]]
name      = "main_pc"
machine   = "main_pc"
phase     = 2
plan      = "machines/main_pc/index.yml"
addr      = "192.168.1.68:7878"
…
```

Then `mooncake apply main_pc` (or `mooncake fleet apply --machine main_pc`)
picks all peers with `machine=main_pc`, sorts by phase, runs them.

Mild preference for (a) because the plan-to-peer binding is a
*per-dotfiles-repo* concern; `peers.toml` should stay about transport
identity (addr/token/transport), not workflow shape. Putting the
phases in the dotfiles repo also means a teammate cloning the repo
gets the same `mooncake apply main_pc` UX without per-machine config
on their controller.

But (b) has the advantage that `mooncake fleet status` could
automatically roll up "is `main_pc` healthy" by checking both peers.
Maybe both: phases declared in the dotfiles, but `peers.toml` can
optionally tag a peer with `machine=` for status rollup. Decide
during specing.

## Constraints / things to get right

- **Ordering, not parallelism.** `fleet apply --peers a,b,c` today runs
  peers in parallel — that's the right default for "apply this one plan
  to many peers." This is the opposite case: many plans, one machine,
  sequence matters. Should be a *different* mode, not a flag on the
  parallel mode.
- **Fail-fast.** A failed phase aborts subsequent phases. (Optional
  `--continue-on-failure` flag like Ansible has, but default fail-fast.)
- **Vars resolution.** Each phase gets the same vars stack
  (`shared/variables.yml` + `machines/<machine>/vars.yml`) — don't make
  the user list them twice.
- **Tag forwarding.** `mooncake apply main_pc --tag wsl` should forward
  `--tag wsl` to *every* phase. (Phase-specific tags can come later
  via `phases[].tags`.)
- **`--plan-dir` shouldn't need to be specified explicitly.** Default
  to the directory of the manifest (or the repo root if a manifest is
  in `machines/<name>/`).
- **Single-peer machines just work.** `machines/mac/fleet.yml` with one
  phase, or no fleet.yml at all → fall back to "apply
  `machines/mac/index.yml` to the local-agentd peer or to the peer
  named `mac`." `mooncake apply mac` is the same UX.
- **First-time bootstrap caveat.** Can't fleet-apply onto a box that
  doesn't yet have an agentd running. The first Windows bootstrap of a
  brand-new machine still has to be done by hand from an Admin
  PowerShell on that box. After agentd is up, fleet apply takes over.
  Worth documenting in user-facing docs; not a blocker for the feature.

## Out of scope (for this request, fine for follow-ups)

- Cross-machine workflows ("apply this config to *every* machine in
  the right order across the whole fleet").
- Phase parallelism within a machine (e.g., "apply A and B in
  parallel, then C"). Real but rare.
- Hot-swap of running services without downtime (phase 2 today bumps
  ssh.service which kicks the session — orthogonal).
- Approval gates between phases.

## Why this lands in Stream 4 / Personal Fleet

This is exactly the "personal fleet" thesis from
[`epic-personal-fleet.md`](../epics/epic-personal-fleet.md):

> Mooncake should make controlling 1–10 personal machines from a
> single terminal feel as natural as controlling one.

Right now mooncake handles "one machine, many peers" with
`fleet apply --peers a,b,c`. It handles "many machines, same plan"
with the same flag plus tag filtering. The gap is exactly **"one
machine, several ordered plans on different peers"** — and that's the
shape Windows+WSL forces on every personal-fleet user who has one of
those boxes.

A clean answer here makes `mooncake apply main_pc` feel as obvious as
`docker compose up`. Which is the bar.

## Concrete next step for whoever picks this up

1. Read [`epic-personal-fleet.md`](../epics/epic-personal-fleet.md)
   for the broader Stream 4 framing.
2. Decide between manifest-in-dotfiles (option a) vs
   peers.toml-extension (option b) vs hybrid — see "Shape of the
   underlying concept" above.
3. Draft a numbered spec under `docs-working/specs/developer-experience/`
   covering CLI surface, manifest schema, ordering semantics,
   failure/exit-code rules, log prefixing, and how single-peer
   machines fall back.
4. Sanity-check against the existing `scripts/sync.sh` workaround in
   `dotfiles/scripts/` — anything that wrapper does today, the
   built-in must do at least as well.
