# BOTTLENECK

> One page. Strategic discipline doc. Read at the start of every
> brainstorm before any new proposal is drafted. Do not let it
> grow. Edit it when the bottleneck changes; don't append history.

## 1. The current bottleneck

**Adoption, not engineering.**

The code is shipping faster than the lighthouse-user funnel can
absorb. Two or three written agent-developer case studies would
matter more right now than another spec landing. The kernel claim
is real; the *legibility* of that claim to the actor we most need
to reach (an agent dev evaluating tools, or the agent itself at
MCP tool-selection time) is what's underserved.

Source: [`docs-working/vision/goals.md` §"The strategic
constraint"](docs-working/vision/goals.md), corroborated by the
2026-05-16 three-persona brainstorm and round-2 synthesis.

**Operational consequence (in force until the trigger below
fires):** the project runs a 70/30 GTM-to-kernel work ratio.
Stories tagged `GTM` and `discipline` get prioritized; `kernel`
work proceeds only at the maintenance level needed to keep GTM
stories truthful. See
[`docs-working/vision/brainstorm/2026-05-16-stories.md`](docs-working/vision/brainstorm/2026-05-16-stories.md)
for the active backlog.

## 2. The inversion trigger

The ratio flips when **the first paid customer — OR two unpaid
lighthouse users compounded — asks, within a 90-day window, for a
feature that genuinely requires a fifth typed property on the
kernel.**

Operational definitions:

- **"Genuinely requires a fifth typed property"** means the
  request cannot be served by adding a handler, a CLI command, a
  flag, a fact producer, an MCP tool, or any other rendering of
  the existing four properties (Diff, Reverse, Cost, Permissions).
  It forces the comparison table in [`kernel.md`](docs-working/vision/kernel.md)
  to grow a column. A request that can be served by an existing
  rendering is not the trigger, no matter how much we want it to
  be.
- **"Paid customer"** excludes us, excludes any persona in the
  brainstorm doc, excludes any reviewer of this file. External
  money or external public commitment, no internal advocacy.
- **"Two unpaid lighthouse users compounded"** means two distinct
  case-study-generating users who independently surface the same
  fifth-property need within a 90-day rolling window. One is
  coincidence; two compounded is signal.
- **"90-day window"** is rolling. Reset when a new candidate user
  appears; do not pretend a request from 14 months ago still
  counts.

Until the trigger fires, all of the following stay
**forbidden** — see
[`docs-working/vision/brainstorm/2026-05-16-stories.md` "Forbid
list"](docs-working/vision/brainstorm/2026-05-16-stories.md) and
[`docs-next/decision-2026-fifth-property.md`](docs-next/decision-2026-fifth-property.md)
(once `S-decision-doc-fifth-property` ships):

- A fifth typed property (Determinism, Provenance, or any other
  candidate).
- The L4 enterprise hub / control plane.
- A generic fact-provider plugin SDK.
- A sixth frontend before the R-series refactor lands.
- Parallel polish budget for the solo-dev / dotfiles UX.

## 3. What happens when the trigger fires

In order:

1. **Stop, don't accelerate.** No new GTM commitments. Resist the
   reflex to ride the win.
2. **Update this file in place** — rewrite Section 1 with the new
   bottleneck (most likely some variant of "kernel column needed
   to honor a user commitment"). Do not append history; the prior
   text lives in `git log`.
3. **Flip the work ratio to ≤30% GTM.** Kernel work becomes the
   majority; in-flight GTM stories finish, no new ones start
   without explicit re-prioritization.
4. **Open `docs-next/decision-2026-fifth-property.md` for
   resolution.** The decision doc has been a frozen argument
   until this moment; the trigger is the event that unfreezes it.
   Resolve it with the user request as the deciding evidence.
5. **Schedule a fresh brainstorm.** New three-persona pass; this
   file is the first thing read at the start. The brainstorm's
   job is to redesign the calendar for the new bottleneck, not to
   re-litigate the old one.

## Discipline notes

- This file is **one page**. If it grows past one screen, delete
  something. The whole point is that a brainstorm starts by
  reading it in 60 seconds.
- Edits to this file are **load-bearing events**. The commit log
  for `BOTTLENECK.md` is itself a strategic record; treat each
  edit accordingly.
- Strategic-diary function (longer-form narrative, dated retros,
  brainstorm transcripts) lives in
  [`docs-working/vision/brainstorm/`](docs-working/vision/brainstorm/),
  not here.

Ships story
[`S-bottleneck-doc`](docs-working/vision/brainstorm/2026-05-16-stories.md#s-bottleneck-doc).
