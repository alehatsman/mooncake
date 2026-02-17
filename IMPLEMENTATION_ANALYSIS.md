# Mooncake LLM Agent Actions - Implementation Analysis

## Existing Capabilities ✅

### File Operations (COMPLETE)
- **file action**: Create/update/delete files, directories, links
  - States: file, directory, absent, touch, link, hardlink, perms
  - Atomic writes, backups, ownership, permissions
  - Path: `internal/actions/file/handler.go` (816 lines)

- **copy action**: Copy files with checksums
  - Idempotency based on checksums
  - Atomic writes, backups
  - Path: `internal/actions/copy/handler.go`

- **template action**: Render Jinja2 templates
  - Path: `internal/actions/template/handler.go`

### Path Safety (COMPLETE)
- **pathutil package**: Path expansion and safety
  - `ExpandPath()` - template rendering, ~/, ./, ../ handling
  - `ValidateNoPathTraversal()` - prevent escaping base
  - `SafeJoin()` - secure path joining
  - `ValidateRemovalPath()` - prevent dangerous deletions
  - Path: `internal/pathutil/`

### Assertions (PARTIAL)
- **assert action**: Already exists with:
  - Command exit code verification
  - File existence/content checks
  - HTTP response validation
  - Path: `internal/actions/assert/handler.go`

### Infrastructure (COMPLETE)
- Action registry system
- Handler interface (Metadata, Validate, Execute, DryRun)
- ExecutionContext with template/evaluator/logger
- Result tracking with changed/failed/data
- Event system for observability
- Become (sudo) support

---

## Missing for LLM Agent Loop ❌

### 1. Text Editing Primitives (NEW ACTIONS NEEDED)
**Problem**: Current `file` action only supports:
- Full file write (content: "...")
- Template rendering

**Missing**:
- Regex/literal search-replace
- Anchor-based insertion
- Range deletion
- Patch application

**Required New Actions**:
```yaml
# file.replace - In-place text replacement
- name: Update API endpoint
  file.replace:
    path: src/client.ts
    pattern: 'oldapi\.com'
    replace: 'newapi.com'
    flags: {regex: true}
    backup: true

# file.insert - Insert after/before anchor
- name: Add import
  file.insert:
    path: src/index.ts
    anchor: '^import'
    position: after
    content: 'import { newUtil } from "./util";'
    regex: true

# file.delete_range - Delete between anchors
- name: Remove deprecated block
  file.delete_range:
    path: src/legacy.ts
    start_anchor: '// BEGIN DEPRECATED'
    end_anchor: '// END DEPRECATED'
    inclusive: true

# file.patch_apply - Apply unified diff
- name: Apply upstream patch
  file.patch_apply:
    patch_file: fixes/security.patch
    strip: 1
```

### 2. Repository Operations (NEW ACTIONS NEEDED)
**Problem**: No codebase-wide operations

**Required New Actions**:
```yaml
# repo.search - Search codebase
- name: Find TODOs
  repo.search:
    pattern: 'TODO:|FIXME:'
    regex: true
    glob: '**/*.{ts,js}'
    output_file: .mooncake/todos.json

# repo.tree - Generate tree
- name: Document structure
  repo.tree:
    max_depth: 3
    exclude_dirs: [node_modules, .git]
    output_file: .mooncake/tree.json
```

### 3. Enhanced Assertions (EXTEND EXISTING)
**Problem**: Missing git and checksum assertions

**Required Extensions**:
```yaml
# assert.file_sha256
- assert:
    file_sha256:
      path: dist/bundle.js
      checksum: "abc123..."

# assert.git_clean
- assert:
    git_clean:
      allow_untracked: false

# assert.git_diff
- assert:
    git_diff:
      expected_diff: |
        +++ src/file.ts
        +  new code
```

### 4. Artifact Generation (NEW SYSTEM)
**Problem**: No execution artifacts for agent loops

**Required**:
- `.mooncake/artifacts/{run-id}/`
  - `diffs/` - Unified diffs per step
  - `checksums/` - SHA256 before/after
  - `logs/` - stdout/stderr per step
  - `results.json` - Structured execution results

### 5. Diff Budget (NEW ENFORCER)
**Problem**: No LOC change limits

**Required**:
- Configurable max lines changed per run (default: 1000)
- Cumulative tracking across steps
- Fail if budget exceeded

