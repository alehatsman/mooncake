# Proposal 04: `mooncake actions show <name>` — per-action documentation from the registry

**Status:** Draft proposal
**Effort:** XS (~1 day, mostly formatting)
**Value:** High — closes the discoverability gap that drives users
to `grep schema.json` or LLM_GUIDE.md. Trivial to build because
all the data is already in the registry.

---

## Problem

Today the surface for "what does action X take?" is:

| Surface | Returns |
|---|---|
| `mooncake actions list` | Action table: name, category, platforms, sudo, check |
| `mooncake schema generate --output schema.json` | JSON Schema for the entire surface (44+ actions) |
| `mooncake docs generate --section action-summary` | Markdown summary of every action |
| `docs/guide/config/actions.md` | Canonical Markdown reference (slow to update) |
| `LLM_GUIDE.md` | "13 actions" (until MT-6 fix) — drift target |

None of these answers the user's actual question:

> What parameters does `file.copy` take? What's required? What's the
> minimum example?

Today's flow: `mooncake schema generate` → open file → `grep -A 30
'"file.copy"' schema.json` → squint at JSON Schema → guess. 250+
times during my audit.

The information is sitting in the registry. Action metadata
(`Permissions`, `Diff`, `Cost`, `Reverse`, `x-platforms`,
`x-implements-check`, `x-version`, `x-emits-events`, descriptions,
required fields) is all generated. There just isn't a CLI surface
that returns one action's worth.

## Proposal

A new subcommand:

```
$ mooncake actions show file.copy
```

Output (text default):
```
file.copy
─────────
Copy a single file from source to destination, preserving mode.

Category:       file
Platforms:      all
Requires sudo:  no
Implements check: yes
Supports dry-run: yes
Version:        1.0.0
Emits events:   file.updated

Required parameters:
  src      string    Path to source file
  dest     string    Path to destination

Optional parameters:
  mode             string    File mode (e.g. "0644"); preserves source mode if unset
  follow_symlinks  boolean   Resolve symlinks before copy (default: true)
  backup           boolean   Create a .bak file before overwriting (default: false)

Minimum example:
  - file.copy:
      src: /etc/hostname
      dest: /tmp/hostname.bak

Common errors:
  - "src is a directory, use shell: cp -r ... instead"
    → file.copy is for single files (see #50)

  - "stat ... no such file or directory"
    → src path doesn't exist

Related:
  - file.write   (write content, not copy a file)
  - file.template (render Jinja2 template)
  - shell        (cp -r for directories)
```

Output (JSON):
```
$ mooncake actions show file.copy --format json
```
Returns the schema definition + the same metadata, machine-parseable.

## API

| Command | Behavior |
|---|---|
| `mooncake actions show <name>` | Per-action card (text default) |
| `mooncake actions show <name> --format json` | JSON Schema + metadata |
| `mooncake actions show <name> --format yaml` | YAML form (for piping into editor) |
| `mooncake actions show` (no args) | Error: "specify an action name; try `mooncake actions list`" |

Also a shortcut alias:
```
$ mooncake action file.copy             # synonym for actions show
$ mooncake docs file.copy               # debatable; conflicts with `mooncake docs generate`
```

I'd vote for `mooncake actions show <name>` as primary, with
`mooncake action <name>` as a one-arg shortcut.

## Receipts

During the audit, the actions I had to grep schema for repeatedly:
- `file.download`: required `checksum:` vs `sha256:` (finding #14, #44 umbrella)
- `git.clone`: `repo:` vs `url:` (finding #33, MT-33 fix)
- `git.config`: `repo:` required when `scope: local` (finding #52)
- `git.checkout`: `dest:` required (finding #52)
- `text.delete_range`: `start_anchor` / `end_anchor` (had to discover from validate error)
- `text.patch.json`: `set:` / `delete:` / `merge:` — NOT RFC 6902 `operations: [{op, ...}]` (finding #32)
- `tool`: `tag:` vs `version:`; `bin:` defaulting; `backend:` enum (finding #39, #40)
- `wait.command`: `expect_exit:` vs my guessed `expected_exit:` (#83)
- `assert: http:`: `status:` vs my guessed `status_code:` (#44 manifest)
- `pkg.repo`: `apt:` / `dnf:` / `brew:` discriminator
- `observe.*`: `.value.X` accessor pattern (finding #70 amend)

Every one of these would have been a one-liner with `mooncake actions
show <name>`. Each cost ~30 seconds to discover via grep.

## Implementation sketch

In `cmd/actions.go` (or wherever `actions list` lives):
```go
func showAction(name string) error {
    meta, ok := registry.ActionMetadata(name)
    if !ok {
        return suggestSimilar(name)  // "did you mean: file.copy?"
    }
    schema := schemagen.ForAction(name)   // already exists for schema generate
    return printActionCard(meta, schema, os.Stdout)
}
```

Both `meta` and `schema` are already populated. The work is just the
text rendering.

"Common errors" section requires curated content — start with the
top 3-5 errors per action (gather from `findings-*/` over time).
This is the part that needs human input; ship without it and
add per-PR.

## What this doesn't address

- **Editor / IDE integration**. The schema.json is already there;
  YAML language servers can be pointed at it. Separate proposal
  could be `mooncake schema install --editor vscode|nvim|...` to
  auto-wire the editor.
- **Localized output** — same caveat as proposals 01/02.

## Why this should ship first

It's the smallest proposal in this set AND has the broadest impact.
Every other proposal helps users *write* mooncake better; this one
helps them *discover* what to write at all. Ship it first; the
others compound on top.
