# mise-bootstrap

Install the [mise](https://mise.jdx.dev) version manager into the mooncake
tool store, so subsequent `tool:` steps can use `backend: mise`.

This is the declarative answer to the mise backend's only precondition:
mooncake refuses `backend: mise` if no `mise` binary is reachable. Run
this preset once at the top of your config and the rest of the run can
use mise-backed tool installs without any PATH manipulation.

## Why it exists

The mise backend shells out to a `mise` binary. v1 of the `tool` action
left "how does mise get there?" as an out-of-mooncake problem. This
preset solves it by using the `github-release` backend to fetch mise
itself, then relying on the mise backend's mooncake-store fallback to
discover the newly-installed binary at apply time.

## Usage

```yaml
- preset: mise-bootstrap
  parameters:
    version: "2026.5.6"

- tool:
    name: node
    version: "24.0.0"
    backend: mise
```

Both steps succeed in a single `mooncake apply`. The bootstrap step
installs mise to `~/.local/share/mooncake/tools/mise/<version>/bin/mise`.
The mise backend's runner falls back to this path when `mise` is not on
PATH, so the second step works immediately.

## Parameters

| Name | Required | Notes |
|---|---|---|
| `version` | Yes | Concrete mise version without the leading `v` (e.g. `2026.5.6`). Pin this for reproducibility. |

## Notes

- Arch + OS mapping is handled inside the preset. mise's release asset
  naming uses `macos` (not `darwin`) and `x64` (not `amd64`); the preset
  handles both via `vars:` steps gated on the system facts.
- To put `mise` on your interactive shell's PATH after bootstrap, run
  `eval "$(mooncake tool env --shell zsh)"` (or your shell's equivalent).
- The bootstrap install is tracked in `mooncake.lock` like any other
  `github-release` install — checksum-pinned on first install, enforced
  on subsequent runs.
