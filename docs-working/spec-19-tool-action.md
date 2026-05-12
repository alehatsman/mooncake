# Spec 19: `tool` Action — Declarative Tool Provisioning

**Epic:** E8 Declarative Tool Provisioning — make mooncake the way you say
"this machine should have these dev tools at these versions," composing with
mise/asdf/direnv for activation rather than competing with them.
**Effort:** M (~1 week)
**Value:** High — closes a long-standing gap (mooncake users currently
hand-write `download` + `unarchive` pairs per tool). Adds lockfile-based
reproducibility that mise/asdf only do weakly.

---

## Problem

Today, installing a dev tool with mooncake is hand-written boilerplate:

```yaml
- download:
    url: https://go.dev/dl/go1.25.3.linux-amd64.tar.gz
    dest: /tmp/go.tar.gz
    checksum: sha256:abc...
- unarchive:
    src: /tmp/go.tar.gz
    dest: ~/.local/share/mooncake/tools/go/1.25.3/
    strip_components: 1
```

Per tool, the user copies this pattern, swaps in OS/arch URL templates,
hard-codes a checksum, picks an install dir convention, and figures out how
PATH should pick it up.

The result: three subtle bugs per preset (wrong arch, missing checksum,
inconsistent install path), no shared lockfile, no interop with the ecosystem
of tools (mise/asdf/direnv) that already know how to activate tools given a
known install layout.

Reference: discussed at length in the brainstorm transcript. Verdict was
"earn the abstraction with a small, opinionated, lockfile-first `tool`
action — and stay out of the activation business."

---

## Goals

- **G1** Add a `tool` action that installs a tool into a stable, queryable
  location, idempotent by `(backend, name, version)`.
- **G2** Three backends in v1:
  - `archive-url` — templated URL + checksum. Direct download path,
    mooncake-owned lockfile integrity.
  - `github-release` — sugar over `archive-url` that resolves URL from a
    release's asset list.
  - `mise` — delegate to mise. Mooncake provides the declarative wrapper;
    mise provides the broad tool catalog (node, python, ruby, java, dozens
    of language ecosystems, plus its own asdf-plugin compatibility).
- **G3** Lockfile (`mooncake.lock` next to the applied config) records
  every declared tool. For URL-based backends it stores
  `(backend, name, version, resolved_url, sha256)` and **enforces** the
  checksum on subsequent installs. For the `mise` backend it stores
  `(backend, name, version)` only — integrity is mise's responsibility, and
  mooncake doesn't try to be a second authority for the same bits.
- **G4** Standard install layout for URL-based backends:
  `~/.local/share/mooncake/tools/<name>/<version>/`. Mise-backed tools live
  wherever mise puts them (`~/.local/share/mise/installs/...`); mooncake
  queries mise for the path rather than relocating.
- **G5** Optional `.tool-versions` side effect for asdf/mise interop —
  opt-in per-step (`write_tool_versions: true`), not on by default. Useful
  for the URL-backed tools, redundant for mise-backed ones (mise reads its
  own state).
- **G6** CLI sugar: `mooncake tool which <name>`, `mooncake tool env --shell zsh`.
  Reads the lockfile, delegates to `mise which` for mise-backed entries,
  uses install-dir paths for URL-backed ones. **No daemon, no shell hook,
  no shim, no exec runtime.**

**Non-goals (deferred):**

- Native language-ecosystem backends (`npm:`, `pipx:`, `cargo:`,
  `go install`). **Covered through the `mise` backend**, which already
  supports them via its own backend system. Mooncake does not duplicate
  this surface.
- Plugin loading. mise's plugin system is a supply-chain swamp; if a user
  wants exotic plugins, they invoke mise directly. Mooncake's `mise`
  backend will not pass through plugin-install commands.
- Version constraints (`>=1.25`, `^1.25.3`, `latest`). v1 requires concrete
  versions across all backends. `latest` resolution lands as a separate
  spec.
- Content-addressed store / GC. Design the API to allow it later (see
  Snags), but ship `installs/<name>/<version>/`.
- Activation runtime, shims, `mooncake exec`. Composition with mise/direnv
  is the integration story.
- Cross-config lockfile reconciliation. If two configs install
  `(go, 1.25.3)` with conflicting checksums, the install dir is shared and
  first-writer wins; both lockfiles record their own view. v1 limitation
  (and irrelevant for the `mise` backend, which doesn't own the install
  dir).
