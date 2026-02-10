# Roadmap

## Current Version: v0.3 (Production-Ready) ✅

**Status**: All core features complete, production-ready

- ✅ 14 actions fully implemented
- ✅ Cross-platform support (Linux, macOS, Windows basic)
- ✅ Preset system with parameterization
- ✅ Deterministic plan compiler
- ✅ Facts system (comprehensive)
- ✅ Event system & artifacts
- ✅ Schema auto-generation
- ✅ Handler-based architecture
- ✅ Comprehensive documentation

---

## Recently Completed (February 2026)

### Schema Generation System ✅ (2026-02-09)
**Commit**: 51232f3

- Auto-generated JSON Schema from action metadata
- Full validation support (oneOf, enums, patterns, ranges, additionalProperties)
- Custom x- extensions for IDE support
- CLI command: `mooncake schema generate`
- 8 new files, 10 tests, 64.9% coverage
- Zero manual schema maintenance

**Impact**: IDE autocomplete, validation, always in sync with code

---

## Next Up: v0.4 Planning

### High Priority (Next Sprint)

#### 1. Preset Parameter Collection 🎯
**Status**: ✅ Complete (2026-02-09)
**Implementation**: `cmd/presets.go` (collectParameters function, 170 lines)

**Completed Features**:
- ✅ Interactive parameter prompts with descriptions and help text
- ✅ Type-specific parsing (string, bool, array)
- ✅ Enum validation during input
- ✅ Required parameter enforcement
- ✅ Default value handling
- ✅ Non-interactive mode (`--non-interactive` flag)
- ✅ CLI parameter overrides (`--param key=value`)
- ✅ Parameter precedence (CLI > Interactive > Defaults)
- ✅ User-friendly error messages

**Usage**:
```bash
# Interactive mode (prompts for parameters)
mooncake presets install ollama

# Non-interactive with CLI params
mooncake presets install ollama --non-interactive \
  --param state=present \
  --param pull=[llama3.1:8b,mistral] \
  --param service=true

# CLI override in interactive mode
mooncake presets install ollama --param service=false
```

**Example Output**:
```bash
$ mooncake presets install ollama

Preset: ollama (v1.0.0)
Install and manage Ollama LLM runtime

Parameters:

? state (optional) [string]
  Whether Ollama should be installed or removed
  Options: [present, absent]
  Default: present
  > [Enter to use default]

? pull (optional) [array]
  List of models to pull
  > llama3.1:8b,mistral

Installing ollama with parameters:
  - state: present
  - pull: [llama3.1:8b mistral]
  - service: true
  - method: auto
```

**Impact**: All 330+ presets now support interactive installation with validation

---

#### 2. Documentation Automation Phase 2-3 📚
**Status**: ✅ Phases 1-2 Complete, Phase 3 CI Integration Complete!
**Proposal**: `proposals/automated-documentation-generation.md`

**✅ Completed** (2026-02-09 or earlier):
- ✅ Platform matrix generation (`mooncake docs generate --section platform-matrix`)
- ✅ Capabilities table (dry-run, become, check mode, category)
- ✅ Action summaries by category
- ✅ Preset examples from actual files with syntax validation
- ✅ Schema documentation generation
- ✅ Makefile integration (`make docs-generate`, `make docs-check`)
- ✅ CI integration (`.github/workflows/ci.yml` docs-check job)
- ✅ Drift detection with detailed diffs
- ✅ Comprehensive tests (10 tests in `generator_test.go`)
- ✅ Generated docs in `docs-next/generated/` (3 files, 272 KB)

**❌ Not Implemented** (Optional):
- Custom template support (`--template` flag)
- User-provided Go templates for custom formatting

**Impact**: ✅ **ACHIEVED** - Zero documentation drift, automated maintenance, CI enforcement

**Remaining Effort**: 2-3 days for custom templates (optional, can be deferred to v0.5)

---

### Medium Priority (Next Month)

#### 3. OpenAPI 3.0 Generation 🔌
**Status**: ✅ Complete (2026-02-09)
**Proposal**: `proposals/schema-generation.md`
**Implementation**: `internal/schemagen/openapi.go`, `cmd/schema.go`

