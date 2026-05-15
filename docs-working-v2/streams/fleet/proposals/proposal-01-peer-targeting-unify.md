# Proposal 01: Unify peer targeting — collapse `--peers` and `--peer-filter` into one DSL

**Status:** Draft proposal
**Effort:** S (~3 days; touches every fleet subcommand)
**Value:** High — every fleet subcommand exposes both flags; users have
to know which one to reach for. Halves the surface, simplifies docs.

---

## Problem

Today **every** fleet subcommand has both:

```
--peers value                 Comma-separated peer names (default: all)
--peer-filter key=value [...] Filter by key=value (tag=, name=, os=, role=)
```

These intersect — both are AND-applied. The result is that
"target peers" has two ways to express itself:

```bash
# All equivalent semantically:
mooncake fleet exec "cmd" --peers main_pc,laptop
mooncake fleet exec "cmd" --peer-filter name=main_pc --peer-filter name=laptop
mooncake fleet exec "cmd" --peers main_pc --peer-filter name=laptop  # intersect = one peer
```

Receipts from manual testing:

- I used `--peers` for one-shot tests, `--peer-filter tag=` for
  multi-peer fan-out. Took several rounds to know which was which.
- Documentation has to mention both on every command.
- `fleet apply`'s help line for `--peer-filter` is multi-paragraph:
  *"Commas within one flag = AND; repeating the flag = OR.
   Intersected with --peers. Example: --peer-filter tag=os=darwin"* —
  that's a lot to remember for the common case "just give me these
  three boxes".

The two-flag design papers over a single concept (peer selection
expression). Pick one shape.

## Proposal

Collapse into a **single flag** `--peer` (singular, repeatable) that
takes either a peer name or a key=value filter:

```bash
# Names:
mooncake fleet exec "cmd" --peer main_pc

# Multiple names (each --peer adds to the union):
mooncake fleet exec "cmd" --peer main_pc --peer laptop

# Tag filter (key=value form):
mooncake fleet exec "cmd" --peer tag=production

# Mix names and filters (UNION semantics, like Kubernetes labels):
mooncake fleet exec "cmd" --peer main_pc --peer tag=gpu
   # → "main_pc" AND every peer tagged gpu

# Intersect via @ syntax (or :):
mooncake fleet exec "cmd" --peer @tag=production,os=linux
   # → peers tagged production AND running linux
```

The disambiguation rule:
- **Contains `=`** → key=value filter (intersect within a single --peer)
- **No `=`** → peer name
- **Leading `@`** → multi-key filter group (intersect within group)
- **Multiple `--peer` flags** → union (matches *any* of them)

Alias for the old shorthand:
```bash
mooncake fleet exec "cmd" --peers main_pc,laptop   # still works (deprecated, comma-split into multiple --peer)
```

## API

Each fleet subcommand drops `--peer-filter` and keeps only `--peer`:

| Old | New |
|---|---|
| `--peers main_pc,laptop` | `--peer main_pc --peer laptop` (or kept as a sugar form) |
| `--peer-filter tag=production` | `--peer tag=production` |
| `--peer-filter tag=production,os=linux` | `--peer @tag=production,os=linux` |
| `--peer-filter tag=prod --peer-filter tag=staging` (OR) | `--peer tag=prod --peer tag=staging` |

Default (no `--peer`) = all peers, same as today.

## Subcommand surface

Same flag shape on every command:
- `fleet apply --peer ...`
- `fleet exec --peer ...`
- `fleet observe cpu --peer ...`
- `fleet ps --peer ...`
- `fleet watch --peer ...`
- `fleet status --peer ...`
- `fleet upgrade --peer ...`

`fleet doctor <peer-name>` keeps its positional argument (single-peer
focus); see proposal-05 for the `--all` form.

## Output sketch

When the user filters, echo the resolved set:

```
$ mooncake fleet exec "uname -s" --peer tag=production --peer gpu-box-1
fleet exec: 4 peers selected (3 by tag=production, 1 by name):
  main_pc, laptop, db-1, gpu-box-1
command = "uname -s"
[main_pc] ...
```

The explicit "X peers selected (...)" prefix makes the filter
behavior auditable — easy to spot when the filter selects more or
fewer than expected. Matches `mooncake fleet exec --peer-filter
tag=nonexistent` current message: `selected 0 of 2 peer(s); nothing
to do` (which is already good).

## Receipts

From the 2026-05-15 audit:
- Round 30: I used `--peers local` for explicit single-peer test
- Round 34: I used `--peer-filter tag=test` for tag-based fan-out
- Round 34: tested `--peer-filter name=local` because I forgot
  whether `--peers` and `--peer-filter` overlapped semantically
- Each iteration cost ~30s of "wait, which one for this case?"

For a CI script writer, having to maintain two flag forms is friction.
One flag, two shapes (name vs. key=value), is cleaner.

## Migration

Deprecate `--peer-filter` in 0.3.x:
- Emit `[deprecated] --peer-filter; use --peer key=value` warning on
  use
- Keep accepting the old flag for one minor cycle
- Drop in 0.4.x

`--peers` (plural, comma-list) can stay as sugar (it's idiomatic for
multi-value flags in many CLI tools), but internally just expands
to `--peer X --peer Y`.

## Risks

- **Existing peers.toml deployments don't break** — only the CLI
  flag shape changes; peers.toml schema stays.
- **Existing scripts using `--peers` / `--peer-filter`** keep
  working through the deprecation window.
- **Kubernetes-label-selector mental model isn't universal.** Some
  users may find `@tag=X,os=Y` syntax (intersect within one flag)
  confusing. Alternatives: spell it out (`--peer-and tag=X,os=Y`),
  or require multiple `--peer` for AND too (lose the within-flag
  intersect). Going with `@` is concise; revisit if testing shows
  confusion.

## What this doesn't address

- **Negation** (`--peer NOT tag=staging`) — out of scope; add later
  if asked.
- **Regex matching on peer names** — out of scope; add later if asked.
- **Wildcard / glob matching on hostnames** — out of scope; the
  current key=value DSL covers most fleet-management cases.