- Auto-bootstrapping mise itself. v1 treats mise-on-PATH as a precondition;
  E8.7 ships a stock preset that installs mise via `archive-url`.

---

## Key files

| File / location | Role |
|---|---|
| `internal/actions/tool/` (new pkg) | Handler: `Metadata`, `Validate`, `Execute`, `DryRun`. Mirrors layout of `internal/actions/download/`. |
| `internal/actions/tool/handler.go` | Action entry point. |
| `internal/actions/tool/backend.go` | `Backend` interface + registry. Strategy-shaped (Plan/Install/Locate) so URL-based and delegating backends both fit. |
| `internal/actions/tool/backend_archive.go` | `archive-url` backend. |
| `internal/actions/tool/backend_github.go` | `github-release` backend. |
| `internal/actions/tool/backend_mise.go` | `mise` backend — shells out to a `mise` binary on PATH. |
| `internal/actions/tool/install.go` | Generic install pipeline for URL-based backends (resolved → install dir). Wraps `download` + `unarchive` primitives. Bypassed by the `mise` backend. |
| `internal/actions/tool/store.go` | Install-dir layout, lookup, existence check. URL-backed tools only. |
| `internal/lockfile/` (new pkg) | `mooncake.lock` read/write/merge. Toml. |
| `internal/register/register.go` | Register the new handler. |
| `internal/config/config.go` | `Tool` struct definition. |
| `internal/config/schema.json` | Schema for `tool:` step. |
| `cmd/tool.go` (new) | `mooncake tool {which,env,list}` subcommands. |
| `internal/actions/download/handler.go` | **Reuse** `downloadFile` (l.309) — extract to a shared helper if it's not already library-callable. |
| `internal/actions/unarchive/handler.go` | **Reuse** `extractTarGzArchive` / `extractZipArchive` etc. (l.339+). Same extraction; same library-shape question. |

---

## YAML surface

### Minimal — `archive-url`

```yaml
- tool:
    name: go
    version: "1.25.3"
    backend: archive-url
    url: "https://go.dev/dl/go{{ version }}.{{ os }}-{{ arch }}.tar.gz"
    # checksum optional on first install; required on subsequent
    # (lockfile enforces). Inline checksum overrides lock.
    checksum: "sha256:0c3b3b1e0b7d..."
    strip_components: 1
    bin: bin/go     # relative path inside install dir; informs `tool which`
```

Templates available in `url`:
- `{{ version }}` — the literal `version:` field
- `{{ os }}` — mooncake fact (`linux`, `darwin`, `windows`)
- `{{ arch }}` — mooncake fact (`amd64`, `arm64`, …)

### `github-release` (sugar)

```yaml
- tool:
    name: terraform
    version: "1.13.0"
    backend: github-release
    repo: hashicorp/terraform
    asset: "terraform_{{ version }}_{{ os }}_{{ arch }}.zip"
    bin: terraform
    # If `checksum:` is omitted on first install, mooncake fetches the
    # release's checksums file (if present at a configurable path) or
    # records the downloaded artifact's SHA256 (TOFU). See Task 3.
```

GitHub-release backend internally constructs the asset URL:
```
https://github.com/{{ repo }}/releases/download/v{{ version }}/{{ asset }}
```

The `v` prefix is the GitHub default; override with `tag: "{{ version }}"`
(no prefix) if the project doesn't use it.

### `mise` (delegation)

```yaml
- tool:
    name: node
    version: "24.0.0"
    backend: mise
```

That's the whole surface. The mise backend shells out to:

```
mise install node@24.0.0
```

…and records the install in `mooncake.lock` as
`(backend=mise, name=node, version=24.0.0)`. No URL, no checksum — mise
owns those, including its own lockfile (`mise.lock`) if the user has one.

Optional fields for mise-backed tools:

```yaml
- tool:
    name: java
    version: "temurin-21.0.5"
    backend: mise
    # Specify mise's tool ID if it differs from the natural `name`.
    # mise treats "java" as a category; specific runtimes use a prefix.
    mise_tool: "java"
    # Pass-through env for the mise invocation (rarely needed).
    env:
      MISE_HTTP_TIMEOUT: "60"
```