**Completed Features**:
- ✅ OpenAPI 3.0.3 spec generation from action metadata
- ✅ Convert JSON Schema to OpenAPI format
- ✅ Custom x- extensions (platforms, capabilities)
- ✅ CLI command: `mooncake schema generate --format openapi`
- ✅ JSON output format
- ✅ Full test coverage (6 tests)
- ✅ Documentation in commands.md

**Usage**:
```bash
mooncake schema generate --format openapi --output openapi.json
```

**Use Cases**:
- API documentation (Swagger UI, ReDoc)
- Client SDK generation (openapi-generator)
- API gateway integration (Kong, Tyk)
- Testing tools (Postman, Insomnia)

**Impact**: Enables API documentation and automated client generation for mooncake configurations

---

#### 4. TypeScript Definitions 📘
**Status**: ✅ Complete (2026-02-09)
**Proposal**: `proposals/schema-generation.md`
**Implementation**: `internal/schemagen/typescript.go`, `cmd/schema.go`

**Completed Features**:
- ✅ Generate `.d.ts` files from action metadata
- ✅ PascalCase interface naming (e.g., `ShellAction`, `ServiceAction`)
- ✅ JSDoc comments with `@platforms`, `@category`, `@values` tags
- ✅ Union types for enums (e.g., `"present" | "absent"`)
- ✅ Complete type coverage for all actions
- ✅ Step interface with all universal and action fields
- ✅ CLI command: `mooncake schema generate --format typescript`
- ✅ Full test coverage (9 tests)
- ✅ Documentation in commands.md

**Command**:
```bash
mooncake schema generate --format typescript --output mooncake.d.ts
```

**Use Cases**:
- Web-based config editors with TypeScript
- VSCode extension development
- Type-safe configuration generation
- IDE autocomplete and validation

**Example Output**:
```typescript
export interface ShellAction {
  cmd?: string;
  capture?: boolean;
  interpreter?: "bash" | "sh" | "pwsh" | "cmd";
}

export interface Step {
  name?: string;
  when?: string;
  shell?: string | ShellAction;
  command?: CommandAction;
  // ... all actions
}

export type MooncakeConfig = Step[];
```

**Impact**: Full TypeScript support for web-based config tools and IDE extensions

---

#### 5. Expression Evaluation Enhancements 🧮
**Status**: ✅ Complete (2026-02-10)
**Implementation**: `internal/expression/functions.go`, `functions_test.go`, `evaluator.go`

**Completed Features**:
- ✅ 40+ custom functions across 5 categories
- ✅ String functions: starts_with, ends_with, lower, upper, trim, split, join, replace, regex_match
- ✅ Math functions: min, max, abs, floor, ceil, round, pow, sqrt
- ✅ Collection functions: len, includes, empty, first, last
- ✅ Type checking: is_string, is_number, is_bool, is_array, is_map, is_defined
- ✅ Utility functions: default, env, has_env, coalesce, ternary
- ✅ Enhanced error messages with helpful hints
- ✅ Comprehensive test coverage (106 tests)

**Impact**: More flexible and powerful conditional logic in configurations

**Actual Effort**: 2 hours (estimated 2 weeks, 95% time saved!)

---

#### 6. TUI ANSI Code Handling 🎨
**Status**: ✅ Complete (2026-02-09)
**Implementation**: `internal/logger/ansi.go`, `tui_display.go`

**Completed Features**:
- ✅ ANSI escape code stripping (`StripANSI`)
- ✅ Visible length calculation (`VisibleLength`)
- ✅ ANSI-aware truncation (`TruncateANSI`)
- ✅ UTF-8 character support (emoji, unicode)
- ✅ Preserves color codes during truncation
- ✅ Handles complex SGR sequences
- ✅ CSI and OSC sequence support
- ✅ Comprehensive test coverage (50+ tests)

**Functions**:
```go
StripANSI(s string) string              // Remove ANSI codes
VisibleLength(s string) int              // Count visible characters
TruncateANSI(s string, maxWidth int) string  // Truncate preserving ANSI
```

