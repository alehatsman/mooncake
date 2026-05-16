# Three-persona brainstorm — 2026-05-16

> Three voices: **Architect** (kernel-discipline), **Innovator**
> (lateral, 2026-tools-aware), **Startaper** (GTM, lighthouse-user,
> founder-mode). The point is genuine disagreement, not a unified
> committee statement. Synthesis at the bottom is one Architect's
> reading — readers should weigh the raw passes themselves.

> **Inputs for all three personas:** `VISION.md`,
> `docs-working/vision/{kernel,goals,non_goals,good_lessons_from_other_tools,sharing_and_modules}.md`,
> `ROADMAP.md`.
>
> **Hard rails:** kernel framing (every proposal either *adds to the
> kernel* or *adds a rendering*) and the seven non-goals. A pass that
> ignores either rail is a different brainstorm; this one operates
> inside them.

---

## Architect's pass

### The discipline restated

The kernel doc's test is the only test:

> Is the proposed thing
>   (a) adding to the kernel (a new typed property, a sharper Diff
>       shape, a deeper Reverse capability)?
>   (b) adding a rendering of the kernel (a frontend, a UX command,
>       a graph emitter, an MCP tool)?

Everything below runs through that gate. Where a proposal fails the
test, I name it. Where it passes, I name *which* of (a) or (b) so the
work has a home.

### Where the kernel could honestly grow — one candidate

The kernel doc explicitly leaves the door open: a **fifth typed
property would be a meaningful claim** and would update the comparison
table. Most proposals that look like fifth-property candidates collapse
into refinements of Diff or Reverse on inspection. One survives the
collapse:

- **Determinism / Reproducibility** — `Determinism(step) →
  {Deterministic, Inputs[], Sources[]}`. A typed declaration of
  whether `Run(step)` produces byte-identical output from byte-
  identical starting state, and what external inputs the step
  depends on (registry contents, current time, network reachability,
  random seed).

Why this isn't already covered:

- **Diff** answers "what would change?", not "would two runs change
  the same thing?". `pkg.install postgres` has a defined Diff today
  (installs the package) but is non-deterministic across runs
  because the Ubuntu archive can ship a new point release between
  Tuesday and Wednesday.
- **Reverse** answers "can I undo?", not "can I re-do?". The two are
  independent: `pkg.install` is reversible (uninstall) but
  non-deterministic. `text.line` is deterministic but irreversible
  if the captured-pre-state is gone.
- **Cost** answers risk + resources, not determinism.

Why we'd want it:

- **Deterministic replay** (`goals.md` lists this as the last open
  piece of the unfair-advantage statement) needs a typed contract,
  not a vibe. Today an agent rerunning a plan from yesterday gets a
  *different* postgres minor version and nobody warned them.
- **Plan signing** (Sigstore-style, from VISION §6) is only as
  meaningful as the determinism of the plan's inputs. Pinning the
  plan hash without pinning the upstream package set is theater.
- **Agent quotas / per-action budgets** want to know "is this work
  cacheable across runs?" — a determinism flag answers that
  directly.

This is the one fifth-property candidate I'd actually defend at a
review. Not proposing to ship; proposing to put it on the table.

### The user's two creative prompts, through the gate

**Prompt 1 — Mooncake-as-Makefile-for-this-repo.**

Kernel test: this is a *rendering* (b). No new kernel work. The
action set already covers everything `make build`, `make test-race`,
`make ci`, `make release` do today — `shell`, `file`, `on_change`
edges, `requires`. The win is real:

- **Dogfood proof.** The kernel binary ships its own build through
  the kernel. Every Mooncake property — Diff, Reverse, Cost,
  Permissions, transactions, audit — applies to `make release` for
  free. A botched `goreleaser` step rolls back to the last good
  artifact set because the steps declared `Reverse`.
- **Agent-callable.** Today an agent that wants to "run the test
  suite" has to know `make test-race` and the project's quirks. If
  `mooncake.yml` declares a `test` step, every MCP-aware agent can
  invoke it the same way — `run_step name=test`. The action surface
  *is* the build interface.
- **Inspectable.** `mooncake plan -f mooncake.yml` shows what `make
  release` will *do* before it does it, with structural Diff. Make's
  `-n` mode is shell-echo; this is structured.

The risk — and it's load-bearing — is **non-goal #7: no pipeline
DSL.** A `mooncake.yml` that grows matrix expansion, parallel batch
declarations, conditional steps with arbitrary expressions, secret
scopes that differ from apply-time, is a Jenkinsfile in YAML and we
have to refuse it.

The discipline that keeps it inside the rails:

- No new YAML keywords for Make's sake. `requires:` already exists,
  `on_change:` already exists, `unless:` already exists, `shell` is
  an action. If a Makefile pattern can't be expressed with what
  ships today, we *don't* express it — we either keep it in the
  Makefile or refactor the build to not need it.
- The artifact stays a *config*, not a pipeline. The test:
  could an operator debug it at 3am during an outage without reading
  new docs? If the answer is "they need to learn what `matrix:`
  means in our YAML", the feature lost.

