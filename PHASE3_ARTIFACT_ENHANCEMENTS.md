# Phase 3: Artifact System Enhancements

## Overview
Enhanced the existing artifact system with diff generation and checksum tracking capabilities for LLM agent operations.

## Implementation Status: ✅ COMPLETE

### What Was Added

#### 1. Enhanced Data Structures

**FileChange** (extended):
```go
type FileChange struct {
    Path           string // File path
    Operation      string // "created", "updated", "template"
    SizeBytes      int64  // File size
    ChecksumBefore string // SHA256 before modification (NEW)
    ChecksumAfter  string // SHA256 after modification (NEW)
    DiffFile       string // Path to unified diff file (NEW)
    StepID         string // Step that made the change (NEW)
}
```

**StepResult** (extended):
```go
type StepResult struct {
    // ... existing fields ...
    FilesChanged []string // Paths of files modified (NEW)
    DiffFiles    []string // Paths to diff files (NEW)
}
```

**FileOperationData** event (extended):
```go
type FileOperationData struct {
    // ... existing fields ...
    ChecksumBefore string // SHA256 before modification (NEW)
    ChecksumAfter  string // SHA256 after modification (NEW)
}
```

#### 2. New Helper Functions

**Checksum Calculation**:
- `calculateFileChecksum(path string) string` - Computes SHA256 checksum
- Returns `"sha256:..."` format

**Diff Generation**:
- `generateUnifiedDiff(path, before, after string) string` - Creates unified diff
- `splitLines(content string) []string` - Helper for diff generation

**File State Capture** (for action handlers):
- `CaptureFileStateBefore(path string) *FileSnapshot` - Captures file state before modification
- `CaptureFileStateAfter(before *FileSnapshot, stepID, operation string) *FileChange` - Generates diff and checksums

#### 3. Public API for Action Handlers

**RecordFileDiff**:
```go
func (w *Writer) RecordFileDiff(stepID, filePath, beforeContent, afterContent string) string
```
- Generates and saves unified diff
- Returns path to diff file in artifacts directory
- Saved to: `.mooncake/runs/{run-id}/diffs/{stepID}_{filename}.diff`

**RecordFileChecksums**:
```go
func (w *Writer) RecordFileChecksums(stepID, filePath, checksumBefore, checksumAfter string) string
```
- Records before/after checksums
- Returns path to checksum file
- Saved to: `.mooncake/runs/{run-id}/checksums/{stepID}_{filename}.sha256`

#### 4. Directory Structure

Artifacts are now organized as:
```
.mooncake/runs/{run-id}/
├── plan.json              # Execution plan
├── facts.json             # System facts
├── events.jsonl           # Event stream
├── results.json           # Step results with checksums
├── diff.json              # Changed files summary
├── summary.json           # Run summary
├── stdout.log             # Full stdout (if enabled)
├── stderr.log             # Full stderr (if enabled)
├── diffs/                 # NEW: Unified diffs per file
│   ├── step-001_path_to_file.diff
│   └── step-002_another_file.diff
└── checksums/             # NEW: SHA256 checksums
    ├── step-001_path_to_file.sha256
    └── step-002_another_file.sha256
```

### Event Flow

1. **Before File Modification** (action handler):
   - Calculate checksum of existing file
   - Store original content

2. **After File Modification** (action handler):
   - Calculate checksum of modified file
   - Emit event with both checksums
   - Optionally call `RecordFileDiff()` with before/after content

3. **Artifact Writer**:
   - Receives event with checksums
   - Records checksums in FileChange
   - Stores in results.json

### Files Modified

1. **internal/artifacts/writer.go**:
   - Added checksum and diff fields to FileChange
   - Added FilesChanged/DiffFiles to StepResult
   - Added `calculateFileChecksum()` function
   - Added `generateUnifiedDiff()` function
   - Added `RecordFileDiff()` method
   - Added `RecordFileChecksums()` method
   - Added `FileSnapshot` type
   - Updated event handlers to process checksums

2. **internal/events/event.go**:
   - Added ChecksumBefore/ChecksumAfter to FileOperationData

### Usage Example

#### For Action Handlers (Optional Enhancement):

```go
// In a file action handler
beforeChecksum := calculateFileChecksum(path)
beforeContent, _ := os.ReadFile(path)

// ... perform file modification ...

afterChecksum := calculateFileChecksum(path)

// Emit event with checksums
publisher.Publish(events.Event{
    Type: events.EventFileUpdated,
    Data: events.FileOperationData{
        Path:           path,
        ChecksumBefore: beforeChecksum,
        ChecksumAfter:  afterChecksum,
        Changed:        true,
    },
})

// Optionally record diff if artifact writer available
if artifactWriter != nil {
    afterContent, _ := os.ReadFile(path)
    diffFile := artifactWriter.RecordFileDiff(
        stepID, path,
        string(beforeContent),
        string(afterContent),
    )
}
```

#### CLI Usage (Already Available):

```bash
# Enable artifact collection
mooncake run --config plan.yml --artifacts-dir .mooncake

# Enable full output capture
mooncake run --config plan.yml \
  --artifacts-dir .mooncake \
  --capture-full-output

# Results stored in .mooncake/runs/{run-id}/
```

### Benefits for LLM Agents

1. **Diff Analysis**:
   - Review exact changes made to files
   - Understand code modifications line-by-line
   - Verify changes match intent

2. **Integrity Verification**:
   - SHA256 checksums for all file operations
   - Detect unexpected modifications
   - Validate file consistency

3. **Debugging**:
   - Unified diffs show what changed
   - Checksums prove files were modified
   - Event stream provides full audit trail

4. **Iteration**:
   - Next LLM run can review diffs from previous run
   - Learn from past modifications
   - Avoid repeating mistakes

### Next Steps

#### Optional: Enhance Action Handlers
Action handlers can be enhanced to use the new diff/checksum API:

1. **file_replace** - Record diffs of replacements
2. **file_insert** - Record diffs of insertions
3. **file_delete_range** - Record diffs of deletions
4. **template** - Record diffs of template rendering
5. **copy** - Record checksums before/after copy

This can be done incrementally without breaking existing functionality.

#### Future Enhancements
- Diff statistics (lines added/removed/changed)
- Binary file change detection
- Diff compression for large files
- Incremental diff generation

### Testing

All existing tests pass:
```
✓ TestNewWriter
✓ TestWriter_OnEvent_StepCompleted
✓ TestWriter_OnEvent_FileOperations
✓ TestWriter_OnEvent_RunCompleted
✓ All other artifact tests (15 tests total)
```

### Code Statistics
- **Lines added**: ~300 lines
- **Functions added**: 7 new functions
- **Files modified**: 2 (artifacts/writer.go, events/event.go)
- **Backwards compatible**: Yes (all new fields are optional)
- **Breaking changes**: None

---

**Status**: Phase 3 ✅ COMPLETE
**Date**: 2026-02-17
**Ready for**: Phase 4 (Enhanced Assertions)
