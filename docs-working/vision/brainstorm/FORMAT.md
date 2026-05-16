# Three-persona brainstorm format

Reusable skeleton for vision / architecture / strategy brainstorms.
Worked example: [`2026-05-16-three-personas.md`](./2026-05-16-three-personas.md).
Operational follow-on: [`2026-05-16-stories.md`](./2026-05-16-stories.md).

This is the format guide. New brainstorms drop into this folder
following the same pattern.

## When to use this format

- The user asks for a "brainstorm" or "explore" on vision,
  architecture, strategy, or feature direction.
- The work is *deliberative*, not implementation. (For
  implementation, write a stories file *after* the brainstorm.)
- Multiple perspectives produce better output than one voice's
  monologue would.

If the user just wants a feature designed, skip this and use a
normal spec. If they want a refactor planned, use a Plan tool.
This format is specifically for *cross-mindset* deliberation
where the disagreement is the deliverable.

## The three personas

| Persona | Voice | Job |
|---|---|---|
| **Architect** | Kernel discipline | Defends the existing framing (kernel doc, non-goals, established rails). Asks of every proposal: "does this fit (a) adds to the kernel, or (b) adds a rendering?" Plays the role the orchestrating agent takes. |
| **Innovator** | Lateral, current-tools-aware | Maps modern tooling lessons against the project. Pushes at non-goals to surface where they pinch. Willing to propose net-new kernel candidates. |
| **Startaper** | GTM, lighthouse-user, founder-mode | Filters for distribution, monetization, narrative. Refuses to argue features. Names the one cut needed to focus. |

**The three must disagree genuinely.** A unanimous committee
output is the failure mode — push back on prompts or passes that
read as "summarize each other."

## The format

1. **One shared worktree.** `git worktree add ../<proj>-vision-brainstorm -b worktree-vision-brainstorm`.
2. **One shared doc** at `docs-working/vision/brainstorm/<DATE>-three-personas.md`.
3. **Pre-create the file** with all three section headers so the
   subagents can append in-place without racing on file creation.
4. **Architect goes first** (the orchestrating agent, in-line),
   then user spawns Innovator and Startaper as separate agents.
5. **Synthesis at the bottom is the Architect's** after both
   land. Synthesis *adjudicates* — names winners and losers on
   disagreements. Default is to make explicit calls. "Both have
   a point" is the committee output the format exists to avoid.
6. **Round 2 is optional.** Only run it if the user asks. If
   they ask "do we need round 2?" — default answer is *no*
   unless there's a specific unresolved crux *and* the user
   wants convergence.
7. **Claim discipline.** Append `claimed` line to
   `~/.mooncake/claims.jsonl` with task slug
   `vision-brainstorm` before editing.

## Prompts for the subagents (round 1)

Single-paragraph, dense. Customize the bracketed bits.

### Innovator prompt template

> You are the Innovator on the [project] vision brainstorm. Work
> in worktree `[path]`. Read [hard-rails docs] first — [kernel
> doc] is load-bearing, [non-goals doc] is hard rails. Then
> append your section to `[brainstorm-doc-path]` under heading
> `## Innovator's pass`. Riff laterally on [year] tooling (Claude
> Skills/Agent SDK, MCP, jj, helix, uv/ruff, bun, devbox,
> OrbStack, Sigstore, OpenTelemetry, Codex/Cursor/Zed) — map
> each lesson to either "adds to the kernel" or "adds a
> rendering". Take seriously the user's specific creative
> prompts: [list]. Push at non-goals to surface where they
> pinch. End with 3–5 sharp questions for Architect + Startaper.
> Don't commit/push. Claim `vision-brainstorm-innovator` in
> `~/.mooncake/claims.jsonl`.

### Startaper prompt template

> You are the Startaper (founder-mode GTM voice) on the
> [project] vision brainstorm. Work in worktree `[path]`. Read
> [vision docs + roadmap] first. Then append your section to
> `[brainstorm-doc-path]` under heading `## Startaper's pass`.
> Think distribution, lighthouse users, monetization, narrative
> — not features. Who is the *single* user whose tweet would
> matter most in the next 90 days? What's the 30-second demo
> that makes [target user] install? Which of the [N wedges]
> actually funds the next one, and what's the one cut you'd
> make to focus? Take the user's creative prompts seriously:
> [list]. Be opinionated. End with 3–5 sharp questions. Don't
> commit/push. Claim `vision-brainstorm-startaper`.

### Round-2 prompt template (only if the user asks)

> Second pass on the [project] brainstorm doc at `[path]`.
> Re-read the Synthesis section first — it [name the round-1
> adjudication against this persona]. Append `## [Persona]'s
> second pass`. Round 2's job is convergence, not expansion:
> concede where the synthesis was right, push back where it was
> too quick, surface what *new* lateral idea the synthesis
> exposed that you didn't see in round 1. Three sharp questions
> at the end. Don't commit/push.

## Architect's job (the orchestrating agent)

- **Pass 1.** Restate the kernel test as the discipline. Run
  user prompts through the gate. Surface architectural tensions.
  Ask sharp questions to the other two.
- **Synthesis 1.** Adjudicate genuine disagreements. Name what
  becomes a spec vs what stays in the doc. Surface open items.
- **Synthesis 2 (round 2 only).** Converge, don't expand. Settle
  items. Adopt the strongest cross-voice insight. Close the
  brainstorm.

## Lessons learned (2026-05-16 retrospective)

- **Synthesis adjudicates, doesn't summarize.** Make explicit
  calls. Name winners and losers on disagreements.
- **The GTM voice often wins where it disagrees with kernel-
  shaped voices.** Especially mid-stage projects where adoption
  is the bottleneck. Don't muffle Startaper.
- **Pre-create the doc with all section headers** to avoid the
  multi-agent race on file creation. Subagents Edit-in-place.
- **Pin hard-rails docs in every prompt.** Kernel doc +
  non-goals. Otherwise an Innovator will propose adopting
  Bazel's hermeticity or Terraform's plugin SDK and the
  synthesis has to spend cycles refusing them.
- **Force cross-persona questions** at the end of each pass —
  3–5 sharp ones. This makes round 1 land as a conversation
  instead of three monologues, and gives round 2 (if needed)
  concrete cruxes to converge on.
- **One pass is usually enough.** Round 2 only when there's a
  specific unresolved crux *and* the user asks for convergence.
- **Convert to stories *after*, not during.** The brainstorm
  produces decisions + open items. A second doc
  (`<DATE>-stories.md`) turns those into pickable agent-shaped
  work. Don't mix the two.
- **The 70/30 ratio observation generalizes.** When kernel-
  shaped passes argue for more kernel work and GTM argues for
  more distribution, the right answer in mid-stage projects is
  usually "shift to ~70% distribution until a measurable signal
  flips it." Surface this pattern explicitly when it applies.

## File-naming convention

- `<DATE>-three-personas.md` — the brainstorm doc itself.
- `<DATE>-stories.md` — operational follow-on; pickable work.
- `FORMAT.md` — this file. One per project.

The date format is `YYYY-MM-DD`. New brainstorms drop into
this folder using the same naming.