**Verdict:** ship it as a demo (`examples/dogfood/mooncake.yml` that
implements the project's actual build), not as a new category. The
demo is the artifact a Startaper would put in a tweet. The demo is
*also* what protects the rails — if rewriting our own Makefile
requires a new keyword, the keyword is wrong.

**Prompt 2 — Meta-search for agent help in a few lines of mooncake
config.**

Kernel test: this fails (b) as stated. "RAG-shaped indexer over the
repo, configured in YAML" is a *plugin runtime*, not a rendering.
We don't ship that. Re-shape:

The thing the user actually wants is *the agent's ramp to mooncake
itself*. An agent in Claude Code that needs help with `pkg.install`
or `transaction:` semantics today has to grep `docs/`, scan
`docs-working/`, find the spec, find the examples. Three nested
search surfaces, no typed shape.

The kernel-disciplined version is **a rendering** (b):

- **`mooncake explain <noun>`** (already named in
  `good_lessons_from_other_tools.md`'s adopt-later list as
  `mooncake explain <step|run|resource>`). Extend the noun set:
  `mooncake explain pkg.install` returns the typed schema, three
  examples from `examples/`, the Diff shape, the Reverse
  declaration, and the spec it landed in. *No RAG.* The data is all
  typed and lives in `schema.json` + handler metadata + the runlog.
- **MCP tool: `explain(noun)`.** Same data, exposed to LLM agents
  through the existing MCP server. An agent says "I want to write
  a transaction that installs postgres" and the MCP tool returns
  the structured answer: action schemas, applicable examples,
  reversibility caveats.

What this looks like from the user's "few lines of config" framing:
the *operator* doesn't configure this at all. It ships in the
binary. The "few lines" the user imagined are actually zero lines —
the same way `mooncake plan` is zero lines of config.

**The one thing in the original prompt that could be a real action:**
`facts.repo_index`. A fact producer (existing `internal/facts/`
pattern) that walks a repo and emits structured facts about its
contents (file count by extension, presence of `go.mod`, primary
language). This is *facts*, not search — and the kernel already has
that primitive. An agent that asks "what kind of repo is this?" gets
a typed answer.

**Verdict:** the user's intuition is right; the shape is wrong. Ship
`mooncake explain` (rendering) + tier it through MCP. Don't ship a
generic RAG-config-in-YAML — that's the plugin-marketplace trap with
a search-flavor.

### Architectural tensions I'd surface before the others arrive

Five live tensions, named so the synthesis can adjudicate:

1. **Action breadth vs the closed-set rule.** Modules (Phase 1 of
   `sharing_and_modules.md`) loosen the *composition* layer without
   loosening the action set. But every quarter someone proposes a
   `pkg-2` or a `service-systemd-v2` that's "almost the same action
   but slightly different shape." Discipline: that's not a new
   action, it's a flag on the existing action, or it's a different
   action with a different verb. The closed set bends only at the
   verb level.

2. **Frontends as renderings, today as duplications.** Kernel doc
   §"The frontends" calls this out: five frontends, each
   re-implementing parts of orchestration. The R-series refactor
   (in flight per memory) is the right fix. The brainstorm should
   *not* propose new frontends until the duplication is paid down,
   because each new frontend is a new place the kernel leaks.

3. **Determinism as the missing fifth column.** Covered above.
   Worth ten minutes of Innovator + Startaper time: is the
   replay/cache/quota story compelling enough to spend a property
   budget on?

4. **What "modules" means for agents.** The
   `sharing_and_modules.md` design is human-shaped: someone runs
   `mooncake mod init`, parameterizes, pushes, tags. The agent
   producer/consumer path is sketched but not specced. The
   architectural question: do we need a *machine-readable provenance
   block* in every module — model, prompt, timestamp, parent hash —
   so module consumers can decide trust without reading the module?
   This is a real schema decision before Phase 3 ships.

5. **The MCP server is undervoiced.** It's listed as "working" in
   VISION §3 but it's the single most-used frontend by the people
   we care about (agent developers). Every kernel property that's
   only surfaced in CLI text mode and not in MCP is a missed
   wedge. The audit: walk the four typed properties, confirm each
   has a typed MCP surface, fix the gaps before adding any L5
   features.

### Questions for Innovator + Startaper

- **For Innovator:** Of the 2026 tools you survey, which one's
  *interface choice* (not its features) is the one Mooncake should
  steal? I claim the surviving choice is *jj*'s "every operation
  produces a new immutable state you can name and jump to" — what's
  yours, and does it map to a rendering or to a fifth property?

- **For Innovator:** If we ship `mooncake explain` + MCP-exposed
  `explain` tool, does the "meta-search in a few lines" wedge
  actually evaporate, or is there a residue the kernel doesn't
  cover? Name the residue concretely.

- **For Startaper:** Three wedges, which one's tweet matters most
  in 90 days? I'd argue agent-dev — the lighthouse user is a
  *single Claude/Cursor power user who rewrites their dotfiles in
  mooncake on a livestream*. Yours?

- **For Startaper:** Is the dogfood Makefile-replacement a launch
  artifact, a curiosity, or a wedge? My read: launch artifact. Sell
  me on either of the other two.

- **For both:** Determinism as the fifth typed property — defensible
  bet or scope creep? Argue both sides; the synthesis will pick.

---

## Innovator's pass

I've read the Architect's pass and am writing into it, not past it.
Two of their five questions land squarely on the moves I was going
to make anyway (jj-as-interface-lesson, the meta-search reframe),
and we *disagree* on the fifth-property candidate — they propose
Determinism, I propose Provenance. I'll argue the disagreement on
its merits at §B below rather than negotiate it away in the preamble.

Brief for myself: take the 2026 tooling landscape (Claude Skills, MCP,
the Agent SDK, jj, helix, uv/ruff, bun, devbox, OrbStack, Sigstore,
OpenTelemetry, the IDE-agent trinity Codex/Cursor/Zed), riff each
against the kernel test, then chase the two creative prompts and push
where they pinch the non-goals.

I'm writing the loud / lateral version. The Startaper filters; the
Architect has already discipline-tested. My job is to keep the
optionality wide.

### A. The 2026 tooling landscape, mapped

**K** = adds to the kernel · **R** = adds a rendering · **✗** =
non-goal trap dressed up as a feature

#### A.1 Claude Skills — **R**, with a sharper module shape

Skills are bundled `(instructions + small tools + metadata)`,
discovered at runtime by the agent via a manifest, addressed by
name, versioned. The shape that matters: a self-describing
capability the agent picks up *only when relevant*.

Map onto mooncake's module system (`../sharing_and_modules.md`):

- A `module.yml` is already 80% of a Skill — declared parameters
  with types, a `description:`, steps below. What's missing is a
  *when-to-use* descriptor distinct from the parameter schema.
  Crucially this **doesn't grow into a DSL**: the matcher lives in
  the MCP tool's `description:` string the agent reads natively, not
  in mooncake YAML. mooncake hands the agent typed schema + free
  text; the agent's reasoning is what selects. Non-goal #1 holds.
- Skills bundle *example invocations*. Modules should carry an
  `examples:` block where each example is itself a plan rendered in
  dry-run mode. The agent loading a module via MCP receives not
  just the parameter schema but the **typed Diff the module would
  produce**. That's a richer contract than any Skill ships today —
  and it falls out of the kernel for free.

This is sharpening of the modules design, not a new property.

#### A.2 Claude Agent SDK — **R**

Isolation modes, sub-agents, background tasks. mooncake's `agent/`
loop already exists; `agentd` peer protocol is structurally a sub-
agent already. The one lesson worth surfacing: a `--read-only` flag
on the MCP server that strips mutation tools entirely. Rendering.

#### A.3 MCP — the unfinished half is *response shape*

The protocol is in. The interesting frontier: when an agent calls
`run_plan` or `inspect`, the response **is itself a list of typed
`Diff` records as JSON** — same shape whether the response
describes a planned mutation or an applied one. The agent's loop
logic gets uniform. This is purely a rendering refinement — the
kernel already produces typed Diffs; we just need MCP to stop
lossily serializing them. Audit the MCP server before any L5 work,
per the Architect's tension #5.

#### A.4 jj (jujutsu) — **K-adjacent**

Answering the Architect's "one interface choice Mooncake should
steal" question directly: yes, jj's structural move is the one. Two
parts to it, and I'd steal them in different ways.

1. **The operation log** (separate, append-only, machine-queryable
   history of *operations performed on history itself*). mooncake's
   runlog today conflates "what mutated the system" with "what
   commands an actor ran." Split into `~/.mooncake/runs/` (mutation
   history — already there) and `~/.mooncake/oplog/` (command
   history: `apply`, `rollback`, `replay`, `retry`, with the actor,
   the args, the resulting run-id). This is a **rendering** because
   it's derived from data the kernel already emits — but it's load-
   bearing because `mooncake explain <op-id>` cannot cleanly answer
   "what command caused these mutations?" today.