---

## Implementation Strategy

### Phase 1: Text Editing Actions (Core Need)
1. **file.replace** (~200 lines)
   - Uses Go regexp package
   - Atomic write pattern (existing in file/copy actions)
   - Backup support (existing pattern)
   - idempotency check (content comparison)

2. **file.insert** (~150 lines)
   - Anchor matching (literal or regex)
   - Line-based insertion
   - Fail if anchor not found (unless allow_no_match)

3. **file.delete_range** (~150 lines)
   - Dual anchor matching
   - Inclusive/exclusive modes
   - Ambiguity detection

4. **file.patch_apply** (~300 lines)
   - Unified diff parser
   - Multi-file patch support
   - Reject file generation

### Phase 2: Repository Actions
5. **repo.search** (~200 lines)
   - Use filepath.Walk + regexp
   - Glob filtering (existing glob patterns)
   - JSON output

6. **repo.tree** (~100 lines)
   - Recursive walk with depth limit
   - JSON output

### Phase 3: Artifact System
7. **Artifact Collector** (~300 lines)
   - Hook into executor lifecycle
   - Generate diffs (go-diff library or simple impl)
   - Checksum tracking (crypto/sha256)
   - results.json generation

8. **Diff Budget** (~100 lines)
   - Track LOC changes
   - Configurable limits
   - Early termination

### Phase 4: Enhanced Assertions
9. Extend **assert action**:
   - `file_sha256` (~50 lines)
   - `git_clean` (~80 lines - exec git status)
   - `git_diff` (~100 lines - exec git diff)

---

## Leverage Existing Code

### Use Existing:
- `actions.Handler` interface - ALL new actions
- `actions.Register()` - Auto-registration
- `executor.ExecutionContext` - Context access
- `executor.NewResult()` - Result creation
- `pathutil.ExpandPath()` - Path rendering
- `pathutil.SafeJoin()` - Traversal protection
- `events.Publisher` - Event emission
- Atomic write pattern from file/copy handlers
- Backup pattern from file handler

### Extend:
- `config.Step` - Add new action fields
- `config/schema.json` - Add action schemas
- `events/event.go` - Add new event types
- `assert` action - Add new assertion types

### Create New:
- `internal/actions/file_replace/` - New action package
- `internal/actions/file_insert/` - New action package
- `internal/actions/file_delete_range/` - New action package
- `internal/actions/file_patch_apply/` - New action package
- `internal/actions/repo_search/` - New action package
- `internal/actions/repo_tree/` - New action package
- `internal/executor/artifacts.go` - New artifact system
- `internal/executor/diff_budget.go` - New budget tracker

---

## Estimated Implementation

### Code Volume
- Text editing actions: ~800 lines (4 actions)
- Repository actions: ~300 lines (2 actions)
- Artifact system: ~400 lines
- Diff budget: ~100 lines
- Assert extensions: ~230 lines
- Config/schema updates: ~300 lines
- Tests: ~2000 lines
- **Total: ~4,130 lines**

### Dependencies
- Standard library only (regexp, crypto/sha256, os/exec for git)
- No external dependencies needed

### Timeline
- Phase 1 (text editing): 2-3 days
- Phase 2 (repository): 1 day
- Phase 3 (artifacts): 1-2 days
- Phase 4 (assertions): 1 day
- Testing: 2 days
- **Total: 7-9 days**

---

## Key Design Decisions

### 1. Action Naming
Use dot notation for clarity:
- `file.replace` (not `file_replace` at YAML level)
- But Go package is `file_replace` (underscores required)

### 2. Sandbox Enforcement
- Repository root detection (git root or cwd)
- All paths validated via `pathutil`
- Optional strict mode (env var)

### 3. Atomic Operations
- All file modifications use temp file + rename
- Rollback not implemented (out of scope for MVP)
- Backup creation optional per action

### 4. Error Handling
- Use existing `executor.FileOperationError` types
- Fail fast (no partial applies)
- Clear error messages with context

### 5. Idempotency
- Text editing actions check if change needed
- Use content comparison (not just checksums)
- Report `changed: false` if no-op

---

## Next Steps

1. **Confirm scope** with user
2. **Start with Phase 1** - file.replace (highest value)
3. **Validate design** - ensure it works for agent loops
4. **Iterate** - add remaining actions based on feedback
