# Next Priorities — 2026-05 (brainstorm)

> Working doc, follow-up to the reality-check review and the
> [DX audit](./dx-audit-2026-05.md). **Not a roadmap.** Use this to
> pressure-test the next 1–2 quarters before committing to specs. Edit freely;
> cross out what's wrong.

---

## The core tension (one paragraph)

Mooncake has shipped a high-quality kernel and a substantial personal-fleet
runtime in 3.5 months. The README and `VISION.md` sell *"the safe execution
runtime for AI-driven system configuration."* But the foundation that turns
the kernel into an **agent-safety** story — the extended handler ABI with
`Diff`/`Reverse`/`Cost`/`Permissions` (spec-22) — is unstarted, and every
spec downstream of it (`transaction:` blocks, framework primitives, policy
DSL, sandbox mode, deterministic replay) is blocked on it. Stream 1 keeps
growing action breadth that doesn't move the agent-safety story forward;
Stream 2 is paused; Stream 3 (the monetizable wedge) has zero specs. **The
work that ships doesn't serve the story being told.** Closing that gap is
the strategic question for the next two quarters.

---

## What's healthy right now (preserve)

- **Kernel quality**: typed errors, snapshot/diff, structured events,
  idempotency contract, ~120k LOC with a healthy production:test ratio.
- **DX wave 1+2** (R1–R10 from the DX audit): `init`, auto-discovery,
  `--dry-run`, `doctor`, examples index, history, `presets recommend`. Most
  shipped in last two weeks.
- **Personal-fleet runtime**: ~5k LOC, 6/14 planned PRs merged, agentd has a
  real SSE hub and a sandboxed file-sync endpoint. The hardest plumbing is
  already in tree.
- **Test discipline**: 194 test files, hooks for `goleak`-style guarantees in
  the multiplexer spec. The "no untested PRs" rule is holding.

None of this should be paused. The question is what gets *added* next.

---

## Three plausible paths

### Path A — Ship the agent-safety wedge ("make the marketing true")

Pause new action families. Drive the extended ABI through to a single
demoable end-state: *"agent edits files, mooncake records the reverse plan,
agent fails halfway through, mooncake auto-reverts the partial change."*

**Concrete spec chain**: spec-22 (ABI) → spec-30 (transactions, the first
real consumer of `Reverse`) → spec-23 (framework primitives: `on_change`,
`!secret`, `try`/`catch`/`finally`) → tiny policy v0 (`deny:` patterns over
`Permissions`) → snapshot-reverse integration.

**Pros**
- Aligns code with the README/VISION pitch the project is already making.
- Produces the *unfair-advantage* demo the vision is missing (open
  question §13.10): "agent did something dumb, undo it" actually works.
- Every Stream-1 spec's final phase (`-P-last: ABI hooks`) unblocks at the
  same time — collapses a lot of pending work into one push.
- The defensible wedge in the vision (§7) becomes real, not slideware.

**Cons**
- Personal-fleet Phase B (PR 7–11: `fleet status`/`logs`/`bootstrap`) sits
  unfinished if it's not protected.
- 1–2 quarters of work where the visible deliverable is "the same demo,
  with a richer audit log" — harder to market than new actions.
- Reverse-correctness is genuinely tricky for some handlers (`pkg.*`,
  `shell`); could surface scope creep mid-spec.

### Path B — Lean into solo-dev as the funnel ("finish what's started")

Finish personal-fleet Phase B (PR 7–11), close the remaining DX-audit items
(R7–R10), polish examples + `presets recommend` into a real onboarding
funnel, ship one "import existing dotfiles" command. Become the boring,
obviously-good tool for dotfiles+devbox, win solo devs, let the agent story
catch up later.

**Pros**
- Plays to current momentum — most of this work is in flight or shipped.
- Lowest scope risk. Each item is < 1 week.
- Solo-dev adopters are who later bring Mooncake to work (vision §4.1
  funnel thesis).
- README claims for the dotfiles wedge are already accurate; nothing to
  retract.

**Cons**
- Doesn't change the marketing-vs-reality gap on the AI-agent pitch.
- "Better Ansible for one user" is not a defensible long-term position.
- Defers the *only* unique wedge (agent safety) by another quarter.

### Path C — Hybrid (the current trajectory)

Keep doing both. Keep shipping action breadth, keep finishing personal
fleet, occasionally pick at spec-22.

**Pros**: nothing has to change.
**Cons**: no wedge reaches "lovable" in a specific quarter; scope sprawl
continues; rebrand-to-Edict discussion will reopen because positioning still
isn't earned.

---

## Recommendation

**Sequence: finish-then-pivot.**

1. **Finish what's in flight (4–6 weeks).** Personal-fleet PR 7–11
   (`status`, `logs`, `bootstrap`, `pair`, `facts`) + DX-audit R7–R10. These
   are mostly known scope, already specced, and stopping mid-PR-chain wastes
   the multiplexer/SSE infra that just landed. Resist any *new* fleet sub-epic
   (C-series, overlays, mDNS polish) until after the pivot.