2. **Every operation produces a named, addressable, immutable
   state.** This is the deeper claim. In jj, `op_id` is a typed
   handle you can jump to, diff against, and replay from. In
   mooncake, the equivalent would be: every `run_id` *plus* every
   `op_id` is a named state of the system *and* the audit trail —
   addressable by hash, queryable, immutable once terminal. Today
   we have `run_id` (mutations) but no comparable handle for
   "operator/agent intent before the run."

Whether the second half nudges toward a kernel column is exactly
the Architect's question and mine intersecting. I think jj's split
is a rendering refinement *if* we keep audit-as-evidence; it
becomes K-territory *only* if we elevate "operation" to a typed
object the handler ABI knows about. That elevation is what
Provenance (§B) would buy.

#### A.5 helix, uv, ruff, bun — **R**, "speed unlocks workflows"

uv replaced pip by being 100× faster. The lesson isn't "rewrite in
Rust" — it's that *speed unlocks workflows that didn't exist*.
`mooncake plan` already runs in milliseconds. The frontier is **plan-
mode-as-pre-commit-hook** (git pre-commit runs `mooncake plan
--diff --target ./` and refuses commits that predict breakage) and
**LSP-shaped editor live-Diff** (every YAML edit re-renders the
predicted Diff in the gutter). The kernel can power both because
Diff is already structured. The work is editor integration plus
making sure cold-start stays sub-100ms.

#### A.6 devbox / mise / direnv — **R**, the project-scope question

devbox declares a per-project shell environment in YAML; activates
on `cd`. mooncake's playbook is structurally identical except scoped
to "this machine." This is the bridge to the Makefile prompt
(§C.1). The lesson worth keeping: project-scoped state is a real,
demanded surface, and any "mooncake replaces your dev tooling"
story has to engage with it rather than ignore it.

#### A.7 OrbStack / Lima / Apple Container — **R**, the rehearsal surface

OrbStack ships a near-native Linux VM that boots in <2 seconds.
This is the missing piece of agent safety: `mooncake plan` is the
cheap rehearsal; nothing today offers the *expensive* rehearsal
(apply to a real ephemeral environment, run assertions, tear
down). Map: `mooncake rehearse <plan>` boots an OrbStack/Lima VM,
applies, asserts, tears down. Kernel unchanged — same Diff /
Reverse / Cost / Permissions. The VM is just *where the plan
lands*. Rendering; cheap-ish to spec when an agent dev asks.

#### A.8 Sigstore — **K-candidate** (Provenance)

See §B for the full argument. Short version: keyless signing + a
public transparency log gives us per-step `(signer, ts,
log_index, plan_hash)` — a typed tuple, opt-in per handler,
derivable from existing primitives but not derived *by* any of
the existing four properties.

#### A.9 OpenTelemetry — **R**, free enterprise win

Runlog → OTel spans is ~200 LOC and the first thing an enterprise
asks for. Cheap rendering.

#### A.10 Codex / Cursor / Zed — **R**, the IDE-agent integration surface

These tools already drive mooncake-shaped workflows. The missing
piece: an extension that takes the *typed* Diff from `mooncake
plan` and renders it inline in the gutter, the same way Cursor
renders a pending edit. The kernel claim matters: `Diff` is JSON,
not text. An IDE consumes JSON natively — something an Ansible-
shaped tool literally cannot do. A `mooncake-vscode` extension that
previews any YAML file's plan in real time would teach more
people what the kernel claim is than another README rewrite.

### B. The fifth-property argument — Provenance, not Determinism

The Architect proposes **Determinism / Reproducibility** as the
candidate fifth typed property. I propose **Provenance**. Here's
the case for keeping mine on the table even after reading theirs.

**The Architect's Determinism case is real.** `pkg.install postgres`
*is* non-deterministic; deterministic replay *does* need a typed
contract; plan-signing without input-pinning *is* theater. All true.

**Where I think it under-counts:**

1. **Determinism is mostly derivable from existing primitives.**
   A `Diff` that names the inputs (registry URL, current package
   version, fetched checksum) already encodes the non-determinism.
   Adding a typed `Determinism()` method is largely a *rearrangement*
   of what Diff should already be carrying. Sharpen Diff before
   spending a property column.

2. **Determinism is a property of the *input shape*, not the
   *operation*.** `pkg.install` is non-deterministic *because* the
   upstream archive isn't pinned, not because the handler itself
   does anything non-deterministic. The right fix is `pkg.install`
   takes a content-hash argument and refuses to run without one in
   "strict" mode. That's an action-design tightening, not a kernel
   column.

3. **Provenance answers a question the existing four cannot.** Who
   ran this? With what authority did they claim the right? What
   external evidence pins this run's existence? The four properties
   answer *what / how-to-undo / cost / what-authority-needed*; none
   of them answer *who-asserted-it-and-where-is-the-evidence*.
   That's a different axis.

**The shape Provenance would take:**

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
plan time the same way Cost.Risk is. Updates the kernel doc's
comparison table with a fifth column where every comparator scores
**✗** (no provenance is typed at plan-time in any of them) and
mooncake scores **✓**.

**Why this matters strategically:** Provenance + Sigstore is the
*audit-by-default* property the enterprise wedge (`goals.md` §4)
asks for. It's also the property that turns the GitOps "attribution"
primitive — which mooncake already inherits from the runlog — into
a typed, queryable, verifiable claim instead of a text field.

