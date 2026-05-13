# Spec 11: Preset Registry — Remote Discovery and Community Presets

**Epic:** Agent-Native Interface / Preset System  
**Effort:** M–L (1–2 days)  
**Value:** Very High — makes mooncake useful on any fresh machine; enables community preset ecosystem

---

## Problem

Presets are the high-level abstraction layer for agents: "give me a Wayland desktop",
"give me a Rust dev environment". But they currently only work if you have the mooncake
source tree. On a fresh machine reached via SSH, you have the binary and nothing else.

A community ecosystem is also impossible: there's no way to discover or install presets
published by others.

---

## Vision

```
# On a fresh Arch machine, only mooncake binary installed:
mooncake preset search wayland
mooncake preset install hyprland

# Add a community registry:
mooncake preset registry add github.com/someone/mooncake-presets

# Agent workflow:
mooncake preset list --format json   → catalog of available presets + metadata
mooncake preset install <name>       → fetches, caches, runs
```

---

## Registry Model

A **registry** is a GitHub repository with the following structure:

```
presets/
  <name>/
    preset.yml        # preset definition (existing format)
    README.md         # description, parameters, examples
    tasks/
      install.yml
      uninstall.yml
index.yml             # catalog: names, descriptions, tags, versions
```

### `index.yml` format

```yaml
version: 1
presets:
  - name: hyprland
    description: Hyprland Wayland compositor stack
    tags: [wayland, desktop, arch]
    platforms: [arch]
    version: "1.0.0"
  - name: neovim
    description: Neovim with Lua config and plugins
    tags: [editor, dev]
    platforms: [arch, debian, macos]
    version: "1.2.0"
```

The mooncake repo itself (`github.com/alehatsman/mooncake`) serves as the official
registry. Its `presets/` directory and `index.yml` are already the right shape.

---

## Registry Configuration

Stored at `~/.config/mooncake/registries.yml`:

```yaml
registries:
  - name: official
    url: github.com/alehatsman/mooncake
    ref: main          # branch, tag, or "latest-release"
  - name: community
    url: github.com/someone/mooncake-presets
    ref: main
```

The official registry is always present (hardcoded default). User registries are
additive. Name collisions: first match wins (official registry is first).

---

## Resolution Order

1. **Bundled** — presets embedded in the binary via `//go:embed`. Always available
   offline. Official presets ship bundled at build time.
2. **Cached** — previously fetched presets in `~/.mooncake/presets/<registry>/<name>/`.
   Used without network if cache is fresh (default TTL: 24h).
3. **Remote** — fetch `index.yml` + preset files from GitHub raw API. Cache result.

---

## Commands

### `mooncake preset list [--registry <name>] [--format text|json]`
Lists all available presets from bundled + all configured registries.

```
NAME        REGISTRY   TAGS                    PLATFORMS
hyprland    official   wayland desktop arch    arch
neovim      official   editor dev              arch debian macos
zsh-config  community  shell                   arch debian macos
```

`--format json` returns the full index entries for agent consumption.

### `mooncake preset search <query>`
Filters preset list by name, description, or tag. Substring match.

### `mooncake preset info <name>`
Shows full metadata + README for a preset. Fetches from registry if not cached.

### `mooncake preset install <name> [--registry <name>] [--dry-run]`
Fetches the preset if not cached, then runs it (equivalent to `mooncake run`
on the preset's `preset.yml`). Accepts the same flags as `mooncake run`.

```
mooncake preset install hyprland
mooncake preset install zsh-config --registry community
mooncake preset install neovim --dry-run
```

### `mooncake preset registry add <url> [--name <alias>] [--ref <branch>]`
Adds a registry to `~/.config/mooncake/registries.yml`.

### `mooncake preset registry list`
Shows configured registries with their URLs and cache status.

### `mooncake preset registry remove <name>`
Removes a registry by alias.

### `mooncake preset update [--registry <name>]`
Refreshes the index cache for all (or one) registry. Forces re-fetch of `index.yml`.

---

## Implementation

### `internal/registry/` package (new, or extend existing `internal/registry/`)

> Note: check if `internal/registry/` already exists and what it does.

```go
// Registry represents a configured preset source.
type Registry struct {
    Name string
    URL  string   // github.com/owner/repo
    Ref  string   // branch or tag
}

// Index is the parsed index.yml from a registry.
type Index struct {
    Version int           `yaml:"version"`
    Presets []PresetMeta  `yaml:"presets"`
}

type PresetMeta struct {
    Name        string   `yaml:"name"`
    Description string   `yaml:"description"`
    Tags        []string `yaml:"tags"`
    Platforms   []string `yaml:"platforms"`
    Version     string   `yaml:"version"`
}

// FetchIndex downloads and parses the index.yml from a registry.
func FetchIndex(reg Registry) (Index, error)

// FetchPreset downloads a preset directory to the local cache.
func FetchPreset(reg Registry, name string) (localPath string, err error)

// CachedPath returns the local cache path for a preset (may not exist).
func CachedPath(reg Registry, name string) string
```

### GitHub raw URL pattern

```
https://raw.githubusercontent.com/<owner>/<repo>/<ref>/index.yml
https://raw.githubusercontent.com/<owner>/<repo>/<ref>/presets/<name>/preset.yml
https://raw.githubusercontent.com/<owner>/<repo>/<ref>/presets/<name>/tasks/install.yml
```

Fetching a preset means downloading all files under `presets/<name>/` recursively.
Use the GitHub Contents API to list files:
`https://api.github.com/repos/<owner>/<repo>/contents/presets/<name>?ref=<ref>`

### Cache layout

```
~/.mooncake/
  presets/
    official/
      index.yml            # cached registry index
      index.yml.fetched    # timestamp of last fetch
      hyprland/
        preset.yml
        tasks/install.yml
        tasks/uninstall.yml
    community/
      index.yml
      zsh-config/
        preset.yml
        tasks/install.yml
```

### `cmd/presets.go` updates

Extend the existing `presetsCommand()` to add registry sub-commands and update
`list`/`install` to use the registry layer.

---

## `index.yml` for the official registry

Create `index.yml` at the mooncake repo root listing current bundled presets.
This is also what remote clients fetch to discover available presets.

---

## Agent-Friendliness

`mooncake preset list --format json` returns structured data agents can use to:
1. Discover what presets are available for the current platform (cross-ref with `mooncake facts`)
2. Pick the right preset by tag
3. Call `mooncake preset install <name>` with full structured feedback

Combined with `mooncake snapshot`, an agent can go from "fresh machine" to
"fully provisioned" without any user-provided YAML.

---

## Acceptance Criteria

1. `mooncake preset list` shows bundled presets without network access.
2. `mooncake preset list` fetches + merges remote registries when online.
3. `mooncake preset install hyprland` works on a machine with only the binary.
4. `mooncake preset registry add github.com/user/repo` adds a registry.
5. `mooncake preset install zsh-config --registry community` installs from a non-default registry.
6. Cached presets are used without re-fetching within the TTL.
7. `mooncake preset update` forces re-fetch of all indexes.
8. `--format json` on list/info returns machine-parseable output.
9. Network failures on registry fetch fall back to cache with a warning.
10. The official `index.yml` is kept in sync with the `presets/` directory.