**Example**:
```go
// Before:
truncate("\x1b[31mLong Red Text\x1b[0m", 8)
// Returns (incorrect): "\x1b[31mLo..." (byte-based, breaks display)

// After:
TruncateANSI("\x1b[31mLong Red Text\x1b[0m", 8)
// Returns (correct): "\x1b[31mLong..." (visible-based, preserves colors)
```

**Impact**: TUI now properly handles colored command output without display corruption

---

### Low Priority / Future Enhancements

#### 7. Global Variables ✅
**Status**: Complete (2026-02-10) - Phase 1
**Implementation**: `internal/config/config.go`, `internal/config/reader.go`, `internal/plan/planner.go`

**Completed Features**:
- ✅ Global variables defined at config level (`vars:` field)
- ✅ ParsedConfig struct to hold Steps, GlobalVars, and Version
- ✅ Reader returns ParsedConfig instead of []Step
- ✅ Backward compatibility for old array format
- ✅ Planner merges global vars with CLI vars and system facts
- ✅ Templates can use global vars (e.g., `{{app_name}}`)
- ✅ Conditions can reference global vars (e.g., `when: environment == "prod"`)
- ✅ CLI variables override global vars
- ✅ Comprehensive tests for global variables
- ✅ Example: `examples/global-variables-example.yml`

**Usage**:
```yaml
version: "1.0"
vars:
  app_name: "myapp"
  environment: "production"
  port: 8080

steps:
  - name: Display config
    shell: echo "{{app_name}} on port {{port}}"

  - name: Production only
    shell: echo "Deploying to prod"
    when: environment == "production"
```

**Impact**: Global variables simplify multi-step configurations by eliminating repetition

#### 8. Config Versioning 📋
**Status**: Placeholder

**Scope**:
- `runConfig.Version` for schema versioning (field exists but not validated)
- Version validation and compatibility checks
- Migration tool for version upgrades

**Estimated Effort**: 1-2 weeks

---

#### 8. Schema Marketplace Integration 🏪
**Status**: Future enhancement
**From**: Schema generation proposal Phase 4+

**Scope**:
- Publish schema to schemastore.org
- Auto-discovery in VSCode and other IDEs
- Version-specific schema URLs
- Schema validation in web tools

**Benefits**: Zero-config IDE integration

**Estimated Effort**: 1 week (mostly coordination)

---

#### 9. Conditional/Platform-Specific Schemas 🔀

---

#### 10. Windows Service Management Completion 🪟
**Status**: Deferred - Linux/macOS complete and production-ready
**Scope**:
- Complete Windows service action implementation
- Windows facts: disk information (wmic/PowerShell)
- Windows facts: GPU detection (wmic/PowerShell)
- Full parity with Linux (systemd) and macOS (launchd)

**TODOs**:
```
internal/facts/windows.go:
  - Line 24: TODO: Implement more Windows-specific fact collection
  - Line 86: TODO: Use wmic or PowerShell to get disk info
  - Line 96: TODO: Use wmic or PowerShell to get GPU info
```

**Impact**: Complete cross-platform support

**Estimated Effort**: 1-2 weeks

**Note**: Basic Windows support exists. Full Windows parity deferred until demand increases.
**Status**: Future enhancement

**Scope**:
- Different schema properties based on OS
- Platform-specific validation rules
- Example: `service.unit` only on Linux

**Benefits**: More accurate validation per platform

**Estimated Effort**: 2-3 weeks

---

## Proposals Status

### ✅ Implemented
1. **Schema Generation** - Phases 1-2 complete (2026-02-09)
2. **Automated Documentation** - Phases 1-3 complete (pre-2026-02-09)
   - Full CLI: `mooncake docs generate`
   - All sections: platform-matrix, capabilities, action-summary, preset-examples, schema
   - CI integration with drift detection
   - Makefile targets: `make docs-generate`, `make docs-check`
   - Only missing: custom template support (optional)
3. **OpenAPI 3.0 Generation** - Complete (2026-02-09)
   - OpenAPI 3.0.3 spec generation
   - CLI: `mooncake schema generate --format openapi`
   - Client SDK foundation
   - Full test coverage
