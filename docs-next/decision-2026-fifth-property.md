# Decision — Fifth typed property (2026)

> This is **not a decision**. It is a *frozen argument*, captured
> while it's still fresh so a future reviewer can re-open it
> without rebuilding the case from scratch. The decision deferred
> until the inversion trigger in [`BOTTLENECK.md`](../BOTTLENECK.md)
> fires; do not edit this doc to "make progress" before that.

The kernel today has four typed properties: **Diff**, **Reverse**,
**Cost**, **Permissions**
([`kernel.md`](../docs-working/vision/kernel.md)). The 2026-05-16
three-persona brainstorm surfaced two candidates for a fifth —
**Determinism** (Architect) and **Provenance** (Innovator) — and a
counter-position (**neither, in 2026**, Startaper). Each section
below records the case in the original voice, near-verbatim from
the brainstorm. No new arguments are added here; if you want to
extend an argument, do it inside the brainstorm-doc directory
([`docs-working/vision/brainstorm/`](../docs-working/vision/brainstorm/))
and re-derive a successor of this doc.

Source passes:

- Architect §"Where the kernel could honestly grow — one
  candidate"
  ([brainstorm lines 36–78](../docs-working/vision/brainstorm/2026-05-16-three-personas.md))
- Innovator §B — "The fifth-property argument — Provenance, not
  Determinism" (brainstorm §B, lines 398–473)
- Round-1 Synthesis §"Disagreement 1 — Determinism vs Provenance
  vs neither" (brainstorm lines 1011–1038)
- Round-2 Synthesis — convergence, leaving the kill criterion to
  this doc (brainstorm §"What round 2 settled," item on
  `BOTTLENECK.md`).

---

## 1. The Determinism case

> **Determinism / Reproducibility** — `Determinism(step) →
> {Deterministic, Inputs[], Sources[]}`. A typed declaration of
> whether `Run(step)` produces byte-identical output from byte-
> identical starting state, and what external inputs the step
> depends on (registry contents, current time, network
> reachability, random seed).

Why this isn't already covered by the existing four:

- **Diff** answers *"what would change?"*, not *"would two runs
  change the same thing?"*. `pkg.install postgres` has a defined
  Diff today (installs the package) but is non-deterministic
  across runs because the Ubuntu archive can ship a new point
  release between Tuesday and Wednesday.
- **Reverse** answers *"can I undo?"*, not *"can I re-do?"*. The
  two are independent. `pkg.install` is reversible (uninstall)
  but non-deterministic. `text.line` is deterministic but
  irreversible if the captured pre-state is gone.
- **Cost** answers risk + resources, not determinism.

Why we'd want it:

- **Deterministic replay** (`goals.md` names this as the last
  open piece of the unfair-advantage statement) needs a typed
  contract, not a vibe. Today an agent rerunning a plan from
  yesterday gets a *different* postgres minor version and nobody
  warned them.
- **Plan signing** (Sigstore-style, from VISION §6) is only as
  meaningful as the determinism of the plan's inputs. Pinning the
  plan hash without pinning the upstream package set is theater.
- **Agent quotas / per-action budgets** want to know "is this
  work cacheable across runs?" — a determinism flag answers that
  directly.

*"This is the one fifth-property candidate I'd actually defend at
a review. Not proposing to ship; proposing to put it on the
table."* — Architect

## 2. The Provenance case

> **Provenance** answers a question the existing four cannot.
> Who ran this? With what authority did they claim the right?
> What external evidence pins this run's existence? The four
> properties answer *what / how-to-undo / cost /
> what-authority-needed*; none of them answer
> *who-asserted-it-and-where-is-the-evidence*. That's a different
> axis.

The shape Provenance would take:

```go
type Provenancer interface {
    Provenance(step Step) Provenance
}

type Provenance struct {
    Signer      Identity      // mooncake actor identity
    Ts          time.Time
    LogIndex    *RekorEntry   // public transparency log ref, optional
    PlanHash    [32]byte
    Predecessor *Provenance   // chain to the prior signed run
}
```

Opt-in per handler. Default-resolver in the registry fills in the
ambient run identity for handlers that don't care. Surfaced at
plan time the same way `Cost.Risk` is. Updates the kernel doc's
comparison table with a fifth column where every comparator
scores **✗** (no provenance is typed at plan-time in any of them)
and mooncake scores **✓**.

Why this matters strategically: **Provenance + Sigstore is the
*audit-by-default* property the enterprise wedge (`goals.md` §4)
asks for.** It's also the property that turns the GitOps
"attribution" primitive — which mooncake already inherits from
the runlog — into a typed, queryable, verifiable claim instead of
a text field.