The `mise` backend does **not** support the `url`, `repo`, `asset`,
`checksum`, `strip_components`, or `bin` fields. Validation errors if any
are set.

**Precondition:** `mise` must be on `PATH`. The action checks via
`exec.LookPath("mise")` during `Validate` and returns a clear error
(`mooncake's mise backend requires the mise binary on PATH; install it
first or use the archive-url backend`). Auto-bootstrap is E8.7.

### What gets installed

**URL-based backends** (`archive-url`, `github-release`):

```
~/.local/share/mooncake/tools/go/1.25.3/
├── bin/go              # <- 'bin' field points here
├── pkg/...
└── ...                 # remainder of the tarball
```

Idempotency: if `~/.local/share/mooncake/tools/<name>/<version>/` already
exists and is non-empty, the action reports `ok` and skips. No checksum
re-verification on already-installed tools (Snag 3).

**`mise` backend**:

```
~/.local/share/mise/installs/node/24.0.0/...
```

Idempotency: check via `mise ls --json --installed` (or
`mise which node --version 24.0.0`). If installed, no-op. Mooncake does not
relocate, symlink, or shadow mise's install dir.

---

## Lockfile: `mooncake.lock`

TOML, lives next to the applied root config. One section per
`(backend, name, version)` triple:

```toml
[[tool]]
backend       = "github-release"
name          = "terraform"
version       = "1.13.0"
resolved_url  = "https://github.com/hashicorp/terraform/releases/download/v1.13.0/terraform_1.13.0_linux_amd64.zip"
sha256        = "abc123..."
locked_at     = "2026-05-12T19:14:02Z"
locked_by_arch = "linux-amd64"

[[tool]]
backend       = "archive-url"
name          = "go"
version       = "1.25.3"
resolved_url  = "https://go.dev/dl/go1.25.3.linux-amd64.tar.gz"
sha256        = "0c3b3b..."
locked_at     = "2026-05-12T19:14:08Z"
locked_by_arch = "linux-amd64"

[[tool]]
backend       = "mise"
name          = "node"
version       = "24.0.0"
locked_at     = "2026-05-12T19:15:01Z"
# No resolved_url or sha256: mise owns the integrity layer.
# locked_by_arch omitted: mise picks the arch artifact itself.
```

### Semantics — URL-based backends (`archive-url`, `github-release`)

- **First install** for `(backend, name, version)`:
  - If inline `checksum:` provided: download, verify, install, write lock
    entry with that checksum.
  - If no checksum: TOFU. Download over HTTPS, compute sha256, install,
    write lock entry with the computed checksum.
- **Subsequent installs** (lock entry exists):
  - Lock entry is the source of truth. Download, verify checksum **matches
    the lock** (not the inline checksum, if any). Mismatch → fail.
  - Inline `checksum:` mismatch with lock → fail with a clear message
    telling the user to either fix one or run `mooncake tool upgrade`.
- **Per-arch entries:** `locked_by_arch` records the arch of the locking
  machine. If a different arch hits the same lock, the URL likely differs;
  v1 simply adds a second entry keyed on arch. (Mooncake.lock entries are
  effectively keyed on `(backend, name, version, arch)`.)
- **HTTPS required for TOFU.** A `url:` starting with `http://` and no
  inline checksum → validation error. (`Validate` step.)

### Semantics — `mise` backend

- **First install**: shell out to `mise install <name>@<version>`. On
  success, write lock entry with only `(backend, name, version, locked_at)`.
- **Subsequent installs**: lock entry exists. Check via
  `mise which <name> --version <version>` whether the install is still
  present; if so, no-op. If not, re-invoke `mise install`. mise handles
  its own integrity.
- Mooncake does **not** verify that mise resolved the same artifact a
  second time. That's mise's job (and mise has its own lockfile for it).
- A lock entry for the same `(name, version)` recorded under
  `backend=mise` cannot be silently re-resolved by another backend — the
  lockfile binds the choice of backend.

### Lockfile location

`filepath.Dir(plan.RootFile) + "/mooncake.lock"`. The planner already
exposes `RootFile`; the action reads it from the execution context.

For daemon-mode plans where `RootFile == "<inline>"` (Spec 18), the lockfile
lives at `<daemon-workdir>/mooncake.lock`. Documented; not pretty, but it
keeps semantics uniform.

