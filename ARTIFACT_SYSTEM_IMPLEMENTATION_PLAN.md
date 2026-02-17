# Artifact Generation System - Implementation Plan

## Overview
Comprehensive artifact generation and validation system for LLM-driven agent loops. Provides structured output capture, change tracking, and validation capabilities.

## Current State (Foundation Exists)
✅ **internal/artifacts/writer.go** (~720 lines)
- FileChange tracking with checksums
- Unified diff generation
- Event-based file tracking
- JSON output (results.json, diff.json, summary.json)
- Per-run artifact directories

## Implementation Phases

### Phase 1: Enhanced Artifact Metadata (~400 lines)
**Goal:** Richer file change metadata for LLM consumption

**Files to Create/Modify:**
1. `internal/artifacts/metadata.go` (NEW ~200 lines)
   - `DetailedFileChange` struct:
     - Before/after content (optional, configurable)
     - Line statistics (added/removed/modified counts)
     - Hunk breakdown per file
     - Language detection
     - Token/character counts
   - `AggregatedChanges` struct:
     - Total files changed
     - Total lines added/removed
     - File types distribution
     - Directory-level summaries

2. `internal/artifacts/writer.go` (EXTEND ~100 lines)
   - Add `EnhancedConfig` options:
     - `CaptureFileContent bool` - Store full before/after
     - `GenerateMarkdownSummary bool` - Human-readable output
     - `MaxDiffSize int` - Limit diff file sizes
   - Add methods:
     - `GenerateMarkdownSummary()` - Create SUMMARY.md
     - `GenerateConsolidatedDiff()` - Single .diff with all changes

### Phase 2: artifact.capture Action (~350 lines)
**Goal:** Wrap steps and capture all artifacts with enhanced metadata

**Files to Create:**
1. `internal/actions/artifact_capture/handler.go` (~250 lines)
   - Handler implementing artifact.capture action
   - Wraps child steps in artifact context
   - Collects all file changes from wrapped steps
   - Generates comprehensive output:
     - `<name>/changes.json` - Structured metadata
     - `<name>/changes.diff` - Consolidated unified diff
     - `<name>/SUMMARY.md` - Human-readable summary
     - `<name>/files/` - Optional before/after content

2. `internal/actions/artifact_capture/handler_test.go` (~100 lines)
   - Test artifact capture with multiple file operations
   - Test output file generation
   - Test metadata accuracy

3. `internal/config/config.go` (EXTEND)
   - Add `ArtifactCapture` struct:
     ```go
     type ArtifactCapture struct {
         Name              string   `yaml:"name"`
         OutputDir         string   `yaml:"output_dir"`
         Format            string   `yaml:"format"` // "json", "markdown", "both"
         CaptureContent    bool     `yaml:"capture_content"`
         MaxDiffSize       int      `yaml:"max_diff_size"`
         IncludeChecksums  bool     `yaml:"include_checksums"`
         Steps             []Step   `yaml:"steps"`
     }
     ```

### Phase 3: artifact.validate Action (~300 lines)
**Goal:** Validate artifacts against constraints (change budgets)

**Files to Create:**
1. `internal/actions/artifact_validate/handler.go` (~200 lines)
   - Handler implementing artifact.validate action
   - Reads artifact JSON from previous capture
   - Validates constraints:
     - Max files changed
     - Max lines changed
     - Max file size
     - Required/forbidden file patterns
     - Test file coverage (e.g., must modify tests if modifying code)
   - Fails step if validation fails

2. `internal/actions/artifact_validate/handler_test.go` (~100 lines)
   - Test various validation scenarios
   - Test constraint violations
   - Test pass/fail behavior

3. `internal/config/config.go` (EXTEND)
   - Add `ArtifactValidate` struct:
     ```go
     type ArtifactValidate struct {
         ArtifactFile       string   `yaml:"artifact_file"`
         MaxFiles           *int     `yaml:"max_files"`
         MaxLinesChanged    *int     `yaml:"max_lines_changed"`
         MaxFileSize        *int     `yaml:"max_file_size"`
         RequireTests       bool     `yaml:"require_tests"`
         AllowedPaths       []string `yaml:"allowed_paths"`
         ForbiddenPaths     []string `yaml:"forbidden_paths"`
     }
     ```

