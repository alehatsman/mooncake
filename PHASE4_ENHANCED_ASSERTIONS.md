# Phase 4: Enhanced Assertions

## Overview
Extended the existing assert action with three new assertion types for LLM agent operations: file checksum verification, git working tree status, and git diff validation.

## Implementation Status: ✅ COMPLETE

### What Was Added

#### 1. New Assertion Types

**FileSHA256** - Verify file checksum:
```go
type AssertFileSHA256 struct {
    Path     string // File path (required)
    Checksum string // Expected SHA256 checksum (required)
}
```

**GitClean** - Verify git working tree is clean:
```go
type AssertGitClean struct {
    AllowUntracked bool // Allow untracked files (default: false)
}
```

**GitDiff** - Verify git diff matches expected:
```go
type AssertGitDiff struct {
    ExpectedDiff string  // Expected diff content (required)
    Cached       bool    // Check staged changes (default: false)
    Files        *string // Limit diff to specific files (optional)
}
```

#### 2. Assert Struct Extension

Updated `Assert` struct in `internal/config/config.go`:
```go
type Assert struct {
    Command    *AssertCommand    // Existing
    File       *AssertFile       // Existing
    HTTP       *AssertHTTP       // Existing
    FileSHA256 *AssertFileSHA256 // NEW
    GitClean   *AssertGitClean   // NEW
    GitDiff    *AssertGitDiff    // NEW
}
```

#### 3. Handler Implementation

**Validation** - Updated `Validate()` method:
- Count all 6 assertion types
- Enforce exactly one assertion type per step
- Updated error messages

**Execution** - Added three new execute methods:
- `executeAssertFileSHA256()` - Calculate SHA256 and compare (~65 lines)
- `executeAssertGitClean()` - Check git status (~60 lines)
- `executeAssertGitDiff()` - Compare git diff output (~80 lines)

**Dry-run** - Added dry-run logging for all three types

#### 4. Features

**FileSHA256**:
- Reads file content and calculates SHA256 checksum
- Normalizes checksum (removes "sha256:" prefix if present)
- Path expansion with ~ and variables
- Template rendering in path and checksum fields

**GitClean**:
- Executes `git status --porcelain` to check for changes
- Optionally allows untracked files (filters out "??" lines)
- Verifies repository is a git repo before checking
- Returns clean status or list of uncommitted changes

**GitDiff**:
- Executes `git diff` or `git diff --cached` depending on `cached` flag
- Compares actual diff output with expected diff
- Optionally limits diff to specific files via `files` parameter
- Template rendering in expected diff and files fields
- Normalizes whitespace for comparison

### Usage Examples

#### File SHA256 Assertion
```yaml
- name: Verify build artifact checksum
  assert:
    file_sha256:
      path: ./dist/mooncake
      checksum: "abc123def456..."
```

#### Git Clean Assertion
```yaml
- name: Verify working tree is clean
  assert:
    git_clean:
      allow_untracked: false
```

#### Git Diff Assertion
```yaml
- name: Verify staged changes match approved patch
  assert:
    git_diff:
      expected_diff: |
        diff --git a/src/main.go b/src/main.go
        --- a/src/main.go
        +++ b/src/main.go
        @@ -10,5 +10,6 @@
          func main() {
        +   fmt.Println("Hello")
          }
      cached: true
      files: "src/*.go"
```

### Files Modified

1. **internal/config/config.go**:
   - Added `FileSHA256`, `GitClean`, `GitDiff` fields to Assert struct
   - Added `AssertFileSHA256`, `AssertGitClean`, `AssertGitDiff` type definitions

2. **internal/actions/assert/handler.go**:
   - Added imports: `crypto/sha256`, `encoding/hex`
   - Updated `Validate()` to check 6 assertion types
   - Updated `Execute()` to dispatch to new assertion types
   - Updated `DryRun()` to log new assertion types
   - Added `executeAssertFileSHA256()` method (~65 lines)
   - Added `executeAssertGitClean()` method (~60 lines)
   - Added `executeAssertGitDiff()` method (~80 lines)

3. **internal/config/schema.json**:
   - Auto-generated via `make schema-generate`
   - Added schema definitions for all three new assertion types
   - Updated Assert oneOf constraints

4. **examples/assert-enhanced-example.yml** (NEW):
   - 8 sections with comprehensive examples
   - Basic usage for all three assertion types
   - Combined examples (pre-commit checks)
   - Practical use cases (deployment, CI, security patches)
   - Error handling examples
   - Conditional assertions
   - Result registration examples

### Benefits for LLM Agents

1. **File Integrity**:
   - Verify configuration files haven't been tampered with
   - Validate build artifacts match expected checksums
   - Ensure downloaded files are correct

2. **Git State Verification**:
   - Ensure no uncommitted changes before deployment
   - Verify clean working tree in CI environments
   - Check for untracked files that shouldn't exist

3. **Diff Validation**:
   - Verify hotfix patches match approved changes
   - Ensure staged changes are exactly as expected
   - Validate automated code modifications
   - Compare actual changes against LLM-generated diffs

4. **CI/CD Integration**:
   - Pre-commit hooks verification
   - Deployment sanity checks
   - Automated code review validation
   - Security patch approval workflow

### Testing

All existing tests pass:
```
✓ TestHandler_Metadata
✓ TestHandler_Validate (8 subtests)
✓ TestHandler_Execute_* (27 subtests)
✓ TestHandler_DryRun_* (8 subtests)
✓ All other assert tests (43 tests total)
```

New assertions tested manually via examples/assert-enhanced-example.yml.

### Code Statistics
- **Lines added**: ~205 lines (handler methods)
- **Structs added**: 3 new assertion types
- **Methods added**: 3 execute methods
- **Files created**: 1 (example file, 250+ lines)
- **Files modified**: 2 (config.go, handler.go)
- **Backwards compatible**: Yes (all new fields are optional)
- **Breaking changes**: None

### Implementation Notes

1. **Checksum Format**:
   - Accepts both "abc123..." and "sha256:abc123..." formats
   - Normalizes to lowercase for comparison
   - Returns "sha256:..." format in assertion messages

2. **Git Commands**:
   - Uses `git status --porcelain` for clean check (machine-readable)
   - Uses `git diff` / `git diff --cached` for diff comparison
   - Verifies git repository before executing commands
   - All commands executed in `ec.CurrentDir`

3. **Error Handling**:
   - File not found → FileOperationError
   - Not a git repo → AssertionError with clear message
   - Git command failed → AssertionError with output
   - Assertion failed → AssertionError with expected/actual

4. **Template Support**:
   - All string fields support variable templating
   - Paths support ~ expansion and relative resolution
   - Expected diff and checksums can use variables

---

**Status**: Phase 4 ✅ COMPLETE
**Date**: 2026-02-17
**Next**: All phases from IMPLEMENTATION_ANALYSIS.md complete! 🎉

## Summary of All Phases

- ✅ Phase 1: Text Editing Actions (file.replace, file.insert, file.delete_range, file.patch_apply)
- ✅ Phase 2: Repository Actions (repo.search, repo.tree)
- ✅ Phase 3: Artifact System (diff generation, checksum tracking)
- ✅ Phase 4: Enhanced Assertions (file_sha256, git_clean, git_diff) ← **Just completed!**

**Optional remaining work**:
- Diff Budget system (track LOC changes per run)
- Additional assertion types (as needed)
- Enhanced artifact statistics (lines added/removed)
