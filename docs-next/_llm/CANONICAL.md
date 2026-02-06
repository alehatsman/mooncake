# Mooncake Documentation Bundle

This file contains the complete canonical documentation for Mooncake.
All duplicate content has been removed.

---

<!-- FILE: about/changelog.md -->

# Changelog

## v0.3 - Current

### Breaking Changes
- **Ollama action removed** - Replaced with `preset: ollama`
  - Old syntax: `ollama: {state: present, ...}`
  - New syntax: `preset: {name: ollama, with: {state: present, ...}}`
  - Migration: Update your configs to use preset syntax
  - Benefit: 81% code reduction, user-extensible workflows

### Preset System
- **Preset action** - Reusable, parameterized workflows
  - Package complex workflows as YAML files
  - Type-safe parameters with validation (string, bool, array, object)
  - Parameter namespacing prevents collisions
  - Discovery paths: `./presets/`, `~/.mooncake/presets/`, system paths
  - Support for both flat (`name.yml`) and directory (`name/preset.yml`) formats
  - Full integration with planner (includes, loops, conditionals work in presets)
  - Aggregate result tracking (changed = any step changed)
- **Ollama preset** - AI runtime management
  - Replaces 1,400 lines of Go code with 250 lines of YAML
  - Multi-platform installation (apt, dnf, yum, brew, script fallback)
  - Service configuration with environment variables
  - Model management (pull, force pull)
  - Uninstallation with optional model cleanup
- **Preset authoring** - User extensibility
  - Create custom presets without Go knowledge
  - Share presets as files (git, package managers, direct distribution)
  - Template rendering in preset steps
  - Platform-aware logic using system facts

### New Actions
- **Print action** - Simple output to console
  - Display messages during execution
  - Variable interpolation support
  - Useful for debugging and user feedback
- **Package action** - Package management
  - Cross-platform package installation
  - Support for apt, dnf, yum, brew, and more
  - State management (present/absent/latest)
  - Idempotent operations
- **Service management** (systemd on Linux, launchd on macOS)
  - Full lifecycle control (start, stop, restart, reload)
  - Enable/disable services on boot
  - Unit/plist file management with templates
  - Drop-in configuration files (systemd)
  - Idempotent operations with change detection
- **Assert action** for state verification
  - Command assertions (exit code verification)
  - File assertions (existence, content, permissions, ownership)
  - HTTP assertions (status codes, response body validation)
  - Always returns changed: false (verify without modifying)
  - Fail-fast behavior with detailed error messages
- **Download action** - Download files from URLs
  - Checksum verification (SHA256, MD5)
  - Idempotent downloads (skip if exists, verify checksums)
  - Timeout support
- **Unarchive action** - Extract archives
  - Support for tar, tar.gz, zip formats
  - Automatic format detection
  - Idempotent extraction
  - Permission preservation
- **Command action** - Structured command execution
  - Multiple interpreter support (bash, sh, pwsh, cmd)
  - Stdin input support
  - Output capture control
- **Copy action** - Advanced file copying
  - Recursive directory copying
  - Content filtering with glob patterns
  - Backup creation before overwrite
  - Checksum-based change detection
  - Permission and ownership preservation
- **Enhanced file action** - Extended file management
  - Multiple states: file, directory, absent, touch, link, hardlink, perms
  - Ownership management (owner/group)
  - Permission management with recursive option
  - Backup creation before modifications
  - Symbolic and hard link creation
  - Safety checks (prevent accidental root deletion)

### Planning & Execution
- **Deterministic plan compiler**
  - Expand all loops and includes into linear plan
  - Origin tracking (file:line:col) for every step
  - Export plans as JSON/YAML
  - Execute from saved plans (`--from-plan`)
  - Plan inspection with `mooncake plan` command
  - Tag filtering at plan time
- **Artifacts system** - Persist execution data
  - Run results and summaries
  - Event stream capture
  - Full stdout/stderr capture (`--capture-full-output`)
  - Unique run IDs with timestamps
  - Structured artifact directory (`.mooncake/runs/<runid>/`)
- **Event system** - Real-time observability
  - Structured event emission (JSON)
  - Full lifecycle events (run start, step execution, completion)
  - Integration-friendly for monitoring and auditing
  - JSON output format for CI/CD (`--output-format json`)

### System Facts
- **Comprehensive system facts collection**
  - CPU: model, cores, instruction set flags (AVX, AVX2, SSE4_2, FMA, AES)
  - Memory: total/free, swap total/free
  - GPU detection: NVIDIA (nvidia-smi), AMD (rocm-smi), Intel/Apple
  - Disk information: mounts, filesystem type, size/used/available
  - Network: interfaces with MAC/IP, default gateway, DNS servers
  - Toolchain versions: Docker, Git, Go, Python
  - Package manager detection (apt, dnf, yum, brew, pacman, zypper, apk, port)
  - Distribution info: name, version, kernel
  - `mooncake facts` command with text/JSON output

### Execution Control & Flow
- **Timeout support** - Prevent long-running commands
  - Per-step timeout configuration
  - Standard exit code (124) on timeout
- **Retry logic** - Automatic retry on failure
  - Configurable retry count and delay
  - Works with shell, command, download actions
- **Idempotency controls**
  - `creates` - Skip step if path exists
  - `unless` - Skip step if command succeeds
- **Custom result evaluation**
  - `changed_when` - Override change detection
  - `failed_when` - Define custom failure conditions

### Template & Expression Language
- **Expression language (expr-lang)** - Powerful expression evaluation
  - Used in `when`, `changed_when`, `failed_when`, `with_items`
  - Rich operators and functions
  - Type-safe evaluation
- **New template functions**
  - `has()` - Check if map contains key
  - All facts available as template variables

### Security & Privilege Escalation
- **Improved sudo handling**
  - Interactive password prompt (`-K`, `--ask-become-pass`) - recommended
  - File-based password (`--sudo-pass-file`) with 0600 permission check
  - CLI password with explicit opt-in (`--insecure-sudo-pass`)
  - Better error messages for privilege escalation failures

### User Interface & Output
- **Animated TUI** with progress tracking
  - Real-time step status updates
  - Spinner animations
  - Color-coded results
  - Optional raw mode (`--raw`) for CI/CD
- **Dry-run mode** for safe preview
  - Preview all changes without execution
  - Shows what would be changed
  - Template rendering validation
- **Execution time tracking**
  - Per-step duration
  - Total run time
  - Performance metrics in summary

### Documentation & Architecture
- **Architecture Decision Records (ADRs)** - Documented key design decisions
  - ADR-001: Handler-Based Action Architecture
  - ADR-002: Preset Expansion System
  - ADR-003: Planner and Execution Model
- **AI Specification** - Guidelines for AI agents using mooncake
  - Safety guidelines and best practices
  - Action risk assessment matrix
  - Idempotency patterns and examples
- **Comprehensive guides**
  - Using Presets guide (600+ lines)
  - Preset Authoring guide (800+ lines)
  - Quick-start examples for all major features
- **Documentation improvements**
  - Restructured navigation with feature-first approach
  - Professional documentation style (removed emojis)
  - Streamlined quick-start guide with copyable examples
  - API documentation for programmatic usage
  - Positioned as "The Standard Runtime for AI System Configuration"

### Developer Experience
- **Handler-based action architecture** - Modular action system
  - Each action self-contained in one file (100-1000 lines)
  - Registry pattern for automatic action discovery
  - No dispatcher updates needed for new actions
  - Net reduction of ~16,000 lines of code
- **Testing infrastructure**
  - Docker-based testing for multiple Linux distributions
  - Comprehensive test suite covering all actions and utilities
  - Integration tests for sudo, file operations, and system interactions
  - Race detector enabled in CI
  - Test coverage improvements across the codebase
- **Code quality tooling**
  - Security scanning (gosec) with zero issues
  - Vulnerability checking (govulncheck)
  - Linter integration with automatic fixes
  - Custom error types with structured error handling
- **Build and release automation**
  - Multi-platform builds (Linux, macOS, Windows)
  - Automated GitHub releases
  - Version management and changelog generation
- **Configuration validation**
  - JSON Schema-based validation with detailed error messages
  - Template syntax validation at parse time
  - Type checking for all action parameters
  - Clear error messages with file:line:col references
- **CLI improvements**
  - `mooncake explain` command for system fact inspection
  - `mooncake validate` command for config validation
  - `mooncake plan` command for plan inspection
  - Better help text and usage examples
  - Improved flag organization and defaults

## v0.2

### Features
- **Loop iteration**
  - `with_items` - Iterate over lists/arrays
  - `with_filetree` - Recursively iterate over directory contents
  - Loop variables: `item`, `item.index`, `item.first`, `item.last`
- **Tag filtering** - Filter execution by tags with `--tags`
- **Register** - Capture step output for reuse
  - Capture stdout, stderr, rc (exit code), changed, failed, skipped
  - Use in subsequent steps via variables
  - Works with all action types
- **Expression language migration** - Migrated from govaluate to expr-lang
  - Nested field access (e.g., `result.stdout`)
  - Built-in functions: len(), contains(), string operations
  - Better error messages and type safety

## v0.1 - Initial Release

### Features
- **Shell command execution** - Run shell commands
- **File and directory operations** - Basic file/directory creation and management
- **Template rendering** - Pongo2 template engine with variable substitution
- **Variables** - Define and use variables in configurations
- **Basic system facts** - OS, architecture, hostname, user home
- **Conditionals** - Execute steps conditionally with `when`
- **Include files** - Split configurations across multiple files
- **Include variables** - Load variables from separate YAML files
- **Sudo/privilege escalation** - Run steps with elevated privileges using `become`


---

<!-- FILE: about/license.md -->

# License

Mooncake is released under the MIT License.

## MIT License

Copyright (c) 2026 Aleh Atsman

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

---

## Third-Party Licenses

Mooncake uses the following open-source libraries:

- [pongo2](https://github.com/flosch/pongo2) - Django-syntax templating
- [expr-lang](https://github.com/expr-lang/expr) - Expression evaluation
- [cobra](https://github.com/spf13/cobra) - CLI framework
- [lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal UI styling

See their respective repositories for license information.


---

<!-- FILE: api/actions.md -->

<!-- Code generated by gomarkdoc. DO NOT EDIT -->

# actions

```go
import "github.com/alehatsman/mooncake/internal/actions"
```

Package actions provides the action handler system for mooncake.

### Overview

The actions package defines a standard interface \(Handler\) that all action implementations must follow, along with a registry for discovering handlers at runtime.

### Architecture

Actions are implemented as packages under internal/actions/. Each action package provides a Handler implementation that is registered globally on import via an init\(\) function.

The executor looks up handlers from the registry based on the action type determined from the step configuration.

### Backward Compatibility

This new system is designed to work alongside the existing action implementations. The Step struct retains all existing action fields \(Shell, File, Template, etc.\), and actions are migrated incrementally to the new Handler interface.

### Creating a New Action

To create a new action handler:

1. Create a package under internal/actions/ \(e.g., internal/actions/notify\)

2. Implement the Handler interface:

```
type Handler struct{}

func (h *Handler) Metadata() actions.ActionMetadata {
    return actions.ActionMetadata{
        Name:           "notify",
        Description:    "Send notifications",
        Category:       actions.CategorySystem,
        SupportsDryRun: true,
    }
}

func (h *Handler) Validate(step *config.Step) error {
    // Validate step.Notify config
    return nil
}

func (h *Handler) Execute(ctx *executor.ExecutionContext, step *config.Step) (*executor.Result, error) {
    // Implement action logic
    return &executor.Result{Changed: true}, nil
}

func (h *Handler) DryRun(ctx *executor.ExecutionContext, step *config.Step) error {
    ctx.Logger.Infof("  [DRY-RUN] Would send notification")
    return nil
}
```

3. Register the handler in init\(\):

```
func init() {
    actions.Register(&Handler{})
}
```

4. Import the package in the executor to ensure registration:

```
import _ "github.com/alehatsman/mooncake/internal/actions/notify"
```

### Migration Strategy

Existing actions are being migrated incrementally:

- Phase 1: Create Handler implementations for simple actions \(print, vars\)
- Phase 2: Migrate complex actions \(shell, file, template\)
- Phase 3: Migrate specialized actions \(service, assert, preset\)
- Phase 4: Remove legacy code paths

During migration, both old and new implementations coexist. The executor checks if a handler is registered and prefers it, falling back to legacy implementations for non\-migrated actions.

Package actions provides the handler interface and registry for mooncake actions.

The actions package defines a standard interface that all action handlers must implement, along with a registry system for discovering and dispatching to handlers at runtime.

To create a new action handler:

1. Create a new package under internal/actions \(e.g., internal/actions/notify\)
2. Implement the Handler interface
3. Register your handler in an init\(\) function
4. The handler will be automatically available for use

Example:

```
package notify

import "github.com/alehatsman/mooncake/internal/actions"

type Handler struct{}

func init() {
    actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
    return actions.ActionMetadata{
        Name:        "notify",
        Description: "Send notifications",
        Category:    actions.CategorySystem,
    }
}

// ... implement other interface methods
```

## Index

- [func Count\(\) int](<#Count>)
- [func Has\(actionType string\) bool](<#Has>)
- [func Register\(handler Handler\)](<#Register>)
- [type ActionCategory](<#ActionCategory>)
- [type ActionMetadata](<#ActionMetadata>)
  - [func List\(\) \[\]ActionMetadata](<#List>)
- [type Context](<#Context>)
- [type Handler](<#Handler>)
  - [func Get\(actionType string\) \(Handler, bool\)](<#Get>)
  - [func NewHandlerFunc\(metadata ActionMetadata, validate func\(\*config.Step\) error, execute func\(Context, \*config.Step\) \(Result, error\), dryRun func\(Context, \*config.Step\) error\) Handler](<#NewHandlerFunc>)
- [type HandlerFunc](<#HandlerFunc>)
  - [func \(h \*HandlerFunc\) DryRun\(ctx Context, step \*config.Step\) error](<#HandlerFunc.DryRun>)
  - [func \(h \*HandlerFunc\) Execute\(ctx Context, step \*config.Step\) \(Result, error\)](<#HandlerFunc.Execute>)
  - [func \(h \*HandlerFunc\) Metadata\(\) ActionMetadata](<#HandlerFunc.Metadata>)
  - [func \(h \*HandlerFunc\) Validate\(step \*config.Step\) error](<#HandlerFunc.Validate>)
- [type Registry](<#Registry>)
  - [func NewRegistry\(\) \*Registry](<#NewRegistry>)
  - [func \(r \*Registry\) Count\(\) int](<#Registry.Count>)
  - [func \(r \*Registry\) Get\(actionType string\) \(Handler, bool\)](<#Registry.Get>)
  - [func \(r \*Registry\) Has\(actionType string\) bool](<#Registry.Has>)
  - [func \(r \*Registry\) List\(\) \[\]ActionMetadata](<#Registry.List>)
  - [func \(r \*Registry\) Register\(handler Handler\)](<#Registry.Register>)
- [type Result](<#Result>)


<a name="Count"></a>
## func [Count](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/registry.go#L135>)

```go
func Count() int
```

Count returns the number of handlers in the global registry.

<a name="Has"></a>
## func [Has](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/registry.go#L130>)

```go
func Has(actionType string) bool
```

Has checks if a handler exists in the global registry.

<a name="Register"></a>
## func [Register](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/registry.go#L115>)

```go
func Register(handler Handler)
```

Register registers a handler in the global registry. This is the most common way to register handlers from init\(\) functions.

Example:

```
func init() {
    actions.Register(&MyHandler{})
}
```

<a name="ActionCategory"></a>
## type [ActionCategory](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/handler.go#L40>)

ActionCategory groups related actions by their primary function.

```go
type ActionCategory string
```

<a name="CategoryCommand"></a>

```go
const (
    // CategoryCommand represents actions that execute commands (shell, command)
    CategoryCommand ActionCategory = "command"

    // CategoryFile represents actions that manipulate files (file, template, copy, download)
    CategoryFile ActionCategory = "file"

    // CategorySystem represents system-level actions (service, assert, preset)
    CategorySystem ActionCategory = "system"

    // CategoryData represents data manipulation actions (vars, include_vars)
    CategoryData ActionCategory = "data"

    // CategoryNetwork represents network-related actions (download, http requests)
    CategoryNetwork ActionCategory = "network"

    // CategoryOutput represents output/display actions (print)
    CategoryOutput ActionCategory = "output"
)
```

<a name="ActionMetadata"></a>
## type [ActionMetadata](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/handler.go#L63-L84>)

ActionMetadata describes an action type and its capabilities.

```go
type ActionMetadata struct {
    // Name is the action name as it appears in YAML (e.g., "shell", "file", "notify")
    Name string

    // Description is a human-readable description of what this action does
    Description string

    // Category groups related actions (command, file, system, etc.)
    Category ActionCategory

    // SupportsDryRun indicates whether this action can be executed in dry-run mode
    SupportsDryRun bool

    // SupportsBecome indicates whether this action supports privilege escalation (sudo)
    SupportsBecome bool

    // EmitsEvents lists the event types this action emits (e.g., "file.created", "notify.sent")
    EmitsEvents []string

    // Version is the action implementation version (semantic versioning)
    Version string
}
```

<a name="List"></a>
### func [List](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/registry.go#L125>)

```go
func List() []ActionMetadata
```

List returns all handlers from the global registry.

<a name="Context"></a>
## type [Context](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/interfaces.go#L43-L121>)

Context provides the execution environment for action handlers.

Context is the primary interface through which handlers interact with the mooncake runtime. It provides access to:

- Template rendering \(Jinja2\-like syntax with variables and filters\)
- Expression evaluation \(when/changed\_when/failed\_when conditions\)
- Logging \(structured output to TUI or text\)
- Variables \(step vars, global vars, facts, registered results\)
- Event publishing \(for observability and artifacts\)
- Execution mode \(dry\-run vs actual execution\)

This interface avoids circular imports between actions and executor packages.

Example usage in a handler:

```
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
    // Render template strings
    path, err := ctx.GetTemplate().RenderString(step.File.Path, ctx.GetVariables())

    // Log progress
    ctx.GetLogger().Infof("Creating file at %s", path)

    // Emit events for observability
    ctx.GetEventPublisher().Publish(events.Event{
        Type: events.EventFileCreated,
        Data: events.FileOperationData{Path: path},
    })

    // Return result
    result := executor.NewResult()
    result.SetChanged(true)
    return result, nil
}
```

```go
type Context interface {
    // GetTemplate returns the template renderer for processing Jinja2-like templates.
    //
    // Use this to render:
    //   - Path strings with variables: "{{ home }}/{{ item }}"
    //   - Content with logic: "{% if os == 'linux' %}...{% endif %}"
    //   - Filters: "{{ path | expanduser }}"
    //
    // The renderer has access to all variables in scope (step vars, globals, facts).
    GetTemplate() template.Renderer

    // GetEvaluator returns the expression evaluator for conditions.
    //
    // Use this to evaluate:
    //   - when: "os == 'linux' && arch == 'amd64'"
    //   - changed_when: "result.rc == 0 and 'changed' in result.stdout"
    //   - failed_when: "result.rc != 0 and result.rc != 5"
    //
    // Returns interface{} which should be cast to bool for conditions.
    GetEvaluator() expression.Evaluator

    // GetLogger returns the logger for handler output.
    //
    // Use levels appropriately:
    //   - Infof: User-visible progress ("Installing package nginx")
    //   - Debugf: Detailed info ("Command: apt install nginx")
    //   - Warnf: Non-fatal issues ("File already exists, skipping")
    //   - Errorf: Failures ("Failed to create directory: permission denied")
    //
    // Output is formatted for TUI or text mode automatically.
    GetLogger() logger.Logger

    // GetVariables returns all variables in the current scope.
    //
    // Includes:
    //   - Step-level vars (defined in step.Vars)
    //   - Global vars (from vars actions)
    //   - System facts (os, arch, cpu_cores, memory_total_mb, etc.)
    //   - Registered results (from register: field on previous steps)
    //   - Loop context (item, item_index when in with_items/with_filetree)
    //
    // Keys are strings, values are interface{} (string, int, bool, []interface{}, map[string]interface{}).
    GetVariables() map[string]interface{}

    // GetEventPublisher returns the event publisher for observability.
    //
    // Emit events for:
    //   - State changes (EventFileCreated, EventServiceStarted)
    //   - Progress tracking (custom events for long operations)
    //   - Artifact generation (paths to created files)
    //
    // Events are consumed by:
    //   - Artifact collector (for rollback support)
    //   - External observers (CI/CD integrations)
    //   - Audit logs
    GetEventPublisher() events.Publisher

    // IsDryRun returns true if this is a dry-run execution.
    //
    // In dry-run mode:
    //   - Handlers MUST NOT make actual changes
    //   - Handlers SHOULD log what would happen
    //   - Template rendering should still work (to validate syntax)
    //   - File existence checks are OK (read-only operations)
    //   - Writing/deleting/executing is NOT OK
    //
    // The DryRun() method handles this automatically, but Execute() can also check.
    IsDryRun() bool

    // GetCurrentStepID returns the unique ID of the currently executing step.
    //
    // Format: "step-{global_step_number}"
    //
    // Use this when:
    //   - Emitting events (so they're associated with the step)
    //   - Creating temporary files (include step ID to avoid conflicts)
    //   - Logging (though step ID is usually added automatically)
    GetCurrentStepID() string
}
```

<a name="Handler"></a>
## type [Handler](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/handler.go#L96-L129>)

Handler defines the interface that all action handlers must implement.

A handler is responsible for:

- Validating action configuration
- Executing the action
- Handling dry\-run mode
- Emitting appropriate events
- Returning results

Handlers should be stateless \- all execution state is passed via ExecutionContext.

```go
type Handler interface {
    // Metadata returns metadata describing this action type.
    Metadata() ActionMetadata

    // Validate checks if the step configuration is valid for this action.
    // This is called before Execute to fail fast on configuration errors.
    // Returns an error if validation fails.
    Validate(step *config.Step) error

    // Execute runs the action and returns a result.
    // The result includes whether the action made changes, output data,
    // and any error information.
    //
    // Handlers should:
    //   - Emit appropriate events via ctx.GetEventPublisher()
    //   - Handle template rendering via ctx.GetTemplate()
    //   - Use ctx.GetLogger() for logging
    //   - Return a Result with Changed=true if the action modified state
    //
    // If an error occurs, return it - the executor will handle result registration.
    Execute(ctx Context, step *config.Step) (Result, error)

    // DryRun logs what would happen if Execute were called, without making changes.
    // This is called when the executor is in dry-run mode.
    //
    // Handlers should:
    //   - Use ctx.GetLogger() to describe what would happen
    //   - Attempt to render templates (but catch errors gracefully)
    //   - NOT make any actual changes to the system
    //   - NOT emit action-specific events (step lifecycle events are handled by executor)
    //
    // Returns an error only if dry-run simulation fails catastrophically.
    DryRun(ctx Context, step *config.Step) error
}
```

<a name="Get"></a>
### func [Get](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/registry.go#L120>)

```go
func Get(actionType string) (Handler, bool)
```

Get retrieves a handler from the global registry.

<a name="NewHandlerFunc"></a>
### func [NewHandlerFunc](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/handler.go#L141-L146>)

```go
func NewHandlerFunc(metadata ActionMetadata, validate func(*config.Step) error, execute func(Context, *config.Step) (Result, error), dryRun func(Context, *config.Step) error) Handler
```

NewHandlerFunc creates a Handler from function implementations.

<a name="HandlerFunc"></a>
## type [HandlerFunc](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/handler.go#L133-L138>)

HandlerFunc is a function type that implements Handler for simple actions. This allows creating handlers without defining a new type.

```go
type HandlerFunc struct {
    // contains filtered or unexported fields
}
```

<a name="HandlerFunc.DryRun"></a>
### func \(\*HandlerFunc\) [DryRun](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/handler.go#L170>)

```go
func (h *HandlerFunc) DryRun(ctx Context, step *config.Step) error
```



<a name="HandlerFunc.Execute"></a>
### func \(\*HandlerFunc\) [Execute](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/handler.go#L166>)

```go
func (h *HandlerFunc) Execute(ctx Context, step *config.Step) (Result, error)
```



<a name="HandlerFunc.Metadata"></a>
### func \(\*HandlerFunc\) [Metadata](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/handler.go#L155>)

```go
func (h *HandlerFunc) Metadata() ActionMetadata
```



<a name="HandlerFunc.Validate"></a>
### func \(\*HandlerFunc\) [Validate](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/handler.go#L159>)

```go
func (h *HandlerFunc) Validate(step *config.Step) error
```



<a name="Registry"></a>
## type [Registry](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/registry.go#L36-L39>)

Registry manages registered action handlers through a thread\-safe map.

The registry pattern enables:

1. Dynamic action discovery \- handlers register themselves via init\(\)
2. Loose coupling \- executor doesn't import all action packages
3. Extensibility \- new actions added without changing executor
4. Thread safety \- concurrent access from multiple goroutines

Registration flow:

1. Action package imports actions: import "github.com/.../internal/actions"
2. Action package defines handler: type Handler struct\{\}
3. Action package registers in init\(\): func init\(\) \{ actions.Register\(&Handler\{\}\) \}
4. Main imports register package: import \_ "github.com/.../internal/register"
5. Register package imports all actions: import \_ ".../actions/shell"
6. All handlers automatically registered before main\(\) runs

Lookup flow:

1. Executor determines action type from step: actionType := step.DetermineActionType\(\)
2. Executor queries registry: handler, ok := actions.Get\(actionType\)
3. If found, executor calls: handler.Validate\(step\), handler.Execute\(ctx, step\)
4. If not found, executor falls back to legacy implementation

This avoids circular imports because:

- actions package defines Handler interface
- action implementations \(shell, file, etc.\) import actions
- executor imports actions but NOT action implementations
- register package imports action implementations \(triggers init\(\)\)
- cmd imports register \(triggers all registrations\)

```go
type Registry struct {
    // contains filtered or unexported fields
}
```

<a name="NewRegistry"></a>
### func [NewRegistry](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/registry.go#L42>)

```go
func NewRegistry() *Registry
```

NewRegistry creates a new action registry.

<a name="Registry.Count"></a>
### func \(\*Registry\) [Count](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/registry.go#L97>)

```go
func (r *Registry) Count() int
```

Count returns the number of registered handlers.

<a name="Registry.Get"></a>
### func \(\*Registry\) [Get](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/registry.go#L66>)

```go
func (r *Registry) Get(actionType string) (Handler, bool)
```

Get retrieves a handler by action type name. Returns the handler and true if found, nil and false otherwise.

<a name="Registry.Has"></a>
### func \(\*Registry\) [Has](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/registry.go#L88>)

```go
func (r *Registry) Has(actionType string) bool
```

Has checks if a handler is registered for the given action type.

<a name="Registry.List"></a>
### func \(\*Registry\) [List](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/registry.go#L76>)

```go
func (r *Registry) List() []ActionMetadata
```

List returns metadata for all registered handlers. Useful for introspection and documentation generation.

<a name="Registry.Register"></a>
### func \(\*Registry\) [Register](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/registry.go#L51>)

```go
func (r *Registry) Register(handler Handler)
```

Register adds a handler to the registry. This is typically called from init\(\) functions in action packages. Panics if a handler with the same name is already registered.

<a name="Result"></a>
## type [Result](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/interfaces.go#L149-L229>)

Result represents the outcome of an action execution.

Results track:

- Whether changes were made \(for idempotency reporting\)
- Output data \(stdout/stderr from commands\)
- Success/failure status
- Custom data \(for result registration\)

Results can be registered to variables for use in subsequent steps via the register: field.

Example:

```
result := executor.NewResult()
result.SetChanged(true)  // File was created/modified
result.SetData(map[string]interface{}{
    "path": "/etc/myapp/config.yml",
    "size": 1024,
    "checksum": "sha256:abc123...",
})

// If step has register: myfile, data is available as:
// {{ myfile.changed }} = true
// {{ myfile.path }} = "/etc/myapp/config.yml"
```

This interface avoids circular imports between actions and executor packages.

```go
type Result interface {
    // SetChanged marks whether this action modified system state.
    //
    // Set to true if the action:
    //   - Created/modified/deleted files or directories
    //   - Started/stopped/restarted services
    //   - Installed/removed packages
    //   - Executed commands that changed state
    //
    // Set to false if the action:
    //   - Found state already as desired (idempotent)
    //   - Only read/queried information
    //   - Failed before making changes
    //
    // Changed count is reported in run summary and used for idempotency tracking.
    SetChanged(changed bool)

    // SetStdout captures standard output from the action.
    //
    // Used primarily by shell/command actions. Output is:
    //   - Available in registered results as {{ result.stdout }}
    //   - Shown in TUI output view
    //   - Logged to artifacts
    //   - Used in changed_when/failed_when expressions
    SetStdout(stdout string)

    // SetStderr captures standard error from the action.
    //
    // Used primarily by shell/command actions. Error output is:
    //   - Available in registered results as {{ result.stderr }}
    //   - Shown in TUI output view (usually in red)
    //   - Logged to artifacts
    //   - Used in changed_when/failed_when expressions
    SetStderr(stderr string)

    // SetFailed marks the result as failed.
    //
    // Usually you should return an error instead of calling this. Use this when:
    //   - The action completed but didn't achieve desired state
    //   - failed_when expression evaluated to true
    //   - Assertion failed (assert action)
    //
    // Failed steps:
    //   - Increment failure count in run summary
    //   - Stop execution (unless ignore_errors: true)
    //   - Are highlighted in TUI
    SetFailed(failed bool)

    // SetData attaches custom data to the result.
    //
    // Data becomes available when the result is registered via register: field.
    //
    // Example:
    //
    //	result.SetData(map[string]interface{}{
    //	    "checksum": "sha256:abc123",
    //	    "size_bytes": 1024,
    //	    "format": "json",
    //	})
    //
    // Then in subsequent steps:
    //	  when: myfile.checksum == "sha256:abc123"
    //	  shell: echo "File size: {{ myfile.size_bytes }}"
    //
    // Keys should be snake_case. Values should be JSON-serializable.
    SetData(data map[string]interface{})

    // RegisterTo registers this result to the variables map.
    //
    // Called automatically by the executor when a step has register: field.
    // Creates a map in variables with:
    //   - changed: bool
    //   - failed: bool
    //   - stdout: string (if set)
    //   - stderr: string (if set)
    //   - rc: int (if applicable)
    //   - ...custom data from SetData()
    //
    // Handlers typically don't call this directly.
    RegisterTo(variables map[string]interface{}, name string)
}
```

# assert

```go
import "github.com/alehatsman/mooncake/internal/actions/assert"
```

Package assert implements the assert action handler. Assertions verify conditions without changing system state.

## Index

- [type Handler](<#Handler>)
  - [func \(h \*Handler\) DryRun\(ctx actions.Context, step \*config.Step\) error](<#Handler.DryRun>)
  - [func \(h \*Handler\) Execute\(ctx actions.Context, step \*config.Step\) \(actions.Result, error\)](<#Handler.Execute>)
  - [func \(h \*Handler\) Metadata\(\) actions.ActionMetadata](<#Handler.Metadata>)
  - [func \(h \*Handler\) Validate\(step \*config.Step\) error](<#Handler.Validate>)


<a name="Handler"></a>
## type [Handler](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/assert/handler.go#L22>)

Handler implements the assert action handler.

```go
type Handler struct{}
```

<a name="Handler.DryRun"></a>
### func \(\*Handler\) [DryRun](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/assert/handler.go#L117>)

```go
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error
```

DryRun logs what the assertion would check.

<a name="Handler.Execute"></a>
### func \(\*Handler\) [Execute](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/assert/handler.go#L67>)

```go
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error)
```

Execute executes the assert action.

<a name="Handler.Metadata"></a>
### func \(\*Handler\) [Metadata](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/assert/handler.go#L29>)

```go
func (h *Handler) Metadata() actions.ActionMetadata
```

Metadata returns the action metadata.

<a name="Handler.Validate"></a>
### func \(\*Handler\) [Validate](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/assert/handler.go#L39>)

```go
func (h *Handler) Validate(step *config.Step) error
```

Validate validates the assert action configuration.

# command

```go
import "github.com/alehatsman/mooncake/internal/actions/command"
```

Package command implements the command action handler.

The command action executes commands directly without shell interpolation. This is safer than shell when you have a known command with arguments, as it prevents shell injection attacks.

## Index

- [type Handler](<#Handler>)
  - [func \(h \*Handler\) DryRun\(ctx actions.Context, step \*config.Step\) error](<#Handler.DryRun>)
  - [func \(h \*Handler\) Execute\(ctx actions.Context, step \*config.Step\) \(actions.Result, error\)](<#Handler.Execute>)
  - [func \(Handler\) Metadata\(\) actions.ActionMetadata](<#Handler.Metadata>)
  - [func \(h \*Handler\) Validate\(step \*config.Step\) error](<#Handler.Validate>)


<a name="Handler"></a>
## type [Handler](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/command/handler.go#L24>)

Handler implements the Handler interface for command actions.

```go
type Handler struct{}
```

<a name="Handler.DryRun"></a>
### func \(\*Handler\) [DryRun](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/command/handler.go#L315>)

```go
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error
```

DryRun logs what would be executed without actually running the command.

<a name="Handler.Execute"></a>
### func \(\*Handler\) [Execute](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/command/handler.go#L58>)

```go
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error)
```

Execute runs the command action with retry logic if configured.

<a name="Handler.Metadata"></a>
### func \(Handler\) [Metadata](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/command/handler.go#L32>)

```go
func (Handler) Metadata() actions.ActionMetadata
```

Metadata returns metadata about the command action.

<a name="Handler.Validate"></a>
### func \(\*Handler\) [Validate](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/command/handler.go#L45>)

```go
func (h *Handler) Validate(step *config.Step) error
```

Validate checks if the command configuration is valid.

# copy

```go
import "github.com/alehatsman/mooncake/internal/actions/copy"
```

Package copy implements the copy action handler.

The copy action copies files from source to destination with: \- Checksum verification \(before and after copy\) \- Atomic write pattern \(temp file \+ rename\) \- Backup support \- Idempotency based on size/modtime

## Index

- [type Handler](<#Handler>)
  - [func \(h \*Handler\) DryRun\(ctx actions.Context, step \*config.Step\) error](<#Handler.DryRun>)
  - [func \(h \*Handler\) Execute\(ctx actions.Context, step \*config.Step\) \(actions.Result, error\)](<#Handler.Execute>)
  - [func \(Handler\) Metadata\(\) actions.ActionMetadata](<#Handler.Metadata>)
  - [func \(h \*Handler\) Validate\(step \*config.Step\) error](<#Handler.Validate>)


<a name="Handler"></a>
## type [Handler](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/copy/handler.go#L30>)

Handler implements the Handler interface for copy actions.

```go
type Handler struct{}
```

<a name="Handler.DryRun"></a>
### func \(\*Handler\) [DryRun](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/copy/handler.go#L210>)

```go
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error
```

DryRun logs what would be done without actually doing it.

<a name="Handler.Execute"></a>
### func \(\*Handler\) [Execute](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/copy/handler.go#L69>)

```go
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error)
```

Execute runs the copy action.

<a name="Handler.Metadata"></a>
### func \(Handler\) [Metadata](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/copy/handler.go#L38>)

```go
func (Handler) Metadata() actions.ActionMetadata
```

Metadata returns metadata about the copy action.

<a name="Handler.Validate"></a>
### func \(\*Handler\) [Validate](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/copy/handler.go#L51>)

```go
func (h *Handler) Validate(step *config.Step) error
```

Validate checks if the copy configuration is valid.

# download

```go
import "github.com/alehatsman/mooncake/internal/actions/download"
```

Package download implements the download action handler.

The download action downloads files from URLs with: \- HTTP/HTTPS support \- Checksum verification \(MD5, SHA1, SHA256\) for idempotency \- Custom HTTP headers \- Timeout and retry support \- Atomic write pattern \(temp file \+ rename\)

## Index

- [type Handler](<#Handler>)
  - [func \(h \*Handler\) DryRun\(ctx actions.Context, step \*config.Step\) error](<#Handler.DryRun>)
  - [func \(h \*Handler\) Execute\(ctx actions.Context, step \*config.Step\) \(actions.Result, error\)](<#Handler.Execute>)
  - [func \(Handler\) Metadata\(\) actions.ActionMetadata](<#Handler.Metadata>)
  - [func \(h \*Handler\) Validate\(step \*config.Step\) error](<#Handler.Validate>)


<a name="Handler"></a>
## type [Handler](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/download/handler.go#L35>)

Handler implements the Handler interface for download actions.

```go
type Handler struct{}
```

<a name="Handler.DryRun"></a>
### func \(\*Handler\) [DryRun](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/download/handler.go#L220>)

```go
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error
```

DryRun logs what would be done without actually doing it.

<a name="Handler.Execute"></a>
### func \(\*Handler\) [Execute](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/download/handler.go#L74>)

```go
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error)
```

Execute runs the download action.

<a name="Handler.Metadata"></a>
### func \(Handler\) [Metadata](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/download/handler.go#L43>)

```go
func (Handler) Metadata() actions.ActionMetadata
```

Metadata returns metadata about the download action.

<a name="Handler.Validate"></a>
### func \(\*Handler\) [Validate](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/download/handler.go#L56>)

```go
func (h *Handler) Validate(step *config.Step) error
```

Validate checks if the download configuration is valid.

# file

```go
import "github.com/alehatsman/mooncake/internal/actions/file"
```

Package file implements the file action handler.

The file action manages files, directories, and links with support for: \- Creating/updating files with content \- Creating directories \- Removing files and directories \- Creating symbolic and hard links \- Setting permissions and ownership \- Touch operations \(update timestamps\)

## Index

- [type Handler](<#Handler>)
  - [func \(h \*Handler\) DryRun\(ctx actions.Context, step \*config.Step\) error](<#Handler.DryRun>)
  - [func \(h \*Handler\) Execute\(ctx actions.Context, step \*config.Step\) \(actions.Result, error\)](<#Handler.Execute>)
  - [func \(Handler\) Metadata\(\) actions.ActionMetadata](<#Handler.Metadata>)
  - [func \(h \*Handler\) Validate\(step \*config.Step\) error](<#Handler.Validate>)


<a name="Handler"></a>
## type [Handler](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/file/handler.go#L36>)

Handler implements the Handler interface for file actions.

```go
type Handler struct{}
```

<a name="Handler.DryRun"></a>
### func \(\*Handler\) [DryRun](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/file/handler.go#L152>)

```go
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error
```

DryRun logs what would be done without actually doing it.

<a name="Handler.Execute"></a>
### func \(\*Handler\) [Execute](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/file/handler.go#L93>)

```go
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error)
```

Execute runs the file action.

<a name="Handler.Metadata"></a>
### func \(Handler\) [Metadata](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/file/handler.go#L44>)

```go
func (Handler) Metadata() actions.ActionMetadata
```

Metadata returns metadata about the file action.

<a name="Handler.Validate"></a>
### func \(\*Handler\) [Validate](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/file/handler.go#L65>)

```go
func (h *Handler) Validate(step *config.Step) error
```

Validate checks if the file configuration is valid.

# include\_vars

```go
import "github.com/alehatsman/mooncake/internal/actions/include_vars"
```

Package include\_vars implements the include\_vars action handler.

The include\_vars action loads variables from YAML files into the execution context. This is useful for organizing variables across multiple files.

## Index

- [type Handler](<#Handler>)
  - [func \(h \*Handler\) DryRun\(ctx actions.Context, step \*config.Step\) error](<#Handler.DryRun>)
  - [func \(h \*Handler\) Execute\(ctx actions.Context, step \*config.Step\) \(actions.Result, error\)](<#Handler.Execute>)
  - [func \(Handler\) Metadata\(\) actions.ActionMetadata](<#Handler.Metadata>)
  - [func \(h \*Handler\) Validate\(step \*config.Step\) error](<#Handler.Validate>)


<a name="Handler"></a>
## type [Handler](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/include_vars/handler.go#L18>)

Handler implements the Handler interface for include\_vars actions.

```go
type Handler struct{}
```

<a name="Handler.DryRun"></a>
### func \(\*Handler\) [DryRun](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/include_vars/handler.go#L108>)

```go
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error
```

DryRun logs what variables would be loaded.

<a name="Handler.Execute"></a>
### func \(\*Handler\) [Execute](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/include_vars/handler.go#L52>)

```go
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error)
```

Execute runs the include\_vars action.

<a name="Handler.Metadata"></a>
### func \(Handler\) [Metadata](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/include_vars/handler.go#L26>)

```go
func (Handler) Metadata() actions.ActionMetadata
```

Metadata returns metadata about the include\_vars action.

<a name="Handler.Validate"></a>
### func \(\*Handler\) [Validate](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/include_vars/handler.go#L39>)

```go
func (h *Handler) Validate(step *config.Step) error
```

Validate checks if the include\_vars configuration is valid.

# package\_handler

```go
import "github.com/alehatsman/mooncake/internal/actions/package"
```

Package package implements the package action handler.

The package action manages system packages with support for: \- Auto\-detection of package manager \(apt, dnf, yum, pacman, zypper, apk, brew, port, choco, scoop\) \- Manual package manager selection \- Install, remove, and update operations \- Cache management and system upgrades

## Index

- [type Handler](<#Handler>)
  - [func \(h \*Handler\) DryRun\(ctx actions.Context, step \*config.Step\) error](<#Handler.DryRun>)
  - [func \(h \*Handler\) Execute\(ctx actions.Context, step \*config.Step\) \(actions.Result, error\)](<#Handler.Execute>)
  - [func \(h \*Handler\) Metadata\(\) actions.ActionMetadata](<#Handler.Metadata>)
  - [func \(h \*Handler\) Validate\(step \*config.Step\) error](<#Handler.Validate>)


<a name="Handler"></a>
## type [Handler](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/package/handler.go#L23>)

Handler implements the Handler interface for package actions.

```go
type Handler struct{}
```

<a name="Handler.DryRun"></a>
### func \(\*Handler\) [DryRun](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/package/handler.go#L119>)

```go
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error
```

DryRun shows what would be done without making changes.

<a name="Handler.Execute"></a>
### func \(\*Handler\) [Execute](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/package/handler.go#L65>)

```go
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error)
```

Execute runs the package action.

<a name="Handler.Metadata"></a>
### func \(\*Handler\) [Metadata](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/package/handler.go#L31>)

```go
func (h *Handler) Metadata() actions.ActionMetadata
```

Metadata returns metadata about the package action.

<a name="Handler.Validate"></a>
### func \(\*Handler\) [Validate](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/package/handler.go#L44>)

```go
func (h *Handler) Validate(step *config.Step) error
```

Validate checks if the package configuration is valid.

# preset

```go
import "github.com/alehatsman/mooncake/internal/actions/preset"
```

Package preset implements the preset action handler. Presets expand into multiple steps with parameter injection.

## Index

- [type Handler](<#Handler>)
  - [func \(h \*Handler\) DryRun\(ctx actions.Context, step \*config.Step\) error](<#Handler.DryRun>)
  - [func \(h \*Handler\) Execute\(ctx actions.Context, step \*config.Step\) \(actions.Result, error\)](<#Handler.Execute>)
  - [func \(h \*Handler\) Metadata\(\) actions.ActionMetadata](<#Handler.Metadata>)
  - [func \(h \*Handler\) Validate\(step \*config.Step\) error](<#Handler.Validate>)


<a name="Handler"></a>
## type [Handler](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/preset/handler.go#L17>)

Handler implements the preset action handler.

```go
type Handler struct{}
```

<a name="Handler.DryRun"></a>
### func \(\*Handler\) [DryRun](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/preset/handler.go#L160>)

```go
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error
```

DryRun logs what the preset would expand.

<a name="Handler.Execute"></a>
### func \(\*Handler\) [Execute](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/preset/handler.go#L79>)

```go
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error)
```

Execute executes the preset action.

<a name="Handler.Metadata"></a>
### func \(\*Handler\) [Metadata](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/preset/handler.go#L58>)

```go
func (h *Handler) Metadata() actions.ActionMetadata
```

Metadata returns the action metadata.

<a name="Handler.Validate"></a>
### func \(\*Handler\) [Validate](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/preset/handler.go#L68>)

```go
func (h *Handler) Validate(step *config.Step) error
```

Validate validates the preset action configuration.

# print

```go
import "github.com/alehatsman/mooncake/internal/actions/print"
```

Package print implements the print action handler.

The print action displays messages to the user during execution. It supports template rendering and is useful for debugging and showing information.

## Index

- [type Handler](<#Handler>)
  - [func \(h \*Handler\) DryRun\(ctx actions.Context, step \*config.Step\) error](<#Handler.DryRun>)
  - [func \(h \*Handler\) Execute\(ctx actions.Context, step \*config.Step\) \(actions.Result, error\)](<#Handler.Execute>)
  - [func \(h \*Handler\) Metadata\(\) actions.ActionMetadata](<#Handler.Metadata>)
  - [func \(h \*Handler\) Validate\(step \*config.Step\) error](<#Handler.Validate>)


<a name="Handler"></a>
## type [Handler](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/print/handler.go#L18>)

Handler implements the Handler interface for print actions.

```go
type Handler struct{}
```

<a name="Handler.DryRun"></a>
### func \(\*Handler\) [DryRun](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/print/handler.go#L86>)

```go
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error
```

DryRun logs what would be printed without actually printing.

<a name="Handler.Execute"></a>
### func \(\*Handler\) [Execute](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/print/handler.go#L52>)

```go
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error)
```

Execute runs the print action.

<a name="Handler.Metadata"></a>
### func \(\*Handler\) [Metadata](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/print/handler.go#L26>)

```go
func (h *Handler) Metadata() actions.ActionMetadata
```

Metadata returns metadata about the print action.

<a name="Handler.Validate"></a>
### func \(\*Handler\) [Validate](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/print/handler.go#L39>)

```go
func (h *Handler) Validate(step *config.Step) error
```

Validate checks if the print configuration is valid.

# service

```go
import "github.com/alehatsman/mooncake/internal/actions/service"
```

Package service implements the service action handler. Manages services across different platforms \(systemd, launchd, Windows\).

## Index

- [Constants](<#constants>)
- [func HandleService\(step config.Step, ec \*executor.ExecutionContext\) error](<#HandleService>)
- [type Handler](<#Handler>)
  - [func \(h \*Handler\) DryRun\(ctx actions.Context, step \*config.Step\) error](<#Handler.DryRun>)
  - [func \(h \*Handler\) Execute\(ctx actions.Context, step \*config.Step\) \(actions.Result, error\)](<#Handler.Execute>)
  - [func \(h \*Handler\) Metadata\(\) actions.ActionMetadata](<#Handler.Metadata>)
  - [func \(h \*Handler\) Validate\(step \*config.Step\) error](<#Handler.Validate>)


## Constants

<a name="ServiceStateStarted"></a>Valid service states

```go
const (
    ServiceStateStarted   = "started"
    ServiceStateStopped   = "stopped"
    ServiceStateReloaded  = "reloaded"
    ServiceStateRestarted = "restarted"
)
```

<a name="HandleService"></a>
## func [HandleService](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/service/handler.go#L129>)

```go
func HandleService(step config.Step, ec *executor.ExecutionContext) error
```

HandleService manages services across different platforms \(systemd, launchd, Windows\).

<a name="Handler"></a>
## type [Handler](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/service/handler.go#L32>)

Handler implements the service action handler.

```go
type Handler struct{}
```

<a name="Handler.DryRun"></a>
### func \(\*Handler\) [DryRun](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/service/handler.go#L90>)

```go
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error
```

DryRun logs what the service operation would do.

<a name="Handler.Execute"></a>
### func \(\*Handler\) [Execute](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/service/handler.go#L80>)

```go
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error)
```

Execute executes the service action.

<a name="Handler.Metadata"></a>
### func \(\*Handler\) [Metadata](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/service/handler.go#L39>)

```go
func (h *Handler) Metadata() actions.ActionMetadata
```

Metadata returns the action metadata.

<a name="Handler.Validate"></a>
### func \(\*Handler\) [Validate](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/service/handler.go#L49>)

```go
func (h *Handler) Validate(step *config.Step) error
```

Validate validates the service action configuration.

# shell

```go
import "github.com/alehatsman/mooncake/internal/actions/shell"
```

Package shell implements the shell action handler.

The shell action executes shell commands with support for: \- Multiple interpreters \(bash, sh, pwsh, cmd\) \- Sudo/become privilege escalation \- Environment variables and working directory \- Timeout and retry logic \- Stdin, stdout, stderr handling \- Result overrides \(changed\_when, failed\_when\)

## Index

- [type Handler](<#Handler>)
  - [func \(h \*Handler\) DryRun\(ctx actions.Context, step \*config.Step\) error](<#Handler.DryRun>)
  - [func \(h \*Handler\) Execute\(ctx actions.Context, step \*config.Step\) \(actions.Result, error\)](<#Handler.Execute>)
  - [func \(h \*Handler\) Metadata\(\) actions.ActionMetadata](<#Handler.Metadata>)
  - [func \(h \*Handler\) Validate\(step \*config.Step\) error](<#Handler.Validate>)


<a name="Handler"></a>
## type [Handler](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/shell/handler.go#L33>)

Handler implements the Handler interface for shell actions.

```go
type Handler struct{}
```

<a name="Handler.DryRun"></a>
### func \(\*Handler\) [DryRun](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/shell/handler.go#L102>)

```go
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error
```

DryRun logs what would be executed.

<a name="Handler.Execute"></a>
### func \(\*Handler\) [Execute](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/shell/handler.go#L85>)

```go
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error)
```

Execute runs the shell action.

<a name="Handler.Metadata"></a>
### func \(\*Handler\) [Metadata](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/shell/handler.go#L41>)

```go
func (h *Handler) Metadata() actions.ActionMetadata
```

Metadata returns metadata about the shell action.

<a name="Handler.Validate"></a>
### func \(\*Handler\) [Validate](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/shell/handler.go#L57>)

```go
func (h *Handler) Validate(step *config.Step) error
```

Validate checks if the shell configuration is valid.

# template

```go
import "github.com/alehatsman/mooncake/internal/actions/template"
```

Package template implements the template action handler.

The template action reads a template file, renders it with variables, and writes the rendered output to a destination file.

## Index

- [type Handler](<#Handler>)
  - [func \(h \*Handler\) DryRun\(ctx actions.Context, step \*config.Step\) error](<#Handler.DryRun>)
  - [func \(h \*Handler\) Execute\(ctx actions.Context, step \*config.Step\) \(actions.Result, error\)](<#Handler.Execute>)
  - [func \(Handler\) Metadata\(\) actions.ActionMetadata](<#Handler.Metadata>)
  - [func \(h \*Handler\) Validate\(step \*config.Step\) error](<#Handler.Validate>)


<a name="Handler"></a>
## type [Handler](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/template/handler.go#L28>)

Handler implements the Handler interface for template actions.

```go
type Handler struct{}
```

<a name="Handler.DryRun"></a>
### func \(\*Handler\) [DryRun](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/template/handler.go#L168>)

```go
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error
```

DryRun logs what would be done without actually doing it.

<a name="Handler.Execute"></a>
### func \(\*Handler\) [Execute](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/template/handler.go#L67>)

```go
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error)
```

Execute runs the template action.

<a name="Handler.Metadata"></a>
### func \(Handler\) [Metadata](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/template/handler.go#L36>)

```go
func (Handler) Metadata() actions.ActionMetadata
```

Metadata returns metadata about the template action.

<a name="Handler.Validate"></a>
### func \(\*Handler\) [Validate](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/template/handler.go#L49>)

```go
func (h *Handler) Validate(step *config.Step) error
```

Validate checks if the template configuration is valid.

# unarchive

```go
import "github.com/alehatsman/mooncake/internal/actions/unarchive"
```

Package unarchive implements the unarchive action handler.

The unarchive action extracts archive files with: \- Format support: tar, tar.gz, tar.bz2, zip \- Strip leading path components \- Idempotency via creates marker \- Path traversal protection \- Extraction statistics

## Index

- [type ArchiveFormat](<#ArchiveFormat>)
  - [func \(f ArchiveFormat\) String\(\) string](<#ArchiveFormat.String>)
- [type ExtractionStats](<#ExtractionStats>)
- [type Handler](<#Handler>)
  - [func \(h \*Handler\) DryRun\(ctx actions.Context, step \*config.Step\) error](<#Handler.DryRun>)
  - [func \(h \*Handler\) Execute\(ctx actions.Context, step \*config.Step\) \(actions.Result, error\)](<#Handler.Execute>)
  - [func \(Handler\) Metadata\(\) actions.ActionMetadata](<#Handler.Metadata>)
  - [func \(h \*Handler\) Validate\(step \*config.Step\) error](<#Handler.Validate>)


<a name="ArchiveFormat"></a>
## type [ArchiveFormat](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/unarchive/handler.go#L36>)

ArchiveFormat represents the type of archive being extracted.

```go
type ArchiveFormat int
```

<a name="ArchiveUnknown"></a>

```go
const (
    ArchiveUnknown ArchiveFormat = iota
    ArchiveTar
    ArchiveTarGz
    ArchiveZip
)
```

<a name="ArchiveFormat.String"></a>
### func \(ArchiveFormat\) [String](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/unarchive/handler.go#L46>)

```go
func (f ArchiveFormat) String() string
```

String returns the string representation of the archive format.

<a name="ExtractionStats"></a>
## type [ExtractionStats](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/unarchive/handler.go#L60-L64>)

ExtractionStats tracks statistics from archive extraction.

```go
type ExtractionStats struct {
    FilesExtracted int
    DirsCreated    int
    BytesExtracted int64
}
```

<a name="Handler"></a>
## type [Handler](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/unarchive/handler.go#L67>)

Handler implements the Handler interface for unarchive actions.

```go
type Handler struct{}
```

<a name="Handler.DryRun"></a>
### func \(\*Handler\) [DryRun](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/unarchive/handler.go#L226>)

```go
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error
```

DryRun logs what would be done without actually doing it.

<a name="Handler.Execute"></a>
### func \(\*Handler\) [Execute](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/unarchive/handler.go#L106>)

```go
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error)
```

Execute runs the unarchive action.

<a name="Handler.Metadata"></a>
### func \(Handler\) [Metadata](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/unarchive/handler.go#L75>)

```go
func (Handler) Metadata() actions.ActionMetadata
```

Metadata returns metadata about the unarchive action.

<a name="Handler.Validate"></a>
### func \(\*Handler\) [Validate](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/unarchive/handler.go#L88>)

```go
func (h *Handler) Validate(step *config.Step) error
```

Validate checks if the unarchive configuration is valid.

# vars

```go
import "github.com/alehatsman/mooncake/internal/actions/vars"
```

Package vars implements the vars action handler.

The vars action sets variables that are available to subsequent steps. Variables can be used in templates and when conditions.

## Index

- [type Handler](<#Handler>)
  - [func \(h \*Handler\) DryRun\(ctx actions.Context, step \*config.Step\) error](<#Handler.DryRun>)
  - [func \(h \*Handler\) Execute\(ctx actions.Context, step \*config.Step\) \(actions.Result, error\)](<#Handler.Execute>)
  - [func \(h \*Handler\) Metadata\(\) actions.ActionMetadata](<#Handler.Metadata>)
  - [func \(h \*Handler\) Validate\(step \*config.Step\) error](<#Handler.Validate>)


<a name="Handler"></a>
## type [Handler](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/vars/handler.go#L18>)

Handler implements the Handler interface for vars actions.

```go
type Handler struct{}
```

<a name="Handler.DryRun"></a>
### func \(\*Handler\) [DryRun](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/vars/handler.go#L99>)

```go
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error
```

DryRun logs what variables would be set.

<a name="Handler.Execute"></a>
### func \(\*Handler\) [Execute](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/vars/handler.go#L48>)

```go
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error)
```

Execute runs the vars action.

<a name="Handler.Metadata"></a>
### func \(\*Handler\) [Metadata](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/vars/handler.go#L26>)

```go
func (h *Handler) Metadata() actions.ActionMetadata
```

Metadata returns metadata about the vars action.

<a name="Handler.Validate"></a>
### func \(\*Handler\) [Validate](<https://github.com/alehatsman/mooncake/blob/master/internal/actions/vars/handler.go#L39>)

```go
func (h *Handler) Validate(step *config.Step) error
```

Validate checks if the vars configuration is valid.

Generated by [gomarkdoc](<https://github.com/princjef/gomarkdoc>)


---

<!-- FILE: api/cmd.md -->

<!-- Code generated by gomarkdoc. DO NOT EDIT -->

# cmd

```go
import "github.com/alehatsman/mooncake/cmd"
```

Package main provides the mooncake CLI application.

## Index



Generated by [gomarkdoc](<https://github.com/princjef/gomarkdoc>)


---

<!-- FILE: api/config.md -->

<!-- Code generated by gomarkdoc. DO NOT EDIT -->

# config

```go
import "github.com/alehatsman/mooncake/internal/config"
```

Package config provides data structures and validation for mooncake configuration files.

This package defines the complete YAML schema for mooncake plans, including:

- Step structure and universal fields
- Action\-specific configuration structs
- Validation logic and error reporting
- YAML unmarshaling with custom behavior
- JSON schema validation

### Configuration Structure

A mooncake configuration file is a YAML document containing an array of steps:

- name: Install nginx package: name: nginx state: present become: true when: os == "linux"

- name: Start nginx service: name: nginx state: started become: true

Each step consists of:

- Universal fields: name, when, register, tags, become, env, cwd, timeout, etc.
- Exactly one action: shell, file, template, package, service, assert, etc.
- Optional control flow: with\_items, with\_filetree

### Step Structure

The Step struct represents a single configuration step. Key fields:

```
type Step struct {
    // Universal fields (apply to all actions)
    Name     string   // Human-readable step name
    When     string   // Conditional expression (e.g., "os == 'linux'")
    Register string   // Variable name to store result
    Tags     []string // Tag filter for selective execution
    Become   bool     // Run with sudo/privilege escalation

    // Action fields (exactly one must be set)
    Shell    *ShellAction
    File     *File
    Template *Template
    Package  *Package
    Service  *ServiceAction
    Assert   *Assert
    // ... other actions
}
```

### Action Types

Each action type has its own struct defining required and optional fields:

- ShellAction: Execute shell commands \(cmd, interpreter, stdin, capture\)
- File: Manage files/directories \(path, state, content, mode, owner, group\)
- Template: Render Jinja2 templates \(src, dest, vars, mode\)
- Package: Install/remove packages \(name/names, state, manager, update\_cache\)
- ServiceAction: Manage services \(name, state, enabled, unit, daemon\_reload\)
- Assert: Verify state \(command, file, http assertions\)
- Copy: Copy files \(src, dest, mode, owner, group, backup, checksum\)
- Download: Download files \(url, dest, checksum, timeout, retries\)
- Unarchive: Extract archives \(src, dest, format, strip\_components\)
- PrintAction: Output messages \(msg\)
- PresetInvocation: Invoke presets \(name, with parameters\)

### Validation

Configuration is validated at multiple levels:

1. YAML syntax: Parser errors caught during ReadConfig\(\)
2. Schema validation: JSON schema enforces structure \(SchemaValidator\)
3. Template syntax: Jinja2 templates validated \(TemplateValidator\)
4. Step validation: Each step must have exactly one action
5. Action validation: Handler\-specific validation before execution

Validation produces Diagnostic objects with:

- Severity: error, warning, info
- Message: Human\-readable error description
- Path: YAML path to the error \(e.g., "steps\[0\].when"\)
- Position: Line and column in source file
- Context: Surrounding YAML for better error messages

### Custom Unmarshaling

Some actions support multiple YAML forms for convenience:

```
# Simple string form
shell: "apt install nginx"

# Structured object form
shell:
  cmd: "apt install nginx"
  interpreter: bash
  capture: true
```

This is implemented via UnmarshalYAML\(\) methods that try string first, then fall back to struct unmarshaling.

### Usage Example

```
// Read and validate configuration
steps, diagnostics, err := config.ReadConfigWithValidation("config.yml")
if err != nil {
    return fmt.Errorf("failed to read config: %w", err)
}

// Check for validation errors
if config.HasErrors(diagnostics) {
    fmt.Println(config.FormatDiagnosticsWithContext(diagnostics))
    return fmt.Errorf("configuration validation failed")
}

// Validate each step has one action
for i, step := range steps {
    if err := step.ValidateOneAction(); err != nil {
        return fmt.Errorf("step %d: %w", i, err)
    }
}
```

### Thread Safety

Config structures are designed to be read\-only after parsing. The executor clones steps when expanding loops to avoid modifying shared structures. Use step.Clone\(\) to create independent copies.

## Index

- [func FormatDiagnostics\(diagnostics \[\]Diagnostic\) string](<#FormatDiagnostics>)
- [func FormatDiagnosticsWithContext\(diagnostics \[\]Diagnostic\) string](<#FormatDiagnosticsWithContext>)
- [func HasErrors\(diagnostics \[\]Diagnostic\) bool](<#HasErrors>)
- [func ReadConfigWithValidation\(path string\) \(\[\]Step, \[\]Diagnostic, error\)](<#ReadConfigWithValidation>)
- [func ReadVariables\(path string\) \(map\[string\]interface\{\}, error\)](<#ReadVariables>)
- [type Assert](<#Assert>)
- [type AssertCommand](<#AssertCommand>)
- [type AssertFile](<#AssertFile>)
- [type AssertHTTP](<#AssertHTTP>)
- [type CommandAction](<#CommandAction>)
- [type Copy](<#Copy>)
- [type Diagnostic](<#Diagnostic>)
  - [func \(d \*Diagnostic\) String\(\) string](<#Diagnostic.String>)
- [type Download](<#Download>)
- [type File](<#File>)
- [type LocationMap](<#LocationMap>)
  - [func NewLocationMap\(\) \*LocationMap](<#NewLocationMap>)
  - [func \(lm \*LocationMap\) Get\(path string\) Position](<#LocationMap.Get>)
  - [func \(lm \*LocationMap\) GetOrDefault\(path string, defaultPos Position\) Position](<#LocationMap.GetOrDefault>)
  - [func \(lm \*LocationMap\) Set\(path string, line, column int\)](<#LocationMap.Set>)
- [type LoopContext](<#LoopContext>)
- [type Origin](<#Origin>)
- [type Package](<#Package>)
- [type Position](<#Position>)
- [type PresetDefinition](<#PresetDefinition>)
- [type PresetInvocation](<#PresetInvocation>)
  - [func \(p \*PresetInvocation\) UnmarshalYAML\(unmarshal func\(interface\{\}\) error\) error](<#PresetInvocation.UnmarshalYAML>)
- [type PresetParameter](<#PresetParameter>)
- [type PrintAction](<#PrintAction>)
  - [func \(p \*PrintAction\) UnmarshalYAML\(unmarshal func\(interface\{\}\) error\) error](<#PrintAction.UnmarshalYAML>)
- [type Reader](<#Reader>)
  - [func NewYAMLConfigReader\(\) Reader](<#NewYAMLConfigReader>)
- [type RunConfig](<#RunConfig>)
- [type SchemaValidator](<#SchemaValidator>)
  - [func NewSchemaValidator\(\) \(\*SchemaValidator, error\)](<#NewSchemaValidator>)
  - [func \(v \*SchemaValidator\) Validate\(steps \[\]Step, locationMap \*LocationMap, filePath string\) \[\]Diagnostic](<#SchemaValidator.Validate>)
- [type ServiceAction](<#ServiceAction>)
- [type ServiceDropin](<#ServiceDropin>)
- [type ServiceUnit](<#ServiceUnit>)
- [type Shell](<#Shell>)
- [type ShellAction](<#ShellAction>)
  - [func \(s \*ShellAction\) UnmarshalYAML\(unmarshal func\(interface\{\}\) error\) error](<#ShellAction.UnmarshalYAML>)
- [type Step](<#Step>)
  - [func ReadConfig\(path string\) \(\[\]Step, error\)](<#ReadConfig>)
  - [func \(s \*Step\) Clone\(\) \*Step](<#Step.Clone>)
  - [func \(s \*Step\) DetermineActionType\(\) string](<#Step.DetermineActionType>)
  - [func \(s \*Step\) Validate\(\) error](<#Step.Validate>)
  - [func \(s \*Step\) ValidateHasAction\(\) error](<#Step.ValidateHasAction>)
  - [func \(s \*Step\) ValidateOneAction\(\) error](<#Step.ValidateOneAction>)
- [type Template](<#Template>)
- [type TemplateValidator](<#TemplateValidator>)
  - [func NewTemplateValidator\(\) \*TemplateValidator](<#NewTemplateValidator>)
  - [func \(v \*TemplateValidator\) ValidateSteps\(steps \[\]Step, locationMap \*LocationMap, filePath string\) \[\]Diagnostic](<#TemplateValidator.ValidateSteps>)
  - [func \(v \*TemplateValidator\) ValidateSyntax\(template string\) error](<#TemplateValidator.ValidateSyntax>)
- [type Unarchive](<#Unarchive>)
- [type ValidationError](<#ValidationError>)
  - [func \(e \*ValidationError\) Error\(\) string](<#ValidationError.Error>)
- [type YAMLConfigReader](<#YAMLConfigReader>)
  - [func \(r \*YAMLConfigReader\) ReadConfig\(path string\) \(\[\]Step, error\)](<#YAMLConfigReader.ReadConfig>)
  - [func \(r \*YAMLConfigReader\) ReadConfigWithValidation\(path string\) \(\[\]Step, \[\]Diagnostic, error\)](<#YAMLConfigReader.ReadConfigWithValidation>)
  - [func \(r \*YAMLConfigReader\) ReadVariables\(path string\) \(map\[string\]interface\{\}, error\)](<#YAMLConfigReader.ReadVariables>)


<a name="FormatDiagnostics"></a>
## func [FormatDiagnostics](<https://github.com/alehatsman/mooncake/blob/master/internal/config/diagnostic.go#L27>)

```go
func FormatDiagnostics(diagnostics []Diagnostic) string
```

FormatDiagnostics formats multiple diagnostics as a newline\-separated string

<a name="FormatDiagnosticsWithContext"></a>
## func [FormatDiagnosticsWithContext](<https://github.com/alehatsman/mooncake/blob/master/internal/config/format.go#L11>)

```go
func FormatDiagnosticsWithContext(diagnostics []Diagnostic) string
```

FormatDiagnosticsWithContext formats diagnostics with YAML context and step names

<a name="HasErrors"></a>
## func [HasErrors](<https://github.com/alehatsman/mooncake/blob/master/internal/config/diagnostic.go#L61>)

```go
func HasErrors(diagnostics []Diagnostic) bool
```

HasErrors returns true if any diagnostic has severity "error" or unspecified severity

<a name="ReadConfigWithValidation"></a>
## func [ReadConfigWithValidation](<https://github.com/alehatsman/mooncake/blob/master/internal/config/reader.go#L165>)

```go
func ReadConfigWithValidation(path string) ([]Step, []Diagnostic, error)
```

ReadConfigWithValidation is a convenience function using the default YAML reader Returns steps, diagnostics, and any parsing errors

<a name="ReadVariables"></a>
## func [ReadVariables](<https://github.com/alehatsman/mooncake/blob/master/internal/config/reader.go#L174>)

```go
func ReadVariables(path string) (map[string]interface{}, error)
```

ReadVariables is a convenience function using the default YAML reader

<a name="Assert"></a>
## type [Assert](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L322-L326>)

Assert represents an assertion/verification operation in a configuration step. Assertions always have changed: false and fail if the assertion doesn't pass. Supports three types: command \(exit code\), file \(content/existence\), and http \(response\).

```go
type Assert struct {
    Command *AssertCommand `yaml:"command" json:"command,omitempty"` // Command assertion
    File    *AssertFile    `yaml:"file" json:"file,omitempty"`       // File assertion
    HTTP    *AssertHTTP    `yaml:"http" json:"http,omitempty"`       // HTTP assertion
}
```

<a name="AssertCommand"></a>
## type [AssertCommand](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L329-L332>)

AssertCommand verifies a command exits with the expected code.

```go
type AssertCommand struct {
    Cmd      string `yaml:"cmd" json:"cmd"`                       // Command to execute (required)
    ExitCode int    `yaml:"exit_code" json:"exit_code,omitempty"` // Expected exit code (default: 0)
}
```

<a name="AssertFile"></a>
## type [AssertFile](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L335-L343>)

AssertFile verifies file existence, content, or properties.

```go
type AssertFile struct {
    Path     string  `yaml:"path" json:"path"`                   // File path (required)
    Exists   *bool   `yaml:"exists" json:"exists,omitempty"`     // Verify existence (true) or non-existence (false)
    Content  *string `yaml:"content" json:"content,omitempty"`   // Expected exact content
    Contains *string `yaml:"contains" json:"contains,omitempty"` // Expected substring
    Mode     *string `yaml:"mode" json:"mode,omitempty"`         // Expected file permissions (e.g., "0644")
    Owner    *string `yaml:"owner" json:"owner,omitempty"`       // Expected owner (username or UID)
    Group    *string `yaml:"group" json:"group,omitempty"`       // Expected group (groupname or GID)
}
```

<a name="AssertHTTP"></a>
## type [AssertHTTP](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L346-L357>)

AssertHTTP verifies HTTP response status, headers, or body content.

```go
type AssertHTTP struct {
    URL        string            `yaml:"url" json:"url"`                                 // URL to request (required)
    Method     string            `yaml:"method" json:"method,omitempty"`                 // HTTP method (default: GET)
    Status     int               `yaml:"status" json:"status,omitempty"`                 // Expected status code (default: 200)
    Headers    map[string]string `yaml:"headers" json:"headers,omitempty"`               // Request headers
    Body       *string           `yaml:"body" json:"body,omitempty"`                     // Request body
    Contains   *string           `yaml:"contains" json:"contains,omitempty"`             // Expected response body substring
    BodyEquals *string           `yaml:"body_equals" json:"body_equals,omitempty"`       // Expected exact response body
    JSONPath   *string           `yaml:"jsonpath" json:"jsonpath,omitempty"`             // JSONPath expression for response body
    JSONValue  interface{}       `yaml:"jsonpath_value" json:"jsonpath_value,omitempty"` // Expected value at JSONPath
    Timeout    string            `yaml:"timeout" json:"timeout,omitempty"`               // Request timeout (e.g., "30s")
}
```

<a name="CommandAction"></a>
## type [CommandAction](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L230-L244>)

CommandAction represents a direct command execution without shell interpolation. This is safer than shell when you have a known command with arguments.

```go
type CommandAction struct {
    // Argv is the command and arguments as a list (required)
    // Example: ["git", "clone", "https://..."]
    Argv []string `yaml:"argv" json:"argv"`

    // Stdin provides input to the command
    Stdin string `yaml:"stdin,omitempty" json:"stdin,omitempty"`

    // Capture controls whether to capture command output
    // When false, output is only streamed (not stored in result)
    // Default: true
    Capture *bool `yaml:"capture,omitempty" json:"capture,omitempty"`
}
```

<a name="Copy"></a>
## type [Copy](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L247-L256>)

Copy represents a file copy operation in a configuration step.

```go
type Copy struct {
    Src      string `yaml:"src" json:"src"`                     // Source file path
    Dest     string `yaml:"dest" json:"dest"`                   // Destination file path
    Mode     string `yaml:"mode" json:"mode,omitempty"`         // Octal file permissions (e.g., "0644", "0755")
    Owner    string `yaml:"owner" json:"owner,omitempty"`       // Username or UID
    Group    string `yaml:"group" json:"group,omitempty"`       // Groupname or GID
    Backup   bool   `yaml:"backup" json:"backup,omitempty"`     // Create .bak before overwrite
    Force    bool   `yaml:"force" json:"force,omitempty"`       // Overwrite if exists
    Checksum string `yaml:"checksum" json:"checksum,omitempty"` // Expected SHA256 or MD5 checksum
}
```

<a name="Diagnostic"></a>
## type [Diagnostic](<https://github.com/alehatsman/mooncake/blob/master/internal/config/diagnostic.go#L9-L15>)

Diagnostic represents a validation error or warning with source location

```go
type Diagnostic struct {
    FilePath string
    Line     int
    Column   int
    Message  string
    Severity string // "error" or "warning"
}
```

<a name="Diagnostic.String"></a>
### func \(\*Diagnostic\) [String](<https://github.com/alehatsman/mooncake/blob/master/internal/config/diagnostic.go#L18>)

```go
func (d *Diagnostic) String() string
```

String formats the diagnostic as "path/to/file.yml:line:col: message"

<a name="Download"></a>
## type [Download](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L268-L278>)

Download represents a file download operation in a configuration step.

```go
type Download struct {
    URL      string            `yaml:"url" json:"url"`                     // Remote URL (required)
    Dest     string            `yaml:"dest" json:"dest"`                   // Destination path (required)
    Checksum string            `yaml:"checksum" json:"checksum,omitempty"` // Expected SHA256 or MD5 checksum
    Mode     string            `yaml:"mode" json:"mode,omitempty"`         // Octal file permissions (e.g., "0644")
    Timeout  string            `yaml:"timeout" json:"timeout,omitempty"`   // Maximum download time (e.g., "30s", "5m")
    Force    bool              `yaml:"force" json:"force,omitempty"`       // Force re-download if destination exists
    Backup   bool              `yaml:"backup" json:"backup,omitempty"`     // Create .bak backup before overwriting
    Headers  map[string]string `yaml:"headers" json:"headers,omitempty"`   // Custom HTTP headers
    Retries  int               `yaml:"retries" json:"retries,omitempty"`   // Number of retry attempts
}
```

<a name="File"></a>
## type [File](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L151-L168>)

File represents a file or directory operation in a configuration step.

```go
type File struct {
    Path    string `yaml:"path" json:"path"`
    State   string `yaml:"state" json:"state,omitempty"` // file|directory|absent|link|hardlink|touch|perms
    Content string `yaml:"content" json:"content,omitempty"`
    Mode    string `yaml:"mode" json:"mode,omitempty"` // Octal file permissions (e.g., "0644", "0755")

    // Ownership
    Owner string `yaml:"owner" json:"owner,omitempty"` // Username or UID
    Group string `yaml:"group" json:"group,omitempty"` // Groupname or GID

    // Link operations
    Src string `yaml:"src" json:"src,omitempty"` // Source path for link/copy operations

    // Behavior flags
    Force   bool `yaml:"force" json:"force,omitempty"`     // Overwrite existing files
    Recurse bool `yaml:"recurse" json:"recurse,omitempty"` // Apply permissions recursively
    Backup  bool `yaml:"backup" json:"backup,omitempty"`   // Create .bak before overwrite
}
```

<a name="LocationMap"></a>
## type [LocationMap](<https://github.com/alehatsman/mooncake/blob/master/internal/config/location.go#L17-L19>)

LocationMap tracks YAML source positions for validation error reporting

```go
type LocationMap struct {
    // contains filtered or unexported fields
}
```

<a name="NewLocationMap"></a>
### func [NewLocationMap](<https://github.com/alehatsman/mooncake/blob/master/internal/config/location.go#L22>)

```go
func NewLocationMap() *LocationMap
```

NewLocationMap creates a new LocationMap

<a name="LocationMap.Get"></a>
### func \(\*LocationMap\) [Get](<https://github.com/alehatsman/mooncake/blob/master/internal/config/location.go#L35>)

```go
func (lm *LocationMap) Get(path string) Position
```

Get retrieves the position for a given JSON pointer path Returns zero Position if not found

<a name="LocationMap.GetOrDefault"></a>
### func \(\*LocationMap\) [GetOrDefault](<https://github.com/alehatsman/mooncake/blob/master/internal/config/location.go#L41>)

```go
func (lm *LocationMap) GetOrDefault(path string, defaultPos Position) Position
```

GetOrDefault retrieves the position for a given JSON pointer path Returns the default position if not found

<a name="LocationMap.Set"></a>
### func \(\*LocationMap\) [Set](<https://github.com/alehatsman/mooncake/blob/master/internal/config/location.go#L29>)

```go
func (lm *LocationMap) Set(path string, line, column int)
```

Set stores a position for a given JSON pointer path

<a name="LoopContext"></a>
## type [LoopContext](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L501-L509>)

LoopContext captures loop iteration metadata

```go
type LoopContext struct {
    Type           string      `yaml:"type" json:"type"` // "with_items" or "with_filetree"
    Item           interface{} `yaml:"item" json:"item"`
    Index          int         `yaml:"index" json:"index"`
    First          bool        `yaml:"first" json:"first"`
    Last           bool        `yaml:"last" json:"last"`
    LoopExpression string      `yaml:"loop_expression,omitempty" json:"loop_expression,omitempty"`
    Depth          int         `yaml:"depth,omitempty" json:"depth,omitempty"` // Directory depth for filetree items
}
```

<a name="Origin"></a>
## type [Origin](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L493-L498>)

Origin tracks source location and include chain for plan traceability

```go
type Origin struct {
    FilePath     string   `yaml:"file" json:"file"`
    Line         int      `yaml:"line" json:"line"`
    Column       int      `yaml:"column" json:"column"`
    IncludeChain []string `yaml:"include_chain,omitempty" json:"include_chain,omitempty"` // "file:line" entries
}
```

<a name="Package"></a>
## type [Package](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L282-L290>)

Package represents a package management operation \(install/remove/update packages\). Supports apt, dnf, yum, pacman, zypper, apk \(Linux\), brew, port \(macOS\), choco, scoop \(Windows\).

```go
type Package struct {
    Name        string   `yaml:"name" json:"name,omitempty"`                 // Package name (single package)
    Names       []string `yaml:"names" json:"names,omitempty"`               // Multiple packages
    State       string   `yaml:"state" json:"state,omitempty"`               // present|absent|latest (default: present)
    Manager     string   `yaml:"manager" json:"manager,omitempty"`           // Package manager to use (auto-detected if empty)
    UpdateCache bool     `yaml:"update_cache" json:"update_cache,omitempty"` // Update package cache before operation
    Upgrade     bool     `yaml:"upgrade" json:"upgrade,omitempty"`           // Upgrade all packages (ignores name/names)
    Extra       []string `yaml:"extra" json:"extra,omitempty"`               // Extra arguments to pass to package manager
}
```

<a name="Position"></a>
## type [Position](<https://github.com/alehatsman/mooncake/blob/master/internal/config/location.go#L11-L14>)

Position represents a line and column position in a source file

```go
type Position struct {
    Line   int
    Column int
}
```

<a name="PresetDefinition"></a>
## type [PresetDefinition](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L361-L368>)

PresetDefinition represents a reusable preset loaded from a YAML file. Presets are parameterized collections of steps that can be invoked as a single action.

```go
type PresetDefinition struct {
    Name        string                     `yaml:"name" json:"name"`                         // Preset name (required)
    Description string                     `yaml:"description" json:"description,omitempty"` // Human-readable description
    Version     string                     `yaml:"version" json:"version,omitempty"`         // Semantic version
    Parameters  map[string]PresetParameter `yaml:"parameters" json:"parameters,omitempty"`   // Parameter definitions
    Steps       []Step                     `yaml:"steps" json:"steps"`                       // Steps to execute
    BaseDir     string                     `yaml:"-" json:"-"`                               // Base directory for relative paths (set by loader)
}
```

<a name="PresetInvocation"></a>
## type [PresetInvocation](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L380-L383>)

PresetInvocation represents a user's invocation of a preset in their playbook.

```go
type PresetInvocation struct {
    Name string                 `yaml:"name" json:"name"`           // Preset name (required)
    With map[string]interface{} `yaml:"with" json:"with,omitempty"` // Parameter values
}
```

<a name="PresetInvocation.UnmarshalYAML"></a>
### func \(\*PresetInvocation\) [UnmarshalYAML](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L387>)

```go
func (p *PresetInvocation) UnmarshalYAML(unmarshal func(interface{}) error) error
```

UnmarshalYAML implements custom YAML unmarshaling to support string form. Supports: preset: "ollama" AND preset: \{ name: "ollama", with: \{...\} \}

<a name="PresetParameter"></a>
## type [PresetParameter](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L371-L377>)

PresetParameter defines a parameter that can be passed to a preset.

```go
type PresetParameter struct {
    Type        string        `yaml:"type" json:"type"`                         // string|bool|array|object
    Required    bool          `yaml:"required" json:"required,omitempty"`       // Whether parameter is required
    Default     interface{}   `yaml:"default" json:"default,omitempty"`         // Default value if not provided
    Enum        []interface{} `yaml:"enum" json:"enum,omitempty"`               // Valid values (if restricted)
    Description string        `yaml:"description" json:"description,omitempty"` // Human-readable description
}
```

<a name="PrintAction"></a>
## type [PrintAction](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L406-L408>)

PrintAction represents a print/output action for displaying messages.

```go
type PrintAction struct {
    Msg string `yaml:"msg,omitempty" json:"msg,omitempty"` // Message to print (supports templates)
}
```

<a name="PrintAction.UnmarshalYAML"></a>
### func \(\*PrintAction\) [UnmarshalYAML](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L412>)

```go
func (p *PrintAction) UnmarshalYAML(unmarshal func(interface{}) error) error
```

UnmarshalYAML implements custom YAML unmarshaling to support both string and object forms. Supports: print: "message" AND print: \{ msg: "message" \}

<a name="Reader"></a>
## type [Reader](<https://github.com/alehatsman/mooncake/blob/master/internal/config/reader.go#L12-L15>)

Reader defines the interface for reading configuration and variables

```go
type Reader interface {
    ReadConfig(path string) ([]Step, error)
    ReadVariables(path string) (map[string]interface{}, error)
}
```

<a name="NewYAMLConfigReader"></a>
### func [NewYAMLConfigReader](<https://github.com/alehatsman/mooncake/blob/master/internal/config/reader.go#L23>)

```go
func NewYAMLConfigReader() Reader
```

NewYAMLConfigReader creates a new YAMLConfigReader

<a name="RunConfig"></a>
## type [RunConfig](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L139-L148>)

RunConfig represents the root configuration structure. This can be either: \- A simple array of steps \(for backward compatibility\) \- A structured config with version, global settings, and steps

```go
type RunConfig struct {
    // Version specifies the config schema version (e.g., "1.0")
    Version string `yaml:"version" json:"version,omitempty"`

    // Vars defines global variables available to all steps
    Vars map[string]interface{} `yaml:"vars" json:"vars,omitempty"`

    // Steps contains the configuration steps to execute
    Steps []Step `yaml:"steps" json:"steps"`
}
```

<a name="SchemaValidator"></a>
## type [SchemaValidator](<https://github.com/alehatsman/mooncake/blob/master/internal/config/validator.go#L16-L18>)

SchemaValidator validates configuration against JSON Schema

```go
type SchemaValidator struct {
    // contains filtered or unexported fields
}
```

<a name="NewSchemaValidator"></a>
### func [NewSchemaValidator](<https://github.com/alehatsman/mooncake/blob/master/internal/config/validator.go#L21>)

```go
func NewSchemaValidator() (*SchemaValidator, error)
```

NewSchemaValidator creates a new SchemaValidator with the embedded schema

<a name="SchemaValidator.Validate"></a>
### func \(\*SchemaValidator\) [Validate](<https://github.com/alehatsman/mooncake/blob/master/internal/config/validator.go#L42>)

```go
func (v *SchemaValidator) Validate(steps []Step, locationMap *LocationMap, filePath string) []Diagnostic
```

Validate validates steps against the JSON Schema and returns diagnostics

<a name="ServiceAction"></a>
## type [ServiceAction](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L294-L301>)

ServiceAction represents a service management operation in a configuration step. Supports systemd \(Linux\), launchd \(macOS\), and Windows services.

```go
type ServiceAction struct {
    Name         string         `yaml:"name" json:"name"`                             // Service name (required)
    State        string         `yaml:"state" json:"state,omitempty"`                 // started|stopped|restarted|reloaded
    Enabled      *bool          `yaml:"enabled" json:"enabled,omitempty"`             // Enable service on boot
    DaemonReload bool           `yaml:"daemon_reload" json:"daemon_reload,omitempty"` // Run daemon-reload after unit changes (systemd)
    Unit         *ServiceUnit   `yaml:"unit" json:"unit,omitempty"`                   // Unit file management
    Dropin       *ServiceDropin `yaml:"dropin" json:"dropin,omitempty"`               // Drop-in configuration file
}
```

<a name="ServiceDropin"></a>
## type [ServiceDropin](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L313-L317>)

ServiceDropin represents a systemd drop\-in configuration file. Drop\-in files are placed in /etc/systemd/system/\<service\>.service.d/\<name\>.conf

```go
type ServiceDropin struct {
    Name        string `yaml:"name" json:"name"`                           // Drop-in file name (e.g., "10-mooncake.conf")
    Content     string `yaml:"content" json:"content,omitempty"`           // Inline content
    SrcTemplate string `yaml:"src_template" json:"src_template,omitempty"` // Template file path
}
```

<a name="ServiceUnit"></a>
## type [ServiceUnit](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L304-L309>)

ServiceUnit represents a systemd unit file or launchd plist configuration.

```go
type ServiceUnit struct {
    Dest        string `yaml:"dest" json:"dest,omitempty"`                 // Unit file path (auto-detected if empty)
    Content     string `yaml:"content" json:"content,omitempty"`           // Inline content
    SrcTemplate string `yaml:"src_template" json:"src_template,omitempty"` // Template file path
    Mode        string `yaml:"mode" json:"mode,omitempty"`                 // File permissions (default: "0644")
}
```

<a name="Shell"></a>
## type [Shell](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L224-L226>)

Shell represents a shell command execution in a configuration step.

Deprecated: Use ShellAction instead.

```go
type Shell struct {
    Command string `yaml:"command"`
}
```

<a name="ShellAction"></a>
## type [ShellAction](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L180-L199>)

ShellAction represents a structured shell command execution in a configuration step. Supports both simple string form and structured object form for backward compatibility.

```go
type ShellAction struct {
    // Cmd is the command to execute (required)
    Cmd string `yaml:"cmd,omitempty" json:"cmd,omitempty"`

    // Interpreter specifies the shell interpreter to use
    // Supported values: "bash", "sh", "pwsh", "cmd"
    // Default: "bash" on Unix, "pwsh" on Windows
    Interpreter string `yaml:"interpreter,omitempty" json:"interpreter,omitempty"`

    // Stdin provides input to the command
    Stdin string `yaml:"stdin,omitempty" json:"stdin,omitempty"`

    // Capture controls whether to capture command output
    // When false, output is only streamed (not stored in result)
    // Default: true
    Capture *bool `yaml:"capture,omitempty" json:"capture,omitempty"`
}
```

<a name="ShellAction.UnmarshalYAML"></a>
### func \(\*ShellAction\) [UnmarshalYAML](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L203>)

```go
func (s *ShellAction) UnmarshalYAML(unmarshal func(interface{}) error) error
```

UnmarshalYAML implements custom YAML unmarshaling to support both string and object forms. Supports: shell: "command" AND shell: \{ cmd: "command", interpreter: "bash", ... \}

<a name="Step"></a>
## type [Step](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L431-L490>)

Step represents a single configuration step that can perform various actions.

```go
type Step struct {
    // Identification
    Name string `yaml:"name" json:"name,omitempty"`

    // Conditionals
    When string `yaml:"when" json:"when,omitempty"`

    // Idempotency controls
    Creates *string `yaml:"creates" json:"creates,omitempty"` // Skip if path exists
    Unless  *string `yaml:"unless" json:"unless,omitempty"`   // Skip if command succeeds

    // Actions (exactly one required)
    Template    *Template               `yaml:"template" json:"template,omitempty"`
    File        *File                   `yaml:"file" json:"file,omitempty"`
    Shell       *ShellAction            `yaml:"shell" json:"shell,omitempty"`
    Command     *CommandAction          `yaml:"command" json:"command,omitempty"`
    Copy        *Copy                   `yaml:"copy" json:"copy,omitempty"`
    Unarchive   *Unarchive              `yaml:"unarchive" json:"unarchive,omitempty"`
    Download    *Download               `yaml:"download" json:"download,omitempty"`
    Package     *Package                `yaml:"package" json:"package,omitempty"`
    Service     *ServiceAction          `yaml:"service" json:"service,omitempty"`
    Assert      *Assert                 `yaml:"assert" json:"assert,omitempty"`
    Preset      *PresetInvocation       `yaml:"preset" json:"preset,omitempty"`
    Print       *PrintAction            `yaml:"print" json:"print,omitempty"`
    Include     *string                 `yaml:"include" json:"include,omitempty"`
    IncludeVars *string                 `yaml:"include_vars" json:"include_vars,omitempty"`
    Vars        *map[string]interface{} `yaml:"vars" json:"vars,omitempty"`

    // Privilege escalation
    Become     bool   `yaml:"become" json:"become,omitempty"`
    BecomeUser string `yaml:"become_user" json:"become_user,omitempty"`

    // Environment
    Env map[string]string `yaml:"env" json:"env,omitempty"`
    Cwd string            `yaml:"cwd" json:"cwd,omitempty"`

    // Execution control
    Timeout    string `yaml:"timeout" json:"timeout,omitempty"`
    Retries    int    `yaml:"retries" json:"retries,omitempty"`
    RetryDelay string `yaml:"retry_delay" json:"retry_delay,omitempty"`

    // Result overrides
    ChangedWhen string `yaml:"changed_when" json:"changed_when,omitempty"`
    FailedWhen  string `yaml:"failed_when" json:"failed_when,omitempty"`

    // Loops
    WithFileTree *string `yaml:"with_filetree" json:"with_filetree,omitempty"`
    WithItems    *string `yaml:"with_items" json:"with_items,omitempty"`

    // Tags and registration
    Tags     []string `yaml:"tags" json:"tags,omitempty"`
    Register string   `yaml:"register" json:"register,omitempty"`

    // Plan metadata (populated during plan expansion, omitted in config files)
    ID          string       `yaml:"id,omitempty" json:"id,omitempty"`
    ActionType  string       `yaml:"action_type,omitempty" json:"action_type,omitempty"`
    Origin      *Origin      `yaml:"origin,omitempty" json:"origin,omitempty"`
    Skipped     bool         `yaml:"skipped,omitempty" json:"skipped,omitempty"`
    LoopContext *LoopContext `yaml:"loop_context,omitempty" json:"loop_context,omitempty"`
}
```

<a name="ReadConfig"></a>
### func [ReadConfig](<https://github.com/alehatsman/mooncake/blob/master/internal/config/reader.go#L159>)

```go
func ReadConfig(path string) ([]Step, error)
```

ReadConfig is a convenience function using the default YAML reader

<a name="Step.Clone"></a>
### func \(\*Step\) [Clone](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L647>)

```go
func (s *Step) Clone() *Step
```

Clone creates a shallow copy of the step.

<a name="Step.DetermineActionType"></a>
### func \(\*Step\) [DetermineActionType](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L563>)

```go
func (s *Step) DetermineActionType() string
```

DetermineActionType returns the action type for this step based on which action field is populated.

<a name="Step.Validate"></a>
### func \(\*Step\) [Validate](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L632>)

```go
func (s *Step) Validate() error
```

Validate checks that the step configuration is valid.

<a name="Step.ValidateHasAction"></a>
### func \(\*Step\) [ValidateHasAction](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L624>)

```go
func (s *Step) ValidateHasAction() error
```

ValidateHasAction checks that the step has at least one action defined.

<a name="Step.ValidateOneAction"></a>
### func \(\*Step\) [ValidateOneAction](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L616>)

```go
func (s *Step) ValidateOneAction() error
```

ValidateOneAction checks that the step has at most one action defined.

<a name="Template"></a>
## type [Template](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L171-L176>)

Template represents a template rendering operation in a configuration step.

```go
type Template struct {
    Src  string                  `yaml:"src" json:"src"`
    Dest string                  `yaml:"dest" json:"dest"`
    Vars *map[string]interface{} `yaml:"vars" json:"vars,omitempty"`
    Mode string                  `yaml:"mode" json:"mode,omitempty"` // Octal file permissions (e.g., "0644", "0755")
}
```

<a name="TemplateValidator"></a>
## type [TemplateValidator](<https://github.com/alehatsman/mooncake/blob/master/internal/config/template_validator.go#L11>)

TemplateValidator validates pongo2 template syntax in configuration fields

```go
type TemplateValidator struct{}
```

<a name="NewTemplateValidator"></a>
### func [NewTemplateValidator](<https://github.com/alehatsman/mooncake/blob/master/internal/config/template_validator.go#L14>)

```go
func NewTemplateValidator() *TemplateValidator
```

NewTemplateValidator creates a new template validator

<a name="TemplateValidator.ValidateSteps"></a>
### func \(\*TemplateValidator\) [ValidateSteps](<https://github.com/alehatsman/mooncake/blob/master/internal/config/template_validator.go#L32>)

```go
func (v *TemplateValidator) ValidateSteps(steps []Step, locationMap *LocationMap, filePath string) []Diagnostic
```

ValidateSteps validates template syntax in all templatable fields across steps Returns diagnostics for any syntax errors found

<a name="TemplateValidator.ValidateSyntax"></a>
### func \(\*TemplateValidator\) [ValidateSyntax](<https://github.com/alehatsman/mooncake/blob/master/internal/config/template_validator.go#L20>)

```go
func (v *TemplateValidator) ValidateSyntax(template string) error
```

ValidateSyntax checks if a template string has valid pongo2 syntax Returns an error if the syntax is invalid

<a name="Unarchive"></a>
## type [Unarchive](<https://github.com/alehatsman/mooncake/blob/master/internal/config/config.go#L259-L265>)

Unarchive represents an archive extraction operation in a configuration step.

```go
type Unarchive struct {
    Src             string `yaml:"src" json:"src"`                                     // Source archive path
    Dest            string `yaml:"dest" json:"dest"`                                   // Destination directory
    StripComponents int    `yaml:"strip_components" json:"strip_components,omitempty"` // Number of leading path components to strip
    Creates         string `yaml:"creates" json:"creates,omitempty"`                   // Skip if this path exists (idempotency marker)
    Mode            string `yaml:"mode" json:"mode,omitempty"`                         // Octal directory permissions (e.g., "0755")
}
```

<a name="ValidationError"></a>
## type [ValidationError](<https://github.com/alehatsman/mooncake/blob/master/internal/config/diagnostic.go#L43-L45>)

ValidationError wraps multiple diagnostics into a single error

```go
type ValidationError struct {
    Diagnostics []Diagnostic
}
```

<a name="ValidationError.Error"></a>
### func \(\*ValidationError\) [Error](<https://github.com/alehatsman/mooncake/blob/master/internal/config/diagnostic.go#L48>)

```go
func (e *ValidationError) Error() string
```

Error implements the error interface

<a name="YAMLConfigReader"></a>
## type [YAMLConfigReader](<https://github.com/alehatsman/mooncake/blob/master/internal/config/reader.go#L18-L20>)

YAMLConfigReader implements Reader for YAML files

```go
type YAMLConfigReader struct {
}
```

<a name="YAMLConfigReader.ReadConfig"></a>
### func \(\*YAMLConfigReader\) [ReadConfig](<https://github.com/alehatsman/mooncake/blob/master/internal/config/reader.go#L30>)

```go
func (r *YAMLConfigReader) ReadConfig(path string) ([]Step, error)
```

ReadConfig reads configuration steps from a YAML file For backward compatibility, this method validates the config and returns an error if any validation errors are found

<a name="YAMLConfigReader.ReadConfigWithValidation"></a>
### func \(\*YAMLConfigReader\) [ReadConfigWithValidation](<https://github.com/alehatsman/mooncake/blob/master/internal/config/reader.go#L46>)

```go
func (r *YAMLConfigReader) ReadConfigWithValidation(path string) ([]Step, []Diagnostic, error)
```

ReadConfigWithValidation reads configuration steps from a YAML file with full validation Returns steps, diagnostics \(which may include warnings\), and any parsing errors

<a name="YAMLConfigReader.ReadVariables"></a>
### func \(\*YAMLConfigReader\) [ReadVariables](<https://github.com/alehatsman/mooncake/blob/master/internal/config/reader.go#L126>)

```go
func (r *YAMLConfigReader) ReadVariables(path string) (map[string]interface{}, error)
```

ReadVariables reads variables from a YAML file

Generated by [gomarkdoc](<https://github.com/princjef/gomarkdoc>)


---

<!-- FILE: api/events.md -->

<!-- Code generated by gomarkdoc. DO NOT EDIT -->

# events

```go
import "github.com/alehatsman/mooncake/internal/events"
```

Package events provides the event system for Mooncake execution lifecycle. Events are emitted during execution and consumed by subscribers for logging, artifacts, and observability.

## Index

- [type ArchiveExtractedData](<#ArchiveExtractedData>)
- [type AssertionData](<#AssertionData>)
- [type ChannelPublisher](<#ChannelPublisher>)
  - [func \(p \*ChannelPublisher\) Close\(\)](<#ChannelPublisher.Close>)
  - [func \(p \*ChannelPublisher\) Flush\(\)](<#ChannelPublisher.Flush>)
  - [func \(p \*ChannelPublisher\) Publish\(event Event\)](<#ChannelPublisher.Publish>)
  - [func \(p \*ChannelPublisher\) Subscribe\(subscriber Subscriber\) int](<#ChannelPublisher.Subscribe>)
  - [func \(p \*ChannelPublisher\) Unsubscribe\(id int\)](<#ChannelPublisher.Unsubscribe>)
- [type Event](<#Event>)
- [type EventType](<#EventType>)
- [type FileCopiedData](<#FileCopiedData>)
- [type FileDownloadedData](<#FileDownloadedData>)
- [type FileOperationData](<#FileOperationData>)
- [type FileRemovedData](<#FileRemovedData>)
- [type LinkCreatedData](<#LinkCreatedData>)
- [type PermissionsChangedData](<#PermissionsChangedData>)
- [type PlanLoadedData](<#PlanLoadedData>)
- [type PresetData](<#PresetData>)
- [type PrintData](<#PrintData>)
- [type Publisher](<#Publisher>)
  - [func NewPublisher\(\) Publisher](<#NewPublisher>)
  - [func NewSyncPublisher\(\) Publisher](<#NewSyncPublisher>)
- [type RunCompletedData](<#RunCompletedData>)
- [type RunStartedData](<#RunStartedData>)
- [type ServiceManagementData](<#ServiceManagementData>)
- [type StepCompletedData](<#StepCompletedData>)
- [type StepFailedData](<#StepFailedData>)
- [type StepOutputData](<#StepOutputData>)
- [type StepSkippedData](<#StepSkippedData>)
- [type StepStartedData](<#StepStartedData>)
- [type Subscriber](<#Subscriber>)
- [type SyncPublisher](<#SyncPublisher>)
  - [func \(p \*SyncPublisher\) Close\(\)](<#SyncPublisher.Close>)
  - [func \(p \*SyncPublisher\) Flush\(\)](<#SyncPublisher.Flush>)
  - [func \(p \*SyncPublisher\) Publish\(event Event\)](<#SyncPublisher.Publish>)
  - [func \(p \*SyncPublisher\) Subscribe\(subscriber Subscriber\) int](<#SyncPublisher.Subscribe>)
  - [func \(p \*SyncPublisher\) Unsubscribe\(id int\)](<#SyncPublisher.Unsubscribe>)
- [type TemplateRenderData](<#TemplateRenderData>)
- [type VarsLoadedData](<#VarsLoadedData>)
- [type VarsSetData](<#VarsSetData>)


<a name="ArchiveExtractedData"></a>
## type [ArchiveExtractedData](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L251-L261>)

ArchiveExtractedData contains data for archive.extracted events

```go
type ArchiveExtractedData struct {
    Src             string `json:"src"`
    Dest            string `json:"dest"`
    Format          string `json:"format"`
    FilesExtracted  int    `json:"files_extracted"`
    DirsCreated     int    `json:"dirs_created"`
    BytesExtracted  int64  `json:"bytes_extracted"`
    StripComponents int    `json:"strip_components,omitempty"`
    DurationMs      int64  `json:"duration_ms"`
    DryRun          bool   `json:"dry_run"`
}
```

<a name="AssertionData"></a>
## type [AssertionData](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L274-L280>)

AssertionData contains data for assert.passed and assert.failed events

```go
type AssertionData struct {
    Type     string `json:"type"`              // Assertion type: "command", "file", or "http"
    Expected string `json:"expected"`          // What was expected
    Actual   string `json:"actual"`            // What was found
    Failed   bool   `json:"failed"`            // Whether the assertion failed
    StepID   string `json:"step_id,omitempty"` // Step ID (added by event bus)
}
```

<a name="ChannelPublisher"></a>
## type [ChannelPublisher](<https://github.com/alehatsman/mooncake/blob/master/internal/events/publisher.go#L23-L32>)

ChannelPublisher implements Publisher using buffered channels

```go
type ChannelPublisher struct {
    // contains filtered or unexported fields
}
```

<a name="ChannelPublisher.Close"></a>
### func \(\*ChannelPublisher\) [Close](<https://github.com/alehatsman/mooncake/blob/master/internal/events/publisher.go#L125>)

```go
func (p *ChannelPublisher) Close()
```

Close closes the publisher and all subscriber channels

<a name="ChannelPublisher.Flush"></a>
### func \(\*ChannelPublisher\) [Flush](<https://github.com/alehatsman/mooncake/blob/master/internal/events/publisher.go#L115>)

```go
func (p *ChannelPublisher) Flush()
```

Flush waits for all pending events to be processed by subscribers

<a name="ChannelPublisher.Publish"></a>
### func \(\*ChannelPublisher\) [Publish](<https://github.com/alehatsman/mooncake/blob/master/internal/events/publisher.go#L46>)

```go
func (p *ChannelPublisher) Publish(event Event)
```

Publish sends an event to all subscribers This is non\-blocking \- if a subscriber's channel is full, the event is dropped

<a name="ChannelPublisher.Subscribe"></a>
### func \(\*ChannelPublisher\) [Subscribe](<https://github.com/alehatsman/mooncake/blob/master/internal/events/publisher.go#L68>)

```go
func (p *ChannelPublisher) Subscribe(subscriber Subscriber) int
```

Subscribe adds a new subscriber and returns its ID

<a name="ChannelPublisher.Unsubscribe"></a>
### func \(\*ChannelPublisher\) [Unsubscribe](<https://github.com/alehatsman/mooncake/blob/master/internal/events/publisher.go#L104>)

```go
func (p *ChannelPublisher) Unsubscribe(id int)
```

Unsubscribe removes a subscriber

<a name="Event"></a>
## type [Event](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L11-L15>)

Event represents a single event in the execution lifecycle

```go
type Event struct {
    Type      EventType   `json:"type"`
    Timestamp time.Time   `json:"timestamp"`
    Data      interface{} `json:"data"`
}
```

<a name="EventType"></a>
## type [EventType](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L18>)

EventType identifies the type of event

```go
type EventType string
```

<a name="EventRunStarted"></a>Event types for run lifecycle

```go
const (
    EventRunStarted   EventType = "run.started"
    EventPlanLoaded   EventType = "plan.loaded"
    EventRunCompleted EventType = "run.completed"
)
```

<a name="EventStepStarted"></a>Event types for step lifecycle

```go
const (
    EventStepStarted   EventType = "step.started"
    EventStepCompleted EventType = "step.completed"
    EventStepSkipped   EventType = "step.skipped"
    EventStepFailed    EventType = "step.failed"
)
```

<a name="EventStepStdout"></a>Event types for output streaming

```go
const (
    EventStepStdout EventType = "step.stdout"
    EventStepStderr EventType = "step.stderr"
    EventStepDebug  EventType = "step.debug"
)
```

<a name="EventFileCreated"></a>Event types for file operations

```go
const (
    EventFileCreated        EventType = "file.created"
    EventFileUpdated        EventType = "file.updated"
    EventFileRemoved        EventType = "file.removed"
    EventFileCopied         EventType = "file.copied"
    EventFileDownloaded     EventType = "file.downloaded"
    EventDirCreated         EventType = "directory.created"
    EventDirRemoved         EventType = "directory.removed"
    EventLinkCreated        EventType = "link.created"
    EventPermissionsChanged EventType = "permissions.changed"
    EventTemplateRender     EventType = "template.rendered"
    EventArchiveExtracted   EventType = "archive.extracted"
)
```

<a name="EventVarsSet"></a>Event types for variables

```go
const (
    EventVarsSet    EventType = "variables.set"
    EventVarsLoaded EventType = "variables.loaded"
)
```

<a name="EventAssertPassed"></a>Event types for assertions

```go
const (
    EventAssertPassed EventType = "assert.passed"
    EventAssertFailed EventType = "assert.failed"
)
```

<a name="EventPresetExpanded"></a>Event types for presets

```go
const (
    EventPresetExpanded  EventType = "preset.expanded"
    EventPresetCompleted EventType = "preset.completed"
)
```

<a name="EventPackageManaged"></a>Event types for package management

```go
const (
    EventPackageManaged EventType = "package.managed"
)
```

<a name="EventPrintMessage"></a>Event types for print

```go
const (
    EventPrintMessage EventType = "print.message"
)
```

<a name="EventServiceManaged"></a>Event types for service management

```go
const (
    EventServiceManaged EventType = "service.managed"
)
```

<a name="FileCopiedData"></a>
## type [FileCopiedData](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L207-L214>)

FileCopiedData contains data for file.copied events

```go
type FileCopiedData struct {
    Src       string `json:"src"`
    Dest      string `json:"dest"`
    SizeBytes int64  `json:"size_bytes"`
    Mode      string `json:"mode"`
    Checksum  string `json:"checksum,omitempty"`
    DryRun    bool   `json:"dry_run"`
}
```

<a name="FileDownloadedData"></a>
## type [FileDownloadedData](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L217-L224>)

FileDownloadedData contains data for file.downloaded events

```go
type FileDownloadedData struct {
    URL       string `json:"url"`
    Dest      string `json:"dest"`
    SizeBytes int64  `json:"size_bytes"`
    Mode      string `json:"mode"`
    Checksum  string `json:"checksum,omitempty"`
    DryRun    bool   `json:"dry_run"`
}
```

<a name="FileOperationData"></a>
## type [FileOperationData](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L172-L178>)

FileOperationData contains data for file operation events

```go
type FileOperationData struct {
    Path      string `json:"path"`
    Mode      string `json:"mode,omitempty"`
    SizeBytes int64  `json:"size_bytes,omitempty"`
    Changed   bool   `json:"changed"`
    DryRun    bool   `json:"dry_run"`
}
```

<a name="FileRemovedData"></a>
## type [FileRemovedData](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L181-L186>)

FileRemovedData contains data for file/directory removal events

```go
type FileRemovedData struct {
    Path      string `json:"path"`
    WasDir    bool   `json:"was_dir"`
    SizeBytes int64  `json:"size_bytes,omitempty"`
    DryRun    bool   `json:"dry_run"`
}
```

<a name="LinkCreatedData"></a>
## type [LinkCreatedData](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L189-L194>)

LinkCreatedData contains data for link creation events

```go
type LinkCreatedData struct {
    Src    string `json:"src"`
    Dest   string `json:"dest"`
    Type   string `json:"type"` // "symlink" or "hardlink"
    DryRun bool   `json:"dry_run"`
}
```

<a name="PermissionsChangedData"></a>
## type [PermissionsChangedData](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L197-L204>)

PermissionsChangedData contains data for permissions.changed events

```go
type PermissionsChangedData struct {
    Path      string `json:"path"`
    Mode      string `json:"mode,omitempty"`
    Owner     string `json:"owner,omitempty"`
    Group     string `json:"group,omitempty"`
    Recursive bool   `json:"recursive"`
    DryRun    bool   `json:"dry_run"`
}
```

<a name="PlanLoadedData"></a>
## type [PlanLoadedData](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L99-L103>)

PlanLoadedData contains data for plan.loaded events

```go
type PlanLoadedData struct {
    RootFile   string   `json:"root_file"`
    TotalSteps int      `json:"total_steps"`
    Tags       []string `json:"tags,omitempty"`
}
```

<a name="PresetData"></a>
## type [PresetData](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L283-L288>)

PresetData contains data for preset events

```go
type PresetData struct {
    Name       string                 `json:"name"`                 // Preset name
    Parameters map[string]interface{} `json:"parameters,omitempty"` // Parameters passed to preset
    StepsCount int                    `json:"steps_count"`          // Number of steps in preset
    Changed    bool                   `json:"changed,omitempty"`    // Whether any step changed (only in completed event)
}
```

<a name="PrintData"></a>
## type [PrintData](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L291-L293>)

PrintData contains data for print.message events

```go
type PrintData struct {
    Message string `json:"message"` // The message that was printed
}
```

<a name="Publisher"></a>
## type [Publisher](<https://github.com/alehatsman/mooncake/blob/master/internal/events/publisher.go#L8-L14>)

Publisher publishes events to subscribers

```go
type Publisher interface {
    Publish(event Event)
    Subscribe(subscriber Subscriber) int
    Unsubscribe(id int)
    Flush()
    Close()
}
```

<a name="NewPublisher"></a>
### func [NewPublisher](<https://github.com/alehatsman/mooncake/blob/master/internal/events/publisher.go#L35>)

```go
func NewPublisher() Publisher
```

NewPublisher creates a new channel\-based event publisher

<a name="NewSyncPublisher"></a>
### func [NewSyncPublisher](<https://github.com/alehatsman/mooncake/blob/master/internal/events/publisher.go#L155>)

```go
func NewSyncPublisher() Publisher
```

NewSyncPublisher creates a new synchronous event publisher for testing.

<a name="RunCompletedData"></a>
## type [RunCompletedData](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L106-L115>)

RunCompletedData contains data for run.completed events

```go
type RunCompletedData struct {
    TotalSteps   int    `json:"total_steps"`
    SuccessSteps int    `json:"success_steps"`
    FailedSteps  int    `json:"failed_steps"`
    SkippedSteps int    `json:"skipped_steps"`
    ChangedSteps int    `json:"changed_steps"`
    DurationMs   int64  `json:"duration_ms"`
    Success      bool   `json:"success"`
    ErrorMessage string `json:"error_message,omitempty"`
}
```

<a name="RunStartedData"></a>
## type [RunStartedData](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L91-L96>)

RunStartedData contains data for run.started events

```go
type RunStartedData struct {
    RootFile   string   `json:"root_file"`
    Tags       []string `json:"tags,omitempty"`
    DryRun     bool     `json:"dry_run"`
    TotalSteps int      `json:"total_steps"`
}
```

<a name="ServiceManagementData"></a>
## type [ServiceManagementData](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L264-L271>)

ServiceManagementData contains data for service.managed events

```go
type ServiceManagementData struct {
    Service    string   `json:"service"`              // Service name
    State      string   `json:"state,omitempty"`      // Desired state (started/stopped/restarted/reloaded)
    Enabled    *bool    `json:"enabled,omitempty"`    // Enabled status
    Changed    bool     `json:"changed"`              // Whether changes were made
    Operations []string `json:"operations,omitempty"` // List of operations performed
    DryRun     bool     `json:"dry_run"`
}
```

<a name="StepCompletedData"></a>
## type [StepCompletedData](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L132-L141>)

StepCompletedData contains data for step.completed events

```go
type StepCompletedData struct {
    StepID     string                 `json:"step_id"`
    Name       string                 `json:"name"`
    Level      int                    `json:"level"`
    DurationMs int64                  `json:"duration_ms"`
    Changed    bool                   `json:"changed"`
    Result     map[string]interface{} `json:"result,omitempty"`
    Depth      int                    `json:"depth,omitempty"` // Directory depth for filetree items
    DryRun     bool                   `json:"dry_run"`
}
```

<a name="StepFailedData"></a>
## type [StepFailedData](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L153-L161>)

StepFailedData contains data for step.failed events

```go
type StepFailedData struct {
    StepID       string `json:"step_id"`
    Name         string `json:"name"`
    Level        int    `json:"level"`
    ErrorMessage string `json:"error_message"`
    DurationMs   int64  `json:"duration_ms"`
    Depth        int    `json:"depth,omitempty"` // Directory depth for filetree items
    DryRun       bool   `json:"dry_run"`
}
```

<a name="StepOutputData"></a>
## type [StepOutputData](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L164-L169>)

StepOutputData contains data for step.stdout/stderr events

```go
type StepOutputData struct {
    StepID     string `json:"step_id"`
    Stream     string `json:"stream"` // "stdout" or "stderr"
    Line       string `json:"line"`
    LineNumber int    `json:"line_number"`
}
```

<a name="StepSkippedData"></a>
## type [StepSkippedData](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L144-L150>)

StepSkippedData contains data for step.skipped events

```go
type StepSkippedData struct {
    StepID string `json:"step_id"`
    Name   string `json:"name"`
    Level  int    `json:"level"`
    Reason string `json:"reason"`
    Depth  int    `json:"depth,omitempty"` // Directory depth for filetree items
}
```

<a name="StepStartedData"></a>
## type [StepStartedData](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L118-L129>)

StepStartedData contains data for step.started events

```go
type StepStartedData struct {
    StepID     string            `json:"step_id"`
    Name       string            `json:"name"`
    Level      int               `json:"level"`
    GlobalStep int               `json:"global_step"`
    Action     string            `json:"action"`
    Tags       []string          `json:"tags,omitempty"`
    When       string            `json:"when,omitempty"`
    Vars       map[string]string `json:"vars,omitempty"`
    Depth      int               `json:"depth,omitempty"` // Directory depth for filetree items
    DryRun     bool              `json:"dry_run"`
}
```

<a name="Subscriber"></a>
## type [Subscriber](<https://github.com/alehatsman/mooncake/blob/master/internal/events/publisher.go#L17-L20>)

Subscriber receives events from a publisher

```go
type Subscriber interface {
    OnEvent(event Event)
    Close()
}
```

<a name="SyncPublisher"></a>
## type [SyncPublisher](<https://github.com/alehatsman/mooncake/blob/master/internal/events/publisher.go#L147-L152>)

SyncPublisher implements Publisher with synchronous event delivery. Events are delivered immediately via direct OnEvent\(\) calls. This is useful for tests to avoid race conditions with async delivery.

```go
type SyncPublisher struct {
    // contains filtered or unexported fields
}
```

<a name="SyncPublisher.Close"></a>
### func \(\*SyncPublisher\) [Close](<https://github.com/alehatsman/mooncake/blob/master/internal/events/publisher.go#L205>)

```go
func (p *SyncPublisher) Close()
```

Close closes the publisher.

<a name="SyncPublisher.Flush"></a>
### func \(\*SyncPublisher\) [Flush](<https://github.com/alehatsman/mooncake/blob/master/internal/events/publisher.go#L200>)

```go
func (p *SyncPublisher) Flush()
```

Flush is a no\-op for SyncPublisher \(already synchronous\).

<a name="SyncPublisher.Publish"></a>
### func \(\*SyncPublisher\) [Publish](<https://github.com/alehatsman/mooncake/blob/master/internal/events/publisher.go#L163>)

```go
func (p *SyncPublisher) Publish(event Event)
```

Publish sends an event to all subscribers synchronously.

<a name="SyncPublisher.Subscribe"></a>
### func \(\*SyncPublisher\) [Subscribe](<https://github.com/alehatsman/mooncake/blob/master/internal/events/publisher.go#L177>)

```go
func (p *SyncPublisher) Subscribe(subscriber Subscriber) int
```

Subscribe adds a new subscriber and returns its ID.

<a name="SyncPublisher.Unsubscribe"></a>
### func \(\*SyncPublisher\) [Unsubscribe](<https://github.com/alehatsman/mooncake/blob/master/internal/events/publisher.go#L192>)

```go
func (p *SyncPublisher) Unsubscribe(id int)
```

Unsubscribe removes a subscriber.

<a name="TemplateRenderData"></a>
## type [TemplateRenderData](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L227-L233>)

TemplateRenderData contains data for template.rendered events

```go
type TemplateRenderData struct {
    TemplatePath string `json:"template_path"`
    DestPath     string `json:"dest_path"`
    SizeBytes    int64  `json:"size_bytes"`
    Changed      bool   `json:"changed"`
    DryRun       bool   `json:"dry_run"`
}
```

<a name="VarsLoadedData"></a>
## type [VarsLoadedData](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L243-L248>)

VarsLoadedData contains data for variables.loaded events

```go
type VarsLoadedData struct {
    FilePath string   `json:"file_path"`
    Count    int      `json:"count"`
    Keys     []string `json:"keys"`
    DryRun   bool     `json:"dry_run"`
}
```

<a name="VarsSetData"></a>
## type [VarsSetData](<https://github.com/alehatsman/mooncake/blob/master/internal/events/event.go#L236-L240>)

VarsSetData contains data for variables.set events

```go
type VarsSetData struct {
    Count  int      `json:"count"`
    Keys   []string `json:"keys"`
    DryRun bool     `json:"dry_run"`
}
```

Generated by [gomarkdoc](<https://github.com/princjef/gomarkdoc>)


---

<!-- FILE: api/executor.md -->

<!-- Code generated by gomarkdoc. DO NOT EDIT -->

# executor

```go
import "github.com/alehatsman/mooncake/internal/executor"
```

Package executor provides the execution engine for mooncake configuration steps.

Package executor implements the core execution engine for mooncake configuration plans.

The executor is responsible for:

- Loading and validating configuration plans
- Expanding steps \(loops, includes, presets\)
- Evaluating conditions \(when, unless, creates\)
- Dispatching actions to handlers
- Managing execution context and variables
- Tracking results and statistics
- Emitting events for observability
- Handling dry\-run mode
- Supporting privilege escalation \(sudo/become\)

### Architecture

The executor follows a pipeline architecture:

```
Plan Loading → Step Expansion → Condition Evaluation → Action Dispatch → Result Handling
```

Each step goes through:

1. Pre\-execution: Check when/unless/creates, apply tags filter
2. Variable processing: Merge step vars into context
3. Loop expansion: Expand with\_items/with\_filetree into multiple executions
4. Action execution: Dispatch to handler or legacy implementation
5. Post\-execution: Evaluate changed\_when/failed\_when, register results
6. Event emission: Publish lifecycle events

### Execution Context

ExecutionContext carries all state needed during execution:

- Variables: Step vars, global vars, facts, registered results
- Template: Jinja2\-like template renderer
- Evaluator: Expression evaluator for conditions
- Logger: Structured logging \(TUI or text\)
- PathUtil: Path resolution and expansion
- EventPublisher: Event emission for observability
- Stats: Execution statistics \(total, success, failed, changed, skipped\)

### Action Dispatch

Actions are dispatched through two paths:

1. Handler\-based \(new\): Look up handler in actions.Registry, call handler.Execute\(\)
2. Legacy: Direct executor methods \(HandleShell, HandleFile, etc.\)

The executor prefers handlers when available, falling back to legacy for non\-migrated actions.

### Idempotency

The executor enforces idempotency through:

- creates: Skip if path exists
- unless: Skip if command succeeds
- changed\_when: Custom change detection
- Handler implementations: Built\-in state checking

### Dry\\\-Run Mode

When DryRun is true:

- No actual changes are made to the system
- Handlers log what would happen
- Template rendering still occurs \(validates syntax\)
- File existence checks are performed \(read\-only\)
- Statistics track what would have changed

### Error Handling

Errors are wrapped with context using custom error types:

- RenderError: Template rendering failures \(field \+ cause\)
- EvaluationError: Expression evaluation failures \(expression \+ cause\)
- CommandError: Command execution failures \(command \+ exit code\)
- FileOperationError: File operation failures \(path \+ operation \+ cause\)
- StepValidationError: Configuration validation failures
- SetupError: Infrastructure/environment setup failures

Use errors.Is\(\) and errors.As\(\) for programmatic error inspection.

### Usage Example

```
// Load configuration
steps, err := config.ReadConfig("config.yml")
if err != nil {
    return err
}

// Create executor
log := logger.NewTextLogger()
exec := NewExecutor(log)

// Execute with options
result, err := exec.Execute(config.Plan{Steps: steps}, ExecuteOptions{
    DryRun: false,
    Tags: []string{"setup", "deploy"},
    Variables: map[string]interface{}{
        "environment": "production",
    },
})

// Check results
if !result.Success {
    log.Errorf("Execution failed: %d failed steps", result.FailedSteps)
}
log.Infof("Summary: %d changed, %d unchanged, %d failed",
    result.ChangedSteps, result.SuccessSteps-result.ChangedSteps, result.FailedSteps)
```

## Index

- [func AddGlobalVariables\(variables map\[string\]interface\{\}\)](<#AddGlobalVariables>)
- [func CheckIdempotencyConditions\(step config.Step, ec \*ExecutionContext\) \(bool, string, error\)](<#CheckIdempotencyConditions>)
- [func CheckSkipConditions\(step config.Step, ec \*ExecutionContext\) \(bool, string, error\)](<#CheckSkipConditions>)
- [func DispatchStepAction\(step config.Step, ec \*ExecutionContext\) error](<#DispatchStepAction>)
- [func ExecutePlan\(p \*plan.Plan, sudoPass string, dryRun bool, log logger.Logger, publisher events.Publisher\) error](<#ExecutePlan>)
- [func ExecuteStep\(step config.Step, ec \*ExecutionContext\) error](<#ExecuteStep>)
- [func ExecuteSteps\(steps \[\]config.Step, ec \*ExecutionContext\) error](<#ExecuteSteps>)
- [func GetStepDisplayName\(step config.Step, ec \*ExecutionContext\) \(string, bool\)](<#GetStepDisplayName>)
- [func HandleVars\(step config.Step, ec \*ExecutionContext\) error](<#HandleVars>)
- [func HandleWhenExpression\(step config.Step, ec \*ExecutionContext\) \(bool, error\)](<#HandleWhenExpression>)
- [func MarkStepFailed\(result \*Result, step config.Step, ec \*ExecutionContext\)](<#MarkStepFailed>)
- [func ParseFileMode\(modeStr string, defaultMode os.FileMode\) os.FileMode](<#ParseFileMode>)
- [func ShouldSkipByTags\(step config.Step, ec \*ExecutionContext\) bool](<#ShouldSkipByTags>)
- [func Start\(StartConfig StartConfig, log logger.Logger, publisher events.Publisher\) error](<#Start>)
- [type AssertionError](<#AssertionError>)
  - [func \(e \*AssertionError\) Error\(\) string](<#AssertionError.Error>)
  - [func \(e \*AssertionError\) Unwrap\(\) error](<#AssertionError.Unwrap>)
- [type CommandError](<#CommandError>)
  - [func \(e \*CommandError\) Error\(\) string](<#CommandError.Error>)
  - [func \(e \*CommandError\) Unwrap\(\) error](<#CommandError.Unwrap>)
- [type EvaluationError](<#EvaluationError>)
  - [func \(e \*EvaluationError\) Error\(\) string](<#EvaluationError.Error>)
  - [func \(e \*EvaluationError\) Unwrap\(\) error](<#EvaluationError.Unwrap>)
- [type ExecutionContext](<#ExecutionContext>)
  - [func \(ec \*ExecutionContext\) Clone\(\) ExecutionContext](<#ExecutionContext.Clone>)
  - [func \(ec \*ExecutionContext\) EmitEvent\(eventType events.EventType, data interface\{\}\)](<#ExecutionContext.EmitEvent>)
  - [func \(ec \*ExecutionContext\) GetCurrentStepID\(\) string](<#ExecutionContext.GetCurrentStepID>)
  - [func \(ec \*ExecutionContext\) GetEvaluator\(\) expression.Evaluator](<#ExecutionContext.GetEvaluator>)
  - [func \(ec \*ExecutionContext\) GetEventPublisher\(\) events.Publisher](<#ExecutionContext.GetEventPublisher>)
  - [func \(ec \*ExecutionContext\) GetLogger\(\) logger.Logger](<#ExecutionContext.GetLogger>)
  - [func \(ec \*ExecutionContext\) GetTemplate\(\) template.Renderer](<#ExecutionContext.GetTemplate>)
  - [func \(ec \*ExecutionContext\) GetVariables\(\) map\[string\]interface\{\}](<#ExecutionContext.GetVariables>)
  - [func \(ec \*ExecutionContext\) HandleDryRun\(logFn func\(\*dryRunLogger\)\) bool](<#ExecutionContext.HandleDryRun>)
  - [func \(ec \*ExecutionContext\) IsDryRun\(\) bool](<#ExecutionContext.IsDryRun>)
- [type ExecutionStats](<#ExecutionStats>)
  - [func NewExecutionStats\(\) \*ExecutionStats](<#NewExecutionStats>)
- [type FileOperationError](<#FileOperationError>)
  - [func \(e \*FileOperationError\) Error\(\) string](<#FileOperationError.Error>)
  - [func \(e \*FileOperationError\) Unwrap\(\) error](<#FileOperationError.Unwrap>)
- [type RenderError](<#RenderError>)
  - [func \(e \*RenderError\) Error\(\) string](<#RenderError.Error>)
  - [func \(e \*RenderError\) Unwrap\(\) error](<#RenderError.Unwrap>)
- [type Result](<#Result>)
  - [func NewResult\(\) \*Result](<#NewResult>)
  - [func \(r \*Result\) RegisterTo\(variables map\[string\]interface\{\}, name string\)](<#Result.RegisterTo>)
  - [func \(r \*Result\) SetChanged\(changed bool\)](<#Result.SetChanged>)
  - [func \(r \*Result\) SetData\(data map\[string\]interface\{\}\)](<#Result.SetData>)
  - [func \(r \*Result\) SetFailed\(failed bool\)](<#Result.SetFailed>)
  - [func \(r \*Result\) SetStderr\(stderr string\)](<#Result.SetStderr>)
  - [func \(r \*Result\) SetStdout\(stdout string\)](<#Result.SetStdout>)
  - [func \(r \*Result\) Status\(\) string](<#Result.Status>)
  - [func \(r \*Result\) ToMap\(\) map\[string\]interface\{\}](<#Result.ToMap>)
- [type SetupError](<#SetupError>)
  - [func \(e \*SetupError\) Error\(\) string](<#SetupError.Error>)
  - [func \(e \*SetupError\) Unwrap\(\) error](<#SetupError.Unwrap>)
- [type StartConfig](<#StartConfig>)
- [type StepValidationError](<#StepValidationError>)
  - [func \(e \*StepValidationError\) Error\(\) string](<#StepValidationError.Error>)


<a name="AddGlobalVariables"></a>
## func [AddGlobalVariables](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/executor.go#L160>)

```go
func AddGlobalVariables(variables map[string]interface{})
```

AddGlobalVariables injects system facts into the variables map. This makes facts like ansible\_os\_family, ansible\_distribution, etc. available during planning.

<a name="CheckIdempotencyConditions"></a>
## func [CheckIdempotencyConditions](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/executor.go#L278>)

```go
func CheckIdempotencyConditions(step config.Step, ec *ExecutionContext) (bool, string, error)
```

CheckIdempotencyConditions evaluates creates and unless conditions for shell steps. Returns \(shouldSkip bool, reason string, error\)

INTERNAL: This function is exported for testing purposes only and is not part of the public API. It may change or be removed in future versions without notice.

<a name="CheckSkipConditions"></a>
## func [CheckSkipConditions](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/executor.go#L334>)

```go
func CheckSkipConditions(step config.Step, ec *ExecutionContext) (bool, string, error)
```

CheckSkipConditions evaluates whether a step should be skipped based on conditional expressions and tag filters.

It first evaluates the step's "when" condition \(if present\), which is an expression that must evaluate to true for the step to execute. If the condition evaluates to false, the step is skipped with reason "when".

Next, it checks if the step should be skipped based on tag filtering. If the execution context has a tags filter and the step's tags don't match, it's skipped with reason "tags".

Returns:

- shouldSkip: true if the step should be skipped
- skipReason: "when" or "tags" indicating why the step was skipped \(empty if not skipped\)
- error: any error encountered while evaluating conditions

INTERNAL: This function is exported for testing purposes only and is not part of the public API. It may change or be removed in future versions without notice.

<a name="DispatchStepAction"></a>
## func [DispatchStepAction](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/executor.go#L413>)

```go
func DispatchStepAction(step config.Step, ec *ExecutionContext) error
```

DispatchStepAction executes the appropriate handler based on step type. All actions are now handled through the actions registry.

INTERNAL: This function is exported for testing purposes only and is not part of the public API. It may change or be removed in future versions without notice.

<a name="ExecutePlan"></a>
## func [ExecutePlan](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/executor.go#L797>)

```go
func ExecutePlan(p *plan.Plan, sudoPass string, dryRun bool, log logger.Logger, publisher events.Publisher) error
```

ExecutePlan executes a pre\-compiled plan. Emits events through the provided publisher for all execution progress.

<a name="ExecuteStep"></a>
## func [ExecuteStep](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/executor.go#L475>)

```go
func ExecuteStep(step config.Step, ec *ExecutionContext) error
```

ExecuteStep executes a single configuration step within the given execution context.

<a name="ExecuteSteps"></a>
## func [ExecuteSteps](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/executor.go#L634>)

```go
func ExecuteSteps(steps []config.Step, ec *ExecutionContext) error
```

ExecuteSteps executes a sequence of configuration steps within the given execution context.

<a name="GetStepDisplayName"></a>
## func [GetStepDisplayName](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/executor.go#L367>)

```go
func GetStepDisplayName(step config.Step, ec *ExecutionContext) (string, bool)
```

GetStepDisplayName determines the display name to show for a step in logs and output.

The function follows a priority order to determine the name:

1. If executing within a with\_filetree loop, uses action \+ destination path
2. If executing within a with\_items loop, uses the string representation of the item
3. Otherwise, uses the step's configured Name field

Returns:

- displayName: the name to display for this step
- hasName: true if a name was found, false if the step is anonymous

INTERNAL: This function is exported for testing purposes only and is not part of the public API. It may change or be removed in future versions without notice.

<a name="HandleVars"></a>
## func [HandleVars](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/executor.go#L174>)

```go
func HandleVars(step config.Step, ec *ExecutionContext) error
```

HandleVars processes the vars field of a step, rendering templates and merging into the execution context.

INTERNAL: This function is exported for testing purposes only and is not part of the public API. It may change or be removed in future versions without notice.

<a name="HandleWhenExpression"></a>
## func [HandleWhenExpression](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/executor.go#L209>)

```go
func HandleWhenExpression(step config.Step, ec *ExecutionContext) (bool, error)
```

HandleWhenExpression evaluates the when condition and returns whether the step should be skipped. Returns \(shouldSkip bool, error\).

INTERNAL: This function is exported for testing purposes only and is not part of the public API. It may change or be removed in future versions without notice.

<a name="MarkStepFailed"></a>
## func [MarkStepFailed](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/executor.go#L150>)

```go
func MarkStepFailed(result *Result, step config.Step, ec *ExecutionContext)
```

MarkStepFailed marks a result as failed and registers it if needed. The caller is responsible for returning an appropriate error.

INTERNAL: This function is exported for testing purposes only and is not part of the public API. It may change or be removed in future versions without notice.

<a name="ParseFileMode"></a>
## func [ParseFileMode](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/executor.go#L935>)

```go
func ParseFileMode(modeStr string, defaultMode os.FileMode) os.FileMode
```

ParseFileMode parses a mode string \(e.g., "0644"\) into os.FileMode. Returns default mode if mode is empty or invalid.

INTERNAL: This function is exported for testing purposes only and is not part of the public API. It may change or be removed in future versions without notice.

<a name="ShouldSkipByTags"></a>
## func [ShouldSkipByTags](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/executor.go#L249>)

```go
func ShouldSkipByTags(step config.Step, ec *ExecutionContext) bool
```

ShouldSkipByTags determines if a step should be skipped based on tag filtering. Returns true if the step should be skipped, false otherwise.

INTERNAL: This function is exported for testing purposes only and is not part of the public API. It may change or be removed in future versions without notice.

<a name="Start"></a>
## func [Start](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/executor.go#L694>)

```go
func Start(StartConfig StartConfig, log logger.Logger, publisher events.Publisher) error
```

Start begins execution of a mooncake configuration with the given settings. Always goes through the planner to expand loops, includes, and variables. Emits events through the provided publisher for all execution progress.

<a name="AssertionError"></a>
## type [AssertionError](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/errors.go#L126-L132>)

AssertionError represents an assertion verification failure. Unlike other errors, assertions are expected to fail when conditions aren't met.

```go
type AssertionError struct {
    Type     string // "command", "file", "http"
    Expected string // What was expected
    Actual   string // What was found
    Details  string // Additional context (optional)
    Cause    error  // Underlying error (optional)
}
```

<a name="AssertionError.Error"></a>
### func \(\*AssertionError\) [Error](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/errors.go#L134>)

```go
func (e *AssertionError) Error() string
```



<a name="AssertionError.Unwrap"></a>
### func \(\*AssertionError\) [Unwrap](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/errors.go#L145>)

```go
func (e *AssertionError) Unwrap() error
```



<a name="CommandError"></a>
## type [CommandError](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/errors.go#L53-L58>)

CommandError represents a command execution failure

```go
type CommandError struct {
    ExitCode int
    Timeout  bool
    Duration string
    Cause    error // Optional underlying error (e.g., exec.ExitError, OS errors)
}
```

<a name="CommandError.Error"></a>
### func \(\*CommandError\) [Error](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/errors.go#L60>)

```go
func (e *CommandError) Error() string
```



<a name="CommandError.Unwrap"></a>
### func \(\*CommandError\) [Unwrap](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/errors.go#L74>)

```go
func (e *CommandError) Unwrap() error
```



<a name="EvaluationError"></a>
## type [EvaluationError](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/errors.go#L36-L39>)

EvaluationError represents an expression evaluation failure

```go
type EvaluationError struct {
    Expression string
    Cause      error
}
```

<a name="EvaluationError.Error"></a>
### func \(\*EvaluationError\) [Error](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/errors.go#L41>)

```go
func (e *EvaluationError) Error() string
```



<a name="EvaluationError.Unwrap"></a>
### func \(\*EvaluationError\) [Unwrap](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/errors.go#L48>)

```go
func (e *EvaluationError) Unwrap() error
```



<a name="ExecutionContext"></a>
## type [ExecutionContext](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/context.go#L51-L126>)

ExecutionContext holds all state needed to execute a step or sequence of steps.

The context is designed to be copied when entering nested execution scopes \(includes, loops\). Most fields are copied by value, but certain fields use pointers to maintain shared state across the entire execution tree.

Field categories:

- Configuration: Variables, CurrentDir, CurrentFile \(copied on nested contexts\)
- Display state: Level, CurrentIndex, TotalSteps \(modified for each scope\)
- Execution settings: Logger, SudoPass, Tags, DryRun \(shared across contexts\)
- Global counters: Pointers that accumulate across all contexts
- Dependencies: Shared service instances

```go
type ExecutionContext struct {
    // Variables contains template variables available to steps.
    // Copied on context copy so nested contexts can have their own variables (e.g., loop items).
    Variables map[string]interface{}

    // CurrentDir is the directory containing the current config file.
    // Used for resolving relative paths in include, template src, etc.
    CurrentDir string

    // CurrentFile is the absolute path to the current config file being executed.
    // Used for error messages and debugging.
    CurrentFile string

    // Level tracks nesting depth for display indentation.
    // 0 = root config, increments by 1 for each include or loop level.
    Level int

    // CurrentIndex is the 0-based index of the current step within the current scope.
    // Resets to 0 when entering includes or loops.
    CurrentIndex int

    // TotalSteps is the number of steps in the current execution scope.
    // Updated when entering includes or loops to reflect the new scope size.
    TotalSteps int

    // Logger handles all output, configured with padding based on Level.
    Logger logger.Logger

    // SudoPass is the password used for steps with become: true.
    // Empty string if not provided via --sudo-pass flag.
    SudoPass string

    // Tags filters which steps execute (empty = all steps execute).
    // Steps without matching tags are skipped when this is non-empty.
    Tags []string

    // DryRun when true prevents any system changes (preview mode).
    // Commands are not executed, files are not created, templates are not rendered.
    DryRun bool

    // Stats holds shared execution statistics counters.
    // SHARED via pointer - all contexts update the same counters.
    Stats *ExecutionStats

    // Template renders template strings with variable substitution.
    // SHARED across all contexts - same instance used everywhere.
    Template template.Renderer

    // Evaluator evaluates when condition expressions.
    // SHARED across all contexts - same instance used everywhere.
    Evaluator expression.Evaluator

    // PathUtil expands paths with tilde and variable substitution.
    // SHARED across all contexts - same instance used everywhere.
    PathUtil *pathutil.PathExpander

    // FileTree walks directory trees for with_filetree.
    // SHARED across all contexts - same instance used everywhere.
    FileTree *filetree.Walker

    // Redactor redacts sensitive values (passwords) from log output.
    // SHARED across all contexts - same instance used everywhere.
    Redactor *security.Redactor

    // EventPublisher publishes execution events to subscribers.
    // SHARED across all contexts - same instance used everywhere.
    EventPublisher events.Publisher

    // CurrentStepID is the unique identifier for the currently executing step.
    // Used for correlating events from the same step execution.
    CurrentStepID string

    // CurrentResult holds the result of the currently executing step.
    // Handlers should set this to provide result data to event emission.
    CurrentResult *Result
}
```

<a name="ExecutionContext.Clone"></a>
### func \(\*ExecutionContext\) [Clone](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/context.go#L130>)

```go
func (ec *ExecutionContext) Clone() ExecutionContext
```

Clone creates a new ExecutionContext for a nested execution scope \(include or loop\). Variables map is shallow copied, display fields are copied by value, and pointer fields remain shared across all contexts.

<a name="ExecutionContext.EmitEvent"></a>
### func \(\*ExecutionContext\) [EmitEvent](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/context.go#L165>)

```go
func (ec *ExecutionContext) EmitEvent(eventType events.EventType, data interface{})
```

EmitEvent publishes an event to all subscribers

<a name="ExecutionContext.GetCurrentStepID"></a>
### func \(\*ExecutionContext\) [GetCurrentStepID](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/context.go#L222>)

```go
func (ec *ExecutionContext) GetCurrentStepID() string
```

GetCurrentStepID returns the current step ID.

<a name="ExecutionContext.GetEvaluator"></a>
### func \(\*ExecutionContext\) [GetEvaluator](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/context.go#L197>)

```go
func (ec *ExecutionContext) GetEvaluator() expression.Evaluator
```

GetEvaluator returns the expression evaluator.

<a name="ExecutionContext.GetEventPublisher"></a>
### func \(\*ExecutionContext\) [GetEventPublisher](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/context.go#L212>)

```go
func (ec *ExecutionContext) GetEventPublisher() events.Publisher
```

GetEventPublisher returns the event publisher.

<a name="ExecutionContext.GetLogger"></a>
### func \(\*ExecutionContext\) [GetLogger](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/context.go#L202>)

```go
func (ec *ExecutionContext) GetLogger() logger.Logger
```

GetLogger returns the logger.

<a name="ExecutionContext.GetTemplate"></a>
### func \(\*ExecutionContext\) [GetTemplate](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/context.go#L192>)

```go
func (ec *ExecutionContext) GetTemplate() template.Renderer
```

GetTemplate returns the template renderer.

<a name="ExecutionContext.GetVariables"></a>
### func \(\*ExecutionContext\) [GetVariables](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/context.go#L207>)

```go
func (ec *ExecutionContext) GetVariables() map[string]interface{}
```

GetVariables returns the execution variables.

<a name="ExecutionContext.HandleDryRun"></a>
### func \(\*ExecutionContext\) [HandleDryRun](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/context.go#L178>)

```go
func (ec *ExecutionContext) HandleDryRun(logFn func(*dryRunLogger)) bool
```

HandleDryRun executes dry\-run logging if in dry\-run mode. Returns true if in dry\-run mode \(caller should return early\). The logFn is called with a dryRunLogger to perform logging.

<a name="ExecutionContext.IsDryRun"></a>
### func \(\*ExecutionContext\) [IsDryRun](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/context.go#L217>)

```go
func (ec *ExecutionContext) IsDryRun() bool
```

IsDryRun returns true if this is a dry\-run execution.

<a name="ExecutionStats"></a>
## type [ExecutionStats](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/context.go#L18-L27>)

ExecutionStats holds shared statistics counters for execution tracking. All fields are pointers to enable shared state across nested execution contexts.

```go
type ExecutionStats struct {
    // Global tracks total non-skipped steps across the entire execution tree
    Global *int
    // Executed counts successfully completed steps
    Executed *int
    // Skipped counts steps skipped due to when conditions or tag filtering
    Skipped *int
    // Failed counts steps that failed with errors
    Failed *int
}
```

<a name="NewExecutionStats"></a>
### func [NewExecutionStats](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/context.go#L30>)

```go
func NewExecutionStats() *ExecutionStats
```

NewExecutionStats creates a new ExecutionStats with all counters initialized to zero

<a name="FileOperationError"></a>
## type [FileOperationError](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/errors.go#L79-L83>)

FileOperationError represents a file operation failure

```go
type FileOperationError struct {
    Operation string // "create", "read", "write", "delete", "chmod", "chown", "link"
    Path      string
    Cause     error
}
```

<a name="FileOperationError.Error"></a>
### func \(\*FileOperationError\) [Error](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/errors.go#L85>)

```go
func (e *FileOperationError) Error() string
```



<a name="FileOperationError.Unwrap"></a>
### func \(\*FileOperationError\) [Unwrap](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/errors.go#L92>)

```go
func (e *FileOperationError) Unwrap() error
```



<a name="RenderError"></a>
## type [RenderError](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/errors.go#L19-L22>)

RenderError represents a template rendering failure

```go
type RenderError struct {
    Field string
    Cause error
}
```

<a name="RenderError.Error"></a>
### func \(\*RenderError\) [Error](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/errors.go#L24>)

```go
func (e *RenderError) Error() string
```



<a name="RenderError.Unwrap"></a>
### func \(\*RenderError\) [Unwrap](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/errors.go#L31>)

```go
func (e *RenderError) Unwrap() error
```



<a name="Result"></a>
## type [Result](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/result.go#L33-L65>)

Result represents the outcome of executing a step and can be registered to variables for use in subsequent steps via the "register" field.

Field usage varies by step type:

Shell steps:

- Stdout: captured standard output from the command
- Stderr: captured standard error from the command
- Rc: exit code \(0 for success, non\-zero for failure\)
- Failed: true if Rc \!= 0
- Changed: always true \(commands are assumed to make changes\)

File steps \(file with state: file or directory\):

- Rc: 0 for success, 1 for failure
- Failed: true if file/directory operation failed
- Changed: true if file/directory was created or content modified

Template steps:

- Rc: 0 for success, 1 for failure
- Failed: true if template rendering or file write failed
- Changed: true if output file was created or content changed

Variable steps \(vars, include\_vars\):

- All fields remain at default values \(not currently used\)

The Skipped field is reserved for future use but not currently set by any step type.

```go
type Result struct {
    // Stdout contains the standard output from shell commands.
    // Only populated by shell steps.
    Stdout string `json:"stdout"`

    // Stderr contains the standard error from shell commands.
    // Only populated by shell steps.
    Stderr string `json:"stderr"`

    // Rc is the return/exit code.
    // For shell steps: the command's exit code (0 = success).
    // For file/template steps: 0 for success, 1 for failure.
    Rc  int `json:"rc"`

    // Failed indicates whether the step execution failed.
    // Set to true when shell commands exit non-zero or when file/template operations error.
    Failed bool `json:"failed"`

    // Changed indicates whether the step made modifications to the system.
    // Shell steps: always true (commands assumed to make changes).
    // File steps: true if file/directory was created or modified.
    // Template steps: true if output file was created or content changed.
    Changed bool `json:"changed"`

    // Skipped is reserved for future use to indicate skipped steps.
    // Currently not set by any step type.
    Skipped bool `json:"skipped"`

    // Timing information
    StartTime time.Time     `json:"start_time,omitempty"`
    EndTime   time.Time     `json:"end_time,omitempty"`
    Duration  time.Duration `json:"duration_ms,omitempty"` // Duration in time.Duration format
}
```

<a name="NewResult"></a>
### func [NewResult](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/result.go#L68>)

```go
func NewResult() *Result
```

NewResult creates a new Result with default values.

<a name="Result.RegisterTo"></a>
### func \(\*Result\) [RegisterTo](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/result.go#L109>)

```go
func (r *Result) RegisterTo(variables map[string]interface{}, name string)
```

RegisterTo registers this result to the variables map under the given name. The result can be accessed using nested field syntax \(e.g., "result.stdout", "result.rc"\) in templates and when conditions.

<a name="Result.SetChanged"></a>
### func \(\*Result\) [SetChanged](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/result.go#L118>)

```go
func (r *Result) SetChanged(changed bool)
```

SetChanged marks whether the action made changes.

<a name="Result.SetData"></a>
### func \(\*Result\) [SetData](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/result.go#L142>)

```go
func (r *Result) SetData(data map[string]interface{})
```

SetData sets custom result data. This merges the provided data into the result's ToMap output.

<a name="Result.SetFailed"></a>
### func \(\*Result\) [SetFailed](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/result.go#L133>)

```go
func (r *Result) SetFailed(failed bool)
```

SetFailed marks the result as failed.

<a name="Result.SetStderr"></a>
### func \(\*Result\) [SetStderr](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/result.go#L128>)

```go
func (r *Result) SetStderr(stderr string)
```

SetStderr sets the stderr output.

<a name="Result.SetStdout"></a>
### func \(\*Result\) [SetStdout](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/result.go#L123>)

```go
func (r *Result) SetStdout(stdout string)
```

SetStdout sets the stdout output.

<a name="Result.Status"></a>
### func \(\*Result\) [Status](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/result.go#L80>)

```go
func (r *Result) Status() string
```

Status returns a string representation of the result status.

<a name="Result.ToMap"></a>
### func \(\*Result\) [ToMap](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/result.go#L94>)

```go
func (r *Result) ToMap() map[string]interface{}
```

ToMap converts Result to a map for use in template variables.

<a name="SetupError"></a>
## type [SetupError](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/errors.go#L107-L111>)

SetupError represents infrastructure or configuration setup failures

```go
type SetupError struct {
    Component string // "become", "timeout", "sudo", "user", "group"
    Issue     string // What went wrong
    Cause     error  // Underlying error (optional)
}
```

<a name="SetupError.Error"></a>
### func \(\*SetupError\) [Error](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/errors.go#L113>)

```go
func (e *SetupError) Error() string
```



<a name="SetupError.Unwrap"></a>
### func \(\*SetupError\) [Unwrap](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/errors.go#L120>)

```go
func (e *SetupError) Unwrap() error
```



<a name="StartConfig"></a>
## type [StartConfig](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/executor.go#L674-L689>)

StartConfig contains configuration for starting a mooncake execution.

```go
type StartConfig struct {
    ConfigFilePath   string
    VarsFilePath     string
    SudoPass         string // Sudo password provided directly (use SudoPassFile for better security)
    SudoPassFile     string
    AskBecomePass    bool
    InsecureSudoPass bool
    Tags             []string
    DryRun           bool

    // Artifact configuration
    ArtifactsDir      string
    CaptureFullOutput bool
    MaxOutputBytes    int
    MaxOutputLines    int
}
```

<a name="StepValidationError"></a>
## type [StepValidationError](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/errors.go#L97-L100>)

StepValidationError represents step parameter validation failure during execution

```go
type StepValidationError struct {
    Field   string
    Message string
}
```

<a name="StepValidationError.Error"></a>
### func \(\*StepValidationError\) [Error](<https://github.com/alehatsman/mooncake/blob/master/internal/executor/errors.go#L102>)

```go
func (e *StepValidationError) Error() string
```



Generated by [gomarkdoc](<https://github.com/princjef/gomarkdoc>)


---

<!-- FILE: api/facts.md -->

<!-- Code generated by gomarkdoc. DO NOT EDIT -->

# facts

```go
import "github.com/alehatsman/mooncake/internal/facts"
```

Package facts provides system information collection for different operating systems.

## Index

- [func ClearCache\(\)](<#ClearCache>)
- [type Disk](<#Disk>)
- [type Facts](<#Facts>)
  - [func Collect\(\) \*Facts](<#Collect>)
  - [func \(f \*Facts\) ToMap\(\) map\[string\]interface\{\}](<#Facts.ToMap>)
- [type GPU](<#GPU>)
- [type NetworkInterface](<#NetworkInterface>)
- [type OllamaModel](<#OllamaModel>)


<a name="ClearCache"></a>
## func [ClearCache](<https://github.com/alehatsman/mooncake/blob/master/internal/facts/cache.go#L21>)

```go
func ClearCache()
```

ClearCache forces re\-collection on next Collect\(\) call. This is primarily intended for testing purposes.

<a name="Disk"></a>
## type [Disk](<https://github.com/alehatsman/mooncake/blob/master/internal/facts/facts.go#L77-L85>)

Disk represents a storage device.

```go
type Disk struct {
    Device     string
    MountPoint string
    Filesystem string
    SizeGB     int64
    UsedGB     int64
    AvailGB    int64
    UsedPct    int
}
```

<a name="Facts"></a>
## type [Facts](<https://github.com/alehatsman/mooncake/blob/master/internal/facts/facts.go#L13-L65>)

Facts contains collected system information.

```go
type Facts struct {
    // Basic
    OS       string
    Arch     string
    Hostname string
    Username string
    UserHome string

    // Distribution (Linux)
    Distribution        string
    DistributionVersion string
    DistributionMajor   string

    // Network
    IPAddresses       []string
    NetworkInterfaces []NetworkInterface

    // Hardware
    CPUCores      int
    MemoryTotalMB int64
    Disks         []Disk
    GPUs          []GPU

    // OS Details
    KernelVersion string // "6.5.0-14-generic" (Linux), "23.6.0" (macOS)

    // CPU Extended
    CPUModel string   // "Intel(R) Core(TM) i9-9900K CPU @ 3.60GHz"
    CPUFlags []string // ["avx", "avx2", "sse4_2", "fma", ...]

    // Memory Extended
    MemoryFreeMB int64 // Available memory
    SwapTotalMB  int64 // Swap size
    SwapFreeMB   int64 // Swap available

    // Network Extended
    DefaultGateway string   // "192.168.1.1"
    DNSServers     []string // ["8.8.8.8", "1.1.1.1"]

    // Software
    PythonVersion  string
    PackageManager string

    // Toolchains
    DockerVersion string // "24.0.7"
    GitVersion    string // "2.43.0"
    GoVersion     string // "1.21.5"

    // Ollama (optional)
    OllamaVersion  string        // "0.1.47"
    OllamaModels   []OllamaModel // List of installed models
    OllamaEndpoint string        // "http://localhost:11434"
}
```

<a name="Collect"></a>
### func [Collect](<https://github.com/alehatsman/mooncake/blob/master/internal/facts/cache.go#L12>)

```go
func Collect() *Facts
```

Collect gathers system facts with per\-process caching. Facts are collected only once per execution and cached in memory.

<a name="Facts.ToMap"></a>
### func \(\*Facts\) [ToMap](<https://github.com/alehatsman/mooncake/blob/master/internal/facts/facts.go#L147>)

```go
func (f *Facts) ToMap() map[string]interface{}
```

ToMap converts Facts to a map for use in templates.

<a name="GPU"></a>
## type [GPU](<https://github.com/alehatsman/mooncake/blob/master/internal/facts/facts.go#L88-L94>)

GPU represents a graphics card.

```go
type GPU struct {
    Vendor      string // nvidia, amd, intel
    Model       string
    Memory      string // e.g. "8GB", "24GB"
    Driver      string
    CUDAVersion string // "12.3" (NVIDIA only)
}
```

<a name="NetworkInterface"></a>
## type [NetworkInterface](<https://github.com/alehatsman/mooncake/blob/master/internal/facts/facts.go#L68-L74>)

NetworkInterface represents a network interface.

```go
type NetworkInterface struct {
    Name       string
    MACAddress string
    MTU        int
    Addresses  []string
    Up         bool
}
```

<a name="OllamaModel"></a>
## type [OllamaModel](<https://github.com/alehatsman/mooncake/blob/master/internal/facts/facts.go#L97-L102>)

OllamaModel represents an installed Ollama model.

```go
type OllamaModel struct {
    Name       string // e.g., "llama3.1:8b"
    Size       string // e.g., "4.7 GB"
    Digest     string // SHA256 hash
    ModifiedAt string // ISO timestamp
}
```

Generated by [gomarkdoc](<https://github.com/princjef/gomarkdoc>)


---

<!-- FILE: api/index.md -->

# API Reference

Complete Go package documentation for Mooncake.

## Core Packages

### [Actions](actions.md)
Action handler registry and interfaces. All actions (shell, file, template, etc.) are registered here.

**Key Interfaces:**
- `Handler` - Base interface for all actions
- `Context` - Execution context passed to handlers
- `Result` - Action execution results

### [Config](config.md)
Configuration structures and validation. Defines the YAML schema for plans and steps.

**Key Types:**
- `Plan` - Top-level configuration
- `Step` - Individual execution steps
- Action structs (Shell, File, Template, etc.)

### [Executor](executor.md)
Execution engine that runs plans and steps. Handles dry-run mode, variable expansion, and result tracking.

**Key Types:**
- `Executor` - Main execution engine
- `ExecutionContext` - Runtime context
- Custom error types (RenderError, CommandError, etc.)

### [Events](events.md)
Event system for execution lifecycle. All events emitted during runs are defined here.

**Key Types:**
- `Event` - Base event structure
- `EventType` - Event type constants
- Event data types (StepStartedData, FileOperationData, etc.)

## System Packages

### [Facts](facts.md)
System information collection. Auto-detects OS, hardware, network, and software facts.

**Key Types:**
- `Facts` - Complete system information
- Platform-specific collectors (Linux, macOS, Windows)

### [Presets](presets.md)
Preset system for reusable workflows. Loads, validates, and expands preset definitions.

**Key Functions:**
- `LoadPreset()` - Load preset from file
- `ValidateParameters()` - Validate preset parameters
- `ExpandSteps()` - Expand preset into steps

### [Logger](logger.md)
Logging infrastructure with TUI and text output modes.

**Key Types:**
- `Logger` - Base logger interface
- `TUILogger` - Terminal UI logger
- `TextLogger` - Plain text logger

## Command Line

### [Commands](cmd.md)
CLI command implementations (run, plan, facts, etc.)

**Commands:**
- `run` - Execute a plan
- `plan` - Generate execution plan
- `facts` - Display system facts

---

## Package Organization

```
mooncake/
├── cmd/               # CLI commands
└── internal/
    ├── actions/       # Action handlers
    ├── config/        # Configuration
    ├── executor/      # Execution engine
    ├── events/        # Event system
    ├── facts/         # System facts
    ├── presets/       # Preset system
    ├── logger/        # Logging
    ├── template/      # Template engine
    ├── expression/    # Expression evaluator
    ├── pathutil/      # Path utilities
    └── utils/         # Shared utilities
```

## Usage Examples

### Implementing a Custom Action

```go
package myaction

import (
    "github.com/alehatsman/mooncake/internal/actions"
    "github.com/alehatsman/mooncake/internal/config"
)

type Handler struct{}

func init() {
    actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
    return actions.ActionMetadata{
        Name:           "myaction",
        Description:    "My custom action",
        Category:       actions.CategorySystem,
        SupportsDryRun: true,
        Version:        "1.0.0",
    }
}

func (h *Handler) Validate(step *config.Step) error {
    // Validate configuration
    return nil
}

func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
    // Implement action logic
    return nil, nil
}

func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
    // Show what would be done
    return nil
}
```

### Using the Executor Programmatically

```go
package main

import (
    "github.com/alehatsman/mooncake/internal/config"
    "github.com/alehatsman/mooncake/internal/executor"
    "github.com/alehatsman/mooncake/internal/logger"
)

func main() {
    // Load plan
    plan, _ := config.LoadPlan("config.yml")

    // Create executor
    log := logger.NewTextLogger()
    exec := executor.NewExecutor(log)

    // Execute
    result, _ := exec.Execute(plan, executor.ExecuteOptions{
        DryRun: false,
    })

    // Check results
    if !result.Success {
        log.Error("Execution failed")
    }
}
```

## External References

- [pkg.go.dev Documentation](https://pkg.go.dev/github.com/alehatsman/mooncake) - Official Go package docs
- [GitHub Repository](https://github.com/alehatsman/mooncake) - Source code
- [User Guide](../guide/core-concepts.md) - Getting started guide


---

<!-- FILE: api/logger.md -->

<!-- Code generated by gomarkdoc. DO NOT EDIT -->

# logger

```go
import "github.com/alehatsman/mooncake/internal/logger"
```

Package logger provides logging interfaces and implementations for mooncake.

## Index

- [Constants](<#constants>)
- [func Fatalf\(logger Logger, format string, v ...interface\{\}\)](<#Fatalf>)
- [func GetTerminalSize\(\) \(width, height int\)](<#GetTerminalSize>)
- [func IsTUISupported\(\) bool](<#IsTUISupported>)
- [func ParseLogLevel\(level string\) \(int, error\)](<#ParseLogLevel>)
- [type AnimationFrames](<#AnimationFrames>)
  - [func LoadEmbeddedFrames\(\) \(\*AnimationFrames, error\)](<#LoadEmbeddedFrames>)
  - [func LoadFramesFromFile\(path string\) \(\*AnimationFrames, error\)](<#LoadFramesFromFile>)
  - [func LoadFramesFromString\(content string\) \(\*AnimationFrames, error\)](<#LoadFramesFromString>)
  - [func \(a \*AnimationFrames\) Current\(\) \[\]string](<#AnimationFrames.Current>)
  - [func \(a \*AnimationFrames\) FrameCount\(\) int](<#AnimationFrames.FrameCount>)
  - [func \(a \*AnimationFrames\) Next\(\) \[\]string](<#AnimationFrames.Next>)
- [type BufferSnapshot](<#BufferSnapshot>)
- [type ConsoleLogger](<#ConsoleLogger>)
  - [func NewConsoleLogger\(logLevel int\) \*ConsoleLogger](<#NewConsoleLogger>)
  - [func \(l \*ConsoleLogger\) Codef\(format string, v ...interface\{\}\)](<#ConsoleLogger.Codef>)
  - [func \(l \*ConsoleLogger\) Complete\(stats ExecutionStats\)](<#ConsoleLogger.Complete>)
  - [func \(l \*ConsoleLogger\) Debugf\(format string, v ...interface\{\}\)](<#ConsoleLogger.Debugf>)
  - [func \(l \*ConsoleLogger\) Errorf\(format string, v ...interface\{\}\)](<#ConsoleLogger.Errorf>)
  - [func \(l \*ConsoleLogger\) Infof\(format string, v ...interface\{\}\)](<#ConsoleLogger.Infof>)
  - [func \(l \*ConsoleLogger\) LogStep\(info StepInfo\)](<#ConsoleLogger.LogStep>)
  - [func \(l \*ConsoleLogger\) Mooncake\(\)](<#ConsoleLogger.Mooncake>)
  - [func \(l \*ConsoleLogger\) SetLogLevel\(logLevel int\)](<#ConsoleLogger.SetLogLevel>)
  - [func \(l \*ConsoleLogger\) SetLogLevelStr\(logLevel string\) error](<#ConsoleLogger.SetLogLevelStr>)
  - [func \(l \*ConsoleLogger\) SetRedactor\(redactor Redactor\)](<#ConsoleLogger.SetRedactor>)
  - [func \(l \*ConsoleLogger\) Textf\(format string, v ...interface\{\}\)](<#ConsoleLogger.Textf>)
  - [func \(l \*ConsoleLogger\) WithPadLevel\(padLevel int\) Logger](<#ConsoleLogger.WithPadLevel>)
- [type ConsoleSubscriber](<#ConsoleSubscriber>)
  - [func NewConsoleSubscriber\(logLevel int, logFormat string\) \*ConsoleSubscriber](<#NewConsoleSubscriber>)
  - [func \(c \*ConsoleSubscriber\) Close\(\)](<#ConsoleSubscriber.Close>)
  - [func \(c \*ConsoleSubscriber\) OnEvent\(event events.Event\)](<#ConsoleSubscriber.OnEvent>)
  - [func \(c \*ConsoleSubscriber\) SetRedactor\(r interface\{ Redact\(string\) string \}\)](<#ConsoleSubscriber.SetRedactor>)
- [type ExecutionStats](<#ExecutionStats>)
- [type LogEntry](<#LogEntry>)
- [type Logger](<#Logger>)
  - [func NewLogger\(logLevel int\) Logger](<#NewLogger>)
- [type ProgressInfo](<#ProgressInfo>)
- [type Redactor](<#Redactor>)
- [type StepEntry](<#StepEntry>)
- [type StepInfo](<#StepInfo>)
- [type TUIBuffer](<#TUIBuffer>)
  - [func NewTUIBuffer\(historySize int\) \*TUIBuffer](<#NewTUIBuffer>)
  - [func \(b \*TUIBuffer\) AddDebug\(message string\)](<#TUIBuffer.AddDebug>)
  - [func \(b \*TUIBuffer\) AddError\(message string\)](<#TUIBuffer.AddError>)
  - [func \(b \*TUIBuffer\) AddStep\(entry StepEntry\)](<#TUIBuffer.AddStep>)
  - [func \(b \*TUIBuffer\) GetSnapshot\(\) BufferSnapshot](<#TUIBuffer.GetSnapshot>)
  - [func \(b \*TUIBuffer\) SetCompletion\(stats ExecutionStats\)](<#TUIBuffer.SetCompletion>)
  - [func \(b \*TUIBuffer\) SetCurrentStep\(name string, progress ProgressInfo\)](<#TUIBuffer.SetCurrentStep>)
- [type TUIDisplay](<#TUIDisplay>)
  - [func NewTUIDisplay\(animator \*AnimationFrames, buffer \*TUIBuffer, width, height int\) \*TUIDisplay](<#NewTUIDisplay>)
  - [func \(d \*TUIDisplay\) Render\(\) string](<#TUIDisplay.Render>)
- [type TUILogger](<#TUILogger>)
  - [func NewTUILogger\(logLevel int\) \(\*TUILogger, error\)](<#NewTUILogger>)
  - [func \(l \*TUILogger\) Codef\(format string, v ...interface\{\}\)](<#TUILogger.Codef>)
  - [func \(l \*TUILogger\) Complete\(stats ExecutionStats\)](<#TUILogger.Complete>)
  - [func \(l \*TUILogger\) Debugf\(format string, v ...interface\{\}\)](<#TUILogger.Debugf>)
  - [func \(l \*TUILogger\) Errorf\(format string, v ...interface\{\}\)](<#TUILogger.Errorf>)
  - [func \(l \*TUILogger\) Infof\(\_ string, \_ ...interface\{\}\)](<#TUILogger.Infof>)
  - [func \(l \*TUILogger\) LogStep\(info StepInfo\)](<#TUILogger.LogStep>)
  - [func \(l \*TUILogger\) Mooncake\(\)](<#TUILogger.Mooncake>)
  - [func \(l \*TUILogger\) SetLogLevel\(logLevel int\)](<#TUILogger.SetLogLevel>)
  - [func \(l \*TUILogger\) SetLogLevelStr\(logLevel string\) error](<#TUILogger.SetLogLevelStr>)
  - [func \(l \*TUILogger\) SetRedactor\(redactor Redactor\)](<#TUILogger.SetRedactor>)
  - [func \(l \*TUILogger\) Start\(\)](<#TUILogger.Start>)
  - [func \(l \*TUILogger\) Stop\(\)](<#TUILogger.Stop>)
  - [func \(l \*TUILogger\) Textf\(format string, v ...interface\{\}\)](<#TUILogger.Textf>)
  - [func \(l \*TUILogger\) WithPadLevel\(padLevel int\) Logger](<#TUILogger.WithPadLevel>)
- [type TUISubscriber](<#TUISubscriber>)
  - [func NewTUISubscriber\(logLevel int\) \(\*TUISubscriber, error\)](<#NewTUISubscriber>)
  - [func \(t \*TUISubscriber\) Close\(\)](<#TUISubscriber.Close>)
  - [func \(t \*TUISubscriber\) OnEvent\(event events.Event\)](<#TUISubscriber.OnEvent>)
  - [func \(t \*TUISubscriber\) SetRedactor\(r Redactor\)](<#TUISubscriber.SetRedactor>)
  - [func \(t \*TUISubscriber\) Start\(\)](<#TUISubscriber.Start>)
  - [func \(t \*TUISubscriber\) Stop\(\)](<#TUISubscriber.Stop>)
- [type TerminalInfo](<#TerminalInfo>)
  - [func DetectTerminal\(\) TerminalInfo](<#DetectTerminal>)
- [type TestLogger](<#TestLogger>)
  - [func NewTestLogger\(\) \*TestLogger](<#NewTestLogger>)
  - [func \(t \*TestLogger\) Clear\(\)](<#TestLogger.Clear>)
  - [func \(t \*TestLogger\) Codef\(format string, v ...interface\{\}\)](<#TestLogger.Codef>)
  - [func \(t \*TestLogger\) Complete\(stats ExecutionStats\)](<#TestLogger.Complete>)
  - [func \(t \*TestLogger\) Contains\(substr string\) bool](<#TestLogger.Contains>)
  - [func \(t \*TestLogger\) ContainsLevel\(level, substr string\) bool](<#TestLogger.ContainsLevel>)
  - [func \(t \*TestLogger\) Count\(\) int](<#TestLogger.Count>)
  - [func \(t \*TestLogger\) CountLevel\(level string\) int](<#TestLogger.CountLevel>)
  - [func \(t \*TestLogger\) Debugf\(format string, v ...interface\{\}\)](<#TestLogger.Debugf>)
  - [func \(t \*TestLogger\) Errorf\(format string, v ...interface\{\}\)](<#TestLogger.Errorf>)
  - [func \(t \*TestLogger\) GetLogs\(\) \[\]LogEntry](<#TestLogger.GetLogs>)
  - [func \(t \*TestLogger\) Infof\(format string, v ...interface\{\}\)](<#TestLogger.Infof>)
  - [func \(t \*TestLogger\) LogStep\(info StepInfo\)](<#TestLogger.LogStep>)
  - [func \(t \*TestLogger\) Mooncake\(\)](<#TestLogger.Mooncake>)
  - [func \(t \*TestLogger\) SetLogLevel\(logLevel int\)](<#TestLogger.SetLogLevel>)
  - [func \(t \*TestLogger\) SetLogLevelStr\(logLevel string\) error](<#TestLogger.SetLogLevelStr>)
  - [func \(t \*TestLogger\) SetRedactor\(redactor Redactor\)](<#TestLogger.SetRedactor>)
  - [func \(t \*TestLogger\) Textf\(format string, v ...interface\{\}\)](<#TestLogger.Textf>)
  - [func \(t \*TestLogger\) WithPadLevel\(padLevel int\) Logger](<#TestLogger.WithPadLevel>)


## Constants

<a name="DebugLevel"></a>

```go
const (
    // DebugLevel logs are typically voluminous, and are usually disabled in
    // production.
    DebugLevel = iota
    // InfoLevel is the default logging priority.
    InfoLevel
    // ErrorLevel logs are high-priority. If an application is running smoothly,
    // it shouldn't generate any error-logLevel logs.
    ErrorLevel
)
```

<a name="StatusRunning"></a>Step status constants used across all logger implementations

```go
const (
    StatusRunning = "running"
    StatusSuccess = "success"
    StatusError   = "error"
    StatusSkipped = "skipped"
)
```

<a name="Fatalf"></a>
## func [Fatalf](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/logger.go#L82>)

```go
func Fatalf(logger Logger, format string, v ...interface{})
```

Fatalf logs an error and exits the program.

<a name="GetTerminalSize"></a>
## func [GetTerminalSize](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_detector.go#L74>)

```go
func GetTerminalSize() (width, height int)
```

GetTerminalSize returns the current terminal size. Returns default 80x24 if detection fails.

<a name="IsTUISupported"></a>
## func [IsTUISupported](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_detector.go#L57>)

```go
func IsTUISupported() bool
```

IsTUISupported checks if the terminal supports TUI mode. Returns true if terminal is detected, supports ANSI codes, and meets minimum size requirements.

<a name="ParseLogLevel"></a>
## func [ParseLogLevel](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/logger.go#L31>)

```go
func ParseLogLevel(level string) (int, error)
```

ParseLogLevel converts a log level string to its integer constant. Valid values are "debug", "info", and "error" \(case\-insensitive\). Returns an error if the level string is not recognized.

<a name="AnimationFrames"></a>
## type [AnimationFrames](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_animator.go#L57-L61>)

AnimationFrames manages animation frames for the mooncake character.

```go
type AnimationFrames struct {
    // contains filtered or unexported fields
}
```

<a name="LoadEmbeddedFrames"></a>
### func [LoadEmbeddedFrames](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_animator.go#L64>)

```go
func LoadEmbeddedFrames() (*AnimationFrames, error)
```

LoadEmbeddedFrames loads animation frames from the embedded content.

<a name="LoadFramesFromFile"></a>
### func [LoadFramesFromFile](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_animator.go#L70>)

```go
func LoadFramesFromFile(path string) (*AnimationFrames, error)
```

LoadFramesFromFile loads animation frames from a file. Frames are expected to be 3 lines each, separated by blank lines.

<a name="LoadFramesFromString"></a>
### func [LoadFramesFromString](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_animator.go#L82>)

```go
func LoadFramesFromString(content string) (*AnimationFrames, error)
```

LoadFramesFromString loads animation frames from a string. Frames are expected to be 3 lines each, separated by blank lines.

<a name="AnimationFrames.Current"></a>
### func \(\*AnimationFrames\) [Current](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_animator.go#L127>)

```go
func (a *AnimationFrames) Current() []string
```

Current returns the current frame without advancing

<a name="AnimationFrames.FrameCount"></a>
### func \(\*AnimationFrames\) [FrameCount](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_animator.go#L135>)

```go
func (a *AnimationFrames) FrameCount() int
```

FrameCount returns the total number of frames

<a name="AnimationFrames.Next"></a>
### func \(\*AnimationFrames\) [Next](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_animator.go#L118>)

```go
func (a *AnimationFrames) Next() []string
```

Next advances to the next frame and returns it

<a name="BufferSnapshot"></a>
## type [BufferSnapshot](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_buffer.go#L23-L30>)

BufferSnapshot is an atomic snapshot of the buffer state for rendering.

```go
type BufferSnapshot struct {
    StepHistory   []StepEntry
    CurrentStep   string
    Progress      ProgressInfo
    DebugMessages []string
    ErrorMessages []string
    Completion    *ExecutionStats
}
```

<a name="ConsoleLogger"></a>
## type [ConsoleLogger](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/console_logger.go#L13-L18>)

ConsoleLogger implements Logger interface with colored console output.

```go
type ConsoleLogger struct {
    // contains filtered or unexported fields
}
```

<a name="NewConsoleLogger"></a>
### func [NewConsoleLogger](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/console_logger.go#L30>)

```go
func NewConsoleLogger(logLevel int) *ConsoleLogger
```

NewConsoleLogger creates a ConsoleLogger directly \(for type\-specific needs\).

<a name="ConsoleLogger.Codef"></a>
### func \(\*ConsoleLogger\) [Codef](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/console_logger.go#L116>)

```go
func (l *ConsoleLogger) Codef(format string, v ...interface{})
```

Codef logs a code snippet message.

<a name="ConsoleLogger.Complete"></a>
### func \(\*ConsoleLogger\) [Complete](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/console_logger.go#L178>)

```go
func (l *ConsoleLogger) Complete(stats ExecutionStats)
```

Complete logs the execution completion summary with statistics.

<a name="ConsoleLogger.Debugf"></a>
### func \(\*ConsoleLogger\) [Debugf](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/console_logger.go#L87>)

```go
func (l *ConsoleLogger) Debugf(format string, v ...interface{})
```

Debugf logs a debug message.

<a name="ConsoleLogger.Errorf"></a>
### func \(\*ConsoleLogger\) [Errorf](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/console_logger.go#L77>)

```go
func (l *ConsoleLogger) Errorf(format string, v ...interface{})
```

Errorf logs an error message.

<a name="ConsoleLogger.Infof"></a>
### func \(\*ConsoleLogger\) [Infof](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/console_logger.go#L67>)

```go
func (l *ConsoleLogger) Infof(format string, v ...interface{})
```

Infof logs an informational message.

<a name="ConsoleLogger.LogStep"></a>
### func \(\*ConsoleLogger\) [LogStep](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/console_logger.go#L150>)

```go
func (l *ConsoleLogger) LogStep(info StepInfo)
```

LogStep logs a step execution with status.

<a name="ConsoleLogger.Mooncake"></a>
### func \(\*ConsoleLogger\) [Mooncake](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/console_logger.go#L125>)

```go
func (l *ConsoleLogger) Mooncake()
```

Mooncake displays the mooncake banner.

<a name="ConsoleLogger.SetLogLevel"></a>
### func \(\*ConsoleLogger\) [SetLogLevel](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/console_logger.go#L39>)

```go
func (l *ConsoleLogger) SetLogLevel(logLevel int)
```

SetLogLevel sets the logging level for the logger.

<a name="ConsoleLogger.SetLogLevelStr"></a>
### func \(\*ConsoleLogger\) [SetLogLevelStr](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/console_logger.go#L44>)

```go
func (l *ConsoleLogger) SetLogLevelStr(logLevel string) error
```

SetLogLevelStr sets the logging level from a string value.

<a name="ConsoleLogger.SetRedactor"></a>
### func \(\*ConsoleLogger\) [SetRedactor](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/console_logger.go#L54>)

```go
func (l *ConsoleLogger) SetRedactor(redactor Redactor)
```

SetRedactor sets the redactor for automatic sensitive data redaction.

<a name="ConsoleLogger.Textf"></a>
### func \(\*ConsoleLogger\) [Textf](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/console_logger.go#L110>)

```go
func (l *ConsoleLogger) Textf(format string, v ...interface{})
```

Textf logs a plain text message.

<a name="ConsoleLogger.WithPadLevel"></a>
### func \(\*ConsoleLogger\) [WithPadLevel](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/console_logger.go#L140>)

```go
func (l *ConsoleLogger) WithPadLevel(padLevel int) Logger
```

WithPadLevel creates a new logger with the specified padding level.

<a name="ConsoleSubscriber"></a>
## type [ConsoleSubscriber](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/console_subscriber.go#L15-L22>)

ConsoleSubscriber implements event\-based console logging

```go
type ConsoleSubscriber struct {
    // contains filtered or unexported fields
}
```

<a name="NewConsoleSubscriber"></a>
### func [NewConsoleSubscriber](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/console_subscriber.go#L25>)

```go
func NewConsoleSubscriber(logLevel int, logFormat string) *ConsoleSubscriber
```

NewConsoleSubscriber creates a new console subscriber

<a name="ConsoleSubscriber.Close"></a>
### func \(\*ConsoleSubscriber\) [Close](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/console_subscriber.go#L53>)

```go
func (c *ConsoleSubscriber) Close()
```

Close implements the Subscriber interface

<a name="ConsoleSubscriber.OnEvent"></a>
### func \(\*ConsoleSubscriber\) [OnEvent](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/console_subscriber.go#L40>)

```go
func (c *ConsoleSubscriber) OnEvent(event events.Event)
```

OnEvent handles incoming events

<a name="ConsoleSubscriber.SetRedactor"></a>
### func \(\*ConsoleSubscriber\) [SetRedactor](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/console_subscriber.go#L33>)

```go
func (c *ConsoleSubscriber) SetRedactor(r interface{ Redact(string) string })
```

SetRedactor sets the redactor for sensitive data

<a name="ExecutionStats"></a>
## type [ExecutionStats](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/logger.go#L53-L58>)

ExecutionStats contains execution statistics.

```go
type ExecutionStats struct {
    Duration time.Duration
    Executed int
    Skipped  int
    Failed   int
}
```

<a name="LogEntry"></a>
## type [LogEntry](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/test_logger.go#L10-L13>)

LogEntry represents a single log entry.

```go
type LogEntry struct {
    Level   string
    Message string
}
```

<a name="Logger"></a>
## type [Logger](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/logger.go#L66-L79>)

Logger interface defines the logging contract.

```go
type Logger interface {
    Infof(format string, v ...interface{})
    Debugf(format string, v ...interface{})
    Errorf(format string, v ...interface{})
    Codef(format string, v ...interface{})
    Textf(format string, v ...interface{})
    Mooncake()
    SetLogLevel(logLevel int)
    SetLogLevelStr(logLevel string) error
    WithPadLevel(padLevel int) Logger
    LogStep(info StepInfo)
    Complete(stats ExecutionStats)
    SetRedactor(redactor Redactor)
}
```

<a name="NewLogger"></a>
### func [NewLogger](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/console_logger.go#L21>)

```go
func NewLogger(logLevel int) Logger
```

NewLogger creates a new ConsoleLogger with the specified log level.

<a name="ProgressInfo"></a>
## type [ProgressInfo](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_buffer.go#L17-L20>)

ProgressInfo tracks overall execution progress.

```go
type ProgressInfo struct {
    Current int
    Total   int
}
```

<a name="Redactor"></a>
## type [Redactor](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/logger.go#L61-L63>)

Redactor interface for redacting sensitive data in logs.

```go
type Redactor interface {
    Redact(string) string
}
```

<a name="StepEntry"></a>
## type [StepEntry](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_buffer.go#L9-L14>)

StepEntry represents a single step in the execution history.

```go
type StepEntry struct {
    Name      string
    Status    string // "success", "error", "skipped", "running"
    Level     int    // Nesting level for indentation
    Timestamp time.Time
}
```

<a name="StepInfo"></a>
## type [StepInfo](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/logger.go#L45-L50>)

StepInfo contains structured information about a step execution.

```go
type StepInfo struct {
    Name       string
    Level      int    // Nesting level for indentation
    GlobalStep int    // Cumulative step number
    Status     string // "running", "success", "error", "skipped"
}
```

<a name="TUIBuffer"></a>
## type [TUIBuffer](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_buffer.go#L33-L49>)

TUIBuffer manages step history and message buffering.

```go
type TUIBuffer struct {
    // contains filtered or unexported fields
}
```

<a name="NewTUIBuffer"></a>
### func [NewTUIBuffer](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_buffer.go#L52>)

```go
func NewTUIBuffer(historySize int) *TUIBuffer
```

NewTUIBuffer creates a new TUI buffer with specified history size.

<a name="TUIBuffer.AddDebug"></a>
### func \(\*TUIBuffer\) [AddDebug](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_buffer.go#L98>)

```go
func (b *TUIBuffer) AddDebug(message string)
```

AddDebug adds a debug message to the buffer.

<a name="TUIBuffer.AddError"></a>
### func \(\*TUIBuffer\) [AddError](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_buffer.go#L110>)

```go
func (b *TUIBuffer) AddError(message string)
```

AddError adds an error message to the buffer.

<a name="TUIBuffer.AddStep"></a>
### func \(\*TUIBuffer\) [AddStep](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_buffer.go#L65>)

```go
func (b *TUIBuffer) AddStep(entry StepEntry)
```

AddStep adds a step to the history \(circular buffer\).

<a name="TUIBuffer.GetSnapshot"></a>
### func \(\*TUIBuffer\) [GetSnapshot](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_buffer.go#L122>)

```go
func (b *TUIBuffer) GetSnapshot() BufferSnapshot
```

GetSnapshot returns an atomic snapshot of the buffer state.

<a name="TUIBuffer.SetCompletion"></a>
### func \(\*TUIBuffer\) [SetCompletion](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_buffer.go#L90>)

```go
func (b *TUIBuffer) SetCompletion(stats ExecutionStats)
```

SetCompletion sets execution completion statistics.

<a name="TUIBuffer.SetCurrentStep"></a>
### func \(\*TUIBuffer\) [SetCurrentStep](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_buffer.go#L81>)

```go
func (b *TUIBuffer) SetCurrentStep(name string, progress ProgressInfo)
```

SetCurrentStep sets the currently executing step.

<a name="TUIDisplay"></a>
## type [TUIDisplay](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_display.go#L12-L17>)

TUIDisplay handles screen rendering for the animated TUI.

```go
type TUIDisplay struct {
    // contains filtered or unexported fields
}
```

<a name="NewTUIDisplay"></a>
### func [NewTUIDisplay](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_display.go#L20>)

```go
func NewTUIDisplay(animator *AnimationFrames, buffer *TUIBuffer, width, height int) *TUIDisplay
```

NewTUIDisplay creates a new TUI display renderer.

<a name="TUIDisplay.Render"></a>
### func \(\*TUIDisplay\) [Render](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_display.go#L30>)

```go
func (d *TUIDisplay) Render() string
```

Render generates the complete screen output.

<a name="TUILogger"></a>
## type [TUILogger](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_logger.go#L17-L30>)

TUILogger implements Logger interface with animated TUI display.

```go
type TUILogger struct {
    // contains filtered or unexported fields
}
```

<a name="NewTUILogger"></a>
### func [NewTUILogger](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_logger.go#L33>)

```go
func NewTUILogger(logLevel int) (*TUILogger, error)
```

NewTUILogger creates a new TUI logger.

<a name="TUILogger.Codef"></a>
### func \(\*TUILogger\) [Codef](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_logger.go#L183>)

```go
func (l *TUILogger) Codef(format string, v ...interface{})
```

Codef logs formatted code.

<a name="TUILogger.Complete"></a>
### func \(\*TUILogger\) [Complete](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_logger.go#L272>)

```go
func (l *TUILogger) Complete(stats ExecutionStats)
```

Complete logs the execution completion summary with statistics.

<a name="TUILogger.Debugf"></a>
### func \(\*TUILogger\) [Debugf](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_logger.go#L148>)

```go
func (l *TUILogger) Debugf(format string, v ...interface{})
```

Debugf logs a debug message.

<a name="TUILogger.Errorf"></a>
### func \(\*TUILogger\) [Errorf](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_logger.go#L162>)

```go
func (l *TUILogger) Errorf(format string, v ...interface{})
```

Errorf logs an error message.

<a name="TUILogger.Infof"></a>
### func \(\*TUILogger\) [Infof](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_logger.go#L141>)

```go
func (l *TUILogger) Infof(_ string, _ ...interface{})
```

Infof logs an info message \(ignored in TUI mode \- use LogStep for steps\).

<a name="TUILogger.LogStep"></a>
### func \(\*TUILogger\) [LogStep](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_logger.go#L93>)

```go
func (l *TUILogger) LogStep(info StepInfo)
```

LogStep handles structured step logging.

<a name="TUILogger.Mooncake"></a>
### func \(\*TUILogger\) [Mooncake](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_logger.go#L214>)

```go
func (l *TUILogger) Mooncake()
```

Mooncake displays the mooncake banner \(initializes display\).

<a name="TUILogger.SetLogLevel"></a>
### func \(\*TUILogger\) [SetLogLevel](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_logger.go#L220>)

```go
func (l *TUILogger) SetLogLevel(logLevel int)
```

SetLogLevel sets the log level.

<a name="TUILogger.SetLogLevelStr"></a>
### func \(\*TUILogger\) [SetLogLevelStr](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_logger.go#L228>)

```go
func (l *TUILogger) SetLogLevelStr(logLevel string) error
```

SetLogLevelStr sets the log level from a string.

<a name="TUILogger.SetRedactor"></a>
### func \(\*TUILogger\) [SetRedactor](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_logger.go#L239>)

```go
func (l *TUILogger) SetRedactor(redactor Redactor)
```

SetRedactor sets the redactor for automatic sensitive data redaction.

<a name="TUILogger.Start"></a>
### func \(\*TUILogger\) [Start](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_logger.go#L60>)

```go
func (l *TUILogger) Start()
```

Start begins the animation and rendering loop.

<a name="TUILogger.Stop"></a>
### func \(\*TUILogger\) [Stop](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_logger.go#L77>)

```go
func (l *TUILogger) Stop()
```

Stop stops the animation and shows final render.

<a name="TUILogger.Textf"></a>
### func \(\*TUILogger\) [Textf](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_logger.go#L201>)

```go
func (l *TUILogger) Textf(format string, v ...interface{})
```

Textf logs plain text.

<a name="TUILogger.WithPadLevel"></a>
### func \(\*TUILogger\) [WithPadLevel](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_logger.go#L254>)

```go
func (l *TUILogger) WithPadLevel(padLevel int) Logger
```

WithPadLevel creates a new logger with the specified padding level.

<a name="TUISubscriber"></a>
## type [TUISubscriber](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_subscriber.go#L12-L25>)

TUISubscriber implements event\-based TUI display.

```go
type TUISubscriber struct {
    // contains filtered or unexported fields
}
```

<a name="NewTUISubscriber"></a>
### func [NewTUISubscriber](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_subscriber.go#L28>)

```go
func NewTUISubscriber(logLevel int) (*TUISubscriber, error)
```

NewTUISubscriber creates a new TUI subscriber.

<a name="TUISubscriber.Close"></a>
### func \(\*TUISubscriber\) [Close](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_subscriber.go#L146>)

```go
func (t *TUISubscriber) Close()
```

Close implements the Subscriber interface.

<a name="TUISubscriber.OnEvent"></a>
### func \(\*TUISubscriber\) [OnEvent](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_subscriber.go#L113>)

```go
func (t *TUISubscriber) OnEvent(event events.Event)
```

OnEvent handles incoming events.

<a name="TUISubscriber.SetRedactor"></a>
### func \(\*TUISubscriber\) [SetRedactor](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_subscriber.go#L106>)

```go
func (t *TUISubscriber) SetRedactor(r Redactor)
```

SetRedactor sets the redactor for sensitive data.

<a name="TUISubscriber.Start"></a>
### func \(\*TUISubscriber\) [Start](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_subscriber.go#L55>)

```go
func (t *TUISubscriber) Start()
```

Start begins the animation and rendering loop.

<a name="TUISubscriber.Stop"></a>
### func \(\*TUISubscriber\) [Stop](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_subscriber.go#L80>)

```go
func (t *TUISubscriber) Stop()
```

Stop stops the animation and shows final render.

<a name="TerminalInfo"></a>
## type [TerminalInfo](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_detector.go#L10-L15>)

TerminalInfo contains information about terminal capabilities.

```go
type TerminalInfo struct {
    IsTerminal   bool
    SupportsANSI bool
    Width        int
    Height       int
}
```

<a name="DetectTerminal"></a>
### func [DetectTerminal](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/tui_detector.go#L18>)

```go
func DetectTerminal() TerminalInfo
```

DetectTerminal detects terminal capabilities and returns terminal information.

<a name="TestLogger"></a>
## type [TestLogger](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/test_logger.go#L16-L22>)

TestLogger implements Logger interface and captures log output for testing.

```go
type TestLogger struct {
    Logs []LogEntry
    // contains filtered or unexported fields
}
```

<a name="NewTestLogger"></a>
### func [NewTestLogger](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/test_logger.go#L25>)

```go
func NewTestLogger() *TestLogger
```

NewTestLogger creates a new TestLogger for use in tests.

<a name="TestLogger.Clear"></a>
### func \(\*TestLogger\) [Clear](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/test_logger.go#L201>)

```go
func (t *TestLogger) Clear()
```

Clear removes all log entries.

<a name="TestLogger.Codef"></a>
### func \(\*TestLogger\) [Codef](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/test_logger.go#L67>)

```go
func (t *TestLogger) Codef(format string, v ...interface{})
```

Codef logs a code snippet message.

<a name="TestLogger.Complete"></a>
### func \(\*TestLogger\) [Complete](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/test_logger.go#L146>)

```go
func (t *TestLogger) Complete(stats ExecutionStats)
```

Complete logs the execution completion summary with statistics.

<a name="TestLogger.Contains"></a>
### func \(\*TestLogger\) [Contains](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/test_logger.go#L157>)

```go
func (t *TestLogger) Contains(substr string) bool
```

Contains checks if any log message contains the substring.

<a name="TestLogger.ContainsLevel"></a>
### func \(\*TestLogger\) [ContainsLevel](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/test_logger.go#L169>)

```go
func (t *TestLogger) ContainsLevel(level, substr string) bool
```

ContainsLevel checks if any log at the specified level contains the substring.

<a name="TestLogger.Count"></a>
### func \(\*TestLogger\) [Count](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/test_logger.go#L181>)

```go
func (t *TestLogger) Count() int
```

Count returns the number of log entries.

<a name="TestLogger.CountLevel"></a>
### func \(\*TestLogger\) [CountLevel](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/test_logger.go#L188>)

```go
func (t *TestLogger) CountLevel(level string) int
```

CountLevel returns the number of log entries at the specified level.

<a name="TestLogger.Debugf"></a>
### func \(\*TestLogger\) [Debugf](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/test_logger.go#L45>)

```go
func (t *TestLogger) Debugf(format string, v ...interface{})
```

Debugf logs a debug message.

<a name="TestLogger.Errorf"></a>
### func \(\*TestLogger\) [Errorf](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/test_logger.go#L56>)

```go
func (t *TestLogger) Errorf(format string, v ...interface{})
```

Errorf logs an error message.

<a name="TestLogger.GetLogs"></a>
### func \(\*TestLogger\) [GetLogs](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/test_logger.go#L208>)

```go
func (t *TestLogger) GetLogs() []LogEntry
```

GetLogs returns a copy of all log entries.

<a name="TestLogger.Infof"></a>
### func \(\*TestLogger\) [Infof](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/test_logger.go#L34>)

```go
func (t *TestLogger) Infof(format string, v ...interface{})
```

Infof logs an informational message.

<a name="TestLogger.LogStep"></a>
### func \(\*TestLogger\) [LogStep](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/test_logger.go#L138>)

```go
func (t *TestLogger) LogStep(info StepInfo)
```

LogStep logs a step execution with status.

<a name="TestLogger.Mooncake"></a>
### func \(\*TestLogger\) [Mooncake](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/test_logger.go#L84>)

```go
func (t *TestLogger) Mooncake()
```

Mooncake displays the mooncake banner.

<a name="TestLogger.SetLogLevel"></a>
### func \(\*TestLogger\) [SetLogLevel](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/test_logger.go#L91>)

```go
func (t *TestLogger) SetLogLevel(logLevel int)
```

SetLogLevel sets the logging level for the logger.

<a name="TestLogger.SetLogLevelStr"></a>
### func \(\*TestLogger\) [SetLogLevelStr](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/test_logger.go#L98>)

```go
func (t *TestLogger) SetLogLevelStr(logLevel string) error
```

SetLogLevelStr sets the logging level from a string value.

<a name="TestLogger.SetRedactor"></a>
### func \(\*TestLogger\) [SetRedactor](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/test_logger.go#L113>)

```go
func (t *TestLogger) SetRedactor(redactor Redactor)
```

SetRedactor sets the redactor for automatic sensitive data redaction.

<a name="TestLogger.Textf"></a>
### func \(\*TestLogger\) [Textf](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/test_logger.go#L77>)

```go
func (t *TestLogger) Textf(format string, v ...interface{})
```

Textf logs a plain text message.

<a name="TestLogger.WithPadLevel"></a>
### func \(\*TestLogger\) [WithPadLevel](<https://github.com/alehatsman/mooncake/blob/master/internal/logger/test_logger.go#L128>)

```go
func (t *TestLogger) WithPadLevel(padLevel int) Logger
```

WithPadLevel creates a new logger with the specified padding level.

Generated by [gomarkdoc](<https://github.com/princjef/gomarkdoc>)


---

<!-- FILE: api/presets.md -->

<!-- Code generated by gomarkdoc. DO NOT EDIT -->

# presets

```go
import "github.com/alehatsman/mooncake/internal/presets"
```

Package presets provides preset loading and expansion functionality.

## Index

- [func ExpandPreset\(invocation \*config.PresetInvocation\) \(\[\]config.Step, map\[string\]interface\{\}, string, error\)](<#ExpandPreset>)
- [func LoadPreset\(name string\) \(\*config.PresetDefinition, error\)](<#LoadPreset>)
- [func PresetSearchPaths\(\) \[\]string](<#PresetSearchPaths>)
- [func ValidateParameters\(definition \*config.PresetDefinition, userParams map\[string\]interface\{\}\) \(map\[string\]interface\{\}, error\)](<#ValidateParameters>)


<a name="ExpandPreset"></a>
## func [ExpandPreset](<https://github.com/alehatsman/mooncake/blob/master/internal/presets/expander.go#L13>)

```go
func ExpandPreset(invocation *config.PresetInvocation) ([]config.Step, map[string]interface{}, string, error)
```

ExpandPreset expands a preset invocation into its constituent steps. It loads the preset definition, validates parameters, and returns the expanded steps with the 'parameters' namespace injected into the execution context, along with the preset's base directory for relative path resolution.

<a name="LoadPreset"></a>
## func [LoadPreset](<https://github.com/alehatsman/mooncake/blob/master/internal/presets/loader.go#L45>)

```go
func LoadPreset(name string) (*config.PresetDefinition, error)
```

LoadPreset loads a preset definition by name. It searches for presets in two formats: 1. Flat: \<name\>.yml \(e.g., presets/ollama.yml\) 2. Directory: \<name\>/preset.yml \(e.g., presets/ollama/preset.yml\) Directory structure takes precedence if both exist. Returns the loaded PresetDefinition or an error if not found or invalid.

<a name="PresetSearchPaths"></a>
## func [PresetSearchPaths](<https://github.com/alehatsman/mooncake/blob/master/internal/presets/loader.go#L20>)

```go
func PresetSearchPaths() []string
```

PresetSearchPaths returns the ordered list of directories to search for presets. Priority order \(highest to lowest\): 1. ./presets/ \(playbook directory\) 2. \~/.mooncake/presets/ \(user presets\) 3. /usr/local/share/mooncake/presets/ \(local installation\) 4. /usr/share/mooncake/presets/ \(system installation\)

<a name="ValidateParameters"></a>
## func [ValidateParameters](<https://github.com/alehatsman/mooncake/blob/master/internal/presets/validator.go#L20>)

```go
func ValidateParameters(definition *config.PresetDefinition, userParams map[string]interface{}) (map[string]interface{}, error)
```

ValidateParameters validates user\-provided parameters against preset parameter definitions. It checks required parameters, validates types, checks enum constraints, and applies defaults. Returns a validated parameter map ready for use in template expansion.

Generated by [gomarkdoc](<https://github.com/princjef/gomarkdoc>)


---

<!-- FILE: architecture-decisions/000-planner-execution-model.md -->

# ADR-003: Planner and Execution Model

**Status**: Accepted
**Date**: 2026-02-05
**Deciders**: Engineering Team
**Technical Story**: Two-phase architecture for deterministic configuration execution

## Context

Early versions of mooncake executed configuration files directly, expanding directives (includes, loops, variables) at runtime as they were encountered. This approach had several problems:

1. **Non-Determinism**: Step order could vary based on runtime conditions
2. **Limited Introspection**: No way to see what would execute before running
3. **Error Discovery**: Syntax errors only discovered when reached
4. **No Dry-Run Support**: Couldn't preview execution without side effects
5. **Circular Dependencies**: Include cycles only detected at runtime
6. **Poor Observability**: No visibility into total steps before execution

Example problematic scenario:
```yaml
- vars:
    items: [a, b, c]

- shell: echo "{{ item }}"
  with_items: "{{ items }}"  # How many steps will this create? Unknown until runtime!

- include: other.yml  # What does this contain? Unknown until now!
  when: "{{ some_condition }}"  # Might not even be evaluated
```

The fundamental issue: **configuration expansion mixed with execution**, making it impossible to answer "What will this do?" before doing it.

## Decision

We adopted a **two-phase architecture** separating configuration expansion (planning) from execution:

**Benefits:**
- **Deterministic**: Same config always produces the same plan
- **Inspectable**: Use `mooncake plan` to see what will execute
- **Traceable**: Every step tracks its origin with include chain
- **Debuggable**: Understand loop expansions and includes before execution

### Phase 1: Planning (Compile-Time)
**Planner** expands configuration into a deterministic execution plan
- Resolves includes recursively
- Expands loops (with_items, with_filetree)
- Processes compile-time variables (vars, include_vars)
- Renders path templates
- Validates configuration structure
- Detects cycles
- Generates step IDs and origin metadata

**Output**: A `Plan` containing a flat list of executable steps

### Phase 2: Execution (Runtime)
**Executor** runs the pre-compiled plan step by step
- Evaluates runtime conditions (when, unless, creates)
- Executes actions through handlers
- Manages variables and results
- Emits events for observability
- Handles errors and failures

**Input**: A `Plan` from phase 1

### Key Architectural Principles

#### 1. Compile-Time vs Runtime Directives

**Compile-Time** (processed by planner):
- `include`: File inclusion
- `with_items`: Loop expansion
- `with_filetree`: Directory tree iteration
- `vars`: Variable setting (when condition evaluable at plan time)
- `include_vars`: Variable file loading (when condition evaluable at plan time)

**Runtime** (processed by executor):
- `when`: Conditional execution
- `unless`: Idempotency check (shell/command only)
- `creates`: Idempotency check (shell/command only)
- `changed_when`: Result override
- `failed_when`: Failure override
- `register`: Result capture

#### 2. Path Resolution Strategy

All relative paths resolved **at plan time** based on the file containing them:

```yaml
# File: /home/user/playbook/main.yml
- include: tasks/setup.yml  # Resolved to /home/user/playbook/tasks/setup.yml

# File: /home/user/playbook/tasks/setup.yml
- template:
    src: templates/config.j2   # Resolved to /home/user/playbook/tasks/templates/config.j2
    dest: /etc/app/config
```

**Rules**:
1. Relative paths joined with `CurrentDir` (directory of containing file)
2. Resolution happens during planning, before execution
3. Absolute paths used as-is
4. Include directives update `CurrentDir` for nested files
5. Loop context (with_filetree) doesn't change `CurrentDir`

#### 3. Variable Handling

Variables split into two categories:

**Plan-Time Variables** (available during expansion):
- Global vars from config (`vars:` at root level)
- CLI-provided vars (`--vars-file`)
- System facts (OS, architecture, etc.)
- Compile-time vars/include_vars (when condition evaluable)

**Runtime-Only Variables**:

- `register` results
- Loop variables (`item`, `index`, `first`, `last`)
- Vars/include_vars with runtime-dependent when conditions

**Why the split?**
- Plan-time: Needed for template expansion during planning
- Runtime-only: Not known until execution, stored in plan for later use

#### 4. Origin Tracking

Every step in the plan tracks its origin:

```go
type Origin struct {
    FilePath     string   // File containing this step
    Line         int      // Line number in file
    Column       int      // Column number in file
    IncludeChain []string // Chain of includes leading here
}
```

**Benefits**:

- Error messages show exact source location
- Debuggability: Can trace step to source
- Observability: Events include origin metadata
- Relative paths resolve correctly

#### 5. Loop Expansion

Loops expanded **during planning** into discrete steps:

**Input** (1 step):
```yaml
- shell: echo "{{ item }}"
  with_items: [a, b, c]
```

**Plan Output** (3 steps):
```yaml
- shell: echo "a"
  loop_context: {type: with_items, item: a, index: 0, first: true, last: false}
- shell: echo "b"
  loop_context: {type: with_items, item: b, index: 1, first: false, last: false}
- shell: echo "c"
  loop_context: {type: with_items, item: c, index: 2, first: false, last: true}
```

**Loop Variables Restored at Runtime**:
Executor uses `loop_context` to restore `item`, `index`, `first`, `last` into execution context before evaluating `when` conditions.

#### 6. Include Expansion

Includes expanded **recursively** during planning:

**Input**:
```yaml
# main.yml
- include: tasks/setup.yml

# tasks/setup.yml
- shell: echo "setup"
- include: common/base.yml

# common/base.yml
- shell: echo "base"
```

**Plan Output** (flat list):
```yaml
- shell: echo "setup"
  origin: {file: tasks/setup.yml, line: 1, chain: [main.yml:1]}
- shell: echo "base"
  origin: {file: common/base.yml, line: 1, chain: [main.yml:1, tasks/setup.yml:2]}
```

**Cycle Detection**:
Planner maintains `seenFiles` map and `includeStack` to detect cycles:

```go
if p.seenFiles[absIncludePath] {
    return fmt.Errorf("include cycle detected: %s\nChain: %s",
        absIncludePath, p.formatIncludeChain())
}
```

## Execution Flow

### Complete Flow Diagram

```
┌─────────────────────────────────────────────────────────────┐
│ User Invokes: mooncake run playbook.yml --vars vars.yml    │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│ Phase 1: PLANNING (Compile-Time)                            │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  1. Load Config File                                         │
│     └─> Read playbook.yml                                    │
│     └─> Validate JSON schema                                 │
│                                                              │
│  2. Initialize Variables                                     │
│     └─> Load vars from --vars-file                           │
│     └─> Collect system facts                                 │
│     └─> Merge into variable context                          │
│                                                              │
│  3. Expand Configuration (Recursive)                         │
│     │                                                         │
│     ├─> include: path                                        │
│     │   ├─> Render path template with vars                   │
│     │   ├─> Resolve to absolute path                         │
│     │   ├─> Check for cycles (seenFiles map)                 │
│     │   ├─> Push to includeStack                             │
│     │   ├─> Recursively expand included file                 │
│     │   └─> Pop from includeStack                            │
│     │                                                         │
│     ├─> with_items: expr                                     │
│     │   ├─> Evaluate expression with vars                    │
│     │   ├─> For each item:                                   │
│     │   │   ├─> Create loop context (item, index, first, last)│
│     │   │   ├─> Clone step                                   │
│     │   │   └─> Render templates with loop vars              │
│     │   └─> Append N steps to plan                           │
│     │                                                         │
│     ├─> with_filetree: path                                  │
│     │   ├─> Walk directory tree                              │
│     │   ├─> Sort entries (determinism)                       │
│     │   ├─> For each file/dir:                               │
│     │   │   ├─> Create loop context with file metadata       │
│     │   │   ├─> Clone step                                   │
│     │   │   └─> Render templates with file vars              │
│     │   └─> Append N steps to plan                           │
│     │                                                         │
│     ├─> vars: {...}                                          │
│     │   ├─> Evaluate when condition (if present)             │
│     │   ├─> If false: skip (don't set vars)                  │
│     │   ├─> Render var values with current context           │
│     │   └─> Merge into variable context                      │
│     │                                                         │
│     ├─> include_vars: path                                   │
│     │   ├─> Evaluate when condition (if present)             │
│     │   ├─> If false: skip (don't load vars)                 │
│     │   ├─> Render path with current context                 │
│     │   ├─> Load YAML file                                   │
│     │   └─> Merge into variable context                      │
│     │                                                         │
│     └─> Regular Action (shell, file, etc)                    │
│         ├─> Render step name with vars                       │
│         ├─> Render action fields with vars                   │
│         ├─> Resolve relative paths (src, dest)               │
│         ├─> Generate step ID (step-0001, step-0002, ...)     │
│         ├─> Build origin metadata (file, line, chain)        │
│         ├─> Check tag filtering (skipped flag)               │
│         └─> Append to plan                                   │
│                                                              │
│  4. Plan Complete                                            │
│     └─> Flat list of executable steps with metadata          │
│                                                              │
└──────────────────────┬───────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│ Phase 2: EXECUTION (Runtime)                                 │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  1. Initialize Execution Context                             │
│     ├─> Variables: copy from plan.InitialVars                │
│     ├─> CurrentDir: directory of root config file            │
│     ├─> Logger, Template, Evaluator, etc.                    │
│     └─> Event publisher for observability                    │
│                                                              │
│  2. Emit Events                                              │
│     ├─> run.started                                          │
│     └─> plan.loaded                                          │
│                                                              │
│  3. For Each Step in Plan:                                   │
│     │                                                         │
│     ├─> Update Context                                       │
│     │   ├─> CurrentDir = dir of step.Origin.FilePath         │
│     │   └─> Restore loop vars if step.LoopContext present    │
│     │                                                         │
│     ├─> Check Skip Conditions                                │
│     │   ├─> when: evaluate expression with current vars      │
│     │   │   └─> Skip if false                                │
│     │   ├─> tags: check if step tags match filter            │
│     │   │   └─> Skip if no match                             │
│     │   ├─> creates: check if file exists                    │
│     │   │   └─> Skip if exists (idempotency)                 │
│     │   └─> unless: run command silently                     │
│     │       └─> Skip if succeeds (idempotency)               │
│     │                                                         │
│     ├─> If Skipped                                           │
│     │   ├─> Increment skipped counter                        │
│     │   ├─> Emit step.skipped event                          │
│     │   └─> Continue to next step                            │
│     │                                                         │
│     ├─> Generate Step ID                                     │
│     │   └─> Use step.ID from plan (step-0001, etc)           │
│     │                                                         │
│     ├─> Emit step.started Event                              │
│     │   └─> Include step ID, name, action, tags, origin      │
│     │                                                         │
│     ├─> Dispatch to Action Handler                           │
│     │   ├─> Lookup handler in registry by action type        │
│     │   ├─> Validate step configuration                      │
│     │   ├─> Execute or DryRun (based on --dry-run flag)      │
│     │   └─> Return result (changed, stdout, stderr, rc)      │
│     │                                                         │
│     ├─> Handle Result                                        │
│     │   ├─> Check changed_when (override changed flag)       │
│     │   ├─> Check failed_when (override failure)             │
│     │   ├─> Register to variable if step.Register set        │
│     │   └─> Store in context.CurrentResult                   │
│     │                                                         │
│     ├─> If Error                                             │
│     │   ├─> Increment failed counter                         │
│     │   ├─> Emit step.failed event                           │
│     │   └─> Return error (stop execution)                    │
│     │                                                         │
│     └─> If Success                                           │
│         ├─> Increment executed counter                       │
│         ├─> Emit step.completed event                        │
│         └─> Continue to next step                            │
│                                                              │
│  4. Emit run.completed Event                                 │
│     └─> Include stats (executed, skipped, failed, changed)   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Path Expansion Details

#### Include Path Resolution

```go
// 1. Render template
includePath, err := template.Render(*step.Include, ctx.Variables)
// Input:  "{{ env }}/tasks.yml"
// Vars:   {env: "production"}
// Output: "production/tasks.yml"

// 2. Resolve relative to current directory
absPath := filepath.Join(ctx.CurrentDir, includePath)
// CurrentDir: "/home/user/playbook"
// Output:     "/home/user/playbook/production/tasks.yml"

// 3. Make absolute
absPath, err := filepath.Abs(absPath)
// Output: "/home/user/playbook/production/tasks.yml"
```

#### Template Source Resolution

```go
// In planner.renderActionTemplates()
src, err := template.Render(step.Template.Src, ctx.Variables)
// Input:  "templates/{{ app_name }}.j2"
// Vars:   {app_name: "nginx"}
// Output: "templates/nginx.j2"

// Resolve relative to directory containing the step
if !filepath.IsAbs(src) {
    src = filepath.Join(ctx.CurrentDir, src)
}
// CurrentDir: "/home/user/playbook/tasks"
// Output:     "/home/user/playbook/tasks/templates/nginx.j2"
```

#### With FileTree Path Resolution

```go
// 1. Render template
treePath, err := template.Render(*step.WithFileTree, ctx.Variables)
// Input:  "files/{{ env }}"
// Vars:   {env: "prod"}
// Output: "files/prod"

// 2. Walk directory tree
items, err := fileTree.GetFileTree(treePath, ctx.CurrentDir, ctx.Variables)
// CurrentDir: "/home/user/playbook"
// Walks:      "/home/user/playbook/files/prod"
// Returns:    [{src: "/home/user/.../file1", path: "/file1", ...}, ...]

// 3. Sort for determinism
sort.Slice(items, func(i, j int) bool {
    return items[i].Src < items[j].Src
})

// 4. Expand step for each item
for i, item := range items {
    loopCtx := &config.LoopContext{
        Type:  "with_filetree",
        Item:  item,      // Full file metadata
        Index: i,
        Depth: calculateDepth(item.Path),
    }
    // Clone step with loop context...
}
```

## Alternatives Considered

### Alternative 1: Single-Phase Execution

**Approach**: Expand and execute simultaneously (original design)

**Pros**:

- Simpler architecture (no planner)
- Less code

**Cons**:

- Non-deterministic step count
- No dry-run support
- No plan introspection
- Late error discovery
- Poor observability

**Rejected**: Observability and determinism critical for production use

### Alternative 2: Three-Phase (Parse, Plan, Execute)

**Approach**: Add explicit parse phase before planning

**Pros**:

- Cleaner separation
- Earlier syntax error detection

**Cons**:

- More complexity
- Parse + Plan can be combined (current approach)
- No clear benefit

**Rejected**: Two phases sufficient, parsing happens in plan phase

### Alternative 3: Runtime Path Resolution

**Approach**: Resolve relative paths during execution, not planning

**Pros**:

- Paths could use runtime variables
- More flexible

**Cons**:

- Non-deterministic plan (paths change at runtime)
- Harder to cache/reuse plans
- Can't validate paths before execution
- Include resolution requires runtime

**Rejected**: Plan determinism more important than runtime flexibility

### Alternative 4: Lazy Loop Expansion

**Approach**: Don't expand loops during planning, expand at runtime

**Pros**:

- Smaller plans
- Could use runtime variables in with_items

**Cons**:

- Non-deterministic step count
- No way to show "X steps will execute"
- Dry-run can't show individual loop iterations
- Worse observability

**Rejected**: Observability requires knowing exact steps upfront

## Consequences

### Positive

1. **Determinism**
   - Same config = same plan = same execution order
   - Reproducible across runs
   - Testable and debuggable

2. **Observability**
   - Know total steps before execution
   - Show progress: "Step 42/150"
   - Dry-run shows exact steps
   - Events include complete context

3. **Early Error Detection**
   - Syntax errors found during planning
   - Include cycles detected before execution
   - Invalid loops fail fast

4. **Introspection**
   - Inspect plan structure
   - Analyze dependencies
   - Estimate execution time

5. **Optimization Opportunities**
   - Cache plans for reuse
   - Parallelize independent steps (future)
   - Skip unchanged steps (future)

6. **Better Error Messages**
   - Origin tracking shows exact source location
   - Include chain visible in errors
   - Loop context preserved

### Negative

1. **Memory Usage**
   - Full plan stored in memory
   - Large loops create many steps
   - Mitigation: Streaming execution (future)

2. **Two-Phase Complexity**
   - Developers must understand plan vs execute
   - Some logic duplicated (template rendering)
   - Mitigation: Clear documentation

3. **Variable Handling Split**
   - Plan-time vs runtime variables confusing
   - Users might expect runtime vars in templates
   - Mitigation: Clear error messages

4. **Limited Runtime Flexibility**
   - Can't change plan based on execution results
   - Loops must be known at plan time
   - Mitigation: Most use cases don't need this

### Risks

1. **Plan Size Explosion**
   - **Risk**: Very large with_filetree could OOM
   - **Mitigation**: Validate tree size before expansion
   - **Status**: Low risk, not observed in practice

2. **Variable Scope Confusion**
   - **Risk**: Users confused by plan-time vs runtime vars
   - **Mitigation**: Documentation, error messages
   - **Status**: Medium risk, needs good docs

3. **Path Resolution Bugs**
   - **Risk**: Edge cases in relative path handling
   - **Mitigation**: Comprehensive test suite
   - **Status**: Low risk, well tested

## Implementation Details

### Plan Data Structure

```go
type Plan struct {
    Version     string                 // Plan format version
    GeneratedAt time.Time              // When plan was created
    RootFile    string                 // Entry point config file
    Steps       []config.Step          // Fully expanded steps
    InitialVars map[string]interface{} // Variables at plan start
    Tags        []string               // Tag filter
}

type Step struct {
    // Plan metadata
    ID          string         // step-0001, step-0002, ...
    ActionType  string         // shell, file, template, ...
    Origin      *Origin        // Source location
    Skipped     bool           // Filtered by tags at plan time
    LoopContext *LoopContext   // Loop metadata (if from loop)

    // User configuration
    Name        string         // Step name
    When        string         // Runtime condition
    Register    string         // Variable to store result
    Tags        []string       // Step tags

    // Action-specific fields
    Shell       *ShellAction
    File        *FileAction
    Template    *TemplateAction
    // ... etc
}
```

### Planner Interface

```go
type Planner struct {
    template      template.Renderer
    pathUtil      *pathutil.PathExpander
    fileTree      *filetree.Walker
    stepIDCounter int
    includeStack  []IncludeFrame
    seenFiles     map[string]bool
    locationMap   map[int]*IncludeFrame
}

// BuildPlan generates a deterministic execution plan
func (p *Planner) BuildPlan(cfg PlannerConfig) (*Plan, error)

// ExpandStepsWithContext expands steps with given variables (for presets)
func (p *Planner) ExpandStepsWithContext(
    steps []config.Step,
    variables map[string]interface{},
    currentDir string,
) ([]config.Step, error)
```

### Executor Interface

```go
type ExecutionContext struct {
    Variables      map[string]interface{}
    CurrentDir     string
    CurrentFile    string
    CurrentResult  *Result
    CurrentStepID  string
    Level          int
    CurrentIndex   int
    TotalSteps     int

    // Dependencies
    Logger         logger.Logger
    Template       template.Renderer
    Evaluator      expression.Evaluator
    PathUtil       *pathutil.PathExpander
    FileTree       *filetree.Walker
    Redactor       *security.Redactor
    EventPublisher events.Publisher

    // Statistics
    Stats          *ExecutionStats

    // Configuration
    SudoPass       string
    Tags           []string
    DryRun         bool
}

// ExecutePlan executes a pre-compiled plan
func ExecutePlan(
    p *plan.Plan,
    sudoPass string,
    dryRun bool,
    log logger.Logger,
    publisher events.Publisher,
) error

// ExecuteStep executes a single step
func ExecuteStep(step config.Step, ec *ExecutionContext) error
```

## Example: Complete Flow

### Input Configuration

```yaml
# playbook.yml
vars:
  app: myapp
  env: production

- include: tasks/{{ env }}.yml

- shell: echo "Done"
  register: result
```

```yaml
# tasks/production.yml
- vars:
    replicas: 3

- shell: echo "Deploy {{ item }}"
  with_items: [web, api, worker]
```

### Planning Phase

1. **Load playbook.yml**
   - Parse YAML
   - Validate schema

2. **Initialize variables**
   ```go
   variables = {
       app: "myapp",
       env: "production",
       // + system facts
   }
   ```

3. **Expand steps**

   a. Process `vars: {app: myapp, env: production}`
      - Merge into variables
      - No step added to plan

   b. Process `include: tasks/{{ env }}.yml`
      - Render: "tasks/production.yml"
      - Resolve: "/home/user/playbook/tasks/production.yml"
      - Check cycles: not seen
      - Mark seen, push to stack
      - Recursively expand:

        c. Process `vars: {replicas: 3}` in production.yml
           - Merge into variables
           - variables = {app: "myapp", env: "production", replicas: 3}

        d. Process `shell` with `with_items: [web, api, worker]`
           - Evaluate with_items: [web, api, worker]
           - Clone step 3 times:
             - Step 1: `shell: echo "Deploy web"`, loop_context: {item: "web", index: 0, first: true, last: false}
             - Step 2: `shell: echo "Deploy api"`, loop_context: {item: "api", index: 1, first: false, last: false}
             - Step 3: `shell: echo "Deploy worker"`, loop_context: {item: "worker", index: 2, first: false, last: true}
           - Render commands: "Deploy web", "Deploy api", "Deploy worker"
           - Assign IDs: step-0001, step-0002, step-0003
           - Set origins: all from tasks/production.yml with chain [playbook.yml:3]
           - Add to plan

      - Pop from stack

   e. Process `shell: echo "Done"`
      - Render command: "Done"
      - Assign ID: step-0004
      - Set origin: playbook.yml:5
      - Add to plan

4. **Plan complete**
   ```go
   plan = &Plan{
       RootFile: "/home/user/playbook/playbook.yml",
       Steps: [
           {ID: "step-0001", ActionType: "shell", Shell: {Cmd: "echo \"Deploy web\""}, LoopContext: {...}},
           {ID: "step-0002", ActionType: "shell", Shell: {Cmd: "echo \"Deploy api\""}, LoopContext: {...}},
           {ID: "step-0003", ActionType: "shell", Shell: {Cmd: "echo \"Deploy worker\""}, LoopContext: {...}},
           {ID: "step-0004", ActionType: "shell", Shell: {Cmd: "echo \"Done\""}, Register: "result"},
       ],
       InitialVars: {app: "myapp", env: "production", replicas: 3, ...facts},
   }
   ```

### Execution Phase

1. **Initialize context**
   ```go
   ec = &ExecutionContext{
       Variables: plan.InitialVars,
       CurrentDir: "/home/user/playbook",
       TotalSteps: 4,
       // ... other fields
   }
   ```

2. **Emit events**
   - `run.started`: 4 total steps
   - `plan.loaded`: 4 total steps

3. **Execute step-0001**
   - Restore loop vars: item="web", index=0, first=true, last=false
   - Check when: (none)
   - Emit `step.started`: step-0001, "Deploy web"
   - Dispatch to shell handler
   - Execute: `echo "Deploy web"`
   - Result: stdout="Deploy web\n", changed=false, rc=0
   - Emit `step.completed`: step-0001, changed=false

4. **Execute step-0002**
   - Restore loop vars: item="api", index=1, first=false, last=false
   - (similar to step-0001)
   - Execute: `echo "Deploy api"`

5. **Execute step-0003**
   - Restore loop vars: item="worker", index=2, first=false, last=true
   - (similar to step-0001)
   - Execute: `echo "Deploy worker"`

6. **Execute step-0004**
   - Clear loop vars (no loop_context)
   - Check when: (none)
   - Emit `step.started`: step-0004, "Done"
   - Execute: `echo "Done"`
   - Result: stdout="Done\n", changed=false, rc=0
   - Register to variables: result = {stdout: "Done\n", changed: false, ...}
   - Emit `step.completed`: step-0004

7. **Emit run.completed**
   - executed=4, skipped=0, failed=0, changed=0

## Compliance

This ADR complies with:
- Go best practices for package separation
- Event-driven architecture patterns
- Immutable data structures (plan)
- Deterministic execution principles

## References

- [Planner Implementation](../../../internal/plan/planner.go) - Planning logic
- [Executor Implementation](../../../internal/executor/executor.go) - Execution logic
- [Plan Data Structure](../../../internal/plan/plan.go) - Plan format
- [Path Utilities](../../../internal/pathutil/pathutil.go) - Path resolution

## Related Decisions

- [ADR-001: Handler-Based Action Architecture](001-handler-based-action-architecture.md) - How actions are executed
- [ADR-002: Preset Expansion System](002-preset-expansion-system.md) - How presets integrate with planner

## Future Considerations

1. **Plan Caching**: Cache compiled plans for faster repeated execution
2. **Parallel Execution**: Execute independent steps concurrently
3. **Incremental Execution**: Skip unchanged steps based on checksums
4. **Plan Diff**: Show what changed between plan versions
5. **Plan Export**: Export plan as JSON for external tools
6. **Streaming Execution**: Process large plans without loading fully into memory
7. **Plan Optimization**: Reorder steps for efficiency (respecting dependencies)
8. **Conditional Includes**: Support when conditions on include directives

## Appendix: Why Not Ansible's Approach?

Ansible uses a similar two-phase model (parse → execute), but with key differences:

### Ansible's Approach
- **Templates at Runtime**: Ansible renders templates during execution, not planning
- **Dynamic Includes**: `include_tasks` expanded at runtime
- **Late Binding**: Variable resolution happens as late as possible

### Mooncake's Approach
- **Templates at Plan Time**: Most templates rendered during planning
- **Static Includes**: All includes expanded during planning
- **Early Binding**: Variable resolution happens as early as possible

### Why We Differ

1. **Determinism**: We prioritize knowing exact steps upfront
2. **Observability**: We want complete plan before execution
3. **Debugging**: Early errors better than late errors
4. **Simplicity**: Clear separation of concerns

Trade-off: Less runtime flexibility, but better observability and determinism.


---

<!-- FILE: architecture-decisions/001-handler-based-action-architecture.md -->

# ADR-001: Handler-Based Action Architecture

**Status**: Accepted
**Date**: 2026-02-05
**Deciders**: Engineering Team
**Technical Story**: Action system refactoring to improve maintainability and extensibility

## Context

The original mooncake executor implemented actions as large switch statements and monolithic step handlers within the executor package. This approach had several issues:

1. **Tight Coupling**: All action logic was tightly coupled to the executor
2. **Poor Modularity**: Adding new actions required changes to multiple files (executor.go, dryrun.go, schema.json, config.go)
3. **Test Complexity**: Testing individual actions required importing the entire executor
4. **Code Duplication**: Similar patterns repeated across action implementations
5. **Limited Extensibility**: No clean way to add actions without modifying core executor code

The codebase had:
- ~20,000 lines of action implementation code in executor package
- 12 `*_step.go` files and 5 `*_step_test.go` files
- Manual dispatcher with 40+ line switch statement
- No separation between action logic and execution orchestration

## Decision

We adopted a **handler-based architecture** with the following key components:

**Benefits:**
- **Modular**: Each action is self-contained in one file
- **Extensible**: Adding new actions requires only 1 file + registration
- **Testable**: Actions can be tested in isolation
- **Reduced Complexity**: Net reduction of ~16,000 lines of code

### 1. Handler Interface

Each action implements a 4-method interface:

```go
type Handler interface {
    Metadata() ActionMetadata          // Name, description, category, version
    Validate(*config.Step) error       // Pre-flight validation
    Execute(Context, *config.Step) (Result, error)  // Main execution
    DryRun(Context, *config.Step) error             // Preview mode
}
```

### 2. Registry Pattern

- Thread-safe registry maps action names to handlers
- Handlers self-register via `init()` functions
- Automatic dispatch without manual routing code

```go
func init() {
    actions.Register(&Handler{})
}
```

### 3. Package Structure

- `internal/actions/` - Interface definitions and registry
- `internal/actions/<name>/` - Individual action implementations
- `internal/register/` - Centralized import hub (avoids circular imports)
- `cmd/mooncake.go` - Imports register package to trigger registration

### 4. Execution Flow

1. User defines step in YAML (e.g., `shell: "echo hello"`)
2. Config parser creates Step struct with appropriate action field
3. Executor determines action type via `step.DetermineActionType()`
4. Dispatcher looks up handler in registry: `actions.Get(actionType)`
5. Handler validates, executes, and returns result
6. Executor registers result and continues

## Alternatives Considered

### Alternative 1: Code Generation

**Approach**: Generate action code from JSON schema or templates

**Pros**:

- Guaranteed consistency
- Easy to add actions via configuration

**Cons**:

- Generated code harder to debug
- Less flexible for complex actions
- Build tooling complexity
- Harder to understand for contributors

**Rejected**: Generated code reduces flexibility and increases complexity

### Alternative 2: Plugin System

**Approach**: Load actions as external plugins (.so files)

**Pros**:

- Users can add actions without recompiling
- Complete isolation between actions

**Cons**:

- Go plugin system is experimental and has limitations
- Platform-specific plugin formats
- Version compatibility issues
- Debugging complexity
- Security concerns with external code

**Rejected**: Go plugins are not mature enough for production use

### Alternative 3: Keep Legacy Monolithic Approach

**Approach**: Continue with switch statements and executor-embedded actions

**Pros**:

- No migration needed
- Familiar to existing contributors

**Cons**:

- Continues to accumulate technical debt
- Poor modularity
- Hard to test
- Difficult to extend

**Rejected**: Does not address core maintainability issues

## Consequences

### Positive

1. **Reduced Code Complexity**
   - Net reduction of ~16,000 lines
   - Each action self-contained in one file (100-1000 lines)
   - Clear separation of concerns

2. **Improved Maintainability**
   - Easy to understand action implementation (single file)
   - Clear interface contract
   - No hidden dependencies

3. **Enhanced Extensibility**
   - Adding new action requires only 1 file + registration
   - No dispatcher updates needed
   - No dry-run logger updates needed

4. **Better Testability**
   - Actions can be tested in isolation
   - Mock context for unit tests
   - 816 tests covering all actions

5. **Zero Breaking Changes**
   - Config format unchanged
   - YAML schema unchanged
   - Drop-in replacement for users

6. **Runtime Introspection**
   - Registry provides list of available actions
   - Metadata queryable at runtime
   - Enables future CLI features (e.g., `mooncake actions list`)

### Negative

1. **More Packages**
   - 15 action packages vs 1 executor package
   - Slightly more complex directory structure
   - Mitigated by clear naming and organization

2. **Exported Test Helpers**
   - Some internal functions exported for testing
   - Risk: Users might depend on internal API
   - Mitigated by `INTERNAL` godoc comments and `internal/` package

3. **Import Cycles Required Special Handling**
   - Needed separate register package
   - Slight indirection in import path
   - Mitigated by clear documentation

### Risks

1. **API Stability**
   - **Risk**: Handler interface changes could break all actions
   - **Mitigation**: Interface is simple and unlikely to change
   - **Status**: Low risk

2. **Performance**
   - **Risk**: Registry lookup overhead
   - **Mitigation**: Map lookup is O(1), negligible overhead
   - **Status**: No measurable impact

3. **Learning Curve**
   - **Risk**: New contributors need to understand handler pattern
   - **Mitigation**: Comprehensive documentation, clear examples
   - **Status**: Low risk with good docs

## Implementation Details

### Migration Strategy

1. Created handler interface and registry (foundation)
2. Migrated actions one-by-one (13 actions over several days)
3. Maintained dual dispatch during migration (registry + legacy fallback)
4. Removed legacy code once all actions migrated
5. Updated tests to use new architecture

### File Organization

```
internal/
├── actions/
│   ├── handler.go              # Handler interface
│   ├── registry.go             # Thread-safe registry
│   ├── interfaces.go           # Context/Result interfaces
│   ├── print/
│   │   └── handler.go          # Print action (98 lines)
│   ├── shell/
│   │   └── handler.go          # Shell action (520 lines)
│   ├── file/
│   │   └── handler.go          # File action (795 lines)
│   └── ... (12 more actions)
├── register/
│   └── register.go             # Import hub
└── executor/
    ├── executor.go             # Orchestration, dispatch
    ├── context.go              # Execution context
    └── result.go               # Result type
```

### Handler Example

```go
package print

import (
    "github.com/alehatsman/mooncake/internal/actions"
    "github.com/alehatsman/mooncake/internal/config"
    "github.com/alehatsman/mooncake/internal/executor"
)

type Handler struct{}

func init() {
    actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
    return actions.ActionMetadata{
        Name: "print",
        Description: "Output messages to console",
        Category: actions.CategoryOutput,
        SupportsDryRun: true,
    }
}

func (h *Handler) Validate(step *config.Step) error {
    if step.Print == nil || *step.Print == "" {
        return fmt.Errorf("print message is empty")
    }
    return nil
}

func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
    message := *step.Print
    ctx.GetLogger().Infof(message)

    result := executor.NewResult()
    result.Changed = false
    result.Stdout = message
    return result, nil
}

func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
    message := *step.Print
    ctx.GetLogger().Infof("  [DRY-RUN] Would print: %s", message)
    return nil
}
```

## Compliance

This ADR complies with:
- Go package design principles
- SOLID principles (especially Single Responsibility and Open/Closed)
- Clean Architecture patterns
- Mooncake code style guidelines

## References

- [Adding Actions Guide](../adding-actions.md) - Developer guide for implementing new actions
- [Action Migration Summary](/.claude/projects/-Users-alehatsman-Projects-mooncake/memory/MEMORY.md) - Complete migration history
- [Handler Interface](../../../internal/actions/handler.go) - Source code
- [Registry Implementation](../../../internal/actions/registry.go) - Source code

## Related Decisions

- None (this is the first ADR)

## Future Considerations

1. **Action Versioning**: Consider adding versioning to handler interface for backward compatibility
2. **Action Discovery**: Add CLI command to list available actions and their metadata
3. **Action Metrics**: Collect performance metrics per action type
4. **Action Lifecycle Hooks**: Consider adding BeforeExecute/AfterExecute hooks
5. **Async Actions**: Evaluate support for long-running actions with progress callbacks

## Appendix: Migration Statistics

- **Actions Migrated**: 15 total (13 core + 2 new)
- **Code Reduced**: ~16,000 lines deleted, ~6,000 lines added (net -10,000 lines)
- **Files Deleted**: 17 legacy files
- **Files Created**: 15 handler files + registry infrastructure
- **Test Coverage**: 816 tests passing, 0 failures
- **Breaking Changes**: Zero
- **Migration Duration**: ~2 weeks
- **Build Status**:  All clean


---

<!-- FILE: architecture-decisions/002-preset-expansion-system.md -->

# ADR-002: Preset Expansion System

**Status**: Accepted
**Date**: 2026-02-05
**Deciders**: Engineering Team
**Technical Story**: Extensible preset system for reusable configuration patterns

## Context

As mooncake matured, users frequently requested support for common deployment patterns (Ollama, Docker, PostgreSQL, Nginx, etc.). The initial approach was to implement each as a native Go action (e.g., `ollama` action with ~1,400 lines of code). This approach had several problems:

1. **Maintenance Burden**: Each new tool required ~1,000+ lines of Go code, tests, documentation
2. **Release Cycle Dependency**: Adding/updating tool support required code releases
3. **Limited User Extensibility**: Users couldn't create their own "actions" without Go knowledge
4. **Feature Bloat**: Core binary size grew with each tool integration
5. **Tight Coupling**: Tool-specific logic mixed with mooncake core

The Ollama action exemplified these issues:
- 672 lines in `ollama_step.go` + 646 lines of tests
- Platform detection logic (apt/dnf/yum/brew)
- Service configuration (systemd/launchd)
- Model management
- Installation/uninstallation workflows

Most of this logic could be expressed in YAML using existing mooncake actions (shell, service, file, etc.), but no mechanism existed for packaging reusable workflows.

## Decision

We adopted a **preset system** that allows packaging reusable workflows as YAML files. Presets expand into constituent steps at execution time with parameter injection.

**Benefits:**
- **Extensible**: Users can create presets without Go knowledge
- **Maintainable**: Update workflows in YAML, no code releases needed
- **Smaller Binary**: Tool-specific code moved out of core
- **Faster Iteration**: Presets can be updated without recompilation

### 1. Preset Structure

Presets are YAML files defining:
- **Name**: Unique identifier
- **Description**: Human-readable summary
- **Version**: Semantic version
- **Parameters**: Typed parameter definitions (string, bool, array, object)
- **Steps**: Mooncake steps using existing actions

Example:
```yaml
preset:
  name: ollama
  description: Install and configure Ollama AI runtime
  version: 1.0.0
  parameters:
    - name: state
      type: string
      default: present
      enum: [present, absent, running, stopped]
    - name: models
      type: array
      required: false
  steps:
    - name: Install Ollama
      shell: curl -fsSL https://ollama.com/install.sh | sh
      when: "{{ parameters.state != 'absent' }}"
    - name: Configure service
      service:
        name: ollama
        state: "{{ parameters.state }}"
```

### 2. Key Architectural Decisions

#### Flat Presets Only (No Nesting)
- Presets CANNOT invoke other presets
- Prevents circular dependencies
- Simpler mental model and execution flow
- Easier to debug and trace

```yaml
#  NOT ALLOWED
preset:
  steps:
    - preset: base-setup  # Would fail validation
```

#### Parameters Namespace
- Parameters accessible via `parameters.name` in templates
- Clear separation from variables and facts
- Prevents naming collisions

```yaml
- shell: echo "{{ parameters.state }}"  #  Explicit namespace
- shell: echo "{{ state }}"              #  Would look in variables
```

#### Register at Preset Level
- Preset returns aggregate result (changed = any step changed)
- Users get `preset_result.changed`, `preset_result.stdout`
- Individual step results not exposed (encapsulation)

```yaml
- preset: ollama
  with:
    state: present
  register: install_result

- print: "Ollama changed: {{ install_result.changed }}"
```

#### Discovery Paths (Priority Order)
1. `./presets/` - Playbook-local presets
2. `~/.mooncake/presets/` - User presets
3. `/usr/local/share/mooncake/presets/` - Local installation
4. `/usr/share/mooncake/presets/` - System installation

#### Two File Formats
- **Flat**: `<name>.yml` (e.g., `presets/ollama.yml`)
- **Directory**: `<name>/preset.yml` (e.g., `presets/ollama/preset.yml`)
- Directory format supports bundling templates/files with preset

### 3. Execution Flow

1. User invokes preset: `preset: {name: ollama, with: {state: present}}`
2. Loader searches discovery paths for preset definition
3. Validator checks parameters (types, required, enum constraints)
4. Expander creates `parameters` namespace with validated params
5. Expander clones preset steps
6. **Planner** expands includes, loops, templates (with parameters injected)
7. Executor runs expanded steps sequentially
8. Handler aggregates results (changed = any step changed)
9. Result registered to user's variable if requested

### 4. Integration with Planner

Presets integrate with the planner's expansion system:
- Preset steps may contain `include` directives → expanded by planner
- Preset steps may contain `with_items` loops → expanded by planner
- Preset steps may use relative paths → resolved from preset base directory
- Parameters injected into variable context before planner expansion

This ensures presets work seamlessly with all mooncake features.

## Alternatives Considered

### Alternative 1: Nested Presets

**Approach**: Allow presets to invoke other presets

**Pros**:

- Better composition and reuse
- DRY principle for common patterns

**Cons**:

- Circular dependency complexity
- Harder to debug (deep nesting)
- Parameter passing complexity
- Execution order ambiguity

**Rejected**: Simplicity and debuggability more important than composition

### Alternative 2: Global Variables Instead of Parameters Namespace

**Approach**: Inject parameters directly into global variable context

**Pros**:

- Simpler template syntax: `{{ state }}` vs `{{ parameters.state }}`

**Cons**:

- Name collisions with user variables
- Unclear where values come from
- Harder to track parameter usage

**Rejected**: Explicit namespace prevents subtle bugs and improves clarity

### Alternative 3: Keep Tool-Specific Actions

**Approach**: Continue implementing tools as Go actions

**Pros**:

- No new concepts for users
- Potentially better performance
- Compile-time validation

**Cons**:

- Doesn't solve maintenance burden
- Users can't extend without Go knowledge
- Binary bloat
- Slow feature iteration

**Rejected**: Doesn't address core extensibility problem

### Alternative 4: Expose Individual Step Results

**Approach**: Return array of results instead of aggregate

**Pros**:

- More granular control for users
- Can inspect each step individually

**Cons**:

- Breaks encapsulation
- Implementation details leak
- API changes when preset internals change
- More complex for users

**Rejected**: Aggregate result better matches abstraction level

## Consequences

### Positive

1. **Code Reduction**
   - Ollama: ~1,400 lines Go → 250 lines YAML
   - Removed: `ollama_step.go`, `ollama_step_test.go`
   - Net: -1,400 lines code, +250 lines YAML

2. **User Extensibility**
   - Users can create presets without Go knowledge
   - Share presets via git/files
   - Community can contribute presets

3. **Faster Iteration**
   - Update presets without recompiling
   - No release cycle for preset changes
   - Users can hotfix/customize locally

4. **Smaller Binary**
   - Tool-specific code moved to YAML
   - Core stays focused and minimal

5. **Better Separation of Concerns**
   - Core: execution engine
   - Presets: tool workflows
   - Clear boundary

6. **Validation and Safety**
   - Parameter type checking
   - Required parameter enforcement
   - Enum constraint validation

### Negative

1. **Two Ways to Do Things**
   - Users might be confused: action vs preset?
   - Mitigation: Clear documentation, use presets for tools

2. **Runtime Errors Instead of Compile-Time**
   - YAML typos discovered at runtime
   - Mitigation: JSON schema validation, dry-run mode

3. **Performance Overhead**
   - YAML parsing and expansion at runtime
   - Mitigation: Overhead negligible for typical workloads

4. **Limited Type Safety**
   - No compile-time checks for parameter usage
   - Mitigation: Parameter validation catches most issues

### Risks

1. **Preset Quality**
   - **Risk**: Community presets may be buggy/insecure
   - **Mitigation**: Discovery path priority (local overrides system)
   - **Status**: Low risk with good docs

2. **Breaking Changes**
   - **Risk**: Preset API changes break users
   - **Mitigation**: Semantic versioning for presets
   - **Status**: Medium risk, needs version checking

3. **Performance**
   - **Risk**: Complex presets might be slow
   - **Mitigation**: Benchmarking, optimization if needed
   - **Status**: Low risk, not observed in practice

## Implementation Details

### File Organization

```
internal/
├── presets/
│   ├── loader.go       # Preset discovery and loading
│   ├── validator.go    # Parameter validation
│   └── expander.go     # Step expansion with parameters
├── actions/
│   └── preset/
│       └── handler.go  # Preset action handler
presets/
├── ollama.yml          # Flat format example
└── complex-app/        # Directory format example
    ├── preset.yml
    ├── templates/
    │   └── config.j2
    └── files/
        └── default.conf
```

### Parameter Validation

```go
// ValidateParameters checks user-provided parameters against definition
func ValidateParameters(def *PresetDefinition, userParams map[string]interface{}) (map[string]interface{}, error) {
    validated := make(map[string]interface{})

    // Check each defined parameter
    for _, param := range def.Parameters {
        value, provided := userParams[param.Name]

        // Apply defaults
        if !provided && param.Default != nil {
            value = param.Default
        }

        // Check required
        if !provided && param.Required {
            return nil, fmt.Errorf("required parameter '%s' not provided", param.Name)
        }

        // Type checking
        if err := validateType(param.Type, value); err != nil {
            return nil, fmt.Errorf("parameter '%s': %w", param.Name, err)
        }

        // Enum constraints
        if len(param.Enum) > 0 && !contains(param.Enum, value) {
            return nil, fmt.Errorf("parameter '%s' must be one of %v", param.Name, param.Enum)
        }

        validated[param.Name] = value
    }

    // Check for unknown parameters
    for name := range userParams {
        if !isDefined(def, name) {
            return nil, fmt.Errorf("unknown parameter '%s'", name)
        }
    }

    return validated, nil
}
```

### Context Isolation

Preset execution preserves caller's variable context:

```go
// Save context before execution
saved := captureContext(ec)
defer saved.restore(ec, parametersNamespace)

// Inject parameters
for k, v := range parametersNamespace {
    ec.Variables[k] = v
}

// Execute steps
ExecuteSteps(expandedSteps, ec)

// Context automatically restored by defer
```

### Relative Path Resolution

Preset base directory used for relative paths:

```yaml
# In presets/myapp/preset.yml
steps:
  - template:
      src: templates/config.j2      # Resolved to presets/myapp/templates/config.j2
      dest: /etc/myapp/config.conf
  - copy:
      src: files/default.conf       # Resolved to presets/myapp/files/default.conf
      dest: /etc/myapp/default.conf
```

## Validation of Approach

### Ollama Migration Success

The Ollama action was successfully migrated to a preset:
- **Before**: 1,400 lines Go (action + tests)
- **After**: 250 lines YAML
- **Functionality**: Identical (all features preserved)
- **Test Coverage**: Manual testing + examples
- **Breaking Changes**: Zero (users updated config)

This validates that presets can handle complex, multi-platform workflows.

### Preset vs Action Guidelines

**Use Presets For**:

- Tool installation/configuration (Ollama, Docker, PostgreSQL)
- Multi-step workflows (deploy webapp, setup dev environment)
- Platform-specific logic (apt vs dnf vs brew)
- Service management patterns

**Use Actions For**:

- Primitive operations (file, shell, template)
- Performance-critical paths
- Complex logic requiring Go
- Core mooncake features

## Compliance

This ADR complies with:
- YAML specification for preset format
- JSON Schema for parameter validation
- Mooncake code style guidelines
- Security best practices (no code execution from presets)

## References

- [Preset User Guide](../../guide/presets.md) - How to use presets
- [Preset Authoring Guide](../../guide/preset-authoring.md) - How to create presets
- [Preset Loader](../../../internal/presets/loader.go) - Discovery and loading
- [Preset Handler](../../../internal/actions/preset/handler.go) - Execution
- [Ollama Preset](../../../presets/ollama.yml) - Real-world example

## Related Decisions

- [ADR-000: Planner and Execution Model](003-planner-execution-model.md) - How presets integrate with planner expansion
- [ADR-001: Handler-Based Action Architecture](001-handler-based-action-architecture.md) - Preset is implemented as an action handler

## Future Considerations

1. **Preset Versioning**: Add version checking and compatibility validation
2. **Preset Registry**: Central repository for community presets
3. **Preset Testing**: Framework for testing presets (like molecule for Ansible)
4. **Preset Documentation**: Auto-generate docs from preset definitions
5. **Preset Dependencies**: Allow declaring required system packages/tools
6. **Conditional Parameters**: Parameter visibility based on other parameter values
7. **Preset Composition**: Explore safe nesting patterns if user demand arises

## Appendix: Migration Statistics

### Ollama Action Removal
- **Files Deleted**: 2 files (1,318 lines)
  - `internal/executor/ollama_step.go` (672 lines)
  - `internal/executor/ollama_step_test.go` (646 lines)
- **Files Created**: 1 file (250 lines)
  - `presets/ollama.yml`
- **Net Change**: -1,068 lines (-81% code reduction)
- **Breaking Changes**: None (config syntax equivalent)
- **User Migration**: Update `ollama:` to `preset: {name: ollama, with: {...}}`

### Preset System Implementation
- **Core Files**: 4 files (715 lines)
  - `internal/presets/loader.go` (120 lines)
  - `internal/presets/validator.go` (180 lines)
  - `internal/presets/expander.go` (50 lines)
  - `internal/actions/preset/handler.go` (205 lines)
  - Tests (160 lines)
- **Documentation**: 3 guides (1,400+ lines)
  - User guide (600 lines)
  - Authoring guide (800 lines)
  - Reference updates
- **Examples**: 30+ examples in `examples/ollama/`

### Overall Impact
- **Code**: -1,068 lines (action removal) + 715 lines (preset system) = -353 net lines
- **Extensibility**: Users can now create presets without Go knowledge
- **Maintenance**: Preset updates = YAML edits (no releases needed)
- **Performance**: Negligible overhead measured


---

<!-- FILE: architecture-decisions/README.md -->

# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) documenting key architectural decisions made during the development of Mooncake.

## ADRs

- [ADR 000: Planner Execution Model](000-planner-execution-model.md) - Three-phase execution model (parse → plan → execute)
- [ADR 001: Handler-Based Action Architecture](001-handler-based-action-architecture.md) - Modular action system with handler interface
- [ADR 002: Preset Expansion System](002-preset-expansion-system.md) - Flat preset architecture with parameter injection

## What is an ADR?

Architecture Decision Records capture important architectural decisions along with their context and consequences. They help developers understand:

- Why the system is structured the way it is
- What alternatives were considered
- What trade-offs were made
- What constraints influenced the decision

## Format

Each ADR follows this structure:

- **Status**: Proposed, Accepted, Deprecated, Superseded
- **Context**: The issue motivating this decision
- **Decision**: The change being proposed or adopted
- **Consequences**: The resulting context after applying the decision

## Contributing

When making significant architectural changes, document them as ADRs. Number them sequentially (003, 004, etc.).


---

<!-- FILE: development/adding-actions.md -->

# Adding New Actions to Mooncake

This guide explains how to add new actions to Mooncake using the handler-based architecture.

## Architecture Overview

Mooncake uses a **handler-based architecture** where each action is a self-contained package implementing a standard interface. This replaces the old approach of spreading action logic across 7+ files.

### Key Components

```
internal/actions/
├── handler.go           # Handler interface definition
├── registry.go          # Thread-safe action registry
├── interfaces.go        # Context and Result interfaces
└── <action_name>/
    └── handler.go       # Self-contained action implementation

internal/register/
└── register.go          # Imports all actions to trigger registration

internal/executor/
└── executor.go          # Dispatches to handlers via registry
```

### The Handler Interface

Every action must implement this 4-method interface:

```go
type Handler interface {
    // Metadata returns action information
    Metadata() ActionMetadata

    // Validate checks if the step configuration is valid
    Validate(step *config.Step) error

    // Execute performs the action and returns a result
    Execute(ctx Context, step *config.Step) (Result, error)

    // DryRun logs what would happen without making changes
    DryRun(ctx Context, step *config.Step) error
}
```

## Step-by-Step Guide

### 1. Create the Action Package

Create a new directory for your action:

```bash
mkdir -p internal/actions/myaction
```

### 2. Implement the Handler

Create `internal/actions/myaction/handler.go`:

```go
// Package myaction implements the myaction action handler.
// Brief description of what this action does.
package myaction

import (
    "fmt"

    "github.com/alehatsman/mooncake/internal/actions"
    "github.com/alehatsman/mooncake/internal/config"
    "github.com/alehatsman/mooncake/internal/executor"
)

// Handler implements the myaction action handler.
type Handler struct{}

func init() {
    actions.Register(&Handler{})
}

// Metadata returns the action metadata.
func (h *Handler) Metadata() actions.ActionMetadata {
    return actions.ActionMetadata{
        Name:           "myaction",
        Description:    "Brief description of what this action does",
        Category:       actions.CategorySystem, // or CategoryCommand, CategoryFile, etc.
        SupportsDryRun: true,
    }
}

// Validate validates the action configuration.
func (h *Handler) Validate(step *config.Step) error {
    if step.MyAction == nil {
        return fmt.Errorf("myaction requires configuration")
    }

    // Validate required fields
    if step.MyAction.SomeRequiredField == "" {
        return fmt.Errorf("myaction.some_required_field is required")
    }

    return nil
}

// Execute executes the action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
    // Cast context to ExecutionContext for full access
    ec, ok := ctx.(*executor.ExecutionContext)
    if !ok {
        return nil, fmt.Errorf("invalid context type")
    }

    myAction := step.MyAction

    // Render template variables in user input
    renderedValue, err := ec.Template.Render(myAction.SomeField, ec.Variables)
    if err != nil {
        return nil, &executor.RenderError{Field: "myaction.some_field", Cause: err}
    }

    // Perform the action
    // ... your logic here ...

    // Create and return result
    result := executor.NewResult()
    result.Changed = true // or false if idempotent and no change made
    result.Stdout = "Output message"

    return result, nil
}

// DryRun logs what the action would do.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
    ec, ok := ctx.(*executor.ExecutionContext)
    if !ok {
        return fmt.Errorf("invalid context type")
    }

    myAction := step.MyAction

    ec.Logger.Infof("  [DRY-RUN] Would execute myaction with value: %s", myAction.SomeField)

    return nil
}
```

### 3. Add Configuration Struct

Add your action's configuration to `internal/config/config.go`:

```go
// In the Step struct, add your action field:
type Step struct {
    // ... existing fields ...

    MyAction *MyActionConfig `yaml:"myaction" json:"myaction,omitempty"`

    // ... other fields ...
}

// Define your action's configuration:
type MyActionConfig struct {
    SomeRequiredField string `yaml:"some_required_field" json:"some_required_field"`
    OptionalField     string `yaml:"optional_field" json:"optional_field,omitempty"`
}
```

Update the `DetermineActionType()` method:

```go
func (s *Step) DetermineActionType() string {
    // ... existing checks ...

    if s.MyAction != nil {
        return "myaction"
    }

    // ... rest of checks ...
}
```

Update the `countActions()` method:

```go
func (s *Step) countActions() int {
    count := 0
    // ... existing counts ...
    if s.MyAction != nil { count++ }
    // ... rest of counts ...
    return count
}
```

### 4. Register the Handler

Add your action to `internal/register/register.go`:

```go
import (
    // ... existing imports ...
    _ "github.com/alehatsman/mooncake/internal/actions/myaction"
)
```

### 5. Update JSON Schema (Optional but Recommended)

Add your action to `internal/config/schema.json` for validation and IDE support.

### 6. Test Your Action

Create a test YAML file:

```yaml
- name: Test my action
  myaction:
    some_required_field: "test value"
    optional_field: "{{ some_var }}"
  register: result

- name: Show result
  print: "Changed: {{ result.changed }}, Output: {{ result.stdout }}"
```

Run it:

```bash
# Dry-run first
go run cmd/mooncake.go run --config test.yml --dry-run

# Then actual execution
go run cmd/mooncake.go run --config test.yml
```

## Common Patterns

### Rendering Variables

Always render user input that might contain template variables:

```go
rendered, err := ec.Template.Render(input, ec.Variables)
if err != nil {
    return nil, &executor.RenderError{Field: "myaction.field", Cause: err}
}
```

### Error Types

Use typed errors from the executor package:

- `executor.RenderError` - Template rendering failures
- `executor.StepValidationError` - Invalid configuration
- `executor.FileOperationError` - File operations
- `executor.CommandError` - Command execution
- `executor.SetupError` - Infrastructure/setup issues

Example:

```go
return nil, &executor.FileOperationError{
    Operation: "read",
    Path:      path,
    Cause:     err,
}
```

### Idempotency

Check current state before making changes:

```go
// Check if change is needed
currentState, err := checkCurrentState()
if err != nil {
    return nil, err
}

if currentState == desiredState {
    result.Changed = false
    return result, nil
}

// Make the change
if err := applyChange(); err != nil {
    return nil, err
}

result.Changed = true
```

### Sudo/Privilege Escalation

If your action needs elevated privileges, check the `step.Become` field:

```go
if step.Become {
    // Use sudo for operations
    cmd := exec.Command("sudo", "-S", "some-command")
    // ... handle sudo execution ...
}
```

Access sudo password via `ec.SudoPass` if needed.

### Working with Files

Use the PathUtil for path operations:

```go
// Expand ~ and render variables
expandedPath, err := ec.PathUtil.ExpandPath(path, ec.CurrentDir, ec.Variables)
if err != nil {
    return nil, err
}

// Make paths absolute relative to config directory
if !filepath.IsAbs(expandedPath) {
    expandedPath = filepath.Join(ec.CurrentDir, expandedPath)
}
```

### Emitting Events

Emit events for important operations:

```go
ec.EmitEvent(events.EventFileCopied, events.FileCopyData{
    Src:  srcPath,
    Dest: destPath,
})
```

### Result Registration

Results are automatically registered if `step.Register` is set. Just create and return the result:

```go
result := executor.NewResult()
result.Changed = true
result.Stdout = "Command output"
result.Stderr = "Error output"
result.Rc = 0

return result, nil
```

## Categories

Choose the appropriate category in your Metadata:

- `CategoryCommand` - Command execution (shell, command)
- `CategoryFile` - File operations (file, copy, template)
- `CategorySystem` - System management (service, assert)
- `CategoryData` - Data operations (vars, include_vars)
- `CategoryNetwork` - Network operations (download)
- `CategoryOutput` - Output operations (print)

## Examples

### Simple Action (Print)

See `internal/actions/print/handler.go` - ~98 lines, straightforward implementation.

### Medium Complexity (Template)

See `internal/actions/template/handler.go` - ~320 lines, file operations and rendering.

### Complex Action (File)

See `internal/actions/file/handler.go` - ~795 lines, multiple states and operations.

### Very Complex (Service)

See `internal/actions/service/handler.go` - ~1090 lines, platform-specific logic.

## Benefits of This Architecture

1. **Self-contained** - All logic for an action in one file
2. **No dispatcher updates** - Registry handles routing automatically
3. **Type safety** - Compiler enforces Handler interface
4. **Easy testing** - Can test handlers in isolation
5. **Clear contracts** - Interface documents requirements
6. **Less boilerplate** - 1 file vs 7 files per action

## Migration from Old System

If migrating an existing action:

1. Copy logic from `internal/executor/<action>_step.go`
2. Wrap in Handler interface methods
3. Update package references (add `executor.` prefix where needed)
4. Register in `internal/register/register.go`
5. Test thoroughly
6. Keep old implementation until verified

## Checklist

- [ ] Created handler package in `internal/actions/<name>/`
- [ ] Implemented all 4 Handler methods
- [ ] Added `init()` with `actions.Register()`
- [ ] Added config struct to `internal/config/config.go`
- [ ] Updated `DetermineActionType()` in config
- [ ] Updated `countActions()` in config
- [ ] Registered in `internal/register/register.go`
- [ ] Added to JSON schema (optional)
- [ ] Created test YAML file
- [ ] Tested in dry-run mode
- [ ] Tested actual execution
- [ ] Verified all existing tests still pass
- [ ] Added documentation (optional)

## Questions?

See existing handlers in `internal/actions/` for reference implementations.


---

<!-- FILE: development/contributing.md -->

# Contributing to Mooncake

Thanks for your interest in contributing to Mooncake! 

## Ways to Contribute

- 🐛 Report bugs
-  Suggest features
-  Improve documentation
- 🧪 Add tests
-  Implement features
- 📚 Create examples
- 🎨 Improve UX

## Getting Started

### Development Setup

1. **Clone the repository**
```bash
git clone https://github.com/alehatsman/mooncake.git
cd mooncake
```

2. **Install dependencies**
```bash
go mod download
```

3. **Run tests**
```bash
go test ./...
```

4. **Run with coverage**
```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

5. **Build locally**
```bash
go build -o mooncake cmd/mooncake.go
./mooncake --help
```

### Project Structure

```
mooncake/
├── cmd/
│   └── mooncake.go          # CLI entry point
├── internal/
│   ├── config/              # Configuration parsing and validation
│   ├── executor/            # Step execution logic
│   ├── expression/          # Condition evaluation
│   ├── facts/               # System information collection
│   ├── filetree/            # File tree iteration
│   ├── logger/              # Logging and TUI
│   ├── pathutil/            # Path resolution
│   └── template/            # Template rendering
├── examples/                # Example configurations
├── README.md                # Main documentation
├── ROADMAP.md              # Feature roadmap
└── CONTRIBUTING.md         # This file
```

## Contribution Workflow

### 1. Find or Create an Issue

- Check existing [issues](https://github.com/alehatsman/mooncake/issues)
- For bugs: describe steps to reproduce
- For features: explain use case and proposed solution
- Wait for discussion/approval before starting work

### 2. Fork and Branch

```bash
# Fork the repo on GitHub, then:
git clone https://github.com/YOUR_USERNAME/mooncake.git
cd mooncake
git checkout -b feature/your-feature-name
```

Branch naming:
- `feature/description` - New features
- `fix/description` - Bug fixes
- `docs/description` - Documentation
- `test/description` - Tests only

### 3. Make Your Changes

**Write good commit messages:**
```
Add support for with_dict loop iteration

- Implement DictIterator in filetree package
- Add with_dict handling in executor
- Add tests for dict iteration
- Update documentation with examples
```

**Follow Go conventions:**
- Run `go fmt ./...`
- Run `go vet ./...`
- Add tests for new code
- Update documentation

**Keep commits focused:**
- One logical change per commit
- Separate refactoring from features
- Separate tests from implementation

### 4. Add Tests

All new features must include tests:

```go
// internal/executor/executor_test.go
func TestWithDict(t *testing.T) {
    // Arrange
    config := []config.Step{
        {
            Name: "Test dict iteration",
            Shell: pointer("echo {{item.key}}: {{item.value}}"),
            WithDict: pointer("{{my_dict}}"),
        },
    }

    // Act
    result := Execute(config, context)

    // Assert
    assert.NoError(t, result.Error)
    assert.Equal(t, 3, result.StepsExecuted)
}
```

**Test coverage:**
- Aim for 80%+ coverage on new code
- Test happy path and error cases
- Test edge cases

### 5. Update Documentation

If your change affects users:

- [ ] Update README.md
- [ ] Add example in examples/
- [ ] Add entry to ROADMAP.md (if feature)
- [ ] Update relevant example READMEs

### 6. Submit Pull Request

```bash
git push origin feature/your-feature-name
```

Then create a PR on GitHub with:

**Title:** Clear, concise description
```
Add with_dict loop iteration support
```

**Description template:**
```markdown
## What does this PR do?

Adds support for iterating over dictionaries using with_dict.

## Why is this needed?

Users often need to iterate over key-value pairs, currently only
list iteration is supported.

## How was it implemented?

- Added DictIterator in filetree package
- Extended executor to handle with_dict
- Added comprehensive tests

## Examples

\`\`\`yaml
- vars:
    ports:
      web: 80
      api: 8080
      admin: 9000

- name: Configure port
  shell: echo "{{item.key}} runs on port {{item.value}}"
  with_dict: "{{ports}}"
\`\`\`

## Testing

- [x] Added unit tests
- [x] Tested manually with examples
- [x] Updated documentation

## Checklist

- [x] Tests pass
- [x] Code formatted (`go fmt`)
- [x] Documentation updated
- [x] Example added
```

## Code Style

### Go Style Guide

Follow [Effective Go](https://golang.org/doc/effective_go.html) and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).

**Key points:**
- Use `gofmt` for formatting
- Keep functions small and focused
- Write clear, descriptive names
- Add comments for exported functions
- Use early returns to reduce nesting

**Example:**
```go
// ExecuteStep executes a single configuration step within the given execution context.
// It validates the step, checks skip conditions, and dispatches to the appropriate handler.
func ExecuteStep(step config.Step, ec *ExecutionContext) error {
    // Validate step configuration
    if err := step.Validate(); err != nil {
        return err
    }

    // Check if step should be skipped
    shouldSkip, skipReason, err := checkSkipConditions(step, ec)
    if err != nil {
        return err
    }
    if shouldSkip {
        logSkipped(step, skipReason, ec)
        return nil
    }

    // Execute the step
    return dispatchStepAction(step, ec)
}
```

### Configuration Style

When adding examples:
- Use clear, descriptive names
- Add comments explaining non-obvious choices
- Keep examples focused on one feature
- Test examples before committing

## Testing Guidelines

### Unit Tests

```bash
# Run all tests
go test ./...

# Run specific package
go test ./internal/executor

# Run with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run with race detector
go test ./... -race
```

### Integration Tests

Add integration tests in `internal/executor/executor_test.go` for:
- End-to-end workflows
- Interaction between features
- Real file system operations

### Example Testing

Before submitting:
```bash
# Test examples work
mooncake run --config examples/01-hello-world/config.yml --dry-run
mooncake run --config examples/05-templates/config.yml --dry-run

# Test all examples
for example in examples/*/config.yml; do
    echo "Testing $example"
    mooncake run --config $example --dry-run || exit 1
done
```

## Documentation Guidelines

### README Updates

When updating README.md:
- Maintain existing structure
- Use clear, concise language
- Include code examples
- Link to detailed examples
- Test all code examples

### Example Documentation

Each example should have:
- README.md with clear explanation
- "What You'll Learn" section
- "Quick Start" commands
- "Key Concepts" section
- Working configuration that can be run

### Code Comments

```go
// Good: Explains why
// Use nested execution context to isolate loop variables
curEc := ec.Clone()

// Bad: Explains what (obvious from code)
// Copy the execution context
curEc := ec.Clone()
```

## Feature Proposals

For significant features, create a proposal in `docs/proposals/`:

```markdown
# Proposal: With Dict Iteration

## Problem

Users need to iterate over dictionaries (key-value pairs) but currently
only list iteration is supported with with_items.

## Proposed Solution

Add `with_dict` that iterates over dictionaries, providing `item.key`
and `item.value` in each iteration.

## Design

### Configuration Syntax

\`\`\`yaml
- vars:
    ports:
      web: 80
      api: 8080

- name: Configure port
  shell: echo "{{item.key}}: {{item.value}}"
  with_dict: "{{ports}}"
\`\`\`

### Implementation

1. Add WithDict field to Step struct
2. Implement dict iteration in executor
3. Add tests

### Alternatives Considered

1. Extend with_items to handle dicts - Rejected, too implicit
2. Use template filters - Rejected, not ergonomic

## Open Questions

- Should we support nested dicts?
- What about empty dicts?
```

## Pull Request Review Process

1. **Automated checks** - CI must pass
2. **Code review** - Maintainer reviews code
3. **Documentation review** - Check docs updated
4. **Testing verification** - Verify tests adequate
5. **Final approval** - Merge when approved

**Review criteria:**
- Code quality and style
- Test coverage
- Documentation completeness
- Backward compatibility
- Performance impact

## Community Guidelines

- Be respectful and constructive
- Welcome newcomers
- Help others learn
- Focus on the problem, not the person
- Assume good intent

## Getting Help

- **Questions:** Open a [discussion](https://github.com/alehatsman/mooncake/discussions)
- **Bugs:** Open an [issue](https://github.com/alehatsman/mooncake/issues)
- **GitHub Discussions:** [Ask questions and share ideas](https://github.com/alehatsman/mooncake/discussions)

## Recognition

Contributors are recognized in:
- Git commit history
- Release notes
- CONTRIBUTORS.md (if we create it)

Thank you for contributing to Mooncake! 


---

<!-- FILE: development/proposals.md -->

# Feature Proposals

This directory contains detailed design proposals for new Mooncake features.

## Purpose

Proposals help:
- Think through design before implementation
- Get feedback from community
- Document decision-making process
- Provide reference during implementation

## When to Write a Proposal

Write a proposal for:
- **New major features** - Significant additions to Mooncake
- **Breaking changes** - Changes that affect existing configurations
- **Complex features** - Features requiring architectural decisions
- **Controversial features** - Features that may have multiple approaches

**Don't need a proposal for:**
- Bug fixes
- Documentation updates
- Small improvements
- Tests

## Proposal Template

Create a new file: `NNNN-feature-name.md`

```markdown
# Proposal: Feature Name

- **Author:** Your Name (@github-username)
- **Status:** Draft | Under Review | Accepted | Rejected | Implemented
- **Created:** YYYY-MM-DD
- **Updated:** YYYY-MM-DD

## Summary

One paragraph explanation of the feature.

## Motivation

Why is this feature needed? What problem does it solve?

### Use Cases

Concrete examples of when users would use this feature:

1. **Use case 1:** User wants to...
2. **Use case 2:** User needs to...

## Proposed Solution

### Configuration Syntax

\`\`\`yaml
# Example of how users would use this feature
- name: Example step
  new_action:
    parameter: value
\`\`\`

### Implementation Overview

High-level approach:
1. Changes to config package
2. Changes to executor
3. New packages/files needed

### Detailed Design

#### Data Structures

\`\`\`go
type NewFeature struct {
    // Fields...
}
\`\`\`

#### Execution Flow

1. Step 1...
2. Step 2...

#### Error Handling

How errors are detected and reported.

### Examples

Complete working examples:

\`\`\`yaml
# Example 1: Basic usage
- name: Basic example
  new_action:
    param: value
\`\`\`

## Alternatives Considered

### Alternative 1: Different Approach

**Pros:**
- Advantage 1
- Advantage 2

**Cons:**
- Disadvantage 1
- Disadvantage 2

**Why rejected:** Explanation

### Alternative 2: Another Approach

[Same format]

## Compatibility

### Backward Compatibility

Does this break existing configurations?
- [ ] Yes (requires migration guide)
- [x] No

### Migration Path

If breaking: How do users migrate?

## Implementation Plan

### Phase 1: Core Implementation
- [ ] Task 1
- [ ] Task 2

### Phase 2: Documentation
- [ ] README updates
- [ ] Example creation
- [ ] Proposal in docs

### Phase 3: Testing
- [ ] Unit tests
- [ ] Integration tests
- [ ] Manual testing

### Estimated Effort

- Implementation: X hours/days
- Testing: X hours/days
- Documentation: X hours/days
- **Total:** X hours/days

## Open Questions

1. **Question 1:** What about edge case X?
2. **Question 2:** How should we handle Y?

## References

- Related issues: #123, #456
- Related PRs: #789
- External docs: [link]

## Decision

**Date:** YYYY-MM-DD
**Decision:** Accepted | Rejected
**Reason:** Why was this decision made?
```

## Proposal Process

### 1. Draft

- Copy template
- Fill in details
- Focus on motivation and use cases

### 2. Community Review

- Open PR with proposal
- Label: `proposal`
- Gather feedback
- Update based on comments

### 3. Decision

- Maintainer reviews
- Community discusses
- Accept, reject, or request changes

### 4. Implementation

- Accepted proposals can be implemented
- Link PR to proposal
- Update proposal status

## Existing Proposals

<!-- Add links to proposals as they're created -->

None yet! Be the first to propose a feature.

## Example Proposals

### Good Examples

**with_dict Iteration**
```
Problem: Clear use case
Solution: Well-defined syntax
Examples: Multiple working examples
Implementation: Clear approach
Alternatives: Considered and rejected with reasons
```

### What to Avoid

**Bad Proposal**
```
Problem: Vague "make it better"
Solution: No concrete syntax
Examples: None or incomplete
Implementation: "Just add the feature"
Alternatives: None considered
```

## Tips for Good Proposals

1. **Start with use cases** - Real problems users have
2. **Show examples first** - Syntax before implementation
3. **Consider alternatives** - Show you've thought it through
4. **Keep it focused** - One feature per proposal
5. **Be specific** - Concrete syntax and behavior
6. **Think about errors** - How will failures be handled?
7. **Consider compatibility** - Impact on existing configs

## Questions?

- Open an issue with `[Proposal]` prefix
- Discuss in community channels
- Tag maintainers for review


---

<!-- FILE: development/releasing.md -->

# Release Process

Mooncake uses [GoReleaser](https://goreleaser.com/) for automated releases.

## Creating a Release

1. **Tag the release:**
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```

2. **Automated process:**
   - GitHub Actions automatically triggers
   - Runs tests
   - Builds binaries for all platforms
   - Creates GitHub release with changelog
   - Uploads all artifacts

## What Gets Built

GoReleaser automatically builds for:
- **Linux**: amd64, arm64, arm, 386
- **macOS**: amd64 (Intel), arm64 (Apple Silicon)
- **Windows**: amd64, arm, 386

## What Gets Published

Each release includes:
- ✓ Compiled binaries for all platforms
- ✓ Archived releases (`.tar.gz` for Linux/macOS, `.zip` for Windows)
- ✓ Checksums file for verification
- ✓ Auto-generated changelog
- ✓ Release notes

## Versioning

Use semantic versioning:
- `v1.0.0` - Major release
- `v1.1.0` - Minor release (new features)
- `v1.0.1` - Patch release (bug fixes)

## Testing Locally

Test the release process without publishing:

```bash
# Install goreleaser
go install github.com/goreleaser/goreleaser@latest

# Test the build
goreleaser build --snapshot --clean

# Test the full release (doesn't publish)
goreleaser release --snapshot --clean
```

## Commit Message Format

For better changelogs, use conventional commits:
- `feat: add new feature` → Features section
- `fix: resolve bug` → Bug fixes section
- `docs: update readme` → Excluded from changelog
- `chore: update deps` → Excluded from changelog

## Example Release

```bash
# Make your changes
git add .
git commit -m "feat: add dry-run mode"

# Create release
git tag -a v1.2.0 -m "Release v1.2.0: Add dry-run mode"
git push origin v1.2.0

# Wait for GitHub Actions to complete
# Release will appear at: https://github.com/alehatsman/mooncake/releases
```

## Rollback

If a release has issues:

```bash
# Delete the tag locally
git tag -d v1.0.0

# Delete the tag remotely
git push origin :refs/tags/v1.0.0

# Delete the GitHub release manually from the web interface
```

## Optional: Homebrew

To publish to Homebrew, uncomment the `brews` section in `.goreleaser.yml` and create a tap repository.


---

<!-- FILE: development/roadmap.md -->

# Mooncake — Detailed Feature Checklist (Dependency-Ordered)

## 0) Foundations (must ship first)

### 0.1 Config model + schema
- [x] Define canonical internal structs:
  - [x] `RunConfig` (root) —  IMPLEMENTED: supports both old ([]Step) and new (version/vars/steps) formats with backward compatibility
  - [x] `Step` (union: exactly one action key) — exists but uses optional pointers, not explicit union type
  - [x] `Action` variants (shell/file/template/include/include_vars/vars/assert/...) — shell/file/template/include/include_vars/vars exist; assert missing
  - [x] `Common` fields: `name`, `tags[]`, `when`, `become`, `become_user`, `env`, `cwd`, `register`, `timeout`, `retries`, `retry_delay`, `changed_when`, `failed_when` —  ALL IMPLEMENTED in config.go
- [x] JSON Schema (or CUE → JSON Schema) for:
  - [x] root document — embedded in validator.go, 196 lines
  - [x] step union (`oneOf`) — implemented with oneOf + not constraints
  - [x] per-action payloads — template, file objects defined
  - [ ] expression strings (typed as string but tagged as `expr`) — no pattern validation for expressions
- [x] Schema constraints:
  - [x] exactly-one action key enforcement — oneOf with not conditions implemented
  - [x] forbid unknown fields (strict mode) — additionalProperties: false throughout
  - [x] validate `tags` format, `timeout` format, paths non-empty —  timeout/retry_delay duration pattern added: ^[0-9]+(ns|us|µs|ms|s|m|h)$; retries range 0-100; tags/paths still no constraints
- [x] YAML source mapping:
  - [x] parse with node position retention — location.go implements LocationMap with JSON pointer tracking
  - [x] map validation errors to `file:line:col` — diagnostic.go formats errors with file:line:col
  - [ ] include include-chain context: `A.yml -> B.yml -> C.yml:line:col` — only shows immediate file location
- [x] Template pre-validation:
  - [x] validate pongo2 syntax for any field marked templatable —  IMPLEMENTED in template_validator.go: validates when, shell, with_items, env vars, file paths, template src/dest, etc.
  - [x] surface template line/col + originating yaml path —  IMPLEMENTED: reports errors with file:line:col + field context
- [x] CLI:
  - [x] `mooncake validate --config ... --vars ...` —  IMPLEMENTED in cmd/mooncake.go: includes --config, --vars (optional), --format (text|json)
  - [x] exit codes: `0 ok`, `2 validation error`, `3 runtime error` —  IMPLEMENTED: proper exit codes in validateCommand()

### 0.2 Deterministic plan compiler
- [x] Plan IR types:
  - [x] `Plan` (ordered steps) —  IMPLEMENTED: /internal/plan/plan.go with Version, GeneratedAt, RootFile, Steps, InitialVars, Tags
  - [x] `PlanStep` fields:
    - [x] `id` (stable) —  IMPLEMENTED: sequential counter format (step-0001, step-0002, ...)
    - [x] `origin` (file, line, col, include stack) —  IMPLEMENTED: Origin struct with FilePath, Line, Column, IncludeChain
    - [x] `name_resolved` (post-template) —  IMPLEMENTED: stored as Name field in PlanStep
    - [x] `tags_effective` —  IMPLEMENTED: stored as Tags field in PlanStep
    - [x] `when_expr_resolved` (string) —  IMPLEMENTED: stored as When field in PlanStep (evaluated at runtime)
    - [x] `become_effective` —  IMPLEMENTED: stored as Become/BecomeUser fields in PlanStep
    - [x] `action` (compiled action payload) —  IMPLEMENTED: ActionPayload with Type and Data map
    - [x] `loop_context` (optional) —  IMPLEMENTED: LoopContext with Type, Item, Index, First, Last, LoopExpression
- [x] Include expansion:
  - [x] recursive includes —  IMPLEMENTED: expandInclude() in /internal/plan/planner.go
  - [x] relative path base = directory of including file —  IMPLEMENTED: uses pathutil.GetDirectoryOfFile()
  - [x] cycle detection with chain display —  IMPLEMENTED: seenFiles map tracks includes, formatIncludeChain() displays cycle path
- [x] Vars layering (deterministic precedence):
  - [x] CLI `--vars` (highest) — supported, loaded in executor
  - [x] include_vars — implemented in expandIncludeVars()
  - [x] config-local vars — vars step merges into ExpansionContext.Variables
  - [x] facts (read-only) — facts collected and merged globally
- [x] Loop expansion:
  - [x] `with_items`: expand to N steps; each has stable id suffix (`stepid[i]`) —  IMPLEMENTED: expandWithItems() generates sequential step IDs with LoopContext
  - [x] `with_filetree`: deterministic ordering (lexicographic path) —  IMPLEMENTED: expandWithFileTree() sorts items with sort.Slice for determinism
  - [x] loop vars: `item`, `index`, `first`, `last` —  IMPLEMENTED: all loop variables in LoopContext and merged into template variables
- [x] Tag filtering at plan stage:
  - [x] if `--tags` set, steps without matching tags are marked skipped (included in plan for visibility) —  IMPLEMENTED: compilePlanStep() marks Skipped=true for non-matching tags
  - [x] dry-run and run show identical step indices/ids — both use same plan generation
- [x] CLI:
  - [x] `mooncake plan --format json|yaml|text` —  IMPLEMENTED: /cmd/mooncake.go with formatters for all three formats
  - [x] `--show-origins` prints file:line:col per step —  IMPLEMENTED: text formatter includes origin with --show-origins flag
  - [x] `--output <file>` saves plan to file —  IMPLEMENTED: SavePlanToFile() in /internal/plan/io.go
  - [x] `mooncake run --from-plan <file>` —  IMPLEMENTED: executor.ExecutePlan() consumes saved plans
- [x] Tests:
  - [x] Comprehensive test coverage —  IMPLEMENTED: 15 tests in /internal/plan/planner_test.go covering all expansion types, error handling, determinism, cycle detection
- [x] Executor integration:
  - [x] ExecutePlan() and ExecutePlanStep() —  IMPLEMENTED: /internal/executor/executor.go consumes Plan IR
  - [x] Backward compatibility —  MAINTAINED: existing `run` command works alongside new plan command
  - [x] Code cleanup —  COMPLETED: removed ~170 lines of dead code (executeLoopStep, handleInclude, HandleWithItems, HandleWithFileTree) as loops/includes now handled at plan-time

### 0.3 Execution semantics (idempotency + check mode) 
- [x] Core step result model:
  - [x] statuses: `ok`, `changed`, `skipped`, `failed` —  IMPLEMENTED: Status() method returns string status; uses boolean fields (Failed, Changed, Skipped) with computed status
  - [x] timings: start/end/duration —  IMPLEMENTED: StartTime, EndTime, Duration fields tracked per-step; accessible as result.duration_ms in registered results
  - [x] stdout/stderr capture policy (bounded) — line-buffered via bufio.Scanner in shell_step.go
  - [x] register payload (structured) — Result.ToMap() converts to map[string]interface{}; accessible as result.stdout, result.rc, result.duration_ms, result.status, etc.
- [x] `--dry-run` (check-mode):
  - [x] identical plan + identical evaluators — same expression engine, same skip logic
  - [x] actions implement `Plan()` and `Apply()`:
    - [x] `Plan()` computes diff/intent, no side effects —  IMPLEMENTED: dry-run handlers render templates, compare content, detect changes without executing
    - [x] `Apply()` executes changes — handlers execute actual changes in non-dry-run mode
  - [x] dry-run prints `would_change` and reason —  ENHANCED: file/template operations distinguish create vs update vs no-change with size comparisons; shows content previews
- [x] Expression engine:
  - [x] `when` boolean expression — handleWhenExpression() in executor.go; uses expr-lang/expr library
  - [x] `changed_when` boolean expression based on action result —  IMPLEMENTED: evaluateResultOverrides() in shell_step.go:19-69; evaluates expression with result context
  - [x] `failed_when` boolean expression based on action result —  IMPLEMENTED: evaluateResultOverrides() in shell_step.go:19-69; overrides failure status based on expression
  - [x] type rules: missing var handling, nulls, strings/bools/numbers, map/list indexing — basic support via expr-lang; nil handling works
- [x] `shell` idempotency:
  - [x] `creates: <path>` → skip if exists —  IMPLEMENTED: config.Step.Creates field; checkIdempotencyConditions() in executor.go:132-169; supports template variables
  - [x] `unless: <command>` → run only if unless returns non-zero —  IMPLEMENTED: config.Step.Unless field; checkIdempotencyConditions() executes command silently; supports template variables
  - [x] `changed_when` override (default: changed if rc==0; or default changed=true; choose explicit contract) —  IMPLEMENTED: shell always sets Changed=true by default; overridable with changed_when expression
- [x] Retries:
  - [x] `retries: N` —  IMPLEMENTED: config.Step.Retries field; HandleShell() in shell_step.go:93-131 implements retry logic with max attempts
  - [x] `retry_delay: duration` —  IMPLEMENTED: config.Step.RetryDelay field; parses duration string and sleeps between retries
  - [x] retry on failure only unless configured —  IMPLEMENTED: retries only on command failure (non-zero exit code); logs retry attempts

### 0.4 Sudo / privilege escalation 
- [x] Input methods:
  - [x] `--ask-become-pass` / `-K` (prompt no-echo) —  IMPLEMENTED: InteractivePasswordProvider in security/password.go uses golang.org/x/term.ReadPassword
  - [x] `--sudo-pass-file` (0600) —  IMPLEMENTED: FilePasswordProvider validates 0600 permissions and file ownership
  - [x] `SUDO_ASKPASS` support (optional) —  IMPLEMENTED: EnvPasswordProvider executes SUDO_ASKPASS helper program as fallback
- [x] Security:
  - [x] forbid plaintext `--sudo-pass` by default (or warn + require explicit insecure flag) —  IMPLEMENTED: requires --insecure-sudo-pass flag; mutual exclusion validation; security warnings in CLI
  - [x] redact password in logs/events —  IMPLEMENTED: Redactor in security/redact.go; integrated into ExecutionContext; redacts all debug logs, stdout, stderr, dry-run output
- [x] Become implementation:
  - [x] Linux/macOS: `sudo -S` / askpass —  IMPLEMENTED: sudo -S in shell_step.go; SUDO_ASKPASS support via EnvPasswordProvider
  - [x] Platform detection —  IMPLEMENTED: IsBecomeSupported() in security/platform.go validates Linux/macOS support
  - [ ] Windows: explicit not supported or use `runas` (define scope) — not implemented; become operations explicitly fail on Windows
- [x] Per-step become:
  - [x] `become: true|false` —  IMPLEMENTED: fully functional for shell, file, template operations
  - [x] `become_user` (optional; linux/mac only) —  IMPLEMENTED: config.Step.BecomeUser field; supported in shell via sudo -u; file/template operations use chown
- [x] Extended become support:
  - [x] File operations —  IMPLEMENTED: createFileWithBecome() uses temp file + sudo move pattern
  - [x] Template operations —  IMPLEMENTED: template rendering respects become flag
  - [x] Directory operations —  IMPLEMENTED: createDirectoryWithBecome() uses sudo mkdir
- [x] Testing:
  - [x] Unit tests —  IMPLEMENTED: 26 tests in security/*_test.go (password providers, redaction, platform)
  - [x] Integration tests —  IMPLEMENTED: sudo_integration_test.go validates password resolution, redaction, file permissions, mutual exclusion

---

## 1) Core Engine UX / Observability 

### 1.1 Event stream + presentation  COMPLETED (2026-02-04)
- [x] JSON event schema:
  - [x] `run.started`, `plan.loaded`, `run.completed` —  IMPLEMENTED: Full run lifecycle events
  - [x] `step.started`, `step.completed`, `step.failed`, `step.skipped` —  IMPLEMENTED: Complete step lifecycle
  - [x] `step.stdout`, `step.stderr` —  IMPLEMENTED: Line-by-line output streaming with line numbers
  - [x] `file.created`, `file.updated`, `directory.created`, `template.rendered` —  IMPLEMENTED: File operation events
  - [x] `variables.set`, `variables.loaded` —  IMPLEMENTED: Variable lifecycle events
- [x] Event system architecture:
  - [x] Publisher/Subscriber pattern with async delivery —  IMPLEMENTED: Channel-based with 100-event buffer
  - [x] Non-blocking: < 1μs overhead per event —  VERIFIED: Performance tested
  - [x] Type-safe: Compile-time checks for event payloads —  IMPLEMENTED: Strongly-typed data structs
- [x] Console subscriber —  IMPLEMENTED: internal/logger/console_subscriber.go
  - [x] Text mode: maintains existing UX (icons, colors, indentation)
  - [x] JSON mode: structured JSONL event stream
- [x] TUI subscriber —  IMPLEMENTED: internal/logger/tui_subscriber.go
  - [x] Event-based: consumes events (not direct logger calls)
  - [x] Reuses existing buffer/display/animation infrastructure
  - [x] Same 150ms refresh rate maintained
- [x] `--output-format json|text` —  IMPLEMENTED: CLI flag in cmd/mooncake.go
- [x] `--log-level debug|info|warn|error` —  EXISTS: Already supported via existing logger
- [x] Output truncation rules:
  - [x] cap stdout/stderr per step (bytes + lines) —  IMPLEMENTED: --max-output-bytes and --max-output-lines flags
  - [x] store full output to artifacts dir optionally —  IMPLEMENTED: --capture-full-output flag

**Documentation**:

- [x] docs/EVENTS.md — Complete event system architecture guide
- [x] examples/json-output-example.md — Usage examples and integration patterns
- [x] Package documentation throughout codebase

**Testing**:

- [x] Unit tests: 6 tests in internal/events/publisher_test.go
- [x] Integration tests: 3 tests in internal/events/integration_test.go
- [x] All tests passing (100%)

### 1.2 Run artifacts  COMPLETED (2026-02-04)
- [x] Artifact writer implementation —  IMPLEMENTED: internal/artifacts/writer.go
- [x] Directory structure: `.mooncake/runs/<YYYYMMDD-HHMMSS-hash>/`
- [x] Write:
  - [x] `plan.json` —  IMPLEMENTED: Full plan with expanded steps
  - [x] `facts.json` —  IMPLEMENTED: System facts
  - [x] `summary.json` —  IMPLEMENTED: Run summary with stats
  - [x] `results.json` (per step) —  IMPLEMENTED: Step-by-step results
  - [x] `events.jsonl` —  IMPLEMENTED: Full JSONL event stream
  - [x] `diff.json` (changed files) —  IMPLEMENTED: List of created/modified files
  - [x] `stdout.log` / `stderr.log` (optional) —  IMPLEMENTED: Full output capture when enabled
- [x] Deterministic naming —  IMPLEMENTED: Timestamp + hash(root_file + hostname)
- [x] Stable machine-readable format —  IMPLEMENTED: JSON with pretty-printing
- [x] CLI integration:
  - [x] `--artifacts-dir` flag —  IMPLEMENTED: cmd/mooncake.go passes to executor.StartConfig
  - [x] `--capture-full-output` flag —  IMPLEMENTED: enables full stdout/stderr capture to artifacts
  - [x] `--max-output-bytes` / `--max-output-lines` —  IMPLEMENTED: configurable truncation limits (default: 1MB, 1000 lines)
  - [x] Default behavior when flags not specified —  IMPLEMENTED: artifacts only created when --artifacts-dir specified

---

## 2) File System Actions (detailed)

### 2.1 `file` action (expand into sub-modes)  COMPLETED (2026-02-04)
Define `file:` as a structured union.

#### 2.1.1 Ensure directory 
- [x] `file: { state: directory, path, mode?, owner?, group? }`
- [x] Idempotent:
  - [x] create if missing
  - [x] chmod/chown only if differs
- [x] Dry-run shows which attributes would change
- [x] Recursive option:
  - [x] `recurse: true` for mode/owner/group on tree (explicit)

#### 2.1.2 Ensure file (touch) 
- [x] `file: { state: touch, path, mode?, owner?, group? }`
- [x] Create empty if missing
- [x] Update metadata only if differs

#### 2.1.3 Remove path 
- [x] `file: { state: absent, path, force?: bool }`
- [x] Safety:
  - [x] refuse empty path
  - [x] refuse `/` unless `--i-accept-danger`
  - [ ] optional `allow_glob` — not implemented (use explicit paths)
- [x] Idempotent: ok if already absent

#### 2.1.4 Symlink 
- [x] `file: { state: link, src, dest, force?: bool }`
- [x] Behavior:
  - [x] create symlink if missing
  - [x] if dest exists and not link:
    - [x] fail unless `force: true` (then replace)
  - [x] if link points elsewhere:
    - [x] replace (counts as changed)
- [x] Windows:
  - [x] define behavior (requires admin or developer mode); if unsupported → explicit error

#### 2.1.5 Hardlink 
- [x] `file: { state: hardlink, src, dest, force?: bool }`

#### 2.1.6 Permissions-only / ownership-only operations 
- [x] `file: { state: perms, path, mode?, owner?, group?, recurse? }`

#### 2.1.7 Copy (separate `copy` action) 
Implemented as separate `copy` action:
- [x] `copy: { src, dest, mode?, owner?, group?, force?, backup?, checksum? }`
- [x] Preserve:
  - [ ] optionally preserve times — not yet implemented
  - [x] optionally preserve mode
- [x] Large files: stream copy, atomic write temp + rename

#### 2.1.8 Sync (separate `sync` action) — PLANNED
- [ ] `sync: { src, dest, delete?: bool, exclude?: [], checksum?: bool }`
- [ ] Implementation:
  - [ ] prefer native `rsync` if present else Go copy-tree
  - [ ] deterministic ordering

**Status**: Phase 2 of 6-week file operations plan

### 2.2 `template` action — PARTIALLY COMPLETE
- [x] `template: { src, dest, mode?, owner?, group?, backup? }` — basic implementation exists
- [ ] Features:
  - [x] atomic write: render → temp file → diff → rename — implemented
  - [x] change detection via content hash — implemented
  - [ ] optional `newline: lf|crlf` — not implemented
- [x] Template validation pre-run (Phase 0) — implemented in 0.1

### 2.3 `unarchive` action  COMPLETED (2026-02-05)
- [x] `unarchive: { src, dest, strip_components?, creates?, mode? }`
- [x] Supported:
  - [x] `.tar`, `.tar.gz`, `.tgz`, `.zip`
- [x] Idempotent:
  - [x] if `creates` exists → skip
- [x] Safety:
  - [x] prevent path traversal (`../`) entries using pathutil.ValidateNoPathTraversal() and SafeJoin()
  - [x] validate symlink targets don't escape destination
  - [x] block absolute paths in archive entries
- [x] Implementation:
  - [x] Automatic format detection from file extension
  - [x] Strip N leading path components (like tar --strip-components)
  - [x] Preserve file permissions from archive
  - [x] Custom directory permissions via mode parameter
  - [x] Event emission (archive.extracted) with extraction stats
  - [x] Dry-run support
  - [x] Variable rendering in all paths (src, dest, creates)
  - [x] Result registration support
- [x] Testing:
  - [x] 17 comprehensive tests covering validation, extraction, security, idempotency
  - [x] Security tests for path traversal attacks
  - [x] All archive formats tested (tar, tar.gz, tgz, zip)

**Status**: Phase 3 of 6-week file operations plan  COMPLETE

### 2.4 `download` action  COMPLETED (2026-02-05)
- [x] `download: { url, dest, checksum?, mode?, timeout?, retries?, headers?, force?, backup? }`
- [x] Implemented:
  - [x] HTTP/HTTPS downloads with atomic writes (temp → verify → rename)
  - [x] SHA256/MD5 checksum verification
  - [x] Idempotent: skips download if destination exists with matching checksum
  - [x] Timeout configuration (e.g., "30s", "5m")
  - [x] Retry logic with configurable attempts
  - [x] Custom HTTP headers (Authorization, User-Agent, etc.)
  - [x] Backup existing files before overwriting (backup: true)
  - [x] File permissions (mode parameter)
  - [x] Template rendering in URL and destination paths
  - [x] Dry-run support with change detection
  - [x] Event emission (file.downloaded) with download stats
  - [x] Result registration support
- [ ] Optional features (not implemented):
  - [ ] Resume capability (HTTP Range headers)
  - [ ] ETag/If-Modified-Since support
- [x] Testing:
  - [x] Build verification: successful compilation
  - [x] Integration tests: downloads work correctly
  - [x] Idempotency tests: verifies checksum-based skipping
- [x] Documentation:
  - [x] Complete API reference in docs/download-action.md
  - [x] Example configurations in examples/download-example.yml
  - [x] Schema validation in internal/config/schema.json

**Status**: Phase 3 of 6-week file operations plan  COMPLETE

---

## 3) Process Actions

### 3.1 `shell` action (structured)  COMPLETED (2026-02-05)
- [x] `shell: { cmd, interpreter?: "bash"|"sh"|"pwsh"|"cmd", stdin?, capture?: bool }` —  IMPLEMENTED: ShellAction struct with all fields
- [x] Prefer `exec.Command` without shell when `argv` provided:
  - [x] allow `command: { argv: ["git","clone",...], stdin?, capture?: bool }` as safer alternative —  IMPLEMENTED: CommandAction with direct exec
- [x] Backward compatibility: simple string `shell: "command"` still works via custom UnmarshalYAML
- [x] Interpreter selection:
  - [x] Supports "bash", "sh", "pwsh", "cmd"
  - [x] Platform-specific defaults (bash on Unix, pwsh on Windows)
- [x] Quoting rules documented in actions.md
- [x] Exit code handling:
  - [x] `rc` always captured — already implemented in shell_step.go
  - [x] `failed_when` overrides rc logic — already implemented
- [x] Streaming output events:
  - [x] emit stdout/stderr chunks for TUI — already implemented (EventStepStdout, EventStepStderr)
- [x] stdin support:
  - [x] Both shell and command actions support stdin field
  - [x] Template rendering for stdin content
  - [x] Works with sudo (password + stdin combined)
- [x] capture flag:
  - [x] Optional bool field to disable output capture (streaming only)

**Note:** env, cwd, timeout remain at Step level for consistency across all actions

**Status**: Complete implementation of structured shell and command actions

---

## 4) Service Management (`systemd` / launchd / Windows)  COMPLETED (2026-02-05)

### 4.1 `systemd` action (Linux) 
- [x] `service: { name, state?: started|stopped|restarted|reloaded, enabled?: bool, daemon_reload?: bool }` —  IMPLEMENTED: ServiceAction with full lifecycle control
- [x] Unit file management:
  - [x] `service: { unit: { dest: "/etc/systemd/system/<name>.service", src_template?: ..., content?: ... } }` —  IMPLEMENTED: ServiceUnit struct with template rendering
  - [x] `dropin:` support:
    - [x] `dropin: { name: "10-override.conf", content?, src_template? }` —  IMPLEMENTED: ServiceDropin writes to `/etc/systemd/system/<name>.service.d/`
- [x] Environment directives via drop-in:
  - [x] `Environment=K=V` lines —  DOCUMENTED: users can add via content/src_template
  - [x] `EnvironmentFile=/etc/<...>` option —  DOCUMENTED: users can configure in unit content
- [x] Common directives supported in templates (user-provided content):
  - [x] `[Unit] After=`, `Wants=`, `Requires=` —  DOCUMENTED
  - [x] `[Service] ExecStart=`, `WorkingDirectory=`, `User=`, `Group=` —  DOCUMENTED
  - [x] `[Service] Environment=`, `EnvironmentFile=` —  DOCUMENTED
  - [x] `[Service] Restart=`, `RestartSec=`, `TimeoutStartSec=` —  DOCUMENTED
  - [x] `[Install] WantedBy=multi-user.target` —  DOCUMENTED
- [x] Verification:
  - [x] `systemctl is-active`, `is-enabled` state checks —  IMPLEMENTED: idempotency checks before state changes
- [x] Idempotent:
  - [x] only `daemon-reload` when unit/dropin changed —  IMPLEMENTED: content-based change detection with checksums
- [x] Implementation:
  - [x] Platform detection (systemd on Linux)
  - [x] Sudo/become support for all operations
  - [x] Template rendering in all fields
  - [x] Event emission (EventServiceManaged)
  - [x] Dry-run support with change preview
  - [x] Result registration support
  - [x] Custom error types (StepValidationError for invalid params)
- [x] Testing:
  - [x] 18 comprehensive tests with platform detection
  - [x] Unit file creation (inline and template)
  - [x] Drop-in configuration tests
  - [x] State management tests
  - [x] Idempotency verification
  - [x] All tests passing with proper platform skipping

### 4.2 `launchd` action (macOS) 
- [x] `service: { name, state?: started|stopped|restarted|reloaded, enabled?: bool }` —  IMPLEMENTED: unified service interface
- [x] Plist file management:
  - [x] `service: { unit: { dest?, content?, src_template?, mode? } }` —  IMPLEMENTED: XML plist creation with template rendering
- [x] Paths:
  - [x] user agents: `~/Library/LaunchAgents` —  IMPLEMENTED: automatic path detection based on become flag
  - [x] system daemons: `/Library/LaunchDaemons` (requires sudo) —  IMPLEMENTED: domain selection (gui/<uid> vs system)
- [x] `launchctl bootstrap/bootout` support —  IMPLEMENTED: full launchctl integration
  - [x] `bootstrap` for loading services
  - [x] `bootout` for unloading services
  - [x] `kickstart` for starting/restarting
  - [x] `kill` for stopping services
- [x] Common plist properties documented:
  - [x] `Label`, `ProgramArguments` —  DOCUMENTED: required fields
  - [x] `RunAtLoad`, `KeepAlive` —  DOCUMENTED: auto-start configuration
  - [x] `StartCalendarInterval` —  DOCUMENTED: scheduled tasks (cron-like)
  - [x] `EnvironmentVariables` —  DOCUMENTED: environment configuration
  - [x] `StandardOutPath`, `StandardErrorPath` —  DOCUMENTED: logging
  - [x] `WorkingDirectory`, `UserName`, `GroupName` —  DOCUMENTED: execution context
- [x] Idempotent:
  - [x] plist content-based change detection —  IMPLEMENTED: checksums prevent unnecessary updates
  - [x] service state checks before operations —  IMPLEMENTED
- [x] Implementation:
  - [x] Platform detection (launchd on macOS)
  - [x] Domain selection (user vs system)
  - [x] Sudo support for system daemons
  - [x] Template rendering with Jinja2-like syntax
  - [x] Idempotency through content comparison
  - [x] Event emission
  - [x] Dry-run support
- [x] Testing:
  - [x] 7 comprehensive tests with platform detection
  - [x] Domain selection tests
  - [x] Plist creation (inline and template)
  - [x] Service lifecycle tests
  - [x] All tests passing on macOS

### 4.3 Windows service action (future)
- [ ] `service: { name, state, start_mode }` — PLACEHOLDER: not yet implemented
- [ ] PowerShell integration (`Set-Service`, `Start-Service`)
- [ ] Service configuration management

**Documentation**:

- [x] Complete actions guide (docs/guide/config/actions.md) — Service section with systemd/launchd examples
- [x] Property reference (docs/guide/config/reference.md) — Detailed property tables
- [x] macOS service examples (examples/macos-services/) — 3 comprehensive YAML examples
- [x] macOS services README (examples/macos-services/README.md) — 440+ lines with patterns and troubleshooting
- [x] Schema validation (internal/config/schema.json) — Full service action schema
- [x] Updated changelog (docs/about/changelog.md)

**Status**: Phase 4 complete for Linux (systemd) and macOS (launchd) 

---

## 5) Assertions / Verification (first-class)

### 5.1 `assert` action (union)
- [ ] `assert: { command: "...", rc?: 0, stdout_contains?: "...", stdout_regex?: "...", timeout_s? }`
- [ ] `assert: { file: { path, exists?: bool, mode?: "0644", owner?: "...", group?: "...", sha256?: "..." } }`
- [ ] `assert: { http: { url, method?: GET|POST, status?: 200, jsonpath?: "...", equals?: any, timeout_s? } }`
- [ ] Result:
  - [ ] never “changed”
  - [ ] fail with precise mismatch diff

---

## 6) Facts (structured, immutable)  COMPLETED (2026-02-05)

### 6.1 Facts collection 
- [x] `facts` run once per execution (cached) —  IMPLEMENTED: internal/facts/cache.go uses sync.Once for per-process caching
- [x] OS:
  - [x] `os.name`, `os.version`, `kernel`, `arch` —  IMPLEMENTED: OS, Arch, Hostname, Username, UserHome, Distribution, DistributionVersion, DistributionMajor, KernelVersion
- [x] CPU:
  - [x] model, cores, flags (AVX etc) —  IMPLEMENTED: CPUCores, CPUModel, CPUFlags[] with AVX, AVX2, SSE4_2, FMA detection
- [x] Memory:
  - [x] total, free, swap total/free —  IMPLEMENTED: MemoryTotalMB, MemoryFreeMB, SwapTotalMB, SwapFreeMB
- [x] Disk:
  - [x] mounts, fs type, size/free —  IMPLEMENTED: Disks[] with Device, MountPoint, Filesystem, SizeGB, UsedGB, AvailGB, UsedPct
- [x] Network:
  - [x] interfaces, default route, DNS —  IMPLEMENTED: IPAddresses[], NetworkInterfaces[] (Name, MACAddress, MTU, Addresses, Up), DefaultGateway, DNSServers[]
- [x] GPU (NVIDIA/AMD/Intel/Apple):
  - [x] `gpu.count`, `gpu.model[]`, `gpu.driver_version`, `gpu.cuda_version` —  IMPLEMENTED: GPUs[] with Vendor, Model, Memory, Driver, CUDAVersion (supports nvidia-smi, rocm-smi, lspci, system_profiler)
- [x] Toolchain probes (optional):
  - [x] `docker.version`, `git.version`, `python.version`, `go.version` —  IMPLEMENTED: DockerVersion, GitVersion, GoVersion, PythonVersion
- [x] Package manager detection —  IMPLEMENTED: PackageManager field (apt, dnf, yum, pacman, zypper, apk, brew, port)

### 6.2 CLI 
- [x] `mooncake facts --format json|text` —  IMPLEMENTED: cmd/mooncake.go:207 with JSON and text output formats
- [x] `--facts-json <path>` emit during run —  IMPLEMENTED: cmd/mooncake.go:512 flag writes facts during run command

**Implementation**:

- [x] internal/facts/facts.go — Core Facts struct and collection logic
- [x] internal/facts/cache.go — Per-process caching with sync.Once
- [x] internal/facts/linux.go — Full Linux support (524 lines)
- [x] internal/facts/darwin.go — Full macOS support (360 lines)
- [x] internal/facts/toolchains.go — Cross-platform toolchain detection
- [x] internal/facts/windows.go — Minimal Windows stubs (27 lines)

**Platform Support**:  Linux (full) |  macOS (full) |  Windows (minimal stubs)

**Testing**:

- [x] Unit tests in internal/facts/*_test.go
- [x] Platform-specific tests with proper skipping
- [x] All tests passing

---

## 7) ML Adoption Modules (after foundations)

### 7.1 `ollama` action/module  COMPLETED (2026-02-05)
- [x] `ollama: { state: present|absent, service?: bool, method?: auto|script|package, host?, models_dir?, pull?: [], force?: bool, env?: {} }` —  IMPLEMENTED: OllamaAction with all fields
- [x] Install:
  - [x] Linux/macOS installer strategy —  IMPLEMENTED: Auto-detection of package managers (apt, dnf, yum, pacman, zypper, apk, brew), official script fallback
  - [x] Installation methods: auto (prefer package manager, fallback to script), script (official installer only), package (package manager only)
- [x] Service:
  - [x] systemd drop-in for env vars (`OLLAMA_HOST`, `OLLAMA_MODELS`, `OLLAMA_DEBUG`) —  IMPLEMENTED: Creates `/etc/systemd/system/ollama.service.d/10-mooncake.conf` with environment variables
  - [x] launchd plist for macOS —  IMPLEMENTED: Creates plist at user or system domain with environment variables
- [x] Model pull:
  - [x] `ollama: { pull: ["llama3.1:8b", ...] }` —  IMPLEMENTED: Pulls models idempotently, force flag for re-pull
- [x] Healthcheck:
  - [x] Integration with `assert.http` to `/api/tags` —  DOCUMENTED: Example in ollama-example.yml
- [x] Facts emitted:
  - [x] `ollama_version`, `ollama_endpoint`, `ollama_models[]` —  IMPLEMENTED: Auto-collected facts in internal/facts/toolchains.go
- [x] Implementation:
  - [x] internal/executor/ollama_step.go — 700 lines of installation, service, and model management logic
  - [x] Configuration structs in internal/config/config.go
  - [x] JSON schema validation in internal/config/schema.json
  - [x] Event emission (EventOllamaManaged)
  - [x] Dry-run support with detailed operation logging
  - [x] Result registration support
  - [x] Sudo/become support for system-wide installation
- [x] Testing:
  - [x] 13 comprehensive test functions in internal/executor/ollama_step_test.go
  - [x] All tests passing (validation, dry-run, service, models, uninstall, idempotency)
- [x] Documentation:
  - [x] Complete actions guide section (docs/guide/config/actions.md) — 250+ lines with examples
  - [x] Property reference (docs/guide/config/reference.md) — Full property table
  - [x] Core concepts update (docs/guide/core-concepts.md)
  - [x] Comprehensive examples (examples/ollama-example.yml) — 260+ lines covering all use cases
- [x] Platform Support: Linux (systemd + package managers)  | macOS (launchd + Homebrew) 

**Status**: Complete Ollama action implementation with installation, service management, and model pulling

### 7.2 Container runtime
- [ ] `docker: { state: present, version_pin? }`
- [ ] `nvidia_container_toolkit: { state: present }`
- [ ] Optional: `apptainer: { state: present }`

### 7.3 Python env
- [ ] `uv: { state: present, version_pin?, cache_dir? }`
- [ ] `micromamba: { state: present, root_prefix?, envs_dir? }`
- [ ] `python_env: { backend: uv|micromamba, name, spec: pyproject|requirements|env_yml }`

---

## 8) Safety rails (needed before “yolo” ideas)

### 8.1 Dangerous ops gating
- [ ] Global allow/deny lists for:
  - [ ] `shell` commands (pattern match)
  - [ ] file deletes outside workspace
- [ ] Require explicit confirmation flags:
  - [ ] deleting `/`, modifying boot configs, driver reinstall, etc.
- [ ] Redaction:
  - [ ] mark vars as secret → never print in logs/events/artifacts

---

## 9) Detailed CLI checklist
- [x] `mooncake run --config ... --vars ... --tags ... --dry-run`
- [x] `mooncake plan --config ... --format json|yaml`
- [x] `mooncake validate --config ...`
- [x] `mooncake facts --format json|text`  IMPLEMENTED
- [ ] `mooncake doctor` (later)

---

## 10) Cross-platform policy (explicit scope)
- [ ] Define per-action availability matrix:
  - [ ] Linux/macOS/Windows support per action
- [ ] For unsupported:
  - [ ] fail at validation/plan-time with actionable message



---

<!-- FILE: examples/01-hello-world.md -->

# 01 - Hello World

**Start here!** This is the simplest possible Mooncake configuration.

## What You'll Learn

- Running basic shell commands
- Using global system variables
- Multi-line shell commands

## Quick Start

```bash
cd examples/01-hello-world
mooncake run --config config.yml
```

## What It Does

1. Prints a hello message
2. Runs system commands to show OS info
3. Uses Mooncake's global variables to display OS and architecture

## Key Concepts

### Shell Commands

Execute commands with the `shell` action:
```yaml
- name: Print message
  shell: echo "Hello!"
```

### Multi-line Commands

Use `|` for multiple commands:
```yaml
- name: Multiple commands
  shell: |
    echo "First command"
    echo "Second command"
```

### Global Variables

Mooncake automatically provides system information:
- `{{os}}` - Operating system (linux, darwin, windows)
- `{{arch}}` - Architecture (amd64, arm64, etc.)

## Output Example

```
▶ Print hello message
Hello from Mooncake!
✓ Print hello message

▶ Print system info
OS: Darwin
Arch: arm64
✓ Print system info

▶ Show global variables
Running on darwin/arm64
✓ Show global variables
```

## Next Steps

Continue to [02-variables-and-facts](02-variables-and-facts.md) to learn about custom variables and all available system facts.


---

<!-- FILE: examples/02-variables-and-facts.md -->

# 02 - Variables and System Facts

Learn how to define custom variables and use Mooncake's comprehensive system facts.

## What You'll Learn

- Defining custom variables with `vars`
- Using all available system facts
- Combining custom variables with system facts
- Using variables in file operations

## Quick Start

```bash
cd examples/02-variables-and-facts
mooncake run --config config.yml
```

## What It Does

1. Defines custom application variables
2. Displays all system facts (OS, hardware, network, software)
3. Creates files using both custom variables and system facts

## Key Concepts

### Custom Variables

Define your own variables:
```yaml
- vars:
    app_name: MyApp
    version: "1.0.0"
    environment: development
```

Use them in commands and paths:
```yaml
- shell: echo "Running {{app_name}} v{{version}}"
```

### System Facts

Mooncake automatically collects system information:

**Basic:**
- `os` - Operating system (linux, darwin, windows)
- `arch` - Architecture (amd64, arm64)
- `hostname` - System hostname
- `user_home` - User's home directory

**Hardware:**
- `cpu_cores` - Number of CPU cores
- `memory_total_mb` - Total RAM in megabytes

**Distribution:**
- `distribution` - Distribution name (ubuntu, debian, macos, etc.)
- `distribution_version` - Full version (e.g., "22.04")
- `distribution_major` - Major version number

**Software:**
- `package_manager` - Detected package manager (apt, yum, brew, etc.)
- `python_version` - Installed Python version

**Network:**
- `ip_addresses` - Array of IP addresses
- `ip_addresses_string` - Comma-separated IP addresses

### Variable Substitution

Variables work everywhere:
```yaml
- file:
    path: "/tmp/{{app_name}}-{{version}}-{{os}}"
    state: directory
```

## Seeing All Facts

Run `mooncake facts` to see all facts for your system:
```bash
mooncake facts
```

You can also output facts as JSON:
```bash
mooncake facts --format json
```

## Next Steps

Continue to [03-files-and-directories](03-files-and-directories.md) to learn about file operations.


---

<!-- FILE: examples/03-files-and-directories.md -->

# 03 - Files and Directories

Learn how to create and manage files and directories with Mooncake.

## What You'll Learn

- Creating directories with `state: directory`
- Creating files with `state: file`
- Setting file permissions with `mode`
- Adding content to files

## Quick Start

```bash
cd examples/03-files-and-directories
mooncake run --config config.yml
```

## What It Does

1. Creates application directory structure
2. Creates files with specific content
3. Sets appropriate permissions (755 for directories, 644 for files)
4. Creates executable scripts

## Key Concepts

### Creating Directories

```yaml
- name: Create directory
  file:
    path: /tmp/myapp
    state: directory
    mode: "0755"  # rwxr-xr-x
```

### Creating Empty Files

```yaml
- name: Create empty file
  file:
    path: /tmp/file.txt
    state: file
    mode: "0644"  # rw-r--r--
```

### Creating Files with Content

```yaml
- name: Create config file
  file:
    path: /tmp/config.txt
    state: file
    content: |
      Line 1
      Line 2
    mode: "0644"
```

### File Permissions

Use octal notation in quotes:
- `"0644"` - rw-r--r-- (readable by all, writable by owner)
- `"0755"` - rwxr-xr-x (executable by all, writable by owner)
- `"0600"` - rw------- (only owner can read/write)

### Using Variables

```yaml
- vars:
    app_dir: /tmp/myapp

- file:
    path: "{{app_dir}}/config"
    state: directory
```

## Permission Examples

| Mode | Meaning | Use Case |
|------|---------|----------|
| 0755 | rwxr-xr-x | Directories, executable scripts |
| 0644 | rw-r--r-- | Regular files, configs |
| 0600 | rw------- | Private files, secrets |
| 0700 | rwx------ | Private directories |

## Next Steps

Continue to [04-conditionals](04-conditionals.md) to learn about conditional execution.


---

<!-- FILE: examples/04-conditionals.md -->

# 04 - Conditionals

Learn how to conditionally execute steps based on system properties or variables.

## What You'll Learn

- Using `when` for conditional execution
- OS and architecture detection
- Complex conditions with logical operators
- Combining conditionals with tags

## Quick Start

```bash
cd examples/04-conditionals

# Run all steps (only matching conditions will execute)
mooncake run --config config.yml

# Run only dev-tagged steps
mooncake run --config config.yml --tags dev
```

## What It Does

1. Demonstrates steps that always run
2. Shows OS-specific steps (macOS vs Linux)
3. Shows architecture-specific steps
4. Demonstrates tag filtering

## Key Concepts

### Basic Conditionals

Use `when` to conditionally execute steps:
```yaml
- name: Linux only
  shell: echo "Running on Linux"
  when: os == "linux"
```

### Available System Variables

- `os` - darwin, linux, windows
- `arch` - amd64, arm64, 386, etc.
- `distribution` - ubuntu, debian, centos, macos, etc.
- `distribution_major` - major version number
- `package_manager` - apt, yum, brew, pacman, etc.

### Comparison Operators

- `==` - equals
- `!=` - not equals
- `>`, `<`, `>=`, `<=` - comparisons
- `&&` - logical AND
- `||` - logical OR
- `!` - logical NOT

### Complex Conditions

```yaml
- name: ARM Mac only
  shell: echo "ARM-based macOS"
  when: os == "darwin" && arch == "arm64"

- name: High memory systems
  shell: echo "Lots of RAM!"
  when: memory_total_mb >= 16000

- name: Ubuntu 20+
  shell: apt update
  when: distribution == "ubuntu" && distribution_major >= "20"
```

### Tags vs Conditionals

**Conditionals (`when`):**
- Evaluated at runtime
- Based on system facts or variables
- Step-level decision making

**Tags:**
- User-controlled filtering
- Specified via CLI `--tags` flag
- Workflow-level decision making

## Testing Different Conditions

Try these commands:
```bash
# See which steps run on your system
mooncake run --config config.yml

# Preview without executing
mooncake run --config config.yml --dry-run

# Run only development steps
mooncake run --config config.yml --tags dev
```

## Next Steps

Continue to [05-templates](05-templates.md) to learn about template rendering.


---

<!-- FILE: examples/05-templates.md -->

# 05 - Templates

Learn how to render configuration files from templates using pongo2 syntax.

## What You'll Learn

- Rendering `.j2` template files
- Using variables in templates
- Template conditionals (`{% if %}`)
- Template loops (`{% for %}`)
- Passing additional vars to templates

## Quick Start

```bash
cd examples/05-templates
mooncake run --config config.yml
```

Check the rendered files:
```bash
ls -lh /tmp/mooncake-templates/
cat /tmp/mooncake-templates/config.yml
```

## What It Does

1. Defines variables for application, server, and database config
2. Renders application config with loops and conditionals
3. Renders nginx config with optional SSL
4. Creates executable script from template
5. Renders same template with different variables

## Key Concepts

### Template Action

```yaml
- name: Render config
  template:
    src: ./templates/config.yml.j2
    dest: /tmp/config.yml
    mode: "0644"
```

### Template Syntax (pongo2)

**Variables:**
```jinja
{{ variable_name }}
{{ nested.property }}
```

**Conditionals:**
```jinja
{% if debug %}
  debug: true
{% else %}
  debug: false
{% endif %}
```

**Loops:**
```jinja
{% for item in items %}
  - {{ item }}
{% endfor %}
```

**Filters:**
```jinja
{{ path | expanduser }}  # Expands ~ to home directory
{{ text | upper }}       # Convert to uppercase
```

### Passing Additional Vars

Override variables for specific templates:
```yaml
- template:
    src: ./templates/config.yml.j2
    dest: /tmp/prod-config.yml
    vars:
      environment: production
      debug: false
```

## Template Files

### config.yml.j2
Application configuration with:
- Conditional debug settings
- Loops over features list
- Variable substitution

### nginx.conf.j2
Web server config with:
- Conditional SSL configuration
- Dynamic port and paths

### script.sh.j2
Executable shell script with:
- Shebang line
- Variable expansion
- Command loops

## Common Use Cases

- **Config files** - app.yml, nginx.conf, etc.
- **Shell scripts** - deployment scripts, setup scripts
- **Systemd units** - service files
- **Dotfiles** - .bashrc, .vimrc with customization

## Testing Templates

```bash
# Render templates
mooncake run --config config.yml

# View rendered output
cat /tmp/mooncake-templates/config.yml

# Check executable permissions
ls -la /tmp/mooncake-templates/deploy.sh
```

## Next Steps

Continue to [06-loops](06-loops.md) to learn about iterating over lists and files.


---

<!-- FILE: examples/06-loops.md -->

# 06 - Loops

Learn how to iterate over lists and files to avoid repetition.

## What You'll Learn

- Iterating over lists with `with_items`
- Iterating over files with `with_filetree`
- Using loop variables: `{{ item }}`, `{{ index }}`, `{{ first }}`, `{{ last }}`
- Accessing file properties in loops

## Quick Start

```bash
cd examples/06-loops

# Run list iteration example
mooncake run --config with-items.yml

# Run file tree iteration example
mooncake run --config with-filetree/config.yml
```

## Examples Included

### 1. with-items.yml - List Iteration

Iterate over lists of items:
```yaml
- vars:
    packages:
      - neovim
      - ripgrep
      - fzf

- name: Install package
  shell: brew install {{ item }}
  with_items: "{{ packages }}"
```

**What it does:**
- Defines lists in variables
- Installs multiple packages
- Creates directories for multiple users
- Creates user-specific config files

### 2. with-filetree/ - File Tree Iteration

Iterate over files in a directory:
```yaml
- name: Copy dotfile
  shell: cp "{{ item.src }}" "/tmp/backup/{{ item.name }}"
  with_filetree: ./files
```

**What it does:**
- Iterates over files in `./files/` directory
- Copies dotfiles to backup location
- Filters directories vs files
- Displays file properties

## Key Concepts

### List Iteration (with_items)

```yaml
- vars:
    users: [alice, bob, charlie]

- name: Create user directory
  file:
    path: "/home/{{ item }}"
    state: directory
  with_items: "{{ users }}"
```

This creates:
- `/home/alice`
- `/home/bob`
- `/home/charlie`

### File Tree Iteration (with_filetree)

```yaml
- name: Process file
  shell: echo "Processing {{ item.name }}"
  with_filetree: ./dotfiles
  when: item.is_dir == false
```

**Available properties:**
- `item.src` - Full source path
- `item.name` - File name
- `item.is_dir` - Boolean, true if directory

### Loop Variables

Both `with_items` and `with_filetree` provide additional loop variables:

```yaml
- vars:
    packages: [git, vim, tmux]

- name: "Installing {{index + 1}}/{{packages|length}}: {{item}}"
  shell: |
    echo "Package: {{item}}"
    echo "Index: {{index}}"
    echo "First: {{first}}"
    echo "Last: {{last}}"
    brew install {{item}}
  with_items: "{{packages}}"
```

**Available loop variables:**
- `{{ item }}` - Current item (for `with_items`) or file object (for `with_filetree`)
- `{{ index }}` - Zero-based iteration index (0, 1, 2, ...)
- `{{ first }}` - Boolean, true for first iteration
- `{{ last }}` - Boolean, true for last iteration

**Example use cases:**
```yaml
# Progress counter
- name: "[{{index + 1}}/3] Processing {{item}}"
  shell: process {{item}}
  with_items: [a, b, c]

# First-only setup
- name: Initialize on first item
  shell: mkdir -p /tmp/output
  with_items: "{{files}}"
  when: first == true

# Last-only cleanup
- name: Cleanup after last item
  shell: echo "All done!"
  with_items: "{{files}}"
  when: last == true
```

### Filtering in Loops

Skip directories:
```yaml
- name: Copy files only
  shell: cp "{{ item.src }}" "/tmp/{{ item.name }}"
  with_filetree: ./files
  when: item.is_dir == false
```

## Real-World Use Cases

**with_items:**
- Installing multiple packages
- Creating multiple users/groups
- Setting up multiple services
- Deploying to multiple servers

**with_filetree:**
- Managing dotfiles
- Deploying configuration directories
- Backing up files
- Processing file collections

## Testing

```bash
# List iteration
mooncake run --config with-items.yml

# Check created files
ls -la /tmp/users/

# File tree iteration
mooncake run --config with-filetree/config.yml

# Check backed up files
ls -la /tmp/dotfiles-backup/
```

## Next Steps

Continue to [07-register](07-register.md) to learn about capturing command output.


---

<!-- FILE: examples/07-register.md -->

# 07 - Register

Learn how to capture command output and use it in subsequent steps.

## What You'll Learn

- Capturing output with `register`
- Accessing stdout, stderr, and return codes
- Using captured data in conditionals
- Detecting if operations made changes

## Quick Start

```bash
cd examples/07-register
mooncake run --config config.yml
```

## What It Does

1. Checks if git is installed and captures the result
2. Uses return code to conditionally show messages
3. Captures username and uses it in file paths
4. Captures OS version and displays it
5. Detects if file operations made changes

## Key Concepts

### Basic Registration

```yaml
- name: Check if git exists
  shell: which git
  register: git_check

- name: Use the result
  shell: echo "Git is at {{ git_check.stdout }}"
  when: git_check.rc == 0
```

### Available Fields

After registering a result, you can access:

**For shell commands:**
- `register_name.stdout` - Standard output
- `register_name.stderr` - Standard error
- `register_name.rc` - Return/exit code (0 = success)
- `register_name.failed` - Boolean, true if rc != 0
- `register_name.changed` - Boolean, always true for shell

**For file operations:**
- `register_name.rc` - 0 for success, 1 for failure
- `register_name.failed` - Boolean, true if operation failed
- `register_name.changed` - Boolean, true if file created/modified

**For template operations:**
- `register_name.rc` - 0 for success, 1 for failure
- `register_name.failed` - Boolean, true if rendering failed
- `register_name.changed` - Boolean, true if output file changed

### Using in Conditionals

Check return codes:
```yaml
- shell: test -f /tmp/file.txt
  register: file_check

- shell: echo "File exists"
  when: file_check.rc == 0

- shell: echo "File not found"
  when: file_check.rc != 0
```

### Using in Templates

Use captured data anywhere:
```yaml
- shell: whoami
  register: current_user

- file:
    path: "/tmp/{{ current_user.stdout }}_config.txt"
    state: file
    content: "User: {{ current_user.stdout }}"
```

### Change Detection

Know if operations actually changed something:
```yaml
- file:
    path: /tmp/test.txt
    state: file
    content: "test"
  register: result

- shell: echo "File was created or modified"
  when: result.changed == true
```

## Common Patterns

### Checking for Command Existence

```yaml
- shell: which docker
  register: docker_check

- shell: echo "Docker not installed"
  when: docker_check.rc != 0
```

### Conditional Installation

```yaml
- shell: python3 --version
  register: python_check

- shell: apt install python3
  become: true
  when: python_check.rc != 0
```

### Using Command Output

```yaml
- shell: hostname
  register: host

- shell: echo "Running on {{ host.stdout }}"
```

## Testing

```bash
# Run the example
mooncake run --config config.yml

# Check created file
cat /tmp/$(whoami)_config.txt
```

## Next Steps

Continue to [08-tags](08-tags.md) to learn about filtering execution with tags.


---

<!-- FILE: examples/08-tags.md -->

# 08 - Tags

Learn how to use tags to selectively run parts of your configuration.

## What You'll Learn

- Adding tags to steps
- Filtering execution with `--tags` flag
- Organizing workflows with tags
- Combining tags with conditionals

## Quick Start

```bash
cd examples/08-tags

# Run all steps (no tag filter)
mooncake run --config config.yml

# Run only development steps
mooncake run --config config.yml --tags dev

# Run only production steps
mooncake run --config config.yml --tags prod

# Run test-related steps
mooncake run --config config.yml --tags test

# Run multiple tag categories
mooncake run --config config.yml --tags dev,test
```

## What It Does

Demonstrates different tagged workflows:
- Development setup
- Production deployment
- Testing
- Security audits
- Staging deployment

## Key Concepts

### Adding Tags

```yaml
- name: Install dev tools
  shell: echo "Installing tools"
  tags:
    - dev
    - tools
```

### Tag Filtering Behavior

**No tags specified:**
- All steps run (including untagged steps)

**Tags specified (`--tags dev`):**
- Only steps with matching tags run
- Untagged steps are skipped

**Multiple tags (`--tags dev,prod`):**
- Steps run if they have ANY of the specified tags
- OR logic: matches `dev` OR `prod`

### Tag Organization Strategies

**By Environment:**
```yaml
tags: [dev, staging, prod]
```

**By Phase:**
```yaml
tags: [setup, deploy, test, cleanup]
```

**By Component:**
```yaml
tags: [database, webserver, cache]
```

**By Role:**
```yaml
tags: [install, configure, security]
```

### Multiple Tags Per Step

Steps can have multiple tags:
```yaml
- name: Security audit
  shell: run-security-scan
  tags:
    - test
    - prod
    - security
```

This runs with:
- `--tags test` ✓
- `--tags prod` ✓
- `--tags security` ✓
- `--tags dev` ✗

## Real-World Examples

### Development Workflow

```bash
# Install dev tools only
mooncake run --config config.yml --tags dev,tools

# Run tests
mooncake run --config config.yml --tags test
```

### Production Deployment

```bash
# Deploy to production
mooncake run --config config.yml --tags prod,deploy

# Run security checks
mooncake run --config config.yml --tags security,prod
```

### Staging Environment

```bash
# Deploy to staging
mooncake run --config config.yml --tags staging,deploy
```

## Combining Tags and Conditionals

```yaml
- name: Install Linux dev tools
  shell: apt install build-essential
  become: true
  when: os == "linux"
  tags:
    - dev
    - tools
```

Both must match:
1. Condition must be true (`os == "linux"`)
2. Tag must match (if `--tags` specified)

## Testing Different Tag Filters

```bash
# Preview what runs with dev tag
mooncake run --config config.yml --tags dev --dry-run

# Run dev and test steps
mooncake run --config config.yml --tags dev,test

# Run only setup steps
mooncake run --config config.yml --tags setup
```

## Best Practices

1. **Use consistent naming** - Pick a scheme (env, phase, role) and stick to it
2. **Multiple tags per step** - Makes filtering more flexible
3. **Document your tags** - In README or comments
4. **Combine with conditionals** - For environment + OS filtering

## Next Steps

Continue to [09-sudo](09-sudo.md) to learn about privilege escalation.


---

<!-- FILE: examples/09-sudo.md -->

# 09 - Sudo / Privilege Escalation

Learn how to execute commands and operations with elevated privileges.

## What You'll Learn

- Using `become: true` for sudo operations
- Providing sudo password via CLI
- System-level operations
- OS-specific privileged operations

## Quick Start

```bash
cd examples/09-sudo

# Requires sudo password
mooncake run --config config.yml --sudo-pass <your-password>

# Preview what would run with sudo
mooncake run --config config.yml --sudo-pass <password> --dry-run
```

 **Warning:** This example contains commands that require root privileges. Review the config before running!

## What It Does

1. Runs regular command (no sudo)
2. Runs privileged command with sudo
3. Updates package list (Linux)
4. Installs system packages
5. Creates system directories and files

## Key Concepts

### Basic Sudo

Add `become: true` to run with sudo:
```yaml
- name: System operation
  shell: apt update
  become: true
```

### Providing Password

Three ways to provide sudo password:

**1. Command line (recommended):**
```bash
mooncake run --config config.yml --sudo-pass mypassword
```

**2. Environment variable:**
```bash
export MOONCAKE_SUDO_PASS=mypassword
mooncake run --config config.yml
```

**3. Interactive prompt:**
Some systems may prompt automatically (if configured)

### Which Operations Need Sudo?

**Typically require sudo:**
- Package management (`apt`, `yum`, `dnf`)
- System file operations (`/etc`, `/opt`, `/usr/local`)
- Service management (`systemctl`)
- User/group management
- Mounting filesystems
- Network configuration

**Don't require sudo:**
- User-space operations
- Home directory files
- `/tmp` directory
- Homebrew on macOS (usually)

### File Operations with Sudo

Create system directories:
```yaml
- name: Create system directory
  file:
    path: /opt/myapp
    state: directory
    mode: "0755"
  become: true
```

Create system files:
```yaml
- name: Create system config
  file:
    path: /etc/myapp/config.yml
    state: file
    content: "config: value"
  become: true
```

### OS-Specific Sudo

```yaml
# Linux package management
- name: Install package (Linux)
  shell: apt install -y curl
  become: true
  when: os == "linux" and package_manager == "apt"

# macOS typically doesn't need sudo for homebrew
- name: Install package (macOS)
  shell: brew install curl
  when: os == "darwin"
```

## Security Considerations

1. **Review before running** - Check what commands will execute with sudo
2. **Use dry-run** - Preview with `--dry-run` first
3. **Minimize sudo usage** - Only use on steps that require it
4. **Specific commands** - Don't use `become: true` on untrusted commands
5. **Password handling** - Be careful with password in shell history

## Common Use Cases

### Package Installation

```yaml
- name: Install system packages
  shell: |
    apt update
    apt install -y nginx postgresql
  become: true
  when: os == "linux"
```

### System Service Setup

```yaml
- name: Create systemd service
  template:
    src: ./myapp.service.j2
    dest: /etc/systemd/system/myapp.service
    mode: "0644"
  become: true

- name: Enable service
  shell: systemctl enable myapp
  become: true
```

### System Directory Setup

```yaml
- name: Create application directories
  file:
    path: "{{ item }}"
    state: directory
    mode: "0755"
  become: true
  with_items:
    - /opt/myapp
    - /etc/myapp
    - /var/log/myapp
```

## Testing

```bash
# Preview what will run with sudo
mooncake run --config config.yml --sudo-pass test --dry-run

# Run with sudo
mooncake run --config config.yml --sudo-pass <password>

# Check created system files
ls -la /opt/myapp/
```

## Troubleshooting

**"sudo: no tty present"**
- Make sure to provide `--sudo-pass` flag

**Permission denied without sudo**
- Add `become: true` to the step

**Command not found**
- Check if command exists: `which <command>`
- Some commands need full paths with sudo

## Next Steps

Continue to [10-multi-file-configs](10-multi-file-configs.md) to learn about organizing large configurations.


---

<!-- FILE: examples/10-multi-file-configs.md -->

# 10 - Multi-File Configurations

Learn how to organize large configurations into multiple files.

## What You'll Learn

- Splitting configuration into multiple files
- Using `include` to load other configs
- Using `include_vars` to load variables
- Organizing by environment (dev/prod)
- Organizing by platform (Linux/macOS)
- Relative path resolution

## Quick Start

```bash
cd examples/10-multi-file-configs

# Run with development environment (default)
mooncake run --config main.yml

# Run with specific tags
mooncake run --config main.yml --tags install
mooncake run --config main.yml --tags dev
```

## Directory Structure

```
10-multi-file-configs/
├── main.yml              # Entry point
├── tasks/                # Modular task files
│   ├── common.yml        # Common setup
│   ├── linux.yml         # Linux-specific
│   ├── macos.yml         # macOS-specific
│   └── dev-tools.yml     # Development tools
└── vars/                 # Environment variables
    ├── development.yml   # Dev settings
    └── production.yml    # Prod settings
```

## What It Does

1. Sets project variables
2. Loads environment-specific variables
3. Runs common setup tasks
4. Runs OS-specific tasks (Linux or macOS)
5. Conditionally runs dev tools setup

## Key Concepts

### Entry Point (main.yml)

The main file orchestrates everything:
```yaml
- vars:
    project_name: MyProject
    env: development

- name: Load environment variables
  include_vars: ./vars/{{env}}.yml

- name: Setup common configuration
  include: ./tasks/common.yml

- name: Setup OS-specific configuration
  include: ./tasks/macos.yml
  when: os == "darwin"
```

### Including Variable Files

Load variables from external YAML:
```yaml
- name: Load development vars
  include_vars: ./vars/development.yml
```

**vars/development.yml:**
```yaml
debug: true
port: 8080
database_host: localhost
```

### Including Task Files

Load and execute tasks from other files:
```yaml
- name: Run common setup
  include: ./tasks/common.yml
```

**tasks/common.yml:**
```yaml
- name: Create project directory
  file:
    path: /tmp/{{project_name}}
    state: directory
```

### Relative Path Resolution

Paths are relative to the **current file**, not the working directory:

```
main.yml:
  include: ./tasks/common.yml  # Relative to main.yml

tasks/common.yml:
  template:
    src: ./templates/config.j2  # Relative to common.yml, not main.yml
```

### Organization Strategies

**By Environment:**
```
vars/
  development.yml
  staging.yml
  production.yml
```

**By Platform:**
```
tasks/
  linux.yml
  macos.yml
  windows.yml
```

**By Component:**
```
tasks/
  database.yml
  webserver.yml
  cache.yml
```

**By Phase:**
```
tasks/
  00-prepare.yml
  01-install.yml
  02-configure.yml
  03-deploy.yml
```

## Real-World Example

### Project Structure
```
my-project/
├── setup.yml              # Main entry
├── environments/
│   ├── dev.yml
│   ├── staging.yml
│   └── prod.yml
├── platforms/
│   ├── linux.yml
│   └── macos.yml
├── components/
│   ├── postgres.yml
│   ├── nginx.yml
│   └── app.yml
└── templates/
    ├── nginx.conf.j2
    └── app-config.yml.j2
```

### Main File
```yaml
# setup.yml
- vars:
    environment: "{{ lookup('env', 'ENVIRONMENT') or 'dev' }}"

- include_vars: ./environments/{{ environment }}.yml

- include: ./platforms/{{ os }}.yml

- include: ./components/postgres.yml
- include: ./components/nginx.yml
- include: ./components/app.yml
```

## Switching Environments

**Method 1: Modify main.yml**
```yaml
- vars:
    env: production  # Change this
```

**Method 2: Use environment variable**
```bash
ENVIRONMENT=production mooncake run --config main.yml
```

**Method 3: Different main files**
```bash
mooncake run --config prod-setup.yml
```

## Benefits of Multi-File Organization

1. **Maintainability** - Easier to find and update specific parts
2. **Reusability** - Share tasks across projects
3. **Collaboration** - Team members can work on different files
4. **Testing** - Test components independently
5. **Clarity** - Clear separation of concerns

## Testing

```bash
# Run full configuration
mooncake run --config main.yml

# Preview what will run
mooncake run --config main.yml --dry-run

# Run with debug logging to see includes
mooncake run --config main.yml --log-level debug

# Run specific tagged sections
mooncake run --config main.yml --tags install
```

## Best Practices

1. **Clear naming** - Use descriptive file names
2. **Logical grouping** - Group related tasks together
3. **Document includes** - Comment what each include does
4. **Avoid deep nesting** - Keep include hierarchy shallow (2-3 levels max)
5. **Use variables** - Make includes reusable with variables

## Next Steps

Explore the [real-world dotfiles example](real-world-dotfiles.md) to see a complete practical application!


---

<!-- FILE: examples/11-execution-control.md -->

# Example 11: Shell Execution Control

Advanced execution control for shell commands with timeouts, retries, and custom result evaluation.

**Note:** These features are specific to shell commands. File and template operations don't support timeout, retries, env, cwd, changed_when, or failed_when fields.

## Timeouts

Prevent commands from running too long:

```yaml
- name: Command with timeout
  shell: ./long-running-script.sh
  timeout: 30s

- name: Build with timeout
  shell: make build
  timeout: 10m
  cwd: /opt/project
```

Timeout exit code is 124 (standard timeout exit code).

## Retries and Delays

Automatically retry failed commands:

```yaml
- name: Download file with retries
  shell: curl -O https://example.com/file.tar.gz
  retries: 5
  retry_delay: 10s

- name: Wait for service
  shell: nc -z localhost 8080
  retries: 10
  retry_delay: 2s
  failed_when: "result.rc != 0"
```

## Environment Variables

Set custom environment variables:

```yaml
- name: Build with custom environment
  shell: make build
  env:
    CC: gcc-11
    CFLAGS: "-O2 -Wall"
    MAKEFLAGS: "-j4"

- name: Run tests with env
  shell: npm test
  env:
    NODE_ENV: test
    DEBUG: "app:*"
```

### Template Variables in Env

```yaml
- vars:
    build_type: release
    num_cores: 4

- name: Compile with variables
  shell: cmake --build .
  env:
    BUILD_TYPE: "{{build_type}}"
    CMAKE_BUILD_PARALLEL_LEVEL: "{{num_cores}}"
```

## Working Directory

Change directory before execution:

```yaml
- name: Build in project directory
  shell: npm install && npm run build
  cwd: /opt/myproject

- name: Run tests from subdir
  shell: pytest tests/
  cwd: "{{project_root}}/backend"
```

## Custom Change Detection

Override whether a step reports as changed:

```yaml
- name: Git pull (only changed if updated)
  shell: git pull
  register: git_result
  changed_when: "'Already up to date' not in result.stdout"

- name: Restart if config changed
  shell: systemctl restart nginx
  become: true
  when: config.changed == true
```

### Always/Never Changed

```yaml
- name: Read-only command (never changed)
  shell: cat /etc/os-release
  changed_when: false

- name: Force changed status
  shell: echo "notify handler"
  changed_when: true
```

## Custom Failure Detection

Override when a command is considered failed:

```yaml
- name: Grep (0=found, 1=not found, 2+=error)
  shell: grep "pattern" file.txt
  failed_when: "result.rc >= 2"

- name: Command with acceptable exit codes
  shell: ./script.sh
  failed_when: "result.rc not in [0, 2, 3]"

- name: Check stderr for errors
  shell: ./noisy-command.sh
  failed_when: "'ERROR' in result.stderr or 'FATAL' in result.stderr"
```

## Privilege Escalation

Run as different users:

```yaml
- name: Run as root
  shell: systemctl restart nginx
  become: true

- name: Run as postgres user
  shell: psql -c "SELECT version()"
  become: true
  become_user: postgres

- name: Run as application user
  shell: ./manage.py migrate
  become: true
  become_user: appuser
  cwd: /opt/application
```

## Complete Example: Robust Deployment

```yaml
- name: Stop application
  shell: systemctl stop myapp
  become: true
  timeout: 30s

- name: Backup current version
  shell: |
    backup_file="/backup/myapp-$(date +%Y%m%d-%H%M%S).tar.gz"
    tar czf "$backup_file" /opt/myapp
    echo "Backed up to $backup_file"
  timeout: 5m
  register: backup_result

- name: Download new version
  shell: curl -o /tmp/myapp.tar.gz https://releases.example.com/myapp-{{version}}.tar.gz
  timeout: 10m
  retries: 3
  retry_delay: 30s

- name: Extract application
  shell: |
    rm -rf /opt/myapp
    tar xzf /tmp/myapp.tar.gz -C /opt
  become: true
  timeout: 2m

- name: Install dependencies
  shell: pip install -r requirements.txt
  cwd: /opt/myapp
  become: true
  become_user: appuser
  timeout: 5m
  env:
    PIP_INDEX_URL: "{{pip_mirror}}"

- name: Run database migrations
  shell: ./manage.py migrate
  cwd: /opt/myapp
  become: true
  become_user: appuser
  timeout: 10m
  register: migrate_result
  changed_when: "'No migrations to apply' not in result.stdout"
  failed_when: "result.rc != 0"

- name: Start application
  shell: systemctl start myapp
  become: true
  timeout: 30s

- name: Wait for application to be ready
  shell: curl -sf http://localhost:8080/health
  retries: 30
  retry_delay: 2s
  register: health_check
  failed_when: "result.rc != 0"

- name: Verify deployment
  shell: |
    version=$(curl -s http://localhost:8080/version)
    echo "Deployed version: $version"
    test "$version" = "{{expected_version}}"
  register: verify
  failed_when: "result.rc != 0"
```

## Real-World: Service Health Check

```yaml
- name: Check service dependencies
  shell: |
    services="postgresql redis nginx"
    for service in $services; do
      systemctl is-active $service || exit 1
    done
  retries: 5
  retry_delay: 10s
  timeout: 5s
  register: deps_check

- name: Start application service
  shell: systemctl start myapp
  become: true
  when: deps_check.rc == 0

- name: Wait for service to be ready
  shell: curl -sf http://localhost:8080/ready
  retries: 60
  retry_delay: 1s
  timeout: 5s
  register: ready_check
  failed_when: "result.rc != 0"
  changed_when: false  # Health check doesn't change anything

- name: Run smoke tests
  shell: ./run-smoke-tests.sh
  cwd: /opt/myapp/tests
  timeout: 2m
  env:
    TEST_URL: http://localhost:8080
    TEST_ENV: staging
  register: smoke_tests
  failed_when: "result.rc != 0 or 'FAIL' in result.stdout"
```

## See Also

- [Actions Reference](../guide/config/actions.md#universal-fields) - Complete field documentation
- [Advanced Configuration](../guide/config/advanced.md#error-handling) - Error handling patterns
- [Example 07: Register](07-register.md) - Using command results


---

<!-- FILE: examples/12-unarchive.md -->

# Unarchive - Extract Archive Files

Learn how to extract archive files with automatic format detection and security protections.

## What You'll Learn

- Extracting tar, tar.gz, tgz, and zip archives
- Using `strip_components` to remove leading directories
- Idempotency with `creates` parameter
- Handling different archive formats
- Security protections against path traversal

## Quick Start

```bash
cd examples/12-unarchive
mooncake run --config config.yml
```

## What It Does

1. Downloads sample archives (or uses provided ones)
2. Extracts various archive formats
3. Demonstrates path stripping
4. Shows idempotent extraction
5. Extracts to system directories with sudo

## Key Concepts

### Basic Extraction

Extract an archive to a destination directory:

```yaml
- name: Extract Node.js
  unarchive:
    src: /tmp/node-v20.tar.gz
    dest: /opt/node
    mode: "0755"
```

The destination directory is created if it doesn't exist.

### Supported Formats

Mooncake automatically detects the archive format from the file extension:

| Format | Extensions | Compression |
|--------|-----------|-------------|
| tar | `.tar` | None |
| tar.gz | `.tar.gz`, `.tgz` | Gzip |
| zip | `.zip` | ZIP compression |

Detection is case-insensitive (`.TAR`, `.TGZ`, `.ZIP` all work).

### Strip Components

Remove leading directory levels from extracted paths:

```yaml
# Archive contains: node-v20/bin/node, node-v20/lib/...
- name: Extract without top-level directory
  unarchive:
    src: /tmp/node-v20.tar.gz
    dest: /opt/node
    strip_components: 1
    # Result: /opt/node/bin/node, /opt/node/lib/...
```

**How it works:**

```
Archive structure:
  project-1.0/src/main.go
  project-1.0/src/utils.go
  project-1.0/README.md

strip_components: 0 (default) → dest/project-1.0/src/main.go
strip_components: 1           → dest/src/main.go
strip_components: 2           → dest/main.go
```

Files with fewer path components than specified are skipped.

### Idempotency with Creates

Skip extraction if a marker file already exists:

```yaml
- name: Extract application
  unarchive:
    src: /tmp/myapp.tar.gz
    dest: /opt/myapp
    creates: /opt/myapp/bin/myapp
    mode: "0755"
```

On subsequent runs, if `/opt/myapp/bin/myapp` exists, extraction is skipped. This prevents unnecessary re-extraction and maintains idempotency.

### Custom Directory Permissions

Set permissions for created directories:

```yaml
- name: Extract with custom permissions
  unarchive:
    src: /tmp/app.tar.gz
    dest: /opt/myapp
    mode: "0700"  # rwx------
```

File permissions are preserved from the archive. The `mode` parameter only affects directories created during extraction.

### Extract with Privilege Escalation

Extract to system directories using sudo:

```yaml
- name: Extract to system directory
  unarchive:
    src: /tmp/app.tar.gz
    dest: /opt/myapp
    strip_components: 1
    mode: "0755"
  become: true
```

### Using Variables

Use template variables in all paths:

```yaml
- vars:
    app_version: "1.2.3"
    install_dir: "/opt/myapp"

- name: Extract versioned release
  unarchive:
    src: "/tmp/app-{{app_version}}.tar.gz"
    dest: "{{install_dir}}"
    creates: "{{install_dir}}/bin/app"
    strip_components: 1
```

### Extract Multiple Archives

Use loops to extract multiple archives:

```yaml
- vars:
    packages:
      - name: app
        file: app-v1.2.3.tar.gz
        strip: 1
      - name: data
        file: data.zip
        strip: 0

- name: Extract {{item.name}}
  unarchive:
    src: /tmp/{{item.file}}
    dest: /opt/{{item.name}}
    strip_components: "{{item.strip}}"
    creates: /opt/{{item.name}}/.installed
  with_items: "{{packages}}"
```

## Security Features

Mooncake automatically protects against path traversal attacks:

### Blocked Patterns

These malicious patterns are automatically blocked:

```yaml
#  Path traversal with ../
Archive entry: ../../../etc/passwd

#  Absolute paths
Archive entry: /etc/passwd

#  Traversal in nested paths
Archive entry: legit/../../sensitive

#  Symlinks escaping destination
Symlink target: ../../../etc/shadow
```

All extracted paths are validated to ensure they stay within the destination directory.

### Security Guarantees

1. **Path Traversal Protection**: All entries with `../` are rejected
2. **Absolute Path Blocking**: Absolute paths are not allowed
3. **Symlink Validation**: Symlink targets are checked for escapes
4. **Safe Joining**: Uses `pathutil.SafeJoin()` for all paths

These protections are always active and cannot be disabled.

## Complete Example

Here's a complete example showing common patterns:

```yaml
version: "1.0"

vars:
  node_version: "20.11.0"
  install_dir: "/opt/node"
  backup_dir: "/var/backups"

steps:
  # Download archive if needed
  - name: Download Node.js
    shell: "curl -fsSL https://nodejs.org/dist/v{{node_version}}/node-v{{node_version}}-linux-x64.tar.gz -o /tmp/node.tar.gz"
    creates: "/tmp/node.tar.gz"

  # Extract with strip_components
  - name: Extract Node.js
    unarchive:
      src: "/tmp/node.tar.gz"
      dest: "{{install_dir}}"
      strip_components: 1
      creates: "{{install_dir}}/bin/node"
      mode: "0755"
    register: node_extracted

  # Verify installation
  - name: Check Node.js version
    shell: "{{install_dir}}/bin/node --version"
    when: node_extracted.changed

  # Extract ZIP archive
  - name: Extract application data
    unarchive:
      src: "/tmp/app-data.zip"
      dest: "{{install_dir}}/data"
      mode: "0755"

  # Extract backup with sudo
  - name: Restore system backup
    unarchive:
      src: "{{backup_dir}}/system-backup.tar.gz"
      dest: "/etc/myapp"
      creates: "/etc/myapp/.restored"
      mode: "0755"
    become: true
```

## Common Use Cases

### Software Installation

Extract and install precompiled binaries:

```yaml
- name: Install Go
  unarchive:
    src: /tmp/go1.21.linux-amd64.tar.gz
    dest: /usr/local
    creates: /usr/local/go/bin/go
  become: true
```

### Application Deployment

Deploy application releases:

```yaml
- name: Deploy application
  unarchive:
    src: /tmp/myapp-{{version}}.tar.gz
    dest: /opt/myapp
    strip_components: 1
    mode: "0755"
  become: true

- name: Create version marker
  file:
    path: /opt/myapp/.version
    content: "{{version}}"
    state: file
  become: true
```

### Backup Restoration

Restore from tar backups:

```yaml
- name: Restore user data
  unarchive:
    src: /backups/user-data-{{date}}.tar.gz
    dest: /home/{{username}}
    creates: /home/{{username}}/.restored
```

### Multi-platform Distribution

Extract platform-specific archives:

```yaml
- name: Extract platform binary
  unarchive:
    src: "/tmp/app-{{os}}-{{arch}}.tar.gz"
    dest: /opt/app
    strip_components: 1
    creates: /opt/app/bin/app
```

## Real-World Example

Complete Node.js installation workflow:

```yaml
version: "1.0"

vars:
  node_version: "20.11.0"
  node_base_url: "https://nodejs.org/dist"
  install_dir: "/opt/node"

steps:
  - name: Detect platform
    shell: "uname -s | tr '[:upper:]' '[:lower:]'"
    register: platform_result

  - name: Detect architecture
    shell: "uname -m"
    register: arch_result

  - name: Set Node.js archive name
    vars:
      platform_map:
        linux: "linux"
        darwin: "darwin"
      arch_map:
        x86_64: "x64"
        aarch64: "arm64"
        arm64: "arm64"
      platform: "{{platform_result.stdout}}"
      arch: "{{arch_result.stdout}}"
      node_platform: "{{platform_map[platform]}}"
      node_arch: "{{arch_map[arch]}}"
      archive_name: "node-v{{node_version}}-{{node_platform}}-{{node_arch}}.tar.gz"

  - name: Download Node.js
    shell: "curl -fsSL {{node_base_url}}/v{{node_version}}/{{archive_name}} -o /tmp/node.tar.gz"
    creates: "/tmp/node.tar.gz"
    timeout: 10m
    retries: 3
    retry_delay: 30s

  - name: Extract Node.js
    unarchive:
      src: "/tmp/node.tar.gz"
      dest: "{{install_dir}}"
      strip_components: 1
      creates: "{{install_dir}}/bin/node"
      mode: "0755"
    become: true
    register: node_install

  - name: Create symlinks
    shell: |
      ln -sf {{install_dir}}/bin/node /usr/local/bin/node
      ln -sf {{install_dir}}/bin/npm /usr/local/bin/npm
      ln -sf {{install_dir}}/bin/npx /usr/local/bin/npx
    when: node_install.changed
    become: true

  - name: Verify installation
    shell: "node --version && npm --version"
    register: versions

  - name: Show installed versions
    shell: "echo 'Node.js installed: {{versions.stdout}}'"
```

## See Also

- [File Operations](03-files-and-directories.md) - File and directory management
- [Loops](06-loops.md) - Iterating over multiple items
- [Sudo](09-sudo.md) - Privilege escalation
- [Actions Reference](../guide/config/actions.md#unarchive) - Complete action documentation
- [Configuration Reference](../guide/config/reference.md#unarchive) - Property reference


---

<!-- FILE: examples/actions/README.md -->

# Mooncake Actions - Comprehensive Examples

This directory contains extensive examples for every Mooncake action, demonstrating both basic and advanced usage patterns.

## Overview

Each file focuses on a single action type with 20-50+ examples covering:
- Basic usage
- Advanced features
- Real-world scenarios
- Error handling
- Best practices

## Quick Start

Run any example file:
```bash
cd examples/actions
mooncake run --config shell.yml
mooncake run --config file.yml --tags basics
```

## Available Actions

### Core Actions

| File | Action | Description | Examples |
|------|--------|-------------|----------|
| **[shell.yml](shell.yml)** | `shell` | Execute shell commands | 50+ examples |
| **[print.yml](print.yml)** | `print` | Print messages | 60+ examples |
| **[vars.yml](vars.yml)** | `vars` | Define variables | 36+ examples |

### File Operations

| File | Action | Description | Examples |
|------|--------|-------------|----------|
| **[file.yml](file.yml)** | `file` | Create/manage files and directories | 50+ examples |
| **[copy.yml](copy.yml)** | `copy` | Copy files with verification | 23+ examples |
| **[template.yml](template.yml)** | `template` | Render Jinja2 templates | 27+ examples |

### Network Operations

| File | Action | Description | Examples |
|------|--------|-------------|----------|
| **[download.yml](download.yml)** | `download` | Download files from URLs | 26+ examples |
| **[unarchive.yml](unarchive.yml)** | `unarchive` | Extract archives (.tar, .zip, etc.) | 25+ examples |

### System Management

| File | Action | Description | Examples |
|------|--------|-------------|----------|
| **[service.yml](service.yml)** | `service` | Manage systemd/launchd services | 24+ examples |

### Validation & Control

| File | Action | Description | Examples |
|------|--------|-------------|----------|
| **[assert.yml](assert.yml)** | `assert` | Verify system state | 48+ examples |
| **[include.yml](include.yml)** | `include` | Load tasks from files | 23+ examples |

### Advanced

| File | Action | Description | Examples |
|------|--------|-------------|----------|
| **[preset.yml](preset.yml)** | `preset` | Use reusable workflows | 20+ examples |

## Running Examples

### Run All Examples
```bash
mooncake run --config shell.yml
```

### Run Specific Tags
```bash
mooncake run --config shell.yml --tags basics
mooncake run --config file.yml --tags permissions
mooncake run --config template.yml --tags real-world
```

### Run with Cleanup
```bash
mooncake run --config file.yml --tags cleanup
```

## Action Details

### shell.yml - Shell Commands
Execute commands with full shell capabilities:
- Basic commands and multi-line scripts
- Output capture with `register`
- Environment variables
- Working directory changes
- Timeouts and retries
- Custom failure/change conditions
- Different shell interpreters
- stdin input

**Example:**
```yaml
- name: Complex deployment
  shell: |
    echo "Deploying..."
    npm install
    npm run build
  env:
    NODE_ENV: production
  cwd: /opt/app
  timeout: 10m
  retries: 3
  register: deploy_result
```

### print.yml - Print Messages
Simple message output without shell:
- Basic messages
- Variable interpolation
- Multi-line output
- Conditional printing
- Loops
- Debug messages
- Progress indicators
- Formatted output

**Example:**
```yaml
- name: Deployment status
  print: |
    Deployed {{ app_name }} v{{ version }}
    Status: Complete
    Platform: {{ os }}/{{ arch }}
```

### file.yml - File Management
Create and manage files, directories, and links:
- Create files with content
- Create directories (nested)
- Set permissions (0644, 0755, 0600, etc.)
- Set ownership (owner/group)
- Create symlinks and hardlinks
- Remove files/directories
- Touch files (update timestamp)
- Recursive operations
- Backups

**Example:**
```yaml
- name: Create application config
  file:
    path: /opt/app/config.yml
    state: file
    content: |
      app: {{ app_name }}
      port: {{ port }}
    mode: "0644"
```

### copy.yml - Copy Files
Copy files with integrity verification:
- Simple file copy
- Copy with permissions
- Backup before overwrite
- Force overwrite
- Checksum verification
- Loops for multiple files

**Example:**
```yaml
- name: Deploy configuration
  copy:
    src: ./configs/production.yml
    dest: /opt/app/config.yml
    mode: "0600"
    backup: true
```

### template.yml - Template Rendering
Render Jinja2 templates with variables:
- Basic template rendering
- Variables and system facts
- Conditionals and loops
- Filters (upper, lower, default, etc.)
- Executable script generation
- Configuration files
- Service definitions

**Example:**
```yaml
- name: Render nginx config
  template:
    src: ./templates/nginx.conf.j2
    dest: /etc/nginx/nginx.conf
    mode: "0644"
    vars:
      port: 80
      server_name: example.com
```

### download.yml - Download Files
Download from URLs with retry support:
- Simple downloads
- Checksum verification (SHA256/MD5)
- Timeouts and retries
- Custom headers
- Authentication
- Force re-download
- Backups
- Integration with unarchive

**Example:**
```yaml
- name: Download Node.js
  download:
    url: "https://nodejs.org/dist/v20.11.0/node-v20.11.0-linux-x64.tar.gz"
    dest: "/tmp/node.tar.gz"
    checksum: "SHA256_HERE"
    timeout: "5m"
    retries: 3
```

### unarchive.yml - Extract Archives
Extract .tar, .tar.gz, .tgz, .zip files:
- Basic extraction
- Strip path components
- Idempotency with markers
- Permission management
- Security features (path traversal protection)
- Integration with download

**Example:**
```yaml
- name: Extract application
  unarchive:
    src: /tmp/app.tar.gz
    dest: /opt/app
    strip_components: 1
    creates: /opt/app/.installed
    mode: "0755"
```

### service.yml - Service Management
Manage systemd (Linux) and launchd (macOS) services:
- Start/stop/restart services
- Enable/disable on boot
- Create service unit files
- Drop-in configurations
- Environment variables
- Dependencies
- Resource limits
- Timer units (scheduled tasks)

**Example:**
```yaml
- name: Deploy application service
  service:
    name: myapp
    unit:
      content: |
        [Unit]
        Description=My Application
        After=network.target

        [Service]
        Type=simple
        ExecStart=/opt/app/bin/server
        Restart=on-failure

        [Install]
        WantedBy=multi-user.target
    daemon_reload: true
    state: started
    enabled: true
  become: true
```

### assert.yml - Assertions
Verify system state (never changes, fails fast):
- Command assertions (exit codes)
- File assertions (exists, content, permissions)
- HTTP assertions (status, response body)
- Prerequisites checking
- Deployment validation
- Security checks
- Assertions with retries

**Example:**
```yaml
- name: Verify deployment
  assert:
    file:
      path: /opt/app/binary
      exists: true
      mode: "0755"

- name: Check health endpoint
  assert:
    http:
      url: http://localhost:8080/health
      status: 200
      contains: "healthy"
```

### include.yml - Include Tasks
Load and execute tasks from external files:
- Basic includes
- Conditional includes
- Include with tags
- Multi-level includes
- Environment-specific includes
- Reusable task libraries
- Platform-specific includes
- Modular configuration patterns

**Example:**
```yaml
- name: Run prerequisites
  include: ./tasks/prerequisites.yml

- name: Deploy application
  include: ./tasks/deploy.yml

- name: Run Linux tasks
  include: ./tasks/linux.yml
  when: os == "linux"
```

### preset.yml - Presets
Use reusable, parameterized workflows:
- Basic preset invocation
- Ollama preset (install/configure LLMs)
- Parameters and variables
- Conditional execution
- Registration
- Integration patterns
- Custom preset creation

**Example:**
```yaml
- name: Setup Ollama with models
  preset: ollama
  with:
    state: present
    service: true
    pull:
      - "llama3.1:8b"
      - "mistral:latest"
  become: true
```

### vars.yml - Variables
Define and manage variables:
- Simple variables
- Different types (string, number, boolean, list, dict)
- Nested structures
- System facts
- Conditional variables
- Default values
- Configuration management
- Multi-environment patterns

**Example:**
```yaml
- vars:
    app_name: "MyApp"
    version: "1.0.0"
    database:
      host: localhost
      port: 5432
      name: myapp_db

- name: Use variables
  print: "Deploying {{ app_name }} v{{ version }}"
```

## Tags Reference

Common tags used across examples:

- `basics` - Fundamental usage
- `advanced` - Complex scenarios
- `loops` - Using with_items
- `conditional` - Conditional execution
- `register` - Output capture
- `variables` - Variable usage
- `real-world` - Practical scenarios
- `best-practices` - Recommended patterns
- `cleanup` - Cleanup operations
- `always` - Always runs

## Tips

1. **Start with basics:**
   ```bash
   mooncake run --config shell.yml --tags basics
   ```

2. **Explore specific features:**
   ```bash
   mooncake run --config file.yml --tags permissions
   ```

3. **Learn from real-world examples:**
   ```bash
   mooncake run --config template.yml --tags real-world
   ```

4. **Clean up after testing:**
   ```bash
   mooncake run --config file.yml --tags cleanup
   ```

5. **Run examples safely:**
   - All examples use /tmp for testing
   - Cleanup tasks remove test files
   - sudo operations are marked with `become: true`

## Documentation

For complete action documentation, see:
- [Actions Reference](../../docs/guide/config/actions.md)
- [Control Flow](../../docs/guide/config/control-flow.md)
- [Variables](../../docs/guide/config/variables.md)
- [Complete Reference](../../docs/guide/config/reference.md)

## Structure

```
examples/actions/
├── README.md                 # This file
├── shell.yml                 # Shell command examples
├── print.yml                 # Print message examples
├── file.yml                  # File operations examples
├── copy.yml                  # Copy file examples
├── template.yml              # Template rendering examples
├── download.yml              # Download file examples
├── unarchive.yml             # Archive extraction examples
├── service.yml               # Service management examples
├── assert.yml                # Assertion examples
├── include.yml               # Include task examples
├── preset.yml                # Preset usage examples
├── vars.yml                  # Variable definition examples
├── templates/                # Template files for examples
│   ├── simple-config.yml.j2
│   ├── nginx.conf.j2
│   ├── script.sh.j2
│   └── systemd-service.j2
└── tasks/                    # Task files for include examples
    ├── common.yml
    ├── setup.yml
    ├── linux-tasks.yml
    ├── macos-tasks.yml
    └── cleanup.yml
```

## Contributing

When adding new examples:
1. Follow the existing format
2. Include clear descriptions
3. Add appropriate tags
4. Test examples work
5. Update this README

## Next Steps

After exploring these examples:
1. Check out the [numbered examples](../) (01-12) for complete workflows
2. See [scenarios](../scenarios/) for real-world setups
3. Read the [official documentation](../../docs/)
4. Build your own configurations!

## Getting Help

- Documentation: `docs/guide/config/actions.md`
- Examples: This directory and `examples/01-12`
- Issues: GitHub issues
- Community: Discussions on GitHub


---

<!-- FILE: examples/advanced-file-operations/README.md -->

# Advanced File Operations

This example demonstrates Mooncake's expanded file management capabilities:

## File States

### `state: file` - Create or update files
```yaml
- file:
    path: /tmp/config.txt
    state: file
    content: "key: value"
    mode: "0644"
```

### `state: directory` - Create directories
```yaml
- file:
    path: /tmp/app
    state: directory
    mode: "0755"
```

### `state: absent` - Remove files or directories
```yaml
- file:
    path: /tmp/old-file
    state: absent
```

Remove non-empty directory:
```yaml
- file:
    path: /tmp/old-dir
    state: absent
    force: true
```

### `state: touch` - Create empty file or update timestamp
```yaml
- file:
    path: /tmp/.marker
    state: touch
```

### `state: link` - Create symbolic links
```yaml
- file:
    path: /usr/local/bin/app
    src: /opt/app/bin/app
    state: link
```

### `state: hardlink` - Create hard links
```yaml
- file:
    path: /backup/data.txt
    src: /data/data.txt
    state: hardlink
```

### `state: perms` - Change permissions without creating
```yaml
- file:
    path: /opt/app
    state: perms
    mode: "0755"
    owner: app
    group: app
    recurse: true
```

## Copy Action

Copy files with checksum verification:

```yaml
- copy:
    src: ./app-v1.2.3
    dest: /usr/local/bin/app
    mode: "0755"
    checksum: "sha256:abc123..."
    backup: true
```

## Ownership Management

Set file owner and group:

```yaml
- file:
    path: /opt/app/config.yml
    state: file
    owner: app
    group: app
    mode: "0600"
  become: true
```

## Running the Example

```bash
# Dry-run to see what would happen
mooncake run config.yml --dry-run

# Execute the configuration
mooncake run config.yml

# View the created structure
tree /tmp/mooncake-demo
```

## Features Demonstrated

- ✅ Creating directory structures with loops
- ✅ Creating files with inline content
- ✅ Touch files (timestamp updates)
- ✅ Symbolic and hard links
- ✅ Permission-only changes
- ✅ File copying with backup
- ✅ Conditional file removal
- ✅ Force removal of non-empty directories
- ✅ Ownership management with become


---

<!-- FILE: examples/conditionals/README.md -->

# 04 - Conditionals

Learn how to conditionally execute steps based on system properties or variables.

## What You'll Learn

- Using `when` for conditional execution
- OS and architecture detection
- Complex conditions with logical operators
- Combining conditionals with tags

## Quick Start

```bash
# Run all steps (only matching conditions will execute)
mooncake run --config config.yml

# Run only dev-tagged steps
mooncake run --config config.yml --tags dev
```

## What It Does

1. Demonstrates steps that always run
2. Shows OS-specific steps (macOS vs Linux)
3. Shows architecture-specific steps
4. Demonstrates tag filtering

## Key Concepts

### Basic Conditionals

Use `when` to conditionally execute steps:
```yaml
- name: Linux only
  shell: echo "Running on Linux"
  when: os == "linux"
```

### Available System Variables

- `os` - darwin, linux, windows
- `arch` - amd64, arm64, 386, etc.
- `distribution` - ubuntu, debian, centos, macos, etc.
- `distribution_major` - major version number
- `package_manager` - apt, yum, brew, pacman, etc.

### Comparison Operators

- `==` - equals
- `!=` - not equals
- `>`, `<`, `>=`, `<=` - comparisons
- `&&` - logical AND
- `||` - logical OR
- `!` - logical NOT

### Complex Conditions

```yaml
- name: ARM Mac only
  shell: echo "ARM-based macOS"
  when: os == "darwin" && arch == "arm64"

- name: High memory systems
  shell: echo "Lots of RAM!"
  when: memory_total_mb >= 16000

- name: Ubuntu 20+
  shell: apt update
  when: distribution == "ubuntu" && distribution_major >= "20"
```

### Tags vs Conditionals

**Conditionals (`when`):**
- Evaluated at runtime
- Based on system facts or variables
- Step-level decision making

**Tags:**
- User-controlled filtering
- Specified via CLI `--tags` flag
- Workflow-level decision making

## Testing Different Conditions

Try these commands:
```bash
# See which steps run on your system
mooncake run --config config.yml

# Preview without executing
mooncake run --config config.yml --dry-run

# Run only development steps
mooncake run --config config.yml --tags dev
```

## Next Steps

→ Continue to [05-templates](../05-templates/) to learn about template rendering.


---

<!-- FILE: examples/execution-control/README.md -->

# Example 11: Execution Control

This example demonstrates advanced execution control features in Mooncake.

## Features Demonstrated

- **Timeouts**: Prevent commands from running too long
- **Retries**: Automatically retry failed commands
- **Retry Delays**: Wait between retry attempts
- **Environment Variables**: Set custom environment for commands
- **Working Directory**: Execute commands in specific directories
- **Changed When**: Custom logic to determine if a step made changes
- **Failed When**: Custom logic to determine if a step failed
- **Become User**: Run commands as different users (see note below)

## Running the Example

```bash
# Run all examples
mooncake run --config config.yml

# Preview what will run
mooncake run --config config.yml --dry-run

# With debug logging
mooncake run --config config.yml --log-level debug
```

## What Each Example Shows

### Example 1: Basic Timeout
Shows how to set a timeout to prevent commands from hanging indefinitely.

### Example 2: Retry with Delay
Demonstrates automatic retry of failed commands with configurable delay between attempts.

### Example 3: Environment Variables
Shows how to set custom environment variables, including template variable expansion.

### Example 4: Working Directory
Demonstrates changing the working directory before executing a command.

### Example 5: Custom Change Detection
Shows how to mark a command as "unchanged" even though it runs.

### Example 6: Git-Style Change Detection
Demonstrates detecting changes based on command output (common pattern with git).

### Example 7: Custom Failure Detection (grep)
Shows how to handle commands where certain non-zero exit codes are acceptable.

### Example 8: Acceptable Exit Codes
Demonstrates accepting multiple exit codes as success.

### Example 9: Combined Features
Shows using timeout and retry together for robust command execution.

### Example 10: Full Featured
Demonstrates using multiple execution control features together.

## Real-World Applications

These features are essential for production deployments:

- **Timeouts**: Prevent CI/CD pipelines from hanging
- **Retries**: Handle flaky network requests, service startups
- **Environment Variables**: Configure build tools, set API keys
- **Working Directory**: Build projects, run tests in correct locations
- **changed_when**: Accurate change reporting, trigger handlers correctly
- **failed_when**: Handle tools with non-standard exit codes

## Note on become_user

The `become_user` feature (running as different users) is not demonstrated in this example as it requires:
- Root privileges
- Sudo password
- Specific users to exist on the system

To use `become_user` in your own configs:

```yaml
- name: Run as postgres user
  shell: psql -c "SELECT version()"
  become: true
  become_user: postgres
  # Requires: mooncake run --config config.yml --sudo-pass <password>
```

## See Also

- [Execution Control Documentation](../../docs/examples/11-execution-control.md)
- [Actions Reference](../../docs/guide/config/actions.md#common-fields)
- [Register Example](../07-register/) - Capturing command output


---

<!-- FILE: examples/files-and-directories/README.md -->

# 03 - Files and Directories

Learn how to create and manage files and directories with Mooncake.

## What You'll Learn

- Creating directories with `state: directory`
- Creating files with `state: file`
- Setting file permissions with `mode`
- Adding content to files

## Quick Start

```bash
mooncake run --config config.yml
```

## What It Does

1. Creates application directory structure
2. Creates files with specific content
3. Sets appropriate permissions (755 for directories, 644 for files)
4. Creates executable scripts

## Key Concepts

### Creating Directories

```yaml
- name: Create directory
  file:
    path: /tmp/myapp
    state: directory
    mode: "0755"  # rwxr-xr-x
```

### Creating Empty Files

```yaml
- name: Create empty file
  file:
    path: /tmp/file.txt
    state: file
    mode: "0644"  # rw-r--r--
```

### Creating Files with Content

```yaml
- name: Create config file
  file:
    path: /tmp/config.txt
    state: file
    content: |
      Line 1
      Line 2
    mode: "0644"
```

### File Permissions

Use octal notation in quotes:
- `"0644"` - rw-r--r-- (readable by all, writable by owner)
- `"0755"` - rwxr-xr-x (executable by all, writable by owner)
- `"0600"` - rw------- (only owner can read/write)

### Using Variables

```yaml
- vars:
    app_dir: /tmp/myapp

- file:
    path: "{{app_dir}}/config"
    state: directory
```

## Permission Examples

| Mode | Meaning | Use Case |
|------|---------|----------|
| 0755 | rwxr-xr-x | Directories, executable scripts |
| 0644 | rw-r--r-- | Regular files, configs |
| 0600 | rw------- | Private files, secrets |
| 0700 | rwx------ | Private directories |

## Next Steps

→ Continue to [04-conditionals](../04-conditionals/) to learn about conditional execution.


---

<!-- FILE: examples/hello-world/README.md -->

# 01 - Hello World

**Start here!** This is the simplest possible Mooncake configuration.

## What You'll Learn

- Running basic shell commands
- Using global system variables
- Multi-line shell commands

## Quick Start

```bash
mooncake run --config config.yml
```

## What It Does

1. Prints a hello message
2. Runs system commands to show OS info
3. Uses Mooncake's global variables to display OS and architecture

## Key Concepts

### Shell Commands

Execute commands with the `shell` action:
```yaml
- name: Print message
  shell: echo "Hello!"
```

### Multi-line Commands

Use `|` for multiple commands:
```yaml
- name: Multiple commands
  shell: |
    echo "First command"
    echo "Second command"
```

### Global Variables

Mooncake automatically provides system information:
- `{{os}}` - Operating system (linux, darwin, windows)
- `{{arch}}` - Architecture (amd64, arm64, etc.)

## Output Example

```
▶ Print hello message
Hello from Mooncake!
✓ Print hello message

▶ Print system info
OS: Darwin
Arch: arm64
✓ Print system info

▶ Show global variables
Running on darwin/arm64
✓ Show global variables
```

## Next Steps

→ Continue to [02-variables-and-facts](../02-variables-and-facts/) to learn about custom variables and all available system facts.


---

<!-- FILE: examples/idempotency.md -->

# Idempotency Patterns

Mooncake provides several features to help you write idempotent playbooks that can be safely run multiple times without unintended side effects.

## Table of Contents

- [Using `creates`](#using-creates)
- [Using `unless`](#using-unless)
- [Using `changed_when`](#using-changed_when)
- [Combining Strategies](#combining-strategies)
- [Result Timing](#result-timing)

## Using `creates`

The `creates` field skips a step if the specified file path already exists. This is useful for one-time installation or setup tasks.

### One-time installation

```yaml
- name: Download installer
  shell: wget https://example.com/installer.sh -O /tmp/installer.sh
  creates: /tmp/installer.sh

- name: Run installer
  shell: bash /tmp/installer.sh
  creates: /opt/myapp/bin/myapp
```

On the first run, both steps execute. On subsequent runs, both steps are skipped because the files exist.

### Compilation steps

```yaml
- name: Compile binary
  shell: go build -o myapp
  creates: ./myapp
```

The compilation only runs if the binary doesn't exist yet.

### Template variables

The `creates` path is rendered through the template engine, so you can use variables:

```yaml
- name: Set build directory
  vars:
    build_dir: /opt/myproject

- name: Compile project
  shell: make build
  creates: "{{ build_dir }}/myapp"
```

## Using `unless`

The `unless` field skips a step if the given command succeeds (returns exit code 0). This provides more flexibility than `creates` for conditional execution.

### Database initialization

```yaml
- name: Initialize database
  shell: psql -f schema.sql mydb
  unless: "psql -c '\\dt' mydb | grep users"
```

The initialization only runs if the `users` table doesn't exist.

### Service configuration

```yaml
- name: Configure service
  shell: systemctl enable myservice
  unless: "systemctl is-enabled myservice"
```

The service is only enabled if it's not already enabled.

### Version checks

```yaml
- name: Install package
  shell: apt-get install -y mypackage
  unless: "dpkg -s mypackage | grep -q 'Version: 2.0'"
```

The package is only installed if version 2.0 is not currently installed.

### Template variables

Like `creates`, the `unless` command is rendered through the template engine:

```yaml
- name: Set database name
  vars:
    db_name: production

- name: Create database
  shell: createdb {{ db_name }}
  unless: "psql -l | grep {{ db_name }}"
```

**Note**: The `unless` command is executed silently (no output logged) to avoid cluttering the logs.

## Using `changed_when`

The `changed_when` field allows you to override whether a shell command is marked as "changed". By default, all shell commands are marked as changed.

### Commands with predictable output

```yaml
- name: Install package
  shell: apt-get install -y package
  register: install_result
  changed_when: "'is already the newest version' not in result.stdout"
```

The step is only marked as changed if the package was actually installed or upgraded.

### Always-safe commands

```yaml
- name: Set sysctl value (idempotent)
  shell: sysctl -w net.ipv4.ip_forward=1
  changed_when: false
```

Setting `changed_when: false` indicates this command is idempotent and doesn't make changes.

### Conditional based on exit code

```yaml
- name: Check and update config
  shell: diff config.new /etc/config && cp config.new /etc/config
  register: config_result
  changed_when: result.rc == 0
  failed_when: false
```

Only mark as changed if the files were different (diff returns 0) and the copy succeeded.

## Combining Strategies

You can combine `creates`, `unless`, `when`, and `changed_when` for sophisticated idempotency control.

### Smart package installation

```yaml
- name: Install package
  shell: apt-get install -y mypackage
  creates: /usr/bin/mypackage
  register: pkg_install
  changed_when: "result.rc == 0 and 'already installed' not in result.stdout"
  failed_when: "result.rc != 0 and 'Unable to locate package' not in result.stderr"
```

This step:
- Skips if `/usr/bin/mypackage` already exists
- Only marks as changed if the package was actually installed
- Only fails if there's a real error (not just "package not found")

### Conditional with multiple checks

```yaml
- name: Install development tools
  shell: apt-get install -y build-essential
  when: ansible_os_family == "Debian"
  unless: "dpkg -l build-essential | grep '^ii'"
  creates: /usr/bin/gcc
```

This step only runs if:
- The OS family is Debian (via `when`)
- The package is not already installed (via `unless`)
- The compiler doesn't exist (via `creates`)

**Evaluation order**: `when` → `creates` → `unless` → execute

### Database setup with safeguards

```yaml
- name: Create database user
  shell: |
    psql -c "CREATE USER myapp WITH PASSWORD '{{ db_password }}';"
  unless: "psql -c '\\du' | grep myapp"
  register: user_created
  changed_when: result.rc == 0

- name: Grant privileges
  shell: |
    psql -c "GRANT ALL PRIVILEGES ON DATABASE mydb TO myapp;"
  when: user_created.changed
  unless: "psql -c '\\l' mydb | grep myapp | grep -q PRIVILEGES"
```

The second step only runs if the user was just created or if privileges aren't already granted.

## Result Timing

All step results now include timing information that can be accessed in registered results:

```yaml
- name: Run expensive operation
  shell: make build
  register: build_result

- name: Show build duration
  shell: echo "Build took {{ build_result.duration_ms }}ms"
```

### Available timing fields

When you register a result, the following timing fields are available:

- `result.duration_ms`: Duration in milliseconds (integer)
- `result.status`: String status ("ok", "changed", "failed", "skipped")

### Example: Performance monitoring

```yaml
- name: Compile project
  shell: make -j4 build
  register: compile

- name: Run tests
  shell: make test
  register: tests

- name: Report performance
  shell: |
    echo "Compilation: {{ compile.duration_ms }}ms"
    echo "Tests: {{ tests.duration_ms }}ms"
    echo "Total: {{ compile.duration_ms + tests.duration_ms }}ms"
```

### Example: Conditional based on performance

```yaml
- name: Run optimization
  shell: optimize-database
  register: optimize_result

- name: Alert if slow
  shell: |
    echo "Warning: Optimization took {{ optimize_result.duration_ms }}ms" | \
    mail -s "Slow optimization" admin@example.com
  when: optimize_result.duration_ms > 60000
```

This sends an alert if optimization takes more than 60 seconds (60000ms).

## Best Practices

### 1. Prefer `creates` for file-based idempotency

Use `creates` when you're creating files or installing software that produces files:

```yaml
# Good
- name: Download file
  shell: wget https://example.com/file.tar.gz
  creates: file.tar.gz

# Less efficient
- name: Download file
  shell: wget https://example.com/file.tar.gz
  unless: "test -f file.tar.gz"
```

`creates` is more efficient because it uses a simple filesystem check.

### 2. Use `unless` for state checks

Use `unless` when idempotency depends on system state rather than file existence:

```yaml
- name: Enable firewall rule
  shell: ufw allow 22/tcp
  unless: "ufw status | grep '22/tcp.*ALLOW'"
```

### 3. Combine with `register` for dependent steps

```yaml
- name: Install package
  shell: apt-get install -y nginx
  creates: /usr/sbin/nginx
  register: nginx_installed

- name: Start nginx
  shell: systemctl start nginx
  when: nginx_installed.changed
```

The service is only started if nginx was just installed.

### 4. Document non-obvious idempotency

```yaml
- name: Apply database migrations (idempotent via migration tracking table)
  shell: ./migrate.sh
  changed_when: result.stdout | contains('Applied migrations')
```

Add comments when idempotency isn't immediately obvious from the command.

### 5. Test your idempotency

Always run your playbook at least twice to verify it's truly idempotent:

```bash
# First run - should make changes
mooncake run -c playbook.yml

# Second run - should skip most steps
mooncake run -c playbook.yml
```

## Common Patterns

### Package management

```yaml
- name: Install package
  shell: apt-get install -y package-name
  creates: /usr/bin/package-name

# Or with unless
- name: Install package
  shell: apt-get install -y package-name
  unless: "dpkg -l package-name | grep '^ii'"
```

### File downloads

```yaml
- name: Download archive
  shell: wget https://example.com/archive.tar.gz
  creates: archive.tar.gz

- name: Extract archive
  shell: tar xzf archive.tar.gz
  creates: archive/
```

### Service management

```yaml
- name: Enable service
  shell: systemctl enable myservice
  unless: "systemctl is-enabled myservice"

- name: Start service
  shell: systemctl start myservice
  unless: "systemctl is-active myservice"
```

### Configuration management

```yaml
- name: Update config
  template:
    src: config.j2
    dest: /etc/myapp/config.yml
  register: config_updated

- name: Restart service if config changed
  shell: systemctl restart myapp
  when: config_updated.changed
```

### Database operations

```yaml
- name: Create database
  shell: createdb mydb
  unless: "psql -l | grep mydb"

- name: Load schema
  shell: psql mydb < schema.sql
  unless: "psql mydb -c '\\dt' | grep users"
```

## Troubleshooting

### Step is not being skipped

1. **Check file paths are correct**:
   ```yaml
   # Wrong - uses relative path that might change
   creates: ./myapp

   # Better - use absolute path
   creates: /opt/myapp/myapp
   ```

2. **Check command exit codes**:
   ```bash
   # Test your unless command manually
   test -f /tmp/marker && echo "Skip" || echo "Run"
   ```

3. **Use debug mode**:
   ```bash
   mooncake run -c playbook.yml --log-level debug
   ```

### Step is being skipped incorrectly

1. **Verify the condition**:
   - For `creates`: Is the file being deleted elsewhere?
   - For `unless`: Is the command returning the wrong exit code?

2. **Check for template variables**:
   ```yaml
   # Make sure variables are set
   - name: Debug variable
     shell: echo "Checking {{ file_path }}"

   - name: Do work
     shell: create-file
     creates: "{{ file_path }}"
   ```

## See Also

- [Configuration Reference](../guide/config/reference.md) - Full field documentation
- [Control Flow](../guide/config/control-flow.md) - Conditionals and when expressions
- [Variables](../guide/config/variables.md) - Template syntax and variables


---

<!-- FILE: examples/index.md -->

# Examples

Complete collection of runnable Mooncake configuration examples.

## Basic Examples

### [01 - Hello World](01-hello-world.md)
Your first Mooncake configuration with shell commands and print actions.

### [02 - Variables and Facts](02-variables-and-facts.md)
Using variables, facts, and template expressions.

### [03 - Files and Directories](03-files-and-directories.md)
File operations: creating directories, managing files, permissions.

### [04 - Conditionals](04-conditionals.md)
Conditional execution with when clauses and facts.

### [05 - Templates](05-templates.md)
Template rendering with Jinja2 syntax.

### [06 - Loops](06-loops.md)
Iterating over lists with loop constructs.

### [07 - Register](07-register.md)
Capturing and using step results.

### [08 - Tags](08-tags.md)
Selective execution using tags.

### [09 - Sudo](09-sudo.md)
Running commands with elevated privileges.

### [10 - Multi-File Configs](10-multi-file-configs.md)
Organizing configurations across multiple files.

### [11 - Execution Control](11-execution-control.md)
Error handling, failed_when, changed_when.

### [12 - Unarchive](12-unarchive.md)
Extracting archives (tar, tar.gz, zip).

## Real-World Examples

### [Real-World: Dotfiles Management](real-world-dotfiles.md)
Complete dotfiles installation and configuration.

### [Idempotency Demonstration](idempotency.md)
How Mooncake ensures operations are idempotent.


---

<!-- FILE: examples/json-output-example.md -->

# JSON Output Example

## Overview

Mooncake now supports structured JSON event output via the event system. This enables integration with external tools, monitoring systems, and custom processing pipelines.

## Usage

```bash
# Run with JSON event output
mooncake run --config myconfig.yml --raw --output-format json

# Process events with jq
mooncake run --config myconfig.yml --raw --output-format json | jq '.'

# Filter specific event types
mooncake run --config myconfig.yml --raw --output-format json | \
  jq 'select(.type == "step.completed")'

# Extract step durations
mooncake run --config myconfig.yml --raw --output-format json | \
  jq 'select(.type == "step.completed") | {name: .data.name, duration_ms: .data.duration_ms}'

# Monitor execution in real-time
mooncake run --config myconfig.yml --raw --output-format json | \
  jq --unbuffered -c 'select(.type | startswith("step."))'
```

## Event Types

### Run Lifecycle
- `run.started` - Execution begins
- `plan.loaded` - Plan has been built
- `run.completed` - Execution finished

### Step Lifecycle
- `step.started` - Step begins execution
- `step.completed` - Step completed successfully
- `step.failed` - Step failed with error
- `step.skipped` - Step was skipped

### Output Streaming
- `step.stdout` - Standard output line from shell step
- `step.stderr` - Standard error line from shell step

### File Operations
- `file.created` - File was created
- `file.updated` - File was updated
- `directory.created` - Directory was created
- `template.rendered` - Template was rendered

### Variables
- `variables.set` - Variables were set inline
- `variables.loaded` - Variables were loaded from file

## Event Schema

### run.started
```json
{
  "type": "run.started",
  "timestamp": "2026-02-04T14:14:19.699336+01:00",
  "data": {
    "root_file": "/path/to/config.yml",
    "tags": ["tag1", "tag2"],
    "dry_run": false,
    "total_steps": 10
  }
}
```

### step.started
```json
{
  "type": "step.started",
  "timestamp": "2026-02-04T14:14:19.699372+01:00",
  "data": {
    "step_id": "step-0001",
    "name": "Install nginx",
    "level": 0,
    "global_step": 1,
    "action": "shell",
    "tags": ["setup"],
    "when": ""
  }
}
```

### step.completed
```json
{
  "type": "step.completed",
  "timestamp": "2026-02-04T14:14:19.705515+01:00",
  "data": {
    "step_id": "step-0001",
    "name": "Install nginx",
    "level": 0,
    "duration_ms": 1250,
    "changed": true
  }
}
```

### step.stdout
```json
{
  "type": "step.stdout",
  "timestamp": "2026-02-04T14:14:19.705324+01:00",
  "data": {
    "step_id": "step-0001",
    "stream": "stdout",
    "line": "nginx installed successfully",
    "line_number": 1
  }
}
```

### run.completed
```json
{
  "type": "run.completed",
  "timestamp": "2026-02-04T14:14:29.180581+01:00",
  "data": {
    "total_steps": 10,
    "success_steps": 9,
    "failed_steps": 1,
    "skipped_steps": 0,
    "changed_steps": 7,
    "duration_ms": 15432,
    "success": false,
    "error_message": "Step 'Deploy app' failed: connection refused"
  }
}
```

## Use Cases

### 1. CI/CD Integration
```bash
# Parse execution results in CI/CD pipeline
mooncake run --config deploy.yml --raw --output-format json > execution.jsonl

# Check if execution succeeded
if jq -e '.type == "run.completed" and .data.success == true' execution.jsonl > /dev/null; then
  echo "Deployment successful"
  exit 0
else
  echo "Deployment failed"
  exit 1
fi
```

### 2. Performance Monitoring
```bash
# Extract step performance metrics
mooncake run --config myconfig.yml --raw --output-format json | \
  jq 'select(.type == "step.completed") |
      {step: .data.name, duration: .data.duration_ms}' | \
  jq -s 'sort_by(.duration) | reverse'
```

### 3. Real-Time Dashboard
```bash
# Stream events to monitoring dashboard
mooncake run --config myconfig.yml --raw --output-format json | \
  while read -r event; do
    # Send to Elasticsearch, Prometheus, etc.
    curl -X POST http://dashboard/api/events -d "$event"
  done
```

### 4. Log Aggregation
```bash
# Forward events to log aggregation system
mooncake run --config myconfig.yml --raw --output-format json | \
  jq -c '.' | \
  filebeat -e -c filebeat.yml
```

### 5. Custom Processing
```bash
# Filter and transform events with custom script
mooncake run --config myconfig.yml --raw --output-format json | \
  python process_events.py
```

## Notes

- JSON output requires `--raw` flag (disables TUI)
- Each event is a single-line JSON object (JSONL format)
- Events are emitted in real-time as execution progresses
- All timestamps are in ISO 8601 format
- Step IDs are unique within a run (e.g., "step-0001", "step-0002")
- Output lines include line numbers for multi-line output

## Example Processing Script

```python
#!/usr/bin/env python3
import sys
import json

for line in sys.stdin:
    event = json.loads(line)

    if event['type'] == 'step.completed':
        data = event['data']
        print(f"✓ {data['name']} ({data['duration_ms']}ms)")

    elif event['type'] == 'step.failed':
        data = event['data']
        print(f"✗ {data['name']}: {data['error_message']}")

    elif event['type'] == 'run.completed':
        data = event['data']
        if data['success']:
            print(f"\nSuccess! {data['success_steps']}/{data['total_steps']} steps completed")
        else:
            print(f"\nFailed: {data['error_message']}")
```

Save as `process_events.py` and use:
```bash
mooncake run --config myconfig.yml --raw --output-format json | python process_events.py
```


---

<!-- FILE: examples/loops/README.md -->

# 06 - Loops

Learn how to iterate over lists and files to avoid repetition.

## What You'll Learn

- Iterating over lists with `with_items`
- Iterating over files with `with_filetree`
- Using the `{{ item }}` variable
- Accessing file properties in loops

## Quick Start

```bash
# Run list iteration example
mooncake run --config with-items.yml

# Run file tree iteration example
mooncake run --config with-filetree/config.yml
```

## Examples Included

### 1. with-items.yml - List Iteration

Iterate over lists of items:
```yaml
- vars:
    packages:
      - neovim
      - ripgrep
      - fzf

- name: Install package
  shell: brew install {{ item }}
  with_items: "{{ packages }}"
```

**What it does:**
- Defines lists in variables
- Installs multiple packages
- Creates directories for multiple users
- Creates user-specific config files

### 2. with-filetree/ - File Tree Iteration

Iterate over files in a directory:
```yaml
- name: Copy dotfile
  shell: cp "{{ item.src }}" "/tmp/backup/{{ item.name }}"
  with_filetree: ./files
```

**What it does:**
- Iterates over files in `./files/` directory
- Copies dotfiles to backup location
- Filters directories vs files
- Displays file properties

## Key Concepts

### List Iteration (with_items)

```yaml
- vars:
    users: [alice, bob, charlie]

- name: Create user directory
  file:
    path: "/home/{{ item }}"
    state: directory
  with_items: "{{ users }}"
```

This creates:
- `/home/alice`
- `/home/bob`
- `/home/charlie`

### File Tree Iteration (with_filetree)

```yaml
- name: Process file
  shell: echo "Processing {{ item.name }}"
  with_filetree: ./dotfiles
  when: item.is_dir == false
```

**Available properties:**
- `item.src` - Full source path
- `item.name` - File name
- `item.is_dir` - Boolean, true if directory

### Filtering in Loops

Skip directories:
```yaml
- name: Copy files only
  shell: cp "{{ item.src }}" "/tmp/{{ item.name }}"
  with_filetree: ./files
  when: item.is_dir == false
```

## Real-World Use Cases

**with_items:**
- Installing multiple packages
- Creating multiple users/groups
- Setting up multiple services
- Deploying to multiple servers

**with_filetree:**
- Managing dotfiles
- Deploying configuration directories
- Backing up files
- Processing file collections

## Testing

```bash
# List iteration
mooncake run --config with-items.yml

# Check created files
ls -la /tmp/users/

# File tree iteration
mooncake run --config with-filetree/config.yml

# Check backed up files
ls -la /tmp/dotfiles-backup/
```

## Next Steps

→ Continue to [07-register](../07-register/) to learn about capturing command output.


---

<!-- FILE: examples/macos-services/README.md -->

# macOS Service Management with Launchd

This guide demonstrates how to use Mooncake to manage macOS services using launchd.

## Table of Contents

1. [Quick Start](#quick-start)
2. [Service Types](#service-types)
3. [Complete Examples](#complete-examples)
4. [Common Patterns](#common-patterns)
5. [Plist Properties](#plist-properties)
6. [Troubleshooting](#troubleshooting)

---

## Quick Start

### Simple User Agent

```yaml
- name: Start my application
  service:
    name: com.example.myapp
    state: started
    enabled: true
    unit:
      content: |
        <?xml version="1.0" encoding="UTF-8"?>
        <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
        <plist version="1.0">
        <dict>
          <key>Label</key>
          <string>com.example.myapp</string>
          <key>ProgramArguments</key>
          <array>
            <string>/usr/local/bin/myapp</string>
          </array>
          <key>RunAtLoad</key>
          <true/>
        </dict>
        </plist>
```

### Using Templates

```yaml
- name: Deploy service from template
  service:
    name: com.example.{{ app_name }}
    state: started
    enabled: true
    unit:
      src_template: templates/service.plist.j2
```

---

## Service Types

### User Agents

User agents run in the user's context (no sudo required):
- **Path**: `~/Library/LaunchAgents/`
- **Domain**: `gui/<uid>`
- **Permissions**: Current user
- **When**: When user logs in

```yaml
- name: User agent
  service:
    name: com.example.myapp
    state: started
    enabled: true
    unit:
      content: |
        <!-- plist content here -->
```

### System Daemons

System daemons run as root (require sudo):
- **Path**: `/Library/LaunchDaemons/`
- **Domain**: `system`
- **Permissions**: root (requires `become: true`)
- **When**: At system boot

```yaml
- name: System daemon
  service:
    name: com.example.daemon
    state: started
    enabled: true
    unit:
      dest: /Library/LaunchDaemons/com.example.daemon.plist
      content: |
        <!-- plist content here -->
  become: true
```

---

## Complete Examples

### 1. Node.js Web Server

See: [`macos-nodejs-app.yml`](./macos-nodejs-app.yml)

Complete example showing:
- Directory setup
- Dependency installation
- Service configuration
- Logging
- Environment variables
- Health checks

Run with:
```bash
mooncake run examples/macos-services/macos-nodejs-app.yml
```

### 2. Service Management Operations

See: [`macos-service-management.yml`](./macos-service-management.yml)

Examples of:
- Starting/stopping services
- Restarting services
- Updating configuration
- Enabling/disabling
- Dry-run mode

Run with:
```bash
mooncake run examples/macos-services/macos-service-management.yml
```

### 3. Various Service Types

See: [`macos-launchd-service.yml`](./macos-launchd-service.yml)

Demonstrates:
- User agents
- System daemons
- Scheduled tasks
- Resource limits
- Keep-alive configuration

---

## Common Patterns

### Auto-Restart on Crash

```xml
<key>KeepAlive</key>
<dict>
  <key>SuccessfulExit</key>
  <false/>
  <key>Crashed</key>
  <true/>
</dict>
```

### Scheduled Task (Cron-like)

```xml
<!-- Run every hour -->
<key>StartCalendarInterval</key>
<dict>
  <key>Minute</key>
  <integer>0</integer>
</dict>
```

```xml
<!-- Run every day at 2:30 AM -->
<key>StartCalendarInterval</key>
<dict>
  <key>Hour</key>
  <integer>2</integer>
  <key>Minute</key>
  <integer>30</integer>
</dict>
```

### Environment Variables

```xml
<key>EnvironmentVariables</key>
<dict>
  <key>PORT</key>
  <string>8080</string>
  <key>NODE_ENV</key>
  <string>production</string>
</dict>
```

### Logging

```xml
<key>StandardOutPath</key>
<string>/var/log/myapp/stdout.log</string>
<key>StandardErrorPath</key>
<string>/var/log/myapp/stderr.log</string>
```

### Prevent Rapid Restarts

```xml
<!-- Wait 10 seconds before restarting -->
<key>ThrottleInterval</key>
<integer>10</integer>
```

---

## Plist Properties

### Essential Properties

| Key | Type | Description |
|-----|------|-------------|
| `Label` | String | Service identifier (required) |
| `ProgramArguments` | Array | Command and arguments to run (required) |

### Execution Control

| Key | Type | Description |
|-----|------|-------------|
| `RunAtLoad` | Boolean | Start when loaded |
| `KeepAlive` | Boolean/Dict | Auto-restart configuration |
| `StartCalendarInterval` | Dict | Schedule (cron-like) |
| `StartInterval` | Integer | Run every N seconds |

### Process Management

| Key | Type | Description |
|-----|------|-------------|
| `WorkingDirectory` | String | Working directory |
| `EnvironmentVariables` | Dict | Environment variables |
| `UserName` | String | Run as specific user |
| `GroupName` | String | Run as specific group |

### Logging

| Key | Type | Description |
|-----|------|-------------|
| `StandardOutPath` | String | Stdout log file |
| `StandardErrorPath` | String | Stderr log file |

### Resource Limits

| Key | Type | Description |
|-----|------|-------------|
| `SoftResourceLimits` | Dict | Soft resource limits |
| `HardResourceLimits` | Dict | Hard resource limits |
| `Nice` | Integer | Process priority (-20 to 20) |

### Network

| Key | Type | Description |
|-----|------|-------------|
| `Sockets` | Dict | Socket activation |

---

## Service States

### Available States

| State | Description | Action |
|-------|-------------|--------|
| `started` | Start the service | `launchctl bootstrap` (if not loaded)<br>`launchctl kickstart` (if loaded) |
| `stopped` | Stop the service | `launchctl kill SIGTERM` |
| `restarted` | Restart the service | `launchctl kickstart -k` |
| `reloaded` | Reload configuration | Same as `restarted` |

### Enabled Status

| Status | Description | Action |
|--------|-------------|--------|
| `enabled: true` | Load service (persistent) | `launchctl bootstrap` |
| `enabled: false` | Unload service | `launchctl bootout` |

---

## Idempotency

Mooncake automatically ensures idempotent operations:

1. **Plist Updates**: Only writes if content changed
2. **Service State**: Checks current state before changing
3. **Load Status**: Only loads/unloads if needed

Example:
```yaml
# First run: Creates plist, loads service, starts it
# Second run: No changes (plist unchanged, service already running)
- name: Deploy service
  service:
    name: com.example.app
    state: started
    enabled: true
    unit:
      content: |
        <!-- plist content -->
```

---

## Troubleshooting

### Check Service Status

```bash
# List all loaded services
launchctl list

# Check specific service
launchctl list | grep com.example.myapp

# Print service details
launchctl print gui/$(id -u)/com.example.myapp
```

### View Logs

```bash
# If using StandardOutPath/StandardErrorPath
tail -f /path/to/stdout.log
tail -f /path/to/stderr.log

# System logs
log stream --predicate 'processImagePath contains "myapp"' --info
```

### Unload Service Manually

```bash
# User agent
launchctl bootout gui/$(id -u)/com.example.myapp

# System daemon
sudo launchctl bootout system/com.example.daemon
```

### Load Service Manually

```bash
# User agent
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.example.myapp.plist

# System daemon
sudo launchctl bootstrap system /Library/LaunchDaemons/com.example.daemon.plist
```

### Common Issues

**Issue**: Service not starting
- Check plist syntax: `plutil -lint ~/Library/LaunchAgents/com.example.myapp.plist`
- Check logs: `tail -f /path/to/error.log`
- Verify program path exists

**Issue**: Permission denied
- User agents: Don't use `become: true`
- System daemons: Must use `become: true`

**Issue**: Service keeps restarting
- Check exit code: `launchctl print gui/$(id -u)/com.example.myapp`
- Review logs for errors
- Add `ThrottleInterval` to prevent rapid restarts

---

## Dry-Run Mode

Preview changes without applying them:

```bash
mooncake run --dry-run examples/macos-services/macos-launchd-service.yml
```

Output shows:
- What plist files would be created/updated
- What services would be started/stopped
- What operations would be performed

---

## Template Variables

Use variables for flexibility:

```yaml
vars:
  app_name: myapp
  app_path: /usr/local/bin/myapp
  port: 8080
  log_dir: /var/log/myapp

steps:
  - name: Deploy {{ app_name }}
    service:
      name: com.example.{{ app_name }}
      unit:
        content: |
          <?xml version="1.0" encoding="UTF-8"?>
          <!-- ... -->
          <key>ProgramArguments</key>
          <array>
            <string>{{ app_path }}</string>
          </array>
          <key>EnvironmentVariables</key>
          <dict>
            <key>PORT</key>
            <string>{{ port }}</string>
          </dict>
```

---

## References

- [launchd.info](http://www.launchd.info/) - Comprehensive launchd documentation
- [Apple Developer Documentation](https://developer.apple.com/library/archive/documentation/MacOSX/Conceptual/BPSystemStartup/Chapters/CreatingLaunchdJobs.html)
- [launchctl man page](https://ss64.com/osx/launchctl.html)
- [plist man page](https://www.manpagez.com/man/5/plist/)

---

## Testing

All launchd functionality is tested:
- ✅ Plist creation (inline and template)
- ✅ Service state management
- ✅ Load/unload operations
- ✅ Idempotency checks
- ✅ Platform detection
- ✅ Dry-run mode

Tests automatically skip on non-macOS platforms.

Run tests:
```bash
go test ./internal/executor -run "Launchd|Service"
```


---

<!-- FILE: examples/multi-file-configs/README.md -->

# 10 - Multi-File Configurations

Learn how to organize large configurations into multiple files.

## What You'll Learn

- Splitting configuration into multiple files
- Using `include` to load other configs
- Using `include_vars` to load variables
- Organizing by environment (dev/prod)
- Organizing by platform (Linux/macOS)
- Relative path resolution

## Quick Start

```bash
# Run with development environment (default)
mooncake run --config main.yml

# Run with specific tags
mooncake run --config main.yml --tags install
mooncake run --config main.yml --tags dev
```

## Directory Structure

```
10-multi-file-configs/
├── main.yml              # Entry point
├── tasks/                # Modular task files
│   ├── common.yml        # Common setup
│   ├── linux.yml         # Linux-specific
│   ├── macos.yml         # macOS-specific
│   └── dev-tools.yml     # Development tools
└── vars/                 # Environment variables
    ├── development.yml   # Dev settings
    └── production.yml    # Prod settings
```

## What It Does

1. Sets project variables
2. Loads environment-specific variables
3. Runs common setup tasks
4. Runs OS-specific tasks (Linux or macOS)
5. Conditionally runs dev tools setup

## Key Concepts

### Entry Point (main.yml)

The main file orchestrates everything:
```yaml
- vars:
    project_name: MyProject
    env: development

- name: Load environment variables
  include_vars: ./vars/{{env}}.yml

- name: Setup common configuration
  include: ./tasks/common.yml

- name: Setup OS-specific configuration
  include: ./tasks/macos.yml
  when: os == "darwin"
```

### Including Variable Files

Load variables from external YAML:
```yaml
- name: Load development vars
  include_vars: ./vars/development.yml
```

**vars/development.yml:**
```yaml
debug: true
port: 8080
database_host: localhost
```

### Including Task Files

Load and execute tasks from other files:
```yaml
- name: Run common setup
  include: ./tasks/common.yml
```

**tasks/common.yml:**
```yaml
- name: Create project directory
  file:
    path: /tmp/{{project_name}}
    state: directory
```

### Relative Path Resolution

Paths are relative to the **current file**, not the working directory:

```
main.yml:
  include: ./tasks/common.yml  # Relative to main.yml

tasks/common.yml:
  template:
    src: ./templates/config.j2  # Relative to common.yml, not main.yml
```

### Organization Strategies

**By Environment:**
```
vars/
  development.yml
  staging.yml
  production.yml
```

**By Platform:**
```
tasks/
  linux.yml
  macos.yml
  windows.yml
```

**By Component:**
```
tasks/
  database.yml
  webserver.yml
  cache.yml
```

**By Phase:**
```
tasks/
  00-prepare.yml
  01-install.yml
  02-configure.yml
  03-deploy.yml
```

## Real-World Example

### Project Structure
```
my-project/
├── setup.yml              # Main entry
├── environments/
│   ├── dev.yml
│   ├── staging.yml
│   └── prod.yml
├── platforms/
│   ├── linux.yml
│   └── macos.yml
├── components/
│   ├── postgres.yml
│   ├── nginx.yml
│   └── app.yml
└── templates/
    ├── nginx.conf.j2
    └── app-config.yml.j2
```

### Main File
```yaml
# setup.yml
- vars:
    environment: "{{ lookup('env', 'ENVIRONMENT') or 'dev' }}"

- include_vars: ./environments/{{ environment }}.yml

- include: ./platforms/{{ os }}.yml

- include: ./components/postgres.yml
- include: ./components/nginx.yml
- include: ./components/app.yml
```

## Switching Environments

**Method 1: Modify main.yml**
```yaml
- vars:
    env: production  # Change this
```

**Method 2: Use environment variable**
```bash
ENVIRONMENT=production mooncake run --config main.yml
```

**Method 3: Different main files**
```bash
mooncake run --config prod-setup.yml
```

## Benefits of Multi-File Organization

1. **Maintainability** - Easier to find and update specific parts
2. **Reusability** - Share tasks across projects
3. **Collaboration** - Team members can work on different files
4. **Testing** - Test components independently
5. **Clarity** - Clear separation of concerns

## Testing

```bash
# Run full configuration
mooncake run --config main.yml

# Preview what will run
mooncake run --config main.yml --dry-run

# Run with debug logging to see includes
mooncake run --config main.yml --log-level debug

# Run specific tagged sections
mooncake run --config main.yml --tags install
```

## Best Practices

1. **Clear naming** - Use descriptive file names
2. **Logical grouping** - Group related tasks together
3. **Document includes** - Comment what each include does
4. **Avoid deep nesting** - Keep include hierarchy shallow (2-3 levels max)
5. **Use variables** - Make includes reusable with variables

## Next Steps

→ Explore [real-world](../real-world/) examples to see complete practical applications!


---

<!-- FILE: examples/ollama/README.md -->

# Ollama Preset Examples

This directory contains examples demonstrating the Ollama preset for managing Ollama installations, service configuration, and LLM model management.

## Quick Start

**New to Ollama?** Start here:

### `ollama-quick-start.yml`
Simple 5-minute demo that installs Ollama and runs test queries:
```bash
mooncake run -c examples/ollama/ollama-quick-start.yml --ask-become-pass
```
- Installs Ollama
- Pulls tinyllama (smallest model, ~637MB)
- Starts server
- Runs test queries (math, geography)

## Examples

### `ollama-example.yml` (Comprehensive)
Complete example demonstrating all Ollama preset capabilities:
- Installation variations (basic, with service, via specific method)
- Model management (single, multiple, force re-pull)
- Service configuration (custom host, models directory, environment variables)
- Complete deployment workflow
- Uninstallation scenarios
- Platform-specific examples (Linux/macOS)
- Integration with other actions

```bash
# Dry-run mode (shows what would happen)
mooncake run -c examples/ollama/ollama-example.yml --dry-run

# Actual execution (requires sudo)
mooncake run -c examples/ollama/ollama-example.yml --ask-become-pass
```

### `ollama-quick-start.yml` (Beginner-Friendly)
Fast introduction to Ollama preset with minimal configuration:
- Quick installation
- Single model download
- Simple test queries
- Good for first-time users

## Basic Usage

### 1. Basic Installation
```yaml
- name: Install Ollama
  preset: ollama
  with:
    state: present
  become: true
```

### 2. Install with Service
```yaml
- name: Install Ollama with service
  preset: ollama
  with:
    state: present
    service: true
  become: true
```

### 3. Install and Pull Models
```yaml
- name: Install Ollama and pull models
  preset: ollama
  with:
    state: present
    service: true
    pull:
      - "llama3.1:8b"
      - "mistral:latest"
  become: true
```

### 4. Complete Configuration
```yaml
- name: Full Ollama deployment
  preset: ollama
  with:
    state: present
    service: true
    method: auto
    host: "0.0.0.0:11434"
    models_dir: "/data/ollama"
    pull:
      - "llama3.1:8b"
    env:
      OLLAMA_DEBUG: "1"
  become: true
```

## Features Demonstrated

- Installation management (auto, script, package methods)
- Service configuration (systemd on Linux, launchd on macOS)
- Model pulling (single, multiple, with force flag)
- Custom configuration (host, models directory, environment variables)
- Uninstallation (with optional model removal)
- Facts integration (automatic detection)
- Idempotency (won't reinstall if present)
- Platform support (Linux, macOS)

## Supported Platforms

- **Linux** (Ubuntu, Debian, Fedora, Arch, etc.)
  - systemd service management
  - Package managers: apt, dnf, yum, pacman, zypper, apk

- **macOS**
  - launchd service management
  - Homebrew integration

## Tips

1. **Start with dry-run**: Use `--dry-run` to see what will happen
2. **Use facts**: Check `{{ ollama_version }}` before installation
3. **Idempotency**: The preset won't reinstall if Ollama is already present
4. **Model size**: Consider starting with tinyllama (~637MB) for testing
5. **Service management**: Use `service: true` for production deployments
6. **Sudo required**: Most operations need `become: true` or `--ask-become-pass`

## Documentation

For complete documentation, see:
- [Preset Reference](../../docs/guide/presets.md) - Full preset documentation
- [Configuration Reference](../../docs/guide/config/reference.md) - Property tables
- [Core Concepts](../../docs/guide/core-concepts.md) - Overview

For questions or issues, see the main [Mooncake documentation](../../docs/).


---

<!-- FILE: examples/real-world/README.md -->

# Real-World Examples

Complete, practical examples showing how to combine Mooncake features for real-world use cases.

## Examples

### Dotfiles Manager

**[dotfiles-manager/](dotfiles-manager/)** - Complete dotfiles deployment system

Demonstrates:
- Managing configuration files across machines
- Template-based dynamic configs
- OS-specific configurations
- Backup and deployment workflows
- Tag-based selective deployment

Perfect for:
- Setting up new development machines
- Keeping dotfiles in sync across systems
- Team configuration standardization

## Contributing Your Own

Have a great real-world example? Consider contributing it! Real-world examples should:

1. **Solve a complete problem** - Not just demonstrate one feature
2. **Be practical** - Something you'd actually use
3. **Combine features** - Show how features work together
4. **Include documentation** - Explain what it does and why
5. **Be testable** - Work with `--dry-run` for safety

## Ideas for Real-World Examples

- **Web Server Setup** - Deploy nginx + app + database
- **Development Environment** - Complete dev stack setup
- **System Hardening** - Security configurations
- **Backup System** - Automated backup routines
- **CI/CD Deployment** - Application deployment pipeline
- **Docker Host Setup** - Container environment provisioning
- **Monitoring Setup** - Prometheus + Grafana + exporters
- **WiFi Configuration** - Network profiles and VPN

## Learning Path

1. Complete the [numbered examples](../) first (01-10)
2. Study how features combine in real-world examples
3. Adapt these examples to your needs
4. Build your own real-world configurations

## Getting Help

- Review individual feature examples (01-10)
- Check the [main README](../../README.md)
- Use `--dry-run` to preview safely
- Use `--log-level debug` to troubleshoot


---

<!-- FILE: examples/real-world/dotfiles-manager/README.md -->

# Real-World Example: Dotfiles Manager

A complete example showing how to manage and deploy dotfiles using Mooncake.

## Features Demonstrated

- Multi-file organization
- Template rendering for dynamic configs
- File tree iteration
- Conditional deployment by OS
- Variable management
- Backup functionality
- Tag-based workflows

## Quick Start

```bash
# Deploy all dotfiles
mooncake run --config setup.yml

# Deploy only shell configs
mooncake run --config setup.yml --tags shell

# Preview what would be deployed
mooncake run --config setup.yml --dry-run
```

## Directory Structure

```
dotfiles-manager/
├── setup.yml              # Main entry point
├── vars.yml               # User configuration
├── dotfiles/              # Your actual dotfiles
│   ├── shell/
│   │   ├── .bashrc
│   │   └── .zshrc
│   ├── vim/
│   │   └── .vimrc
│   └── git/
│       └── .gitconfig
└── templates/             # Dynamic config templates
    ├── .tmux.conf.j2
    └── .config/
        └── nvim/
            └── init.lua.j2
```

## What It Does

1. Backs up existing dotfiles
2. Creates necessary directories
3. Deploys static dotfiles
4. Renders dynamic configs from templates
5. Sets appropriate permissions
6. OS-specific configuration

## Configuration

Edit `vars.yml` to customize:
```yaml
user_email: your@email.com
user_name: Your Name
editor: nvim
shell: zsh
color_scheme: gruvbox
```

## Usage

### Full Deployment
```bash
mooncake run --config setup.yml
```

### Selective Deployment
```bash
# Only shell configs
mooncake run --config setup.yml --tags shell

# Only vim/neovim
mooncake run --config setup.yml --tags vim

# Only git config
mooncake run --config setup.yml --tags git
```

### Backup Only
```bash
mooncake run --config setup.yml --tags backup
```

## Extending

### Adding New Dotfiles

1. Add file to `dotfiles/` directory
2. Add deployment step in `setup.yml`:
```yaml
- name: Deploy new config
  shell: cp {{ item.src }} ~/{{ item.name }}
  with_filetree: ./dotfiles/new-app
  tags:
    - new-app
```

### Adding Templates

1. Create template in `templates/`
2. Add rendering step:
```yaml
- name: Render new config
  template:
    src: ./templates/new-config.j2
    dest: ~/.config/new-app/config
  tags:
    - new-app
```

## Real-World Tips

1. **Version control** - Keep this in git
2. **Test first** - Use `--dry-run` before applying
3. **Incremental** - Add configs gradually
4. **Backup** - The example includes backup steps
5. **Document** - Add comments for custom settings

## See Also

This example combines concepts from:
- [06-loops](../../06-loops/) - File iteration
- [05-templates](../../05-templates/) - Config rendering
- [08-tags](../../08-tags/) - Selective deployment
- [10-multi-file-configs](../../10-multi-file-configs/) - Organization


---

<!-- FILE: examples/real-world-dotfiles.md -->

# Real-World Example: Dotfiles Manager

A complete example showing how to manage and deploy dotfiles using Mooncake.

## Features Demonstrated

- Multi-file organization
- Template rendering for dynamic configs
- File tree iteration
- Conditional deployment by OS
- Variable management
- Backup functionality
- Tag-based workflows

## Quick Start

```bash
cd examples/real-world/dotfiles-manager

# Deploy all dotfiles
mooncake run --config setup.yml

# Deploy only shell configs
mooncake run --config setup.yml --tags shell

# Preview what would be deployed
mooncake run --config setup.yml --dry-run
```

## Directory Structure

```
dotfiles-manager/
├── setup.yml              # Main entry point
├── vars.yml               # User configuration
├── dotfiles/              # Your actual dotfiles
│   ├── shell/
│   │   ├── .bashrc
│   │   └── .zshrc
│   ├── vim/
│   │   └── .vimrc
│   └── git/
│       └── .gitconfig
└── templates/             # Dynamic config templates
    ├── .tmux.conf.j2
    └── .config/
        └── nvim/
            └── init.lua.j2
```

## What It Does

1. Backs up existing dotfiles
2. Creates necessary directories
3. Deploys static dotfiles
4. Renders dynamic configs from templates
5. Sets appropriate permissions
6. OS-specific configuration

## Configuration

Edit `vars.yml` to customize:
```yaml
user_email: your@email.com
user_name: Your Name
editor: nvim
shell: zsh
color_scheme: gruvbox
```

## Usage

### Full Deployment
```bash
mooncake run --config setup.yml
```

### Selective Deployment
```bash
# Only shell configs
mooncake run --config setup.yml --tags shell

# Only vim/neovim
mooncake run --config setup.yml --tags vim

# Only git config
mooncake run --config setup.yml --tags git
```

### Backup Only
```bash
mooncake run --config setup.yml --tags backup
```

## Extending

### Adding New Dotfiles

1. Add file to `dotfiles/` directory
2. Add deployment step in `setup.yml`:
```yaml
- name: Deploy new config
  shell: cp {{ item.src }} ~/{{ item.name }}
  with_filetree: ./dotfiles/new-app
  tags:
    - new-app
```

### Adding Templates

1. Create template in `templates/`
2. Add rendering step:
```yaml
- name: Render new config
  template:
    src: ./templates/new-config.j2
    dest: ~/.config/new-app/config
  tags:
    - new-app
```

## Real-World Tips

1. **Version control** - Keep this in git
2. **Test first** - Use `--dry-run` before applying
3. **Incremental** - Add configs gradually
4. **Backup** - The example includes backup steps
5. **Document** - Add comments for custom settings

## See Also

This example combines concepts from:
- [06-loops](06-loops.md) - File iteration
- [05-templates](05-templates.md) - Config rendering
- [08-tags](08-tags.md) - Selective deployment
- [10-multi-file-configs](10-multi-file-configs.md) - Organization


---

<!-- FILE: examples/register/README.md -->

# 07 - Register

Learn how to capture command output and use it in subsequent steps.

## What You'll Learn

- Capturing output with `register`
- Accessing stdout, stderr, and return codes
- Using captured data in conditionals
- Detecting if operations made changes

## Quick Start

```bash
mooncake run --config config.yml
```

## What It Does

1. Checks if git is installed and captures the result
2. Uses return code to conditionally show messages
3. Captures username and uses it in file paths
4. Captures OS version and displays it
5. Detects if file operations made changes

## Key Concepts

### Basic Registration

```yaml
- name: Check if git exists
  shell: which git
  register: git_check

- name: Use the result
  shell: echo "Git is at {{ git_check.stdout }}"
  when: git_check.rc == 0
```

### Available Fields

After registering a result, you can access:

**For shell commands:**
- `register_name.stdout` - Standard output
- `register_name.stderr` - Standard error
- `register_name.rc` - Return/exit code (0 = success)
- `register_name.failed` - Boolean, true if rc != 0
- `register_name.changed` - Boolean, always true for shell

**For file operations:**
- `register_name.rc` - 0 for success, 1 for failure
- `register_name.failed` - Boolean, true if operation failed
- `register_name.changed` - Boolean, true if file created/modified

**For template operations:**
- `register_name.rc` - 0 for success, 1 for failure
- `register_name.failed` - Boolean, true if rendering failed
- `register_name.changed` - Boolean, true if output file changed

### Using in Conditionals

Check return codes:
```yaml
- shell: test -f /tmp/file.txt
  register: file_check

- shell: echo "File exists"
  when: file_check.rc == 0

- shell: echo "File not found"
  when: file_check.rc != 0
```

### Using in Templates

Use captured data anywhere:
```yaml
- shell: whoami
  register: current_user

- file:
    path: "/tmp/{{ current_user.stdout }}_config.txt"
    state: file
    content: "User: {{ current_user.stdout }}"
```

### Change Detection

Know if operations actually changed something:
```yaml
- file:
    path: /tmp/test.txt
    state: file
    content: "test"
  register: result

- shell: echo "File was created or modified"
  when: result.changed == true
```

## Common Patterns

### Checking for Command Existence

```yaml
- shell: which docker
  register: docker_check

- shell: echo "Docker not installed"
  when: docker_check.rc != 0
```

### Conditional Installation

```yaml
- shell: python3 --version
  register: python_check

- shell: apt install python3
  become: true
  when: python_check.rc != 0
```

### Using Command Output

```yaml
- shell: hostname
  register: host

- shell: echo "Running on {{ host.stdout }}"
```

## Testing

```bash
# Run the example
mooncake run --config config.yml

# Check created file
cat /tmp/$(whoami)_config.txt
```

## Next Steps

→ Continue to [08-tags](../08-tags/) to learn about filtering execution with tags.


---

<!-- FILE: examples/scenarios/docker-stack/README.md -->

# Docker Stack Setup

Install Docker and Docker Compose on Ubuntu, then deploy a simple multi-container stack.

## What This Does

This scenario demonstrates:
- Installing Docker Engine from official Docker repository
- Installing Docker Compose plugin
- Building a custom Flask application image
- Orchestrating multiple containers with docker-compose
- Setting up nginx as a reverse proxy for the Flask app
- Container networking and health checks
- Managing user permissions for Docker

## Prerequisites

- Ubuntu 20.04 or later
- Root/sudo access
- Mooncake installed
- Internet connection

## Files

- `setup.yml` - Main deployment playbook
- `files/app.py` - Flask web application
- `files/Dockerfile` - Container image definition
- `templates/docker-compose.yml.j2` - Multi-container orchestration config
- `templates/nginx.conf.j2` - Nginx reverse proxy configuration

## How to Run

```bash
# Run the setup
mooncake run setup.yml

# Or with custom settings
mooncake run setup.yml --var project_name=mystack --var nginx_port=9090
```

## Variables

You can customize these variables:

- `project_name` (default: `mooncake-stack`) - Project name
- `project_dir` (default: `/opt/{{ project_name }}`) - Project directory
- `app_port` (default: `5000`) - Flask application port
- `nginx_port` (default: `8080`) - Nginx public port
- `docker_user` (default: current user) - User to add to docker group

## What Gets Deployed

### Docker Components
- Docker Engine (latest stable)
- Docker Compose plugin
- containerd runtime
- Docker Buildx plugin

### Container Stack
- **Flask App Container** - Python web application
- **Nginx Container** - Reverse proxy and load balancer

### Network
- Custom bridge network for container communication
- Port mappings for external access

## Stack Architecture

```
Internet
    |
    | :8080
    v
[Nginx Container]
    |
    | internal network
    |
    v
[Flask App Container] :5000
```

## Using Your Stack

### Access the Application

```bash
# Through nginx (production-like)
curl http://localhost:8080

# Direct app access
curl http://localhost:5000

# API endpoints
curl http://localhost:8080/api/info
curl http://localhost:8080/api/health
curl http://localhost:8080/api/env
```

### Docker Compose Commands

```bash
cd /opt/mooncake-stack

# View running containers
sudo docker compose ps

# View logs
sudo docker compose logs
sudo docker compose logs -f        # Follow logs
sudo docker compose logs app       # Specific service

# Restart services
sudo docker compose restart
sudo docker compose restart app    # Specific service

# Stop the stack
sudo docker compose down

# Stop and remove volumes
sudo docker compose down -v

# Rebuild and restart
sudo docker compose up -d --build

# Scale services (if stateless)
sudo docker compose up -d --scale app=3
```

### Docker Commands

```bash
# List all containers
sudo docker ps -a

# View container stats
sudo docker stats

# Execute command in container
sudo docker exec -it mooncake-stack-app bash

# View container logs
sudo docker logs mooncake-stack-app

# Inspect container
sudo docker inspect mooncake-stack-app

# View images
sudo docker images

# Remove unused resources
sudo docker system prune
```

### Using Docker Without Sudo

After setup, you'll need to re-login or run:

```bash
newgrp docker
```

Then you can use docker without sudo:

```bash
docker ps
docker compose ps
```

## Project Structure

```
/opt/mooncake-stack/
├── docker-compose.yml    # Orchestration config
├── nginx.conf            # Nginx configuration
└── app/
    ├── Dockerfile        # Image definition
    └── app.py           # Flask application
```

## Customizing the Application

### Modify Flask App

```bash
sudo nano /opt/mooncake-stack/app/app.py
```

### Rebuild and Deploy

```bash
cd /opt/mooncake-stack
sudo docker compose up -d --build
```

### Add Environment Variables

Edit `docker-compose.yml`:

```yaml
services:
  app:
    environment:
      - MY_VAR=value
      - DATABASE_URL=postgresql://...
```

## Monitoring and Debugging

### Check Container Health

```bash
sudo docker compose ps
sudo docker inspect mooncake-stack-app | grep -A 10 Health
```

### View Resource Usage

```bash
sudo docker stats
```

### Troubleshooting

```bash
# View detailed logs
sudo docker compose logs --tail=100

# Check if containers are running
sudo docker compose ps

# Restart problematic service
sudo docker compose restart app

# Rebuild from scratch
sudo docker compose down
sudo docker compose up -d --build

# Check Docker daemon status
sudo systemctl status docker

# View Docker daemon logs
sudo journalctl -u docker -f
```

## Cleanup

To remove the stack:

```bash
# Stop and remove containers
cd /opt/mooncake-stack
sudo docker compose down

# Remove project directory
sudo rm -rf /opt/mooncake-stack

# Remove Docker images
sudo docker rmi mooncake-stack-app nginx:alpine python:3.11-slim

# Optionally remove Docker completely
sudo systemctl stop docker
sudo apt-get remove --purge docker-ce docker-ce-cli containerd.io docker-compose-plugin
sudo rm -rf /var/lib/docker
```

## Learning Points

This example teaches:
- Installing Docker from official repositories
- Building custom Docker images with Dockerfile
- Multi-container orchestration with Docker Compose
- Container networking and service discovery
- Reverse proxy configuration with Nginx
- Container health checks
- Volume management
- Docker security basics (user groups)
- Container logs and monitoring

## Production Considerations

For production deployments, also consider:

- **Security:**
  - Use specific image tags, not `latest`
  - Scan images for vulnerabilities
  - Run containers as non-root users
  - Use secrets management

- **Reliability:**
  - Implement proper health checks
  - Configure restart policies
  - Set resource limits (CPU, memory)
  - Use volume backups

- **Monitoring:**
  - Centralized logging (ELK, Grafana Loki)
  - Metrics collection (Prometheus)
  - Alerting (Alertmanager)

- **Scaling:**
  - Load balancing across multiple instances
  - Container orchestration (Kubernetes)
  - Database connection pooling
  - Caching layers (Redis)

## Next Steps

After deployment, try:
- Adding a PostgreSQL database service
- Implementing Redis for caching
- Adding more API endpoints
- Setting up SSL with Let's Encrypt
- Deploying your own application
- Exploring Docker Swarm or Kubernetes
- Adding monitoring with Prometheus and Grafana


---

<!-- FILE: examples/scenarios/nginx-ubuntu/README.md -->

# Nginx Ubuntu Setup

A simple "hello world" example that sets up an nginx web server on Ubuntu.

## What This Does

This scenario demonstrates:
- Installing nginx via apt
- Creating site configurations using templates
- Deploying static content
- Managing nginx service
- Verifying the setup with assertions

## Prerequisites

- Ubuntu 20.04 or later
- Root/sudo access
- Mooncake installed

## Files

- `setup.yml` - Main playbook
- `templates/nginx.conf.j2` - Nginx main configuration template
- `templates/site.conf.j2` - Site-specific configuration template
- `files/index.html` - Welcome page

## How to Run

```bash
# Run the setup
mooncake run setup.yml

# Or with custom variables
mooncake run setup.yml --var site_name=myapp --var site_port=9090
```

## Variables

You can customize these variables:

- `site_name` (default: `mysite`) - Name of your site
- `site_port` (default: `8080`) - Port to listen on
- `document_root` (default: `/var/www/{{ site_name }}`) - Root directory for site files

## What Gets Created

- Nginx installation via apt
- Site directory: `/var/www/mysite/`
- Nginx config: `/etc/nginx/nginx.conf`
- Site config: `/etc/nginx/sites-available/mysite`
- Symlink: `/etc/nginx/sites-enabled/mysite`
- Welcome page with styled HTML

## Testing

After running, test your site:

```bash
# Check nginx status
sudo systemctl status nginx

# Test the site
curl http://localhost:8080

# View in browser
firefox http://localhost:8080
```

## Cleanup

To remove the setup:

```bash
sudo systemctl stop nginx
sudo apt-get remove --purge nginx nginx-common
sudo rm -rf /var/www/mysite
sudo rm -f /etc/nginx/sites-available/mysite /etc/nginx/sites-enabled/mysite
```

## Learning Points

This example teaches:
- Installing packages with shell actions
- Using templates for configuration files
- File management (directories, copies, symlinks)
- Service management
- Using assert to verify success
- Using register and print for debugging


---

<!-- FILE: examples/scenarios/nodejs-webapp/README.md -->

# Node.js Web App Deployment

Deploy a simple Node.js Express application with PM2 process manager and nginx reverse proxy.

## What This Does

This scenario demonstrates a complete web application deployment stack:
- Installing Node.js and npm from NodeSource
- Creating an Express.js web application
- Managing the app with PM2 process manager
- Setting up nginx as a reverse proxy
- Configuring logging and health checks
- Verifying the deployment

## Prerequisites

- Ubuntu 20.04 or later
- Root/sudo access
- Mooncake installed
- Internet connection

## Files

- `deploy.yml` - Main deployment playbook
- `files/app.js` - Express.js application
- `files/package.json` - Node.js dependencies
- `templates/ecosystem.config.js.j2` - PM2 configuration template
- `templates/nginx-proxy.conf.j2` - Nginx reverse proxy config template

## How to Run

```bash
# Run the deployment
mooncake run deploy.yml

# Or with custom settings
mooncake run deploy.yml --var app_name=mywebapp --var app_port=3000 --var nginx_port=80
```

## Variables

You can customize these variables:

- `app_name` (default: `myapp`) - Application name
- `app_port` (default: `3000`) - Application port
- `app_dir` (default: `/opt/{{ app_name }}`) - Application directory
- `nginx_port` (default: `80`) - Nginx listen port
- `node_user` (default: `www-data`) - User to run the app

## What Gets Deployed

### System Components
- Node.js LTS (from NodeSource)
- npm (Node Package Manager)
- PM2 (Process Manager)
- nginx (Reverse Proxy)

### Application Stack
- Express.js web framework
- PM2 process management with auto-restart
- Nginx reverse proxy with proper headers
- Logging to files and systemd
- Health check endpoint

### File Structure
```
/opt/myapp/
├── app.js                    # Main application
├── package.json              # Dependencies
├── ecosystem.config.js       # PM2 config
├── node_modules/             # Installed packages
└── logs/                     # Application logs
```

## Using Your Application

### Access the App

```bash
# Through nginx (public facing)
curl http://localhost

# Direct access
curl http://localhost:3000

# API endpoint
curl http://localhost/api/status

# Health check
curl http://localhost/health
```

### PM2 Management

```bash
# View running apps
sudo -u www-data pm2 list

# View logs
sudo -u www-data pm2 logs myapp

# Restart app
sudo -u www-data pm2 restart myapp

# Stop app
sudo -u www-data pm2 stop myapp

# Monitor
sudo -u www-data pm2 monit
```

### View Logs

```bash
# Application logs
sudo -u www-data pm2 logs

# Nginx access logs
sudo tail -f /var/log/nginx/myapp_access.log

# Nginx error logs
sudo tail -f /var/log/nginx/myapp_error.log
```

### Nginx Management

```bash
# Check status
sudo systemctl status nginx

# Restart nginx
sudo systemctl restart nginx

# Test configuration
sudo nginx -t
```

## Modifying the Application

Edit the application code:

```bash
sudo nano /opt/myapp/app.js
```

After making changes, restart with PM2:

```bash
sudo -u www-data pm2 restart myapp
```

## Cleanup

To remove the deployment:

```bash
# Stop and remove PM2 process
sudo -u www-data pm2 delete myapp
sudo -u www-data pm2 save

# Remove application
sudo rm -rf /opt/myapp

# Remove nginx config
sudo rm -f /etc/nginx/sites-{available,enabled}/myapp
sudo systemctl restart nginx

# Optionally remove Node.js
sudo apt-get remove --purge nodejs npm
```

## Learning Points

This example teaches:
- Installing Node.js from NodeSource repository
- Creating Express.js applications
- Using PM2 for process management
- Configuring nginx as a reverse proxy
- Managing file ownership and permissions
- Setting up proper logging
- Health checks and monitoring
- Service management with systemd

## Production Considerations

For production use, also consider:
- SSL/TLS certificates with Let's Encrypt
- Environment variable management
- Database connections
- Monitoring and alerting
- Load balancing with multiple instances
- Log rotation
- Security hardening
- Firewall configuration

## Next Steps

After deployment, try:
- Modify the app to add new routes
- Scale with PM2: `pm2 scale myapp 4`
- Add SSL with certbot
- Connect to a database
- Deploy your own Node.js application


---

<!-- FILE: examples/scenarios/postgresql-db/README.md -->

# PostgreSQL Database Setup

Install and configure PostgreSQL on Ubuntu with a sample database schema.

## What This Does

This scenario demonstrates:
- Installing PostgreSQL database server
- Starting and enabling the PostgreSQL service
- Creating a database and user
- Granting proper permissions
- Loading initial schema with tables and data
- Verifying database connectivity
- Running queries to test the setup

## Prerequisites

- Ubuntu 20.04 or later
- Root/sudo access
- Mooncake installed

## Files

- `setup.yml` - Main playbook
- `files/init.sql` - Initial database schema and sample data

## How to Run

```bash
# Run the setup
mooncake run setup.yml

# Or with custom database settings
mooncake run setup.yml --var db_name=mydb --var db_user=myuser --var db_password=mypass123
```

## Variables

You can customize these variables:

- `db_name` (default: `myapp_db`) - Database name
- `db_user` (default: `myapp_user`) - Database user
- `db_password` (default: `myapp_password_123`) - User password
- `postgres_version` (default: `14`) - PostgreSQL version

## What Gets Created

### Database Objects

**Database:** `myapp_db`

**Tables:**
- `users` - User accounts with username, email, full_name
- `posts` - User posts with title and content

**Views:**
- `active_users` - View of active users only

**Functions:**
- `get_user_post_count()` - Count posts for a user

**Sample Data:**
- 4 users (Alice, Bob, Charlie, Diana)
- 4 posts

### Indexes
- Username index
- Email index
- User ID foreign key index

## Using Your Database

### Connect with psql

```bash
# As the created user
PGPASSWORD=myapp_password_123 psql -h localhost -U myapp_user -d myapp_db

# As postgres superuser
sudo -u postgres psql myapp_db
```

### Sample Queries

```sql
-- List all users
SELECT * FROM users;

-- List active users
SELECT * FROM active_users;

-- List posts with usernames
SELECT u.username, p.title, p.created_at
FROM posts p
JOIN users u ON p.user_id = u.id
ORDER BY p.created_at DESC;

-- Get post count for a user
SELECT get_user_post_count(1);

-- Insert a new user
INSERT INTO users (username, email, full_name)
VALUES ('eve', 'eve@example.com', 'Eve Wilson');

-- Insert a new post
INSERT INTO posts (user_id, title, content)
VALUES (1, 'My New Post', 'This is my newest post!');
```

### Python Connection Example

```python
import psycopg2

conn = psycopg2.connect(
    host="localhost",
    database="myapp_db",
    user="myapp_user",
    password="myapp_password_123"
)

cur = conn.cursor()
cur.execute("SELECT * FROM users;")
rows = cur.fetchall()

for row in rows:
    print(row)

cur.close()
conn.close()
```

### Node.js Connection Example

```javascript
const { Client } = require('pg');

const client = new Client({
  host: 'localhost',
  database: 'myapp_db',
  user: 'myapp_user',
  password: 'myapp_password_123',
});

client.connect();

client.query('SELECT * FROM users', (err, res) => {
  console.log(res.rows);
  client.end();
});
```

## Database Management

### Check Status

```bash
sudo systemctl status postgresql
```

### View Logs

```bash
sudo tail -f /var/log/postgresql/postgresql-*-main.log
```

### Backup Database

```bash
# As postgres user
sudo -u postgres pg_dump myapp_db > myapp_db_backup.sql

# As created user
PGPASSWORD=myapp_password_123 pg_dump -h localhost -U myapp_user myapp_db > backup.sql
```

### Restore Database

```bash
# As postgres user
sudo -u postgres psql myapp_db < myapp_db_backup.sql

# As created user
PGPASSWORD=myapp_password_123 psql -h localhost -U myapp_user myapp_db < backup.sql
```

### Access PostgreSQL Shell

```bash
# As postgres superuser
sudo -u postgres psql

# List databases
\l

# Connect to database
\c myapp_db

# List tables
\dt

# Describe table
\d users

# List users/roles
\du

# Quit
\q
```

## Cleanup

To remove the database setup:

```bash
# Drop database and user
sudo -u postgres psql -c "DROP DATABASE IF EXISTS myapp_db;"
sudo -u postgres psql -c "DROP USER IF EXISTS myapp_user;"

# Optionally remove PostgreSQL
sudo systemctl stop postgresql
sudo apt-get remove --purge postgresql postgresql-contrib
sudo rm -rf /var/lib/postgresql/
sudo rm -rf /etc/postgresql/
```

## Learning Points

This example teaches:
- Installing PostgreSQL from Ubuntu repositories
- Starting and managing PostgreSQL service
- Creating databases and users programmatically
- Setting up proper database permissions
- Running SQL scripts from files
- Testing database connectivity
- Basic SQL operations (CREATE, INSERT, SELECT)
- Using views and functions
- Database security basics

## Security Notes

**Important:** This example uses a simple password for demonstration. In production:

- Use strong, randomly generated passwords
- Store passwords in environment variables or secret management
- Configure `pg_hba.conf` for proper authentication
- Enable SSL/TLS connections
- Use connection pooling
- Regular backups and monitoring
- Keep PostgreSQL updated

## Next Steps

After setup, try:
- Adding more tables and relationships
- Creating triggers and stored procedures
- Setting up replication
- Configuring pgAdmin for GUI management
- Implementing full-text search
- Adding PostGIS for spatial data
- Performance tuning and optimization


---

<!-- FILE: examples/scenarios/python-ml-lab/README.md -->

# Python ML Lab Setup

Set up a complete Python machine learning environment on Ubuntu with popular ML libraries.

## What This Does

This scenario demonstrates:
- Installing Python 3 and pip
- Creating a Python virtual environment
- Installing ML packages (numpy, pandas, matplotlib, scikit-learn, jupyter)
- Setting up a workspace for ML projects
- Running a simple ML demo script

## Prerequisites

- Ubuntu 20.04 or later
- Root/sudo access
- Mooncake installed
- Internet connection for package downloads

## Files

- `setup.yml` - Main playbook
- `files/requirements.txt` - Python package requirements
- `files/hello_ml.py` - Sample ML demonstration script

## How to Run

```bash
# Run the setup
mooncake run setup.yml

# Or with custom workspace location
mooncake run setup.yml --var workspace_dir=$HOME/my-ml-workspace
```

## Variables

You can customize these variables:

- `workspace_dir` (default: `$HOME/ml-workspace`) - Workspace directory path
- `venv_dir` (default: `{{ workspace_dir }}/venv`) - Virtual environment path
- `python_version` (default: `3`) - Python version

## What Gets Installed

### System Packages
- python3
- python3-pip
- python3-venv

### Python ML Packages
- numpy - Numerical computing
- pandas - Data analysis
- matplotlib - Plotting and visualization
- scikit-learn - Machine learning algorithms
- jupyter - Interactive notebooks
- seaborn - Statistical data visualization
- scipy - Scientific computing

## Using Your ML Environment

### Activate Virtual Environment

```bash
source ~/ml-workspace/venv/bin/activate
```

### Run Sample Script

```bash
source ~/ml-workspace/venv/bin/activate
python3 ~/ml-workspace/hello_ml.py
```

### Start Jupyter Notebook

```bash
source ~/ml-workspace/venv/bin/activate
jupyter notebook --notebook-dir=~/ml-workspace/notebooks
```

Then open your browser to the URL shown (usually http://localhost:8888).

### Create Your First ML Project

```bash
cd ~/ml-workspace
source venv/bin/activate

# Create a new Python script
nano my_analysis.py

# Or create a new notebook
jupyter notebook notebooks/
```

## Sample Script

The included `hello_ml.py` demonstrates:
1. NumPy - Creating and manipulating arrays
2. Pandas - Creating and analyzing DataFrames
3. Scikit-learn - Training a simple classification model

## Cleanup

To remove the ML environment:

```bash
rm -rf ~/ml-workspace
sudo apt-get remove --purge python3-pip python3-venv
```

## Learning Points

This example teaches:
- Installing system packages with apt
- Creating Python virtual environments
- Installing Python packages with pip
- Managing workspace directories
- Running Python scripts from Mooncake
- Using assertions to verify installations
- Organizing ML project structure

## Next Steps

After setup, try:
- Creating Jupyter notebooks in `~/ml-workspace/notebooks/`
- Installing additional packages: `pip install tensorflow pytorch`
- Following scikit-learn tutorials
- Exploring kaggle datasets


---

<!-- FILE: examples/sudo/README.md -->

# 09 - Sudo / Privilege Escalation

Learn how to execute commands and operations with elevated privileges.

## What You'll Learn

- Using `become: true` for sudo operations
- Providing sudo password via CLI
- System-level operations
- OS-specific privileged operations

## Quick Start

```bash
# Interactive prompt (recommended)
mooncake run --config config.yml --ask-become-pass

# Or using short flag
mooncake run --config config.yml -K

# Preview what would run with sudo
mooncake run --config config.yml -K --dry-run
```

⚠️ **Warning:** This example contains commands that require root privileges. Review the config before running!

## What It Does

1. Runs regular command (no sudo)
2. Runs privileged command with sudo
3. Updates package list (Linux)
4. Installs system packages
5. Creates system directories and files

## Key Concepts

### Basic Sudo

Add `become: true` to run with sudo:
```yaml
- name: System operation
  shell: apt update
  become: true
```

### Providing Password

Four ways to provide sudo password (mutually exclusive):

**1. Interactive prompt (recommended):**
```bash
mooncake run --config config.yml --ask-become-pass
# or
mooncake run --config config.yml -K
```
Password is hidden while typing. Most secure option.

**2. File-based (secure for automation):**
```bash
echo "mypassword" > ~/.mooncake/sudo_pass
chmod 0600 ~/.mooncake/sudo_pass
mooncake run --config config.yml --sudo-pass-file ~/.mooncake/sudo_pass
```
⚠️ File must have 0600 permissions and be owned by current user.

**3. SUDO_ASKPASS (password manager integration):**
```bash
export SUDO_ASKPASS=/usr/bin/ssh-askpass
mooncake run --config config.yml
```
Uses external helper program for password input.

**4. Command line (insecure, not recommended):**
```bash
mooncake run --config config.yml --sudo-pass mypassword --insecure-sudo-pass
```
⚠️ **WARNING:** Password visible in shell history and process list. Requires explicit `--insecure-sudo-pass` flag.

**Security Features:**
- Passwords are automatically redacted from all log output
- Only one password method can be used at a time
- File permissions are strictly validated

### Which Operations Need Sudo?

**Typically require sudo:**
- Package management (`apt`, `yum`, `dnf`)
- System file operations (`/etc`, `/opt`, `/usr/local`)
- Service management (`systemctl`)
- User/group management
- Mounting filesystems
- Network configuration

**Don't require sudo:**
- User-space operations
- Home directory files
- `/tmp` directory
- Homebrew on macOS (usually)

### File Operations with Sudo

Create system directories:
```yaml
- name: Create system directory
  file:
    path: /opt/myapp
    state: directory
    mode: "0755"
  become: true
```

Create system files:
```yaml
- name: Create system config
  file:
    path: /etc/myapp/config.yml
    state: file
    content: "config: value"
  become: true
```

### OS-Specific Sudo

```yaml
# Linux package management
- name: Install package (Linux)
  shell: apt install -y curl
  become: true
  when: os == "linux" and package_manager == "apt"

# macOS typically doesn't need sudo for homebrew
- name: Install package (macOS)
  shell: brew install curl
  when: os == "darwin"
```

## Security Considerations

1. **Review before running** - Check what commands will execute with sudo
2. **Use dry-run** - Preview with `--dry-run` first
3. **Minimize sudo usage** - Only use on steps that require it
4. **Specific commands** - Don't use `become: true` on untrusted commands
5. **Password input** - Use interactive prompt or file-based methods, avoid CLI flag
6. **Password redaction** - Passwords are automatically redacted from logs (debug, stdout, stderr)
7. **File permissions** - If using `--sudo-pass-file`, ensure 0600 permissions
8. **Platform support** - Only works on Linux and macOS (explicitly fails on Windows)

## Common Use Cases

### Package Installation

```yaml
- name: Install system packages
  shell: |
    apt update
    apt install -y nginx postgresql
  become: true
  when: os == "linux"
```

### System Service Setup

```yaml
- name: Create systemd service
  template:
    src: ./myapp.service.j2
    dest: /etc/systemd/system/myapp.service
    mode: "0644"
  become: true

- name: Enable service
  shell: systemctl enable myapp
  become: true
```

### System Directory Setup

```yaml
- name: Create application directories
  file:
    path: "{{ item }}"
    state: directory
    mode: "0755"
  become: true
  with_items:
    - /opt/myapp
    - /etc/myapp
    - /var/log/myapp
```

## Testing

```bash
# Preview what will run with sudo
mooncake run --config config.yml -K --dry-run

# Run with sudo
mooncake run --config config.yml -K

# Check created system files
sudo ls -la /opt/myapp/

# Verify password redaction in debug logs
mooncake run --config config.yml -K --log-level debug | grep -i password
# Should show [REDACTED] instead of actual password
```

## Troubleshooting

**"step requires sudo but no password provided"**
- Provide password using `--ask-become-pass`, `--sudo-pass-file`, or `SUDO_ASKPASS`

**"--sudo-pass requires --insecure-sudo-pass flag"**
- CLI password flag requires explicit security acknowledgment
- Use `--ask-become-pass` instead (more secure)

**"password file must have 0600 permissions"**
- Fix permissions: `chmod 0600 /path/to/password/file`
- Verify ownership: `ls -l /path/to/password/file`

**"only one password method can be specified"**
- Remove conflicting password flags
- Use only one of: `--ask-become-pass`, `--sudo-pass-file`, or `--sudo-pass`

**"become is not supported on windows"**
- Privilege escalation only works on Linux and macOS
- Use platform-specific conditionals with `when`

**Permission denied without sudo**
- Add `become: true` to the step

**Command not found**
- Check if command exists: `which <command>`
- Some commands need full paths with sudo

## Next Steps

→ Continue to [10-multi-file-configs](../10-multi-file-configs/) to learn about organizing large configurations.


---

<!-- FILE: examples/tags/README.md -->

# 08 - Tags

Learn how to use tags to selectively run parts of your configuration.

## What You'll Learn

- Adding tags to steps
- Filtering execution with `--tags` flag
- Organizing workflows with tags
- Combining tags with conditionals

## Quick Start

```bash
# Run all steps (no tag filter)
mooncake run --config config.yml

# Run only development steps
mooncake run --config config.yml --tags dev

# Run only production steps
mooncake run --config config.yml --tags prod

# Run test-related steps
mooncake run --config config.yml --tags test

# Run multiple tag categories
mooncake run --config config.yml --tags dev,test
```

## What It Does

Demonstrates different tagged workflows:
- Development setup
- Production deployment
- Testing
- Security audits
- Staging deployment

## Key Concepts

### Adding Tags

```yaml
- name: Install dev tools
  shell: echo "Installing tools"
  tags:
    - dev
    - tools
```

### Tag Filtering Behavior

**No tags specified:**
- All steps run (including untagged steps)

**Tags specified (`--tags dev`):**
- Only steps with matching tags run
- Untagged steps are skipped

**Multiple tags (`--tags dev,prod`):**
- Steps run if they have ANY of the specified tags
- OR logic: matches `dev` OR `prod`

### Tag Organization Strategies

**By Environment:**
```yaml
tags: [dev, staging, prod]
```

**By Phase:**
```yaml
tags: [setup, deploy, test, cleanup]
```

**By Component:**
```yaml
tags: [database, webserver, cache]
```

**By Role:**
```yaml
tags: [install, configure, security]
```

### Multiple Tags Per Step

Steps can have multiple tags:
```yaml
- name: Security audit
  shell: run-security-scan
  tags:
    - test
    - prod
    - security
```

This runs with:
- `--tags test` ✓
- `--tags prod` ✓
- `--tags security` ✓
- `--tags dev` ✗

## Real-World Examples

### Development Workflow

```bash
# Install dev tools only
mooncake run --config config.yml --tags dev,tools

# Run tests
mooncake run --config config.yml --tags test
```

### Production Deployment

```bash
# Deploy to production
mooncake run --config config.yml --tags prod,deploy

# Run security checks
mooncake run --config config.yml --tags security,prod
```

### Staging Environment

```bash
# Deploy to staging
mooncake run --config config.yml --tags staging,deploy
```

## Combining Tags and Conditionals

```yaml
- name: Install Linux dev tools
  shell: apt install build-essential
  become: true
  when: os == "linux"
  tags:
    - dev
    - tools
```

Both must match:
1. Condition must be true (`os == "linux"`)
2. Tag must match (if `--tags` specified)

## Testing Different Tag Filters

```bash
# Preview what runs with dev tag
mooncake run --config config.yml --tags dev --dry-run

# Run dev and test steps
mooncake run --config config.yml --tags dev,test

# Run only setup steps
mooncake run --config config.yml --tags setup
```

## Best Practices

1. **Use consistent naming** - Pick a scheme (env, phase, role) and stick to it
2. **Multiple tags per step** - Makes filtering more flexible
3. **Document your tags** - In README or comments
4. **Combine with conditionals** - For environment + OS filtering

## Next Steps

→ Continue to [09-sudo](../09-sudo/) to learn about privilege escalation.


---

<!-- FILE: examples/templates/README.md -->

# 05 - Templates

Learn how to render configuration files from templates using pongo2 syntax.

## What You'll Learn

- Rendering `.j2` template files
- Using variables in templates
- Template conditionals (`{% if %}`)
- Template loops (`{% for %}`)
- Passing additional vars to templates

## Quick Start

```bash
mooncake run --config config.yml
```

Check the rendered files:
```bash
ls -lh /tmp/mooncake-templates/
cat /tmp/mooncake-templates/config.yml
```

## What It Does

1. Defines variables for application, server, and database config
2. Renders application config with loops and conditionals
3. Renders nginx config with optional SSL
4. Creates executable script from template
5. Renders same template with different variables

## Key Concepts

### Template Action

```yaml
- name: Render config
  template:
    src: ./templates/config.yml.j2
    dest: /tmp/config.yml
    mode: "0644"
```

### Template Syntax (pongo2)

**Variables:**
```jinja
{{ variable_name }}
{{ nested.property }}
```

**Conditionals:**
```jinja
{% if debug %}
  debug: true
{% else %}
  debug: false
{% endif %}
```

**Loops:**
```jinja
{% for item in items %}
  - {{ item }}
{% endfor %}
```

**Filters:**
```jinja
{{ path | expanduser }}  # Expands ~ to home directory
{{ text | upper }}       # Convert to uppercase
```

### Passing Additional Vars

Override variables for specific templates:
```yaml
- template:
    src: ./templates/config.yml.j2
    dest: /tmp/prod-config.yml
    vars:
      environment: production
      debug: false
```

## Template Files

### config.yml.j2
Application configuration with:
- Conditional debug settings
- Loops over features list
- Variable substitution

### nginx.conf.j2
Web server config with:
- Conditional SSL configuration
- Dynamic port and paths

### script.sh.j2
Executable shell script with:
- Shebang line
- Variable expansion
- Command loops

## Common Use Cases

- **Config files** - app.yml, nginx.conf, etc.
- **Shell scripts** - deployment scripts, setup scripts
- **Systemd units** - service files
- **Dotfiles** - .bashrc, .vimrc with customization

## Testing Templates

```bash
# Render templates
mooncake run --config config.yml

# View rendered output
cat /tmp/mooncake-templates/config.yml

# Check executable permissions
ls -la /tmp/mooncake-templates/deploy.sh
```

## Next Steps

→ Continue to [06-loops](../06-loops/) to learn about iterating over lists and files.


---

<!-- FILE: examples/variables-and-facts/README.md -->

# 02 - Variables and System Facts

Learn how to define custom variables and use Mooncake's comprehensive system facts.

## What You'll Learn

- Defining custom variables with `vars`
- Using all available system facts
- Combining custom variables with system facts
- Using variables in file operations

## Quick Start

```bash
mooncake run --config config.yml
```

## What It Does

1. Defines custom application variables
2. Displays all system facts (OS, hardware, network, software)
3. Creates files using both custom variables and system facts

## Key Concepts

### Custom Variables

Define your own variables:
```yaml
- vars:
    app_name: MyApp
    version: "1.0.0"
    environment: development
```

Use them in commands and paths:
```yaml
- shell: echo "Running {{app_name}} v{{version}}"
```

### System Facts

Mooncake automatically collects system information:

**Basic:**
- `os` - Operating system (linux, darwin, windows)
- `arch` - Architecture (amd64, arm64)
- `hostname` - System hostname
- `user_home` - User's home directory

**Hardware:**
- `cpu_cores` - Number of CPU cores
- `memory_total_mb` - Total RAM in megabytes

**Distribution:**
- `distribution` - Distribution name (ubuntu, debian, macos, etc.)
- `distribution_version` - Full version (e.g., "22.04")
- `distribution_major` - Major version number

**Software:**
- `package_manager` - Detected package manager (apt, yum, brew, etc.)
- `python_version` - Installed Python version

**Network:**
- `ip_addresses` - Array of IP addresses
- `ip_addresses_string` - Comma-separated IP addresses

### Variable Substitution

Variables work everywhere:
```yaml
- file:
    path: "/tmp/{{app_name}}-{{version}}-{{os}}"
    state: directory
```

## Seeing All Facts

Run `mooncake facts` to see all facts for your system:
```bash
mooncake facts
```

## Next Steps

→ Continue to [03-files-and-directories](../03-files-and-directories/) to learn about file operations.


---

<!-- FILE: guide/best-practices.md -->

# Best Practices

## 1. Always Use Dry-Run

Preview changes before applying:
```bash
mooncake run --config config.yml --dry-run
```

## 2. Organize by Purpose

```
project/
├── main.yml
├── tasks/
│   ├── common.yml
│   ├── dev.yml
│   └── prod.yml
└── vars/
    ├── dev.yml
    └── prod.yml
```

## 3. Use Variables

Make configs reusable:
```yaml
- vars:
    app_name: myapp
    version: "1.0.0"
```

## 4. Tag Your Workflows

```yaml
- name: Dev setup
  shell: install-dev-tools
  tags: [dev]

- name: Prod deploy
  shell: deploy-prod
  tags: [prod]
```

## 5. Document Conditions

```yaml
# Ubuntu 20+ only (older versions incompatible)
- name: Install package
  shell: apt install package
  when: distribution == "ubuntu" and distribution_major >= "20"
```

## 6. Use System Facts

```yaml
- shell: "{{package_manager}} install neovim"
  when: os == "linux"
```

## 7. Test Incrementally

1. Start simple
2. Test with `--dry-run`
3. Add complexity gradually
4. Use `--log-level debug`

## 8. Handle Errors

```yaml
- shell: which docker
  register: docker_check

- shell: install-docker
  when: docker_check.rc != 0
```


---

<!-- FILE: guide/commands.md -->

# Commands

## mooncake plan

Generate and inspect a deterministic execution plan from your configuration.

### Usage

```bash
mooncake plan --config <file> [options]
```

### Flags

| Flag | Description |
|------|-------------|
| `--config, -c` | Path to configuration file (required) |
| `--vars, -v` | Path to variables file |
| `--tags, -t` | Filter steps by tags |
| `--format, -f` | Output format: text, json, yaml (default: text) |
| `--show-origins` | Display file:line:col origin for each step |
| `--output, -o` | Save plan to file |

### What is a Plan?

A **plan** is a fully expanded, deterministic representation of your configuration:

- **All loops expanded** - `with_items` and `with_filetree` expanded to individual steps
- **All includes resolved** - Nested includes flattened into a linear sequence
- **Origin tracking** - Every step tracks its source file:line:col and include chain
- **Deterministic** - Same config always produces identical plan
- **Tag filtering** - Steps not matching tags are marked as `skipped`

### Examples

```bash
# View plan as text
mooncake plan --config config.yml

# View plan with origins
mooncake plan --config config.yml --show-origins

# Export plan as JSON
mooncake plan --config config.yml --format json

# Save plan to file
mooncake plan --config config.yml --format json --output plan.json

# Filter by tags
mooncake plan --config config.yml --tags dev

# With variables
mooncake plan --config config.yml --vars prod.yml
```

### Use Cases

- **Inspect expansions** - See exactly how loops and includes expand
- **Debug configurations** - Understand step ordering and variable resolution
- **Verify determinism** - Ensure same config produces same plan
- **CI/CD integration** - Export plans for review before execution
- **Traceability** - Track every step back to source file location

### Plan Output Format

**Text format** (default):
```
[1] Install package (ID: step-0001)
    Action: shell
    Loop: with_items[0] (first=true, last=false)

[2] Install package (ID: step-0002)
    Action: shell
    Loop: with_items[1] (first=false, last=false)
```

**With `--show-origins`:**
```
[1] Install package (ID: step-0001)
    Action: shell
    Origin: /path/to/config.yml:15:3
    Chain: main.yml:10 -> tasks/setup.yml:15

[2] Install package (ID: step-0002)
    Action: shell
    Origin: /path/to/config.yml:15:3
```

**JSON format** includes full step details:
```json
{
  "version": "1.0",
  "generated_at": "2026-02-04T10:30:00Z",
  "root_file": "/path/to/config.yml",
  "steps": [
    {
      "id": "step-0001",
      "name": "Install package",
      "origin": {
        "file": "/path/to/config.yml",
        "line": 15,
        "column": 3,
        "include_chain": ["main.yml:10", "tasks/setup.yml:15"]
      },
      "loop_context": {
        "type": "with_items",
        "item": "neovim",
        "index": 0,
        "first": true,
        "last": false
      },
      "action": {
        "type": "shell",
        "data": {
          "command": "brew install neovim"
        }
      }
    }
  ]
}
```

## mooncake run

Run a configuration file.

### Usage

```bash
mooncake run --config <file> [options]
```

### Flags

| Flag | Description |
|------|-------------|
| `--config, -c` | Path to configuration file (required, unless using --from-plan) |
| `--from-plan` | Execute from a saved plan file (JSON/YAML) |
| `--vars, -v` | Path to variables file |
| `--tags, -t` | Filter steps by tags |
| `--dry-run` | Preview without executing |
| **Privilege Escalation** ||
| `--ask-become-pass, -K` | Prompt for sudo password interactively (recommended) |
| `--sudo-pass-file` | Read sudo password from file (must have 0600 permissions) |
| `--sudo-pass, -s` | Sudo password (requires --insecure-sudo-pass) |
| `--insecure-sudo-pass` | Allow --sudo-pass flag (password visible in history) |
| **Display Options** ||
| `--raw, -r` | Disable animated TUI |
| `--log-level, -l` | Log level (debug, info, error) |

### Examples

```bash
# Basic execution
mooncake run --config config.yml

# Preview changes
mooncake run --config config.yml --dry-run

# Filter by tags
mooncake run --config config.yml --tags dev

# With sudo (interactive prompt - recommended)
mooncake run --config config.yml --ask-become-pass
# or
mooncake run --config config.yml -K

# With sudo (file-based)
echo "mypassword" > ~/.mooncake/sudo_pass
chmod 0600 ~/.mooncake/sudo_pass
mooncake run --config config.yml --sudo-pass-file ~/.mooncake/sudo_pass

# With sudo (insecure CLI - not recommended)
mooncake run --config config.yml --sudo-pass mypass --insecure-sudo-pass

# Execute from saved plan
mooncake plan --config config.yml --format json --output plan.json
mooncake run --from-plan plan.json
```

## mooncake facts

Display system facts that are available as template variables.

### Usage

```bash
mooncake facts [--format text|json]
```

### Flags

| Flag | Description |
|------|-------------|
| `--format, -f` | Output format: text or json (default: text) |

### What Facts Are Shown?

System information collected and available as template variables:

**System:**
- OS, distribution, kernel version, architecture, hostname

**Hardware:**
- CPU model, cores, flags (AVX, SSE, etc.)
- Memory total/free, swap
- GPUs (vendor, model, memory, driver, CUDA version)
- Disks (device, mount point, size, usage)

**Network:**
- Network interfaces (name, MAC, MTU, addresses)
- Default gateway
- DNS servers
- IP addresses

**Software:**
- Package manager (apt, brew, etc.)
- Python version
- Docker, Git, Go versions

### Examples

**Text Output (Human-Readable)**

```bash
mooncake facts
```

Example output:
```
╭─────────────────────────────────────────────────────────────╮
│                    System Information                       │
╰─────────────────────────────────────────────────────────────╯

OS:         ubuntu 22.04
Arch:       amd64
Hostname:   server01
Kernel:     6.5.0-14-generic

CPU:
  Cores:    8
  Model:    Intel(R) Core(TM) i7-10700K CPU @ 3.80GHz
  Flags:    avx avx2 sse4_2 fma aes

Memory:
  Total:    16384 MB (16.0 GB)
  Free:     8192 MB (8.0 GB)
  Swap:     4096 MB total, 2048 MB free

Software:
  Package Manager: apt
  Python:          3.11.5
  Docker:          24.0.7
  Git:             2.43.0
  Go:              1.21.5

GPUs:
  • NVIDIA GeForce RTX 4090, Memory: 24GB, Driver: 535.54.03, CUDA: 12.3

Storage:
  Device        Mount     Type      Size        Used       Avail
  ────────────────────────────────────────────────────────────
  /dev/sda1     /         ext4      500 GB      250 GB     250 GB
  /dev/sdb1     /data     ext4      1000 GB     500 GB     500 GB

Network:
  Gateway:  192.168.1.1
  DNS:      8.8.8.8, 1.1.1.1

Network Interfaces:
  • eth0  |  MAC: 00:11:22:33:44:55  |  192.168.1.100/24
```

**JSON Output (Machine-Readable)**

```bash
mooncake facts --format json
```

Example output:
```json
{
  "OS": "linux",
  "Arch": "amd64",
  "Hostname": "server01",
  "Username": "admin",
  "UserHome": "/home/admin",
  "Distribution": "ubuntu",
  "DistributionVersion": "22.04",
  "DistributionMajor": "22",
  "KernelVersion": "6.5.0-14-generic",
  "CPUCores": 8,
  "CPUModel": "Intel(R) Core(TM) i7-10700K CPU @ 3.80GHz",
  "CPUFlags": ["fpu", "vme", "avx", "avx2", "sse4_2", "fma"],
  "MemoryTotalMB": 16384,
  "MemoryFreeMB": 8192,
  "SwapTotalMB": 4096,
  "SwapFreeMB": 2048,
  "DefaultGateway": "192.168.1.1",
  "DNSServers": ["8.8.8.8", "1.1.1.1"],
  "IPAddresses": ["192.168.1.100"],
  "NetworkInterfaces": [
    {
      "Name": "eth0",
      "MACAddress": "00:11:22:33:44:55",
      "MTU": 1500,
      "Addresses": ["192.168.1.100/24"],
      "Up": true
    }
  ],
  "Disks": [
    {
      "Device": "/dev/sda1",
      "MountPoint": "/",
      "Filesystem": "ext4",
      "SizeGB": 500,
      "UsedGB": 250,
      "AvailGB": 250,
      "UsedPct": 50
    }
  ],
  "GPUs": [
    {
      "Vendor": "nvidia",
      "Model": "GeForce RTX 4090",
      "Memory": "24GB",
      "Driver": "535.54.03",
      "CUDAVersion": "12.3"
    }
  ],
  "PythonVersion": "3.11.5",
  "PackageManager": "apt",
  "DockerVersion": "24.0.7",
  "GitVersion": "2.43.0",
  "GoVersion": "1.21.5"
}
```

### Using Facts in Templates

All facts are available as variables in your configuration templates:

```yaml
steps:
  - name: Show system info
    shell: |
      echo "Running on {{ os }}/{{ arch }}"
      echo "CPU: {{ cpu_model }}"
      echo "Memory: {{ memory_total_mb }}MB"
      echo "Kernel: {{ kernel_version }}"

  - name: Iterate over disks
    shell: |
      {% for disk in disks %}
      echo "Disk: {{ disk.Device }} at {{ disk.MountPoint }} ({{ disk.SizeGB }}GB)"
      {% endfor %}

  - name: Check Docker availability
    shell: echo "Docker {{ docker_version }} is installed"
    when: docker_version != ""
```

See [Variables](config/variables.md) for complete list of available facts.


---

<!-- FILE: guide/config/actions.md -->

# Actions

Actions are the operations Mooncake performs. Each step in your configuration uses one action type.

## Quick Navigation

| Action | Purpose | Jump to |
|--------|---------|---------|
| **shell** | Execute commands | [↓](#shell) |
| **command** | Direct execution (no shell) | [↓](#command) |
| **file** | Create/manage files | [↓](#file) |
| **copy** | Copy files | [↓](#copy) |
| **download** | Download from URLs | [↓](#download) |
| **package** | Manage packages | [↓](#package) |
| **unarchive** | Extract archives | [↓](#unarchive) |
| **template** | Render templates | [↓](#template) |
| **service** | Manage services | [↓](#service) |
| **assert** | Verify state | [↓](#assert) |
| **preset** | Reusable workflows | [↓](#preset) |
| **include** | Load configs | [↓](#include) |
| **include_vars** | Load variables | [↓](#include-vars) |
| **vars** | Define variables | [↓](#vars) |

📋 **[Complete Properties Reference →](reference.md)** - All properties organized by type

---

## Shell

Execute shell commands with full shell interpolation and scripting capabilities.

### Basic Usage (Simple String)

```yaml
- name: Run command
  shell: echo "Hello"
```

### Structured Shell (Advanced)

```yaml
- name: Run with interpreter
  shell:
    cmd: echo "Hello"
    interpreter: bash
    stdin: "input data"
    capture: true
```

### Shell Properties

Shell commands support both simple string form and structured object form:

**Simple Form:**
```yaml
shell: "command here"
```

**Structured Form:**

| Property | Type | Description |
|----------|------|-------------|
| `shell.cmd` | string | Command to execute (required) |
| `shell.interpreter` | string | Shell interpreter: "bash", "sh", "pwsh", "cmd" (default: "bash" on Unix, "pwsh" on Windows) |
| `shell.stdin` | string | Input to pipe into command (supports templates) |
| `shell.capture` | boolean | Capture output (default: true). Set false for streaming-only mode |

**Step-Level Properties** (work with all actions):

| Property | Type | Description |
|----------|------|-------------|
| `env` | object | Environment variables |
| `cwd` | string | Working directory |
| `timeout` | string | Maximum execution time (e.g., '30s', '5m') |
| `retries` | integer | Number of retry attempts (0-100) |
| `retry_delay` | string | Delay between retries (e.g., '5s') |
| `changed_when` | string | Expression to override changed status |
| `failed_when` | string | Expression to override failure status |
| `become_user` | string | User for sudo (when become: true) |

Plus all [universal fields](#universal-fields): `name`, `when`, `become`, `tags`, `register`, `with_items`, `with_filetree`

### Multi-line Commands

```yaml
- name: Multiple commands
  shell: |
    echo "First"
    echo "Second"
    cd /tmp && ls -la
```

### With Variables

```yaml
- vars:
    package: neovim

- name: Install package
  shell: "{{package_manager}} install {{package}}"
```

### With Execution Control

```yaml
- name: Robust download
  shell: curl -O https://example.com/file.tar.gz
  timeout: 10m
  retries: 3
  retry_delay: 30s
  env:
    HTTP_PROXY: "{{proxy_url}}"
  cwd: /tmp/downloads
```

### Structured Shell with Interpreter

```yaml
- name: PowerShell on Windows
  shell:
    cmd: Get-Process | Where-Object {$_.CPU -gt 100}
    interpreter: pwsh

- name: POSIX shell for compatibility
  shell:
    cmd: printf '%s\n' "Hello"
    interpreter: sh
```

### Shell with stdin

```yaml
- name: Pipe data to command
  shell:
    cmd: python3 process_input.py
    stdin: |
      line1
      line2
      line3

- name: Use template in stdin
  shell:
    cmd: psql -U {{db_user}} {{db_name}}
    stdin: |
      SELECT * FROM users WHERE active = true;
```

### Shell Quoting Rules

**When to use shell vs command:**

- Use `shell` when you need:
  - Shell features: pipes (`|`), redirects (`>`, `<`), wildcards (`*`)
  - Command substitution: `$(command)` or `` `command` ``
  - Environment variable expansion: `$VAR`
  - Shell scripting: `if`, `for`, `while` loops

- Use `command` (see below) when:
  - You have a fixed command with known arguments
  - You don't need shell interpretation
  - You want to avoid quoting issues
  - You want better security (no shell injection)

**Quoting in shell:**

```yaml
# Good - quotes protect spaces
- shell: echo "hello world"

# Good - single quotes prevent variable expansion
- shell: echo 'The $PATH is set'

# Template variables - use quotes if they might contain spaces
- shell: echo "User: {{username}}"

# Multiple commands
- shell: |
    cd /tmp
    echo "Working in $(pwd)"
    ls -la
```

## Command

Execute commands directly without shell interpolation. This is safer and faster when you don't need shell features.

### Basic Usage

```yaml
- name: Clone repository
  command:
    argv: ["git", "clone", "https://github.com/user/repo.git"]
```

### Command Properties

| Property | Type | Description |
|----------|------|-------------|
| `command.argv` | array | Command and arguments as list (required) |
| `command.stdin` | string | Input to pipe into command (supports templates) |
| `command.capture` | boolean | Capture output (default: true). Set false for streaming-only mode |

**Step-Level Properties** (same as shell):

| Property | Type | Description |
|----------|------|-------------|
| `env` | object | Environment variables |
| `cwd` | string | Working directory |
| `timeout` | string | Maximum execution time |
| `retries` | integer | Number of retry attempts |
| `retry_delay` | string | Delay between retries |
| `changed_when` | string | Expression to override changed status |
| `failed_when` | string | Expression to override failure status |
| `become_user` | string | User for sudo (when become: true) |

Plus all [universal fields](#universal-fields): `name`, `when`, `become`, `tags`, `register`, `with_items`, `with_filetree`

### Command with Templates

```yaml
- vars:
    repo_url: "https://github.com/user/repo.git"
    target_dir: "/opt/repo"

- name: Clone with template variables
  command:
    argv:
      - git
      - clone
      - "{{repo_url}}"
      - "{{target_dir}}"
```

### Command with stdin

```yaml
- name: Feed data to process
  command:
    argv: ["python3", "-c", "import sys; print(sys.stdin.read().upper())"]
    stdin: "hello world"
```

### Command vs Shell Comparison

```yaml
# Shell - uses shell interpolation
- name: Shell with pipe
  shell: ls -la | grep myfile

# Command - direct execution (no shell)
- name: Command (no pipes/wildcards)
  command:
    argv: ["ls", "-la", "/tmp"]

# Shell - variable expansion
- name: Shell with $HOME
  shell: echo $HOME

# Command - literal arguments (no variable expansion)
- name: Command (explicit paths)
  command:
    argv: ["echo", "{{ansible_env.HOME}}"]
```

### Security: Shell vs Command

**Shell injection risk:**
```yaml
# UNSAFE if user_input contains "; rm -rf /"
- shell: echo "{{user_input}}"

# SAFE - no shell interpretation
- command:
    argv: ["echo", "{{user_input}}"]
```

**When to use each:**

- `shell`: Trust the input, need shell features
- `command`: Don't trust input, simple command execution

## File

Create and manage files and directories.

### File Properties

| Property | Type | Description |
|----------|------|-------------|
| `file.path` | string | File or directory path (required) |
| `file.state` | string | `file`, `directory`, `absent`, `touch`, `link`, `hardlink`, or `perms` |
| `file.content` | string | Content to write to file (for `state: file`) |
| `file.mode` | string | Permissions (e.g., "0644", "0755") |
| `file.owner` | string | File owner (username or UID) |
| `file.group` | string | File group (group name or GID) |
| `file.src` | string | Source path (required for `link` and `hardlink` states) |
| `file.force` | boolean | Force overwrite existing files or remove non-empty directories |
| `file.recurse` | boolean | Apply permissions recursively (with `state: perms`) |
| `file.backup` | boolean | Create `.bak` backup before overwriting |

Plus [universal fields](#universal-fields): `name`, `when`, `become`, `tags`, `register`, `with_items`, `with_filetree`

**Note:** File operations do NOT support shell-specific fields (timeout, retries, env, cwd, etc.)

### Create Directory

```yaml
- name: Create directory
  file:
    path: /tmp/myapp
    state: directory
    mode: "0755"
```

### Create File

```yaml
- name: Create empty file
  file:
    path: /tmp/config.txt
    state: file
    mode: "0644"
```

### Create File with Content

```yaml
- name: Create config
  file:
    path: /tmp/config.txt
    state: file
    mode: "0644"
    content: |
      key: value
      debug: true
```

### File Permissions

Common permission modes:

- `"0755"` - rwxr-xr-x (directories, executables)
- `"0644"` - rw-r--r-- (regular files)
- `"0600"` - rw------- (private files)
- `"0700"` - rwx------ (private directories)

### Remove File or Directory

```yaml
- name: Remove file
  file:
    path: /tmp/old-file.txt
    state: absent

- name: Remove directory (empty)
  file:
    path: /tmp/old-dir
    state: absent

- name: Remove directory (recursive)
  file:
    path: /tmp/old-dir
    state: absent
    force: true
```

### Touch File (Update Timestamp)

```yaml
- name: Create empty marker file
  file:
    path: /tmp/.marker
    state: touch
    mode: "0644"
```

### Create Symbolic Link

```yaml
- name: Create symlink
  file:
    path: /usr/local/bin/myapp
    src: /opt/myapp/bin/myapp
    state: link

- name: Force replace existing file with symlink
  file:
    path: /etc/config.yml
    src: /opt/configs/prod.yml
    state: link
    force: true
```

### Create Hard Link

```yaml
- name: Create hard link
  file:
    path: /backup/important.txt
    src: /data/important.txt
    state: hardlink
```

### Change Permissions Only

```yaml
- name: Fix permissions on existing file
  file:
    path: /opt/app/data
    state: perms
    mode: "0755"
    owner: app
    group: app

- name: Recursively fix directory permissions
  file:
    path: /var/www/html
    state: perms
    mode: "0644"
    owner: www-data
    group: www-data
    recurse: true
  become: true
```

### Set Ownership

```yaml
- name: Change file owner
  file:
    path: /opt/app/config.yml
    state: file
    owner: app
    group: app
    mode: "0600"
  become: true
```

## Copy

Copy files with checksum verification and backup support.

### Copy Properties

| Property | Type | Description |
|----------|------|-------------|
| `copy.src` | string | Source file path (required) |
| `copy.dest` | string | Destination file path (required) |
| `copy.mode` | string | Permissions (e.g., "0644", "0755") |
| `copy.owner` | string | File owner (username or UID) |
| `copy.group` | string | File group (group name or GID) |
| `copy.backup` | boolean | Create `.bak` backup before overwriting |
| `copy.force` | boolean | Force overwrite if destination exists |
| `copy.checksum` | string | Expected SHA256 or MD5 checksum |

Plus [universal fields](#universal-fields): `name`, `when`, `become`, `tags`, `register`, `with_items`, `with_filetree`

### Basic Copy

```yaml
- name: Copy configuration file
  copy:
    src: ./configs/app.yml
    dest: /opt/app/config.yml
    mode: "0644"
```

### Copy with Backup

```yaml
- name: Update config with backup
  copy:
    src: ./configs/prod.yml
    dest: /etc/app/config.yml
    mode: "0600"
    owner: app
    group: app
    backup: true
  become: true
```

### Copy with Checksum Verification

```yaml
- name: Copy binary with integrity check
  copy:
    src: ./downloads/app-v1.2.3
    dest: /usr/local/bin/app
    mode: "0755"
    checksum: "sha256:a3b5c6d7e8f9..."
```

## Unarchive

Extract archive files with automatic format detection and security protections.

### Unarchive Properties

| Property | Type | Description |
|----------|------|-------------|
| `unarchive.src` | string | Path to archive file (required) |
| `unarchive.dest` | string | Destination directory (required) |
| `unarchive.strip_components` | integer | Number of leading path components to strip (default: 0) |
| `unarchive.creates` | string | Skip extraction if this path exists (idempotency marker) |
| `unarchive.mode` | string | Directory permissions (e.g., "0755") |

Plus [universal fields](#universal-fields): `name`, `when`, `become`, `tags`, `register`, `with_items`, `with_filetree`

**Supported formats:** `.tar`, `.tar.gz`, `.tgz`, `.zip` (auto-detected from extension)

**Security:** Automatically blocks path traversal attacks (`../` sequences) and validates all extracted paths.

### Basic Extraction

```yaml
- name: Extract Node.js
  unarchive:
    src: /tmp/node-v20.tar.gz
    dest: /opt/node
    mode: "0755"
```

### Extract with Path Stripping

Strip leading path components (like tar's `--strip-components`):

```yaml
# Archive contains: node-v20/bin/node, node-v20/lib/...
# Result: /opt/node/bin/node, /opt/node/lib/...
- name: Extract Node.js without top-level directory
  unarchive:
    src: /tmp/node-v20.tar.gz
    dest: /opt/node
    strip_components: 1
```

### Extract with Idempotency

Skip extraction if marker file already exists:

```yaml
- name: Extract application
  unarchive:
    src: /tmp/myapp.tar.gz
    dest: /opt/myapp
    creates: /opt/myapp/.installed
    mode: "0755"

# Run again - will skip because marker exists
```

### Extract Multiple Archives

```yaml
- vars:
    archives:
      - name: app
        file: app-v1.2.3.tar.gz
      - name: data
        file: data.zip

- name: Extract {{item.name}}
  unarchive:
    src: /tmp/{{item.file}}
    dest: /opt/{{item.name}}
    strip_components: 1
  with_items: "{{archives}}"
```

### Extract with Become

```yaml
- name: Extract to system directory
  unarchive:
    src: /tmp/archive.tar.gz
    dest: /opt/myapp
    mode: "0755"
  become: true
```

### Supported Archive Formats

- **tar** - Uncompressed tar archives (`.tar`)
- **tar.gz** - Gzip compressed tar archives (`.tar.gz`)
- **tgz** - Alternative gzip tar extension (`.tgz`)
- **zip** - ZIP archives (`.zip`)

Format is detected automatically from the file extension (case-insensitive).

### How strip_components Works

```yaml
# Archive structure:
#   project-1.0/src/main.go
#   project-1.0/src/utils.go
#   project-1.0/README.md

# strip_components: 0 (default)
# Result: dest/project-1.0/src/main.go

# strip_components: 1
# Result: dest/src/main.go

# strip_components: 2
# Result: dest/main.go
```

Files with fewer path components than `strip_components` are skipped.

### Security Features

All extracted paths are validated to prevent:

- **Path traversal attacks** - Blocks `../` sequences
- **Absolute paths** - Prevents extracting to system paths
- **Symlink escapes** - Validates symlink targets stay within destination

These protections are always active and cannot be disabled.

## Download

Download files from remote URLs with checksum verification and retry support.

### Download Properties

| Property | Type | Description |
|----------|------|-------------|
| `download.url` | string | Remote URL to download from (required) |
| `download.dest` | string | Destination file path (required) |
| `download.checksum` | string | Expected SHA256 (64 chars) or MD5 (32 chars) checksum |
| `download.mode` | string | File permissions (e.g., "0644", "0755") |
| `download.timeout` | string | Maximum download time (e.g., "30s", "5m") |
| `download.retries` | integer | Number of retry attempts on failure (0-100) |
| `download.force` | boolean | Force re-download even if destination exists |
| `download.backup` | boolean | Create `.bak` backup before overwriting |
| `download.headers` | object | Custom HTTP headers (Authorization, User-Agent, etc.) |

Plus [universal fields](#universal-fields): `name`, `when`, `become`, `tags`, `register`, `with_items`, `with_filetree`

**Idempotency:** Downloads are skipped when:
- Destination file exists with matching checksum (when `checksum` is provided)
- Destination file exists and `force: false` (without checksum - not recommended)

**Best practice:** Always use `checksum` for reliable idempotency and security.

### Basic Download

```yaml
- name: Download file
  download:
    url: "https://example.com/file.tar.gz"
    dest: "/tmp/file.tar.gz"
    mode: "0644"
```

### Download with Checksum (Idempotent)

```yaml
- name: Download Go tarball
  download:
    url: "https://go.dev/dl/go1.21.5.linux-amd64.tar.gz"
    dest: "/tmp/go.tar.gz"
    checksum: "e2bc0b3e4b64111ec117295c088bde5f00eeed1567999ff77bc859d7df70078e"
    mode: "0644"
  register: go_download

# Second run will skip download (idempotent)
```

### Download with Retry and Timeout

```yaml
- name: Download large file
  download:
    url: "https://releases.ubuntu.com/22.04/ubuntu.iso"
    dest: "/tmp/ubuntu.iso"
    timeout: "10m"
    retries: 3
    mode: "0644"
```

### Authenticated Download

```yaml
- name: Download from private API
  download:
    url: "https://api.example.com/files/document.pdf"
    dest: "/tmp/document.pdf"
    headers:
      Authorization: "Bearer {{ api_token }}"
      User-Agent: "Mooncake/1.0"
    mode: "0644"
```

### Download with Backup

```yaml
- name: Update config file safely
  download:
    url: "https://example.com/config/app.conf"
    dest: "/etc/myapp/app.conf"
    backup: true
    force: true
    mode: "0644"
  become: true
```

### Download and Extract

```yaml
- name: Download Node.js
  download:
    url: "https://nodejs.org/dist/v18.19.0/node-v18.19.0-linux-x64.tar.gz"
    dest: "/tmp/node.tar.gz"
    checksum: "f27e33ebe5a0c2ec8d5d6b5f5c7c2c0c1c3f7b1a2a3d4e5f6g7h8i9j0k1l2m3n"
    mode: "0644"
  register: node_download

- name: Extract if downloaded
  unarchive:
    src: "/tmp/node.tar.gz"
    dest: "/opt/node"
    strip_components: 1
  when: node_download.changed
```

### How Checksum Works

The `checksum` field supports both SHA256 and MD5:

```yaml
# SHA256 (64 hexadecimal characters) - recommended
checksum: "e2bc0b3e4b64111ec117295c088bde5f00eeed1567999ff77bc859d7df70078e"

# MD5 (32 hexadecimal characters) - legacy support
checksum: "5d41402abc4b2a76b9719d911017c592"
```

**How it works:**
1. If destination exists, calculate its checksum
2. If checksums match → skip download (idempotent)
3. If checksums differ → download new version
4. After download, verify checksum matches expected value

### Security Features

All downloads include these security features:

- **Atomic writes** - Downloads to temp file, verifies, then renames (prevents partial downloads)
- **Checksum verification** - Prevents man-in-the-middle attacks (when checksum provided)
- **HTTPS support** - Secure downloads over TLS
- **Timeout protection** - Prevents hanging on slow connections

### Performance Tips

```yaml
# Good - Fast idempotency check (4ms vs 40ms)
- download:
    url: "https://example.com/large-file.iso"
    dest: "/tmp/file.iso"
    checksum: "abc123..."  # Enables fast skip on second run

# Avoid - Always re-downloads without checksum verification
- download:
    url: "https://example.com/file.iso"
    dest: "/tmp/file.iso"
    force: true  # No idempotency
```

## Package

Manage system packages (install, remove, update) with automatic package manager detection.

### Package Properties

| Property | Type | Description |
|----------|------|-------------|
| `package.name` | string | Single package name to manage |
| `package.names` | array | Multiple package names to manage |
| `package.state` | string | Desired state: `present` (default), `absent`, `latest` |
| `package.manager` | string | Package manager to use (auto-detected if not specified) |
| `package.update_cache` | boolean | Update package cache before operation |
| `package.upgrade` | boolean | Upgrade all installed packages (ignores name/names) |
| `package.extra` | array | Extra arguments to pass to package manager |

Plus [universal fields](#universal-fields): `name`, `when`, `become`, `tags`, `register`, `with_items`, `with_filetree`

**Supported Package Managers:**
- **Linux:** apt, dnf, yum, pacman, zypper, apk
- **macOS:** brew, port
- **Windows:** choco, scoop

**Auto-detection:** Uses `package_manager` system fact or detects based on OS if not specified.

**Idempotency:** Checks if package is already installed before attempting installation.

### Install Single Package

```yaml
- name: Install neovim
  package:
    name: neovim
    state: present
  become: true
```

### Install Multiple Packages

```yaml
- name: Install development tools
  package:
    names:
      - git
      - curl
      - vim
      - wget
    state: present
  become: true
```

### Auto-Detect Package Manager

```yaml
- name: Install Python (cross-platform)
  package:
    name: python3
    update_cache: true
  become: true
  # Uses apt on Debian/Ubuntu, dnf on Fedora, brew on macOS, etc.
```

### Specify Package Manager

```yaml
- name: Install Node.js with Homebrew
  package:
    name: node
    manager: brew
  # Explicitly use Homebrew even if other managers are available
```

### Remove Package

```yaml
- name: Remove Apache
  package:
    name: apache2
    state: absent
  become: true
```

### Install Latest Version

```yaml
- name: Ensure latest Git
  package:
    name: git
    state: latest
    update_cache: true
  become: true
```

### Upgrade All Packages

```yaml
- name: System-wide package upgrade
  package:
    upgrade: true
  become: true
  # Runs: apt-get upgrade, dnf upgrade, brew upgrade, etc.
```

### With Extra Arguments

```yaml
- name: Install with specific options
  package:
    name: nginx
    state: present
    extra:
      - "--no-install-recommends"
  become: true
  # For apt: apt-get install -y --no-install-recommends nginx
```

### Loop Over Packages

```yaml
- vars:
    dev_packages:
      - gcc
      - make
      - autoconf
      - pkg-config

- name: Install build tools
  package:
    name: "{{ item }}"
    state: present
  with_items: "{{ dev_packages }}"
  become: true
```

### Conditional Package Management

```yaml
- name: Install package manager specific tools
  package:
    name: "{{ item.package }}"
    manager: "{{ item.manager }}"
    state: present
  with_items:
    - { manager: "apt", package: "apt-transport-https" }
    - { manager: "dnf", package: "dnf-plugins-core" }
  when: package_manager == item.manager
  become: true
```

### Update Cache Before Install

```yaml
- name: Install with fresh cache
  package:
    name: docker-ce
    state: present
    update_cache: true
  become: true
  # Runs apt-get update or equivalent before installation
```

## Service

Manage system services (systemd on Linux, launchd on macOS).

### Service Properties

| Property | Type | Description |
|----------|------|-------------|
| `service.name` | string | Service name (required) |
| `service.state` | string | Desired state: `started`, `stopped`, `restarted`, `reloaded` |
| `service.enabled` | boolean | Enable service on boot (systemd: enable/disable, launchd: bootstrap/bootout) |
| `service.daemon_reload` | boolean | Run `systemctl daemon-reload` after unit file changes (systemd only) |
| `service.unit` | object | Unit/plist file configuration (see below) |
| `service.dropin` | object | Drop-in configuration (systemd only, see below) |

**Unit File Properties:**

| Property | Type | Description |
|----------|------|-------------|
| `unit.dest` | string | Destination path (default: `/etc/systemd/system/<name>.service` or `~/Library/LaunchAgents/<name>.plist`) |
| `unit.content` | string | Inline unit/plist file content (supports templates) |
| `unit.src_template` | string | Path to unit/plist template file |
| `unit.mode` | string | File permissions (e.g., "0644") |

**Drop-in Properties (systemd only):**

| Property | Type | Description |
|----------|------|-------------|
| `dropin.name` | string | Drop-in file name (e.g., "10-override.conf") - required |
| `dropin.content` | string | Inline drop-in content (supports templates) |
| `dropin.src_template` | string | Path to drop-in template file |

Plus [universal fields](#universal-fields): `name`, `when`, `become`, `tags`, `register`, `with_items`, `with_filetree`

### Linux (systemd) Examples

#### Start and Enable Service

```yaml
- name: Start nginx
  service:
    name: nginx
    state: started
    enabled: true
  become: true
```

#### Create Service from Template

```yaml
- name: Deploy custom service
  service:
    name: myapp
    unit:
      src_template: templates/myapp.service.j2
      dest: /etc/systemd/system/myapp.service
    daemon_reload: true
    state: started
    enabled: true
  become: true
```

#### Create Service with Inline Content

```yaml
- name: Create simple service
  service:
    name: myapp
    unit:
      content: |
        [Unit]
        Description=My Application
        After=network.target

        [Service]
        Type=simple
        ExecStart=/usr/local/bin/myapp
        Restart=on-failure

        [Install]
        WantedBy=multi-user.target
    daemon_reload: true
    state: started
    enabled: true
  become: true
```

#### Add Drop-in Configuration

```yaml
- name: Override service environment
  service:
    name: myapp
    dropin:
      name: "10-env.conf"
      content: |
        [Service]
        Environment="API_KEY={{ api_key }}"
        Environment="DEBUG=true"
    daemon_reload: true
    state: restarted
  become: true
```

#### Stop and Disable Service

```yaml
- name: Remove old service
  service:
    name: old-service
    state: stopped
    enabled: false
  become: true
```

### macOS (launchd) Examples

#### Create User Agent

```yaml
- name: Start user agent
  service:
    name: com.example.myapp
    state: started
    enabled: true
    unit:
      content: |
        <?xml version="1.0" encoding="UTF-8"?>
        <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
        <plist version="1.0">
        <dict>
          <key>Label</key>
          <string>com.example.myapp</string>
          <key>ProgramArguments</key>
          <array>
            <string>/usr/local/bin/myapp</string>
          </array>
          <key>RunAtLoad</key>
          <true/>
        </dict>
        </plist>
```

#### Create System Daemon (requires sudo)

```yaml
- name: Create system daemon
  service:
    name: com.example.daemon
    state: started
    enabled: true
    unit:
      dest: /Library/LaunchDaemons/com.example.daemon.plist
      src_template: templates/daemon.plist.j2
  become: true
```

#### Create Scheduled Task

```yaml
- name: Create backup task
  service:
    name: com.example.backup
    enabled: true
    unit:
      content: |
        <?xml version="1.0" encoding="UTF-8"?>
        <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
        <plist version="1.0">
        <dict>
          <key>Label</key>
          <string>com.example.backup</string>
          <key>ProgramArguments</key>
          <array>
            <string>/usr/local/bin/backup.sh</string>
          </array>
          <key>StartCalendarInterval</key>
          <dict>
            <key>Hour</key>
            <integer>2</integer>
            <key>Minute</key>
            <integer>30</integer>
          </dict>
        </dict>
        </plist>
```

### Service States

| State | Linux (systemd) | macOS (launchd) |
|-------|----------------|-----------------|
| `started` | `systemctl start` | `launchctl bootstrap` / `kickstart` |
| `stopped` | `systemctl stop` | `launchctl kill` |
| `restarted` | `systemctl restart` | `launchctl kickstart -k` |
| `reloaded` | `systemctl reload` | Same as restart |

### Idempotency

Service operations are idempotent:

- **Unit/plist files:** Only updated if content changed
- **Service state:** Checked before changing
- **Enable status:** Only changed if different

```yaml
# First run: Creates unit, reloads daemon, starts service, enables on boot
# Second run: No changes (unit unchanged, service already running and enabled)
- name: Deploy service
  service:
    name: myapp
    state: started
    enabled: true
    unit:
      content: |
        [Unit]
        Description=My App
        [Service]
        ExecStart=/usr/local/bin/myapp
        [Install]
        WantedBy=multi-user.target
  become: true
```

### Platform Detection

Mooncake automatically detects the platform and uses the appropriate service manager:

- **Linux:** Uses systemd (`systemctl`)
- **macOS:** Uses launchd (`launchctl`)
- **Windows:** Not yet supported

### Complete Examples

See detailed examples with real-world use cases:

- **macOS Services:** `examples/macos-services/` - Complete launchd examples with Node.js apps, scheduled tasks, and service management patterns
- **Service Management README:** `examples/macos-services/README.md` - Comprehensive guide to macOS service management

## Assert

Verify system state, command results, file properties, or HTTP responses. Assertions **never report `changed: true`** and **fail fast** if verification doesn't pass.

**Use cases:**
- Verify prerequisites before deployment
- Check system configuration meets requirements
- Validate API responses
- Test infrastructure state
- Ensure files have correct permissions

### Assert Properties

Assertions require exactly **one** of these types:

**Command Assertion:**

| Property | Type | Description |
|----------|------|-------------|
| `assert.command.cmd` | string | Command to execute (required) |
| `assert.command.exit_code` | integer | Expected exit code (default: 0) |

**File Assertion:**

| Property | Type | Description |
|----------|------|-------------|
| `assert.file.path` | string | File path to check (required) |
| `assert.file.exists` | boolean | Verify file exists (true) or doesn't exist (false) |
| `assert.file.content` | string | Expected exact file content (supports templates) |
| `assert.file.contains` | string | Expected substring in file (supports templates) |
| `assert.file.mode` | string | Expected file permissions (e.g., "0644") |
| `assert.file.owner` | string | Expected file owner (UID or username) |
| `assert.file.group` | string | Expected file group (GID or groupname) |

**HTTP Assertion:**

| Property | Type | Description |
|----------|------|-------------|
| `assert.http.url` | string | URL to request (required) |
| `assert.http.method` | string | HTTP method: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS (default: GET) |
| `assert.http.status` | integer | Expected status code (default: 200) |
| `assert.http.headers` | object | Request headers (supports templates) |
| `assert.http.body` | string | Request body (supports templates) |
| `assert.http.contains` | string | Expected substring in response body |
| `assert.http.body_equals` | string | Expected exact response body |
| `assert.http.timeout` | string | Request timeout (e.g., "30s", "5m") |

Plus [universal fields](#universal-fields): `name`, `when`, `tags`, `register`, `with_items`, `with_filetree`

### Command Assertions

#### Verify Command Succeeds

```yaml
- name: Check Docker is installed
  assert:
    command:
      cmd: docker --version
      exit_code: 0
```

#### Expect Specific Exit Code

```yaml
- name: Verify configuration is invalid
  assert:
    command:
      cmd: validate-config broken.yml
      exit_code: 1
```

#### Check Command with Template

```yaml
- name: Verify package installed
  assert:
    command:
      cmd: "dpkg -l | grep {{ package_name }}"
      exit_code: 0
```

### File Assertions

#### Check File Exists

```yaml
- name: Verify config file exists
  assert:
    file:
      path: /etc/nginx/nginx.conf
      exists: true
```

#### Check File Does Not Exist

```yaml
- name: Ensure temp file removed
  assert:
    file:
      path: /tmp/install.lock
      exists: false
```

#### Verify File Content

```yaml
- name: Check hostname configuration
  assert:
    file:
      path: /etc/hostname
      content: "{{ expected_hostname }}"
```

#### Check File Contains String

```yaml
- name: Verify SSH config has setting
  assert:
    file:
      path: ~/.ssh/config
      contains: "ForwardAgent yes"
```

#### Verify File Permissions

```yaml
- name: Check private key permissions
  assert:
    file:
      path: ~/.ssh/id_rsa
      mode: "0600"
```

#### Check File Ownership

```yaml
- name: Verify log directory owner
  assert:
    file:
      path: /var/log/myapp
      owner: "1000"
      group: "1000"
```

### HTTP Assertions

#### Check HTTP Status

```yaml
- name: Verify service is up
  assert:
    http:
      url: https://api.example.com/health
      status: 200
```

#### Check Response Body Contains

```yaml
- name: Verify API returns success
  assert:
    http:
      url: https://api.example.com/status
      status: 200
      contains: '"status":"healthy"'
```

#### POST Request with Body

```yaml
- name: Verify API accepts login
  assert:
    http:
      url: https://api.example.com/auth
      method: POST
      status: 200
      headers:
        Content-Type: application/json
      body: |
        {"username": "test", "password": "{{ api_password }}"}
      contains: "token"
```

#### Check Exact Response

```yaml
- name: Verify API version
  assert:
    http:
      url: https://api.example.com/version
      status: 200
      body_equals: '{"version":"2.0.0"}'
```

#### With Custom Timeout

```yaml
- name: Check slow endpoint
  assert:
    http:
      url: https://api.example.com/slow-operation
      status: 200
      timeout: 2m
```

### Practical Examples

#### Verify Prerequisites

```yaml
- name: Check system requirements
  block:
    - name: Verify Docker installed
      assert:
        command:
          cmd: docker --version

    - name: Verify Docker Compose installed
      assert:
        command:
          cmd: docker-compose --version

    - name: Verify port 80 available
      assert:
        command:
          cmd: "! nc -z localhost 80"
          exit_code: 0

    - name: Check SSL certificate exists
      assert:
        file:
          path: /etc/ssl/certs/server.crt
          exists: true
```

#### Validate Deployment

```yaml
- name: Verify deployment succeeded
  block:
    - name: Check application binary exists
      assert:
        file:
          path: /usr/local/bin/myapp
          exists: true
          mode: "0755"

    - name: Verify config has correct settings
      assert:
        file:
          path: /etc/myapp/config.yml
          contains: "production: true"

    - name: Check service is running
      assert:
        command:
          cmd: systemctl is-active myapp
          exit_code: 0

    - name: Verify health endpoint responds
      assert:
        http:
          url: http://localhost:8080/health
          status: 200
          contains: "healthy"
```

#### Test Infrastructure

```yaml
- name: Run infrastructure tests
  block:
    - name: Check database is accessible
      assert:
        command:
          cmd: "psql -U {{ db_user }} -h {{ db_host }} -c 'SELECT 1'"
          exit_code: 0

    - name: Verify Redis is responding
      assert:
        command:
          cmd: redis-cli ping
          exit_code: 0

    - name: Check API returns expected data
      assert:
        http:
          url: "{{ api_base_url }}/test"
          status: 200
          contains: "test_passed"
```

#### With Registered Results

```yaml
- name: Check API and capture result
  assert:
    http:
      url: https://api.example.com/status
      status: 200
  register: api_check

- name: Log assertion result
  shell: echo "API check passed - changed={{ api_check.changed }}"
  # Output: API check passed - changed=false
```

### Key Behaviors

**Never Changed:**
```yaml
# Assertions always report changed: false
- assert:
    command:
      cmd: echo "test"
  register: result
# result.changed is always false
```

**Fail Fast:**
```yaml
# Execution stops immediately on assertion failure
- assert:
    file:
      path: /missing/file
      exists: true
# This fails - subsequent steps won't run

- name: This won't execute
  shell: echo "skipped"
```

**Detailed Error Messages:**
```
assertion failed (command): expected exit code 0, got exit code 1 (false)
assertion failed (file): expected file exists: true, got file exists: false (/tmp/missing)
assertion failed (http): expected HTTP 200, got HTTP 404 (https://example.com)
```

## Ollama

Manage [Ollama](https://ollama.com) installation, service configuration, and model management. Ollama is a tool for running large language models locally.

**Platforms:** Linux (systemd), macOS (launchd/Homebrew)

**Use cases:**
- Install Ollama via package manager or official script
- Configure Ollama as a system service
- Pull and manage LLM models
- Verify Ollama API health
- Uninstall Ollama and optionally remove models

### Ollama Properties

| Property | Type | Description |
|----------|------|-------------|
| `ollama.state` | string | Installation state: `present` (install), `absent` (uninstall) - required |
| `ollama.service` | boolean | Enable and start Ollama service (systemd/launchd) |
| `ollama.method` | string | Installation method: `auto` (prefer package manager, fallback to script), `script` (official installer only), `package` (package manager only) - default: `auto` |
| `ollama.host` | string | Server bind address (e.g., `localhost:11434`, `0.0.0.0:11434`) - sets `OLLAMA_HOST` |
| `ollama.models_dir` | string | Custom models directory path - sets `OLLAMA_MODELS` |
| `ollama.pull` | array | List of models to pull (e.g., `["llama3.1:8b", "mistral"]`) |
| `ollama.force` | boolean | Force operations: re-pull existing models, remove models directory on uninstall |
| `ollama.env` | object | Additional environment variables for Ollama service (e.g., `OLLAMA_DEBUG`, `OLLAMA_ORIGINS`) |

Plus [universal fields](#universal-fields): `name`, `when`, `become`, `tags`, `register`, `with_items`, `with_filetree`

**Note:** Most operations require `become: true` for system-wide installation.

### Basic Installation

#### Install Ollama (binary only)

```yaml
- name: Install Ollama
  ollama:
    state: present
  become: true
```

#### Install with Service

```yaml
- name: Install Ollama with service
  ollama:
    state: present
    service: true
  become: true
```

#### Install via Specific Method

```yaml
# Prefer package manager (apt, brew, etc.)
- name: Install via package manager
  ollama:
    state: present
    method: package
  become: true

# Use official installer script
- name: Install via script
  ollama:
    state: present
    method: script
  become: true
```

### Model Management

#### Pull Models

```yaml
- name: Install Ollama and pull models
  ollama:
    state: present
    service: true
    pull:
      - "llama3.1:8b"
      - "mistral:latest"
      - "codellama:7b"
  become: true
```

#### Force Re-pull Models

```yaml
- name: Update models
  ollama:
    state: present
    pull:
      - "llama3.1:8b"
    force: true  # Re-pull even if exists
  become: true
```

### Service Configuration

#### Custom Bind Address

```yaml
- name: Ollama accessible from network
  ollama:
    state: present
    service: true
    host: "0.0.0.0:11434"
  become: true
```

#### Custom Models Directory

```yaml
- name: Store models on separate disk
  ollama:
    state: present
    service: true
    models_dir: "/data/ollama/models"
  become: true
```

#### With Environment Variables

```yaml
- name: Configure Ollama service
  ollama:
    state: present
    service: true
    host: "0.0.0.0:11434"
    env:
      OLLAMA_DEBUG: "1"
      OLLAMA_ORIGINS: "*"
      OLLAMA_MAX_LOADED_MODELS: "2"
  become: true
```

### Complete Installation

```yaml
- name: Full Ollama deployment
  ollama:
    state: present
    service: true
    method: auto
    host: "0.0.0.0:11434"
    models_dir: "/data/ollama"
    pull:
      - "llama3.1:8b"
      - "mistral"
    env:
      OLLAMA_DEBUG: "1"
  become: true
  register: ollama_result

- name: Verify Ollama API
  assert:
    http:
      url: "http://localhost:11434/api/tags"
      status: 200
      timeout: "10s"
```

### Uninstallation

#### Remove Ollama (keep models)

```yaml
- name: Uninstall Ollama binary and service
  ollama:
    state: absent
  become: true
```

#### Complete Removal (including models)

```yaml
- name: Complete Ollama removal
  ollama:
    state: absent
    force: true  # Also remove models directory
  become: true
```

### Platform-Specific Behavior

**Linux (systemd):**
- Installation methods: apt/dnf/yum/pacman/zypper/apk (package), script (official installer)
- Service: systemd unit at `/etc/systemd/system/ollama.service`
- Configuration: Drop-in at `/etc/systemd/system/ollama.service.d/10-mooncake.conf`
- Models: `~/.ollama/models` or custom `models_dir`

**macOS (launchd):**
- Installation methods: Homebrew (package), official script
- Service: plist at `~/Library/LaunchAgents/` (user) or `/Library/LaunchDaemons/` (system, requires `become`)
- Models: `~/.ollama/models` or custom `models_dir`

### Conditional Installation

```yaml
# Only install if not already present
- name: Check if Ollama installed
  shell: which ollama
  register: ollama_check
  failed_when: false

- name: Install Ollama
  ollama:
    state: present
    service: true
    pull: ["llama3.1:8b"]
  become: true
  when: "{{ ollama_check.rc != 0 }}"
```

### Using System Facts

```yaml
# Facts are automatically collected: ollama_version, ollama_models, ollama_endpoint
- name: Show Ollama information
  shell: |
    echo "Ollama version: {{ ollama_version }}"
    echo "Endpoint: {{ ollama_endpoint }}"
    {{ range .ollama_models }}
    echo "Model: {{ .Name }} ({{ .Size }})"
    {{ end }}
  when: "{{ ollama_version != '' }}"
```

### Integration Examples

#### Deploy Application with Ollama

```yaml
- name: Install Ollama
  ollama:
    state: present
    service: true
    pull: ["llama3.1:8b"]
  become: true

- name: Wait for Ollama to start
  assert:
    http:
      url: "http://localhost:11434/api/tags"
      status: 200
      timeout: "30s"
  retries: 5
  retry_delay: "5s"

- name: Run inference test
  shell: ollama run llama3.1:8b 'Say hello'
  register: test_result

- name: Display test result
  shell: echo "{{ test_result.stdout }}"
```

#### Development Environment Setup

```yaml
- name: Setup local LLM development environment
  ollama:
    state: present
    service: true
    host: "localhost:11434"
    pull:
      - "llama3.1:8b"      # General purpose
      - "codellama:7b"     # Code generation
      - "mistral:latest"   # Alternative model
    env:
      OLLAMA_DEBUG: "0"
      OLLAMA_MAX_LOADED_MODELS: "1"
  become: true
  register: ollama_setup

- name: Create API wrapper script
  file:
    path: ~/bin/ask-llm
    state: file
    mode: "0755"
    content: |
      #!/bin/bash
      ollama run llama3.1:8b "$@"
```

### Idempotency and Changed Detection

The `ollama` action is idempotent:
- Installation: Reports `changed: false` if Ollama already installed
- Service: Reports `changed: false` if service already running with correct configuration
- Models: Reports `changed: false` if models already pulled (unless `force: true`)
- Uninstall: Reports `changed: false` if Ollama not installed

### Result Registration

When using `register`, the result includes:

```yaml
ollama_result:
  changed: true/false
  failed: false
  operations: ["installed", "service_configured", "models_pulled"]
  # Other standard result fields
```

### Error Handling

```yaml
- name: Install Ollama with error handling
  ollama:
    state: present
    service: true
    pull: ["llama3.1:8b"]
  become: true
  register: ollama_result
  failed_when: false

- name: Handle installation failure
  shell: echo "Installation failed: {{ ollama_result.stderr }}"
  when: "{{ ollama_result.failed }}"
```

## Template

Render templates with variables and logic.

### Template Properties

| Property | Type | Description |
|----------|------|-------------|
| `template.src` | string | Source template file path (required) |
| `template.dest` | string | Destination file path (required) |
| `template.vars` | object | Additional variables for rendering |
| `template.mode` | string | Permissions (e.g., "0644") |

Plus [universal fields](#universal-fields): `name`, `when`, `become`, `tags`, `register`, `with_items`, `with_filetree`

**Note:** Template operations do NOT support shell-specific fields (timeout, retries, env, cwd, etc.)

### Basic Usage

```yaml
- name: Render config
  template:
    src: ./templates/config.yml.j2
    dest: /tmp/config.yml
    mode: "0644"
```

### With Additional Variables

```yaml
- template:
    src: ./templates/nginx.conf.j2
    dest: /etc/nginx/nginx.conf
    mode: "0644"
    vars:
      port: 8080
      ssl_enabled: true
```

### Template Syntax (pongo2)

**Variables:**
```jinja
server_name: {{ hostname }}
port: {{ port }}
```

**Conditionals:**
```jinja
{% if ssl_enabled %}
ssl on;
ssl_certificate {{ ssl_cert }};
{% endif %}
```

**Loops:**
```jinja
{% for server in servers %}
upstream {{ server.name }} {
    server {{ server.host }}:{{ server.port }};
}
{% endfor %}
```

**Filters:**
```jinja
home: {{ "~/.config" | expanduser }}
name: {{ app_name | upper }}
```

## Include

Load and execute tasks from other files.

### Include Properties

| Property | Type | Description |
|----------|------|-------------|
| `include` | string | Path to YAML file with steps (required) |

Plus [universal fields](#universal-fields): `name`, `when`, `tags`, `with_items`

**Note:** Include does NOT support shell-specific fields or `register`.

### Basic Usage

```yaml
- name: Run common tasks
  include: ./tasks/common.yml
```

### Conditional Include

```yaml
- name: Run Linux tasks
  include: ./tasks/linux.yml
  when: os == "linux"
```

## Include Vars

Load variables from external files.

### Include Vars Properties

| Property | Type | Description |
|----------|------|-------------|
| `include_vars` | string | Path to YAML file with variables (required) |

Plus [universal fields](#universal-fields): `name`, `when`, `tags`

**Note:** Include vars does NOT support shell-specific fields, `register`, or loops.

### Basic Usage

```yaml
- name: Load environment variables
  include_vars: ./vars/development.yml
```

### Dynamic Include

```yaml
- vars:
    env: production

- name: Load env-specific vars
  include_vars: ./vars/{{env}}.yml
```

## Universal Fields

These fields work with all action types (shell, file, template, include, etc.):

### name

Human-readable description displayed in output:
```yaml
- name: Install dependencies
  shell: npm install
```

### when

Conditional execution - step runs only if expression evaluates to `true`:
```yaml
- shell: brew install git
  when: os == "darwin"
```

### tags

Filter execution - run only steps with specified tags:
```yaml
- shell: npm test
  tags: [test, dev]
```

Run with: `mooncake run --config config.yml --tags test`

### become

Run with elevated privileges (sudo). Works with shell, file, and template actions:
```yaml
- shell: apt update
  become: true
```

Requires `--sudo-pass` flag or `--raw` mode for interactive sudo.

### register

Capture command output in a variable:
```yaml
- shell: whoami
  register: current_user

- name: Use captured output
  shell: echo "Running as {{current_user.stdout}}"
```

Result contains:

- `rc` - Exit code
- `stdout` - Standard output
- `stderr` - Standard error
- `changed` - Whether step made changes
- `failed` - Whether step failed

### with_items

Iterate over list items:
```yaml
- shell: echo "{{item}}"
  with_items: ["a", "b", "c"]
```

Or with variables:
```yaml
- vars:
    packages: [git, vim, tmux]

- shell: brew install {{item}}
  with_items: "{{packages}}"
```

### with_filetree

Iterate over files in a directory:
```yaml
- shell: cp "{{item.src}}" "/backup/{{item.name}}"
  with_filetree: ./dotfiles
```

Item properties:

- `name` - File name
- `src` - Full source path
- `is_dir` - Whether it's a directory

### creates

Skip step if path exists (idempotency check):
```yaml
- name: Extract application
  unarchive:
    src: /tmp/myapp.tar.gz
    dest: /opt/myapp
    creates: /opt/myapp/.installed

# Second run skips - marker file exists
```

Works with all actions to provide idempotency without checking actual state:
```yaml
- name: Initialize database
  shell: pg_restore backup.sql
  creates: /var/lib/postgresql/.initialized
```

### unless

Skip step if command succeeds (conditional idempotency):
```yaml
- name: Create user
  shell: useradd myuser
  unless: id myuser

# Skips if user already exists (exit code 0)
```

The `unless` command is executed before the step. If it exits with code 0 (success), the step is skipped:
```yaml
- name: Install package
  shell: apt install nginx
  unless: dpkg -l | grep nginx
  become: true

# Skips if nginx is already installed
```

## Shell-Specific Fields

The following fields **only work with shell commands**. They don't apply to file, template, or include operations.

### become_user

Specify user when using `become` with shell commands (default is root):
```yaml
- name: Run as postgres user
  shell: psql -c "SELECT version()"
  become: true
  become_user: postgres
```

### env

Set environment variables for shell commands:
```yaml
- name: Build with custom env
  shell: make build
  env:
    CC: gcc-11
    CFLAGS: "-O2 -Wall"
    PATH: "/custom/bin:$PATH"
```

Values support template rendering.

### cwd

Change working directory before executing shell command:
```yaml
- name: Build in project directory
  shell: npm run build
  cwd: "/opt/{{project_name}}"
```

### timeout

Enforce maximum execution time (duration string):
```yaml
- name: Long running command
  shell: ./slow-script.sh
  timeout: 5m
```

Supported units: `ns`, `us`, `ms`, `s`, `m`, `h`. Command times out with exit code 124.

### retries

Retry failed shell commands up to N times:
```yaml
- name: Flaky API call
  shell: curl https://api.example.com/data
  retries: 3
  retry_delay: 5s
```

Total attempts = retries + 1 (initial attempt).

### retry_delay

Wait duration between retry attempts:
```yaml
- name: Wait for service
  shell: nc -z localhost 8080
  retries: 5
  retry_delay: 2s
```

### changed_when

Override changed status based on expression (shell commands only):
```yaml
- name: Check if update needed
  shell: git fetch && git status
  register: git_status
  changed_when: "'behind' in result.stdout"
```

Expression has access to `result.rc`, `result.stdout`, `result.stderr`.

### failed_when

Override failure status based on expression (shell commands only):
```yaml
- name: Command that may return 2
  shell: ./script.sh
  failed_when: "result.rc != 0 and result.rc != 2"
```

Useful for commands where certain non-zero exit codes are acceptable.

## Shell Command Examples

### Robust shell command with retry and timeout
```yaml
- name: Download large file
  shell: curl -O https://example.com/large-file.tar.gz
  timeout: 10m
  retries: 3
  retry_delay: 30s
  failed_when: "result.rc != 0 and result.rc != 18"  # 18 = partial transfer
```

### Conditional change detection
```yaml
- name: Update git repository
  shell: git pull
  cwd: "/opt/{{project}}"
  register: git_pull
  changed_when: "'Already up to date' not in result.stdout"
```

### Complex shell execution control
```yaml
- name: Deploy with validation
  shell: ./deploy.sh
  cwd: "/opt/app"
  env:
    ENVIRONMENT: "{{env}}"
    DEBUG: "{{debug_mode}}"
  timeout: 15m
  become: true
  become_user: deployer
  register: deploy_result
  failed_when: "result.rc != 0 or 'ERROR' in result.stderr"
  changed_when: "'deployed successfully' in result.stdout"
```

## Preset

Invoke reusable, parameterized collections of steps. Presets encapsulate complex workflows into simple declarations.

📖 **See [Presets Guide](../presets.md)** for complete documentation and **[Preset Authoring](../preset-authoring.md)** for creating your own.

### Basic Usage

Simple string form (no parameters):
```yaml
- name: Quick preset
  preset: my-preset
```

With parameters:
```yaml
- name: Install Ollama
  preset: ollama
  with:
    state: present
    service: true
    pull: [llama3.1:8b]
```

Full object form:
```yaml
- name: Deploy application
  preset:
    name: deploy-webapp
    with:
      app_name: myapp
      version: v1.2.3
      port: 8080
      environment: production
  become: true
  register: deploy_result
```

### Preset Properties

| Property | Type | Description |
|----------|------|-------------|
| `preset` | string or object | Preset name (string) or preset invocation (object) |
| `preset.name` | string | Preset name (when using object form) |
| `preset.with` | object | Parameters to pass to preset (optional) |

### Parameter Passing

Parameters are validated by the preset definition:

```yaml
- preset: ollama
  with:
    state: present         # string (required, enum: [present, absent])
    service: true          # bool (optional, default: true)
    pull: [model1, model2] # array (optional, default: [])
    force: false           # bool (optional, default: false)
```

### Result Registration

Presets return aggregate results:

```yaml
- name: Install Ollama
  preset: ollama
  with:
    state: present
  register: ollama_result

- name: Check result
  shell: echo "Changed = {{ ollama_result.changed }}"
```

**Result fields:**
- `changed`: `true` if any step in preset changed
- `failed`: `true` if preset execution failed
- `rc`: Exit code (0 = success)
- `stdout`: Summary message

### Conditionals

Presets work with standard conditionals:

```yaml
- name: Install Ollama on Linux only
  preset: ollama
  with:
    state: present
  when: os == "linux"
```

### Tags

```yaml
- name: Setup LLM
  preset: ollama
  with:
    state: present
  tags: [setup, ml]
```

### Become (Privilege Escalation)

```yaml
- name: Install system-wide
  preset: my-preset
  with:
    scope: system
  become: true
```

### Available Presets

Built-in presets:

- **ollama**: Install and manage Ollama LLM runtime
  - Parameters: `state`, `service`, `pull`, `method`, `host`, `models_dir`, `force`
  - See: [examples/ollama/](../../examples/ollama/)

### Creating Custom Presets

Place preset files in:
1. `./presets/` (playbook directory)
2. `~/.mooncake/presets/` (user directory)
3. `/usr/share/mooncake/presets/` (system directory)

Example preset (`presets/hello.yml`):
```yaml
preset:
  name: hello
  description: Print a greeting
  version: 1.0.0

  parameters:
    name:
      type: string
      required: true
      description: Name to greet

    excited:
      type: bool
      default: false
      description: Use exclamation mark

  steps:
    - name: Print greeting
      shell: echo "Hello, {{ parameters.name }}{% if parameters.excited %}!{% endif %}"
```

Usage:
```yaml
- preset: hello
  with:
    name: World
    excited: true
```

### Key Features

 **Reusability**: Write once, use everywhere
 **Type Safety**: Parameters validated before execution
 **Idempotency**: Presets inherit step idempotency
 **Platform-aware**: Can detect and adapt to OS/package managers
 **Composable**: Use with all standard step features (when, tags, register)

### Limitations

- **No Nesting**: Presets cannot call other presets (architectural constraint)
- **Flat Parameters**: No nested parameter structures (use object type for complex data)
- **Sequential Execution**: Steps run in order, not parallel

**Note**: Preset steps support includes, loops, and conditionals - the preset definition is static, but steps can be dynamically expanded.

### See Also

- [Presets User Guide](../presets.md) - Using presets
- [Preset Authoring Guide](../preset-authoring.md) - Creating presets
- [Ollama Examples](../../examples/ollama/) - Real-world preset usage

## See Also

- [Control Flow](control-flow.md) - Conditionals, loops, tags
- [Variables](variables.md) - Variable usage and system facts
- [Examples](../../examples/index.md) - Practical examples


---

<!-- FILE: guide/config/advanced.md -->

# Advanced Configuration

Advanced patterns and techniques for complex configurations.

## Multi-File Organization

Break large configurations into manageable pieces.

### Directory Structure

```
project/
├── main.yml              # Entry point
├── tasks/                # Task modules
│   ├── common.yml
│   ├── linux.yml
│   └── macos.yml
├── vars/                 # Variable files
│   ├── dev.yml
│   └── prod.yml
└── templates/            # Template files
    └── config.j2
```

### Main Configuration

```yaml
# main.yml
- vars:
    env: development

- include_vars: ./vars/{{env}}.yml

- include: ./tasks/common.yml

- include: ./tasks/{{os}}.yml
```

### Benefits

- **Maintainability** - Easier to find and update
- **Reusability** - Share modules across projects
- **Collaboration** - Work on different files
- **Testing** - Test modules independently

See [Example 10](../../examples/index.md#10-multi-file-configurations) for details.

## Complex Conditionals

### Multiple Conditions

```yaml
- name: Install on Ubuntu 20+ with enough RAM
  shell: install-heavy-package
  when: >
    distribution == "ubuntu" &&
    distribution_major >= "20" &&
    memory_total_mb >= 8000
```

### Using Register Results

```yaml
- shell: docker --version
  register: docker

- shell: curl --version
  register: curl

- name: Run if both installed
  shell: deploy-app
  when: docker.rc == 0 && curl.rc == 0
```

### Checking Changed State

```yaml
- file:
    path: /tmp/config
    state: file
    content: "data"
  register: result

- name: Restart service if config changed
  shell: systemctl restart myapp
  become: true
  when: result.changed == true
```

## Advanced Loops

### Nested Data

```yaml
- vars:
    servers:
      - name: web1
        port: 8080
      - name: web2
        port: 8081

- name: Configure {{item.name}}
  template:
    src: ./server.conf.j2
    dest: "/etc/{{item.name}}.conf"
    vars:
      server_port: "{{item.port}}"
  with_items: "{{servers}}"
```

### Filtering File Trees

```yaml
# Only process .conf files
- name: Copy config files
  shell: cp "{{item.src}}" "/backup/{{item.name}}"
  with_filetree: ./configs
  when: item.name.endswith(".conf") && item.is_dir == false
```

### Multiple Loops

```yaml
- vars:
    environments: [dev, prod]
    services: [web, api, worker]

# First loop
- name: Create env directory
  file:
    path: "/opt/{{item}}"
    state: directory
  with_items: "{{environments}}"

# Second loop
- name: Configure service
  shell: setup-{{item}}
  with_items: "{{services}}"
```

## Dynamic Templates

### Template Variables

```yaml
- vars:
    servers:
      - host: server1.com
        port: 443
      - host: server2.com
        port: 443

- template:
    src: ./load-balancer.conf.j2
    dest: /etc/nginx/nginx.conf
```

**load-balancer.conf.j2:**
```nginx
upstream backend {
    {% for server in servers %}
    server {{server.host}}:{{server.port}};
    {% endfor %}
}

server {
    {% if ssl_enabled %}
    listen 443 ssl;
    ssl_certificate {{ssl_cert}};
    {% else %}
    listen 80;
    {% endif %}

    location / {
        proxy_pass http://backend;
    }
}
```

### Conditional Sections

```jinja
{% if os == "linux" %}
# Linux-specific config
user www-data;
pid /var/run/nginx.pid;
{% elif os == "darwin" %}
# macOS-specific config
user _www;
pid /usr/local/var/run/nginx.pid;
{% endif %}
```

### Template Filters

```jinja
# Expand home directory
config_path: {{ "~/.config/app" | expanduser }}

# String manipulation
app_name: {{ name | upper }}
description: {{ desc | lower }}

# Default values
port: {{ port | default:"8080" }}
```

## Workflow Orchestration

### Phased Deployment

```yaml
# Phase 1: Preparation
- name: Backup current version
  shell: backup-app
  tags: [backup, phase1]

- name: Stop services
  shell: systemctl stop myapp
  become: true
  tags: [stop, phase1]

# Phase 2: Deploy
- name: Deploy new version
  shell: install-new-version
  tags: [deploy, phase2]

# Phase 3: Start
- name: Start services
  shell: systemctl start myapp
  become: true
  tags: [start, phase3]

# Phase 4: Verify
- name: Health check
  shell: curl localhost:8080/health
  register: health
  tags: [verify, phase4]
```

**Run specific phases:**
```bash
# Run only backup and stop
mooncake run --config deploy.yml --tags phase1

# Run only deployment
mooncake run --config deploy.yml --tags phase2

# Run all phases
mooncake run --config deploy.yml
```

### Environment-Specific Workflows

```yaml
- vars:
    env: "{{ lookup('env', 'ENVIRONMENT') or 'dev' }}"

- include_vars: ./vars/{{env}}.yml

# Dev-specific steps
- name: Enable debug logging
  shell: enable-debug
  when: env == "dev"
  tags: [dev]

# Prod-specific steps
- name: Configure monitoring
  shell: setup-monitoring
  when: env == "prod"
  tags: [prod]
```

## Error Handling

### Check Before Action

```yaml
- shell: which docker
  register: docker_check

- name: Fail if Docker missing
  shell: echo "Docker required but not installed" && exit 1
  when: docker_check.rc != 0
```

### Conditional Installation

```yaml
- shell: python3 --version
  register: python

- name: Install Python
  shell: apt install python3
  become: true
  when: python.rc != 0
```

### Verify Operations

```yaml
- file:
    path: /tmp/important-file
    state: file
    content: "data"
  register: file_result

- shell: test -f /tmp/important-file
  register: verify

- name: Alert if verification failed
  shell: echo "File creation failed!" && exit 1
  when: verify.rc != 0
```

## Performance Optimization

### Skip Unchanged Files

```yaml
- name: Deploy config
  template:
    src: ./app.conf.j2
    dest: /etc/app.conf
  register: config

- name: Restart only if config changed
  shell: systemctl restart myapp
  become: true
  when: config.changed == true
```

### Batch Operations

```yaml
# Instead of individual package installs
- vars:
    packages: [git, curl, vim, tmux, htop]

- name: Install all packages at once
  shell: apt install -y {{packages | join(' ')}}
  become: true
```

### Targeted Execution

```bash
# Run only what you need
mooncake run --config config.yml --tags deploy

# Skip expensive operations
mooncake run --config config.yml --tags quick
```

## Debugging

### Verbose Logging

```bash
# Debug level shows all details
mooncake run --config config.yml --log-level debug
```

### Dry Run

```bash
# See what would run without executing
mooncake run --config config.yml --dry-run
```

### Selective Debugging

```yaml
- name: Debug info
  shell: |
    echo "OS: {{os}}"
    echo "Arch: {{arch}}"
    echo "Home: {{user_home}}"
  tags: [debug]
```

Run only debug steps:
```bash
mooncake run --config config.yml --tags debug
```

## Best Practices

1. **Start Simple** - Begin with basic config, add complexity gradually
2. **Test Incrementally** - Use `--dry-run` after each change
3. **Document Decisions** - Add comments explaining non-obvious choices
4. **Version Control** - Keep configurations in git
5. **Environment Separation** - Use separate variable files for dev/prod
6. **Tag Consistently** - Use a clear tagging scheme
7. **Fail Fast** - Validate prerequisites early in the workflow
8. **Idempotency** - Make operations safe to run multiple times

## See Also

- [Multi-File Example](../../examples/index.md#10-multi-file-configurations) - Organization patterns
- [Real-World Example](../../examples/index.md#dotfiles-manager) - Complete application
- [Control Flow](control-flow.md) - Conditionals and loops
- [Variables](variables.md) - Variable management


---

<!-- FILE: guide/config/control-flow.md -->

# Control Flow

Control when and how steps execute using conditionals, loops, and tags.

## Conditionals (when)

Execute steps based on conditions.

### Basic Conditionals

```yaml
- name: Linux only
  shell: echo "Running on Linux"
  when: os == "linux"
```

### Comparison Operators

- `==` - equals
- `!=` - not equals
- `>`, `<` - greater/less than
- `>=`, `<=` - greater/less than or equal

```yaml
- name: High memory systems
  shell: echo "Lots of RAM"
  when: memory_total_mb >= 16000

- name: Ubuntu 22+
  shell: apt install package
  when: distribution == "ubuntu" && distribution_major >= "22"
```

### Logical Operators

- `&&` - AND
- `||` - OR
- `!` - NOT

```yaml
- name: ARM Mac only
  shell: echo "ARM macOS"
  when: os == "darwin" && arch == "arm64"

- name: Debian-based systems
  shell: apt update
  when: distribution == "ubuntu" || distribution == "debian"

- name: Not Windows
  shell: echo "Unix-like system"
  when: os != "windows"
```

### Using Register Results

```yaml
- shell: which docker
  register: docker_check

- shell: echo "Docker not installed"
  when: docker_check.rc != 0
```

## Tags

Filter which steps run using command-line flags.

### Adding Tags

```yaml
- name: Development setup
  shell: install-dev-tools
  tags: [dev]

- name: Production deployment
  shell: deploy-app
  tags: [prod, deploy]
```

### Running Tagged Steps

```bash
# Run only dev steps
mooncake run --config config.yml --tags dev

# Run multiple tag categories
mooncake run --config config.yml --tags dev,test

# Run all steps (no filter)
mooncake run --config config.yml
```

### Tag Behavior

**No `--tags` flag:**
- All steps run (tagged and untagged)

**With `--tags dev`:**
- Only steps with `dev` tag run
- Untagged steps are skipped

**With `--tags dev,prod`:**
- Steps run if they have ANY matching tag (OR logic)
- Step with `[dev]` runs
- Step with `[prod]` runs
- Step with `[dev, prod]` runs
- Step with `[test]` does NOT run

### Organization Strategies

**By Environment:**
```yaml
tags: [dev, staging, prod]
```

**By Phase:**
```yaml
tags: [setup, deploy, test, cleanup]
```

**By Component:**
```yaml
tags: [database, webserver, cache]
```

## Loops

Avoid repetition by iterating over lists or files.

### List Iteration (with_items)

```yaml
- vars:
    packages: [git, curl, vim]

- name: Install package
  shell: brew install {{item}}
  with_items: "{{packages}}"
```

**Inline lists:**
```yaml
- name: Create user directory
  file:
    path: "/home/{{item}}"
    state: directory
  with_items: [alice, bob, charlie]
```

### File Tree Iteration (with_filetree)

```yaml
- name: Copy dotfiles
  shell: cp "{{item.src}}" "~/{{item.name}}"
  with_filetree: ./dotfiles
  when: item.is_dir == false
```

**Available properties:**
- `item.src` - Full source path
- `item.name` - File name
- `item.is_dir` - Boolean (true for directories)

### Combining Loops and Conditionals

```yaml
- name: Install Linux packages
  shell: apt install {{item}}
  become: true
  with_items: "{{packages}}"
  when: os == "linux"
```

## Privilege Escalation (become)

Run commands with sudo.

```yaml
- name: Update package list
  shell: apt update
  become: true
```

### Providing Sudo Password

**Command line:**
```bash
mooncake run --config config.yml --sudo-pass mypassword
```

**Environment variable:**
```bash
export MOONCAKE_SUDO_PASS=mypassword
mooncake run --config config.yml
```

### OS-Specific Sudo

```yaml
# Linux needs sudo for system packages
- name: Install package (Linux)
  shell: apt install curl
  become: true
  when: os == "linux"

# macOS Homebrew doesn't need sudo
- name: Install package (macOS)
  shell: brew install curl
  when: os == "darwin"
```

## Register

Capture command output for use in later steps.

```yaml
- name: Check for Docker
  shell: which docker
  register: docker_check

- name: Install Docker
  shell: install-docker
  when: docker_check.rc != 0
```

### Available Fields

**For shell commands:**
- `.stdout` - Standard output
- `.stderr` - Standard error
- `.rc` - Return code (0 = success)
- `.failed` - Boolean (true if rc != 0)
- `.changed` - Boolean

**For file/template:**
- `.rc` - 0 for success, 1 for failure
- `.failed` - Boolean
- `.changed` - Boolean (true if file modified)

### Using Captured Data

```yaml
- shell: hostname
  register: host_info

- shell: echo "Running on {{host_info.stdout}}"

- file:
    path: "/tmp/{{host_info.stdout}}_config"
    state: file
```

## Combining Control Flow

All control flow features work together:

```yaml
- vars:
    packages: [neovim, ripgrep, fzf]

- name: Install dev tool
  shell: brew install {{item}}
  with_items: "{{packages}}"
  when: os == "darwin"
  tags: [dev, tools]
```

This step:
- ✓ Iterates over packages
- ✓ Only runs on macOS
- ✓ Only runs with `--tags dev` or `--tags tools`

## See Also

- [Actions](actions.md) - Available actions
- [Variables](variables.md) - Using variables
- [Examples](../../examples/index.md#04-conditionals) - Conditional examples
- [Examples](../../examples/index.md#06-loops) - Loop examples
- [Examples](../../examples/index.md#08-tags) - Tag examples


---

<!-- FILE: guide/config/variables.md -->

# Variables

Variables make configurations reusable and dynamic. Use system facts and custom variables throughout your configuration.

## Defining Variables

### Inline Variables

```yaml
- vars:
    app_name: myapp
    version: "1.0.0"
    port: 8080
```

### External Variable Files

```yaml
- name: Load variables
  include_vars: ./vars/common.yml
```

**vars/common.yml:**
```yaml
app_name: myapp
environment: development
debug: true
```

### Dynamic Variable Loading

```yaml
- vars:
    env: production

- include_vars: ./vars/{{env}}.yml
```

## Using Variables

### In Shell Commands

```yaml
- vars:
    package: neovim

- shell: brew install {{package}}
```

### In File Paths

```yaml
- vars:
    app_dir: /opt/myapp

- file:
    path: "{{app_dir}}/config"
    state: directory
```

### In File Content

```yaml
- vars:
    api_key: secret123

- file:
    path: /tmp/config.txt
    state: file
    content: |
      api_key: {{api_key}}
      environment: production
```

### In Templates

**config.yml:**
```yaml
- template:
    src: ./nginx.conf.j2
    dest: /etc/nginx/nginx.conf
```

**nginx.conf.j2:**
```nginx
server {
    listen {{port}};
    server_name {{hostname}};
}
```

## System Facts

Mooncake automatically provides system information as variables.

### Basic Facts

```yaml
# Operating system
os: "linux"                    # linux, darwin, windows
arch: "amd64"                  # amd64, arm64, 386, etc.
hostname: "myserver"
username: "admin"
user_home: "/home/admin"
kernel_version: "6.5.0-14"     # Kernel/Darwin version
```

### Distribution Info

```yaml
distribution: "ubuntu"           # ubuntu, debian, centos, macos, etc.
distribution_version: "22.04"    # Full version
distribution_major: "22"         # Major version only
```

### CPU Facts

```yaml
cpu_cores: 8                                        # Number of CPU cores
cpu_model: "Intel(R) Core(TM) i7-10700K"           # CPU model name
cpu_flags: ["avx", "avx2", "sse4_2", "fma", "aes"] # CPU feature flags
cpu_flags_string: "avx avx2 sse4_2 fma aes"        # Flags as string
```

### Memory Facts

```yaml
memory_total_mb: 16384      # Total RAM in megabytes
memory_free_mb: 8192        # Available RAM in megabytes
swap_total_mb: 4096         # Total swap space
swap_free_mb: 2048          # Available swap space
```

### Network Facts

```yaml
# IP addresses
ip_addresses: ["192.168.1.100", "10.0.0.5"]
ip_addresses_string: "192.168.1.100, 10.0.0.5"

# Network configuration
default_gateway: "192.168.1.1"
dns_servers: ["8.8.8.8", "1.1.1.1"]
dns_servers_string: "8.8.8.8, 1.1.1.1"

# Network interfaces (array - can iterate)
network_interfaces:
  - name: "eth0"
    mac_address: "00:11:22:33:44:55"
    mtu: 1500
    addresses: ["192.168.1.100/24"]
    up: true
```

### GPU Facts

```yaml
# GPUs array - can iterate with {% for gpu in gpus %}
gpus:
  - vendor: "nvidia"
    model: "GeForce RTX 4090"
    memory: "24GB"
    driver: "535.54.03"
    cuda_version: "12.3"     # NVIDIA only
```

### Storage Facts

```yaml
# Disks array - can iterate with {% for disk in disks %}
disks:
  - device: "/dev/sda1"
    mount_point: "/"
    filesystem: "ext4"
    size_gb: 500
    used_gb: 250
    avail_gb: 250
    used_pct: 50
```

### Software Detection

```yaml
# Package managers and languages
package_manager: "apt"      # apt, yum, brew, pacman, etc.
python_version: "3.11.5"    # Installed Python version

# Development tools
docker_version: "24.0.7"    # Docker version (if installed)
git_version: "2.43.0"       # Git version (if installed)
go_version: "1.21.5"        # Go version (if installed)
```

## Viewing System Facts

Run `mooncake facts` to see all available facts:

```bash
mooncake facts
```

Output shows:
- Operating system details (OS, distribution, kernel version)
- CPU (cores, model, flags)
- Memory (total, free, swap)
- GPUs (vendor, model, memory, driver, CUDA version)
- Storage devices (disks with mount points and sizes)
- Network (interfaces, gateway, DNS)
- Software (package manager, Python, Docker, Git, Go)

## Using System Facts

### OS Detection

```yaml
- shell: apt update
  when: os == "linux"

- shell: brew update
  when: os == "darwin"
```

### Distribution-Specific Commands

```yaml
- shell: apt install package
  when: distribution == "ubuntu" || distribution == "debian"

- shell: yum install package
  when: distribution == "centos" || distribution == "fedora"
```

### Architecture Detection

```yaml
- shell: install-amd64-binary
  when: arch == "amd64"

- shell: install-arm64-binary
  when: arch == "arm64"
```

### Memory-Based Decisions

```yaml
- name: Configure for high-memory system
  shell: set-large-buffers
  when: memory_total_mb >= 32000

- name: Check available memory
  shell: echo "Free memory: {{memory_free_mb}}MB"
```

### Package Manager Detection

```yaml
- shell: "{{package_manager}} install neovim"
  when: os == "linux"
```

### Iterating Over Arrays

```yaml
# Iterate over disks
- name: Show disk info
  shell: |
    {% for disk in disks %}
    echo "Disk: {{ disk.Device }} mounted at {{ disk.MountPoint }} ({{ disk.SizeGB }}GB)"
    {% endfor %}

# Iterate over GPUs
- name: Setup GPU
  shell: nvidia-smi -i {{loop.index0}}
  with_items: "{{gpus}}"
  when: gpus|length > 0

# Iterate over network interfaces
- name: Configure interface
  shell: |
    {% for iface in network_interfaces %}
    {% if iface.Up %}
    echo "Active: {{ iface.Name }} ({{ iface.MACAddress }})"
    {% endif %}
    {% endfor %}
```

### Toolchain Detection

```yaml
# Check if Docker is installed
- name: Run Docker container
  shell: docker run hello-world
  when: docker_version != ""

# Use Git if available
- name: Clone repository
  shell: git clone https://github.com/user/repo.git
  when: git_version != ""

# Show installed versions
- shell: |
    echo "Docker: {{docker_version}}"
    echo "Git: {{git_version}}"
    echo "Go: {{go_version}}"
```

## Variable Precedence

When the same variable is defined in multiple places:

1. **Template vars** (highest priority)
   ```yaml
   - template:
       vars:
         port: 9000
   ```

2. **Step-level vars**
   ```yaml
   - vars:
       port: 8080
   ```

3. **Included vars**
   ```yaml
   - include_vars: ./vars.yml
   ```

4. **System facts** (lowest priority)
   ```yaml
   # Automatically available
   os: "linux"
   ```

## Variable Scoping

Variables are available to all subsequent steps:

```yaml
# Step 1: Define
- vars:
    app_name: myapp

# Step 2: Use in same file
- shell: echo "{{app_name}}"

# Step 3: Use in included files
- include: ./tasks/setup.yml  # Can use app_name
```

## Register Variables

Capture command output as variables:

```yaml
- shell: whoami
  register: current_user

- shell: echo "User is {{current_user.stdout}}"

- file:
    path: "/home/{{current_user.stdout}}/config"
    state: file
```

## Loop Variables

Special `item` variable in loops:

```yaml
- vars:
    users: [alice, bob]

- name: Create directory for {{item}}
  file:
    path: "/home/{{item}}"
    state: directory
  with_items: "{{users}}"
```

## Best Practices

1. **Use descriptive names**
   ```yaml
   # Good
   database_host: "localhost"

   # Bad
   h: "localhost"
   ```

2. **Quote version strings**
   ```yaml
   # Good
   version: "1.0.0"

   # Bad (may be parsed as number)
   version: 1.0.0
   ```

3. **Group related variables**
   ```yaml
   - vars:
       # Database config
       db_host: localhost
       db_port: 5432
       db_name: myapp

       # App config
       app_port: 8080
       app_debug: false
   ```

4. **Use external files for environments**
   ```
   vars/
     development.yml
     staging.yml
     production.yml
   ```

5. **Use system facts when possible**
   ```yaml
   # Good - adapts to system
   - shell: "{{package_manager}} install curl"

   # Bad - hardcoded for one OS
   - shell: apt install curl
   ```

## See Also

- [Actions](actions.md) - Using variables in actions
- [Control Flow](control-flow.md) - Using variables in conditions
- [Examples](../../examples/index.md#02-variables-and-system-facts) - Variable examples
- [Commands](../commands.md#mooncake-facts) - View system facts


---

<!-- FILE: guide/core-concepts.md -->

# Core Concepts

Mooncake configurations are YAML files containing an array of **steps**. Each step performs one **action**.

## Two-Phase Architecture

Mooncake uses a two-phase architecture for configuration execution:

1. **Planning Phase** - Expands configuration into a deterministic plan
   - Resolves all includes recursively
   - Expands all loops (`with_items`, `with_filetree`) into individual steps
   - Tracks origin (file:line:col) for every step
   - Filters steps by tags (marked as skipped)
   - Produces a deterministic, inspectable plan

2. **Execution Phase** - Executes the plan
   - Evaluates `when` conditions at runtime
   - Executes actions (shell, file, template, etc.)
   - Captures results and updates variables
   - Logs progress and status

**Benefits:**
- **Deterministic** - Same config always produces the same plan
- **Inspectable** - Use `mooncake plan` to see what will execute
- **Traceable** - Every step tracks its origin with include chain
- **Debuggable** - Understand loop expansions and includes before execution

## Steps

Steps are executed sequentially:

```yaml
- name: First step
  shell: echo "hello"

- name: Second step
  file:
    path: /tmp/test
    state: directory
```

## Actions

Available actions:
- **shell** / **command** - Execute shell commands or direct commands
- **file** - Create files, directories, links, and manage permissions
- **copy** - Copy files with checksum verification
- **download** - Download files from URLs with checksums and retry
- **unarchive** - Extract tar.gz, zip archives with security protections
- **template** - Render configuration templates
- **service** - Manage system services (systemd on Linux, launchd on macOS)
- **assert** - Verify state (command results, file properties, HTTP responses)
- **preset** - Invoke reusable, parameterized workflows (e.g., ollama preset)
- **include** - Load other configuration files
- **include_vars** - Load variables from files
- **vars** - Define variables

## Variables

Use `{{variable}}` syntax for dynamic values:

```yaml
- vars:
    app_name: MyApp

- shell: echo "Installing {{app_name}}"
```

## System Facts

Automatically available variables:
- `os` - Operating system (linux, darwin, windows)
- `arch` - Architecture (amd64, arm64)
- `hostname` - System hostname
- `distribution` - Linux/macOS distribution

See all facts: `mooncake facts`

## Next

Continue to [Commands](commands.md) to learn about CLI usage.


---

<!-- FILE: guide/faq.md -->

# Frequently Asked Questions

Common questions about Mooncake and their answers.

---

## General Questions

### What is Mooncake?

Mooncake is a configuration management tool designed specifically for AI agents and modern development workflows. It provides a safe, validated execution environment for system configuration with idempotency guarantees, dry-run validation, and full observability.

Think of it as "the standard runtime for AI system configuration" - similar to how Docker provides a standard runtime for containers.

### Why "Mooncake"?

The name comes from the show "Final Space" where mooncakes are a beloved treat. Also, configuration management should be as delightful as eating mooncakes! **Chookity!**

### Is Mooncake production-ready?

Yes! Mooncake is actively used for:
- Personal dotfiles management
- Development environment setup
- System provisioning
- AI agent configuration tasks

It has comprehensive test coverage, runs on multiple platforms, and follows semantic versioning.

---

## Comparison Questions

### How is Mooncake different from Ansible?

| Feature | Mooncake | Ansible |
|---------|----------|---------|
| **Target Audience** | AI agents, developers, dotfiles | Enterprise infrastructure |
| **Installation** | Single binary, zero dependencies | Python + pip packages + galaxy collections |
| **Setup** | None required | Inventory files, host management |
| **Complexity** | Simple YAML | Complex playbooks, roles, collections |
| **AI-Friendly** | Designed for AI generation | Complex for AI to generate correctly |
| **Dry-run** | Built-in, always available | Check mode (limited) |
| **Learning Curve** | Minutes | Hours to days |

**When to use Mooncake**:

- AI agent configuration tasks
- Personal dotfiles and dev environments
- Simple automation scripts
- Cross-platform configurations
- When you want zero dependencies

**When to use Ansible**:

- Large-scale enterprise infrastructure
- Complex role-based architectures
- Existing Ansible investments
- Multi-host orchestration

### How is Mooncake different from shell scripts?

| Feature | Mooncake | Shell Scripts |
|---------|----------|---------------|
| **Idempotency** | Guaranteed | Manual |
| **Dry-run** | Native | Manual implementation |
| **Error Handling** | Built-in | Manual |
| **Cross-platform** | Unified syntax | OS-specific scripts |
| **Validation** | Schema validation | None |
| **Variables** | Built-in with facts | Manual |

**Mooncake provides**:

- Automatic idempotency
- Built-in dry-run mode
- Schema validation
- Cross-platform abstractions
- Structured error handling
- System fact detection

### Can I migrate from Ansible to Mooncake?

Yes! Mooncake uses similar YAML syntax. See the [Migration Guide](guide/migration.md) for details.

Common migrations:

**Ansible playbook**:
```yaml
- hosts: localhost
  tasks:
    - name: Install package
      apt:
        name: neovim
        state: present
      become: yes
```

**Mooncake equivalent**:
```yaml
- name: Install package
  shell: apt install -y neovim
  become: true
  when: package_manager == "apt"
```

---

## AI & LLM Questions

### Can AI agents use Mooncake safely?

Yes! Mooncake was designed specifically for AI agents:

1. **Safe by Default**: Dry-run mode lets AI preview changes before applying
2. **Validated Operations**: Schema validation prevents malformed configurations
3. **Idempotency**: Same config can be run multiple times safely
4. **Full Observability**: Structured events enable AI to understand execution
5. **Simple Format**: YAML is easy for AI models to generate and parse

### How do AI models generate Mooncake configs?

1. **Use the AI Specification**: See [AI Specification](ai-specification.md) for a complete guide LLMs can follow

2. **Provide system context**:
```bash
# Give AI the system facts
mooncake facts --format json > facts.json
```

3. **Let AI generate config**:
```yaml
# AI generates based on request and facts
- name: Install development tools
  shell: {{package_manager}} install {{item}}
  with_items: [git, vim, curl]
  become: true
  when: os == "linux"
```

4. **Validate before executing**:
```bash
# AI can validate without risk
mooncake run --config config.yml --dry-run
```

### What's the AI agent workflow?

```
1. User Request → AI Agent
2. AI generates Mooncake config
3. AI runs dry-run to validate
4. AI shows preview to user
5. User approves
6. AI executes configuration
7. AI observes results via events
```

---

## Security Questions

### Is it safe to give AI agents sudo access?

Mooncake provides several safety layers:

1. **Dry-run First**: Always preview with `--dry-run`
2. **Explicit sudo**: Only steps with `become: true` get sudo
3. **Password Control**: You control sudo password access
4. **Validation**: Schema validation prevents malformed commands
5. **Audit Trail**: Full logging of all operations

**Best practices**:

- Always review dry-run output before executing
- Use tags to limit execution scope
- Run sensitive operations manually
- Monitor execution logs

### How do I handle secrets?

**Option 1: Environment Variables**
```yaml
- name: Use secret
  shell: echo "API_KEY=$API_KEY"
  environment:
    API_KEY: "{{ lookup('env', 'API_KEY') }}"
```

**Option 2: Password Files**
```yaml
# Load from secure file
- include_vars:
    file: ~/.mooncake/secrets.yml

- name: Use secret
  shell: echo "{{api_key}}"
```

**Option 3: External Secret Management**
```yaml
# Fetch from vault/keychain
- name: Get secret
  shell: security find-generic-password -s myapp -w
  register: secret
  no_log: true

- name: Use secret
  shell: curl -H "Authorization: {{secret.stdout}}"
```

**Never**:

- Commit secrets to version control
- Print secrets in logs (`no_log: true`)
- Use plain text passwords in configs

### Can I restrict what AI agents can do?

Yes, several ways:

1. **Tags**: Limit execution to specific operations
```yaml
# AI can only run setup tasks
- name: Setup step
  shell: setup.sh
  tags: [setup]
```
```bash
mooncake run --config config.yml --tags setup
```

2. **Conditional Execution**: Restrict by facts
```yaml
# Only allow dev operations
- name: Dev task
  shell: install-dev-tools.sh
  when: environment == "dev"
```

3. **File Permissions**: Control config access with file permissions

4. **Sudo Control**: Control sudo password access

---

## Technical Questions

### What languages/tools does Mooncake support?

**Built-in language version managers**:

- Python (pyenv)
- Node.js (nvm)
- Ruby (rbenv)
- Go (direct install)
- Rust (rustup)
- Java (OpenJDK)

**Package managers detected automatically**:

- apt, dnf, yum, zypper, pacman, apk (Linux)
- brew, port (macOS)
- choco, scoop (Windows)

**See all presets**:
```bash
mooncake presets list
```

### Does Mooncake work on Windows?

Yes! Mooncake supports Windows with some limitations:

**Fully supported**:

- Shell commands (PowerShell/cmd)
- File operations
- Variable expansion
- Templates
- Downloads

**Limited support**:

- Service management (basic Windows services)
- Package management (via choco)

**Use conditionals for cross-platform configs**:
```yaml
- name: Unix command
  shell: ls -la
  when: os != "windows"

- name: Windows command
  shell: dir
  when: os == "windows"
```

### Can I use Mooncake in CI/CD?

Yes! Mooncake works great in CI/CD:

```bash
# Disable interactive TUI
mooncake run --config config.yml --raw

# JSON output for parsing
mooncake run --config config.yml --raw --output-format json

# Exit codes
# 0 = success
# 1+ = failure
```

**Example GitHub Actions**:
```yaml
- name: Install Mooncake
  run: go install github.com/alehatsman/mooncake@latest

- name: Run configuration
  run: mooncake run --config config.yml --raw

- name: Verify
  run: mooncake facts
```

### Does Mooncake support remote hosts?

Not yet. Mooncake currently executes on localhost only. Remote execution is planned for a future release.

For now, you can:
1. Copy config to remote host and run locally
2. Use SSH wrapper scripts
3. Wait for remote execution support (coming soon!)

### Can I create my own presets?

Yes! Presets are just YAML files:

**Create preset structure**:
```bash
mkdir -p ~/.mooncake/presets/mypreset
cat > ~/.mooncake/presets/mypreset/preset.yml <<EOF
name: mypreset
version: "1.0.0"
description: My custom preset

parameters:
  - name: state
    type: string
    default: present
    enum: [present, absent]

steps:
  - name: Install
    shell: echo "Installing with state={{parameters.state}}"
    when: parameters.state == "present"
EOF
```

**Use it**:
```yaml
- preset: mypreset
  with:
    state: present
```

See [Preset Authoring Guide](guide/preset-authoring.md) for details.

---

## Usage Questions

### Can I use Mooncake for dotfiles?

Yes! Mooncake is excellent for dotfiles:

```yaml
- name: Create config directories
  file:
    path: "{{item}}"
    state: directory
  with_items:
    - ~/.config/nvim
    - ~/.config/tmux
    - ~/.config/zsh

- name: Deploy dotfiles
  copy:
    src: "{{item.src}}"
    dest: "~/{{item.name}}"
  with_filetree: ./dotfiles
  when: item.is_dir == false
```

**Benefits**:

- Idempotent (run multiple times safely)
- Cross-platform (same config for macOS/Linux)
- Dry-run before applying
- Version control friendly

### Can I use loops?

Yes! Two types:

**List loops**:
```yaml
- name: Install packages
  shell: brew install {{item}}
  with_items:
    - neovim
    - ripgrep
    - fzf
```

**File tree loops**:
```yaml
- name: Deploy configs
  copy:
    src: "{{item.src}}"
    dest: "~/{{item.name}}"
  with_filetree: ./configs
  when: item.is_dir == false
```

### Can I split configs across multiple files?

Yes! Use `include_vars` and organize by topic:

**Main config**:
```yaml
- include_vars:
    file: ./vars/dev-tools.yml

- include_vars:
    file: ./vars/languages.yml

- name: Install dev tools
  shell: brew install {{item}}
  with_items: "{{dev_tools}}"
```

**vars/dev-tools.yml**:
```yaml
dev_tools:
  - neovim
  - ripgrep
  - fzf
```

### How do I handle different environments (dev/staging/prod)?

**Option 1: Environment-specific configs**
```bash
mooncake run --config config-dev.yml
mooncake run --config config-prod.yml
```

**Option 2: Variables**
```yaml
- vars:
    environment: dev  # Change per environment

- name: Dev-only task
  shell: install-dev-tools.sh
  when: environment == "dev"
```

**Option 3: Tags**
```yaml
- name: Dev setup
  shell: setup-dev.sh
  tags: [dev]

- name: Prod deployment
  shell: deploy-prod.sh
  tags: [prod]
```
```bash
mooncake run --config config.yml --tags dev
```

---

## Troubleshooting Questions

### Why is my variable undefined?

Common causes:

1. **Variable not defined**:
```yaml
# Wrong
- shell: echo "{{my_var}}"

# Right
- vars:
    my_var: value
- shell: echo "{{my_var}}"
```

2. **Using system fact incorrectly**:
```yaml
# Wrong - no such fact
- shell: echo "{{operating_system}}"

# Right - use 'os'
- shell: echo "{{os}}"
```

3. **Variable scope issue** - variables are scoped to the config file

Check available facts:
```bash
mooncake facts
```

### Why does my step keep running (not idempotent)?

Some operations aren't idempotent by default:

**Problem**:
```yaml
# Runs every time
- shell: echo "test" >> /tmp/file
```

**Solutions**:

1. Use idempotent actions:
```yaml
- file:
    path: /tmp/file
    state: file
    content: "test"  # Idempotent
```

2. Use `creates` condition:
```yaml
- shell: echo "test" > /tmp/file
  args:
    creates: /tmp/file  # Only if doesn't exist
```

3. Use `changed_when`:
```yaml
- shell: echo "test" >> /tmp/file
  register: result
  changed_when: false  # Never report as changed
```

### How do I debug template errors?

1. **Check variable values**:
```yaml
- name: Debug variable
  shell: echo "Value is {{my_var}}"
```

2. **Test template separately**:
```bash
# Create test template
echo "{{ my_var }}" > test.j2

# Test with mooncake
mooncake run --config test-template.yml
```

3. **Use simpler templates first**:
```yaml
# Start simple
- template:
    dest: /tmp/test
    content: "{{ simple_var }}"

# Then add complexity
- template:
    src: complex.j2
    dest: /tmp/test
```

---

## Performance Questions

### Is Mooncake fast?

Yes! Mooncake is written in Go and has minimal overhead:

- Binary size: ~20MB
- Startup time: <100ms
- Memory usage: <50MB typically
- No interpreter overhead (unlike Python-based tools)

### Can I run steps in parallel?

Not yet. Steps currently run sequentially for safety and predictability. Parallel execution is planned for a future release.

### How do I make my configs faster?

1. **Remove unnecessary operations**:
```yaml
# Slow - updates package cache every time
- shell: apt update && apt install {{item}}
  with_items: [vim, git, curl]

# Fast - update once
- shell: apt update
- shell: apt install -y {{item}}
  with_items: [vim, git, curl]
```

2. **Use tags to run only what's needed**:
```bash
mooncake run --config config.yml --tags quick
```

3. **Use `creates`/`removes` for idempotency**:
```yaml
- shell: tar xzf large-file.tar.gz
  args:
    creates: /opt/app/bin/app  # Skip if already extracted
```

---

## Contributing Questions

### How can I contribute?

Contributions are welcome!

1. **Report bugs**: [GitHub Issues](https://github.com/alehatsman/mooncake/issues)
2. **Request features**: [GitHub Issues](https://github.com/alehatsman/mooncake/issues)
3. **Submit PRs**: See [Contributing Guide](development/contributing.md)
4. **Share presets**: Submit to the presets repository
5. **Improve docs**: Documentation PRs always welcome!

### What's the roadmap?

See the [Roadmap](development/roadmap.md) for planned features:
- Remote host execution
- Parallel step execution
- Enhanced service management
- More built-in actions
- Plugin system

---

## See Also

- [Quick Reference](quick-reference.md) - One-page cheat sheet
- [Troubleshooting](guide/troubleshooting.md) - Common issues and solutions
- [Full Documentation](https://mooncake.alehatsman.com) - Complete guide
- [GitHub Issues](https://github.com/alehatsman/mooncake/issues) - Ask questions


---

<!-- FILE: guide/preset-authoring.md -->

# Creating Presets

This guide shows you how to create your own mooncake presets for sharing complex workflows and configurations.

## Preset Structure

### Flat Structure (Simple)

A preset is a YAML file with this structure:

```yaml
preset:
  name: my-preset
  description: What this preset does
  version: 1.0.0

  parameters:
    param1:
      type: string
      required: true
      description: Description of param1

    param2:
      type: bool
      default: false
      description: Description of param2

  steps:
    - name: First step
      shell: echo "{{ parameters.param1 }}"

    - name: Second step
      file:
        path: /tmp/flag
        state: file
      when: parameters.param2
```

### Directory Structure (Advanced)

For complex presets with multiple files, use a directory structure:

```
presets/
└── my-preset/
    ├── preset.yml           # Main preset definition
    ├── tasks/               # Modular task files
    │   ├── install.yml
    │   ├── configure.yml
    │   └── cleanup.yml
    ├── templates/           # Configuration templates
    │   ├── config.j2
    │   └── service.j2
    └── README.md            # Documentation
```

The main preset file uses `include` to organize steps:

```yaml
# presets/my-preset/preset.yml
preset:
  name: my-preset
  description: Modular preset with includes
  version: 1.0.0

  parameters:
    state:
      type: string
      enum: [present, absent]

  steps:
    - name: Install
      include: tasks/install.yml
      when: parameters.state == "present"

    - name: Configure
      include: tasks/configure.yml
      when: parameters.state == "present"

    - name: Cleanup
      include: tasks/cleanup.yml
      when: parameters.state == "absent"
```

## Minimal Example

The simplest preset:

```yaml
preset:
  name: hello
  description: Print hello message
  version: 1.0.0

  steps:
    - name: Say hello
      shell: echo "Hello from preset!"
```

Usage:
```yaml
- preset: hello
```

## Parameters

### Defining Parameters

```yaml
parameters:
  environment:
    type: string
    required: true
    enum: [dev, staging, production]
    description: Deployment environment

  replicas:
    type: number
    required: false
    default: 3
    description: Number of replicas

  features:
    type: array
    required: false
    default: []
    description: Feature flags to enable

  config:
    type: object
    required: false
    description: Additional configuration
```

### Parameter Types

| Type | Go Type | YAML Example |
|------|---------|--------------|
| `string` | `string` | `"value"` |
| `bool` | `bool` | `true` / `false` |
| `array` | `[]interface{}` | `[item1, item2]` |
| `object` | `map[string]interface{}` | `{key: value}` |

### Accessing Parameters

Parameters are available under the `parameters` namespace:

```yaml
steps:
  - name: Use string parameter
    shell: echo "Env{{ ":" }} {{ parameters.environment }}"

  - name: Use boolean parameter
    file:
      path: /tmp/feature
      state: file
    when: parameters.enable_feature

  - name: Loop over array parameter
    shell: echo "Feature{{ ":" }} {{ item }}"
    with_items: "{{ parameters.features }}"

  - name: Access object parameter
    shell: echo "DB{{ ":" }} {{ parameters.config.database_url }}"
```

## Includes

### Using Includes for Modularity

Break large presets into smaller, focused files using `include`:

```yaml
# preset.yml
steps:
  - name: Run installation tasks
    include: tasks/install.yml

  - name: Run configuration tasks
    include: tasks/configure.yml
```

```yaml
# tasks/install.yml
- name: Check if already installed
  shell: command -v myapp
  register: check
  failed_when: false

- name: Install if not present
  shell: ./install.sh
  when: check.rc != 0
```

### Path Resolution

**All paths in presets resolve relative to the file they're written in** (Node.js-style):

```
presets/my-preset/
├── preset.yml
├── tasks/
│   └── configure.yml
└── templates/
    └── config.j2
```

From `tasks/configure.yml`, reference the template:

```yaml
# tasks/configure.yml
- name: Render config
  template:
    src: ../templates/config.j2  # Relative to tasks/ directory
    dest: /etc/myapp/config
```

From `preset.yml`, reference the template directly:

```yaml
# preset.yml
- name: Render config
  template:
    src: templates/config.j2  # Relative to preset.yml
    dest: /etc/myapp/config
```

**Key principle**: Paths are always relative to the YAML file containing them, not the preset root.

### Nested Includes

Includes can include other files (but avoid deep nesting):

```yaml
# preset.yml
steps:
  - include: tasks/setup.yml

# tasks/setup.yml
- include: common/dependencies.yml
- include: common/permissions.yml
```

### Include Conditions

Apply conditions to entire include blocks:

```yaml
steps:
  - name: Linux setup
    include: tasks/linux.yml
    when: os == "linux"

  - name: macOS setup
    include: tasks/macos.yml
    when: os == "darwin"
```

## Steps

### Using Built-in Actions

Presets can use any mooncake action **except other presets** (no nesting):

```yaml
steps:
  # Shell commands
  - name: Run script
    shell: ./install.sh
    become: true

  # File operations
  - name: Create config
    file:
      path: /etc/myapp/config.yml
      state: file
      content: |
        port: {{ parameters.port }}

  # Template rendering
  - name: Render template
    template:
      src: ./templates/config.j2
      dest: /etc/myapp/config
      vars:
        port: "{{ parameters.port }}"

  # Service management
  - name: Start service
    service:
      name: myapp
      state: started
      enabled: true
```

### Conditionals

Use `when` to execute steps conditionally:

```yaml
steps:
  - name: Install on Ubuntu
    shell: apt-get install -y myapp
    when: os == "linux" and apt_available
    become: true

  - name: Install on macOS
    shell: brew install myapp
    when: os == "darwin" and brew_available

  - name: Configure if parameter set
    file:
      path: /etc/myapp/config
      state: file
    when: parameters.configure == true
```

### Variables and Facts

Presets have access to:

**Parameters** (via `parameters` namespace):
```yaml
{{ parameters.my_param }}
```

**Variables** (playbook-level):
```yaml
{{ my_variable }}
```

**Facts** (system information):
```yaml
{{ os }}
{{ arch }}
{{ hostname }}
```

**Step Results** (via `register`):
```yaml
steps:
  - name: Check something
    shell: which myapp
    register: check_result
    failed_when: false

  - name: Use result
    shell: echo "Found at {{ check_result.stdout }}"
    when: check_result.rc == 0
```

## Platform Handling

### Detect Package Managers

Use facts to detect available package managers:

```yaml
steps:
  - name: Install via apt
    shell: apt-get install -y {{ parameters.package }}
    when: apt_available
    become: true

  - name: Install via dnf
    shell: dnf install -y {{ parameters.package }}
    when: dnf_available
    become: true

  - name: Install via brew
    shell: brew install {{ parameters.package }}
    when: brew_available
```

Available package manager facts:
- `apt_available` (Debian/Ubuntu)
- `dnf_available` (Fedora/RHEL 8+)
- `yum_available` (RHEL/CentOS 7)
- `pacman_available` (Arch)
- `zypper_available` (openSUSE)
- `apk_available` (Alpine)
- `brew_available` (macOS/Linux)

### Operating System Detection

```yaml
steps:
  - name: Linux-specific step
    shell: systemctl start myapp
    when: os == "linux"

  - name: macOS-specific step
    shell: launchctl load ~/Library/LaunchAgents/myapp.plist
    when: os == "darwin"
```

## Service Configuration

### systemd (Linux)

```yaml
steps:
  - name: Configure systemd service
    service:
      name: myapp
      state: started
      enabled: true
      daemon_reload: true
      dropin:
        name: 10-preset.conf
        content: |
          [Service]
          {% if parameters.host %}
          Environment="HOST={{ parameters.host }}"
          {% endif %}
          {% if parameters.port %}
          Environment="PORT={{ parameters.port }}"
          {% endif %}
    become: true
    when: os == "linux"
```

### launchd (macOS)

```yaml
steps:
  - name: Configure launchd service
    service:
      name: com.example.myapp
      state: started
      enabled: true
      unit:
        content: |
          <?xml version="1.0" encoding="UTF-8"?>
          <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
          <plist version="1.0">
          <dict>
            <key>Label</key>
            <string>com.example.myapp</string>
            <key>ProgramArguments</key>
            <array>
              <string>/usr/local/bin/myapp</string>
            </array>
            {% if parameters.host or parameters.port %}
            <key>EnvironmentVariables</key>
            <dict>
              {% if parameters.host %}
              <key>HOST</key>
              <string>{{ parameters.host }}</string>
              {% endif %}
              {% if parameters.port %}
              <key>PORT</key>
              <string>{{ parameters.port }}</string>
              {% endif %}
            </dict>
            {% endif %}
            <key>RunAtLoad</key>
            <true/>
            <key>KeepAlive</key>
            <true/>
          </dict>
          </plist>
    when: os == "darwin"
```

## Error Handling

### Validation

Validate parameters at the start:

```yaml
steps:
  - name: Validate port range
    shell: test {{ parameters.port }} -ge 1024 && test {{ parameters.port }} -le 65535
    when: parameters.port is defined

  - name: Validate required files
    shell: test -f {{ parameters.config_file }}
    when: parameters.config_file is defined
```

### Idempotency

Make steps idempotent:

```yaml
steps:
  # Check before installing
  - name: Check if already installed
    shell: command -v myapp
    register: check
    failed_when: false

  - name: Install only if not present
    shell: ./install.sh
    when: check.rc != 0

  # Use 'creates' for idempotency
  - name: Download archive
    shell: curl -L -o /tmp/myapp.tar.gz {{ parameters.url }}
    creates: /tmp/myapp.tar.gz
```

### Failed When

Control when steps should fail:

```yaml
steps:
  - name: Try package manager install
    shell: apt-get install -y myapp
    register: apt_install
    failed_when: false

  - name: Fallback to script install
    shell: curl -fsSL https://get.myapp.com | sh
    when: apt_install.rc != 0
```

## Complete Example: Custom Application Preset

```yaml
preset:
  name: deploy-webapp
  description: Deploy a web application with service management
  version: 1.0.0

  parameters:
    app_name:
      type: string
      required: true
      description: Application name

    version:
      type: string
      required: true
      description: Version to deploy (e.g., v1.2.3)

    port:
      type: number
      default: 8080
      description: Application port

    environment:
      type: string
      default: production
      enum: [development, staging, production]
      description: Deployment environment

    enable_service:
      type: bool
      default: true
      description: Configure and start systemd/launchd service

  steps:
    # Step 1: Create application directory
    - name: Create app directory
      file:
        path: "/opt/{{ parameters.app_name }}"
        state: directory
        mode: "0755"
      become: true

    # Step 2: Download application binary
    - name: Download application
      shell: |
        curl -L -o /opt/{{ parameters.app_name }}/app \
          https://releases.example.com/{{ parameters.app_name }}/{{ parameters.version }}/app
        chmod +x /opt/{{ parameters.app_name }}/app
      become: true
      creates: "/opt/{{ parameters.app_name }}/app"

    # Step 3: Create configuration file
    - name: Create config file
      file:
        path: "/etc/{{ parameters.app_name }}/config.yml"
        state: file
        mode: "0644"
        content: |
          app_name: {{ parameters.app_name }}
          version: {{ parameters.version }}
          port: {{ parameters.port }}
          environment: {{ parameters.environment }}
      become: true

    # Step 4: Configure systemd service (Linux)
    - name: Configure systemd service
      service:
        name: "{{ parameters.app_name }}"
        state: started
        enabled: true
        unit:
          content: |
            [Unit]
            Description={{ parameters.app_name }} service
            After=network.target

            [Service]
            Type=simple
            User=www-data
            WorkingDirectory=/opt/{{ parameters.app_name }}
            ExecStart=/opt/{{ parameters.app_name }}/app
            Restart=always
            RestartSec=10
            Environment="PORT={{ parameters.port }}"
            Environment="ENV={{ parameters.environment }}"

            [Install]
            WantedBy=multi-user.target
      become: true
      when: parameters.enable_service and os == "linux"

    # Step 5: Configure launchd service (macOS)
    - name: Configure launchd service
      service:
        name: "com.example.{{ parameters.app_name }}"
        state: started
        enabled: true
        unit:
          content: |
            <?xml version="1.0" encoding="UTF-8"?>
            <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
            <plist version="1.0">
            <dict>
              <key>Label</key>
              <string>com.example.{{ parameters.app_name }}</string>
              <key>ProgramArguments</key>
              <array>
                <string>/opt/{{ parameters.app_name }}/app</string>
              </array>
              <key>WorkingDirectory</key>
              <string>/opt/{{ parameters.app_name }}</string>
              <key>EnvironmentVariables</key>
              <dict>
                <key>PORT</key>
                <string>{{ parameters.port }}</string>
                <key>ENV</key>
                <string>{{ parameters.environment }}</string>
              </dict>
              <key>RunAtLoad</key>
              <true/>
              <key>KeepAlive</key>
              <true/>
              <key>StandardOutPath</key>
              <string>/var/log/{{ parameters.app_name }}.log</string>
              <key>StandardErrorPath</key>
              <string>/var/log/{{ parameters.app_name }}-error.log</string>
            </dict>
            </plist>
      become: true
      when: parameters.enable_service and os == "darwin"

    # Step 6: Wait for service to be ready
    - name: Wait for service
      assert:
        http:
          url: "http://localhost:{{ parameters.port }}/health"
          status: 200
          timeout: "5s"
      retries: 10
      retry_delay: "3s"
      when: parameters.enable_service
```

Usage:
```yaml
- name: Deploy my web app
  preset: deploy-webapp
  with:
    app_name: mywebapp
    version: v1.2.3
    port: 8080
    environment: production
    enable_service: true
  become: true
  register: deploy_result
```

## Best Practices

### 1. Single Responsibility

Each preset should do one thing well:

**Good**: `install-postgres`, `configure-postgres`, `backup-postgres`

**Avoid**: `setup-everything` (monolithic preset)

### 2. Sensible Defaults

Choose defaults that work for 80% of users:

```yaml
parameters:
  port:
    type: number
    default: 8080  # Common default

  enabled:
    type: bool
    default: true  # Most users want this enabled
```

### 3. Clear Documentation

Document every parameter:

```yaml
parameters:
  timeout:
    type: number
    default: 30
    description: Connection timeout in seconds (1-300)
```

### 4. Platform Detection

Use facts, don't hardcode:

```yaml
# Good
when: apt_available

# Bad
when: os == "linux"  # Not all Linux distros have apt
```

### 5. Fail Fast

Validate inputs early:

```yaml
steps:
  - name: Validate version format
    shell: echo "{{ parameters.version }}" | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$'
```

### 6. Idempotent Operations

Every step should be safe to run multiple times:

```yaml
- name: Create directory (idempotent)
  file:
    path: /opt/myapp
    state: directory

- name: Download if not exists (idempotent)
  shell: curl -o /tmp/file https://example.com/file
  creates: /tmp/file
```

### 7. Version Your Presets

Use semantic versioning:

```yaml
preset:
  version: 1.2.3  # Breaking.Feature.Fix
```

## Testing Presets

### Dry Run

Always test with `--dry-run` first:

```bash
mooncake run -c test-preset.yml --dry-run
```

### Multiple Platforms

Test on different operating systems:

```yaml
# test-preset.yml
- name: Test on current OS
  preset: my-preset
  with:
    state: present
```

### Parameter Validation

Test with missing/invalid parameters:

```yaml
# Should fail
- preset: my-preset
  # Missing required parameter

# Should fail
- preset: my-preset
  with:
    invalid_param: value
```

## Distribution

### Local Presets

Place in playbook directory:

```
my-project/
├── playbook.yml
└── presets/
    └── custom.yml
```

### User Presets

Install to user directory:

```bash
mkdir -p ~/.mooncake/presets
cp my-preset.yml ~/.mooncake/presets/
```

### System Presets

Install system-wide:

```bash
sudo mkdir -p /usr/share/mooncake/presets
sudo cp my-preset.yml /usr/share/mooncake/presets/
```

### Sharing

Share presets via:
- Git repositories
- Package managers
- Direct file distribution

## Limitations

Current architectural constraints:

1. **No Nesting**: Presets cannot call other presets (architectural decision for simplicity)
2. **Sequential Execution**: Steps execute in order, not parallel (may be relaxed in future)
3. **Parameter Types**: Only string, bool, array, object types supported

**Note**: Preset steps fully support includes, loops (with_items, with_filetree), and conditionals (when). The preset definition file itself must be static YAML, but the steps within can be dynamically expanded.

## Next Steps

- [Using Presets Guide](presets.md)
- [Built-in Presets](#) <!-- TODO -->
- [Community Presets](#) <!-- TODO -->


---

<!-- FILE: guide/presets.md -->

# Using Presets

Presets are reusable, parameterized collections of steps that can be invoked as a single action. They provide a way to encapsulate complex workflows into simple, declarative configurations.

## What is a Preset?

A preset is essentially a YAML file that defines:
- **Parameters**: Configurable inputs with types, defaults, and validation
- **Steps**: A sequence of mooncake steps to execute
- **Metadata**: Name, description, and version information

Think of presets as functions or modules - they take parameters and execute a predefined sequence of operations.

## Why Use Presets?

**Benefits:**
- **Reusability**: Write once, use everywhere
- **Maintainability**: Update logic in one place
- **Discoverability**: Share presets as files, no code changes needed
- **Simplicity**: Complex workflows become single-line declarations
- **Type Safety**: Parameter validation catches errors early

**Example**: Instead of writing 20+ steps to install Ollama, configure the service, and pull models, you can write:

```yaml
- preset: ollama
  with:
    state: present
    service: true
    pull: [llama3.1:8b]
```

## Basic Usage

### Simple Invocation

```yaml
- name: Install Ollama
  preset: ollama
  with:
    state: present
```

### With Parameters

```yaml
- name: Install Ollama with full configuration
  preset: ollama
  with:
    state: present
    service: true
    method: auto
    host: "0.0.0.0:11434"
    models_dir: "/data/ollama"
    pull:
      - "llama3.1:8b"
      - "mistral:latest"
    force: false
  become: true
  register: ollama_result
```

### String Shorthand

For presets without parameters:

```yaml
- name: Quick preset invocation
  preset: my-preset
```

Is equivalent to:

```yaml
- name: Quick preset invocation
  preset:
    name: my-preset
```

## Parameters

### Accessing Parameters in Presets

When a preset is executed, its parameters are available in the `parameters` namespace:

```yaml
# In preset definition
- name: Show parameter value
  shell: echo "State is {{ parameters.state }}"
```

This namespacing prevents collisions with variables and facts.

### Parameter Types

Presets support four parameter types:

| Type | Description | Example |
|------|-------------|---------|
| `string` | Text value | `"present"`, `"localhost:11434"` |
| `bool` | Boolean | `true`, `false` |
| `array` | List of values | `["item1", "item2"]` |
| `object` | Key-value map | `{key: "value"}` |

### Default Values

Parameters can have defaults:

```yaml
# Preset definition
parameters:
  service:
    type: bool
    default: true
    description: Enable service
```

```yaml
# User playbook (uses default service: true)
- preset: ollama
  with:
    state: present
```

### Required Parameters

Mark critical parameters as required:

```yaml
# Preset definition
parameters:
  state:
    type: string
    required: true
    enum: [present, absent]
```

```yaml
# User playbook - fails without 'state'
- preset: ollama  # ERROR: required parameter 'state' not provided
```

### Enum Constraints

Restrict parameters to specific values:

```yaml
# Preset definition
parameters:
  method:
    type: string
    enum: [auto, script, package]
```

```yaml
# User playbook - fails with invalid value
- preset: ollama
  with:
    method: invalid  # ERROR: invalid value, allowed: [auto, script, package]
```

## Preset Discovery

Mooncake searches for presets in this order (highest priority first):

1. `./presets/` - Playbook directory
2. `~/.mooncake/presets/` - User presets
3. `/usr/local/share/mooncake/presets/` - Local installation
4. `/usr/share/mooncake/presets/` - System installation

### Preset File Formats

Presets can use two formats:

**Flat format** (simple presets):
```
presets/
└── mypreset.yml
```

**Directory format** (complex presets with includes):
```
presets/
└── mypreset/
    ├── preset.yml       # Main definition
    ├── tasks/           # Modular task files
    │   ├── install.yml
    │   └── configure.yml
    └── templates/       # Configuration templates
        └── config.j2
```

When both exist, the directory format takes precedence:
- `presets/ollama/preset.yml` is loaded before `presets/ollama.yml`

### Example Directory Structure

```
my-project/
├── playbook.yml
└── presets/
    ├── ollama/          # Directory-based preset
    │   ├── preset.yml
    │   └── tasks/
    │       └── install.yml
    └── myapp.yml        # Flat preset

~/.mooncake/
└── presets/
    └── common.yml       # User-wide preset

/usr/share/mooncake/presets/
└── ollama/              # Built-in directory preset
    ├── preset.yml
    ├── tasks/
    ├── templates/
    └── README.md
```

## Result Registration

Presets support result registration at the preset level:

```yaml
- name: Install Ollama
  preset: ollama
  with:
    state: present
  register: ollama_result

- name: Check if changed
  shell: echo "Changed = {{ ollama_result.changed }}"
```

**Preset results contain:**
- `changed`: `true` if any step changed
- `stdout`: Summary message
- `rc`: Always 0 (success) or error
- `failed`: `false` on success

## Conditionals and Loops

Presets work with all standard step features:

### When Conditions

```yaml
- name: Install Ollama on Linux only
  preset: ollama
  with:
    state: present
  when: os == "linux"
```

### Tags

```yaml
- name: Install Ollama
  preset: ollama
  with:
    state: present
  tags: [setup, llm]
```

### Loops

```yaml
- name: Setup multiple LLM backends
  preset: ollama
  with:
    state: present
    pull: ["{{ item }}"]
  with_items: "{{ llm_models }}"
```

## Error Handling

### Preset Errors

Presets can fail at two levels:

1. **Parameter validation**: Before execution
   ```
   Error: preset 'ollama' parameter validation failed:
   required parameter 'state' not provided
   ```

2. **Step execution**: During execution
   ```
   Error: preset 'ollama' step 3 failed:
   installation via package manager failed
   ```

### Failed When

```yaml
- name: Try installing Ollama
  preset: ollama
  with:
    state: present
  register: ollama_result
  failed_when: false

- name: Handle failure
  shell: echo "Installation failed"
  when: ollama_result.failed
```

## Dry Run Mode

Presets fully support dry-run mode:

```bash
mooncake run -c playbook.yml --dry-run
```

Output:
```
  [DRY-RUN] Would expand preset 'ollama' with 3 parameters
▶ Install Ollama
✓ Install Ollama
```

In dry-run mode, presets:
- Show parameter count
- Don't execute steps (but may expand them for display)
- Return `changed: true` (pessimistic assumption)

## Best Practices

### 1. Use Presets for Complex Workflows

**Good** (preset hides complexity):
```yaml
- preset: ollama
  with:
    state: present
    service: true
```

**Avoid** (simple operations don't need presets):
```yaml
- preset: echo-hello  # Just use: shell: echo "hello"
```

### 2. Provide Sensible Defaults

```yaml
# Good: Service enabled by default (most common use case)
parameters:
  service:
    type: bool
    default: true
```

### 3. Use Descriptive Names

```yaml
# Good
- preset: ollama

# Bad
- preset: install-llm  # Too generic
- preset: ollama-installer-and-service-configurator  # Too verbose
```

### 4. Document Parameters

```yaml
parameters:
  host:
    type: string
    description: Ollama server bind address (e.g., 'localhost:11434', '0.0.0.0:11434')
```

### 5. Handle Platform Differences

Use `when` conditions in preset steps:

```yaml
# In preset definition
- name: Install via apt (Linux)
  shell: apt-get install -y ollama
  when: apt_available and os == "linux"

- name: Install via brew (macOS)
  shell: brew install ollama
  when: brew_available and os == "darwin"
```

## Available Presets

Mooncake includes several built-in presets for common development tools and workflows.

### Development Tools

#### modern-unix - Modern CLI Tools

Install modern replacements for classic Unix commands.

```yaml
- name: Install modern Unix tools
  preset: modern-unix
```

**What's included**: bat (cat), ripgrep (grep), fd (find), exa (ls), zoxide (cd), dust (du), duf (df), bottom (top)

**Parameters**:

- `tools` (array): List of tools to install (default: all)
- `state` (string): "present" or "absent"

**Platform support**: Linux (apt, dnf, yum, pacman, zypper), macOS (brew)

[Full documentation →](../../presets/modern-unix/)

---

#### nodejs - Node.js via nvm

Install Node.js using nvm (Node Version Manager) for easy version management.

```yaml
- name: Install Node.js LTS with tools
  preset: nodejs
  with:
    version: lts
    global_packages:
      - typescript
      - eslint
      - prettier
```

**Parameters**:

- `version` (string): Node version ("lts", "latest", "20.10.0")
- `set_default` (bool): Set as default version (default: true)
- `additional_versions` (array): Other versions to install
- `global_packages` (array): npm packages to install globally

**Platform support**: Linux, macOS

[Full documentation →](../../presets/nodejs/)

---

#### rust - Rust via rustup

Install Rust programming language using rustup.

```yaml
- name: Install Rust with dev tools
  preset: rust
  with:
    toolchain: stable
    components:
      - clippy
      - rustfmt
      - rust-analyzer
    targets:
      - wasm32-unknown-unknown
```

**Parameters**:

- `toolchain` (string): "stable", "beta", "nightly", or version (default: stable)
- `profile` (string): "minimal", "default", "complete" (default: default)
- `components` (array): Additional components (clippy, rustfmt, rust-analyzer, rust-src)
- `targets` (array): Compilation targets (wasm32, cross-compile)

**Platform support**: Linux, macOS, Windows

[Full documentation →](../../presets/rust/)

---

#### python - Python via pyenv

Install Python using pyenv for version management.

```yaml
- name: Install Python 3.12
  preset: python
  with:
    version: "3.12.1"
    install_virtualenv: true
```

**Parameters**:

- `version` (string): Python version to install (default: "3.12.1")
- `set_global` (bool): Set as global Python version (default: true)
- `additional_versions` (array): Other versions to install
- `install_virtualenv` (bool): Install pyenv-virtualenv plugin (default: true)

**Platform support**: Linux (with build dependencies), macOS

[Full documentation →](../../presets/python/)

---

### Productivity Tools

#### tmux - Terminal Multiplexer

Install and configure tmux with sensible defaults.

```yaml
- name: Install tmux with custom config
  preset: tmux
  with:
    prefix_key: "C-a"
    mouse_mode: true
    vi_mode: true
```

**Parameters**:

- `configure` (bool): Install configuration file (default: true)
- `prefix_key` (string): Tmux prefix key (default: "C-a")
- `mouse_mode` (bool): Enable mouse support (default: true)
- `vi_mode` (bool): Use vi key bindings (default: true)
- `config_path` (string): Path to config file (default: "~/.tmux.conf")

**Platform support**: Linux, macOS

[Full documentation →](../../presets/tmux/)

---

### AI/ML Tools

#### ollama - Ollama LLM Runtime

Install and configure Ollama for running large language models locally.

```yaml
- name: Install Ollama with models
  preset: ollama
  with:
    state: present
    service: true
    pull:
      - llama3.1:8b
      - mistral:latest
  become: true
```

**Parameters**:

- `state` (string): "present" or "absent" (default: present)
- `service` (bool): Configure as system service (default: false)
- `method` (string): Installation method - "auto", "package", "script" (default: auto)
- `pull` (array): Models to download
- `force` (bool): Force model re-download (default: false)
- `host` (string): Ollama server host (default: "localhost:11434")
- `models_dir` (string): Models storage directory

**Platform support**: Linux (systemd), macOS (launchd)

[Full documentation →](../../presets/ollama/)

---

## Common Patterns

### Configuration Template

```yaml
- name: Deploy app with generated config
  preset: myapp
  with:
    version: "1.2.3"
    config:
      database_url: "{{ db_url }}"
      cache_enabled: true
```

### Multi-Stage Deployment

```yaml
- name: Stage 1 - Dependencies
  preset: install-deps

- name: Stage 2 - Application
  preset: deploy-app
  with:
    environment: production

- name: Stage 3 - Healthcheck
  preset: verify-deployment
```

### Conditional Installation

```yaml
- name: Check if already installed
  shell: which ollama
  register: check
  failed_when: false

- name: Install if not present
  preset: ollama
  with:
    state: present
  when: check.rc != 0
```

## Limitations

Current architectural constraints:

1. **No nesting**: Presets cannot call other presets (prevents circular dependencies)
2. **Flat parameters**: Parameter definitions are not nested (use object type for structured data)
3. **No output schemas**: Presets return aggregate results, not structured outputs
4. **Sequential execution**: Steps run in order, not parallel

**Note**: Preset steps support all mooncake features - includes, loops (with_items, with_filetree), conditionals (when), and templates. These limitations are intentional design choices for simplicity and predictability.

## Troubleshooting

### Preset Not Found

```
Error: preset 'mypreset' not found in search paths:
[./presets, ~/.mooncake/presets, /usr/share/mooncake/presets]
```

**Solution**: Check preset filename matches (`mypreset.yml`) and is in a search path.

### Parameter Type Mismatch

```
Error: parameter 'service' must be a boolean, got string
```

**Solution**: Check parameter types in your invocation:
```yaml
with:
  service: true  # Not "true"
```

### Unknown Parameter

```
Error: unknown parameter 'services' (preset 'ollama' does not define this parameter)
```

**Solution**: Check parameter spelling in preset definition.

## Next Steps

- [Create your own presets](preset-authoring.md)
- [View available presets](#available-presets)
- [Examples directory](../../examples/)


---

<!-- FILE: guide/quick-reference.md -->

# Quick Reference

A one-page cheat sheet for common Mooncake operations.

---

## Installation & Setup

```bash
# Install
go install github.com/alehatsman/mooncake@latest

# Verify
mooncake --version

# Get help
mooncake --help
mooncake run --help
```

---

## Basic Commands

```bash
# Run configuration
mooncake run --config config.yml

# Preview changes (dry-run)
mooncake run --config config.yml --dry-run

# Show system facts
mooncake facts
mooncake facts --format json

# Generate execution plan
mooncake plan --config config.yml
mooncake plan --config config.yml --format json --output plan.json

# Execute from plan
mooncake run --from-plan plan.json

# Filter by tags
mooncake run --config config.yml --tags dev,test

# With sudo password
mooncake run --config config.yml --ask-become-pass
mooncake run --config config.yml -K  # shorthand

# Disable TUI (for CI/CD)
mooncake run --config config.yml --raw

# JSON output
mooncake run --config config.yml --raw --output-format json

# Debug mode
mooncake run --config config.yml --log-level debug
```

---

## Presets

```bash
# List all available presets
mooncake presets list

# Install preset interactively
mooncake presets -K

# Install specific preset
mooncake presets install docker
mooncake presets install -K postgres  # with sudo

# Show preset status
mooncake presets status
mooncake presets status docker

# Uninstall preset
mooncake presets uninstall docker
```

---

## Configuration Structure

```yaml
# Basic step
- name: Step description
  action_name:
    parameter: value

# With variables
- vars:
    my_var: value

# With conditionals
- name: Only on Linux
  shell: echo "Linux!"
  when: os == "linux"

# With loops
- name: Install packages
  shell: apt install {{item}}
  with_items: [git, vim, curl]
  become: true

# With tags
- name: Dev setup
  shell: install-dev.sh
  tags: [dev, setup]
```

---

## Common Actions

### Shell Command
```yaml
- name: Run command
  shell: echo "Hello {{os}}"

- name: Multi-line script
  shell: |
    apt update
    apt install -y neovim
  become: true
  timeout: 5m
```

### File Operations
```yaml
# Create file
- name: Create config file
  file:
    path: ~/.config/app.conf
    state: file
    content: "key=value"
    mode: "0644"

# Create directory
- name: Create directory
  file:
    path: ~/.local/bin
    state: directory
    mode: "0755"

# Create symlink
- name: Create link
  file:
    path: ~/bin/myapp
    state: link
    target: /usr/local/bin/myapp

# Remove file
- name: Remove file
  file:
    path: /tmp/old-file
    state: absent
```

### Template Rendering
```yaml
- name: Render nginx config
  template:
    src: ./nginx.conf.j2
    dest: /etc/nginx/nginx.conf
    mode: "0644"
    vars:
      port: 8080
      workers: 4
```

### Copy Files
```yaml
- name: Copy with backup
  copy:
    src: ./app.conf
    dest: /etc/app.conf
    mode: "0644"
    backup: true
```

### Download Files
```yaml
- name: Download file
  download:
    url: https://example.com/file.tar.gz
    dest: /tmp/file.tar.gz
    checksum: sha256:abc123...
    timeout: 10m
    retries: 3
```

### Extract Archives
```yaml
- name: Extract tarball
  unarchive:
    src: /tmp/archive.tar.gz
    dest: /opt/app
    strip_components: 1
```

### Service Management
```yaml
- name: Start and enable service
  service:
    name: nginx
    state: started
    enabled: true
  become: true
```

### Assertions
```yaml
# Verify command
- name: Check Docker installed
  assert:
    command:
      cmd: docker --version
      exit_code: 0

# Verify file
- name: Check file exists
  assert:
    file:
      path: /etc/nginx/nginx.conf
      exists: true
      mode: "0644"

# Verify HTTP
- name: Check API health
  assert:
    http:
      url: https://api.example.com/health
      status: 200
```

### Presets
```yaml
- name: Install Ollama
  preset: ollama
  with:
    state: present
    service: true
    pull: [llama3.1:8b]
  become: true
```

---

## Variables & Facts

### Define Variables
```yaml
- vars:
    app_name: MyApp
    version: "1.0.0"
    packages:
      - git
      - vim
      - curl

- name: Use variable
  shell: echo "Installing {{app_name}} v{{version}}"
```

### Auto-Detected Facts
```yaml
# Available system facts
{{os}}                    # darwin, linux, windows
{{arch}}                  # amd64, arm64
{{distribution}}          # ubuntu, fedora, arch, macos
{{package_manager}}       # apt, dnf, yum, brew, choco
{{cpu_cores}}             # Number of CPU cores
{{memory_total_mb}}       # Total RAM in MB
{{hostname}}              # System hostname
{{kernel_version}}        # Kernel version
{{python_version}}        # Python version (if installed)
{{docker_version}}        # Docker version (if installed)
{{git_version}}           # Git version (if installed)
```

---

## Control Flow

### Conditionals
```yaml
# Simple condition
- name: Linux only
  shell: apt update
  when: os == "linux"

# Multiple conditions (AND)
- name: Ubuntu with apt
  shell: apt install vim
  when: os == "linux" && package_manager == "apt"

# OR condition
- name: macOS or Linux
  shell: echo "Unix system"
  when: os == "darwin" || os == "linux"

# Negation
- name: Not Windows
  shell: echo "Not Windows"
  when: os != "windows"

# Check variable
- name: If defined
  shell: echo "{{my_var}}"
  when: my_var is defined
```

### Operators
- `==` Equal
- `!=` Not equal
- `>` Greater than
- `<` Less than
- `>=` Greater than or equal
- `<=` Less than or equal
- `&&` AND
- `||` OR
- `!` NOT
- `in` Contains
- `is defined` / `is not defined`

### Loops
```yaml
# Loop over list
- name: Install package
  shell: brew install {{item}}
  with_items:
    - neovim
    - ripgrep
    - fzf

# Loop over files
- name: Deploy dotfile
  copy:
    src: "{{item.src}}"
    dest: "~/{{item.name}}"
  with_filetree: ./dotfiles
  when: item.is_dir == false
```

### Tags
```yaml
# Tag steps
- name: Dev setup
  shell: setup-dev.sh
  tags: [dev, setup]

- name: Production deploy
  shell: deploy-prod.sh
  tags: [prod, deploy]
```

Run with tags:
```bash
mooncake run --config config.yml --tags dev
mooncake run --config config.yml --tags dev,test  # OR logic
```

---

## Execution Control

### Timeout & Retry
```yaml
- name: Download with retry
  shell: curl -O https://example.com/file.tar.gz
  timeout: 10m
  retries: 3
  retry_delay: 30s
```

### Changed/Failed Conditions
```yaml
- name: Custom changed detection
  shell: make install
  register: result
  changed_when: "'installed' in result.stdout"

- name: Custom failure detection
  shell: curl https://example.com
  register: result
  failed_when: result.rc != 0 and result.rc != 18
```

### Result Registration
```yaml
- name: Check status
  shell: systemctl is-active nginx
  register: nginx_status
  ignore_errors: true

- name: Restart if not running
  service:
    name: nginx
    state: restarted
  when: nginx_status.rc != 0
  become: true
```

---

## Sudo Operations

```yaml
# Inline sudo
- name: Install package
  shell: apt install neovim
  become: true

# Prompt for password
$ mooncake run --config config.yml --ask-become-pass
$ mooncake run --config config.yml -K  # shorthand

# Password from file
$ mooncake run --config config.yml --sudo-pass-file ~/.mooncake/sudo_pass

# Environment variable
$ export SUDO_ASKPASS=/usr/bin/ssh-askpass
$ mooncake run --config config.yml
```

---

## Template Syntax

```yaml
# Variables
{{ variable_name }}

# Filters
{{ "/tmp/file" | expanduser }}       # ~/file
{{ "hello" | upper }}                # HELLO
{{ "/tmp/file.tar.gz" | basename }}  # file.tar.gz

# Conditionals
{% if os == "darwin" %}
macOS specific
{% elif os == "linux" %}
Linux specific
{% else %}
Other OS
{% endif %}

# Loops
{% for item in packages %}
- {{ item }}
{% endfor %}
```

---

## Common Patterns

### Multi-OS Configuration
```yaml
- name: Install on macOS
  shell: brew install neovim
  when: os == "darwin"

- name: Install on Linux (apt)
  shell: apt install -y neovim
  become: true
  when: os == "linux" && package_manager == "apt"

- name: Install on Linux (dnf)
  shell: dnf install -y neovim
  become: true
  when: os == "linux" && package_manager == "dnf"
```

### Idempotent File Creation
```yaml
- name: Create config directory
  file:
    path: ~/.config/myapp
    state: directory

- name: Create config file
  file:
    path: ~/.config/myapp/config.yml
    state: file
    content: |
      setting: value
    creates: ~/.config/myapp/config.yml  # Only if doesn't exist
```

### Backup Before Modify
```yaml
- name: Update config
  copy:
    src: ./new-config.yml
    dest: ~/.config/app/config.yml
    backup: true  # Creates timestamped backup
```

### Download, Extract, Install
```yaml
- name: Download tarball
  download:
    url: https://example.com/app.tar.gz
    dest: /tmp/app.tar.gz
    checksum: sha256:abc123...

- name: Extract
  unarchive:
    src: /tmp/app.tar.gz
    dest: /opt/app
    creates: /opt/app/bin/app

- name: Create symlink
  file:
    path: /usr/local/bin/app
    state: link
    target: /opt/app/bin/app
  become: true
```

---

## Debugging

```bash
# Dry-run (preview changes)
mooncake run --config config.yml --dry-run

# Debug logging
mooncake run --config config.yml --log-level debug

# Show facts
mooncake facts

# Validate without running
mooncake plan --config config.yml

# Check specific step
mooncake run --config config.yml --tags mystep --dry-run
```

---

## Exit Codes

- `0` - Success
- `1` - General error
- `2` - Configuration error
- `3` - Validation error
- `4` - Execution error

---

## See Also

- [Full Documentation](https://mooncake.alehatsman.com)
- [Actions Reference](guide/config/actions.md)
- [Complete Reference](guide/config/reference.md)
- [Examples](../examples/)
- [Troubleshooting](guide/troubleshooting.md)
- [FAQ](faq.md)


---

<!-- FILE: guide/troubleshooting.md -->

# Troubleshooting Guide

Common issues and their solutions when working with Mooncake.

---

## Installation Issues

### "command not found: mooncake"

**Problem**: After installing with `go install`, the `mooncake` command is not found.

**Solution**: Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is in your PATH:

```bash
# Add to ~/.bashrc, ~/.zshrc, or equivalent
export PATH="$PATH:$HOME/go/bin"

# Or find your GOPATH
go env GOPATH

# Verify
which mooncake
```

### Permission denied when installing

**Problem**: `go install` fails with permission errors.

**Solution**: Don't use `sudo` with `go install`. Install to your user directory:

```bash
# Correct way
go install github.com/alehatsman/mooncake@latest

# Wrong way (don't do this)
sudo go install github.com/alehatsman/mooncake@latest
```

---

## Configuration Errors

### "invalid configuration: unknown field"

**Problem**: YAML contains a typo or invalid field name.

```
Error: invalid configuration: unknown field 'shel' in step 1
```

**Solution**: Check spelling of action names and fields. Common typos:
- `shel` → `shell`
- `comand` → `command`
- `templete` → `template`

Use schema validation to catch these early:
```bash
mooncake plan --config config.yml  # Validates without running
```

### "yaml: unmarshal errors"

**Problem**: Invalid YAML syntax.

**Solution**: Check YAML formatting:
- Proper indentation (use spaces, not tabs)
- Quoted strings containing special characters
- Proper list syntax

```yaml
# Wrong
- name: Test
shell: echo "hello"

# Right
- name: Test
  shell: echo "hello"

# Wrong - mixed indentation
- name: Test
  shell: |
    echo "line 1"
      echo "line 2"  # Too much indent

# Right
- name: Test
  shell: |
    echo "line 1"
    echo "line 2"
```

Use a YAML validator:
```bash
# Install yamllint
pip install yamllint

# Validate
yamllint config.yml
```

### "failed to expand template"

**Problem**: Template variable is undefined or template syntax is invalid.

```
Error: failed to expand template: variable 'my_var' is not defined
```

**Solution**:
1. Define the variable before using it:
```yaml
- vars:
    my_var: value

- name: Use variable
  shell: echo "{{my_var}}"
```

2. Use conditional to check if variable exists:
```yaml
- name: Optional variable
  shell: echo "{{my_var}}"
  when: my_var is defined
```

3. Check template syntax:
```yaml
# Wrong
- shell: "{{ variable }"  # Missing closing brace

# Right
- shell: "{{ variable }}"
```

---

## Execution Errors

### "permission denied"

**Problem**: Trying to access a file or directory without sufficient permissions.

**Solution**: Use `become: true` for operations requiring root:

```yaml
- name: Install system package
  shell: apt install neovim
  become: true
```

Then run with sudo password:
```bash
mooncake run --config config.yml -K
```

### "sudo: no password provided"

**Problem**: Step requires sudo but no password method was specified.

**Solution**: Provide sudo password using one of these methods:

```bash
# Interactive prompt
mooncake run --config config.yml --ask-become-pass
mooncake run --config config.yml -K  # shorthand

# Password file
echo "your_password" > ~/.mooncake/sudo_pass
chmod 600 ~/.mooncake/sudo_pass
mooncake run --config config.yml --sudo-pass-file ~/.mooncake/sudo_pass

# SSH askpass (for GUI environments)
export SUDO_ASKPASS=/usr/bin/ssh-askpass
mooncake run --config config.yml
```

### "command not found"

**Problem**: Shell command doesn't exist on the system.

**Solution**:
1. Check if command is installed:
```yaml
- name: Check if docker exists
  shell: which docker
  register: docker_check
  ignore_errors: true

- name: Install docker
  preset: docker
  when: docker_check.rc != 0
  become: true
```

2. Use OS-specific commands:
```yaml
- name: Install on macOS
  shell: brew install neovim
  when: os == "darwin"

- name: Install on Linux
  shell: apt install neovim
  when: os == "linux" && package_manager == "apt"
```

### "timeout: command took too long"

**Problem**: Command exceeds default 2-minute timeout.

**Solution**: Increase timeout:

```yaml
- name: Long-running task
  shell: ./build-script.sh
  timeout: 30m  # 30 minutes
```

---

## File Operation Errors

### "file already exists"

**Problem**: Trying to create file/directory that already exists.

**Solution**: This is usually fine - Mooncake operations are idempotent. If you see an error, check the `state` parameter:

```yaml
# Creates or ensures file exists (idempotent)
- name: Ensure file exists
  file:
    path: /tmp/myfile
    state: file

# Creates or ensures directory exists (idempotent)
- name: Ensure directory exists
  file:
    path: /tmp/mydir
    state: directory
```

### "no such file or directory"

**Problem**: Trying to operate on a file that doesn't exist, or parent directory doesn't exist.

**Solution**:
1. Create parent directories first:
```yaml
- name: Create parent directory
  file:
    path: ~/.config/myapp
    state: directory

- name: Create config file
  file:
    path: ~/.config/myapp/config.yml
    state: file
    content: "key: value"
```

2. Use `creates` to make operation conditional:
```yaml
- name: Extract only if doesn't exist
  unarchive:
    src: /tmp/archive.tar.gz
    dest: /opt/app
    creates: /opt/app/bin/app
```

### "checksum mismatch"

**Problem**: Downloaded file checksum doesn't match expected value.

**Solution**:
1. Verify the checksum value is correct
2. Re-download the file (might be corrupted)
3. Check if upstream changed the file

```yaml
- name: Download with correct checksum
  download:
    url: https://example.com/file.tar.gz
    dest: /tmp/file.tar.gz
    checksum: sha256:abc123def456...  # Verify this is correct
    retries: 3  # Retry on failure
```

Get correct checksum:
```bash
# Calculate SHA-256 checksum
sha256sum file.tar.gz
shasum -a 256 file.tar.gz  # macOS
```

---

## Variable & Template Issues

### "undefined variable"

**Problem**: Using a variable that hasn't been defined.

**Solution**:
1. Define variable before use:
```yaml
- vars:
    app_name: MyApp

- shell: echo "{{app_name}}"
```

2. Use system facts (automatically available):
```yaml
- shell: echo "Running on {{os}}/{{arch}}"
# No need to define os and arch
```

3. Check variable scope - variables defined in one config file aren't available in included files unless passed explicitly.

### "template rendering failed"

**Problem**: Invalid Jinja2 template syntax.

**Solution**: Check template syntax:

```yaml
# Wrong - spaces in variable name
{{ my var }}

# Right
{{ my_var }}

# Wrong - missing endif
{% if condition %}
  something

# Right
{% if condition %}
  something
{% endif %}

# Wrong - invalid filter
{{ value | badfilter }}

# Right - use valid filters
{{ path | expanduser }}
{{ text | upper }}
{{ file | basename }}
```

---

## Platform-Specific Issues

### macOS: "operation not permitted"

**Problem**: macOS security restrictions prevent file operations.

**Solution**:
1. Grant Full Disk Access to Terminal:
   - System Settings → Privacy & Security → Full Disk Access
   - Add Terminal.app or iTerm.app

2. Use `become: true` for system modifications

### Linux: "systemd service not found"

**Problem**: Trying to manage a service that doesn't exist.

**Solution**:
1. Verify service name:
```bash
systemctl list-units --type=service | grep myservice
```

2. Create service first, then manage it:
```yaml
- name: Create systemd unit
  service:
    name: myapp
    unit:
      dest: /etc/systemd/system/myapp.service
      content: |
        [Unit]
        Description=My App

        [Service]
        ExecStart=/usr/local/bin/myapp

        [Install]
        WantedBy=multi-user.target
    daemon_reload: true
  become: true

- name: Start service
  service:
    name: myapp
    state: started
    enabled: true
  become: true
```

### Windows: "command not supported"

**Problem**: Some actions work differently on Windows.

**Solution**: Use platform-specific conditionals:

```yaml
- name: Unix command
  shell: ls -la
  when: os != "windows"

- name: Windows command
  shell: dir
  when: os == "windows"
```

---

## Preset Issues

### "preset not found"

**Problem**: Trying to use a preset that doesn't exist.

**Solution**:
1. List available presets:
```bash
mooncake presets list
```

2. Check preset name spelling:
```yaml
# Wrong
- preset: postgress

# Right
- preset: postgres
```

3. Verify preset is installed (if using custom presets)

### "invalid preset parameters"

**Problem**: Preset parameters don't match schema.

**Solution**: Check preset documentation:

```bash
# Show preset details
mooncake presets status docker
```

Use correct parameter names and types:
```yaml
# Wrong - state is string, not boolean
- preset: docker
  with:
    state: true

# Right
- preset: docker
  with:
    state: present
```

### "preset failed during execution"

**Problem**: Preset step failed.

**Solution**:
1. Run with debug logging:
```bash
mooncake run --config config.yml --log-level debug
```

2. Check preset source code:
```bash
# View preset definition
cat ~/.mooncake/presets/docker/preset.yml
```

3. Try manual installation to isolate issue

---

## Performance Issues

### "execution is very slow"

**Problem**: Configuration takes a long time to run.

**Solution**:
1. Use dry-run to identify slow steps:
```bash
mooncake run --config config.yml --dry-run
```

2. Reduce retries and timeouts where not needed:
```yaml
# Instead of this
- shell: echo "hello"
  timeout: 10m
  retries: 5

# Use this
- shell: echo "hello"
  timeout: 10s
```

3. Use tags to run only necessary steps:
```bash
mooncake run --config config.yml --tags quick
```

4. Check for unnecessary loops:
```yaml
# Inefficient - runs apt update 10 times
- shell: apt update && apt install {{item}}
  with_items: [vim, git, curl, ...]
  become: true

# Better - update once, then install
- shell: apt update
  become: true

- shell: apt install -y {{item}}
  with_items: [vim, git, curl, ...]
  become: true
```

---

## Debugging Techniques

### Enable debug logging

```bash
mooncake run --config config.yml --log-level debug
```

### Use dry-run mode

```bash
# See what would happen without making changes
mooncake run --config config.yml --dry-run
```

### Generate execution plan

```bash
# See the execution plan
mooncake plan --config config.yml

# Export as JSON for analysis
mooncake plan --config config.yml --format json --output plan.json
```

### Test individual steps

Use tags to isolate problematic steps:

```yaml
- name: Problematic step
  shell: complex-command.sh
  tags: [debug]
```

```bash
mooncake run --config config.yml --tags debug --dry-run
```

### Check system facts

```bash
# View all detected system information
mooncake facts

# Export as JSON
mooncake facts --format json > facts.json
```

### Register and inspect results

```yaml
- name: Run command
  shell: my-command.sh
  register: result

- name: Show result
  shell: echo "RC={{result.rc}} STDOUT={{result.stdout}}"
```

### Use ignore_errors

```yaml
- name: Optional step
  shell: might-fail.sh
  register: result
  ignore_errors: true

- name: Check if failed
  shell: echo "Previous step failed"
  when: result.rc != 0
```

---

## Getting Help

### Check documentation

- [Quick Reference](../quick-reference.md)
- [Actions Guide](config/actions.md)
- [Complete Reference](config/reference.md)
- [FAQ](../faq.md)

### Validate configuration

```bash
# Validate without executing
mooncake plan --config config.yml
```

### Report bugs

If you've found a bug:

1. Create minimal reproduction:
```yaml
# Simplest config that reproduces the issue
- name: Bug reproduction
  shell: echo "This fails"
```

2. Include system information:
```bash
mooncake facts > system-info.txt
mooncake --version
```

3. Report at [GitHub Issues](https://github.com/alehatsman/mooncake/issues)

---

## Common Patterns

### Safe file operations

```yaml
# Always create parent directories first
- name: Create config directory
  file:
    path: ~/.config/myapp
    state: directory

# Then create files
- name: Create config
  file:
    path: ~/.config/myapp/config.yml
    state: file
    content: "..."
```

### Idempotent commands

```yaml
# Use creates/removes for idempotency
- name: Extract tarball
  shell: tar xzf /tmp/app.tar.gz -C /opt
  args:
    creates: /opt/app/bin/app  # Only if doesn't exist

- name: Clean up
  shell: rm -rf /tmp/cache
  args:
    removes: /tmp/cache  # Only if exists
```

### Error handling

```yaml
- name: Try to download
  download:
    url: https://example.com/file.tar.gz
    dest: /tmp/file.tar.gz
  register: download_result
  ignore_errors: true

- name: Use fallback if download failed
  download:
    url: https://mirror.example.com/file.tar.gz
    dest: /tmp/file.tar.gz
  when: download_result.rc != 0
```

---

## See Also

- [Quick Reference](../quick-reference.md) - Common commands and patterns
- [FAQ](../faq.md) - Frequently asked questions
- [Examples](../../examples/) - Working examples
- [GitHub Issues](https://github.com/alehatsman/mooncake/issues) - Report bugs


---

<!-- FILE: index.md -->

# Mooncake

**The Standard Runtime for AI System Configuration**

Mooncake is to AI agents what Docker is to containers - a safe, validated execution layer for system configuration. **Chookity!**

<div class="grid cards" markdown>

-   :material-shield-check:{ .lg .middle } **Safe by Default**

    ---

    Dry-run validation, idempotency guarantees, rollback support for AI-driven configuration

-   :material-chart-line:{ .lg .middle } **Full Observability**

    ---

    Structured events, audit trails, execution logs for AI agent compliance

-   :material-check-circle:{ .lg .middle } **Validated Operations**

    ---

    Schema validation, type checking, state verification before execution

-   :material-robot:{ .lg .middle } **AI-Friendly Format**

    ---

    Simple YAML that any AI can generate and understand - no complex DSL

-   :rocket:{ .lg .middle } **Zero Dependencies**

    ---

    Single Go binary with no Python, no modules, no setup required

-   :material-devices:{ .lg .middle } **Cross-Platform**

    ---

    Unified interface for Linux, macOS, and Windows

</div>

---

## What is Mooncake?

Mooncake provides a safe, validated execution environment for AI agents to configure systems. Built for the AI-driven infrastructure era.

**Target Audiences:**

- **AI Agent Developers** - Build agents that configure systems safely with validated execution, observability, and compliance
- **Platform Engineers** - Manage AI-driven infrastructure with audit trails and safety guardrails
- **Developers with AI Assistants** - Let AI manage your dotfiles and dev setup with built-in safety and undo
- **DevOps Teams** - Simpler alternative to Ansible for personal/team configs with AI workflow integration

**Why AI Agents Choose Mooncake:**

- Industry-standard YAML format that any AI can target
- Guarantees idempotency and reproducibility
- Enables system configuration without risk
- Provides observability and compliance out of the box

---

## Installation

```bash
go install github.com/alehatsman/mooncake@latest
```

Verify installation:
```bash
mooncake --help
```

---

## 30 Second Quick Start

```bash
# Create config.yml
cat > config.yml <<'EOF'
- name: Hello Mooncake
  shell: echo "Chookity! Running on {{os}}/{{arch}}"

- name: Create a file
  file:
    path: /tmp/mooncake-test.txt
    state: file
    content: "Hello from Mooncake!"
EOF

# Preview changes (safe!)
mooncake run --config config.yml --dry-run

# Run it for real
mooncake run --config config.yml
```

**What just happened?**

1. Mooncake detected your OS automatically (`{{os}}`, `{{arch}}`)
2. Ran a shell command using those variables
3. Created a file with specific content

Check the result:
```bash
cat /tmp/mooncake-test.txt
# Output: Hello from Mooncake!
```

→ **[Try More Examples](examples/)** - Step-by-step learning path from beginner to advanced

---

## What You Can Do

Quick reference of available actions with examples:

### Run Commands

Execute shell commands with variables and conditionals.

```yaml
- name: OS-specific package install
  shell: "{{package_manager}} install neovim"
  become: true
  when: os == "linux"
```

**Features**: Multi-line scripts, timeouts, retries, environment variables, working directory

[Learn more: Shell Action →](guide/config/actions.md#shell)

---

### Manage Files & Directories

Create files, directories, links with permissions and ownership.

```yaml
- name: Create config directory
  file:
    path: ~/.config/myapp
    state: directory
    mode: "0755"

- name: Create config file
  file:
    path: ~/.config/myapp/settings.yml
    state: file
    content: |
      app_name: myapp
      version: 1.0
    mode: "0644"
```

**Features**: File/directory creation, symlinks, hard links, permissions, ownership, removal

[Learn more: File Action →](guide/config/actions.md#file)

---

### Render Templates

Render configuration files from templates with variables and logic.

```yaml
- name: Render nginx config
  template:
    src: ./nginx.conf.j2
    dest: /etc/nginx/nginx.conf
    mode: "0644"
    vars:
      port: 8080
      ssl_enabled: true
```

**Template syntax**: Variables `{{ var }}`, conditionals `{% if %}`, loops `{% for %}`, filters `{{ path | expanduser }}`

[Learn more: Template Action →](guide/config/actions.md#template)

---

### Copy Files

Copy files with checksum verification and backup support.

```yaml
- name: Deploy application config
  copy:
    src: ./configs/app.yml
    dest: /etc/app/config.yml
    mode: "0644"
    owner: app
    group: app
    backup: true
```

**Features**: Checksum verification, automatic backups, ownership management

[Learn more: Copy Action →](guide/config/actions.md#copy)

---

### Download Files

Download files from URLs with checksums and retry logic.

```yaml
- name: Download Go tarball
  download:
    url: "https://go.dev/dl/go1.21.5.linux-amd64.tar.gz"
    dest: "/tmp/go.tar.gz"
    checksum: "e2bc0b3e4b64111ec117295c088bde5f00eeed1567999ff77bc859d7df70078e"
    timeout: "10m"
    retries: 3
```

**Features**: Checksum verification, retry logic, custom headers, idempotent downloads

[Learn more: Download Action →](guide/config/actions.md#download)

---

### Extract Archives

Extract tar, tar.gz, and zip archives with security protections.

```yaml
- name: Extract Node.js
  unarchive:
    src: /tmp/node-v20.tar.gz
    dest: /opt/node
    strip_components: 1
    creates: /opt/node/bin/node
```

**Features**: Automatic format detection, path stripping, security validation, idempotency

[Learn more: Unarchive Action →](guide/config/actions.md#unarchive)

---

### Install Packages

Manage system packages with automatic package manager detection.

```yaml
- name: Install packages
  package:
    names:
      - neovim
      - ripgrep
      - fzf
    state: present
  become: true
```

**Features**: Auto-detect package manager (apt, dnf, yum, brew, choco, etc.), install/remove/upgrade, idempotent

[Learn more: Package Action →](guide/config/actions.md#package)

---

### Manage Services

Manage system services (systemd on Linux, launchd on macOS).

```yaml
- name: Configure and start nginx
  service:
    name: nginx
    state: started
    enabled: true
  become: true
```

**Features**: Start/stop/restart services, enable on boot, create unit files, drop-in configs

[Learn more: Service Action →](guide/config/actions.md#service)

---

### Verify State

Assert command results, file properties, and HTTP responses.

```yaml
- name: Verify Docker is installed
  assert:
    command:
      cmd: docker --version
      exit_code: 0

- name: Verify API is healthy
  assert:
    http:
      url: https://api.example.com/health
      status: 200
```

**Features**: Command assertions, file property checks, HTTP response validation, fail-fast behavior

[Learn more: Assert Action →](guide/config/actions.md#assert)

---

### Reusable Workflows

Use presets for complex, parameterized workflows.

```yaml
- name: Install Ollama
  preset: ollama
  with:
    state: present
    service: true
    pull:
      - llama3.1:8b
      - mistral:latest
  become: true
```

**Features**: Parameter validation, type safety, idempotency, platform detection

[Learn more: Presets →](guide/presets.md)

---

**→ [See All Actions in Reference](guide/config/actions.md)** - Complete action documentation with examples

---

## Control Your Execution

### Variables & System Facts

Define custom variables and use auto-detected system information.

```yaml
- vars:
    app_name: MyApp
    version: "1.0.0"

- name: Install application
  shell: echo "Installing {{app_name}} v{{version}} on {{os}}"
```

**Auto-detected facts**: `os`, `arch`, `cpu_cores`, `memory_total_mb`, `distribution`, `package_manager`, `hostname`, and more

```bash
mooncake facts  # See all available system facts
```

[Learn more: Variables Guide →](guide/config/variables.md)

---

### Conditionals

Execute steps based on conditions.

```yaml
- name: Install on macOS
  shell: brew install neovim
  when: os == "darwin"

- name: Install on Linux
  shell: apt install neovim
  become: true
  when: os == "linux" && package_manager == "apt"
```

**Operators**: `==`, `!=`, `>`, `<`, `>=`, `<=`, `&&`, `||`, `!`, `in`

[Learn more: Control Flow →](guide/config/control-flow.md)

---

### Loops

Iterate over lists or files to avoid repetition.

```yaml
# Iterate over lists
- vars:
    packages: [neovim, ripgrep, fzf, tmux]

- name: Install package
  shell: brew install {{item}}
  with_items: "{{packages}}"

# Iterate over files
- name: Deploy dotfile
  shell: cp "{{item.src}}" "~/{{item.name}}"
  with_filetree: ./dotfiles
  when: item.is_dir == false
```

[Learn more: Loops →](guide/config/control-flow.md#loops)

---

### Tags

Filter execution by workflow.

```yaml
- name: Development setup
  shell: install-dev-tools.sh
  tags: [dev, setup]

- name: Production deployment
  shell: deploy-prod.sh
  tags: [prod, deploy]
```

**Usage**:
```bash
# Run only dev-tagged steps
mooncake run --config config.yml --tags dev

# Multiple tags (OR logic)
mooncake run --config config.yml --tags dev,test
```

[Learn more: Tags →](guide/config/control-flow.md#tags)

---

## Key Features

### Dry-Run Mode

Preview all changes before applying with `--dry-run`:

```bash
mooncake run --config config.yml --dry-run
```

**What it shows**: Validates syntax, checks paths, shows what would execute - without making any changes.

---

### System Facts Collection

Mooncake automatically detects system information:

- **OS**: `os`, `arch`, `distribution`, `distribution_version`, `kernel_version`
- **Hardware**: `cpu_cores`, `cpu_model`, `memory_total_mb`, `memory_free_mb`
- **Network**: `ip_addresses`, `default_gateway`, `dns_servers`, `network_interfaces`
- **Software**: `package_manager`, `python_version`, `docker_version`, `git_version`
- **Storage**: `disks` (mounts, filesystem, size, usage)
- **GPU**: `gpus` (vendor, model, memory, driver, CUDA version)

```bash
mooncake facts              # Text output
mooncake facts --format json  # JSON output
```

[See all facts →](guide/config/reference.md#system-facts-reference)

---

### Execution Planning

Generate deterministic execution plans before running:

```bash
# View plan as text
mooncake plan --config config.yml

# Export as JSON for CI/CD
mooncake plan --config config.yml --format json --output plan.json

# Execute from saved plan
mooncake run --from-plan plan.json
```

**Use cases**: Debugging, verification, CI/CD integration, configuration analysis

[Learn more: Commands →](guide/commands.md)

---

### Robust Execution

Control command execution with timeouts, retries, and custom conditions:

```yaml
- name: Download with retry
  shell: curl -O https://example.com/file.tar.gz
  timeout: 10m
  retries: 3
  retry_delay: 30s
  failed_when: "result.rc != 0 and result.rc != 18"  # 18 = partial transfer
```

[Learn more: Execution Control →](examples/11-execution-control.md)

---

### Sudo Support

Execute privileged operations securely:

```yaml
- name: Install system package
  shell: apt update && apt install neovim
  become: true
```

**Password methods**:

- Interactive: `mooncake run --config config.yml --ask-become-pass` (or `-K`)
- File-based: `--sudo-pass-file ~/.mooncake/sudo_pass`
- Environment variable: `export SUDO_ASKPASS=/usr/bin/ssh-askpass`

[Learn more: Sudo →](examples/09-sudo.md)

---

## Quick Commands Reference

```bash
# Run configuration
mooncake run --config config.yml

# Preview changes (safe!)
mooncake run --config config.yml --dry-run

# Show system facts
mooncake facts
mooncake facts --format json

# Generate execution plan
mooncake plan --config config.yml
mooncake plan --config config.yml --format json --output plan.json

# Filter by tags
mooncake run --config config.yml --tags dev,test

# With sudo
mooncake run --config config.yml --ask-become-pass

# Execute from plan
mooncake run --from-plan plan.json

# Debug mode
mooncake run --config config.yml --log-level debug

# Disable TUI (for CI/CD)
mooncake run --config config.yml --raw

# JSON output
mooncake run --config config.yml --raw --output-format json
```

[See all commands →](guide/commands.md)

---

## Common Use Cases

### Dotfiles Management

Deploy and manage dotfiles across machines:

```yaml
- name: Create backup directory
  file:
    path: ~/.dotfiles-backup
    state: directory

- name: Deploy dotfiles
  shell: cp "{{item.src}}" "~/{{item.name}}"
  with_filetree: ./dotfiles
  when: item.is_dir == false
```

[See complete example →](examples/real-world-dotfiles.md)

---

### Development Environment Setup

Automate dev tool installation:

```yaml
- vars:
    dev_tools:
      - neovim
      - ripgrep
      - fzf
      - tmux
      - docker

- name: Install dev tools (macOS)
  shell: brew install {{item}}
  with_items: "{{dev_tools}}"
  when: os == "darwin"

- name: Install dev tools (Linux)
  shell: apt install -y {{item}}
  become: true
  with_items: "{{dev_tools}}"
  when: os == "linux" && package_manager == "apt"
```

---

### Multi-OS Configuration

Write once, run anywhere:

```yaml
- name: Install on Linux
  shell: apt install neovim
  become: true
  when: os == "linux"

- name: Install on macOS
  shell: brew install neovim
  when: os == "darwin"

- name: Install on Windows
  shell: choco install neovim
  when: os == "windows"
```

---

### System Provisioning

Set up new machines automatically:

```yaml
- name: Install system packages
  shell: "{{package_manager}} install {{item}}"
  become: true
  with_items:
    - git
    - curl
    - vim
    - htop
  when: os == "linux"

- name: Create user directories
  file:
    path: "{{item}}"
    state: directory
  with_items:
    - ~/.local/bin
    - ~/.config
    - ~/projects
    - ~/backup

- name: Deploy SSH config
  template:
    src: ./ssh_config.j2
    dest: ~/.ssh/config
    mode: "0600"
```

---

## Comparison

| Feature | Mooncake | Ansible | Shell Scripts |
|---------|----------|---------|---------------|
| **Setup** | Single binary | Python + modules | Text editor |
| **Dependencies** | None | Python, modules | System tools |
| **AI Agent Friendly** | Native support | Complex | Unsafe |
| **Dry-run** | Native | Check mode | Manual |
| **Idempotency** | Guaranteed | Yes | Manual |
| **Cross-platform** | Built-in | Limited | OS-specific |
| **System Facts** | Auto-detected | Gathered | Manual |
| **Best For** | AI agents, dotfiles | Enterprise automation | Quick tasks |

**Mooncake is the execution layer for AI-driven system configuration** - providing safety, validation, and observability that AI agents need.

---

## Next Steps

1. **[Actions Guide →](guide/config/actions.md)** - See what you can do
2. **[Complete Reference →](guide/config/reference.md)** - All properties and types
3. **[AI Specification →](ai-specification.md)** - For AI agents and LLMs
4. **[Complete Reference →](guide/config/reference.md)** - All properties

---

## Community & Support

- [:fontawesome-brands-github: GitHub Issues](https://github.com/alehatsman/mooncake/issues) - Report bugs and request features
- [:material-star: Star the project](https://github.com/alehatsman/mooncake) if you find it useful!
- [:material-book-open: Contributing Guide](development/contributing.md) - Help make Mooncake better
- [:material-map: Roadmap](development/roadmap.md) - Planned features
- [:material-history: Changelog](about/changelog.md) - What's new

---

## License

MIT License - Copyright (c) 2024-2026 Aleh Atsman

See [LICENSE](https://github.com/alehatsman/mooncake/blob/master/LICENSE) for details.


---

<!-- FILE: presets/catalog.md -->

# Preset Catalog

Complete catalog of all Mooncake presets.

**Total presets:** 388

---

## Kubernetes (18 presets)

### [argocd](../../presets/argocd/README.md)

GitOps / CD tool

**Platforms:** Linux, macOS, Windows

### [argocd-autopilot](../../presets/argocd-autopilot/README.md)

Automated GitOps bootstrap for Argo CD with opinionated repository structure

**Platforms:** Linux, macOS, Windows

### [flux](../../presets/flux/README.md)

Install Flux v1 (deprecated) - GitOps CD toolkit for Kubernetes (use fluxcd for v2)

**Platforms:** Linux, macOS, Windows

### [fluxcd](../../presets/fluxcd/README.md)

Install FluxCD v2 - GitOps continuous delivery for Kubernetes clusters

**Platforms:** Linux, macOS, Windows

### [gitkube](../../presets/gitkube/README.md)

Install Gitkube - git push to deploy applications on Kubernetes clusters

**Platforms:** Linux, macOS, Windows

### [helm](../../presets/helm/README.md)

Kubernetes package manager - deploy and manage applications

**Platforms:** Linux, macOS, Windows

### [helmfile](../../presets/helmfile/README.md)

Declarative Helm chart deployment

**Platforms:** Linux, macOS, Windows

### [influx-cli](../../presets/influx-cli/README.md)

InfluxDB command-line client for interacting with InfluxDB servers

**Platforms:** Linux, macOS, Windows

### [influxdb3](../../presets/influxdb3/README.md)

Time-series / analytics database

**Platforms:** Linux, macOS, Windows

### [istio](../../presets/istio/README.md)

Service mesh / proxy

**Platforms:** Linux, macOS, Windows

### [k8s-tools](../../presets/k8s-tools/README.md)

Install Kubernetes command-line tools (kubectl, k9s, helm)

**Platforms:** Linux, macOS, Windows

### [k9s](../../presets/k9s/README.md)

Install k9s - Kubernetes TUI for managing clusters

**Platforms:** Linux, macOS, Windows

### [kube-bench](../../presets/kube-bench/README.md)

Kubernetes CIS Benchmark security checker

**Platforms:** Linux, macOS, Windows

### [kube-hunter](../../presets/kube-hunter/README.md)

Kubernetes penetration testing and security scanner

**Platforms:** Linux, macOS, Windows

### [kubectl](../../presets/kubectl/README.md)

Install and configure Kubernetes command-line tool

**Platforms:** Linux, macOS, Windows

### [kubectx](../../presets/kubectx/README.md)

Fast way to switch between Kubernetes contexts

**Platforms:** Linux, macOS, Windows

### [kubeflow](../../presets/kubeflow/README.md)

Workflow orchestration

**Platforms:** Linux, macOS, Windows

### [kubescape](../../presets/kubescape/README.md)

Kubernetes security and compliance scanner

**Platforms:** Linux, macOS, Windows

## Databases (10 presets)

### [cassandra](../../presets/cassandra/README.md)

Apache Cassandra CLI

**Platforms:** Linux, macOS, Windows

### [elastic-apm](../../presets/elastic-apm/README.md)

Elastic APM Server

**Platforms:** Linux, macOS, Windows

### [elasticsearch](../../presets/elasticsearch/README.md)

Install and configure Elasticsearch search and analytics engine

**Platforms:** Linux, macOS, Windows

### [mongodb](../../presets/mongodb/README.md)

Document-oriented NoSQL database with flexible schema and native replication

**Platforms:** Linux, macOS, Windows

### [mongosh](../../presets/mongosh/README.md)

Modern MongoDB Shell with REPL, autocomplete, and syntax highlighting

**Platforms:** Linux, macOS, Windows

### [mysql](../../presets/mysql/README.md)

Relational database management system for data persistence

**Platforms:** Linux, macOS, Windows

### [neo4j](../../presets/neo4j/README.md)

Graph database management system with native graph storage and Cypher query language

**Platforms:** Linux, macOS, Windows

### [postgres](../../presets/postgres/README.md)

Install and configure PostgreSQL database

**Platforms:** Linux, macOS, Windows

### [redis](../../presets/redis/README.md)

Install and configure Redis in-memory data store

**Platforms:** Linux, macOS, Windows

### [redis-cli](../../presets/redis-cli/README.md)

Redis command-line interface for interacting with Redis servers

**Platforms:** Linux, macOS, Windows

## Containers (5 presets)

### [crane](../../presets/crane/README.md)

Container image manipulation tool

**Platforms:** Linux, macOS, Windows

### [dive](../../presets/dive/README.md)

Docker image layer explorer

**Platforms:** Linux, macOS, Windows

### [docker](../../presets/docker/README.md)

Install and configure Docker container runtime

**Platforms:** Linux, macOS, Windows

### [lazydocker](../../presets/lazydocker/README.md)

Install lazydocker - terminal UI for Docker and Docker Compose

**Platforms:** Linux, macOS, Windows

### [skopeo](../../presets/skopeo/README.md)

Work with container images and registries

**Platforms:** Multiple

## Cloud Platforms (7 presets)

### [awscli](../../presets/awscli/README.md)

Install AWS Command Line Interface (AWS CLI)

**Platforms:** Linux, macOS, Windows

### [azure-cli](../../presets/azure-cli/README.md)

Official command-line interface for managing Microsoft Azure cloud resources

**Platforms:** Linux, macOS, Windows

### [cdk](../../presets/cdk/README.md)

Infrastructure as Code tool

**Platforms:** Linux, macOS, Windows

### [cdktf](../../presets/cdktf/README.md)

Infrastructure as Code tool

**Platforms:** Linux, macOS, Windows

### [gcloud](../../presets/gcloud/README.md)

Google Cloud CLI

**Platforms:** Linux, macOS, Windows

### [pulumi](../../presets/pulumi/README.md)

Infrastructure as Code tool

**Platforms:** Multiple

### [terraform](../../presets/terraform/README.md)

Install Terraform infrastructure as code tool

**Platforms:** Multiple

## Monitoring & Observability (9 presets)

### [datadog-agent](../../presets/datadog-agent/README.md)

Install Datadog Agent for infrastructure monitoring, metrics collection, and APM tracing

**Platforms:** Linux, macOS, Windows

### [grafana](../../presets/grafana/README.md)

Install and configure Grafana visualization and analytics platform

**Platforms:** Linux, macOS, Windows

### [grafana-agent](../../presets/grafana-agent/README.md)

Time-series / analytics database

**Platforms:** Linux, macOS, Windows

### [loki-server](../../presets/loki-server/README.md)

Grafana Loki log aggregation system

**Platforms:** Linux, macOS

### [newrelic-cli](../../presets/newrelic-cli/README.md)

New Relic observability platform command-line interface

**Platforms:** Linux, macOS, Windows

### [prometheus](../../presets/prometheus/README.md)

Install and configure Prometheus monitoring system

**Platforms:** Linux, macOS, Windows

### [prometheus-server](../../presets/prometheus-server/README.md)

Time-series / analytics database

**Platforms:** Multiple

### [tempo](../../presets/tempo/README.md)

Distributed tracing backend

**Platforms:** Multiple

### [temporal](../../presets/temporal/README.md)

Workflow orchestration

**Platforms:** Multiple

## Languages & Runtimes (26 presets)

### [algolia-cli](../../presets/algolia-cli/README.md)

Command-line interface for Algolia search and analytics platform

**Platforms:** Linux, macOS, Windows

### [arangodb](../../presets/arangodb/README.md)

Multi-model database supporting graphs, documents, key-value, and search

**Platforms:** Linux, macOS, Windows

### [argo-events](../../presets/argo-events/README.md)

Event-driven workflow automation for Kubernetes with 20+ event sources

**Platforms:** Linux, macOS, Windows

### [argo-rollouts](../../presets/argo-rollouts/README.md)

Progressive delivery controller for Kubernetes with canary and blue-green deployments

**Platforms:** Linux, macOS, Windows

### [argo-workflows](../../presets/argo-workflows/README.md)

Kubernetes-native workflow engine for orchestrating parallel jobs and pipelines

**Platforms:** Linux, macOS, Windows

### [cargo-edit](../../presets/cargo-edit/README.md)

Rust/Go tool

**Platforms:** Linux, macOS, Windows

### [cargo-make](../../presets/cargo-make/README.md)

Rust/Go tool

**Platforms:** Linux, macOS, Windows

### [cargo-watch](../../presets/cargo-watch/README.md)

Rust/Go tool

**Platforms:** Linux, macOS, Windows

### [chruby](../../presets/chruby/README.md)

Language tool

**Platforms:** Linux, macOS, Windows

### [dragonfly](../../presets/dragonfly/README.md)

Cache / storage system

**Platforms:** Linux, macOS, Windows

### [go](../../presets/go/README.md)

Install Go programming language

**Platforms:** Linux, macOS, Windows

### [golangci-lint](../../presets/golangci-lint/README.md)

Fast linters runner for Go with parallel execution and caching

**Platforms:** Linux, macOS, Windows

### [gopass](../../presets/gopass/README.md)

Team password manager with Git synchronization and GPG encryption

**Platforms:** Linux, macOS, Windows

### [goreleaser](../../presets/goreleaser/README.md)

Release automation for Go projects with multi-platform builds

**Platforms:** Linux, macOS, Windows

### [gotty](../../presets/gotty/README.md)

Share terminal as web application with authentication support

**Platforms:** Linux, macOS, Windows

### [gox](../../presets/gox/README.md)

Dead simple cross-compilation tool for Go

**Platforms:** Linux, macOS, Windows

### [hugo](../../presets/hugo/README.md)

Fast static site generator written in Go for building websites

**Platforms:** Linux, macOS, Windows

### [java](../../presets/java/README.md)

Install OpenJDK Java Development Kit

**Platforms:** Linux, macOS, Windows

### [linode-cli](../../presets/linode-cli/README.md)

Linode cloud platform command-line interface

**Platforms:** Linux, macOS, Windows

### [nodejs](../../presets/nodejs/README.md)

Install Node.js via nvm (Node Version Manager)

**Platforms:** Linux, macOS, Windows

### [php](../../presets/php/README.md)

Install PHP programming language with Composer

**Platforms:** Linux, macOS, Windows

### [python](../../presets/python/README.md)

Install Python via pyenv (Python version manager)

**Platforms:** Linux, macOS, Windows

### [ruby](../../presets/ruby/README.md)

Install Ruby via rbenv (Ruby version manager)

**Platforms:** Multiple

### [rust](../../presets/rust/README.md)

Install Rust via rustup (Rust toolchain installer)

**Platforms:** Multiple

### [rustup](../../presets/rustup/README.md)

Official Rust toolchain installer for managing Rust versions and components

**Platforms:** Linux, macOS, Windows

### [ts-node](../../presets/ts-node/README.md)

Node.js / JavaScript tool

**Platforms:** Linux, macOS, Windows

## Data Tools (5 presets)

### [csvkit](../../presets/csvkit/README.md)

Suite of command-line tools for working with CSV

**Platforms:** Linux, macOS, Windows

### [fx](../../presets/fx/README.md)

Terminal JSON viewer with interactive exploration

**Platforms:** Linux, macOS, Windows

### [jq](../../presets/jq/README.md)

Install jq - lightweight and flexible command-line JSON processor

**Platforms:** Linux, macOS, Windows

### [miller](../../presets/miller/README.md)

Process CSV, JSON, TSV, and other structured data formats using named fields

**Platforms:** Linux, macOS, Windows

### [yq](../../presets/yq/README.md)

Portable command-line YAML, JSON, XML, and properties processor with jq-like syntax

**Platforms:** Linux, macOS, Windows

## Editors & IDEs (4 presets)

### [intellij](../../presets/intellij/README.md)

Text editor / IDE

**Platforms:** Linux, macOS, Windows

### [neovim](../../presets/neovim/README.md)

Install Neovim text editor with plugin manager

**Platforms:** Linux, macOS, Windows

### [vscode](../../presets/vscode/README.md)

Text editor / IDE

**Platforms:** Multiple

### [zed](../../presets/zed/README.md)

High-performance collaborative code editor built in Rust with AI integration

**Platforms:** Linux, macOS, Windows

## Shell & Terminal (6 presets)

### [fish](../../presets/fish/README.md)

Install fish - friendly interactive shell with autosuggestions and syntax highlighting

**Platforms:** Linux, macOS, Windows

### [nushell](../../presets/nushell/README.md)

Modern shell with structured data pipelines and type system

**Platforms:** Linux, macOS, Windows

### [screen](../../presets/screen/README.md)

Classic terminal multiplexer for running multiple shell sessions in one terminal with detach/reattach support

**Platforms:** Multiple

### [tmux](../../presets/tmux/README.md)

Install and configure Tmux terminal multiplexer

**Platforms:** Multiple

### [tmuxinator](../../presets/tmuxinator/README.md)

Utility tool

**Platforms:** Linux, macOS, Windows

### [zsh](../../presets/zsh/README.md)

Install Zsh shell with Oh My Zsh framework

**Platforms:** Linux, macOS, Windows

## Version Control (3 presets)

### [gh](../../presets/gh/README.md)

Install GitHub CLI - work with GitHub from the command line

**Platforms:** Linux, macOS, Windows

### [gitlab-runner](../../presets/gitlab-runner/README.md)

Install GitLab Runner - execute GitLab CI/CD pipeline jobs

**Platforms:** Linux, macOS, Windows

### [lazygit](../../presets/lazygit/README.md)

Install lazygit - terminal UI for git commands

**Platforms:** Linux, macOS, Windows

## Build Tools (7 presets)

### [bazel](../../presets/bazel/README.md)

Google's build tool supporting multi-language projects with incremental builds and remote caching

**Platforms:** Linux, macOS, Windows

### [cmake](../../presets/cmake/README.md)

Cross-platform build system

**Platforms:** Linux, macOS, Windows

### [gradle](../../presets/gradle/README.md)

Build automation tool for JVM projects with dependency management

**Platforms:** Linux, macOS, Windows

### [just](../../presets/just/README.md)

Command runner and task automation

**Platforms:** Linux, macOS, Windows

### [make](../../presets/make/README.md)

GNU Make build tool

**Platforms:** Linux, macOS, Windows

### [maven](../../presets/maven/README.md)

Java build tool and dependency manager

**Platforms:** Linux, macOS, Windows

### [task](../../presets/task/README.md)

Modern task runner and build tool with YAML configuration, cross-platform support, and smart caching

**Platforms:** Linux, macOS, Windows

## Security & Secrets (9 presets)

### [1password-cli](../../presets/1password-cli/README.md)

Command-line tool for 1Password password manager and secret storage

**Platforms:** Linux, macOS, Windows

### [age](../../presets/age/README.md)

Simple and secure file encryption tool, a modern GPG alternative

**Platforms:** Linux, macOS, Windows

### [bitwarden-cli](../../presets/bitwarden-cli/README.md)

Official command-line interface for Bitwarden password manager

**Platforms:** Linux, macOS, Windows

### [buildkite-agent](../../presets/buildkite-agent/README.md)

CI/CD tool

**Platforms:** Linux, macOS, Windows

### [imagemagick](../../presets/imagemagick/README.md)

Image manipulation suite for editing and converting images

**Platforms:** Linux, macOS, Windows

### [lastpass-cli](../../presets/lastpass-cli/README.md)

LastPass command-line password manager

**Platforms:** Linux, macOS, Windows

### [pass](../../presets/pass/README.md)

Unix password manager

**Platforms:** Multiple

### [sops](../../presets/sops/README.md)

Secrets management

**Platforms:** Multiple

### [vault](../../presets/vault/README.md)

Install and configure HashiCorp Vault secrets management platform

**Platforms:** Linux, macOS, Windows

## Other Tools (279 presets)

### [act](../../presets/act/README.md)

Run GitHub Actions workflows locally with Docker for testing and debugging

**Platforms:** Linux, macOS, Windows

### [actionlint](../../presets/actionlint/README.md)

Static linter for GitHub Actions workflow files with syntax and security checks

**Platforms:** Linux, macOS, Windows

### [activemq](../../presets/activemq/README.md)

Apache ActiveMQ message broker for JMS and multi-protocol messaging

**Platforms:** Linux, macOS, Windows

### [aerospike](../../presets/aerospike/README.md)

Cache / storage system

**Platforms:** Linux, macOS, Windows

### [air](../../presets/air/README.md)

Live reload utility for Go applications with hot reloading during development

**Platforms:** Linux, macOS, Windows

### [airbyte](../../presets/airbyte/README.md)

Open-source data integration platform for building ETL/ELT pipelines

**Platforms:** Linux, macOS, Windows

### [airflow](../../presets/airflow/README.md)

Apache Airflow workflow orchestration platform for data pipeline scheduling

**Platforms:** Linux, macOS, Windows

### [alacritty](../../presets/alacritty/README.md)

GPU-accelerated terminal emulator

**Platforms:** Linux, macOS, Windows

### [ambassador](../../presets/ambassador/README.md)

Kubernetes-native API gateway built on Envoy Proxy for traffic management

**Platforms:** Linux, macOS, Windows

### [anchore](../../presets/anchore/README.md)

Container image security and compliance scanning with vulnerability detection

**Platforms:** Linux, macOS, Windows

### [ant](../../presets/ant/README.md)

Apache Ant build automation tool for Java projects

**Platforms:** Linux, macOS, Windows

### [apisix](../../presets/apisix/README.md)

Cloud-native API gateway with dynamic routing and plugin ecosystem

**Platforms:** Linux, macOS, Windows

### [aqua](../../presets/aqua/README.md)

Cloud-native security platform for container and Kubernetes protection

**Platforms:** Linux, macOS, Windows

### [artemis](../../presets/artemis/README.md)

Install and configure Apache ActiveMQ Artemis message broker CLI

**Platforms:** Linux, macOS, Windows

### [asciinema](../../presets/asciinema/README.md)

Terminal session recorder and player for creating demos and tutorials

**Platforms:** Linux, macOS, Windows

### [asdf](../../presets/asdf/README.md)

CLI productivity tool

**Platforms:** Linux, macOS, Windows

### [astro](../../presets/astro/README.md)

Static site generator

**Platforms:** Linux, macOS, Windows

### [atlantis](../../presets/atlantis/README.md)

Terraform pull request automation for GitOps workflows

**Platforms:** Linux, macOS, Windows

### [atuin](../../presets/atuin/README.md)

CLI productivity tool

**Platforms:** Linux, macOS, Windows

### [autojump](../../presets/autojump/README.md)

CLI productivity tool

**Platforms:** Linux, macOS, Windows

### [bandwhich](../../presets/bandwhich/README.md)

Terminal bandwidth utilization tool showing real-time network usage per process

**Platforms:** Linux, macOS, Windows

### [beam](../../presets/beam/README.md)

Data processing / ETL

**Platforms:** Linux, macOS, Windows

### [black](../../presets/black/README.md)

Uncompromising Python code formatter with deterministic output

**Platforms:** Linux, macOS, Windows

### [blast](../../presets/blast/README.md)

Search / analytics engine

**Platforms:** Linux, macOS, Windows

### [bookkeeper](../../presets/bookkeeper/README.md)

Message queue / streaming

**Platforms:** Linux, macOS, Windows

### [borg](../../presets/borg/README.md)

Deduplicating backup program with compression and encryption

**Platforms:** Linux, macOS

### [bottom](../../presets/bottom/README.md)

Graphical process and system monitor with customizable widgets

**Platforms:** Linux, macOS, Windows

### [btop](../../presets/btop/README.md)

System monitoring utility

**Platforms:** Linux, macOS, Windows

### [bun](../../presets/bun/README.md)

Node.js / JavaScript tool

**Platforms:** Linux, macOS, Windows

### [byobu](../../presets/byobu/README.md)

Terminal multiplexer with status notifications and configuration profiles

**Platforms:** Linux, macOS, Windows

### [cabal](../../presets/cabal/README.md)

Language tool

**Platforms:** Linux, macOS, Windows

### [caddy](../../presets/caddy/README.md)

Install and configure Caddy web server with automatic HTTPS

**Platforms:** Linux, macOS, Windows

### [cadence](../../presets/cadence/README.md)

Workflow orchestration

**Platforms:** Linux, macOS, Windows

### [chamber](../../presets/chamber/README.md)

Secrets management

**Platforms:** Linux, macOS, Windows

### [checkov](../../presets/checkov/README.md)

Security scanning tool

**Platforms:** Linux, macOS, Windows

### [circleci-cli](../../presets/circleci-cli/README.md)

CI/CD tool

**Platforms:** Linux, macOS, Windows

### [clair](../../presets/clair/README.md)

Security scanning tool

**Platforms:** Linux, macOS, Windows

### [clickhouse-client](../../presets/clickhouse-client/README.md)

ClickHouse client

**Platforms:** Linux, macOS, Windows

### [clojure](../../presets/clojure/README.md)

Functional Lisp dialect for the JVM with immutable data structures

**Platforms:** Linux, macOS, Windows

### [consul](../../presets/consul/README.md)

Install HashiCorp Consul - service mesh, service discovery, and configuration

**Platforms:** Linux, macOS, Windows

### [contour](../../presets/contour/README.md)

API gateway / load balancer

**Platforms:** Linux, macOS, Windows

### [cortex](../../presets/cortex/README.md)

Time-series / analytics database

**Platforms:** Linux, macOS, Windows

### [cosign](../../presets/cosign/README.md)

Container signing and verification

**Platforms:** Linux, macOS, Windows

### [couchbase](../../presets/couchbase/README.md)

Cache / storage system

**Platforms:** Linux, macOS, Windows

### [cqlsh](../../presets/cqlsh/README.md)

Cassandra Query Language Shell - interactive CQL command-line interface

**Platforms:** Linux, macOS, Windows

### [croc](../../presets/croc/README.md)

File transfer / backup tool

**Platforms:** Linux, macOS, Windows

### [crossplane](../../presets/crossplane/README.md)

Infrastructure as Code tool

**Platforms:** Linux, macOS, Windows

### [ctop](../../presets/ctop/README.md)

Container metrics and monitoring

**Platforms:** Linux, macOS, Windows

### [curlie](../../presets/curlie/README.md)

curl + httpie frontend

**Platforms:** Linux, macOS, Windows

### [dagster](../../presets/dagster/README.md)

Data processing / ETL

**Platforms:** Linux, macOS, Windows

### [dbt](../../presets/dbt/README.md)

Install dbt (Data Build Tool) for SQL-based data transformations and analytics engineering

**Platforms:** Linux, macOS, Windows

### [delta](../../presets/delta/README.md)

Install delta - syntax-highlighting pager for git, diff, and grep output

**Platforms:** Linux, macOS, Windows

### [deno](../../presets/deno/README.md)

Install Deno - secure runtime for JavaScript and TypeScript

**Platforms:** Linux, macOS, Windows

### [dgraph](../../presets/dgraph/README.md)

Graph / vector database

**Platforms:** Linux, macOS, Windows

### [direnv](../../presets/direnv/README.md)

Environment variable manager that loads/unloads per-directory configurations

**Platforms:** Linux, macOS, Windows

### [doctl](../../presets/doctl/README.md)

DigitalOcean CLI

**Platforms:** Linux, macOS, Windows

### [docusaurus](../../presets/docusaurus/README.md)

Static site generator

**Platforms:** Linux, macOS, Windows

### [dog](../../presets/dog/README.md)

Modern DNS client

**Platforms:** Linux, macOS, Windows

### [doppler](../../presets/doppler/README.md)

Secrets management

**Platforms:** Linux, macOS, Windows

### [drone-cli](../../presets/drone-cli/README.md)

CI/CD tool

**Platforms:** Linux, macOS, Windows

### [dstat](../../presets/dstat/README.md)

System monitoring utility

**Platforms:** Linux, macOS, Windows

### [duf](../../presets/duf/README.md)

System monitoring utility

**Platforms:** Linux, macOS, Windows

### [duplicity](../../presets/duplicity/README.md)

Install Duplicity - encrypted bandwidth-efficient backup tool

**Platforms:** Linux, macOS, Windows

### [editorconfig](../../presets/editorconfig/README.md)

Install EditorConfig core - maintain consistent coding styles across editors

**Platforms:** Linux, macOS, Windows

### [eleventy](../../presets/eleventy/README.md)

Static site generator

**Platforms:** Linux, macOS, Windows

### [elvish](../../presets/elvish/README.md)

Install Elvish - friendly interactive shell and expressive programming language

**Platforms:** Linux, macOS, Windows

### [emissary](../../presets/emissary/README.md)

API gateway / load balancer

**Platforms:** Linux, macOS

### [entr](../../presets/entr/README.md)

Run commands when files change

**Platforms:** Linux, macOS

### [envoy](../../presets/envoy/README.md)

Service mesh / proxy

**Platforms:** Linux, macOS

### [envoy-gateway](../../presets/envoy-gateway/README.md)

API gateway / load balancer

**Platforms:** Linux, macOS

### [etcd](../../presets/etcd/README.md)

Install and configure etcd distributed key-value store for Kubernetes and service discovery

**Platforms:** Linux, macOS, Windows

### [falco](../../presets/falco/README.md)

Install Falco runtime security and threat detection for containers and Kubernetes

**Platforms:** Linux, macOS, Windows

### [ffmpeg](../../presets/ffmpeg/README.md)

Install FFmpeg multimedia framework for video/audio processing and conversion

**Platforms:** Linux, macOS, Windows

### [fivetran](../../presets/fivetran/README.md)

Install Fivetran CLI - automated data pipeline management tool

**Platforms:** Linux, macOS, Windows

### [fleet](../../presets/fleet/README.md)

Install Rancher Fleet - GitOps at scale for managing Kubernetes clusters

**Platforms:** Linux, macOS, Windows

### [flink](../../presets/flink/README.md)

Install Apache Flink - stream processing framework for distributed data processing

**Platforms:** Linux, macOS, Windows

### [fly](../../presets/fly/README.md)

Fly.io CLI

**Platforms:** Linux, macOS, Windows

### [flyte](../../presets/flyte/README.md)

Install Flyte CLI - workflow orchestration platform for data and ML pipelines

**Platforms:** Linux, macOS, Windows

### [fnm](../../presets/fnm/README.md)

Install fnm (Fast Node Manager) - fast and simple Node.js version manager

**Platforms:** Linux, macOS, Windows

### [fzf](../../presets/fzf/README.md)

Install fzf - command-line fuzzy finder

**Platforms:** Linux, macOS, Windows

### [garnet](../../presets/garnet/README.md)

Install Microsoft Garnet - high-performance Redis-compatible cache server

**Platforms:** Linux, macOS, Windows

### [gatsby](../../presets/gatsby/README.md)

Install Gatsby - React-based static site generator and web framework

**Platforms:** Linux, macOS, Windows

### [glances](../../presets/glances/README.md)

System monitoring utility

**Platforms:** Linux, macOS, Windows

### [gloo](../../presets/gloo/README.md)

Kubernetes-native API gateway and ingress controller CLI (glooctl)

**Platforms:** Linux, macOS, Windows

### [gping](../../presets/gping/README.md)

Ping with graph

**Platforms:** Linux, macOS, Windows

### [graphviz](../../presets/graphviz/README.md)

Graph visualization software for rendering diagrams and networks

**Platforms:** Linux, macOS, Windows

### [gron](../../presets/gron/README.md)

Make JSON greppable - transform JSON into discrete assignments

**Platforms:** Linux, macOS, Windows

### [groovy](../../presets/groovy/README.md)

Dynamic language for the JVM with scripting and DSL capabilities

**Platforms:** Linux, macOS, Windows

### [grype](../../presets/grype/README.md)

Vulnerability scanner for container images

**Platforms:** Linux, macOS, Windows

### [hadolint](../../presets/hadolint/README.md)

Dockerfile linter for best practices and common errors

**Platforms:** Linux, macOS, Windows

### [haproxy](../../presets/haproxy/README.md)

High performance TCP/HTTP load balancer

**Platforms:** Linux, macOS, Windows

### [harness](../../presets/harness/README.md)

DevOps platform CLI for CI/CD, GitOps, and feature flags

**Platforms:** Linux, macOS, Windows

### [hatch](../../presets/hatch/README.md)

Modern Python project manager with versioning and publishing

**Platforms:** Linux, macOS, Windows

### [hazelcast](../../presets/hazelcast/README.md)

Distributed in-memory data grid for fast caching and computing

**Platforms:** Linux, macOS, Windows

### [hcloud](../../presets/hcloud/README.md)

Hetzner Cloud CLI

**Platforms:** Linux, macOS, Windows

### [helix](../../presets/helix/README.md)

Post-modern modal text editor with built-in LSP and tree-sitter

**Platforms:** Linux, macOS, Windows

### [hexo](../../presets/hexo/README.md)

Static site generator

**Platforms:** Linux, macOS, Windows

### [httpie](../../presets/httpie/README.md)

Install HTTPie - modern, user-friendly HTTP client for APIs

**Platforms:** Linux, macOS, Windows

### [httpstat](../../presets/httpstat/README.md)

Visualize curl statistics in a beautiful way

**Platforms:** Linux, macOS, Windows

### [httpx](../../presets/httpx/README.md)

Fast HTTP toolkit

**Platforms:** Linux, macOS, Windows

### [ibmcloud-cli](../../presets/ibmcloud-cli/README.md)

IBM Cloud CLI

**Platforms:** Linux, macOS, Windows

### [ignite](../../presets/ignite/README.md)

Cache / storage system

**Platforms:** Linux, macOS, Windows

### [infisical](../../presets/infisical/README.md)

Secrets management

**Platforms:** Linux, macOS, Windows

### [infracost](../../presets/infracost/README.md)

Infrastructure as Code tool

**Platforms:** Linux, macOS, Windows

### [iperf3](../../presets/iperf3/README.md)

Network bandwidth measurement

**Platforms:** Linux, macOS, Windows

### [janusgraph](../../presets/janusgraph/README.md)

Distributed graph database with Gremlin query support

**Platforms:** Linux, macOS, Windows

### [jekyll](../../presets/jekyll/README.md)

Static site generator for blogs and documentation

**Platforms:** Linux, macOS, Windows

### [jenv](../../presets/jenv/README.md)

Java version manager for switching between multiple JDK installations

**Platforms:** Linux, macOS, Windows

### [jless](../../presets/jless/README.md)

Command-line JSON viewer with collapsing and search

**Platforms:** Linux, macOS, Windows

### [jupyter](../../presets/jupyter/README.md)

Install Jupyter Lab/Notebook for interactive data science and ML

**Platforms:** Linux, macOS, Windows

### [k3sup](../../presets/k3sup/README.md)

Bootstrap Kubernetes with k3s over SSH

**Platforms:** Linux, macOS, Windows

### [kafka](../../presets/kafka/README.md)

Install and configure Apache Kafka distributed streaming platform

**Platforms:** Linux, macOS, Windows

### [kakoune](../../presets/kakoune/README.md)

Modal code editor with multiple selections and interactive feedback

**Platforms:** Linux, macOS, Windows

### [kestra](../../presets/kestra/README.md)

Workflow orchestration

**Platforms:** Linux, macOS, Windows

### [keydb](../../presets/keydb/README.md)

Cache / storage system

**Platforms:** Linux, macOS, Windows

### [kitty](../../presets/kitty/README.md)

Fast GPU-based terminal emulator

**Platforms:** Linux, macOS, Windows

### [ko](../../presets/ko/README.md)

Rust/Go tool

**Platforms:** Linux, macOS, Windows

### [kong](../../presets/kong/README.md)

API gateway / load balancer

**Platforms:** Linux, macOS, Windows

### [kotlin](../../presets/kotlin/README.md)

Modern statically-typed JVM language with concise syntax

**Platforms:** Linux, macOS, Windows

### [krakend](../../presets/krakend/README.md)

API gateway / load balancer

**Platforms:** Linux, macOS, Windows

### [kusk](../../presets/kusk/README.md)

OpenAPI-driven API gateway for Kubernetes

**Platforms:** Linux, macOS, Windows

### [lapce](../../presets/lapce/README.md)

Lightning-fast GPU-accelerated code editor with built-in LSP

**Platforms:** Linux, macOS, Windows

### [leiningen](../../presets/leiningen/README.md)

Clojure build automation and dependency management tool

**Platforms:** Linux, macOS, Windows

### [linkerd](../../presets/linkerd/README.md)

Linkerd service mesh CLI for Kubernetes

**Platforms:** Linux, macOS, Windows

### [lite-xl](../../presets/lite-xl/README.md)

Lightweight text editor with minimal resource footprint

**Platforms:** Linux, macOS, Windows

### [litecli](../../presets/litecli/README.md)

SQLite CLI with autocomplete

**Platforms:** Linux, macOS, Windows

### [m3db](../../presets/m3db/README.md)

Distributed time-series database for metrics at scale

**Platforms:** Linux, macOS

### [magic-wormhole](../../presets/magic-wormhole/README.md)

Secure peer-to-peer file transfer tool

**Platforms:** Linux, macOS, Windows

### [manticore](../../presets/manticore/README.md)

High-performance full-text search engine

**Platforms:** Linux, macOS, Windows

### [markdownlint](../../presets/markdownlint/README.md)

Markdown linter and style checker for consistent documentation

**Platforms:** Linux, macOS, Windows

### [masscan](../../presets/masscan/README.md)

Ultra-fast TCP port scanner for network reconnaissance

**Platforms:** Linux, macOS, Windows

### [mcfly](../../presets/mcfly/README.md)

Neural network-powered shell history search with intelligent command ranking and prediction

**Platforms:** Linux, macOS, Windows

### [mdl](../../presets/mdl/README.md)

Ruby-based Markdown linter for style checking and consistency validation

**Platforms:** Linux, macOS, Windows

### [meilisearch](../../presets/meilisearch/README.md)

Install Meilisearch, a fast, open-source search engine with typo tolerance and instant results

**Platforms:** Linux, macOS, Windows

### [meltano](../../presets/meltano/README.md)

Install Meltano, an open-source ELT platform for building data integration pipelines with Singer taps and targets

**Platforms:** Linux, macOS, Windows

### [memcached](../../presets/memcached/README.md)

Install Memcached, a free, open-source, high-performance distributed memory caching system for reducing database load

**Platforms:** Linux, macOS, Windows

### [meson](../../presets/meson/README.md)

Fast, developer-friendly build system with cross-compilation support and multi-language capabilities

**Platforms:** Linux, macOS, Windows

### [metaflow](../../presets/metaflow/README.md)

Python framework for building and managing data science workflows with cloud integration

**Platforms:** Linux, macOS, Windows

### [micro](../../presets/micro/README.md)

Modern terminal text editor with mouse support, syntax highlighting, and plugins

**Platforms:** Linux, macOS, Windows

### [milvus](../../presets/milvus/README.md)

Vector database for AI applications featuring cloud-native design and high-performance similarity search

**Platforms:** Linux, macOS, Windows

### [mimir](../../presets/mimir/README.md)

Prometheus-compatible metrics backend with multi-tenancy and horizontal scaling for long-term metric storage

**Platforms:** Linux, macOS, Windows

### [miniconda](../../presets/miniconda/README.md)

Minimal Conda installer for managing Python environments and dependencies with built-in package management

**Platforms:** Linux, macOS, Windows

### [minio](../../presets/minio/README.md)

S3-compatible object storage server for private cloud and data lakes

**Platforms:** Linux, macOS, Windows

### [mise](../../presets/mise/README.md)

Polyglot development environment and version manager for 100+ languages

**Platforms:** Linux, macOS, Windows

### [mitmproxy](../../presets/mitmproxy/README.md)

Interactive HTTPS proxy for debugging, testing, and traffic inspection

**Platforms:** Linux, macOS, Windows

### [mix](../../presets/mix/README.md)

Install Mix build tool and task runner for Elixir projects

**Platforms:** Linux, macOS, Windows

### [mkcert](../../presets/mkcert/README.md)

Install mkcert tool for generating locally-trusted HTTPS certificates

**Platforms:** Linux, macOS, Windows

### [mkdocs](../../presets/mkdocs/README.md)

Install MkDocs static site generator for project documentation

**Platforms:** Linux, macOS, Windows

### [mlflow](../../presets/mlflow/README.md)

Machine learning experiment tracking and model registry platform

**Platforms:** Linux, macOS, Windows

### [modern-unix](../../presets/modern-unix/README.md)

High-performance replacements for classic Unix tools with better UX and speed

**Platforms:** Linux, macOS, Windows

### [mosquitto](../../presets/mosquitto/README.md)

Lightweight MQTT message broker for IoT and edge computing

**Platforms:** Linux, macOS, Windows

### [mtr](../../presets/mtr/README.md)

Network diagnostic tool combining traceroute and ping with real-time latency monitoring

**Platforms:** Linux, macOS

### [mycli](../../presets/mycli/README.md)

MySQL client with auto-completion and syntax highlighting

**Platforms:** Linux, macOS, Windows

### [mypy](../../presets/mypy/README.md)

Static type checker for Python code

**Platforms:** Linux, macOS, Windows

### [nats-cli](../../presets/nats-cli/README.md)

NATS command-line client for managing NATS servers and messaging operations

**Platforms:** Linux, macOS, Windows

### [nats-server](../../presets/nats-server/README.md)

NATS cloud-native messaging server for distributed systems communication

**Platforms:** Linux, macOS, Windows

### [navi](../../presets/navi/README.md)

Interactive cheatsheet tool for discovering and executing command-line commands

**Platforms:** Linux, macOS, Windows

### [ncdu](../../presets/ncdu/README.md)

Fast interactive disk usage analyzer with ncurses interface

**Platforms:** Linux, macOS

### [nebula](../../presets/nebula/README.md)

Overlay networking tool for creating scalable private networks between hosts

**Platforms:** Linux, macOS, Windows

### [next](../../presets/next/README.md)

React framework with server-side rendering and static generation

**Platforms:** Linux, macOS, Windows

### [nginx](../../presets/nginx/README.md)

High-performance HTTP server, reverse proxy, and load balancer

**Platforms:** Linux, macOS, Windows

### [ngrok](../../presets/ngrok/README.md)

Secure introspectable tunnels to localhost for webhook testing and demos

**Platforms:** Linux, macOS, Windows

### [nim](../../presets/nim/README.md)

Efficient systems programming language with Python-like syntax and C performance

**Platforms:** Linux, macOS, Windows

### [ninja](../../presets/ninja/README.md)

Small build system focused on speed for large projects

**Platforms:** Linux, macOS, Windows

### [nkeys](../../presets/nkeys/README.md)

NATS cryptographic key utility for generating and managing NKEYs

**Platforms:** Linux, macOS, Windows

### [nmap](../../presets/nmap/README.md)

Network discovery and security scanner for port scanning and vulnerability assessment

**Platforms:** Linux, macOS, Windows

### [nsc](../../presets/nsc/README.md)

NATS account management tool for creating users and managing permissions

**Platforms:** Linux, macOS, Windows

### [nvm](../../presets/nvm/README.md)

Node Version Manager for managing multiple Node.js versions

**Platforms:** Multiple

### [oci-cli](../../presets/oci-cli/README.md)

Oracle Cloud Infrastructure command-line interface for managing OCI resources

**Platforms:** Linux, macOS, Windows

### [oh-my-posh](../../presets/oh-my-posh/README.md)

Customizable prompt theme engine for any shell with Git integration

**Platforms:** Linux, macOS, Windows

### [ollama](../../presets/ollama/README.md)

Install and manage Ollama LLM runtime

**Platforms:** Linux, macOS

### [opam](../../presets/opam/README.md)

OCaml package manager

**Platforms:** Linux, macOS

### [opensearch](../../presets/opensearch/README.md)

Open-source search and analytics engine

**Platforms:** Linux, macOS

### [orientdb](../../presets/orientdb/README.md)

Multi-model graph database

**Platforms:** Linux, macOS, Windows

### [p10k](../../presets/p10k/README.md)

Powerlevel10k Zsh theme

**Platforms:** Linux, macOS, Windows

### [pandoc](../../presets/pandoc/README.md)

Universal document converter

**Platforms:** Linux, macOS, Windows

### [pdm](../../presets/pdm/README.md)

Modern Python package manager

**Platforms:** Multiple

### [pelican](../../presets/pelican/README.md)

Static site generator for Python

**Platforms:** Linux, macOS, Windows

### [pgcli](../../presets/pgcli/README.md)

PostgreSQL CLI with autocomplete

**Platforms:** Linux, macOS, Windows

### [pinecone](../../presets/pinecone/README.md)

Pinecone CLI for vector database

**Platforms:** Linux, macOS, Windows

### [pipenv](../../presets/pipenv/README.md)

Python dependency manager

**Platforms:** Linux, macOS, Windows

### [pnpm](../../presets/pnpm/README.md)

Fast disk space efficient package manager

**Platforms:** Linux, macOS, Windows

### [poetry](../../presets/poetry/README.md)

Python dependency management and packaging

**Platforms:** Linux, macOS, Windows

### [polaris](../../presets/polaris/README.md)

Validate Kubernetes configurations against policy-as-code best practices

**Platforms:** Linux, macOS, Windows

### [powershell](../../presets/powershell/README.md)

Cross-platform task automation and configuration management framework

**Platforms:** Linux, macOS, Windows

### [prefect](../../presets/prefect/README.md)

Workflow orchestration

**Platforms:** Linux, macOS, Windows

### [procs](../../presets/procs/README.md)

System monitoring utility

**Platforms:** Linux, macOS, Windows

### [proxychains](../../presets/proxychains/README.md)

Service mesh / proxy

**Platforms:** Multiple

### [pulsar](../../presets/pulsar/README.md)

Message queue / streaming

**Platforms:** Multiple

### [pylint](../../presets/pylint/README.md)

Python tool

**Platforms:** Multiple

### [pyroscope](../../presets/pyroscope/README.md)

Continuous profiling platform

**Platforms:** Multiple

### [pytorch](../../presets/pytorch/README.md)

Install PyTorch deep learning framework with CPU/GPU support

**Platforms:** Multiple

### [qdrant](../../presets/qdrant/README.md)

Graph / vector database

**Platforms:** Multiple

### [questdb](../../presets/questdb/README.md)

Time-series / analytics database

**Platforms:** Multiple

### [quickwit](../../presets/quickwit/README.md)

Cloud-native search engine for logs and analytics

**Platforms:** Linux, macOS, Windows

### [rabbitmq](../../presets/rabbitmq/README.md)

Install and configure RabbitMQ message broker

**Platforms:** Linux, macOS, Windows

### [rancher](../../presets/rancher/README.md)

Command-line interface for Rancher Kubernetes management

**Platforms:** Linux, macOS, Windows

### [rbenv](../../presets/rbenv/README.md)

Lightweight Ruby version manager for managing multiple Ruby versions

**Platforms:** Multiple

### [rclone](../../presets/rclone/README.md)

Cloud storage sync tool supporting 70+ providers

**Platforms:** Linux, macOS, Windows

### [restic](../../presets/restic/README.md)

Fast, secure backup program with encryption and deduplication

**Platforms:** Linux, macOS, Windows

### [riak](../../presets/riak/README.md)

Cache / storage system

**Platforms:** Multiple

### [rsync](../../presets/rsync/README.md)

Fast file synchronization and transfer tool with delta-transfer algorithm

**Platforms:** Linux, macOS, Windows

### [rtx](../../presets/rtx/README.md)

Polyglot runtime manager compatible with asdf (now called mise)

**Platforms:** Linux, macOS, Windows

### [ruff](../../presets/ruff/README.md)

Extremely fast Python linter and code formatter written in Rust

**Platforms:** Linux, macOS, Windows

### [rvm](../../presets/rvm/README.md)

Manage multiple Ruby environments with isolated gemsets

**Platforms:** Linux, macOS, Windows

### [rye](../../presets/rye/README.md)

Rust-powered Python package and project manager

**Platforms:** Linux, macOS, Windows

### [sbt](../../presets/sbt/README.md)

Interactive build tool for Scala and Java projects

**Platforms:** Linux, macOS, Windows

### [scaleway-cli](../../presets/scaleway-cli/README.md)

Manage Scaleway cloud resources from the terminal

**Platforms:** Linux, macOS, Windows

### [sccache](../../presets/sccache/README.md)

Rust/Go tool

**Platforms:** Multiple

### [scp](../../presets/scp/README.md)

File transfer / backup tool

**Platforms:** Multiple

### [scylladb](../../presets/scylladb/README.md)

Cache / storage system

**Platforms:** Multiple

### [sdkman](../../presets/sdkman/README.md)

Java/JVM tool

**Platforms:** Multiple

### [sentry-cli](../../presets/sentry-cli/README.md)

Sentry command-line client

**Platforms:** Multiple

### [sftp](../../presets/sftp/README.md)

File transfer / backup tool

**Platforms:** Multiple

### [shellcheck](../../presets/shellcheck/README.md)

Static analysis tool for shell scripts that catches bugs and enforces best practices

**Platforms:** Linux, macOS, Windows

### [shfmt](../../presets/shfmt/README.md)

Utility tool

**Platforms:** Multiple

### [signoz](../../presets/signoz/README.md)

Open-source observability platform

**Platforms:** Multiple

### [singer](../../presets/singer/README.md)

Data processing / ETL

**Platforms:** Multiple

### [snyk](../../presets/snyk/README.md)

Security scanning tool

**Platforms:** Multiple

### [socat](../../presets/socat/README.md)

Service mesh / proxy

**Platforms:** Multiple

### [sonic](../../presets/sonic/README.md)

Search / analytics engine

**Platforms:** Multiple

### [spark](../../presets/spark/README.md)

Data processing / ETL

**Platforms:** Multiple

### [sphinxsearch](../../presets/sphinxsearch/README.md)

Search / analytics engine

**Platforms:** Multiple

### [spinnaker](../../presets/spinnaker/README.md)

GitOps / CD tool

**Platforms:** Multiple

### [squid](../../presets/squid/README.md)

Service mesh / proxy

**Platforms:** Multiple

### [stack](../../presets/stack/README.md)

Language tool

**Platforms:** Linux, macOS, Windows

### [starship](../../presets/starship/README.md)

Install Starship - minimal, fast, and customizable cross-shell prompt

**Platforms:** Linux, macOS, Windows

### [stitch](../../presets/stitch/README.md)

Data processing / ETL

**Platforms:** Multiple

### [sublime](../../presets/sublime/README.md)

Text editor / IDE

**Platforms:** Multiple

### [swiftenv](../../presets/swiftenv/README.md)

Language tool

**Platforms:** Linux, macOS, Windows

### [syft](../../presets/syft/README.md)

Software Bill of Materials (SBOM) generation tool for containers and filesystems with multi-format support

**Platforms:** Linux, macOS, Windows

### [systat](../../presets/systat/README.md)

System monitoring utility

**Platforms:** Multiple

### [tekton-cli](../../presets/tekton-cli/README.md)

CI/CD tool

**Platforms:** Multiple

### [tensorflow](../../presets/tensorflow/README.md)

Install TensorFlow machine learning framework with CPU/GPU support

**Platforms:** Multiple

### [terragrunt](../../presets/terragrunt/README.md)

Terraform wrapper for keeping configurations DRY with remote state management and dependency handling

**Platforms:** Linux, macOS, Windows

### [terrascan](../../presets/terrascan/README.md)

Infrastructure as Code tool

**Platforms:** Multiple

### [tflint](../../presets/tflint/README.md)

Pluggable Terraform linter that catches errors, enforces best practices, and provides cloud-specific checks

**Platforms:** Linux, macOS, Windows

### [tfsec](../../presets/tfsec/README.md)

Static analysis security scanner for Terraform that finds security issues before deployment

**Platforms:** Linux, macOS, Windows

### [thanos](../../presets/thanos/README.md)

Time-series / analytics database

**Platforms:** Multiple

### [tide](../../presets/tide/README.md)

Modern Fish prompt

**Platforms:** Multiple

### [timescaledb](../../presets/timescaledb/README.md)

Time-series / analytics database

**Platforms:** Multiple

### [tinyproxy](../../presets/tinyproxy/README.md)

Service mesh / proxy

**Platforms:** Multiple

### [tmate](../../presets/tmate/README.md)

Utility tool

**Platforms:** Linux, macOS, Windows

### [traefik](../../presets/traefik/README.md)

Install Traefik modern reverse proxy and load balancer

**Platforms:** Multiple

### [transfer-sh](../../presets/transfer-sh/README.md)

File transfer / backup tool

**Platforms:** Multiple

### [trivy](../../presets/trivy/README.md)

Vulnerability scanner for containers

**Platforms:** Multiple

### [tsx](../../presets/tsx/README.md)

Node.js / JavaScript tool

**Platforms:** Linux, macOS, Windows

### [ttyd](../../presets/ttyd/README.md)

Utility tool

**Platforms:** Linux, macOS, Windows

### [tyk](../../presets/tyk/README.md)

API gateway / load balancer

**Platforms:** Multiple

### [typesense](../../presets/typesense/README.md)

Search / analytics engine

**Platforms:** Multiple

### [uptrace](../../presets/uptrace/README.md)

Distributed tracing and metrics

**Platforms:** Multiple

### [usql](../../presets/usql/README.md)

Universal SQL CLI

**Platforms:** Linux, macOS, Windows

### [uv](../../presets/uv/README.md)

Python tool

**Platforms:** Linux, macOS, Windows

### [valkey](../../presets/valkey/README.md)

Cache / storage system

**Platforms:** Linux, macOS, Windows

### [vegeta](../../presets/vegeta/README.md)

HTTP load testing tool and library

**Platforms:** Multiple

### [victoriametrics](../../presets/victoriametrics/README.md)

Time-series database and monitoring

**Platforms:** Multiple

### [victoriametrics-single](../../presets/victoriametrics-single/README.md)

Time-series / analytics database

**Platforms:** Multiple

### [vite](../../presets/vite/README.md)

Install Vite next-generation frontend build tool

**Platforms:** Linux, macOS, Windows

### [volta](../../presets/volta/README.md)

Install Volta JavaScript tool manager for Node.js version management

**Platforms:** Linux, macOS, Windows

### [vscodium](../../presets/vscodium/README.md)

Text editor / IDE

**Platforms:** Multiple

### [vuepress](../../presets/vuepress/README.md)

Install VuePress Vue-powered static site generator

**Platforms:** Linux, macOS, Windows

### [vultr-cli](../../presets/vultr-cli/README.md)

Install Vultr CLI for managing cloud infrastructure

**Platforms:** Linux, macOS, Windows

### [watchexec](../../presets/watchexec/README.md)

Install watchexec file watcher that executes commands on changes

**Platforms:** Linux, macOS, Windows

### [weaviate](../../presets/weaviate/README.md)

Graph / vector database

**Platforms:** Multiple

### [werf](../../presets/werf/README.md)

Install werf GitOps delivery tool for Kubernetes

**Platforms:** Linux, macOS, Windows

### [wezterm](../../presets/wezterm/README.md)

GPU-accelerated cross-platform terminal emulator with rich configuration and multiplexer capabilities

**Platforms:** Multiple

### [woodpecker-cli](../../presets/woodpecker-cli/README.md)

Install Woodpecker CI command-line tool

**Platforms:** Linux, macOS, Windows

### [wrk](../../presets/wrk/README.md)

Modern HTTP benchmarking tool capable of generating significant load with multi-threaded design

**Platforms:** Multiple

### [xh](../../presets/xh/README.md)

Fast HTTP client in Rust, httpie-compatible with additional features like downloads and sessions

**Platforms:** Linux, macOS, Windows

### [xsv](../../presets/xsv/README.md)

Fast CSV command-line toolkit in Rust for indexing, slicing, analyzing, splitting and joining CSV files

**Platforms:** Multiple

### [yamllint](../../presets/yamllint/README.md)

Install yamllint linter for YAML files

**Platforms:** Linux, macOS, Windows

### [yarn](../../presets/yarn/README.md)

Install Yarn package manager for JavaScript projects

**Platforms:** Linux, macOS, Windows

### [ytop](../../presets/ytop/README.md)

TUI system monitor written in Rust showing CPU, memory, network, and disk usage

**Platforms:** Linux, macOS, Windows

### [z](../../presets/z/README.md)

Jump to frequently used directories by partial name, tracks your directory history

**Platforms:** Linux, macOS, Windows

### [zenith](../../presets/zenith/README.md)

Cross-platform system monitoring tool with process viewer and interactive charts

**Platforms:** Linux, macOS, Windows

### [zig](../../presets/zig/README.md)

General-purpose programming language and toolchain for maintaining robust, optimal, and reusable software

**Platforms:** Linux, macOS, Windows

### [zinc](../../presets/zinc/README.md)

Lightweight alternative to Elasticsearch with full-text search and aggregations

**Platforms:** Linux, macOS, Windows

### [zookeeper](../../presets/zookeeper/README.md)

Message queue / streaming

**Platforms:** Multiple

### [zoxide](../../presets/zoxide/README.md)

Smarter cd command that remembers frequently used directories and enables quick navigation with partial names

**Platforms:** Multiple



---

<!-- FILE: presets/style-guide.md -->

# The Definitive Mooncake Preset Style Guide

**Version**: 1.0.0
**Last Updated**: 2026-02-06
**Purpose**: Production-ready standards for creating high-quality, consistent Mooncake presets

This guide defines the gold standard for creating Mooncake presets. Following these patterns ensures presets are discoverable, maintainable, and provide excellent user experience for both humans and AI agents.

---

## Table of Contents

1. [Philosophy & Principles](#philosophy--principles)
2. [Preset Structure](#preset-structure)
3. [Documentation Standards](#documentation-standards)
4. [Parameter Design](#parameter-design)
5. [Task Organization](#task-organization)
6. [Platform Handling](#platform-handling)
7. [Idempotency & Change Detection](#idempotency--change-detection)
8. [Error Handling](#error-handling)
9. [Testing & Validation](#testing--validation)
10. [Examples & Templates](#examples--templates)
11. [Checklist](#checklist)

---

## Philosophy & Principles

### Core Values

**1. Simplicity First**
- Presets should make complex operations simple, not simple operations complex
- Minimize required parameters, maximize sensible defaults
- A basic installation should be ONE command: `preset: tool-name`

**2. Copy-Paste Ready**
- Every example must work without modification
- No placeholders without clear substitution instructions
- Provide complete, working configurations

**3. Production Grade**
- Assume presets run on real infrastructure
- Include error handling, validation, and safety checks
- Document security implications and best practices

**4. AI-Agent Friendly**
- Structure documentation for both human and LLM consumption
- Include "Agent Use" sections describing automation use cases
- Provide machine-readable success criteria

**5. Discoverability**
- Users should understand what a preset does in 10 seconds
- Quick Start section must come first
- Common operations clearly documented

---

## Preset Structure

### Directory Layout

For complex presets (with templates, multiple task files):

```
presets/
└── tool-name/
    ├── preset.yml              # Main preset definition (orchestration)
    ├── README.md               # User-facing documentation
    ├── tasks/                  # Modular task files
    │   ├── install.yml        # Installation logic
    │   ├── configure.yml      # Service/config setup
    │   ├── verify.yml         # Health checks (optional)
    │   └── uninstall.yml      # Cleanup tasks
    ├── templates/              # Configuration templates
    │   ├── service.conf.j2    # Service configs
    │   └── config.yml.j2      # App configs
    └── files/                  # Static files (optional)
        └── defaults.conf
```

For simple presets (single action, minimal logic):

```
presets/
└── tool-name.yml              # Flat format - all in one file
```

**When to use directory format:**
- Tool requires service configuration (systemd/launchd)
- Multiple installation methods (package manager, script, source)
- Platform-specific logic (Linux vs macOS vs Windows)
- Template files needed for configuration
- More than 50 lines of preset logic

**When to use flat format:**
- Simple package installation (single command)
- No service configuration
- Minimal platform differences
- No templates or additional files

### preset.yml Structure

```yaml
name: tool-name
description: One-line description of what this preset does
version: 1.0.0

parameters:
  state:
    type: string
    required: false
    default: present
    enum: [present, absent]
    description: Whether tool should be installed or removed

  # Additional parameters...

steps:
  # Use include for complex presets
  - name: Install tool
    include: tasks/install.yml
    when: parameters.state == "present"

  - name: Configure service
    include: tasks/configure.yml
    when: parameters.state == "present" and parameters.service

  - name: Uninstall tool
    include: tasks/uninstall.yml
    when: parameters.state == "absent"
```

**Naming Conventions:**
- **Preset name**: Use tool's official name (lowercase, hyphens for multi-word)
  - ✅ `kubectl`, `helm`, `modern-unix`
  - ❌ `kube-ctl`, `Helm`, `modern_unix`
- **Task files**: Action-oriented, lowercase
  - `install.yml`, `configure.yml`, `uninstall.yml`, `verify.yml`
- **Templates**: Descriptive with `.j2` extension
  - `systemd-service.conf.j2`, `config.yml.j2`

---

## Documentation Standards

### README.md Structure

Every preset MUST have a README.md with these sections in this order:

```markdown
# Tool Name - Brief Description

One-sentence description of what this tool does.

## Quick Start
```yaml
- preset: tool-name
```

## Features
- Feature 1
- Feature 2
- Feature 3

## Basic Usage
```bash
# Most common commands with actual examples
tool-name --version
tool-name command arg
```

## Advanced Configuration
```yaml
- preset: tool-name
  with:
    param1: value1
    param2: value2
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove |

## Platform Support
- ✅ Linux (apt, dnf, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not supported)

## Configuration
- **Config file**: `/path/to/config`
- **Data directory**: `/path/to/data`
- **Port**: 8080

## Real-World Examples
Practical use cases showing tool in context

## Agent Use
How AI agents can leverage this tool:
- Use case 1
- Use case 2

## Uninstall
```yaml
- preset: tool-name
  with:
    state: absent
```

## Resources
- Official docs: https://...
- Search: "tool-name tutorial"
```

### Section Guidelines

#### 1. Quick Start (REQUIRED)
- **First code block users see**
- **Must work without modification**
- **Shows simplest possible usage**

```markdown
## Quick Start
```yaml
- preset: kubectl
```
```

#### 2. Features (REQUIRED)
- **Bullet list of key capabilities**
- **Focus on value, not implementation**
- **3-8 items maximum**

```markdown
## Features
- **Cross-platform**: Linux, macOS, BSD
- **Fast**: Written in Rust, minimal overhead
- **Beautiful**: Color-coded output with icons
- **Smart**: Respects .gitignore by default
```

#### 3. Basic Usage (REQUIRED)
- **Real commands users will run**
- **Common operations first**
- **Include expected output when helpful**

```markdown
## Basic Usage
```bash
# Check version
kubectl version --client

# List pods
kubectl get pods

# Create deployment
kubectl create deployment nginx --image=nginx
```
```

#### 4. Advanced Configuration (REQUIRED)
- **Show all parameter options**
- **Real working examples, not just parameter lists**
- **Group related parameters**

```markdown
## Advanced Configuration
```yaml
- preset: kubectl
  with:
    version: "1.29.0"              # Specific version
    configure_completion: true     # Shell completion
    install_krew: true             # Plugin manager
    krew_plugins:
      - ctx                        # Context switcher
      - ns                         # Namespace switcher
```
```

#### 5. Real-World Examples (HIGHLY RECOMMENDED)
- **Show tool in realistic scenarios**
- **Complete workflows, not isolated commands**
- **Include context (why you'd do this)**

```markdown
## Real-World Examples

### CI/CD Pipeline
```bash
# Check disk space before build
if duf --json / | jq '.[0].usage' | awk '$1 > 90 {exit 1}'; then
  echo "ERROR: Disk usage above 90%"
  exit 1
fi
```

### Development Workflow
```bash
# Extract API response field
curl https://api.example.com/data | jq '.users[].email'
```
```

#### 6. Agent Use (REQUIRED)
- **How AI agents can use this tool**
- **Automation-friendly use cases**
- **Decision criteria for agents**

```markdown
## Agent Use
- Parse and extract data from JSON APIs
- Transform configuration files in deployment pipelines
- Filter and aggregate log data
- Validate API responses in test suites
- Generate reports from structured data
```

#### 7. Configuration (RECOMMENDED)
- **File locations (absolute paths)**
- **Default ports/addresses**
- **Directory permissions**
- **Environment variables**

```markdown
## Configuration
- **Config file**: `~/.config/tool/config.yml` (Linux), `~/Library/Preferences/tool/config.yml` (macOS)
- **Data directory**: `~/.local/share/tool/` (Linux), `~/Library/Application Support/tool/` (macOS)
- **Cache**: `~/.cache/tool/` (Linux), `~/Library/Caches/tool/` (macOS)
- **Default port**: 8080
```

#### 8. Platform Support (REQUIRED)
- **Clear matrix of what works where**
- **Use ✅ ❌ symbols for clarity**
- **Note installation methods**

```markdown
## Platform Support
- ✅ Linux (systemd, apt, dnf, yum, pacman, zypper)
- ✅ macOS (launchd, Homebrew)
- ❌ Windows (not yet supported)
```

#### 9. Troubleshooting (RECOMMENDED)
- **Common issues and solutions**
- **How to check logs**
- **Debug mode instructions**

```markdown
## Troubleshooting

### Service won't start
Check logs:
```bash
journalctl -u service-name -f  # Linux
tail -f ~/Library/Logs/service.log  # macOS
```

### Permission errors
Most operations require `become: true` (sudo).
```

#### 10. Resources (REQUIRED)
- **Official documentation link**
- **Search suggestions (for AI agents)**
- **Community resources**

```markdown
## Resources
- Official docs: https://tool.example.com/docs/
- GitHub: https://github.com/org/tool
- Search: "tool-name tutorial", "tool-name best practices"
```

### Writing Style

**DO:**
- ✅ Use active voice ("Install Docker" not "Docker installation")
- ✅ Write concise descriptions (one sentence per bullet)
- ✅ Include concrete examples, not abstract descriptions
- ✅ Use consistent terminology throughout
- ✅ Add context to code blocks (what it does, when to use)

**DON'T:**
- ❌ Write marketing copy ("the best tool", "amazing")
- ❌ Use vague placeholders (`<your-value>` without guidance)
- ❌ Assume prior knowledge (explain domain-specific terms)
- ❌ Include incomplete examples
- ❌ Copy-paste from tool's docs without adaptation

### Code Block Standards

**Always include:**
- Language identifier (yaml, bash, python, etc.)
- Context comment (what this does, when to use)
- Complete working example

```markdown
## Example
```yaml
# Production deployment with custom settings
- preset: myapp
  with:
    environment: production
    replicas: 3
    enable_monitoring: true
  become: true
```
```

**DON'T:**
```markdown
## Example
```
preset: myapp
  with:
    environment: <your-env>  # ❌ Placeholder without guidance
```
```

---

## Parameter Design

### Standard Parameters

**Every preset SHOULD support:**

```yaml
parameters:
  state:
    type: string
    required: false
    default: present
    enum: [present, absent]
    description: Whether tool should be installed or removed
```

### Parameter Naming Conventions

**Standard names** (use these for consistency):
- `state`: Installation state (present/absent)
- `version`: Specific version to install
- `service`: Enable as system service (bool)
- `configure`: Run configuration steps (bool)
- `force`: Force reinstall/reconfigure (bool)
- `method`: Installation method (auto/package/script)
- `port`: Network port number
- `host`: Bind address
- `data_dir`: Data storage location
- `config_file`: Configuration file path

**Naming rules:**
- Use snake_case (not camelCase)
- Be specific: `database_url` not `url`
- Avoid abbreviations: `configuration` not `cfg`
- Use singular for single values: `port` not `ports`
- Use plural for arrays: `models` not `model`

### Parameter Types

```yaml
parameters:
  # String - text values
  environment:
    type: string
    enum: [development, staging, production]
    default: development
    description: Deployment environment

  # Boolean - yes/no flags
  enable_monitoring:
    type: bool
    default: false
    description: Enable Prometheus metrics endpoint

  # Array - lists
  features:
    type: array
    default: []
    description: List of feature flags to enable

  # Object - structured data
  config:
    type: object
    required: false
    description: Additional configuration options
```

### Default Values Strategy

**Principle**: Defaults should work for 80% of users.

```yaml
# ✅ Good - sensible production defaults
parameters:
  port:
    type: number
    default: 8080
    description: Application port

  workers:
    type: number
    default: 4
    description: Number of worker processes

  log_level:
    type: string
    default: info
    enum: [debug, info, warn, error]
    description: Logging verbosity

# ❌ Bad - forces users to specify everything
parameters:
  port:
    type: number
    required: true  # Why? 8080 is fine for most
    description: Application port
```

### Parameter Validation

**Use enum for limited choices:**
```yaml
parameters:
  state:
    type: string
    enum: [present, absent]  # Only these values allowed
    description: Installation state
```

**Document valid ranges:**
```yaml
parameters:
  port:
    type: number
    default: 8080
    description: Application port (1024-65535)
```

**Describe format requirements:**
```yaml
parameters:
  version:
    type: string
    default: latest
    description: Version to install (e.g., '1.2.3', 'latest')
```

### Required vs Optional

**Make required ONLY when:**
- No sensible default exists
- Value is user-specific (API keys, hostnames)
- Incorrect guess would be dangerous

**Examples:**

```yaml
# ✅ Optional with default - most users want service
parameters:
  service:
    type: bool
    default: true
    description: Enable and start system service

# ✅ Required - no safe default
parameters:
  database_password:
    type: string
    required: true
    description: Database password for application

# ❌ Bad - has obvious default
parameters:
  install:
    type: bool
    required: true  # Just default to true!
    description: Whether to install
```

---

## Task Organization

### Task File Structure

**Principle**: One file per logical phase

```yaml
# tasks/install.yml - Installation logic only
- name: Check if tool exists
  shell: command -v tool-name
  register: check
  failed_when: false

- name: Install via package manager
  shell: apt-get install -y tool-name
  when: apt_available and check.rc != 0
  become: true

- name: Install via Homebrew
  shell: brew install tool-name
  when: brew_available and check.rc != 0

- name: Install via script
  shell: curl -fsSL https://get.tool.sh | sh
  when: check.rc != 0 and not (apt_available or brew_available)
  become: true
```

### Task File Guidelines

**install.yml:**
- Platform detection
- Multiple installation methods with fallback
- Idempotency (check if already installed)
- Exit early if already present

**configure.yml:**
- Create configuration files
- Set up service files (systemd/launchd)
- Apply configuration changes
- Restart services if needed

**verify.yml** (optional):
- Health checks
- Connectivity tests
- Version verification
- Configuration validation

**uninstall.yml:**
- Stop services
- Remove binaries
- Clean up configuration (optional)
- Remove data directories (only with force: true)

### Step Naming

**Template:**
```
[Action verb] [object] [context]
```

**Examples:**
```yaml
# ✅ Good - clear action and object
- name: Install kubectl binary
- name: Configure systemd service
- name: Pull Docker image
- name: Create config directory
- name: Stop running service

# ❌ Bad - vague or passive
- name: Installation  # What's being installed?
- name: Setup  # Too vague
- name: The service is configured  # Passive voice
```

### Conditional Logic Patterns

**Platform detection:**
```yaml
# ✅ Use system facts
- name: Install via apt
  shell: apt-get install -y tool
  when: apt_available
  become: true

- name: Install via Homebrew
  shell: brew install tool
  when: brew_available

# ❌ Don't hardcode OS checks
- name: Install on Linux
  shell: apt-get install -y tool
  when: os == "linux"  # Not all Linux has apt!
```

**Parameter-based:**
```yaml
- name: Configure service
  include: tasks/service.yml
  when: parameters.service == true

- name: Pull models
  include: tasks/models.yml
  when: parameters.models | length > 0
```

**State-based:**
```yaml
- name: Install workflow
  include: tasks/install.yml
  when: parameters.state == "present"

- name: Uninstall workflow
  include: tasks/uninstall.yml
  when: parameters.state == "absent"
```

---

## Platform Handling

### Detection Strategy

**Use system facts, not OS checks:**

```yaml
# ✅ Good - specific capability detection
- name: Install via apt
  shell: apt-get install -y {{ tool }}
  when: apt_available
  become: true

- name: Install via dnf
  shell: dnf install -y {{ tool }}
  when: dnf_available
  become: true

- name: Install via Homebrew
  shell: brew install {{ tool }}
  when: brew_available

# ❌ Bad - broad OS checks
- name: Install on Linux
  shell: apt-get install -y {{ tool }}  # Assumes apt!
  when: os == "linux"
  become: true
```

### Available Facts

**Package managers:**
- `apt_available` (Debian, Ubuntu)
- `dnf_available` (Fedora, RHEL 8+)
- `yum_available` (CentOS, RHEL 7)
- `pacman_available` (Arch)
- `zypper_available` (openSUSE)
- `apk_available` (Alpine)
- `brew_available` (macOS, Linux)
- `port_available` (macOS)

**Operating systems:**
- `os` ("linux", "darwin", "windows")
- `arch` ("amd64", "arm64")
- `hostname`

**System info:**
- `cpu_cores`
- `memory_total_mb`
- `distribution` (Linux only: "ubuntu", "fedora", etc.)

### Service Configuration

**systemd (Linux):**
```yaml
- name: Configure systemd service
  service:
    name: myapp
    state: started
    enabled: true
    unit:
      content: |
        [Unit]
        Description={{ parameters.description }}
        After=network.target

        [Service]
        Type=simple
        ExecStart=/usr/local/bin/myapp
        Restart=always
        Environment="PORT={{ parameters.port }}"

        [Install]
        WantedBy=multi-user.target
  when: os == "linux"
  become: true
```

**launchd (macOS):**
```yaml
- name: Configure launchd service
  service:
    name: com.example.myapp
    state: started
    enabled: true
    unit:
      content: |
        <?xml version="1.0" encoding="UTF-8"?>
        <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
          "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
        <plist version="1.0">
        <dict>
          <key>Label</key>
          <string>com.example.myapp</string>
          <key>ProgramArguments</key>
          <array>
            <string>/usr/local/bin/myapp</string>
          </array>
          <key>RunAtLoad</key>
          <true/>
          <key>KeepAlive</key>
          <true/>
          <key>EnvironmentVariables</key>
          <dict>
            <key>PORT</key>
            <string>{{ parameters.port }}</string>
          </dict>
        </dict>
        </plist>
  when: os == "darwin"
```

### Installation Method Hierarchy

**Preferred order:**
1. Package manager (most reliable, gets updates)
2. Official installation script (maintained by tool vendor)
3. Binary download (with checksum verification)
4. Source compilation (last resort)

```yaml
- name: Try package manager install
  shell: "{{ package_manager }} install -y {{ tool }}"
  when: package_manager_available
  register: pkg_install
  failed_when: false
  become: true

- name: Fall back to official script
  shell: curl -fsSL https://get.tool.sh | sh
  when: pkg_install.rc != 0 or not package_manager_available
  become: true
```

---

## Idempotency & Change Detection

### Principle

**Every step should be safe to run multiple times.**

First run: Makes changes, reports `changed: true`
Second run: No changes needed, reports `changed: false`

### Check Before Action

```yaml
# ✅ Good - idempotent
- name: Check if tool installed
  shell: command -v tool-name
  register: check
  failed_when: false

- name: Install tool
  shell: curl -fsSL https://get.tool.sh | sh
  when: check.rc != 0
  become: true

# ❌ Bad - always runs, always reports changed
- name: Install tool
  shell: curl -fsSL https://get.tool.sh | sh
  become: true
```

### Use Built-in Idempotency

```yaml
# file action is idempotent
- name: Create directory
  file:
    path: /opt/myapp
    state: directory
    mode: "0755"

# download with checksum is idempotent
- name: Download binary
  download:
    url: https://example.com/tool-v1.2.3
    dest: /usr/local/bin/tool
    checksum: "sha256:abc123..."
    mode: "0755"

# service action is idempotent
- name: Start service
  service:
    name: myapp
    state: started
    enabled: true
```

### Marker Files

```yaml
# Use 'creates' for one-time operations
- name: Initialize database
  shell: pg_restore backup.sql
  creates: /var/lib/postgresql/.initialized
  become: true

# Use marker files for complex operations
- name: Run one-time setup
  shell: |
    # Complex multi-step setup
    ./setup.sh
    touch /opt/myapp/.setup-complete
  creates: /opt/myapp/.setup-complete
```

### Changed Detection

```yaml
# Override changed status based on output
- name: Pull Docker image
  shell: docker pull nginx:latest
  register: pull
  changed_when: "'Downloaded' in pull.stdout"

# Check if update is needed
- name: Update git repository
  shell: git pull
  cwd: /opt/repo
  register: git_pull
  changed_when: "'Already up to date' not in git_pull.stdout"
```

---

## Error Handling

### Validation

**Validate early:**
```yaml
# Validate required files exist
- name: Verify config file exists
  assert:
    file:
      path: "{{ parameters.config_file }}"
      exists: true
  when: parameters.config_file is defined

# Validate connectivity
- name: Check database connection
  shell: pg_isready -h {{ db_host }}
  when: parameters.verify_connection

# Validate version format
- name: Check version format
  shell: echo "{{ parameters.version }}" | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$'
  when: parameters.version != "latest"
```

### Graceful Degradation

```yaml
# Try preferred method, fall back gracefully
- name: Try package manager install
  shell: apt-get install -y tool
  when: apt_available
  register: apt_result
  failed_when: false
  become: true

- name: Fall back to script install
  shell: curl -fsSL https://get.tool.sh | sh
  when: apt_result.rc != 0 or not apt_available
  become: true
```

### Clear Error Messages

```yaml
# ✅ Good - helpful error message
- name: Check prerequisites
  assert:
    command:
      cmd: docker --version
      exit_code: 0
  register: docker_check
  failed_when: docker_check.rc != 0
  # User sees: "assertion failed (command): expected exit code 0, got exit code 127"

# ❌ Bad - cryptic failure
- name: Setup
  shell: ./setup.sh  # Fails with no context
```

### Cleanup on Failure

```yaml
# Use register + conditional cleanup
- name: Download archive
  download:
    url: https://example.com/tool.tar.gz
    dest: /tmp/tool.tar.gz
  register: download_result

- name: Extract archive
  unarchive:
    src: /tmp/tool.tar.gz
    dest: /opt/tool
    strip_components: 1

- name: Cleanup download
  file:
    path: /tmp/tool.tar.gz
    state: absent
  when: download_result.changed
```

---

## Testing & Validation

### Dry Run Testing

**Every preset MUST work with `--dry-run`:**

```bash
# Test preset without making changes
mooncake run -c test.yml --dry-run

# Verify output shows intended actions
```

### Multi-Platform Testing

**Test matrix:**
- [ ] Ubuntu 22.04 (apt)
- [ ] Fedora 39 (dnf)
- [ ] macOS 14 (Homebrew)
- [ ] Arch Linux (pacman) - if claiming support

### Idempotency Testing

```bash
# Run preset twice - second run should report no changes
mooncake run -c test.yml
mooncake run -c test.yml  # Should show changed: false
```

### Verification Steps

**Include verification in preset:**
```yaml
- name: Verify installation
  assert:
    command:
      cmd: tool-name --version
      exit_code: 0

- name: Verify service running
  assert:
    command:
      cmd: systemctl is-active tool-service
      exit_code: 0
  when: parameters.service and os == "linux"

- name: Verify API responding
  assert:
    http:
      url: "http://localhost:{{ parameters.port }}/health"
      status: 200
  when: parameters.service
```

### Test Playbook Template

```yaml
# test-preset.yml
- name: Test basic installation
  preset: my-tool
  become: true

- name: Verify installed
  shell: command -v my-tool
  register: check

- name: Test with all options
  preset: my-tool
  with:
    version: "1.2.3"
    service: true
    configure: true
  become: true

- name: Verify service running
  shell: systemctl is-active my-tool
  when: os == "linux"

- name: Test uninstall
  preset: my-tool
  with:
    state: absent
  become: true

- name: Verify removed
  shell: command -v my-tool
  register: removed
  failed_when: removed.rc == 0
```

---

## Examples & Templates

### Simple Tool Preset Template

```yaml
# presets/simple-tool.yml
name: simple-tool
description: Install simple-tool CLI utility
version: 1.0.0

parameters:
  state:
    type: string
    default: present
    enum: [present, absent]
    description: Install or remove tool

steps:
  # Check if installed
  - name: Check if tool exists
    shell: command -v simple-tool
    register: check
    failed_when: false

  # Installation
  - name: Install via apt
    shell: apt-get install -y simple-tool
    when: parameters.state == "present" and apt_available and check.rc != 0
    become: true

  - name: Install via brew
    shell: brew install simple-tool
    when: parameters.state == "present" and brew_available and check.rc != 0

  # Uninstallation
  - name: Uninstall via apt
    shell: apt-get remove -y simple-tool
    when: parameters.state == "absent" and apt_available
    become: true

  - name: Uninstall via brew
    shell: brew uninstall simple-tool
    when: parameters.state == "absent" and brew_available
```

### Complex Preset Template

```yaml
# presets/complex-tool/preset.yml
name: complex-tool
description: Install and configure complex-tool with service management
version: 1.0.0

parameters:
  state:
    type: string
    default: present
    enum: [present, absent]
    description: Install or remove tool

  version:
    type: string
    default: latest
    description: Version to install

  service:
    type: bool
    default: true
    description: Configure as system service

  port:
    type: number
    default: 8080
    description: Service port (1024-65535)

  data_dir:
    type: string
    required: false
    description: Custom data directory

steps:
  - name: Install complex-tool
    include: tasks/install.yml
    when: parameters.state == "present"

  - name: Configure service
    include: tasks/configure.yml
    when: parameters.state == "present" and parameters.service

  - name: Verify installation
    include: tasks/verify.yml
    when: parameters.state == "present"

  - name: Uninstall complex-tool
    include: tasks/uninstall.yml
    when: parameters.state == "absent"
```

### README Template

```markdown
# Tool Name - One-Line Description

Brief paragraph describing what this tool does and why it's useful.

## Quick Start
```yaml
- preset: tool-name
```

## Features
- **Feature 1**: Description
- **Feature 2**: Description
- **Feature 3**: Description
- **Cross-platform**: Linux, macOS

## Basic Usage
```bash
# Most common operation
tool-name command

# Second most common
tool-name other-command arg
```

## Advanced Configuration
```yaml
- preset: tool-name
  with:
    version: "1.2.3"
    option1: value1
    option2: true
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove |
| version | string | latest | Version to install |
| option1 | string | - | Description |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not supported)

## Configuration
- **Config file**: `/etc/tool/config.yml`
- **Data directory**: `/var/lib/tool/`
- **Port**: 8080

## Real-World Examples

### Use Case 1
```bash
# Context and explanation
commands here
```

### Use Case 2
```yaml
# Mooncake example
- preset: tool-name
  with:
    specific: configuration
```

## Agent Use
- Use case for AI agents
- Another automation scenario
- Integration pattern

## Troubleshooting

### Problem 1
Solution:
```bash
command to fix
```

### Problem 2
Solution and explanation.

## Uninstall
```yaml
- preset: tool-name
  with:
    state: absent
```

## Resources
- Official docs: https://tool.example.com/docs
- GitHub: https://github.com/org/tool
- Search: "tool-name tutorial", "tool-name examples"
```

---

## Checklist

### Before Submitting Preset

**Structure:**
- [ ] Directory structure follows conventions
- [ ] All files in correct locations
- [ ] No unnecessary files included

**preset.yml:**
- [ ] Name matches directory/filename
- [ ] Description is clear and concise (one line)
- [ ] Version follows semver (1.0.0)
- [ ] All parameters have descriptions
- [ ] Parameters use consistent naming (snake_case)
- [ ] Defaults are sensible for common use
- [ ] Steps are well-organized (install/configure/uninstall)
- [ ] Conditionals use system facts (not hardcoded OS checks)

**README.md:**
- [ ] Quick Start section comes first
- [ ] Quick Start example works without modification
- [ ] Features section lists 3-8 key capabilities
- [ ] Basic Usage shows real commands
- [ ] Advanced Configuration has working examples
- [ ] Parameters table is complete and accurate
- [ ] Platform Support clearly lists OS/package managers
- [ ] Configuration section lists file locations
- [ ] Agent Use section describes automation scenarios
- [ ] Uninstall instructions provided
- [ ] Resources include official docs and search terms

**Code Quality:**
- [ ] All steps have descriptive names
- [ ] No hardcoded values (use parameters)
- [ ] Idempotency: runs safely multiple times
- [ ] Error handling: graceful failures
- [ ] Platform detection: uses facts not OS checks
- [ ] Templates use .j2 extension
- [ ] Task files use action-oriented names

**Testing:**
- [ ] Tested with `--dry-run`
- [ ] Tested on at least one Linux distro
- [ ] Tested on macOS (if claiming support)
- [ ] Run twice - second run reports no changes
- [ ] Uninstall tested and verified
- [ ] All examples in README tested

**Documentation:**
- [ ] No typos or grammatical errors
- [ ] Code blocks have language identifiers
- [ ] All examples are complete and working
- [ ] No dead links
- [ ] Search terms provided for AI agents

---

## Version History

**1.0.0** (2026-02-06)
- Initial comprehensive style guide
- Consolidated patterns from 16 production presets
- Added templates and checklists
- Defined documentation standards

---

## Contributing

This guide is a living document. When you create a great preset that establishes a new pattern, update this guide with that pattern.

**To propose changes:**
1. Create example preset demonstrating the pattern
2. Document the pattern with rationale
3. Update relevant sections of this guide
4. Submit for review

**Principles for guide updates:**
- Patterns must be proven in production presets
- Keep the guide concise - quality over quantity
- Examples must be complete and tested
- Optimize for both human and AI comprehension


---

<!-- FILE: testing/README.md -->

# Mooncake Testing Documentation

Complete testing documentation for the Mooncake multi-platform testing infrastructure.

## 📚 Documentation Guide

### For Getting Started

**[Quick Reference](quick-reference.md)** - Start here for common commands
- One-line commands for daily use
- Quick examples and usage patterns
- Troubleshooting quick fixes
- Perfect for daily development

### For Understanding the System

**[Testing Guide](guide.md)** - Complete testing guide
- Overview of the testing setup
- Detailed instructions for local and CI testing
- Test structure and organization
- Platform-specific notes (Linux, macOS, Windows)
- Comprehensive troubleshooting section
- Best practices

**[Architecture](architecture.md)** - System architecture and design
- Visual diagrams of the testing infrastructure
- How components work together
- Data flow and execution paths
- Platform coverage matrix
- Design decisions explained

### For Implementation Details

**[Implementation Summary](implementation-summary.md)** - What was built
- Complete list of files created and modified
- Phase-by-phase implementation breakdown
- Success criteria checklist
- Known limitations and trade-offs

## Quick Start

```bash
# Fast smoke test (2 minutes)
make test-quick

# Test all Linux distros (10 minutes)
make test-smoke

# Complete local test suite (15 minutes)
make test-all-platforms
```

## 📖 Documentation Map

```
Testing Documentation Structure:
├── README.md (this file)           # Documentation index
├── quick-reference.md              # Quick commands and examples
├── guide.md                        # Complete testing guide
├── architecture.md                 # System architecture
└── implementation-summary.md       # Implementation details
```

## Find What You Need

| I want to... | Read this... |
|--------------|--------------|
| Run tests quickly | [Quick Reference](quick-reference.md) |
| Understand the full setup | [Testing Guide](guide.md) |
| See how it works | [Architecture](architecture.md) |
| Know what was implemented | [Implementation Summary](implementation-summary.md) |
| Add new tests | [Testing Guide - Adding Tests](guide.md#adding-new-tests) |
| Troubleshoot issues | [Testing Guide - Troubleshooting](guide.md#troubleshooting) |
| Understand design decisions | [Architecture - Design Decisions](architecture.md#key-design-decisions) |

## 🌍 Platform Coverage

- **Linux**: Ubuntu 22.04/20.04, Alpine 3.19, Debian 12, Fedora 39
- **macOS**: Intel (macos-13) + Apple Silicon (macos-latest)
- **Windows**: Windows Server (GitHub Actions)

## Key Commands

```bash
# Quick validation
make test              # Unit tests (10 sec)
make test-quick        # Smoke test on Ubuntu (2 min)

# Linux testing
make test-docker-ubuntu    # Test on Ubuntu
make test-docker-alpine    # Test on Alpine
make test-smoke            # Smoke tests all distros (10 min)

# Complete testing
make test-all-platforms    # Native + Docker (15 min)

# Verification
./scripts/verify-testing-setup.sh    # Verify setup
```

## 🔗 External Links

- [Main README](../../README.md)
- [GitHub Actions Workflow](../../.github/workflows/ci.yml)
- [Test Fixtures](../../testing/fixtures/)
- [Test Scripts](../../scripts/)

## Next Steps

1. **New to testing?** Start with [Quick Reference](quick-reference.md)
2. **Setting up?** Read [Testing Guide](guide.md)
3. **Want to understand?** Check [Architecture](architecture.md)
4. **Need implementation details?** See [Implementation Summary](implementation-summary.md)

---

**Last Updated**: 2026-02-05
**Status**:  Fully Implemented and Tested


---

<!-- FILE: testing/architecture.md -->

# Mooncake Testing Architecture

> 📚 **Other Docs**: [Index](README.md) | [Quick Reference](quick-reference.md) | [Testing Guide](guide.md) | [Implementation](implementation-summary.md)

## Overview Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    Mooncake Test Infrastructure                  │
└─────────────────────────────────────────────────────────────────┘

┌──────────────────────────┐  ┌──────────────────────────────────┐
│   LOCAL DEVELOPMENT      │  │        CI/CD (GitHub)            │
└──────────────────────────┘  └──────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                      Makefile Commands                           │
│  ┌───────────┐  ┌──────────┐  ┌────────────┐  ┌──────────────┐ │
│  │make test  │  │make test-│  │make test-  │  │make test-all-│ │
│  │           │  │quick     │  │docker-all  │  │platforms     │ │
│  └─────┬─────┘  └────┬─────┘  └──────┬─────┘  └──────┬───────┘ │
└────────┼─────────────┼───────────────┼────────────────┼─────────┘
         │             │               │                │
         ▼             ▼               ▼                ▼
┌─────────────┐  ┌──────────────────────────────────────────────┐
│   Native    │  │           Docker Test Matrix                 │
│  Go Tests   │  ├──────────┬──────────┬──────────┬─────────────┤
│  (10 sec)   │  │ Ubuntu   │ Alpine   │ Debian   │   Fedora    │
└─────────────┘  │ 22.04    │ 3.19     │ 12       │   39        │
                 │ 20.04    │          │          │             │
                 └──────────┴──────────┴──────────┴─────────────┘
                        │          │          │          │
                        └──────────┴──────────┴──────────┘
                                   │
                        ┌──────────▼──────────┐
                        │  test-runner.sh     │
                        │  ┌────────────────┐ │
                        │  │ Smoke Tests    │ │
                        │  │ Integration    │ │
                        │  └────────────────┘ │
                        └─────────────────────┘
```

## CI/CD Pipeline

```
GitHub Push
    │
    ▼
┌─────────────────────────────────────────────────────────────────┐
│                    GitHub Actions Workflow                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  JOB: unit-tests (Matrix: 4 platforms)                    │  │
│  │  ┌─────────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │  │
│  │  │ ubuntu-     │ │ macos-   │ │ macos-13 │ │ windows- │  │  │
│  │  │ latest      │ │ latest   │ │ (Intel)  │ │ latest   │  │  │
│  │  │ (x86_64)    │ │ (ARM)    │ │          │ │          │  │  │
│  │  └─────────────┘ └──────────┘ └──────────┘ └──────────┘  │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                  │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  JOB: docker-tests (Matrix: 5 distros)                    │  │
│  │  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐            │  │
│  │  │Ubuntu│ │Ubuntu│ │Alpine│ │Debian│ │Fedora│            │  │
│  │  │22.04 │ │20.04 │ │ 3.19 │ │  12  │ │  39  │            │  │
│  │  └──┬───┘ └──┬───┘ └──┬───┘ └──┬───┘ └──┬───┘            │  │
│  │     │        │        │        │        │                 │  │
│  │     └────────┴────────┴────────┴────────┘                 │  │
│  │                      │                                     │  │
│  │              Smoke + Integration                           │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                  │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  JOB: integration-tests (Needs: unit-tests)               │  │
│  │  ┌─────────────┐ ┌──────────┐ ┌──────────┐               │  │
│  │  │ ubuntu-     │ │ macos-   │ │ windows- │               │  │
│  │  │ latest      │ │ latest   │ │ latest   │               │  │
│  │  └─────────────┘ └──────────┘ └──────────┘               │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
         │                    │                    │
         ▼                    ▼                    ▼
    Pass               Pass               Pass
```

## Test Fixture Organization

```
testing/fixtures/
├── configs/
│   ├── smoke/                    # Fast validation (<1 min total)
│   │   ├── 001-version-check.yml      # Binary works
│   │   ├── 002-simple-file.yml        # File operations
│   │   ├── 003-simple-shell.yml       # Shell execution
│   │   └── 004-simple-vars.yml        # Variables
│   │
│   └── integration/              # Full features (5-10 min total)
│       ├── 010-file-operations.yml    # Complete file workflow
│       ├── 020-loops.yml              # Loop iteration
│       ├── 030-conditionals.yml       # When conditions
│       └── 040-shell-commands.yml     # Complex shell
│
└── templates/
    └── test-template.j2          # Template rendering test
```

## Docker Test Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    Docker Test Process                       │
└─────────────────────────────────────────────────────────────┘

Step 1: Build Binary
┌─────────────────────────────────────────────┐
│ env GOOS=linux GOARCH=amd64 \              │
│ go build -o out/mooncake-linux-amd64 ./cmd │
└──────────────────┬──────────────────────────┘
                   │
                   ▼
Step 2: Build Docker Image
┌─────────────────────────────────────────────┐
│ docker build -f testing/docker/ubuntu.      │
│   Dockerfile -t mooncake-test-ubuntu .      │
│                                             │
│ Dockerfile:                                 │
│   - FROM ubuntu:22.04                       │
│   - Install: curl, git, sudo                │
│   - COPY binary → /usr/local/bin/mooncake   │
│   - COPY test-runner.sh → /test-runner.sh  │
│   - COPY fixtures/ → /fixtures/             │
└──────────────────┬──────────────────────────┘
                   │
                   ▼
Step 3: Run Tests
┌─────────────────────────────────────────────┐
│ docker run --rm mooncake-test-ubuntu smoke  │
│                                             │
│ Container executes:                         │
│   /test-runner.sh smoke                     │
│      ↓                                      │
│   For each test in /fixtures/configs/smoke/│
│      mooncake run -c test.yml               │
│      Save results to /workspace/results/    │
└─────────────────────────────────────────────┘
```

## Test Runner Logic

```
test-runner.sh
├── Verify mooncake binary exists
├── Parse TEST_SUITE parameter (smoke|integration|all)
│
├── run_smoke_tests()
│   ├── Find all *.yml in /fixtures/configs/smoke/
│   ├── For each test:
│   │   ├── Run: mooncake run -c test.yml
│   │   ├── Capture output to results/
│   │   ├── Track pass/fail count
│   │   └── Print ✓ or ✗
│   └── Return status
│
├── run_integration_tests()
│   ├── Find all *.yml in /fixtures/configs/integration/
│   ├── For each test:
│   │   ├── Run: mooncake run -c test.yml
│   │   ├── Capture output to results/
│   │   ├── Track pass/fail count
│   │   └── Print ✓ or ✗
│   └── Return status
│
└── Exit with appropriate code
```

## Script Orchestration

```
make test-all-platforms
    ├── Run native Go tests
    │   └── go test -v ./...
    │
    └── Run Docker multi-distro tests
        └── scripts/test-docker-all.sh
            ├── Build Linux binary once
            ├── For each distro:
            │   ├── scripts/test-docker.sh <distro>
            │   │   ├── Build Docker image
            │   │   └── Run container with test suite
            │   └── Track pass/fail
            └── Print summary
```

## Platform Coverage Matrix

```
┌─────────────────┬────────┬────────┬─────────┬──────────────┐
│                 │ Local  │  CI    │ Docker  │ Native       │
├─────────────────┼────────┼────────┼─────────┼──────────────┤
│ Ubuntu 22.04    │      │      │       │              │
│ Ubuntu 20.04    │      │      │       │              │
│ Alpine 3.19     │      │      │       │              │
│ Debian 12       │      │      │       │              │
│ Fedora 39       │      │      │       │              │
│ macOS (ARM)     │      │      │         │            │
│ macOS (Intel)   │      │      │         │            │
│ Windows         │        │      │         │            │
└─────────────────┴────────┴────────┴─────────┴──────────────┘

Legend:
   = Tested
  Local = Developer machine
  CI = GitHub Actions
  Docker = Container-based testing
  Native = Direct OS execution
```

## Test Types and Timing

```
┌──────────────┬─────────────┬─────────┬──────────────────────┐
│ Test Type    │ Count       │ Time    │ Purpose              │
├──────────────┼─────────────┼─────────┼──────────────────────┤
│ Unit         │ Per package │ ~10s    │ Go code validation   │
│ Smoke        │ 4 tests     │ ~30s    │ Basic functionality  │
│ Integration  │ 4 tests     │ ~2-5m   │ Feature validation   │
│ Full Suite   │ All above   │ ~15m    │ Complete validation  │
└──────────────┴─────────────┴─────────┴──────────────────────┘

Smoke Tests:
  - Binary execution ✓
  - File operations ✓
  - Shell commands ✓
  - Variables ✓

Integration Tests:
  - File workflow ✓
  - Loops ✓
  - Conditionals ✓
  - Complex shell ✓
```

## Directory Structure

```
mooncake/
│
├── .github/workflows/
│   └── ci.yml ────────────────► CI/CD configuration
│
├── testing/
│   ├── docker/ ───────────────► Dockerfiles (5 distros)
│   ├── common/
│   │   └── test-runner.sh ────► Test orchestration
│   ├── fixtures/
│   │   ├── configs/
│   │   │   ├── smoke/ ────────► 4 smoke tests
│   │   │   └── integration/ ──► 4 integration tests
│   │   └── templates/ ────────► Test templates
│   ├── results/ ──────────────► Test outputs (gitignored)
│   └── README.md ─────────────► Testing documentation
│
├── scripts/
│   ├── test-docker.sh ────────► Single distro test
│   ├── test-docker-all.sh ────► Multi-distro test
│   ├── test-all-platforms.sh ─► Complete test suite
│   └── run-integration-tests.sh ► Integration runner
│
├── Makefile ──────────────────► Test targets
└── out/ ──────────────────────► Compiled binaries
```

## Data Flow

```
Developer
    │
    ├─► Edit Code
    │       │
    │       ▼
    │   make test ────► go test ./... ────► /
    │
    ├─► make test-quick
    │       │
    │       ├─► Build Linux binary
    │       ├─► Build Docker image (Ubuntu)
    │       ├─► Run smoke tests
    │       └─► /
    │
    ├─► make test-all-platforms
    │       │
    │       ├─► go test ./...
    │       ├─► Build Linux binary
    │       ├─► For each distro:
    │       │   ├─► Build image
    │       │   └─► Run smoke + integration
    │       └─► / Summary
    │
    └─► git push
            │
            ▼
        GitHub Actions
            │
            ├─► unit-tests (4 platforms)
            ├─► docker-tests (5 distros)
            └─► integration-tests (3 platforms)
                    │
                    ▼
                / Status
```

## Success Metrics

```
┌────────────────────────────────────────────────────┐
│              Testing Success Criteria               │
├────────────────────────────────────────────────────┤
│                                                     │
│  ⚡ Fast Feedback                                   │
│    • Local smoke: < 2 minutes                    │
│    • CI complete: < 10 minutes                   │
│                                                     │
│  🔍 Comprehensive Coverage                          │
│    • 5 Linux distros tested                      │
│    • macOS Intel + ARM                           │
│    • Windows Server                              │
│                                                     │
│  👨‍💻 Developer Experience                            │
│    • Single command testing                      │
│    • Clear documentation                         │
│    • Easy to add tests                           │
│                                                     │
│   Quality Assurance                               │
│    • Unit tests: >60% coverage                   │
│    • Smoke tests: 4 scenarios                    │
│    • Integration: 4 workflows                    │
│                                                     │
└────────────────────────────────────────────────────┘
```

## Future Architecture Enhancements

```
Potential Additions:
├── ARM64 Linux Testing
│   └── Add arm64 Docker builds
│
├── Performance Benchmarking
│   ├── Benchmark suite
│   └── Historical tracking
│
├── Test Result Dashboard
│   ├── Web UI for results
│   └── Trend analysis
│
└── Extended Platform Support
    ├── FreeBSD
    ├── OpenBSD
    └── Additional Linux distros
```

## Key Design Decisions

1. **Docker for Linux** - Portable, reproducible, multi-distro support
2. **Native for macOS** - Simplest, fastest, no containerization overhead
3. **CI-only for Windows** - Avoid VM complexity, sufficient for validation
4. **Two-tier tests** - Smoke (fast feedback) + Integration (thorough)
5. **Parallel execution** - Maximize throughput in CI
6. **Cached layers** - Docker caching for faster rebuilds
7. **Gitignored results** - Keep repo clean, results are ephemeral
8. **Shell scripts** - Portable, easy to debug, no extra dependencies

---

**For detailed usage instructions, see**: `guide.md`
**For quick reference, see**: `quick-reference.md`


---

<!-- FILE: testing/guide.md -->

# Mooncake Multi-Platform Testing Guide

> 📚 **Other Docs**: [Index](README.md) | [Quick Reference](quick-reference.md) | [Architecture](architecture.md) | [Implementation](implementation-summary.md)

This document describes the comprehensive testing setup for Mooncake across Linux, macOS, and Windows platforms.

## Overview

Mooncake uses a hybrid testing approach that balances fast local development with thorough CI validation:

- **Linux**: Docker containers for multiple distributions (local + CI)
- **macOS**: Native testing locally + GitHub Actions for CI
- **Windows**: GitHub Actions only (no local Windows testing required)

## Quick Start

### Local Testing

```bash
# Run unit tests on current platform
make test

# Quick smoke test (Linux via Docker, ~2 minutes)
make test-quick

# Test on specific Linux distro
make test-docker-ubuntu    # Ubuntu 22.04
make test-docker-alpine    # Alpine 3.19
make test-docker-debian    # Debian 12
make test-docker-fedora    # Fedora 39

# Run smoke tests on all Linux distros (~10 minutes)
make test-smoke

# Run integration tests locally
make test-integration

# Run ALL Docker tests (smoke + integration on all distros, ~15 minutes)
make test-docker-all

# Run complete local test suite (native + Docker)
make test-all-platforms
```

### CI Testing

Push to any branch and GitHub Actions will automatically run:

1. **Unit Tests** - Go tests on Ubuntu, macOS (Intel + Apple Silicon), Windows
2. **Docker Tests** - Smoke + integration tests on 5 Linux distros
3. **Integration Tests** - Full feature tests on Ubuntu, macOS, Windows

All CI jobs run in parallel for fast feedback (~5-10 minutes total).

## Test Structure

```
testing/
├── docker/                          # Docker configurations
│   ├── ubuntu-22.04.Dockerfile     # Ubuntu 22.04 LTS
│   ├── ubuntu-20.04.Dockerfile     # Ubuntu 20.04 LTS
│   ├── alpine-3.19.Dockerfile      # Alpine Linux (minimal)
│   ├── debian-12.Dockerfile        # Debian Bookworm
│   └── fedora-39.Dockerfile        # Fedora 39 (RPM-based)
├── common/
│   └── test-runner.sh              # Common test orchestration
├── fixtures/
│   ├── configs/
│   │   ├── smoke/                  # Fast validation tests (<1 min)
│   │   │   ├── 001-version-check.yml
│   │   │   ├── 002-simple-file.yml
│   │   │   ├── 003-simple-shell.yml
│   │   │   └── 004-simple-vars.yml
│   │   └── integration/            # Full feature tests (5-10 min)
│   │       ├── 010-file-operations.yml
│   │       ├── 020-loops.yml
│   │       ├── 030-conditionals.yml
│   │       └── 040-shell-commands.yml
│   └── templates/
│       └── test-template.j2        # Test template file
├── results/                         # Test results (gitignored)
└── README.md                        # This file
```

## Test Types

### Smoke Tests

Fast validation tests that verify basic functionality:
- Binary execution and version check
- Simple file operations
- Basic shell commands
- Variable substitution

**Runtime**: ~30 seconds per distro
**Purpose**: Catch obvious breakage quickly

### Integration Tests

Comprehensive tests that validate full features:
- Complete file management operations
- Loop iteration
- Conditional execution
- Complex shell commands
- Template rendering

**Runtime**: ~2-5 minutes per distro
**Purpose**: Ensure features work correctly across platforms

## Adding New Tests

### 1. Create a Test Config

Create a YAML file in the appropriate directory:

```yaml
# testing/fixtures/configs/smoke/005-my-test.yml
- name: My test step
  shell: echo "test"
  register: result

- name: Verify result
  shell: test "{{ result.stdout }}" = "test"
```

### 2. Test Locally

```bash
# Test with mooncake directly
./out/mooncake run -c testing/fixtures/configs/smoke/005-my-test.yml

# Test in Docker
make test-docker-ubuntu
```

### 3. Verify in CI

Push your changes and verify all platforms pass:

```bash
git add testing/fixtures/configs/smoke/005-my-test.yml
git commit -m "test: add my new test"
git push
```

Check GitHub Actions for results.

## Platform-Specific Notes

### Linux (Docker)

**Supported Distributions**:

- Ubuntu 22.04 LTS (Jammy)
- Ubuntu 20.04 LTS (Focal)
- Alpine 3.19 (minimal, musl libc)
- Debian 12 (Bookworm)
- Fedora 39 (RPM-based)

**Requirements**:

- Docker or Podman installed
- ~2GB disk space for images

**Tips**:

- Use `test-quick` for rapid iteration
- Images are cached after first build
- Add new distros by creating a new Dockerfile in `testing/docker/`

### macOS (Native)

**Testing Approach**:

- Local: Run tests natively on your Mac
- CI: Tests on both Intel (macos-13) and Apple Silicon (macos-latest)

**Requirements**:

- Go 1.25+
- No additional dependencies

**Tips**:

- Use `make test` for quick validation
- CI covers both architectures automatically

### Windows (CI Only)

**Testing Approach**:

- No local testing required (use Docker/WSL if needed)
- Automated testing via GitHub Actions on Windows Server

**Requirements**:

- None for local development
- Push to GitHub to test Windows

**Tips**:

- Use WSL2 with Docker for Linux testing on Windows
- Integration tests use `bash` shell (available via Git Bash on Windows)

## Troubleshooting

### Docker Build Fails

**Problem**: Docker build fails with "binary not found"

**Solution**:
```bash
# Ensure binary is built first
env GOOS=linux GOARCH=amd64 go build -v -o out/mooncake-linux-amd64 ./cmd

# Or use the script which builds automatically
./scripts/test-docker.sh ubuntu-22.04
```

### Docker Not Running

**Problem**: `Cannot connect to the Docker daemon`

**Solution**:
```bash
# Check Docker is running
docker ps

# Start Docker Desktop (macOS/Windows)
# Or start docker service (Linux)
sudo systemctl start docker
```

### Tests Fail on Specific Distro

**Problem**: Tests pass on Ubuntu but fail on Alpine

**Solution**:
```bash
# Run container interactively
docker build -f testing/docker/alpine-3.19.Dockerfile -t mooncake-test-alpine .
docker run -it mooncake-test-alpine /bin/sh

# Debug inside container
/test-runner.sh smoke
```

### Test Results Not Visible

**Problem**: Can't see detailed test output

**Solution**:
```bash
# Check results directory
ls -la testing/results/

# View specific test log
cat testing/results/smoke-001-version-check.yml.log
```

### Integration Tests Fail Locally

**Problem**: Integration tests fail with "binary not found"

**Solution**:
```bash
# Build binary for current platform
go build -v -o out/mooncake ./cmd

# Run integration tests
./scripts/run-integration-tests.sh
```

## CI Workflow Details

### GitHub Actions Jobs

1. **unit-tests**: Go tests on 4 platforms (Ubuntu, macOS x2, Windows)
2. **docker-tests**: Smoke + integration on 5 Linux distros
3. **integration-tests**: Full feature tests on 3 platforms

### Viewing Results

1. Go to the [Actions tab](../../actions) in GitHub
2. Click on your workflow run
3. Expand job details to see logs
4. Download artifacts for detailed test results

### Coverage Reports

Code coverage is automatically calculated and uploaded to Codecov:
- Only runs on Ubuntu (to avoid duplicate reports)
- View at: https://codecov.io/gh/alehatsman/mooncake

## Performance

### Local Testing Times

- `make test`: ~10 seconds (Go unit tests)
- `make test-quick`: ~2 minutes (smoke tests on Ubuntu)
- `make test-smoke`: ~10 minutes (smoke on all distros)
- `make test-docker-all`: ~15 minutes (all tests, all distros)
- `make test-all-platforms`: ~15 minutes (native + Docker)

### CI Testing Times

- Unit tests: ~2-3 minutes per platform (parallel)
- Docker tests: ~5-7 minutes total (parallel builds)
- Integration tests: ~3-5 minutes per platform (parallel)
- **Total CI runtime**: ~7-10 minutes

## Best Practices

### For Developers

1. **Run `make test` before committing** - catches obvious issues
2. **Run `make test-quick` for local validation** - tests Linux compatibility
3. **Let CI handle comprehensive testing** - covers all platforms
4. **Add smoke tests for new features** - ensures basic functionality works
5. **Add integration tests for complex features** - validates complete workflows

### For Test Authors

1. **Keep smoke tests fast** - under 1 minute total
2. **Make tests idempotent** - can run multiple times
3. **Clean up test artifacts** - remove temp files/directories
4. **Use descriptive names** - clear what's being tested
5. **Test cross-platform** - avoid platform-specific commands in shared tests

### For CI

1. **Don't make CI required initially** - let it stabilize first
2. **Monitor flaky tests** - fix or remove unreliable tests
3. **Keep CI fast** - parallel execution, cached dependencies
4. **Fail fast** - stop on first critical failure
5. **Provide clear errors** - logs should point to root cause

## Future Enhancements

Potential improvements not in current scope:

- ARM64 Linux testing (currently x86_64 only)
- Performance benchmarking across platforms
- Windows WSL2 local testing support
- Visual regression testing for TUI output
- Test result dashboard/reporting
- Automated test generation from examples

## Contributing

When adding tests:

1. Add smoke test if testing basic functionality
2. Add integration test if testing complex features
3. Update this README if adding new test patterns
4. Ensure tests pass locally before pushing
5. Verify CI passes on all platforms

## Support

For issues or questions:

1. Check this README first
2. Look at existing test examples
3. Try running tests with verbose output
4. Check GitHub Actions logs for CI failures
5. Open an issue with reproduction steps

## License

Same as Mooncake project license.


---

<!-- FILE: testing/implementation-summary.md -->

# Multi-Platform Testing Setup - Implementation Summary

> 📚 **Other Docs**: [Index](README.md) | [Quick Reference](quick-reference.md) | [Testing Guide](guide.md) | [Architecture](architecture.md)

## Overview

Successfully implemented comprehensive multi-platform testing infrastructure for Mooncake supporting Ubuntu, macOS, and Windows with Docker containers for Linux testing, native testing for macOS, and GitHub Actions for Windows.

## What Was Implemented

### Phase 1: Docker Testing Infrastructure 

**Created 5 Distribution-Specific Dockerfiles**:

- `testing/docker/ubuntu-22.04.Dockerfile` - Ubuntu 22.04 LTS (Jammy)
- `testing/docker/ubuntu-20.04.Dockerfile` - Ubuntu 20.04 LTS (Focal)
- `testing/docker/alpine-3.19.Dockerfile` - Alpine 3.19 (minimal, musl libc)
- `testing/docker/debian-12.Dockerfile` - Debian 12 (Bookworm)
- `testing/docker/fedora-39.Dockerfile` - Fedora 39 (RPM-based)

**Common Test Runner**:

- `testing/common/test-runner.sh` - Orchestrates smoke and integration tests
  - Colored output for better readability
  - Test result collection
  - Support for multiple test suites (smoke, integration, all)

### Phase 2: Local Development Workflow 

**Test Orchestration Scripts**:

- `scripts/test-docker.sh` - Test on single Linux distribution
- `scripts/test-docker-all.sh` - Test on all Linux distributions
- `scripts/test-all-platforms.sh` - Complete local test suite (native + Docker)
- `scripts/run-integration-tests.sh` - Integration test runner for CI

**Makefile Targets**:

- `make test-quick` - Quick smoke test on Ubuntu 22.04 (~2 min)
- `make test-smoke` - Smoke tests on all distros (~10 min)
- `make test-integration` - Integration tests locally
- `make test-docker-ubuntu` - Test specific distro (Ubuntu)
- `make test-docker-alpine` - Test specific distro (Alpine)
- `make test-docker-debian` - Test specific distro (Debian)
- `make test-docker-fedora` - Test specific distro (Fedora)
- `make test-docker-all` - All tests on all distros (~15 min)
- `make test-all-platforms` - Complete local test suite

### Phase 3: Enhanced CI/CD Workflow 

**Updated `.github/workflows/ci.yml`**:

- **unit-tests** job: Now tests on 4 platforms (Ubuntu, macOS Intel, macOS ARM, Windows)
- **docker-tests** job: Tests on 5 Linux distros with smoke + integration tests
- **integration-tests** job: Full feature tests on Ubuntu, macOS, Windows
- All jobs run in parallel for fast feedback (~7-10 min total)

### Phase 4: Test Fixtures and Scenarios 

**Smoke Tests** (4 tests):
- `001-version-check.yml` - Verify mooncake installation and version
- `002-simple-file.yml` - Basic file operations (create, verify, delete)
- `003-simple-shell.yml` - Shell command execution and output capture
- `004-simple-vars.yml` - Variable substitution and usage

**Integration Tests** (4 tests):
- `010-file-operations.yml` - Complete file management workflow
- `020-loops.yml` - Loop iteration with file creation
- `030-conditionals.yml` - Conditional execution tests
- `040-shell-commands.yml` - Complex shell command scenarios

**Templates**:

- `test-template.j2` - Test template with system facts

### Phase 5: Documentation 

**Created**:

- `docs/testing/guide.md` - Comprehensive testing guide (300+ lines)
  - Quick start guide
  - Test structure overview
  - Platform-specific notes
  - Troubleshooting guide
  - Best practices
  - CI workflow details

**Updated**:

- `README.md` - Added testing section with quick commands
- `.gitignore` - Added `testing/results/` to ignore test outputs

## File Structure

```
mooncake/
├── .github/
│   └── workflows/
│       └── ci.yml                           # Enhanced with Windows + Docker matrix
├── testing/
│   ├── docker/
│   │   ├── ubuntu-22.04.Dockerfile          # NEW
│   │   ├── ubuntu-20.04.Dockerfile          # NEW
│   │   ├── alpine-3.19.Dockerfile           # NEW
│   │   ├── debian-12.Dockerfile             # NEW
│   │   └── fedora-39.Dockerfile             # NEW
│   ├── common/
│   │   └── test-runner.sh                   # NEW
│   ├── fixtures/
│   │   ├── configs/
│   │   │   ├── smoke/
│   │   │   │   ├── 001-version-check.yml   # NEW
│   │   │   │   ├── 002-simple-file.yml     # NEW
│   │   │   │   ├── 003-simple-shell.yml    # NEW
│   │   │   │   └── 004-simple-vars.yml     # NEW
│   │   │   └── integration/
│   │   │       ├── 010-file-operations.yml  # NEW
│   │   │       ├── 020-loops.yml            # NEW
│   │   │       ├── 030-conditionals.yml     # NEW
│   │   │       └── 040-shell-commands.yml   # NEW
│   │   └── templates/
│   │       └── test-template.j2             # NEW
│   ├── results/                             # NEW (gitignored)
│   └── README.md                            # NEW
├── scripts/
│   ├── test-docker.sh                       # NEW
│   ├── test-docker-all.sh                   # NEW
│   ├── test-all-platforms.sh                # NEW
│   └── run-integration-tests.sh             # NEW
├── Makefile                                 # UPDATED (added 9 new targets)
├── README.md                                # UPDATED (added testing section)
└── .gitignore                               # UPDATED (added testing/results/)
```

## New Files Created: 24
## Files Modified: 4

## Usage Examples

### Local Development

```bash
# Quick validation during development
make test-quick

# Test specific distro
make test-docker-alpine

# Full local test suite
make test-all-platforms

# Run just integration tests
make test-integration
```

### CI Workflow

1. Push to any branch
2. GitHub Actions automatically runs:
   - Unit tests on Ubuntu, macOS (2 versions), Windows
   - Docker tests on 5 Linux distros
   - Integration tests on 3 platforms
3. All jobs complete in ~7-10 minutes
4. Coverage report uploaded to Codecov

## Key Features

### Fast Iteration
- `make test-quick` completes in ~2 minutes
- Cached Docker layers speed up subsequent builds
- Parallel CI execution maximizes throughput

### Comprehensive Coverage
- 5 Linux distributions tested
- macOS Intel and Apple Silicon covered
- Windows Server testing in CI
- Both unit and integration tests

### Developer-Friendly
- Single command testing (`make test-all-platforms`)
- Clear error messages and logs
- Test results saved locally
- Comprehensive documentation

### CI/CD Integration
- Parallel job execution
- Matrix testing for platforms and distros
- Coverage reporting
- Clear job naming and status

## Verification Steps

### 1. Verify Scripts Are Executable
```bash
ls -la scripts/*.sh testing/common/*.sh
# All should have execute permissions (x)
```

### 2. Test Local Smoke Test
```bash
# This will:
# - Build Linux binary
# - Build Docker image
# - Run smoke tests
make test-quick
```

### 3. Test CI Workflow
```bash
# Push to GitHub and check Actions tab
git add -A
git commit -m "feat: add multi-platform testing setup"
git push
# Check: https://github.com/alehatsman/mooncake/actions
```

### 4. Verify Documentation
```bash
# Read testing guide
cat docs/testing/guide.md

# Check main README has testing section
grep -A 20 "## Testing" README.md
```

## Expected Test Times

### Local
- `make test`: ~10 seconds (Go unit tests)
- `make test-quick`: ~2 minutes (smoke on Ubuntu)
- `make test-smoke`: ~10 minutes (smoke on all distros)
- `make test-docker-all`: ~15 minutes (all tests, all distros)
- `make test-all-platforms`: ~15 minutes (native + Docker)

### CI
- Unit tests: ~2-3 minutes per platform (4 platforms in parallel)
- Docker tests: ~5-7 minutes total (5 distros in parallel)
- Integration tests: ~3-5 minutes per platform (3 platforms in parallel)
- **Total CI time**: ~7-10 minutes

## Next Steps

### Immediate (Required for First Run)
1.  Build mooncake binary: `go build -v -o out/mooncake ./cmd`
2.  Run first smoke test: `make test-quick`
3.  Verify CI workflow: Push to GitHub and check Actions
4.  Review test results: Check `testing/results/` for logs

### Short-term Improvements
1. Add more integration tests as features are developed
2. Monitor CI for flaky tests and fix or remove them
3. Add platform-specific tests where needed
4. Update documentation based on team feedback

### Long-term Enhancements
1. ARM64 Linux testing support
2. Performance benchmarking across platforms
3. Test result dashboard/reporting
4. Automated test generation from examples

## Success Criteria Status

 Local testing works with single command
 Docker tests support multiple Linux distros
 CI tests all platforms (Linux, macOS, Windows)
 Clear documentation for developers
 Fast feedback (< 2 min for quick test, < 10 min for CI)
 Easy to add new tests
 Test results are visible and debuggable

## Known Limitations

1. **Windows local testing**: Not supported - use GitHub Actions for Windows validation
2. **ARM64 Linux**: Currently only x86_64 tested
3. **Test coverage**: Initial set of 8 tests - will grow over time
4. **Docker requirement**: Local Docker/Podman needed for Linux testing

## Troubleshooting

### "Binary not found"
```bash
# Build binary first
env GOOS=linux GOARCH=amd64 go build -v -o out/mooncake-linux-amd64 ./cmd
```

### "Docker daemon not running"
```bash
# Start Docker Desktop (macOS/Windows)
# Or: sudo systemctl start docker (Linux)
```

### "Script permission denied"
```bash
# Make scripts executable
chmod +x scripts/*.sh testing/common/*.sh
```

### Tests fail in Docker but pass natively
```bash
# Run container interactively to debug
docker build -f testing/docker/ubuntu-22.04.Dockerfile -t mooncake-test-ubuntu .
docker run -it mooncake-test-ubuntu /bin/sh
```

## Conclusion

The multi-platform testing setup is complete and ready for use. All phases have been implemented:

-  Phase 1: Docker testing infrastructure
-  Phase 2: Local development workflow
-  Phase 3: Enhanced CI/CD workflow
-  Phase 4: Test fixtures and scenarios
-  Phase 5: Documentation and polish

The implementation provides fast local iteration with `make test-quick`, comprehensive multi-distro validation with `make test-docker-all`, and automated CI testing on all platforms. The setup balances developer productivity with thorough validation, making it easy to catch platform-specific issues early.

## References

- Testing Guide: `docs/testing/guide.md`
- CI Workflow: `.github/workflows/ci.yml`
- Test Scripts: `scripts/test-*.sh`
- Test Fixtures: `testing/fixtures/configs/`
- Docker Images: `testing/docker/`


---

<!-- FILE: testing/quick-reference.md -->

# Mooncake Testing - Quick Reference

> 📚 **Other Docs**: [Index](README.md) | [Testing Guide](guide.md) | [Architecture](architecture.md) | [Implementation](implementation-summary.md)

## One-Line Commands

```bash
# Quick smoke test (2 min)
make test-quick

# Test all distros (10 min)
make test-smoke

# Full local test suite (15 min)
make test-all-platforms

# Just unit tests (10 sec)
make test

# Just integration tests
make test-integration
```

## Test by Platform

```bash
# Linux - Specific distro
make test-docker-ubuntu     # Ubuntu 22.04
make test-docker-alpine     # Alpine 3.19
make test-docker-debian     # Debian 12
make test-docker-fedora     # Fedora 39

# Linux - All distros
make test-docker-all

# macOS - Native
make test

# Windows - Push to GitHub
git push  # Check GitHub Actions
```

## Direct Script Usage

```bash
# Test single distro with specific suite
./scripts/test-docker.sh ubuntu-22.04 smoke
./scripts/test-docker.sh alpine-3.19 integration

# Test all distros with specific suite
./scripts/test-docker-all.sh smoke
./scripts/test-docker-all.sh integration
./scripts/test-docker-all.sh all

# Run integration tests
./scripts/run-integration-tests.sh

# Complete local test
./scripts/test-all-platforms.sh
```

## Test a Single Config Manually

```bash
# Build binary
go build -v -o out/mooncake ./cmd

# Run a specific test
./out/mooncake run -c testing/fixtures/configs/smoke/001-version-check.yml
./out/mooncake run -c testing/fixtures/configs/integration/010-file-operations.yml
```

## Debug Docker Tests

```bash
# Build image
docker build -f testing/docker/ubuntu-22.04.Dockerfile -t mooncake-test-ubuntu .

# Run interactively
docker run -it mooncake-test-ubuntu /bin/sh

# Inside container:
mooncake --version
/test-runner.sh smoke
```

## View Test Results

```bash
# List results
ls -la testing/results/

# View specific log
cat testing/results/smoke-001-version-check.yml.log

# View all smoke test logs
cat testing/results/smoke-*.log
```

## CI Status

```bash
# Check CI status
gh run list --limit 5

# View specific run
gh run view <run-id>

# Watch current run
gh run watch
```

## Common Workflows

### Before Committing
```bash
make test                    # Quick unit tests
make test-quick              # Smoke test on Ubuntu
```

### Before Pushing
```bash
make test-all-platforms      # Full local suite
```

### After Pushing
Check GitHub Actions:
- https://github.com/alehatsman/mooncake/actions

### Adding New Test
```bash
# 1. Create test file
vim testing/fixtures/configs/smoke/005-my-test.yml

# 2. Test directly
./out/mooncake run -c testing/fixtures/configs/smoke/005-my-test.yml

# 3. Test in Docker
make test-quick

# 4. Commit and push
git add testing/fixtures/configs/smoke/005-my-test.yml
git commit -m "test: add my new test"
git push
```

## File Locations

```
testing/fixtures/configs/smoke/          # Smoke tests (<1 min)
testing/fixtures/configs/integration/    # Integration tests (5-10 min)
testing/docker/                          # Dockerfiles for each distro
scripts/test-*.sh                        # Test orchestration scripts
testing/results/                         # Test output logs (gitignored)
```

## Troubleshooting Quick Fixes

```bash
# Binary not found
env GOOS=linux GOARCH=amd64 go build -v -o out/mooncake-linux-amd64 ./cmd

# Docker not running
docker ps  # If fails, start Docker Desktop

# Scripts not executable
chmod +x scripts/*.sh testing/common/*.sh

# Clean Docker cache
docker system prune -f

# Clean test results
rm -rf testing/results/*
```

## Expected Times

| Command | Time | Purpose |
|---------|------|---------|
| `make test` | 10s | Quick Go unit tests |
| `make test-quick` | 2 min | Smoke test on Ubuntu |
| `make test-smoke` | 10 min | Smoke on all distros |
| `make test-docker-all` | 15 min | All tests, all distros |
| `make test-all-platforms` | 15 min | Native + Docker |
| CI complete | 7-10 min | All jobs in parallel |

## Need More Details?

- Full guide: `guide.md`
- Implementation: `implementation-summary.md`
- CI config: `.github/workflows/ci.yml`
- Makefile: `Makefile` (lines 70+)


---