### Phase 4: Integration & Examples (~200 lines)

**Files to Create:**
1. `examples/artifact-capture-example.yml` (~100 lines)
   - Basic artifact capture
   - LLM agent workflow example
   - Validation examples
   - Rollback patterns

2. `examples/llm-agent-workflow.yml` (~100 lines)
   - Complete LLM agent loop:
     ```yaml
     - artifact.capture:
         name: "refactor-auth"
         output_dir: "./ai-artifacts"
         format: "both"
         steps:
           - file.replace: ...
           - file.insert: ...
           - repo.apply_patchset: ...

     - artifact.validate:
         artifact_file: "./ai-artifacts/refactor-auth/changes.json"
         max_files: 10
         max_lines_changed: 500
         require_tests: true

     - shell:
         cmd: "npm test"
       register: tests

     - assert:
         command:
           cmd: "npm test"
           exit_code: 0
       when: tests.failed
       failed_when: true  # Rollback if tests fail
     ```

### Phase 5: Enhanced Diff Analysis (~200 lines)

**Files to Create:**
1. `internal/artifacts/analysis.go` (NEW ~200 lines)
   - Analyze code changes:
     - Detect language from file extensions
     - Count tokens (for LLM context budgets)
     - Identify import changes
     - Detect test file changes
     - Calculate complexity metrics
   - Methods:
     - `AnalyzeChanges(files []FileChange) *ChangeAnalysis`
     - `DetectLanguage(path string) string`
     - `CountTokens(content string) int`

## Total Estimated Scope
- **New Code:** ~1,650 lines
- **Modified Code:** ~200 lines
- **Test Code:** ~300 lines
- **Examples:** ~200 lines
- **Total:** ~2,350 lines

## Implementation Order
1. ✅ Phase 1: Enhanced metadata structures (foundation)
2. ✅ Phase 2: artifact.capture action (core feature)
3. ✅ Phase 3: artifact.validate action (validation)
4. ✅ Phase 4: Examples & integration
5. ✅ Phase 5: Analysis enhancements (optional nice-to-have)

## Success Criteria
- [x] artifact.capture wraps steps and generates structured output
- [x] artifact.validate enforces change budgets
- [x] All file actions automatically contribute to artifacts
- [x] JSON output is LLM-consumable
- [x] Markdown summaries are human-readable
- [x] Comprehensive tests for all new actions
- [x] Example workflows for common LLM agent patterns

## Key Design Decisions

### 1. Nested Steps in artifact.capture
- Uses same step execution engine as loops
- Inherits variables and context
- Events bubble up to parent

### 2. Artifact Output Structure
```
artifacts/
  refactor-auth/
    changes.json      # Structured metadata
    changes.diff      # Consolidated unified diff
    SUMMARY.md        # Human-readable summary
    files/            # Optional before/after content
      before/
        src_auth.js
      after/
        src_auth.js
```

### 3. Backward Compatibility
- Existing artifact writer continues to work
- New actions are additive
- No breaking changes to existing APIs

### 4. Performance Considerations
- Artifact capture adds ~5% overhead
- Content capture (optional) can be expensive for large files
- Diff generation is cached
- Checksums calculated once per file

## Integration Points

### File Actions Enhancement
All file modification actions (file.replace, file.insert, file.delete_range, file.patch_apply, repo.apply_patchset) will:
- Emit detailed FileChange events
- Include before/after checksums
- Provide unified diffs
- Report line counts

### Event System
New events:
- `EventArtifactCaptureStart` - Capture begins
- `EventArtifactCaptureComplete` - Capture ends with metadata
- `EventArtifactValidationFailed` - Validation constraint violated

## Risk Mitigation
- **Large Files:** Max diff size limit prevents OOM
- **Nested Captures:** Prevent nesting artifact.capture actions
- **Disk Space:** Configurable artifact retention
- **Performance:** Optional content capture for large codebases

## Future Enhancements (Post-MVP)
- Artifact comparison (diff between runs)
- Artifact compression for storage
- Artifact signing for integrity
- Remote artifact storage (S3, etc.)
- Visual diff viewer web UI
- AST-based semantic diff
