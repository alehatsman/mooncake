# Changelog

## v0.3 - Current

### New Actions
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

### Developer Experience
- Custom error types with better error messages
- Comprehensive test suite (file_step, copy_step, dryrun, assert, service)
- Improved logging and debugging
- Security scanning (gosec) and vulnerability checking (govulncheck)
- Race detector enabled in CI

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