---

## Task 1 — Backend abstraction

The abstraction is **strategy-shaped**, not just URL-resolution-shaped. URL-
based backends and the `mise` delegating backend both have to fit, and they
work very differently — URL backends compose with a shared install pipeline,
mise owns its own install.

`internal/actions/tool/backend.go`:

```go
type Spec struct {
    Backend  string
    Name     string
    Version  string
    // archive-url:
    URL              string
    // github-release:
    Repo, Asset, Tag string
    // mise:
    MiseTool         string            // override of Name → mise tool ID
    Env              map[string]string // pass-through env for mise invocation
    // Common (URL-based):
    InlineChecksum   string
    StripComponents  int
    Bin              string
}

// FactSnapshot is the OS+arch view used for URL templating.
type FactSnapshot struct{ OS, Arch string }

// Plan describes what installing this tool will require. Filled by Backend.Plan,
// consumed by Backend.Install (and the action handler for logging/lockfile).
type Plan struct {
    // For URL-based backends: the resolved download.
    URL             string
    Checksum        string // "" → TOFU on first install
    StripComponents int
    BinRel          string // bin path relative to install dir

    // For all backends: where the tool will end up after install. For
    // URL-based this is the mooncake install dir; for mise it's whatever
    // mise reports.
    InstallDir      string // may be "" pre-install for mise; resolved by Locate after

    // Whether this Plan executes through the shared URL install pipeline
    // (download/verify/extract) or through the backend's own Install.
    UseSharedPipeline bool
}

type Backend interface {
    // Validate is called from the handler's Validate phase to check that
    // backend-specific fields are well-formed and any preconditions
    // (e.g., `mise` on PATH for the mise backend) are satisfied.
    Validate(spec Spec) error

    // Plan returns what installing this tool will look like. For URL-based
    // backends this returns the resolved URL+checksum; for mise it returns
    // a Plan{UseSharedPipeline: false} and Install does the work.
    Plan(ctx context.Context, spec Spec, facts FactSnapshot) (Plan, error)

    // Install performs backend-specific install steps. For URL-based
    // backends with UseSharedPipeline=true, the handler calls the shared
    // install pipeline instead; this method is a no-op or unused.
    // For the mise backend, this shells out to `mise install`.
    Install(ctx context.Context, spec Spec, plan Plan) error

    // Locate returns the absolute path to the tool's bin dir for a given
    // installed tool. Used by `mooncake tool which` and `tool env`.
    Locate(ctx context.Context, spec Spec) (binDir string, err error)
}
```

### Backend implementations

**`archive-url`**

- `Validate`: requires `URL`. If no `InlineChecksum`, `URL` must be HTTPS.
- `Plan`: templates `URL` against `{{ version }}/{{ os }}/{{ arch }}`.
  Returns `Plan{URL, Checksum: spec.InlineChecksum, ..., UseSharedPipeline: true}`.
- `Install`: not called (shared pipeline handles it).
- `Locate`: returns `<installDir>/<bin>` from the lockfile + store layout.

**`github-release`**

- `Validate`: requires `Repo` and `Asset`.
- `Plan`: builds URL from `Repo`/`Tag`/`Asset` templates. Same shape as
  archive-url. Optional v1.5: probe a `SHA256SUMS` asset and pre-fill the
  checksum (punted for v1).
- `Install`, `Locate`: same as archive-url.

**`mise`**

- `Validate`: rejects URL-based fields (`URL`, `Repo`, `Checksum`,
  `StripComponents`, `Bin`). Checks `exec.LookPath("mise")` and returns a
  clear error if missing.
- `Plan`: returns `Plan{UseSharedPipeline: false}` — the install dir is not
  known until after `mise install` runs.
- `Install`: shells out:
  ```
  MISE_HTTP_TIMEOUT=60 mise install <mise_tool>@<version>
  ```
  with `spec.Env` overlaid. Idempotency check via
  `mise which <name> --version <version>` before invoking install.
- `Locate`: runs `mise which <name> --version <version>` and returns
  `filepath.Dir(stdout)`. Cached per-process.

**Backends do not touch the lockfile directly.** The handler reads/writes
the lockfile; backends only describe what they're going to do (`Plan`) and
do it (`Install`).

---

