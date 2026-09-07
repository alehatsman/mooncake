# Multi-persona brainstorm prompt

A reusable shape for vision / strategy / positioning brainstorms in
this repo. Distilled from `2026-05-16-three-personas.md`, which is the
first run of this format and the worked example.

When to invoke: the user asks for a "vision pass", "strategy
brainstorm", "think this through with multiple voices", or similar
positioning work. **Not** for implementation, bug fixes, code review,
or "what should I do next" triage — those are different shapes.

---

## The three personas

- **Architect** — kernel-discipline voice. Tests every proposal
  against the kernel claim ([`kernel.md`](../kernel.md): "adds to the
  kernel OR adds a rendering") and the [seven non-goals](../non_goals.md).
  Names the (a)/(b) classification per proposal. First pass; sets the
  discipline rails.
- **Innovator** — lateral / 2026-tools-aware voice. Surveys the
  current tooling landscape (Claude Skills, MCP, Agent SDK, jj,
  helix, uv/ruff, bun, devbox, OrbStack, Sigstore, OTel, IDE-agent
  trinity). Riffs each against the kernel test. Loud / wide / keeps
  optionality open. Goes second.
- **Startaper** — GTM / founder-mode voice. Distribution, lighthouse
  user, monetization, narrative. *Not features.* Goes third; filters
  the other two through "what ships and converts in 90 days." Demands
  single-track wedge focus and refuses parallel investment.

Three voices, real disagreement, no committee statement. The point
of disagreement *is* the brainstorm's output.

## The rounds

- **Round 1**: each persona writes a pass, ending with 3–5 sharp
  questions for the other two.
- **Synthesis** (Architect-led): names points of agreement, lists
  disagreements with explicit adjudication, produces a "what becomes
  a spec vs. stays in this doc" table, names open items the synthesis
  explicitly does *not* close.
- **Round 2**: each persona pressure-tests the synthesis and the
  others' round 1. **Round 2's job is to converge or sharpen
  disagreement, not to repeat round 1.** Concede cleanly where
  another voice changed your mind. End with 3 sharp questions.

Subsequent rounds only if a real new input arrives (a published
artifact, a real user request, a triggering event). Do not loop
round-3-round-4 indefinitely; the synthesis is the deliverable.

## Operating discipline (the rules these brainstorms run on)

These rules emerged from `2026-05-16-three-personas.md` and should
hold by default in future runs.

- **Hard rails:** [`kernel.md`](../kernel.md) framing and the
  [seven non-goals](../non_goals.md). Any proposal that violates them
  is reshaped, not polished.
- **"Forbidden" beats "deferred"** for things that attract spec-shaped
  optimism. "Deferred" gets re-opened every six weeks; "forbidden"
  closes the question.
- **Every "we'll revisit when..." needs a triggering event**, not a
  date. "Q4 2026" is a date; "first paying customer asks for X" is a
  trigger. Brainstorms whose "deferred" items have no triggers
  produce slow drift.
- **Lighthouse-user shortlists do not live in public docs.** Private
  file, dated, with concrete handles. Calendar steps that depend on
  it cannot proceed until it exists.
- **Single-track wedge.** Cut the parallel investments. The "three
  wedges" framing in [`VISION.md`](../../../VISION.md) is internal
  planning fiction; externally pick one funnel.
- **String surfaces speak in different voices, sized to scope.**
  Architect voice for spec preambles + kernel docs. Startaper voice
  for homepage + README hero + Loom + cold-DM + **MCP tool
  descriptions** (the LLM is an external audience reading those
  strings at runtime).

## Coordination when running personas as separate Agents

- Claim each persona in `~/.mooncake/claims.jsonl` with a task slug
  like `vision-brainstorm-<persona>` or `...-<persona>-r2`.
- Use a worktree off a `vision-brainstorm` branch; doc-only edit, no
  code.
- **Append-only writing.** Edit only your placeholder (`*reserved for
  X agent — appended in-place*`). Never overwrite another persona's
  section.
- Poll for the prior persona's claim status to flip `done` AND for
  file size to stabilize before editing. The "skeleton + placeholders"
  pattern lets the Architect set up the doc shape, then later personas
  slot in.

## Reusable prompt skeleton

Copy and fill the `<braces>`. One invocation per persona per round.

```
You are the <PERSONA> (<voice-tagline>) on the Mooncake vision
brainstorm.

Work in worktree </absolute/path/to/worktree>.

Read VISION.md, docs-working/vision/{kernel,goals,non_goals}.md,
docs-working/vision/sharing_and_modules.md, and ROADMAP.md first.
[Round 2: also re-read the Synthesis and the other personas' most
recent passes.]

Then append your section to:
docs-working/vision/brainstorm/<date>-<topic>.md

Use heading:
## <PERSONA>'s [second] pass

Use flock or git status checks to avoid stomping concurrent agents.

<Persona-specific brief — examples:
 Architect: kernel discipline; (a)/(b) classification per proposal;
   name where the kernel could honestly grow.
 Innovator: 2026 tools landscape mapped to K / R / non-goal-trap;
   chase user's creative prompts; push at the non-goals.
 Startaper: think distribution, lighthouse user, monetization,
   narrative — not features. Be opinionated.>

[Round 2 only:]
Pressure test <specific claim from round 1 / synthesis> against
what <other persona> just argued. Be willing to revise your own
first pass where the other voices changed your mind. Round 2's
job is to converge or sharpen disagreement, not to repeat round 1.

[Startaper-specific seeds, if applicable:]
- Who is the single user whose tweet would matter most in 90 days?
- What is the 30-second demo that makes an AI-agent developer install?
- Which wedge funds the next one in 2026?
- What is the one cut you'd make to focus?

[Take user-supplied creative prompts seriously and run them through
the rails.]

End with 3–5 sharp questions for <the other personas>.

Don't commit/push; user handles git.

Claim vision-brainstorm-<persona>[-r2] in ~/.mooncake/claims.jsonl
before editing.
```

## The skeleton the Architect lays down first

Before personas run, the Architect (first invocation) creates the doc
with this scaffold:

```markdown
# <N>-persona brainstorm — <YYYY-MM-DD>

> Three voices: **Architect** (kernel-discipline), **Innovator**
> (lateral, 2026-tools-aware), **Startaper** (GTM, lighthouse-user,
> founder-mode). The point is genuine disagreement, not a unified
> committee statement.

> **Inputs for all three personas:** `VISION.md`,
> `docs-working/vision/{kernel,goals,non_goals,sharing_and_modules}.md`,
> `ROADMAP.md`.
>
> **Hard rails:** kernel framing + seven non-goals.

---

## Architect's pass
<…written first, in full>

---

## Innovator's pass

*(reserved for Innovator agent — appended in-place)*

---

## Startaper's pass

*(reserved for Startaper agent — appended in-place)*

---

## Synthesis

*(Architect-led — written after Innovator and Startaper have landed.)*
```

Round 2 placeholders get appended below Synthesis, same shape.

## Worked example

[`2026-05-16-three-personas.md`](./2026-05-16-three-personas.md) is
the first run of this format. Read it before launching another
brainstorm — the *shape* of the disagreements (Provenance vs
Determinism, wedge ordering, homepage one-sentence, calendar
sequencing) is the canonical example of "real disagreement adjudicated
in synthesis, sharpened in round 2."

Notable outputs from that run, to register the pattern's productivity:

- The "MCP tool descriptions are an external-canonical-sentence
  surface" insight surfaced in round 2 from Innovator pressure-testing
  Synthesis — a sub-1-day GTM win that would not have appeared in a
  single-voice pass.
- "Forbidden, not deferred" as a discipline rule.
- The two-tracked lighthouse-user shortlist (rollback wedge + tool-
  quality wedge), driven by round 2 convergence.
- A proposed `BOTTLENECK.md` top-level strategy diary, read at the
  start of every future brainstorm — itself a discipline rail for
  future runs of this format.