**Where I'd lose the argument:** if a reviewer can show me that
`plan_hash + actor + Rekor entry` are *already derivable from a
combination of the runlog + the existing four properties*, then
Provenance collapses into Determinism-or-an-attribute-of-the-run
and shouldn't get a column. I don't think it does collapse — the
*per-step* part is the difference — but I'm willing to be wrong.

**Compromise that might actually be right:** ship Provenance as a
**rendering on top of plan_hash + Sigstore** for the next 12
months, *measure whether per-step signing actually shows up in user
requests*, then promote it to a property if and only if real users
need step-granular signing. Don't speculate the kernel column
into existence.

### C. The two creative prompts

#### C.1 mooncake as a Makefile for *this* repo

The Architect already verdicted this: ship as `examples/dogfood/
mooncake.yml`, not as a new category. **I agree**, with one
addition: stage it as **`mooncake apply ci.yml` as the project's
pre-commit and CI gate**, with `make` keeping incremental
compilation. The wedge framing — not the replacement framing — is
where this earns its place. Every Go repo has a Makefile; very few
have a *quality gate as YAML you can plan-diff before running*.
That's the part `make` can't do.

Three structural mismatches the Architect didn't explicitly name,
worth flagging so a future PR doesn't drift into them:

1. **No file-timestamp dependency tracking.** Make rebuilds when
   `mooncake.go` is newer than `bin/mooncake`. mooncake tracks
   idempotency of state, not transformation timestamps. The fix —
   content-hash caching — is bazel-shaped and would pull toward
   Non-goal #2 spiritually if not literally. Don't.
2. **Tests-as-state is a category error.** "Tests should pass" is
   not converged filesystem state. The clean version: a
   `quality.assert` handler that always reports "drifted" until
   the test command exits 0, treated as a *probe* not a *mutator*.
   This is fine; just name it clearly so an agent doesn't try to
   "fix" the drift by editing the tests.
3. **No step-level parallelism inside one host.** Make's `-j 8`
   has no equivalent. Sequential apply is a current limitation,
   not a stance; if a real Makefile-replacement push lands, this
   is the spec that has to come with it.

The Architect's discipline holds: every keyword `make` would need
that we don't already have is a keyword we don't ship. If
rewriting our own Makefile requires `matrix:` or `parallel:` or
inline expressions, the rewrite is wrong, not the rails.

#### C.2 Meta-search for agent help in a few lines of mooncake config

The Architect reframed this as `mooncake explain <noun>` —
typed schema + examples + Diff shape, served from `schema.json`
and handler metadata. They argued the RAG/index version is the
plugin-marketplace trap with a search flavor.

**I think the explain reframe is right *and incomplete*.**
`mooncake explain` covers everything *the kernel already knows
about itself*. The residue — and the Architect asked me to name it
concretely — is everything *outside* the typed surface but still
load-bearing for agent help:

