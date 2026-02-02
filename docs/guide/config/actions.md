# Actions

Actions are the operations Mooncake performs. Each step in your configuration uses one action type.

📖 **See [Property Reference](reference.md)** for a complete table of all available properties.

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

## See Also

- [Control Flow](control-flow.md) - Conditionals, loops, tags
- [Variables](variables.md) - Variable usage and system facts
- [Examples](../../examples/index.md) - Practical examples