## Task 2 — Install pipeline

`internal/actions/tool/install.go`:

```go
// Install resolves the lockfile, downloads, verifies, extracts. Idempotent
// on (name, version): if the target dir is non-empty, returns "ok, no change".
func Install(ec *executor.ExecutionContext, spec Spec, resolved Resolved, lock *lockfile.Lock) (Outcome, error)
```

Flow:

1. Compute `installDir = ~/.local/share/mooncake/tools/<name>/<version>/`
   (use `os.UserConfigDir`-style resolution; expand `~`).
2. If `installDir` exists and non-empty → return `ok` (no change). Skip
   everything below.
3. Reconcile checksum source:
   - Lock entry exists → `expectedChecksum = lock.entry.SHA256`.
   - No lock entry, `resolved.Checksum != ""` → `expectedChecksum =
     resolved.Checksum`.
   - No lock entry, no resolved checksum → TOFU mode (`expectedChecksum =
     ""`, compute after download).
4. Download to a temp file (reuse `download.downloadFile` — see Snag 1).
5. Verify or compute checksum.
6. Extract to `installDir` (reuse `unarchive` extraction — see Snag 1).
   Honor `strip_components`.
7. Write/update lock entry.
8. If step has `write_tool_versions: true`, write/update `.tool-versions`
   at the same dir as `mooncake.lock`.

DryRun: steps 1, 3, 4 (HEAD request to verify URL exists, no body), report
what would happen. No filesystem writes.

---

## Task 3 — Lockfile package

`internal/lockfile/lockfile.go`. Small. TOML via the existing dependency
mooncake already uses (or `github.com/pelletier/go-toml/v2` — check
`go.mod`).

```go
type Lock struct{ Entries []Entry }

type Entry struct {
    Backend, Name, Version, ResolvedURL, SHA256, LockedAt, LockedByArch string
}

func Load(path string) (*Lock, error)           // missing file → empty lock
func (l *Lock) Lookup(backend, name, version, arch string) (Entry, bool)
func (l *Lock) Set(e Entry)
func (l *Lock) Save(path string) error          // atomic write (tmp + rename)
```

Concurrency: `flock(2)` on the lockfile during read-modify-write. Multiple
`mooncake apply` invocations in parallel against the same lockfile must not
corrupt it.

---

## Task 4 — Action handler

`internal/actions/tool/handler.go`. Standard mooncake handler:

```go
func init() { actions.Register(&Handler{}) }

func (Handler) Metadata() actions.ActionMetadata {
    return actions.ActionMetadata{
        Name: "tool",
        Description: "Install a developer tool at a pinned version with lockfile-backed reproducibility",
        Platforms: []string{"linux", "darwin"}, // windows later
    }
}

func (h *Handler) Validate(step *config.Step) error {
    // Required: name, version, backend. Per-backend: url XOR (repo + asset).
    // HTTPS required if no inline checksum.
}

func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
    // 1. Build Spec from step.Tool
    // 2. Resolve via backend
    // 3. Load lockfile (filepath.Dir(plan.RootFile) + "/mooncake.lock")
    // 4. Install (the pipeline from Task 2)
    // 5. Save lockfile if changed
    // 6. Return Result with changed/ok/skip + a clear log line
}

func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
    // Same as Execute up to download — HEAD the URL, don't fetch body.
}
```

Register in `internal/register/register.go` alongside the other handlers.

---

## Task 5 — CLI commands

`cmd/tool.go`:

| Command | What |
|---|---|
| `mooncake tool which <name>` | Print the absolute bin path for `<name>` from the lockfile + install dir. Uses CWD's `mooncake.lock`; falls back to `~/.mooncake/mooncake.lock` if none. |
| `mooncake tool list` | List installed tools (across all `~/.local/share/mooncake/tools/<name>/<version>/` dirs). Annotate which are referenced from the current dir's lockfile. |
| `mooncake tool env --shell zsh` | Print PATH-prepend lines for every tool in the lockfile, with comment headers. User pipes to `eval` or sources. **String generator, not a runtime.** |

Explicit non-features in v1:
- No `mooncake tool install <name>@<ver>` (would need a config; defeats the
  declarative model). Install happens via `mooncake apply` with a `tool:`
  step.
- No `mooncake tool upgrade` — punted to a follow-up spec. v1 workaround:
  bump the version in YAML and re-apply.
