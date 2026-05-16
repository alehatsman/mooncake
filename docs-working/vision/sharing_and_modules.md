# Sharing configurations and the module system

A first-principles think about the distribution layer Mooncake needs.

The in-tree `presets/` directory is dropped. The replacement is not "a
bigger in-tree presets library" — it is **distribution as a primitive**.

Related GitHub issue: [#24 — "Replace large built-in preset library
with versioned Git-based module system"](https://github.com/alehatsman/mooncake/issues/24).

---

## Three audiences want to share configurations

The design has to serve all three. They have different needs.

### 1. Humans sharing dotfiles / setups

> "I want my postgres config on every machine I own, plus on my
> teammate's box when he asks."

Today: copy YAML into someone's repo, hope it works.

What this audience needs:
- A way to reference a config by URL and pin its version.
- Parameter overrides without forking ("install postgres but turn TLS
  on for me").
- Trust: a checksum so the config doesn't change under them silently.
- Optional curation: somewhere to look for "official" postgres / ollama
  / nvidia setups.

### 2. Agents composing modules

> An LLM is given "set up postgres with TLS on this host." It should
> *find* a postgres module, *understand its parameters from the typed
> schema*, and *emit a plan that uses it* — not write postgres-install
> shell scripts from scratch.

What this audience needs:
- Typed parameter schemas it can reflect on. (Already shipped per
  preset.)
- Discoverability — a way to enumerate "what postgres modules exist?"
  with their parameters + last-applied checksum.
- Plan-mode dry-run on whatever module it finds, before commitment.
- The same `Reverse()` / transactional guarantees inherited from the
  module's underlying actions.
- Provenance: who/what wrote this module, with what prompt, when.

### 3. Agents *producing* modules

> An agent that has already converged a system via ad-hoc shell-and-YAML
> should be able to *promote* its work into a reusable module. The
> module is then shareable to the agent's other deployments, or to the
> human's other machines, or to other agents.

What this audience needs:
- A clean shape for "this is a publishable module" — same shape humans
  write by hand. Parameters declared at the top, steps below.
- A scaffold command: `mooncake mod init` writes the skeleton.
- A publish step that is just `git push --tags` — no central registry,
  no API key, no review queue. Anyone can publish a module the same way
  anyone can publish a Go module today.

**The combined property the design has to land:** the same artifact
shape that an LLM produces, a human writes by hand, and another LLM
consumes downstream. No code is generated inside the module — only
typed YAML. That keeps the module surface inspectable end-to-end.

---

## What Mooncake gets if it adopts the Go-module shape verbatim

Go's module system is the most successful decentralized package
distribution model of the last decade. The properties worth stealing:

| Property | Why it matters here |
|---|---|
| Import path = `github.com/owner/repo/subpath` | Identity without a central registry. The Git URL *is* the namespace. |
| Versions = Git tags + semver | Independent of Mooncake's release cycle. A module can ship v2.0.0 while Mooncake is still on v0.9. |
| Pseudo-versions (`v0.0.0-<ts>-<sha>`) | Pin to any commit before a tag exists. Critical for agents that want to point at "exactly what worked." |
| `go.sum`-style checksum lockfile | Supply-chain integrity for free. If a module changes under you, the next `mod verify` fails loudly. |
| Module proxy (`proxy.golang.org`) | We may be able to **reuse Go's actual proxy infrastructure** by making each module a valid Go module with a `go.mod` that just declares the path. The proxy doesn't care that the content is YAML. Free CDN + transparent caching. |
| Vendor mode | Commit the resolved module tree into the playbook repo. Offline-first stays workable. |
| Push tag = release | No publish API. Strictly simpler than npm / Galaxy / Terraform registry. |

The non-goal we're *not* violating: this is not a plugin marketplace.
Modules ship typed YAML data, not loadable code. The non_goals.md §2
carve-out ("out-of-tree integrations are separate tools that produce
Mooncake YAML, not plugins inside the runtime") was written precisely
for this shape.

---

## What changes from the dropped `presets/` directory

| Concept | Old (`presets/`) | New (module system) |
|---|---|---|
| Identity | Directory name (`docker`, `ollama`) | Full Git URL (`github.com/owner/repo/path`) |
| Version | Whatever's in the Mooncake binary right now | Git tag, lockfile-pinned per playbook |
| Update | Bump Mooncake version, hope no regressions | `mooncake mod tidy` updates the lockfile; old plays keep working until you ask for the new version |
| Discovery | `mooncake presets list` (in-tree only) | `mooncake mod search <query>` — initially just GitHub search, later optional curated index |
| Forking | Fork all of Mooncake | Fork the module repo (5-line change) |
| Quality | We owned the quality bar for all 21 in-tree presets | Module author owns their module's quality; the lockfile mechanism protects the consumer |
| Trust | Implicit (it shipped with the binary) | Explicit (checksum + signed tags later) |
| Authorship | Mooncake committers only | Anyone with a GitHub repo (or any Git host) |

The `presets/` content carried weight that almost no production user
needed. Dropping it now is reversible (the deletion is in `git log`)
and frees us to design the distribution layer without backward-compat
weight.

---

## What the playbook will look like

```yaml
# mooncake.yml — the user's playbook
modules:
  postgres: github.com/mooncake-modules/postgres v1.3.0
  ollama:   github.com/alice/dotfiles-modules/ollama v0.2.1

steps:
  - use: postgres
    with:
      tls: true
      replication: true
  - use: ollama
    with:
      models: [llama3.1:8b, mistral:7b]
```

```yaml
# mooncake.lock — committed to repo, generated by `mooncake mod tidy`
modules:
  github.com/mooncake-modules/postgres:
    version: v1.3.0
    commit: a1b2c3d
    h1: sha256:abcd…   # hash of resolved module zip
  github.com/alice/dotfiles-modules/ollama:
    version: v0.2.1
    commit: 9f8e7d6
    h1: sha256:wxyz…
```

The module repo:

```
github.com/mooncake-modules/postgres/
├── module.yml               # parameters schema + step skeleton
├── tasks/
│   ├── install.yml
│   ├── configure.yml
│   └── tls.yml              # included when parameters.tls = true
└── templates/
    └── postgresql.conf.j2
```

---

## What an agent-generated module looks like

The LLM produces *exactly the same artifact shape*. The pattern:

1. Agent converges a system via inline `mooncake apply`.
2. Operator says "promote this to a reusable module."
3. Agent runs `mooncake mod init <name>`, extracts the steps it just
   ran, parameterizes the values that diverged across hosts, writes
   `module.yml`.
4. Agent commits + pushes + tags `v0.1.0`.
5. The next agent that needs the same setup finds the module via
   search or by reading the operator's previous playbook lockfile.

What makes this trustable:
- Module YAML is still data, not code. The `module.yml` declares
  parameters with types; the steps use Mooncake's typed action
  vocabulary. Nothing executes outside the typed ABI.
- The lockfile commits `h1:sha256:…` of the resolved module tree —
  the consuming agent can re-verify before applying.
- Plan-mode runs the module without mutation. The consuming agent
  sees the rendered diff before committing.
- The four-method ABI (Permissions / Diff / Reverse / Cost) flows
  through whatever the module composes. A module that wraps
  `pkg.install` inherits its reversibility and risk band.

The combined effect: **even modules an agent wrote can be safely
consumed by another agent**, because the typed surface plus
plan-mode plus the lockfile gives the consumer a high-confidence
preview.

---

## Strategic implications

### What this enables that the in-tree library couldn't

- **`mooncake share <preset>` UX** — `goals.md §1`'s last 5% gap.
  Sharing reduces to `git push && git tag`.
- **Agent-to-agent module exchange** — turns Mooncake into the typed
  exchange medium between agents, not just a target one agent drives.
- **Operator-owned forks** — every team can have their own
  `internal-modules/` repo without negotiating with us.
- **Honest preset count** — the binary stops claiming "330+ built-in
  presets" (which was always overstated). The new claim is "Git-native
  module distribution; here are X modules we curate at
  `github.com/mooncake-modules/*`."

### Non-goals this stays inside

- **No central registry.** Identity is `github.com/...`. We never
  operate the registry. If we eventually run a curated index
  (`mod.mooncake.io`), it's a discovery surface over GitHub search,
  not the source of truth.
- **No plugin runtime.** Modules ship typed YAML, not code. The same
  schema validation that runs over `presets/*/preset.yml` today runs
  over every module before its steps reach the executor.
- **No dependency graph between modules in v1.** A module can use
  `pkg.install`, `file.write`, `text.line` — the typed action
  vocabulary — but it cannot declare `requires: another-module`. If a
  user wants composition, they use multiple `use:` blocks in their own
  playbook. This keeps us out of npm-style dependency hell.
- **No mandatory module-author quality bar.** Anyone publishes
  anything. Quality is enforced by the lockfile (consumer chooses
  what to trust) and by curation of `mooncake-modules/*` (we run that
  org for the modules we vouch for; community modules carry implicit
  "use at your own risk").

### Phasing

| Phase | What ships | Risk |
|---|---|---|
| 1. Loader + cache | `use: github.com/foo/bar@v1.0.0` resolves, fetches, caches in `~/.cache/mooncake/modules/`, loads as a preset | low — adds a new source to existing loader |
| 2. Lockfile + checksums | `mooncake mod tidy` writes `mooncake.lock`; subsequent loads verify h1 hashes | low — pure data layer |
| 3. Curated `mooncake-modules/*` org | Migrate the dropped presets here as v1.0.0 of each module | low — we control the repo |
| 4. Pseudo-versions + vendor mode | Pin to commit hash; `mooncake mod vendor` copies resolved tree into repo | medium — vendor mode interacts with playbook layout |
| 5. Signed tags / supply-chain hardening | Sigstore-style cosign on module tags; `mooncake mod verify --signed` | future — gated on real demand |

### What we explicitly defer

- **Discovery service / search index.** GitHub's search is "good enough"
  for v1. A curated `mod.mooncake.io` index can come later if community
  modules accumulate.
- **Private Git support.** Important for corporate use but not v1.
  Works via SSH-authenticated Git URLs once we wire the fetcher to use
  the operator's existing SSH config; this is mostly mechanical.
- **Cross-module dependency.** Stays a hard `no` until a real user
  hits the wall.

---

## Open questions worth pulling on

1. **Should we reuse `proxy.golang.org` directly, or implement our own
   fetcher?** Reusing is free + battle-tested but couples us to Go's
   proxy behavior (which is good but has its own quirks). Implementing
   ourselves is ~500 LOC and gives total control. Probably reuse first,
   reserve the option to fork the fetcher later.

2. **What's the right namespace for the curated module org?**
   `github.com/mooncake-modules/*` (one repo per module) vs
   `github.com/alehatsman/mooncake/modules/*` (subdirs in this repo).
   Subdir approach wastes the Git-tag-per-module property; separate
   repos preserve it. Probably separate repos under
   `github.com/mooncake-modules/`.

3. **How do modules declare which Mooncake versions they support?**
   A `mooncake_version: ">=0.9.0"` field in `module.yml`. The loader
   refuses incompatible modules with a clear error. Cheap to add.

4. **Should the lockfile be `mooncake.lock` or `mooncake.lock.yaml`?**
   YAML extension makes it greppable + opens the file in YAML editors.
   `go.sum` chose extensionless but textual — either works. Probably
   extensionless to match the format precedent.

5. **What does `mooncake mod search` do without a registry?** v1:
   shells out to `gh search repos mooncake-modules-` or similar.
   Returns a list of GitHub repos matching the namespace pattern.
   That's enough until volume justifies more.

---

## Next concrete step

Draft a spec under `docs-working/streams/core/specs/` capturing the
**Phase 1 loader + cache + `use:` step type**. The spec doesn't have
to commit to all of Phases 2–5; it has to commit to a Phase 1 that
doesn't paint Phase 2 into a corner.