2. **Then pivot to Path A.** Land spec-22 against spec-30 as a concrete user
   (per `streams.md`'s own work-order). Ship `transaction:` blocks. Add a
   minimum policy DSL (`deny:` over `Permissions`). Wire `Reverse()` into at
   least the file family, `pkg.install`, and `text.line`.
3. **Land one lighthouse user during step 2.** A real AI-agent project or
   one platform team running mooncake against a non-trivial config. The
   work in step 2 is exploratory enough that a real user's edge cases
   should drive design.

The reason for "finish-then-pivot" rather than "pivot now": personal-fleet
PR 6 just merged, the multiplexer is fresh in-cache, and stopping with
`fleet apply` working but `fleet status` missing makes the whole stream feel
unfinished. Better to bank the demo, then commit to the harder pivot from
a clean position.

---

## Concrete next moves (after step 1 above)

| Order | Move | Why it's first | Risk |
|---|---|---|---|
| 1 | Write spec-22 ABI draft against spec-30 transactions (real consumer, not abstract) | `streams.md` already names this | Low — design risk only |
| 2 | Implement `Diff()` + `Reverse()` for the 3 most-used handlers: `file.write`, `text.line`, `pkg.install` | Covers ~70% of an agent's mutations | Medium — `pkg.install` reverse is non-trivial (uninstall vs. "was it already installed?") |
| 3 | Ship spec-30 `transaction:` blocks with auto-revert-on-failure | Headline demo for the agent-safety pitch | Medium |
| 4 | Add `Permissions` declaration + executor preflight check (refuse `requires: [sudo]` if non-root) | Cheap; converts implicit failures into typed errors agents can catch | Low |
| 5 | Tiny policy v0: `deny:` patterns in plan front-matter that match against `Permissions` and `Diff.Resource` | Closes the "agent literally cannot do X" gap with ~200 LOC | Low |
| 6 | One end-to-end demo: agent edits 4 files, third edit fails, mooncake reverts the first two | The thing to put on the README | Low — assembly only |
| 7 | Rewrite README's agent-safety section against the demo | Replace marketing with falsifiable claims | None |

Land all 7 before any new action family.

---

## Explicit "not now" (defer or kill)

| Item | Why defer | Revisit when |
|---|---|---|
| Enterprise hub specs (C1–C5) | No paying users, large engineering surface | A platform team adopts and asks for it |
| WASM plugin model (spec-31) | Premature; in-tree Go is fine for the first year | Two external contributors stalled by a fork |
| Preset marketplace | Cool, not unblocking; presets registry already works locally | Someone is publishing presets and wants discovery |
| Rename to "Edict" | Zero leverage before users | After 100+ stars / 10+ active users |
| `os.firewall` non-ufw drivers (nftables/firewalld) | Driver sprawl; ufw covers 80% | Community PR or paid request |
| New action families (e.g. `cloud.*`, `k8s.*`) | Stream 1 is overheated; ABI is the bottleneck, not breadth | After spec-22 lands |
| Cross-host DAG in fleet plans | YAGNI for 4-box personal fleet | If real users hit it |
| TUI for `fleet apply` | Interleaved stdout covers 99% per the epic | Real user feedback says they need it |
| Windows support (spec-49 in worktree) | Adjacent to the wedge question, but not on the critical path | After Path A demo lands |

---

## Open questions worth resolving before step 2

1. **The unfair-advantage statement.** VISION §13.10 leaves this open. The
   honest answer is probably *"plan + snapshot + reverse + deterministic
   replay, all typed."* Write that down and use it as a filter: any work
   that doesn't strengthen one of those four loses priority.
2. **Reverse correctness for `shell`/`pkg.install`.** Reverse for `file.write`
   is obvious (write the old bytes back). Reverse for `pkg.install` requires
   distinguishing "was already installed" from "we installed it." Is the
   answer (a) declare `reversible: false` and require a transaction wrapper
   to skip pkg steps in reverse, (b) capture pre-state in snapshot and reverse
   from that, or (c) something else? Decide before spec-22 lands.
3. **One audience or two?** The README pitches three audiences. Pick one to
   put first on the README until Path A ships. Best candidate: **AI-agent
   developers**, because that's the marketing being made anyway.
4. **Do we still need `mooncake init` to scaffold an "agent sandbox" template?**
   The DX audit calls it out as one of four `init` templates but no agent
   sandbox runtime exists yet. Drop until Path A ships.
5. **Lighthouse-user shape.** Is the target "an open-source agent project
   (e.g. Aider, Continue) embeds mooncake as its execution layer" or "a
   single team uses mooncake to manage their dev-box fleet via Claude/Cursor"?
   Different feedback loop, different specs.

---

## What success looks like at end of next 2 quarters

If Path A lands cleanly:

1. The README's agent-safety section maps 1:1 to working features and a
   demo. No claims need hedging.
2. `transaction:` blocks ship with at least three real-world examples in
   `examples/`.
3. One external user (agent project or fleet operator) is running mooncake
   in something that matters to them.
4. Stream-1 specs' final phases (`-P-last: ABI hooks`) all close, retiring
   the "blocked on spec-22" line from `streams.md`.
5. The open question "what's the unfair advantage" has a one-sentence answer
   that survives ten minutes of skeptical questioning.

If that's where the project is on 2026-11-14, the rebrand-to-Edict question
answers itself because the positioning is earned.

---

## How to use this doc

- Re-read at the start of every spec planning session for the next two
  quarters.
- When a new spec proposal arrives, check it against the "not now" list and
  the unfair-advantage filter.
- Update the doc — don't preserve it as scripture. If reverse-correctness
  for `pkg.install` turns out to be trivial, edit step 2 of the move list
  and explain why.
- If reality drifts from Path A despite the recommendation, write a new
  brainstorm doc explaining the drift rather than pretending this one
  predicted it.