- No `mooncake exec` (see brainstorm: direnv/mise/just already cover it).

---

## Task 6 — Schema + struct

`internal/config/config.go`:

```go
type Tool struct {
    Name             string `yaml:"name"`
    Version          string `yaml:"version"`
    Backend          string `yaml:"backend"`
    URL              string `yaml:"url,omitempty"`
    Repo             string `yaml:"repo,omitempty"`
    Asset            string `yaml:"asset,omitempty"`
    Tag              string `yaml:"tag,omitempty"`
    Checksum         string `yaml:"checksum,omitempty"`
    StripComponents  int    `yaml:"strip_components,omitempty"`
    Bin              string `yaml:"bin,omitempty"`
    WriteToolVersions bool  `yaml:"write_tool_versions,omitempty"`
}
```

Add `Tool *Tool` to `config.Step`. Update `internal/config/schema.json` —
mirror the field constraints (oneOf `archive-url` requires `url`;
`github-release` requires `repo` + `asset`). Regenerate schema artifacts
via `make schema` (or whatever the project target is).

---

## Tests

Add `internal/actions/tool/handler_test.go`:

| Layer | Test |
|---|---|
| Validate | Missing required fields → error. `archive-url` without `url` → error. `github-release` without `repo`/`asset` → error. `http://` URL + no checksum → error. |
| Backend `archive-url` | Resolve renders `{{ version }}/{{ os }}/{{ arch }}` correctly with a fixed `FactSnapshot`. |
| Backend `github-release` | Resolve builds the expected GitHub URL with default `v` tag prefix and with `tag:` override. |
| Install pipeline | Mock the HTTP fetcher + extractor. Given a TOFU spec, install writes a lock entry with the computed checksum. Given an existing lock entry with mismatched checksum, fails clearly. |
| Idempotency | Install when `installDir` already exists → returns `ok`, no fetch attempted. |
| Lockfile | Round-trip TOML. Concurrent writes (two goroutines `Set`+`Save`) under `flock` produce a valid file with both entries. |
| Action E2E | A canned http test server serves a tiny tarball; full `Execute` path installs to a temp dir, writes lockfile, second run is no-op. |
| `.tool-versions` | `write_tool_versions: true` appends `name version\n` to `<lockfile-dir>/.tool-versions`, deduping on rerun. |

CLI smoke tests under `cmd/`:

- `mooncake tool which go` prints absolute path after installing `go` via a
  config in a temp dir.
- `mooncake tool env --shell zsh` produces a parseable script with one
  `export PATH=...` line per installed tool.

---

## Docs

1. New page: `docs/guide/actions/tool.md` — full action reference. Examples
   for both backends. Explain lockfile semantics with a worked example.
2. New page: `docs/guide/tools/composition.md` — "how to use mooncake's
   `tool` action alongside mise / direnv / asdf." Three small recipes.
3. Update `docs/guide/config/actions.md` index to list `tool`.
4. Update `LLM_GUIDE.md` action count (`14 actions migrated ✅` from 13).
5. Changelog: "Add `tool` action for declarative tool provisioning with
   lockfile-backed reproducibility. Two backends in v1 (`archive-url`,
   `github-release`)."

---

## Acceptance criteria

1. `go test ./internal/actions/tool/... ./internal/lockfile/... ./cmd/...`
   passes. `make ci` clean.
2. This YAML, applied twice on Linux/amd64, installs Go 1.25.3 once,
   reports no-change on rerun, and produces a valid lockfile:
   ```yaml
   - tool:
       name: go
       version: "1.25.3"
       backend: archive-url
       url: "https://go.dev/dl/go{{ version }}.{{ os }}-{{ arch }}.tar.gz"
       strip_components: 1
       bin: bin/go
   ```
3. After step 2, `mooncake tool which go` prints
   `~/.local/share/mooncake/tools/go/1.25.3/bin/go`.
4. After step 2, `mooncake tool env --shell zsh` prints a line of the form
   `export PATH="$HOME/.local/share/mooncake/tools/go/1.25.3/bin:$PATH"`.
5. Editing the lockfile to a wrong checksum and rerunning the apply fails
   with a clear error pointing at the lock entry.
6. `github-release` backend installs `terraform 1.13.0` end-to-end against
   real GitHub (or a recorded fixture — pick one).
