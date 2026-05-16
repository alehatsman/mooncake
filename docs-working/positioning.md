# Positioning

One page that pins **which sentence lives where** and forbids the
"let me invent a third framing for this talk" failure mode named
in [`kernel.md` §R2](./vision/kernel.md#r2-narrative-fragmentation).

Derived from the 2026-05-16 brainstorm:
[Disagreement 3](./vision/brainstorm/2026-05-16-three-personas.md),
Innovator §γ, Startaper §S2.3. Ships story
[`S-positioning-doc`](./vision/brainstorm/2026-05-16-stories.md#s-positioning-doc).

---

## 1. The wedge frame

AI coding agents now routinely emit shell commands that mutate the
operator's system: `apt install`, `systemctl restart`, file
rewrites, config drift. The cost of an agent breaking the system
is borne by the human, on a Saturday. **Agent-rollback is the
wedge.** Mooncake's typed mutation kernel — Diff, Reverse, audit,
idempotency — is what makes rollback a first-class property rather
than a hope. MCP gives us a path into every agent runtime
(Claude Code, Cursor, Codex, Zed) simultaneously, so the rollback
claim is legible to the actor we most need to reach (the agent
choosing which tool to call), not just to the human reading a
homepage. Every external string this project ships should be a
rendering of that one wedge at a different scope.

## 2. String catalog

Every external string surface gets a Startaper-voice sentence
sized to its scope, **derived from the same wedge**. Internal
surfaces stay Architect-voice. The wedge-derivation column is
the test: if you can't trace a surface's string back to
agent-rollback, the string is wrong.

| Surface | Voice | Read by | Wedge derivation |
|---|---|---|---|
| `kernel.md`, spec preambles, comparison tables, ADRs | Architect — "Mooncake is a typed mutation kernel." | Contributors, reviewers, spec authors | None required. Kernel claim is canonical for internal docs; defer to it instead of re-framing. |
| Homepage, README hero, cold-DM, HN title, Loom voiceover | Startaper, broadest — "Mooncake is the rollback button for AI agents touching your system." | Strangers (devs scrolling, HN readers, DM recipients) | The wedge said in one breath. Leads with the visceral consequence (agent broke prod), names the mechanism (rollback), reaches the broadest possible non-contributor. |
| README hero subline, Loom narration mid-beats | Startaper, one breath narrower — "Let your agent run `apt install` and `systemctl` — without losing Saturday to a botched config." | Same strangers, after the hero earned the second sentence | Concretizes the wedge with two failure verbs the lighthouse reader has lived. Inherits scope from the hero above; never replaces it. |
| MCP tool descriptions (`internal/mcp/*.go` `Description:` fields) | Startaper, per-tool narrowest — e.g. "Preview what this change will do before you let the agent run it." | **Agents at tool-selection time** (Claude, Cursor, Codex deciding which tool to call) | The wedge rendered as a selection signal. The agent is choosing between this tool and a generic `shell` tool; the description has to make rollback legible *to the LLM*. Highest-leverage surface in the project — see [`S-mcp-description-rewrite`](./vision/brainstorm/2026-05-16-stories.md#s-mcp-description-rewrite). |
| CLI `--help` one-liner | Startaper for the one-liner | Humans skimming, shell completion tooling | Mirrors the homepage hero, sized to terminal width. |
| CLI `--help` flag descriptions | Architect, terse | Humans reading flags | Operators here already opted in; precision over evangelism. The one-liner did the wedge work. |
| Plan-output recap (`max-risk=9, 3 steps need sudo, 1 reversible`) | Architect, terse | Operators making a go/no-go call on a specific plan | None. The operator is *inside* the kernel claim at this point; render the typed state, don't sell. |
| Error messages (human portion) | Startaper — terse, actionable, oriented at the stuck human | Humans hitting a failure | The wedge as recovery: the error says what was rolled back, what wasn't, and what to do next. Not a third "compiler-diagnostic" register; that becomes wall-of-text. |
| Error messages (structured suffix) | Internal-canonical (JSON / event names) | Tooling, log pipelines | Machine-readable companion to the human portion. Same record, two renderings. |
| Telemetry / event names, log keys, OTel attributes | Internal-canonical only | Contributors, future ourselves grepping logs | Wedge irrelevant. Stability and grep-ability win; do not rename for marketing. |

## 3. Forbid list

Any string surface drifting to a *different* wedge is the failure
pattern named in
[`kernel.md` §R2](./vision/kernel.md#r2-narrative-fragmentation):
the repo carries seeds of seven possible products (provisioning
tool, agent runtime, fleet manager, reconciliation engine,
GitOps-like, cluster substrate, AI execution layer), and any one
of them will compete for the external sentence if allowed.

Specific drifts to refuse:

- **"Declarative config management for your machine"** (the
  provisioning-tool framing). True, but it puts mooncake in the
  Ansible / NixOS comparison where the rollback property reads as
  a feature instead of the wedge. Reserve for the Architect-voice
  comparison table inside `kernel.md`.
- **"Reconciliation engine for desired-state systems"** (the
  control-plane framing). The L4 control plane is forbidden until
  two written agent-dev case studies exist; the *string* is
  forbidden on the same trigger. See
  [`BOTTLENECK.md`](../BOTTLENECK.md) once
  [`S-bottleneck-doc`](./vision/brainstorm/2026-05-16-stories.md#s-bottleneck-doc)
  ships.
- **"Dotfiles for AI"** / **"npm for system config"** (the
  package-manager framing). Wrong audience (solo dev) and wrong
  mental model (install, not mutate-with-rollback). Solo dev is
  the funnel, not the wedge — see
  [`2026-05-16-stories.md` Forbid list](./vision/brainstorm/2026-05-16-stories.md#forbid-list).
- **A new external sentence invented for a talk, podcast, or
  blog post** that does not derive from the agent-rollback wedge.
  If a surface needs new wording, derive it from the canonical
  external sentence (Section 4) — do not invent a fourth wedge
  for one venue.
- **Architect voice on any of the four "strangers" surfaces** in
  the catalog (homepage, README hero, cold-DM, HN title, Loom).
  "Typed mutation kernel" is accurate and useless for tool
  selection; the surface that meets a stranger has to do the
  wedge work first.

When in doubt: re-read Section 1, re-read the canonical sentences
in Section 4, then write the string. If the string can't be
traced back to the wedge, the string is wrong before the surface
is wrong.

## 4. Anchor sentences

Two sentences. Memorize both. Do not invent a third.

- **Internal canonical** (contributors, kernel docs, spec
  preambles, comparison tables, ADRs):

  > Mooncake is a typed mutation kernel.

- **External canonical** (homepage, README hero, cold-DM, HN
  title, Loom voiceover, MCP-tool-description derivations):

  > Mooncake is the rollback button for AI agents touching your
  > system.

Every other external string in this project is a rendering of
the external canonical at a different scope. Every other internal
string defers to the internal canonical. If a doc, a tool
description, or a pitch needs different words, derive them — do
not replace them.
