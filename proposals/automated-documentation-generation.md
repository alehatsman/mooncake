# Proposal: Automated Documentation Generation from Action Metadata

**Status**: Draft
**Author**: Platform Support Implementation
**Date**: 2026-02-07
**Related**: Platform Support Matrix (#1)

## Summary

Implement automated generation of action documentation tables from runtime metadata to ensure docs stay in sync with code and eliminate manual maintenance overhead.

## Motivation

### Current State

The platform support documentation (`docs-next/guide/platform-support.md`) contains manually-written tables showing action capabilities:

```markdown
| Action | Linux | macOS | Windows | FreeBSD | Notes |
|--------|-------|-------|---------|---------|-------|
| service | ✓ | ✓ | ✓ | ✗ | systemd (Linux), launchd (macOS)... |
| package | ✓ | ✓ | ✓ | ✓ | Multiple package managers... |
```

### Problems with Manual Documentation

1. **Out-of-sync risk** - When action metadata changes, docs can become stale
2. **Manual maintenance** - Every action update requires doc updates
3. **Human error** - Easy to forget updating docs or make mistakes
4. **No validation** - Can't verify docs match code automatically
5. **Duplication** - Same information exists in code and docs

### Opportunity

All action metadata already exists in code:

```go
func (h *Handler) Metadata() actions.ActionMetadata {
    return actions.ActionMetadata{
        Name:               "service",
        SupportedPlatforms: []string{"linux", "darwin", "windows"},
        RequiresSudo:       true,
        ImplementsCheck:    true,
        // ...
    }
}
```

This metadata is accessible via `actions.List()` and the CLI command `mooncake actions list --format json`.

## Proposal

### Option 1: CLI Command (Recommended)

Add a `mooncake docs generate` command that generates markdown documentation from action metadata.

**Usage:**
```bash
# Generate all documentation
mooncake docs generate --output docs-next/guide/

# Generate specific sections
mooncake docs generate --section platform-matrix --output platform-matrix.md
mooncake docs generate --section capabilities --output capabilities.md

# Preview without writing
mooncake docs generate --dry-run
```

**Advantages:**
- Standalone tool, easy to run manually or in CI
- Can be integrated into Makefile
- Dogfooding our own CLI
- Easy to test and debug

**Implementation:**
- Add `cmd/docs.go` with generate command
- Add `internal/docgen/generator.go` for markdown generation
- Support multiple output formats (markdown tables, HTML, JSON schema)

### Option 2: Build-Time Generation

Generate docs during `go generate` using a generator tool.

**Usage:**
```go
//go:generate go run cmd/docgen/main.go -output docs-next/guide/platform-support.md
```

**Advantages:**
- Automatic during build
- Standard Go tooling
- No separate command needed

**Disadvantages:**
- Less visible to users
- Harder to run selectively
- Requires committing generated files

### Option 3: CI/CD Only

Generate docs only in CI pipeline, fail if out of sync.

**Advantages:**
- Enforces docs staying current
- No local tooling needed

**Disadvantages:**
- Doesn't help developers locally
- Slower feedback loop
- Still requires manual regeneration

## Detailed Design (Option 1)

### CLI Interface

```bash
# Command structure
mooncake docs generate [flags]

# Flags
--output, -o      Output directory or file (default: stdout)
--section, -s     Section to generate (platform-matrix, capabilities, all)
--format, -f      Output format (markdown, html, json-schema)
--dry-run         Preview without writing files
--template, -t    Custom template file
```

### Generated Sections

#### 1. Platform Support Matrix

Generated from `SupportedPlatforms` field:

```markdown
| Action | Linux | macOS | Windows | FreeBSD |
|--------|-------|-------|---------|---------|
| service | ✓ | ✓ | ✓ | ✗ |
| package | ✓ | ✓ | ✓ | ✓ |
```

**Source**: `metadata.SupportedPlatforms` (empty = all platforms)

#### 2. Action Capabilities Table

Generated from `RequiresSudo` and `ImplementsCheck` fields:

```markdown
| Action | Requires Sudo | Implements Check |
|--------|---------------|------------------|
| service | Yes | Yes |
| package | Yes | Yes |
| file | No | Yes |
```

**Source**: `metadata.RequiresSudo`, `metadata.ImplementsCheck`

#### 3. Action Summary

Generated from all metadata fields:

```markdown
### service

**Category**: system
**Platforms**: linux, darwin, windows
**Requires Sudo**: Yes
**Implements Check**: Yes
**Supports Dry-Run**: Yes
**Supports Become**: No
**Version**: 1.0.0

**Description**: Manage services across platforms (systemd, launchd, Windows)

**Events Emitted**: service.started, service.stopped
```

### Implementation Plan

#### Phase 1: Core Generator (Week 1)

1. **Add docgen package** (`internal/docgen/`)
   - `generator.go` - Main generator interface
   - `markdown.go` - Markdown formatter
   - `templates.go` - Default templates

2. **Add CLI command** (`cmd/docs.go`)
   - `docs generate` subcommand
   - Flag parsing and validation
   - File output handling

3. **Generate platform matrix**
   - Iterate `actions.List()`
   - Build platform support table
   - Write markdown output

#### Phase 2: Extended Tables (Week 2)

4. **Generate capabilities table**
   - Extract sudo and check metadata
   - Format as markdown table

5. **Generate action summaries**
   - Full metadata for each action
   - Grouped by category
   - Include descriptions and events

#### Phase 3: Templates & CI (Week 3)

6. **Template support**
   - Custom Go templates
   - Allow users to customize output format
   - Examples in `docs/templates/`

7. **CI integration**
   - Add `make docs` target
   - CI check for stale docs
   - Auto-commit updated docs

### Example Output

```bash
$ mooncake docs generate --section platform-matrix

Generating documentation from action metadata...

✓ Loaded 14 actions
✓ Generated platform-matrix.md (234 lines)
✓ Generated capabilities.md (145 lines)

Documentation generation complete!
```

### Template Example

Users can provide custom templates:

```go
// templates/platform-matrix.tmpl
# Platform Support Matrix

Generated on: {{ .Timestamp }}
Mooncake version: {{ .Version }}

{{ range .Actions }}
## {{ .Name }}

Platforms: {{ join .SupportedPlatforms ", " }}
Requires Sudo: {{ .RequiresSudo }}
{{ end }}
```

## Testing Strategy

### Unit Tests

- Test markdown table generation
- Test platform support detection
- Test template rendering
- Test file writing with --dry-run

### Integration Tests

- Generate full docs and compare with fixtures
- Test all CLI flags
- Test error handling (invalid paths, permissions)

### CI Validation

```yaml
# .github/workflows/docs.yml
- name: Generate docs
  run: make docs-generate

- name: Check for changes
  run: |
    if ! git diff --quiet docs-next/; then
      echo "Documentation is out of sync!"
      echo "Run 'make docs-generate' to update"
      exit 1
    fi
```

## Migration Plan

### Phase 1: Side-by-Side (Week 1-2)

- Generate docs to `docs-next/generated/`
- Keep manual docs in `docs-next/guide/`
- Compare outputs for accuracy

### Phase 2: Integration (Week 3)

- Merge generated sections into guide docs
- Add generation markers:
  ```markdown
  <!-- BEGIN GENERATED: platform-matrix -->
  | Action | Linux | macOS |
  ...
  <!-- END GENERATED -->
  ```

### Phase 3: Full Automation (Week 4)

- CI fails on stale docs
- Remove manual table maintenance
- Document the generation process

## Alternatives Considered

### A. Embed docs in code

```go
// ActionDoc returns documentation for this action.
func (h *Handler) ActionDoc() string {
    return `## Service Action

    Manages system services...`
}
```

**Rejected**: Mixes concerns, harder to maintain, not DRY

### B. External doc site generator

Use tools like Hugo or MkDocs with custom plugins.

**Rejected**: Adds complexity, not Go-native, harder to integrate

### C. JSON Schema only

Generate JSON Schema, let users build docs from it.

**Rejected**: Doesn't solve the problem, just shifts burden

## Success Metrics

1. **Zero stale docs** - CI catches any drift within 1 commit
2. **Reduced maintenance** - No manual table updates needed
3. **Faster updates** - Action changes auto-update docs
4. **Developer satisfaction** - Survey shows reduced doc friction
5. **Accuracy** - 100% match between code and docs

## Open Questions

1. **Where should templates live?**
   - In repo: `docs/templates/`
   - User config: `~/.mooncake/templates/`
   - Both with precedence?

2. **How to handle custom documentation?**
   - Use markers like `<!-- BEGIN GENERATED -->`
   - Separate generated vs. manual sections
   - Allow template overrides per section

3. **Should we generate API docs too?**
   - Could extend to generate Go API docs
   - OpenAPI specs from action metadata
   - Future consideration

4. **Version the generated docs?**
   - Commit generated files or generate on-the-fly?
   - Tag releases with matching docs?
   - Docs version should match mooncake version

## Future Extensions

### Phase 4+: Advanced Features

1. **Multi-format output**
   - HTML with search and navigation
   - PDF for offline reading
   - Man pages for CLI help

2. **Interactive docs**
   - Live action metadata API
   - Try-it-in-browser playground
   - Version selector (docs for v0.1, v0.2, etc.)

3. **Validation rules**
   - Detect missing descriptions
   - Require examples for actions
   - Enforce documentation standards

4. **Internationalization**
   - Generate docs in multiple languages
   - Translate descriptions and examples
   - Community contributions

## References

- [Platform Support Implementation](../docs-next/guide/platform-support.md)
- [Action Handler Interface](../internal/actions/handler.go)
- [Actions List Command](../cmd/mooncake.go)
- Similar tools:
  - Kubernetes API docs generation
  - Terraform provider docs
  - Ansible module docs

## Appendix: Proof of Concept

A working prototype demonstrating table generation from metadata:

```bash
#!/bin/bash
# Generate platform support matrix

actions=$(mooncake actions list --format json)

echo "| Action | Linux | macOS | Windows | FreeBSD |"
echo "|--------|-------|-------|---------|---------|"

echo "$actions" | jq -r '.[] |
  "\(.Name) | " +
  (if (.SupportedPlatforms | length) == 0 or
      (.SupportedPlatforms | contains(["linux"]))
   then "✓" else "✗" end) + " | " +
  (if (.SupportedPlatforms | length) == 0 or
      (.SupportedPlatforms | contains(["darwin"]))
   then "✓" else "✗" end) + " | " +
  (if (.SupportedPlatforms | length) == 0 or
      (.SupportedPlatforms | contains(["windows"]))
   then "✓" else "✗" end) + " | " +
  (if (.SupportedPlatforms | length) == 0 or
      (.SupportedPlatforms | contains(["freebsd"]))
   then "✓" else "✗" end)
'
```

This proves the concept works and could be implemented in Go with proper error handling, templates, and CI integration.