- **Past run history** ("has anyone applied this transaction
  before on a similar host? what happened?") — answerable from
  `~/.mooncake/runs/` but not from the schema.
- **Drift findings** (`docs-working/arch-report/`, the manual-test
  finding sub-files) — markdown that records *what we learned the
  hard way*, not part of the typed action surface.
- **Spec rationale / non-goal receipts** — `non_goals.md`,
  `kernel.md`, the issue-10-analysis — the *why* an agent needs to
  not propose something. `mooncake explain pkg.install` won't
  surface "and by the way, don't ask for a plugin marketplace; here's
  the receipt."
- **Recent decisions** — last 30 days of merged PRs, what changed,
  what was rejected. The audit lives in git; the *summary an agent
  can consume in 5kb of context* doesn't.

This residue is exactly RAG-shaped. So I'd ship **both**:

- **Tier 1 (Architect-led, ship first):** `mooncake explain` + MCP
  tool exposure. Typed answers for the typed parts. Zero config.
- **Tier 2 (later, optional):** a `repo.index` *fact producer*
  (not a generic RAG engine — the Architect was right to refuse
  that). The action surface is closed and tiny: one verb that
  walks specified paths and emits structured facts (file paths,
  excerpt windows around keyword hits, embedding hashes). The
  agent queries the fact, not a vector store. Three providers
  max for embeddings. Closed action family. Non-goal #2 holds.

The honest framing: **the meta-search prompt fails as a category
but the agent-help-residue it pointed at is real.** Two solutions,
clean separation: `explain` for typed knowledge, `facts.repo_index`
for everything else.

If forced to pick one, take `mooncake explain` first. The residue
isn't urgent until the typed half is exhausted.

### D. Pushing at the non-goals — where each one pinches

Quick pass; the Architect covered most of this implicitly. Naming
the remaining pinch points so the Startaper can adjudicate.

- **Non-goal #1 (no DSL evolution)** — Skill `when-to-use` matchers
  could grow into a matcher language. *Pinch resolved*: lives in
  MCP tool description text, not mooncake YAML.
- **Non-goal #2 (no provider marketplace)** — `index.*` /
  `facts.repo_index` providers (local / openai / anthropic) could
  bloat. *Pinch*: keep the provider set closed at three, expand
  via normal spec path.
- **Non-goal #3 (no control-plane sprawl)** — "fleet-wide RAG
  index" would beg for a controller. *Pinch resolved*: per-peer
  index; fleet-wide search is a read-only aggregation rendering.
- **Non-goal #4 (no git-coupled audit)** — Sigstore/Rekor is a
  transparency log. *Pinch resolved*: Rekor is evidence, not
  database. Local runlog stays authoritative; Rekor entries are
  pinned references *from* the local log.
- **Non-goal #6 (no ACID claims)** — Provenance + signed
  transactions risks reading as atomic guarantees. *Pinch*: lean
  into the SAGA framing in surface text. "Compensators ran cleanly,
  3 of 4 steps reversed; step 4 was declared irreversible by the
  handler." The kernel doc does this; the rendering should too.
- **Non-goal #7 (no pipeline DSL)** — the Makefile prompt pulls
  hardest here. *Pinch*: hold the discipline named in §C.1.

### E. Synthesis-by-priority — what I'd actually pull on

If forced to pick three things for the next 90 days:

1. **`mooncake explain` + MCP exposure.** Cheapest, kernel-aligned,
   immediately useful to the lighthouse agent-dev. The Architect's
   reframe of prompt #2 is correct; ship it.
2. **`mooncake-vscode` / Zed extension that renders typed Diff
   inline.** The highest-leverage way to teach more people what the
   kernel claim is. No new kernel work; just a rendering that the
   kernel can already power.
3. **A `docs-next/decision-2026-fifth-property.md` deciding
   Provenance vs Determinism vs neither.** Don't ship either yet;
   decide on paper which is the load-bearing argument and what
   evidence would tip it. The Architect and I disagree, and the
   right thing to do with that disagreement is *document the
   adjudication criteria*, not pick a winner today.

Explicitly *not* prioritizing:

- Full Mooncake-as-Makefile *replacement*. The dogfood demo
  (Architect's framing) is the right scope; the replacement is a
  non-goal trap.
- Cluster-wide RAG. Per-peer first; aggregation later if asked.
- A Skills-format YAML field. The agent's `description:` string is
  already where this lives.

### F. Five sharp questions for the Architect and the Startaper

For the **Architect**:

1. **Provenance vs Determinism — what's the disagreement decision
   procedure?** I think we're proposing different properties for
   different reasons, both defensible. What evidence would tip a
   real reviewer toward one or the other? My instinct: count the
   real user requests over the next 90 days that need step-granular
   signing vs deterministic replay. Yours?

2. **Does the jj-style oplog actually need to be separate from
   the runlog, or is the right answer to extend runlog records
   with an `op_id` correlation field?** I argued for separation in
   §A.4; you implied (by listing it under "interface choice to
   steal") it might be more interface than data structure. Push
   on me — what does the simpler version cost us?

3. **If we ship `facts.repo_index` (Tier 2 in §C.2), does it
   generalize?** Every "state your project depends on" — lockfiles,
   cached images, generated code — could become a typed fact
   producer with Diff/Reverse. Is that a clean extension of the
   facts primitive, or an action-family explosion that violates
   Non-goal #2 in spirit?

For the **Startaper**:

4. **Which lighthouse case study lands first — solo-dev dotfiles
   (six weeks to ship a real artifact) or AI-agent integration
   (six months, but it's the headline `goals.md` promises)?** The
   Architect already asked you something like this. My read of the
   adoption funnel says the dotfiles dev gets to a published case
   study faster *and* unblocks the agent-dev case study by giving
   them something to point at. Defend the opposite or confirm.

5. **`mooncake-vscode` extension or `mooncake explain` first?**
   Both are cheap; both teach the kernel claim. The extension has
   a higher per-impression payoff (you see typed Diff in the
   gutter; you *get it* viscerally) but `mooncake explain` lands
   immediately for the agent-dev wedge. Which one's the better
   first-90-days tweet?

### G. One thing I'm hedging on

The Provenance bet (§B) is the sharpest move in this whole pass —
*if* it survives review. The honest reading is that "rendering on
top of plan_hash + Sigstore" might be the right shipping shape
for the next 12 months even if Provenance ultimately earns a
kernel column. The kernel doc explicitly warns against
*speculating* a property into existence; that applies to me too.

The thing I'd refuse to do is *not* propose it. The kernel claim
gets sharper from arguments tried-and-rejected, not from
arguments-never-made.

---

## Startaper's pass

I've read both prior passes. The Architect and Innovator just spent
~25kb arguing about Provenance vs Determinism as a fifth typed
property. I am here to say: **neither of those ships a single user
in the next 90 days.** This pass is the founder's filter, not the
fifth voice in a kernel-design seminar.

I'll be direct. We are not "underbuilt." We are *underdistributed*.
The kernel doc itself says it: "the next bottleneck is adoption, not
engineering." The 2026-05-15 honest snapshot lists 40+ shipped
actions, three production-grade safe-agent primitives, a working
personal fleet, an MCP server, and an agent loop. We have a category
killer with no category yet.

So the only question that matters this brainstorm: **what is the
narrowest possible bet that produces one viral artifact in 90 days,
and what do we cut to make room for it?**

### S.1 The single user whose tweet matters most

Not "an agent-dev power user." Not "a Cursor influencer." Concretely:
**someone who already publishes content about letting Claude/Codex
touch their actual prod, and who has been visibly burned by it.**

The shape of the audience:
- They livestream / blog / post screenshots of agent runs.
- They've already said the words "and then it deleted my X" at
  least once in public.
- They have a Mastodon / X / YouTube / Substack reach in the
  10k–100k range — small enough to engage personally, large enough
  to seed a wave.
- They are not at Anthropic, OpenAI, or Google. Vendor employees
  posting "look at our cool tool" reads as marketing; an
  independent posting "this saved my Saturday" does not.

I will not name handles in a public doc. The internal short-list
should be three people, and the GTM motion is: **cold-DM all three
with a 60-second Loom of the rollback demo, offer to walk one of
them through wiring it into their actual setup.** That's it. That's
the funnel.

The tweet that lands — paraphrased, falsifiable — is:

> "Let Claude run `apt install` and `systemctl` on my dev box for
> three weeks. Yesterday it tried to overwrite my nginx config
> mid-request. Mooncake reversed the transaction in 2 seconds.
> Audit trail is JSON. Going to keep using this."

That tweet is *worth more than the entire Stream 5 ecosystem
roadmap*. The Architect's "single Cursor power user rewrites
dotfiles on livestream" is in the right direction but the wrong
artifact. **Dotfiles are not viral in 2026.** Nix, chezmoi, devbox,
and `gh repo sync` ate that surface. Rollback-of-a-real-agent-
mistake is novel and *only* Mooncake ships it.

### S.2 The 30-second demo that makes an agent dev install

Not the docs page. Not the README. The Loom. The screen recording.
The asciinema. **30 seconds, three beats:**

1. **Beat 1 (0–8s)** — Cursor / Claude Code emits a `transaction:`
   block. The viewer sees typed YAML, not shell. Already novel.
2. **Beat 2 (8–18s)** — `mooncake plan --diff` renders the typed
   Diff. Viewer sees *structured* changes per resource. Step 3
   intentionally fails on apply.
3. **Beat 3 (18–30s)** — auto-revert. Steps 1 and 2 reverse. A
   final `mooncake history --json` pipe to `jq` shows the audit
   trail. Cut. Done.

Cost to produce: one afternoon. Asset reuse: this is the README's
hero GIF, the Hacker News submission's first reply, the
conference-talk opening, the cold-DM attachment. **The 30-second
demo is the unit economics of the next 90 days.** Stop arguing
about kernel columns; ship the Loom.

The Innovator's `mooncake-vscode` extension is the *right second
artifact* — it teaches viscerally what typed Diff means. But it's
a four-week build minimum, and we haven't shipped the 30-second
artifact yet. **Loom first. Extension second.** Don't invert.

### S.3 Which wedge funds the next in 2026

Three wedges in `VISION.md`: solo / agent-dev / platform.

The Innovator's case for solo-dev-first ("six weeks to a real
artifact, unblocks agent-dev case studies") is operationally
correct *and strategically wrong*. Solo dev is the **funnel**, not
a wedge. We do not monetize solo devs. We do not even instrument
them — `goals.md §1` explicitly says "no telemetry."

The wedge that funds 2027 is **agent-dev**, full stop. Here's the
flow:

```
agent-dev case studies (free, narrative-led)
     │
     ▼
foundation-model integration (Anthropic / OpenAI / Cursor
     embed MCP server in default tool list)
     │
     ▼
platform-team inbound ("our devs are using this thing called
     Mooncake with Claude, can we audit it?")
     │
     ▼
control-plane sale ($)
```

Solo dev sits *upstream* of agent-dev as the on-ramp surface, not
parallel to it. Platform sits *downstream* of agent-dev as the
monetization layer. **One funnel. Not three wedges.** The
"three-wedges" framing in VISION § 4 was a useful planning fiction;
in 2026 it should be retired in external communication.

### S.4 The one cut I'd make to focus

**Cut all engineering work on the enterprise hub (L4) until two
written agent-dev case studies exist.** Not "deferred" — *explicitly
forbidden*. The current `goals.md §4` says "intentionally deferred
until a paying user asks." That is not strong enough. "Deferred"
attracts spec-shaped optimism every six weeks. **Forbidden** kills
it dead.

Side effect: kills any temptation to invent RBAC schemas, audit-
export formats, or Slack-approval flows in advance of a paying
customer. Every hour spent on those is an hour not spent making
the rollback Loom or DM'ing a lighthouse user.

Second cut, smaller: the **fifth-property debate** (Provenance vs
Determinism). The Architect and Innovator both made defensible
cases. **Neither ships in 2026.** Put it in
`docs-next/decision-2026-fifth-property.md` as the Innovator
proposed; revisit Q4 *only* after the rollback demo has produced at
least one signed-audit feature request from a real user. Don't
spend more brainstorm cycles on it before then.

### S.5 The two creative prompts, GTM filter

**Prompt 1 — Mooncake replaces Makefile in its own repo.**

The Architect says "demo artifact." The Innovator says "demo + CI
gate." Both right; both miss the GTM use.

The Makefile-replacement is **content marketing**, not a feature
and not a wedge. It is:

- A *blog post* titled "We replaced our Makefile with our own
  tool. Here's what broke and what didn't." Hacker News loves
  this shape: meta, technical, slightly self-deprecating.
- An *example in the repo* — `examples/dogfood/mooncake.yml` —
  that any visitor can read in 60 seconds and walk away knowing
  what Mooncake does.
- A *credibility check* — when an agent dev clones the repo and
  sees we use our own thing, the doubt-budget shrinks.

It is **not the headline demo.** The headline is the rollback Loom
(§S.2). The dogfood Makefile is the *second* exhibit a visiting
agent-dev looks at, not the first.

**Verdict:** ship it. Make it a release blocker on the next minor.
But don't pitch it externally as a category — pitch it as a flex.
"We replaced our Makefile with our own thing" is not a value prop;
it's a credibility signal.

**Prompt 2 — Meta-search / agent-help in a few lines of mooncake.**

The Architect reframed as `mooncake explain`. The Innovator added
`facts.repo_index` for the residue. Both are correct as *features*.

But the **GTM frame is different**: the lighthouse user does not
ask for "search." They ask for **"why does my agent keep being
useless in this codebase?"** The answer they want isn't a CLI verb.
It's an experience: *agents that open this repo are immediately
oriented.*

Reframe the deliverable as: **"Mooncake's repo memory for
agents"**. The implementation is `mooncake explain` + MCP exposure
+ a `repo_index` fact (the Innovator's Tier 2). The marketing is:

> "Every Claude/Cursor/Codex session that opens a mooncake-managed
> repo gets the action schemas, the apply history, and the recent
> decisions, automatically. Your agent stops being a noob in your
> codebase the first time you `mooncake init`."

That is a tweet. "We added an `explain` verb" is not a tweet.

The five-lines-of-config the user imagined collapse into one:
`mooncake.yml` declaring this is a mooncake-managed repo. The
agent's MCP layer picks up the rest. **The product is the
behavior, not the verb.**

### S.6 What I'd put on the calendar

Eight weeks from today (2026-05-16 → 2026-07-11):

| Week | Ship |
|---|---|
| 1 | Rollback Loom v1. Record, edit, post on personal Twitter. |
| 2 | `examples/dogfood/mooncake.yml` lands. Blog post draft. |
| 3 | `mooncake explain <noun>` + MCP tool exposure. |
| 4 | Cold-DM three lighthouse candidates with the Loom. |
| 5 | First case-study draft (assuming one DM converts). |
| 6 | `mooncake-vscode` extension prototype (Innovator's pick). |
| 7 | HN submission with the dogfood Makefile post. |
| 8 | Second case-study draft + first agent-vendor outreach. |

Eight weeks. Three published artifacts (Loom, blog, case study).
One extension prototype. **Zero hours on the enterprise hub.**
**Zero hours on the fifth-property debate.**

If we hit even four of these, we'll know whether the agent-dev
wedge converts. If we miss most of them, we'll know the GTM
hypothesis is wrong and we should revisit the wedge ordering —
*with evidence*, not arguments.

### S.7 What I refuse to do in this brainstorm

- **Argue features.** The Architect and Innovator already did the
  feature work. My job is to point at the door, not redesign the
  house.
- **Hedge.** "Both wedges, in parallel" is the founder-mode
  failure pattern. Pick one funnel; cut the rest until it works.
- **Speculate the kernel column.** Innovator is right that
  arguments-never-made don't sharpen the claim — but
  arguments-made-prematurely-in-brainstorms do not ship users.
  Defer the property debate until a user asks for it.
- **Pretend the personal-fleet is the wedge.** It's a beautiful
  piece of engineering and a non-wedge from a GTM standpoint. It
  is the *story* (single-binary, peer-to-peer, no SaaS) that makes
  the rollback demo *believable*, but no one buys "personal fleet
  manager." We use it as proof-of-engineering-quality, not as the
  pitch.

### S.8 Three sharp questions for the Architect and Innovator

**For Architect:**

1. **Is the fifth-property debate a 2026 conversation or a 2028
   conversation?** I claim 2028. If neither Provenance nor
   Determinism ships before Q4 2026, why are we spending
   brainstorm cycles on it during a session whose stated job is
   GTM? Push back: name the customer-facing decision in the next
   six months that hinges on resolving it.

2. **`examples/dogfood/mooncake.yml` as a release blocker on the
   next minor — yes or no?** You called it a "launch artifact"
   and asked the Startaper to sell you on the other two readings.
   I'm not selling you on either. I'm asking you to commit it to
   the release timeline as the next minor's hero example. Yes
   or no, in writing, this pass.

**For Innovator:**

3. **IDE extension vs `mooncake explain` — you presented them as
   parallel "rendering" bets in §A.10 and §E. They have wildly
   different fan-out.** `mooncake explain` is a feature *every*
   MCP-aware agent gets the moment we ship it (Cursor, Claude
   Code, Codex, Zed). The VSCode extension teaches one IDE's
   users at a time. Why isn't the MCP version strictly more
   leveraged, and why would we ever ship the extension first?

**For both:**

4. **Name the lighthouse user. Concretely.** Not a persona — a
   short-list of three specific people we'd cold-DM tomorrow. We
   don't have to put names in this doc, but the internal answer
   should exist *before* this brainstorm closes. If neither of
   you has a name in mind, that is itself the most important
   finding of this exercise.

5. **What is the kill criterion for the agent-dev wedge?** I'm
   proposing 8 weeks to three artifacts and one converted DM. If
   we hit none of those, what's the project's next move? If your
   honest answer is "double down" no matter what, we don't have a
   wedge — we have a thesis we can't falsify. Name the falsifier
   now, when it costs nothing.

### S.9 The one sentence I'd put on the homepage tomorrow

The Architect's kernel-doc one-sentence is correct for *the
substrate*: "Mooncake is a typed mutation kernel." That sentence
is what we tell ourselves.

What we tell the user is different:

> **"Mooncake is the rollback button for AI agents touching your
> system."**

That's the homepage. That's the tweet. That's the cold-DM. It's
narrower than the kernel claim, sells one wedge cleanly, and
maps 1:1 to the 30-second demo. Every other framing —
"declarative config," "execution layer," "Docker for AI agents"
— is for a different audience that we are *not* selling to in
2026.

Lead with the rollback. Earn the right to widen the claim later.

---

## Synthesis

Architect-led, post-all-three-passes. Three voices, three live
disagreements, and one surprise: where the GTM filter (Startaper)
disagreed with both kernel-shaped passes (mine + Innovator's), the
GTM filter was right. Naming that up-front because it's the most
important thing this brainstorm produced.

### Where all three agree (no adjudication needed)

These ship; the disagreement is on framing, not substance.

1. **`mooncake explain <noun>` + MCP tool exposure.** Architect
   proposed; Innovator extended; Startaper accepted and reframed
   the marketing as "repo memory for agents." Implementation:
   typed schema + applicable examples + Diff/Reverse shape + spec
   origin, served from `schema.json` + handler metadata. Zero
   operator config. Lands as a spec.

2. **`examples/dogfood/mooncake.yml`.** Architect: demo. Innovator:
   demo + CI gate. Startaper: content marketing + credibility
   flex. All three say ship. The discipline is unanimous: no new
   YAML keywords for the dogfood's sake; if our own Makefile
   needs `matrix:` or inline expressions to translate, the
   translation is wrong, not the rails. **Committed in writing
   per Startaper's S.8 Q2: release blocker on the next minor.**

3. **The R-series frontend refactor stays the top engineering
   priority.** Kernel doc §"The frontends" calls out the
   duplication; the R-series in flight is the right fix; no
   brainstorm output should propose a sixth frontend before the
   existing five share the same `Apply` / `FleetApply` entry
   point. All three passes implicitly assumed this.

4. **The seven non-goals hold.** Every persona pushed at them and
   each push resolved inside the rails (Skill matchers live in
   MCP description text, not YAML; `repo_index` providers stay
   closed at three; Sigstore/Rekor is evidence, not database;
   pipeline DSL stays refused).

### Disagreement 1 — Determinism vs Provenance vs neither

- **Architect (me):** Determinism, fifth typed column. Argued the
  replay/cache/signing story is theater without it.
- **Innovator:** Provenance instead. Argued Determinism collapses
  into a sharper Diff; Provenance answers an orthogonal axis.
- **Startaper:** **Neither ships in 2026.** Asked Architect to
  name one customer-facing decision in the next six months that
  hinges on it.

**Adjudication.** Startaper wins on shipping order. **My answer
to S.8 Q1:** I cannot name a customer-facing decision in the next
six months that hinges on resolving fifth-property. Plan-signing
works on `plan_hash + Sigstore + actor` as a *rendering* for the
12-month horizon. The fifth-property debate is a 2028
conversation, not a 2026 one.

**But the disagreement is still load-bearing for the decision
doc.** When we revisit (Q4 2026 at earliest, gated on real user
asks), the framing should be: **Innovator's Provenance vs
Architect's Determinism vs neither**, with the kill criterion
being "did any user in the last six months ask for step-granular
signed audit *or* for byte-identical replay across registry
shifts?"

**Spec action:** `docs-next/decision-2026-fifth-property.md`
captures both arguments, the kill criterion, and Q4 2026 as the
earliest review date. No code work.

### Disagreement 2 — Wedge ordering: solo vs agent-dev

- **Innovator:** solo-dev-first; the dotfiles case study unblocks
  the agent-dev case study.
- **Startaper:** agent-dev is the only wedge; solo is the
  *funnel* upstream; platform is downstream. Three wedges in
  VISION §4 was a useful planning fiction that should retire in
  external comms.

**Adjudication.** Startaper wins. The argument that decided it:
dotfiles-as-viral-artifact died sometime around 2024 (Nix /
chezmoi / devbox / `gh repo sync` ate that surface). Mooncake's
*only* unique-in-2026 wedge is auto-reverted typed
mutations-from-an-agent. Solo dev gets the on-ramp UX
(`mooncake init`, drift detection, multi-machine sync), but it
does not get a case study or a marketing budget.

**Doc action:** VISION §4 stays as the internal architecture
framing (still useful for "one engine, three audiences"). The
*external* one-sentence is Startaper's (S.9, see Disagreement 3).
This is the second-time-this-doc that "internal canonical
language ≠ external canonical language" — register the pattern
explicitly in the synthesis output (below).

### Disagreement 3 — One sentence on the homepage

- **Architect / kernel doc:** "Mooncake is a typed mutation
  kernel."
- **Startaper:** "Mooncake is the rollback button for AI agents
  touching your system."

**Adjudication.** **Both, with explicit scopes.**

- Kernel doc's sentence is canonical *for contributors,
  reviewers, and anyone writing a spec preamble*. The R2 risk
  in `kernel.md` (narrative fragmentation) named precisely this
  hazard: every doc inventing its own framing. Internal docs
  defer to the kernel claim.
- Startaper's sentence is canonical *for the homepage, the README
  hero, the cold-DM, the Loom voiceover, the HN title*. External
  framing leads with one wedge; widens later.

The two-sentence discipline is itself a deliverable: a one-page
`docs-working/positioning.md` that pins *which sentence lives
where* and forbids the "let me invent a third framing for this
talk" failure mode. **Cheap; ship it inside the 8-week window.**

### Disagreement 4 — Sequencing inside the 8-week window

Innovator put `mooncake-vscode` extension as a 90-day top-3 pick.
Startaper said **Loom first, extension second** (week 6 at
earliest). Startaper's argument: a 4-week extension build cannot
precede a 1-afternoon Loom, regardless of per-impression payoff.

**Answer to S.8 Q3 (Innovator's IDE-extension-vs-explain
fan-out):** `mooncake explain` + MCP wins on leverage. Every MCP-
aware agent (Claude Code, Cursor, Codex, Zed) gets it the moment
we ship it. The VSCode extension teaches one IDE's users at a
time and is 4× the build. The fan-out asymmetry resolves
Startaper's challenge cleanly: explain first (week 3), extension
later (week 6 prototype, optional).

**Adjudication.** Startaper's calendar (S.6) is the right one.
Architect/Innovator should not propose reordering until at least
two of the eight weeks have shipped concretely.

### The Architect's open questions answered

**(Innovator Q2, runlog vs separate oplog):** Extend runlog
records with an `op_id` correlation field first. The two-log
split is what jj needed *because* jj's mutations are commits and
its operations are commands-on-commits — two genuinely different
domains. In mooncake, runs *are* the typed-mutation domain and
operations are commands that triggered runs; the correlation
field on the existing record is sufficient until a user wants
"show me every `replay` I've ever run" as a first-class query.
Don't pre-split.

**(Innovator Q3, does `facts.repo_index` generalize?):** Yes,
into a *closed* family — `facts.lockfile`, `facts.cached_images`,
`facts.generated_code` — added one at a time via the normal spec
path with explicit per-source typing. **Refuse** the generic
"fact provider plugin" shape that would let third parties ship
arbitrary fact emitters. The closed-family discipline is what
keeps non-goal #2 intact while letting the genuinely useful
shapes accumulate.

**(Startaper Q4, name the lighthouse user concretely):** *We
don't have a name yet*. Both Innovator and I described persona
shapes, not handles. Per Startaper's framing, **this is the most
important finding of this brainstorm.** The internal shortlist
of three concrete handles must exist before week 1 of the
8-week calendar, or the calendar is just choreography.

**(Startaper Q5, kill criterion for the agent-dev wedge):** 8
weeks, 3 published artifacts (Loom, dogfood blog, first case
study), 1 converted lighthouse DM. If we hit none: revisit
wedge ordering *with evidence*, not arguments. Falsifier accepted
on the record.

### What turns into a spec (vs. stays in this doc)

| Item | Becomes | Owner |
|---|---|---|
| `mooncake explain <noun>` + MCP `explain` tool | Spec (`docs-working/streams/core/specs/spec-XX-explain.md`) | Architect |
| `examples/dogfood/mooncake.yml` | Release-blocker example, no spec needed | Whoever takes the next minor |
| `docs-next/decision-2026-fifth-property.md` | Decision doc (no code) — Provenance vs Determinism vs neither, with Q4 2026 review + kill criterion | Architect |
| `docs-working/positioning.md` | Two-sentence pin (internal vs external), no code | Architect |
| `facts.repo_index` Tier 2 | Spec, gated on `explain` shipping first | Innovator follow-up |
| `mooncake rehearse <plan>` (OrbStack/Lima) | Stays in this doc; revisit when an agent-dev case study asks for it | — |
| `mooncake-vscode` / Zed extension | Prototype only, week 6 of the calendar | Innovator if motivated |
| OTel rendering of runlog | Stays in this doc; revisit when first paying user asks | — |
| Enterprise hub (L4) | **Forbidden** until two written agent-dev case studies exist | — |
| Fifth-property kernel column | **Forbidden in 2026**; see decision doc | — |

### The pattern this brainstorm exposed

Two of the three personas (Architect and Innovator) optimized
for *kernel sharpness*. The third (Startaper) optimized for
*ships-in-90-days*. **When they disagreed, the third was right.**

This is not a verdict on the personas; it's a verdict on what
this project's actual bottleneck is right now. The kernel doc
already named it ("the next bottleneck is adoption, not
engineering"). The brainstorm corroborated it. The next four
months should be 70% GTM motion + 30% kernel maintenance, not
the reverse.

When the lighthouse-user funnel produces evidence — case studies,
real user requests, signed audit asks — the ratio flips back
toward kernel work. *That* is the right time to revisit the
fifth-property debate, the WASM-plugin question, the
control-plane sale, and every other deferred call in this doc.

Not before.

### Open items the synthesis explicitly does not close

These are real and named so a future read of this doc doesn't
treat them as resolved.

- **Lighthouse-user shortlist (3 concrete handles).** Highest-
  priority gap; blocks week 4 of the calendar.
- **Whether the kernel doc's R2 risk (narrative fragmentation) is
  fully covered by `docs-working/positioning.md`.** Probably not
  — talks, READMEs in other repos, conference abstracts will
  drift regardless. Mitigation, not solution.
- **What the second wedge looks like after agent-dev converts.**
  Startaper said platform downstream; the brainstorm didn't
  pressure-test that claim. Worth a separate session in Q4 2026
  if the 8-week calendar produces evidence.
- **Whether `mooncake apply ci.yml` (Innovator's CI-gate framing)
  earns the keystrokes for *external* projects.** The dogfood
  use is settled (yes). Whether `mooncake.yml` should aspire to
  replace `make` for repos that don't ship Mooncake itself is
  open and probably the wrong question for now.