4. **Preset Parameter Collection** - Complete (2026-02-09)
   - Interactive parameter prompts
   - Non-interactive mode for CI/CD
   - CLI parameter overrides
   - Full validation support
5. **TypeScript Definitions** - Complete (2026-02-09)
   - TypeScript `.d.ts` generation
   - CLI: `mooncake schema generate --format typescript`
   - PascalCase interfaces with JSDoc
   - Union types for enums
   - Full test coverage (9 tests)
6. **TUI ANSI Code Handling** - Complete (2026-02-09)
   - ANSI escape code handling
   - Visible length calculation
   - ANSI-aware truncation
   - UTF-8/Unicode support
   - 50+ tests, all passing
7. **Expression Evaluation Enhancements** - Complete (2026-02-10)
   - 40+ custom functions (string, math, collection, type, utility)
   - Enhanced error messages
   - 106 comprehensive tests
   - Snake_case naming for expr compatibility
8. **Global Variables** - Complete (2026-02-10)
   - ParsedConfig struct with Steps, GlobalVars, and Version
   - Global variables at config level (`vars:` field)
   - Backward compatibility for array format
   - Full template and condition support
   - CLI variable override support
   - Example: `examples/global-variables-example.yml`

### 🔄 In Progress
None currently

### 📋 Planned
1. **Config Versioning** - Schema version validation (1-2 weeks)
2. **Windows Completion** - Platform parity (low priority, 1-2 weeks)
3. **Custom Template Support** - Documentation generation (optional, 2-3 days)

### 💡 Future Ideas
1. **Web-based Config Builder** - Visual preset editor
2. **Plugin System** - Third-party action extensions
3. **Remote Execution** - Execute over SSH/WinRM
4. **GUI Application** - Desktop app for configuration
5. **Schema Diff Tool** - Migration assistance
6. **Performance Profiling** - Execution optimization

---

## Version Planning

### v0.4 (Planned - Q2 2026)
**Theme**: Schema Ecosystem & Polish

- ✅ Preset parameter collection (~1 week)
- ✅ OpenAPI 3.0 generation (~2 weeks)
- ✅ TypeScript definitions (~1-2 weeks)
- ✅ TUI improvements (~1 week)
- ✅ Expression evaluation enhancements (~2 weeks)
- 🔧 Custom template support for docs (optional, ~2-3 days)

**Timeline**: 3-4 weeks (revised down from 4-6 weeks)

---

### v0.5 (Planned - Q3 2026)
**Theme**: Advanced Features

- ✅ Web-based config validator
- ✅ Schema marketplace integration
- ✅ Global variables support
- ✅ Preset library expansion
- ✅ Performance optimizations

**Timeline**: 4-6 weeks

---

### v1.0 (Future)
**Theme**: Production Hardening

- ✅ Performance optimizations
- ✅ Advanced error recovery
- ✅ Comprehensive audit logging
- ✅ Enterprise features (RBAC, compliance)
- ✅ Plugin ecosystem

**Timeline**: TBD

---

## Contributing to the Roadmap

Want to contribute? Here's how:

1. **Pick a TODO** - Check code for `TODO` comments
2. **Propose a feature** - Open an issue with `[Proposal]` tag
3. **Improve documentation** - PRs always welcome
4. **Report bugs** - Help us improve quality
5. **Write presets** - Expand the preset library

**High-impact areas**:
- ✅ Preset parameter collection (complete)
- ✅ Documentation automation (complete)
- ✅ OpenAPI/TypeScript generation (complete)
- Expression evaluation enhancements
- TUI improvements
- Platform-specific validation
- Preset library expansion

---

## Project Health

**Current State**: Production-ready v0.3
- Core features: 100% complete
- Primary platforms: Linux ✅, macOS ✅ (Windows: basic support)
- Documentation: Up-to-date (auto-generated)
- Testing: Comprehensive (300+ tests)
- Maintenance: Minimal (auto-generated schema/docs)

**Next Milestone**: v0.4 - Schema Ecosystem & Polish (4-6 weeks)

**Focus**: Linux/macOS production use cases, schema ecosystem expansion
