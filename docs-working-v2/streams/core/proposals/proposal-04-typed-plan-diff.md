# Proposal 04: Typed plan diffs — `plan --diff` produces per-action-type diffs, not just file diffs

**Status:** Draft proposal
**Effort:** M (~5–7 days; needs `Diff` method work per handler)
**Value:** High — `plan --diff` is the safety story's centerpiece.
Today it works great for file content; for everything else it shows
"would create" / "would update" with no detail. Agents and humans
both want richer diffs.

---

## Problem

Today `mooncake plan --diff`:

```
$ mooncake plan -c cfg.yml --diff

↑ write config             content differs (15 -> 22 bytes)
    --- /etc/foo.conf
    +++ /etc/foo.conf (proposed)
    @@ -1,1 +1,1 @@
    -host = localhost
    +host = db.production.com

↑ install nginx            would install
↑ create user alice        would create
↑ open firewall port 8080  would update
```

The file diff is excellent (#10 keeper). The non-file diffs are
opaque. The user has to infer:
- "would install nginx" — what version? from what repo?
- "would create user alice" — what UID? home dir? groups?
- "would update firewall" — what's the current state? what's the
  proposed state?

For an agent reviewing a plan before approval, "would update
firewall" is useless. It needs to know *what* would change.

The kernel already has the data. `spec-22` shipped the
`Diff` method on the handler ABI. The output of `Diff` is just
not surfaced.

## Proposal

When `plan --diff` runs, each action's `Diff` method emits a
typed diff structured by action category:

### Package diffs

```
↑ install nginx
    package    nginx
    manager    apt
    -          (not installed)
    +          nginx 1.24.0-1ubuntu1.1
```

```
↑ upgrade jq
    package    jq
    manager    apt
    -          1.6-2.1ubuntu3
    +          1.7.1-1
```

### User / group diffs

```
↑ create user alice
    user       alice
    uid        - (does not exist)
    +          1001 (auto-assigned)
    home       +/home/alice
    shell      +/bin/bash
    groups     +alice
```

```
↑ update user bob (add to docker group)
    user       bob
    uid        1002 (unchanged)
    groups     -bob,sudo
    +          bob,sudo,docker
```

### Firewall diffs

```
↑ open firewall port 8080
    backend    ufw
    rule       allow 8080/tcp
    -          (not present)
    +          rule index 5: allow 8080/tcp from any
```

### Service / cron diffs

```
↑ enable nginx service
    service    nginx
    state      -inactive
    +          active (running)
    enabled    -false
    +true
```

```
↑ install cron entry: backup
    file       /etc/cron.d/backup
    schedule   "0 3 * * *"
    command    /usr/local/bin/backup.sh
    user       root
    -          (not present)
    +          (above)
```

### Repo / git diffs (when applicable)

```
↑ git.checkout
    repo       /opt/app
    branch     -main (HEAD: a1b2c3d)
    +deploy (HEAD: f7g8h9i)
```

### Compound diffs (transactions, try, for_each expansions)

```
↑ transaction (3 children)
  ↑ child 1: file.write /etc/foo.conf
      [file diff as above]
  ↑ child 2: pkg install nginx
      [package diff as above]
  ↑ child 3: os.service nginx enable
      [service diff as above]

  All 3 children commit together. If any fails, all revert.
```

## Output formats

- **text** (default for `plan --diff`) — the human-readable shapes above
- **json** — typed object per step, machine-parseable:

```json
{
  "step_id": "step-0001",
  "action_type": "pkg",
  "diff": {
    "kind": "package",
    "manager": "apt",
    "package": "nginx",
    "from": {"installed": false},
    "to": {"version": "1.24.0-1ubuntu1.1"}
  }
}
```

- **yaml** — same as JSON but YAML for piping into other tools

```bash
mooncake plan -c cfg.yml --diff --format json | jq '...'
```

## Implementation

Each handler implementing `Diff` returns a typed `ActionDiff`:

```go
type ActionDiff interface {
    Kind() DiffKind  // "file", "package", "user", "firewall", "service", ...
    Render(w io.Writer, format Format) error  // text/json/yaml
}

type FileDiff struct {
    Path string
    From []byte
    To   []byte
    Mode FromTo[string]
    // ...
}

type PackageDiff struct {
    Manager string
    Package string
    From    *PackageState  // nil = absent
    To      *PackageState
}
```

The renderer per kind lives in `internal/diff/render_<kind>.go`.

For handlers that don't yet implement `Diff`, fall back to the
current "would <verb>" placeholder.

## API

```bash
# Current
mooncake plan -c cfg.yml --diff
mooncake plan -c cfg.yml --diff --format json

# New (just richer output; no flag changes)
```

## Pairs with

- **Proposal 01 (result schema)** — `ActionDiff.Kind` aligns with
  `Result.Operation` for symmetric "diff before" + "result after"
- **Agent proposal-04 (mcp typed diff)** — same diff structure
  exposed via the `diff_plan` MCP tool
- **spec-22** (handler ABI) — `Diff` method already required;
  this proposal makes its output meaningful

## Receipts

- During the audit, every `plan --diff` ran on file-write playbooks
  because the file-diff is the only useful diff
- For `pkg`, `os.user`, `os.firewall`, `os.cron`, `os.service`
  testing, I checked the plan output once, saw "would create",
  shrugged, and ran apply blind. A typed diff would let me audit
  before mutating.

## What this doesn't address

- **`Diff` not implemented for every handler** — that's a per-handler
  rollout. spec-22 phases addressed the priority handlers; this
  proposal pushes the rollout the rest of the way.
- **External diffs** (e.g., upstream package availability) — `pkg
  upgrade` should ideally show what version is available upstream
  vs. installed. Requires probing the package manager. Defer; v1
  can show just "would upgrade to <latest>" if available.

## Why this lives in core

The `Diff` method IS core's contract. Surfacing its output is also
core's surface (the planner consumes Diff, the renderer presents
it). The CLI subcommand `mooncake plan` is the entry point but the
logic is `internal/plan/` + `internal/diff/`.