The Innovator's own rebuttal of the Determinism case:

1. **Determinism is mostly derivable from existing primitives.**
   A `Diff` that names the inputs (registry URL, current package
   version, fetched checksum) already encodes the
   non-determinism. Adding a typed `Determinism()` method is
   largely a *rearrangement* of what Diff should already be
   carrying. Sharpen Diff before spending a property column.
2. **Determinism is a property of the *input shape*, not the
   *operation*.** `pkg.install` is non-deterministic *because*
   the upstream archive isn't pinned. The right fix is
   `pkg.install` takes a content-hash argument and refuses to run
   without one in "strict" mode. That's an action-design
   tightening, not a kernel column.
3. **Provenance answers an axis the other four cannot.** (Above.)

Where Innovator would lose the argument: *"if a reviewer can
show me that `plan_hash + actor + Rekor entry` are already
derivable from a combination of the runlog + the existing four
properties, then Provenance collapses into Determinism-or-an-
attribute-of-the-run and shouldn't get a column. I don't think it
does collapse — the *per-step* part is the difference — but I'm
willing to be wrong."*

Innovator's own compromise: *"ship Provenance as a **rendering**
on top of plan_hash + Sigstore for the next 12 months, measure
whether per-step signing actually shows up in user requests, then
promote it to a property if and only if real users need
step-granular signing. Don't speculate the kernel column into
existence."*

## 3. The "neither in 2026" case

The Startaper challenged the Architect: *name one customer-facing
decision in the next six months that hinges on resolving
fifth-property.*

The Architect's recorded answer in Round-1 Synthesis: *"I cannot
name a customer-facing decision in the next six months that
hinges on resolving fifth-property. Plan-signing works on
`plan_hash + Sigstore + actor` as a **rendering** for the
12-month horizon. The fifth-property debate is a 2028
conversation, not a 2026 one."*

That answer is the position this doc currently sits on. **Until
the kill criterion below trips, neither Determinism nor
Provenance ships as a kernel column.** Both arguments stay valid;
neither becomes code.

The shipping order Startaper extracted, and which round 2
confirmed:

- The audit/replay/signing story is served by `plan_hash` +
  Sigstore-on-top + the existing runlog. Render it, don't grow
  the kernel for it.
- The lighthouse-user funnel is the bottleneck (see
  [`BOTTLENECK.md`](../BOTTLENECK.md)), and **kernel-column
  expansion in the absence of a user request that requires it is
  the failure mode the brainstorm was built to prevent.**

## 4. The kill criterion — when this doc opens for resolution

This doc unfreezes only when the inversion trigger in
[`BOTTLENECK.md`](../BOTTLENECK.md) §2 fires:

> The first paid customer — OR two unpaid lighthouse users
> compounded — asks, within a 90-day window, for a feature that
> genuinely requires a fifth typed property on the kernel.

with the operational definitions there ("genuinely requires"
means the request cannot be served by adding a handler, a CLI
command, a flag, a fact producer, an MCP tool, or any other
rendering of the four existing properties; "paid customer"
excludes us and any reviewer; "two unpaid compounded" means two
distinct case-study-generating users surfacing the same need in a
90-day rolling window).

The framing at the time of resolution should be the same
disagreement this doc captures, sharpened by the deciding
evidence:

> **Innovator's Provenance vs Architect's Determinism vs
> neither** — with the deciding question:
>
> *"Did any user in the last six months ask for step-granular
> signed audit, *or* for byte-identical replay across registry
> shifts?"*
>
> If yes-to-signing → Provenance wins.
> If yes-to-replay → Determinism wins.
> If yes-to-both → resolve by which user, which urgency, and
> stage them; don't ship two columns at once.
> If neither → close this doc for another window. The kernel
> didn't need the column; the runlog and Diff sharpening did.

— Round-1 Synthesis §Disagreement 1, near-verbatim.

## 5. What is *not* in scope for this doc

- A third candidate property. If a future brainstorm surfaces
  one, it gets recorded in the brainstorm-doc directory first;
  this doc is only re-opened by the trigger above, and only with
  the candidates that were on the table at the trigger moment.
- Implementation specs. There is no spec file, no handler ABI
  change, no migration plan. Those are downstream of the
  decision; the decision is downstream of the trigger.
- Changes to the existing four properties. Diff-sharpening,
  Reverse-deepening, Cost-refinement, Permissions-refinement —
  all proceed independently of this doc.

---

Ships story
[`S-decision-doc-fifth-property`](../docs-working/vision/brainstorm/2026-05-16-stories.md#s-decision-doc-fifth-property).
