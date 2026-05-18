# Spec 71: `fleet init` — auto-pair via SSH (close the spec-47 follow-up)

**Epic:** Personal Fleet — extends `mooncake fleet init` from spec-45
PR13 / spec-47 with the one leg that didn't ship: SSH-fetch the
bearer token (and, when needed, SSH-bootstrap the daemon) without a
manual paste or a second command.
**Status:** Draft
**Effort:** S (~2 days)
**Value:** High for the "I got a new laptop / I want to control my
existing PCs from it" path — eliminates the 3× `ssh <pc> sudo cat
... | fleet pair` ritual and reduces fleet setup to one command +
one Y/n per host.
**Depends on:** none. Independent of spec-70 (local agentd
bootstrap) — they intersect only because spec-70's `--user`-mode
install puts the token at a different path, which §Design 3 handles.

---

## Problem

The current `mooncake fleet init` (cmd/fleet_init.go) discovers
candidates from mDNS + `~/.ssh/config` + existing `peers.toml`,
renders a candidate table, and walks the operator through them one
by one. For agentd-up candidates it asks the operator to paste a
token; for ssh-only candidates it prints a hint and stops:

```go
// cmd/fleet_init.go:242-244
fmt.Fprintf(w, "   `fleet bootstrap` integration with `fleet init` is a spec-47 follow-up.\n")
fmt.Fprintf(w, "   run `mooncake fleet bootstrap %s` to add this peer.\n", sshTargetFor(cand))
```

```go
// cmd/fleet_init.go:273
token, err := promptSecret(w, reader, fmt.Sprintf("? %s — paste bearer token (cat /etc/mooncake/agentd.token on %s)", cand.Name, cand.Name))
```

Both legs assume the operator does the SSH work themselves: `ssh pc1
sudo cat /etc/mooncake/agentd.token`, then paste back. For three
machines that's three SSH logins, three sudo prompts, three tokens
shuttled by hand. The token never had to leave the controller's SSH
session; today's code just doesn't carry it through.

Concrete user scenario: **operator has 3 PCs already running agentd,
adds a fresh laptop as a new controller.** SSH keys are set up
(prereq the operator accepts). Today this is `curl install.sh | sh`
followed by either:

- `scp pc1:~/.config/mooncake/peers.toml ~/.config/mooncake/` —
  works only if pc1 was already the controller, and copies state
  the new laptop didn't intend to inherit; or
- 3× `ssh pc{n} sudo cat /etc/mooncake/agentd.token | mooncake
  fleet pair pc{n}.local:7878 --token-via stdin` — works, but is
  the painful path this spec eliminates.

The fix is to teach `fleet init` the two SSH actions it already
hints at, reusing helpers that already exist in tree.

---

## Goals

- **G1** `fleet init` auto-fetches the bearer token over SSH for
  candidates that are both agentd-up *and* ssh-reachable (the
  common LAN case: mDNS + `~/.ssh/config` both saw the host).
  Default Y/n prompt, defaults to Y.
- **G2** `fleet init` chains into `fleet.Bootstrap` for candidates
  that are ssh-reachable but not yet running agentd. Reuses the
  existing 8-step flow; the prompt is "bootstrap over SSH now?",
  defaults to Y.
- **G3** Graceful fallback. When SSH fails, sudo wants a password,
  or the token path doesn't exist, `fleet init` offers the
  existing paste prompt instead of abandoning the candidate.
- **G4** `--accept-all` becomes useful: every ssh-reachable
  candidate auto-adds. The existing error on mDNS-only candidates
  stays (no SSH = no automated path); spec-71 doesn't change the
  invite/pair-redeem story.
- **G5** No protocol changes, no new endpoints, no new discovery
  source. Pure CLI orchestration over existing primitives.

**Non-goals:**

- **`/v1/pair-redeem` or any invite/join verb.** Tracked separately
  as a follow-up proposal (see Out-of-scope). Spec-71 stays inside
  the SSH-prereqs-are-fine bound the operator already accepts.
- **Windows targets.** mDNS + ssh-config discovery is Linux/macOS-shaped
  in practice; Windows hosts come in via `fleet bootstrap` directly.
  No init-side regression for Windows because today's init already
  doesn't auto-handle them.
- **Pulling an existing peers.toml from a current controller.** A
  cheaper-than-pairing-every-PC path exists conceptually (`fleet
  adopt-fleet`), but it requires the new endpoint and is in the
  invite/join proposal, not here.
- **Changing the candidate table or discovery output.** Pure
  prompt-flow rewrite below the existing renderCandidateTable.

---

## Reuse map

**Reused:**

- `cmd/fleet_init.go` — `fleetInitAction`, `runInitPrompts`,
  `promptOneCandidate`, `promptYesNo`, `promptSecret`,
  `parseTagInput`, `sshTargetFor`. The prompt loop's shape stays;
  individual branch bodies change.
- `internal/fleet/discovery/aggregate.go` — `Aggregate`, `Options`.
  No change. The aggregator already populates `SSHUser` / `SSHPort`
  on a candidate when ssh-config sees the same host as mDNS or
  peers.toml (aggregate.go:185-205). That's the join we lean on.
- `internal/fleet/bootstrap.go` — `Bootstrap`, `BootstrapOptions`,
  `BootstrapResult`. Called directly for the ssh-only branch
  (cmd/fleet.go:121-133 is the existing call site to mirror).
- `internal/fleet/bootstrap.go:360` — `readToken`. Today it's
  package-private and reached only via the `Bootstrap` orchestrator.
  Promote to an exported `fleet.FetchToken(ctx, target, opts)` so
  `fleet init` can call it without doing the rest of bootstrap.
- `internal/fleet/transport.Connect` — same `transport.ConnectOptions`
  that `fleet bootstrap` already uses.
- `internal/fleet/peers.go` — `fleet.Upsert`. Unchanged.

**Extracted / lightly refactored:**

- `readToken` (plus the `sudoer` newtype it relies on) become
  callable from outside `Bootstrap`. The minimal extraction is a
  new exported function in `internal/fleet/bootstrap.go`:

```go
// FetchToken connects to target over SSH, sudo-cats the agentd
// token file at the canonical path for the detected OS + install
// shape, and returns the token. Idempotent and side-effect-free
// on the remote.
//
// Probe order:
//   1. system path: /etc/mooncake/agentd.token  (needs sudo -n)
//   2. user path:   ~/.config/mooncake/agentd.token  (no sudo)
//
// The order is chosen so a system-mode install (the default) is
// found first; user-mode installs are detected by the system probe
// 404'ing and the user probe succeeding.
func FetchToken(ctx context.Context, target transport.SSHTarget, opts transport.ConnectOptions) (token string, mode TokenMode, err error)
```

  `TokenMode` is `system` | `user` so the init caller can label the
  added row's transport notes; it's not persisted in peers.toml.

---

## Design

### 1. Surface SSH user/port everywhere we need it

`discovery.Aggregate` already merges ssh-config's `SSHUser` /
`SSHPort` into a candidate when ssh-config sees the same host as
another source (aggregate.go:185-205). For an mDNS-only host the
fields are empty; the new prompt branch treats empty `SSHUser` as
"fall back to `$USER`" and empty `SSHPort` as 22, matching what the
operator types into `fleet bootstrap` manually today.

No discovery-side change is required. The aggregator's merge is the
only join we need; we just teach the init action to use those
fields when present.

### 2. Update `promptOneCandidate` (mDNS-up branch)

Today's body asks for `promptSecret` directly. New body:

```
if candidate is agentd-up (AgentdOK) {
    // Try the automated path first.
    if candidate has SSHUser OR $USER works {
        ask Y/n: "fetch token over ssh (aleh@pc1)?"  default Y
        if yes:
            token = fleet.FetchToken(ctx, sshTargetFor(cand), opts)
            on success: write the row via fleet.Upsert; done
            on auth failure: print clear error, fall through to paste
            on sudo -n failure: print "sudo needs a password; falling back to paste"
                                fall through to paste
            on no-such-file: print "agentd.token not where we expected; falling back to paste"
                              fall through to paste
    }
    // Manual fallback (today's behavior).
    token = promptSecret(...)
    if token == "": skip
    fleet.Upsert with token
}
```

The fall-through covers every "SSH auto-fetch was attempted but
didn't work" case — the operator never loses access to today's
paste path.

`--accept-all` short-circuits the fall-through: if auto-fetch fails
and `--accept-all` is set, the candidate errors out (matching how
mDNS-only candidates already error under `--accept-all`).

### 3. Update the SSHCandidates branch

Today (cmd/fleet_init.go:225-245) prints a "run `fleet bootstrap`
yourself" suggestion. Replace with a Y/n that calls `fleet.Bootstrap`
directly:

```
for cand in plan.SSHCandidates:
    ask Y/n: "bootstrap over SSH now (aleh@pc3)?"  default Y
    if no:
        print "   skipped; run `mooncake fleet bootstrap %s` later."
        continue

    res, err := fleet.Bootstrap(ctx, fleet.BootstrapOptions{
        Target:            <sshTargetFor(cand)>,
        Name:              cand.Name,
        Port:              7878,
        LocalBinary:       <fleet.EnsureLocalBinaryPath()>,
        ControllerVersion: <build-time version>,
        Writer:            w,   // bootstrap's per-step report lines
    })
    if err:
        report; continue (don't abort the whole init)
    fleet.Upsert(peersPath, res.Peer)
```

Implementation note: `fleet init` already imports `internal/fleet`;
no new dep. The progress lines are already prefixed with
`[<peer-name>]` by `bootstrap.go`'s `report` closure, which is
exactly the shape the user expects when something multi-step is
happening inside a prompt loop.

### 4. Sudo password handling

`bootstrap.readToken` runs through `sudoer.Run` which is
`sudo -n sh -c '...'` — fails fast on a missing NOPASSWD entry.
Spec-71 keeps this behavior; the fall-through to paste-prompt is the
correct response. The error message should name the failure mode so
the operator knows whether to fix NOPASSWD or use `--user`-mode
agentd or just paste:

```
   could not fetch token from pc1 over ssh: sudo -n failed
   (the agentd token at /etc/mooncake/agentd.token needs sudo).
   options:
     - install agentd in --user mode (no sudo for token reads),
     - add a NOPASSWD entry for `cat /etc/mooncake/agentd.token`, or
     - paste the token below.
```

(The error string is one place, kept short in code, expanded only
in the help text or `fleet doctor` ladder.)

### 5. Flags

Add to `fleetInitCommand()`:

- `--no-ssh-fetch` — disable G1 + G2 entirely; revert to today's
  prompt-only flow. For operators who specifically don't want
  init to ssh anywhere.
- `--ssh-user <name>` — override the SSH user for mDNS-only
  candidates that don't have an ssh-config user. Default: `$USER`.
- `--ssh-port <n>` — same idea for port. Default: 22.

`--accept-all` and `--peers-file` already exist; their semantics
extend naturally (per G4 above).

### 6. Output shape

The summary line gets a small extension to count auto vs paste:

```
fleet init: wrote ~/.config/mooncake/peers.toml
  (3 added via SSH, 0 added via paste, 0 skipped).
✓ fleet ready: `mooncake fleet status` to verify.
```

The per-host line tells the operator *which* path the row came in
through (relevant for audit; e.g. "did this token come from SSH I
can audit, or from clipboard I can't"):

```
   ✓ added pc1 (token via ssh aleh@pc1)
   ✓ added pc2 (token via ssh aleh@pc2)
   ✓ added pc3 (bootstrap via ssh aleh@pc3, version 1.4.0)
```

---

## Test plan

### Unit (Go)

- `cmd/fleet_init_test.go` — extend the existing prompt fixture
  tests:
  - Auto-fetch happy path: candidate has both mDNS + ssh-config;
    init invokes a stub `FetchToken` that returns a token; row
    appears in the written `peers.toml`.
  - Auto-fetch sudo-fail fallback: stub `FetchToken` returns
    `ErrSudoRequiresPassword`; prompt then asks for a paste,
    paste succeeds, row written.
  - Auto-fetch auth-fail fallback: stub returns
    `ErrSSHAuth`; prompt asks for paste.
  - SSH-only candidate, bootstrap path: stub `fleet.Bootstrap`
    returns a `BootstrapResult`; row written.
  - `--accept-all` + agentd-up + SSH available: auto-fetched, no
    prompt.
  - `--accept-all` + agentd-up + SSH fails: hard error (don't
    silently skip).
  - `--no-ssh-fetch`: every candidate goes through paste-prompt
    (today's behavior preserved).

- `internal/fleet/bootstrap_test.go` — add a `FetchToken` test
  with a fake transport.Session covering: system-path success,
  user-path success after system-path NXDOMAIN, both-paths-fail
  surfaces the system error (more informative).

### Integration

- New: `cmd/fleet_init_integration_test.go` (build-tagged
  `integration`) — spin a real agentd in `--user` mode in a
  tempdir-rooted XDG env, set up a fake ssh-config pointing at
  127.0.0.1, run `mooncake fleet init --accept-all
  --no-mdns --peers-file <tempfile>`, assert the row landed with
  the token agentd printed.

### Manual verification (consumer scenario)

- Test rig: 3 Linux boxes running agentd (mix of system and user
  mode), one fresh laptop. SSH keys to all three set up. Run
  `mooncake fleet init` on the laptop. Confirm:
  - all three rows added with one Y per host
  - tokens match what agentd has on disk
  - `mooncake fleet status` shows all three reachable

---

## Migration

- `fleet init` callers (humans + the agent harness) see additive
  behavior: candidates that today require a paste now default to
  an SSH fetch with a Y/n prompt that still defaults to Y. Anyone
  who actually wants the old paste flow uses `--no-ssh-fetch`.
- No protocol / on-disk / discovery changes. `peers.toml` rows
  written are bit-identical to what today's paste prompt would
  produce.
- The `cmd/fleet_init.go:242-244` "run `mooncake fleet bootstrap`
  later" hint disappears — operators who relied on reading it as
  a copy-paste source can run `fleet init --no-ssh-fetch` and get
  the same suggestion path.

---

## Open questions

1. **User-mode-first vs system-mode-first probe.** §Design "Reuse
   map" lists system-first. The user-mode install is opt-in
   (`--user`) and rarer in v1, so system-first finds the common
   case fastest. Worth re-checking after spec-70 ships if user-mode
   becomes the recommended path for laptops.
2. **`--ssh-user` global vs per-prompt.** v1 makes it a global
   flag. If operators commonly have different default users per
   host they'd already have those in `~/.ssh/config`; the global
   flag is for the rare mDNS-only host where ssh-config doesn't
   know about it. If the per-prompt case shows up, add an
   interactive override at the Y/n step.
3. **Sudo password prompt instead of fall-back.** Some operators
   would prefer "ask me for the sudo password and retry" over
   "fall through to paste". Mooncake's existing sudo-pass plumbing
   (cmd/fleet.go's `--insecure-sudo-pass` shape, and the
   security.FilePasswordProvider) could be wired here, but it's
   more code than v1 needs. Defer until asked.
4. **Bootstrap progress concurrency.** `fleet init` prompts are
   serial today. A `--parallel` flag is tempting but interacts
   weirdly with TTY prompts; v1 keeps serial.

---

## Out-of-scope follow-ups

- **Invite / join model** (`fleet invite`, `fleet join`,
  `/v1/pair-redeem`). Eliminates SSH entirely for the
  peer-to-controller direction. Targeted as a separate proposal
  once spec-71's SSH-fine path has proven the demand for further
  reduction.
- **`fleet adopt-fleet`** — pull an entire peers.toml from an
  existing controller in one shot. Belongs in the same proposal
  as invite/join (shares the pair-redeem endpoint).
- **`mooncake fleet doctor init`** — a pre-flight check for the
  exact SSH prereqs `fleet init` needs (key auth, sudo NOPASSWD
  for the token path, port 7878 reachable). Cheap to add once the
  fall-back error messages prove what operators actually trip on.

---

## Pickup checklist (for the implementing agent)

1. Read `cmd/fleet_init.go` end-to-end (~440 LOC; small file). The
   prompt loop in `runInitPrompts` and `promptOneCandidate` is
   where this spec lives.
2. Read `internal/fleet/bootstrap.go:355-403` (`readToken` +
   `enableLinger`) so the FetchToken extraction stays a move, not
   a rewrite.
3. Land the extraction first (`FetchToken` exported, no behavior
   change). Run `task ci` — every existing bootstrap test should
   pass.
4. Wire the mDNS-up branch (G1) next. Add the unit tests in
   `cmd/fleet_init_test.go` alongside.
5. Wire the SSH-only branch (G2). Mirror the call site shape from
   `cmd/fleet.go:121-133`.
6. Add the fall-through error messages (§Design 4); these are the
   part operators will see when something goes wrong, so polish
   them.
7. Add the `--no-ssh-fetch` / `--ssh-user` / `--ssh-port` flags
   last. Document them in `--help` and any docs-next page that
   references `fleet init`.

Claim spec-71 in `~/.mooncake/claims.jsonl` before starting (per
mooncake's `CLAUDE.md`). Worktree: `git worktree add
../mooncake-spec-71 -b worktree-spec-71`.