7. `write_tool_versions: true` produces an asdf/mise-compatible
   `.tool-versions` file in the lockfile's directory.
8. mise installed on the same machine, reading the generated
   `.tool-versions`, activates the same versions. (Manual smoke test;
   document in PR.)

---

## Snags to handle

1. **`download` / `unarchive` library shape.** Both currently live as
   action handlers. `downloadFile` (`download/handler.go:309`) and the
   `extract*` family in `unarchive/handler.go` need to be callable from the
   tool action. Same problem as Spec 18's planner extraction. Either:
   (a) extract the body into an exported helper in the same package
   (`func Download(url, dest, checksum string, ...) error`), or (b) move
   into a shared `internal/fetch/` package. Day-one decision; (a) is
   cheaper and likely sufficient.
2. **`installDir` empty-vs-existent.** "Non-empty dir" is the idempotency
   key. A *partially-installed* dir from a previous crashed extraction
   would be mistakenly treated as installed. Mitigation: extract into a
   sibling `<version>.tmp/` and rename atomically on success.
3. **No re-verify of existing installs.** v1 trusts the install dir once
   created. A future spec can add `mooncake tool verify` that re-hashes
   installed contents against a manifest. v1 keeps idempotency cheap and
   defers integrity-checking.
4. **`os.UserConfigDir` vs hardcoded `~/.local/share/mooncake/tools`.**
   Mooncake may already have a convention here — check
   `internal/facts/` or any `XDG_DATA_HOME` handling. Don't invent a new
   path if mooncake has a `DataDir()` already. (Day-one verify.)
5. **`plan.RootFile` for inline plans.** Spec 18 set this to `"<inline>"`.
   The tool action's lockfile path derives from it. For daemon mode,
   resolve to `<daemon-workdir>/mooncake.lock`. Document.
6. **Lockfile churn in git.** Users will commit `mooncake.lock`. The
   `locked_by_arch` field means CI on a different arch adds a second entry
   → a second commit. Acceptable for v1 (this is also how mise / cargo work
   for some cases). Document.

---

## Sequencing / follow-up specs in E8

In rough priority order; spec numbers assigned when written.

- **E8.2** `mooncake tool upgrade <name>` — explicit verb to bump a lock
  entry. Needs version-constraint parsing.
- **E8.3** `latest` / version-constraint resolution (`>=1.25`, `^1.25.3`).
  Touches every backend's `Resolve`.
- **E8.4** `mooncake tool verify` — re-hash installed contents against
  recorded checksum (integrity check, not just idempotency).
- **E8.5** Content-addressed store + GC — `installs/<name>/<version>` becomes
  a symlink into `store/<sha>/`. Multi-config dedup.
- **E8.6** Additional backends: `npm:`, `pipx:`, `cargo:`, `go install`.
  Each gated on a "is the ecosystem already bootstrapped" precondition.
- **E8.7** A small library of stock tool presets that wrap the `tool`
  action (`preset: go-tool`, `preset: terraform-tool`, etc.) — what users
  actually copy-paste.

---

## Risk notes

- **TOFU vs strict.** Trust-on-first-use is the default in v1 because
  forcing every user to copy a checksum out of a release page is friction
  that no one will tolerate. A `--strict-checksums` apply flag (and/or a
  `mooncake.lock` config option) can opt into "no inline checksum and no
  lock entry → fail." Land the default and the strict mode in the same PR
  if cheap, otherwise strict can come later.
- **`mooncake.lock` location.** Project-local (next to the applied config)
  is the right v1 call — it mirrors npm/cargo/poetry and lets per-project
  reproducibility work without coordination. A "system" lockfile at
  `/etc/mooncake/lock.toml` for system-wide configs is plausible but adds
  complexity; defer.
- **No activation runtime is the whole point.** Resist scope creep here —
  `mooncake exec`, shims, shell hooks, and prompt integration are
  explicitly out of scope. If a follow-up PR wants any of them, it needs a
  separate spec and a real justification beyond "asdf has it."
- **macOS code signing / quarantine.** Downloaded binaries on macOS get the
  quarantine bit set, which can prompt Gatekeeper on first run. Document;
  don't try to strip xattrs in v1 (that's a privilege/permissions can of
  worms).
