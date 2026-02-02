# Configuration Reference

Complete reference for all Mooncake configuration options.

→ **Learning by doing?** Try [Examples](../../examples/) instead
→ **Need action examples?** See [Actions Guide](actions.md)

---

## Actions Quick Reference

Actions are what your steps **do**. Every step uses exactly one action.

| Action | Purpose | Example |
|--------|---------|---------|
| **shell** | Execute commands | `shell: echo "hello"` |
| **command** | Direct execution (no shell) | `command: {argv: [git, status]}` |
| **file** | Create/manage files | `file: {path: /tmp/test, state: file}` |
| **copy** | Copy files | `copy: {src: ./app.conf, dest: /etc/app.conf}` |
| **download** | Download from URLs | `download: {url: https://..., dest: /tmp/file}` |
| **package** | Manage packages | `package: {name: nginx, state: present}` |
| **unarchive** | Extract archives | `unarchive: {src: /tmp/app.tar.gz, dest: /opt/app}` |
| **template** | Render templates | `template: {src: app.j2, dest: /etc/app.conf}` |
| **service** | Manage services | `service: {name: nginx, state: started}` |
| **assert** | Verify state | `assert: {file: {path: /tmp/test, exists: true}}` |
| **preset** | Reusable workflows | `preset: ollama` |
| **include** | Load other configs | `include: ./tasks/common.yml` |
| **include_vars** | Load variables | `include_vars: ./vars/prod.yml` |
| **vars** | Define variables | `vars: {app_name: MyApp}` |

**→ See [Actions Guide](actions.md) for detailed examples and use cases**

---

## Actions (Detailed)

### shell

Execute commands in a shell with full shell interpolation.

**Basic form**:
```yaml
- shell: echo "hello"
```

**Structured form**:
```yaml
- shell:
    cmd: echo "hello"
    interpreter: bash
    stdin: "input data"
    capture: true
```

**Properties**:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `shell` | string | Yes | Command to execute (simple form) |
| `shell.cmd` | string | Yes | Command to execute (structured form) |
| `shell.interpreter` | string | No | Shell: "bash", "sh", "pwsh", "cmd" (default: bash on Unix, pwsh on Windows) |
| `shell.stdin` | string | No | Input to pipe into command |
| `shell.capture` | boolean | No | Capture output (default: true) |

**Works with**: All [universal properties](#universal-properties) + [shell-specific properties](#shell-specific-properties)

**Examples**:
```yaml
# Simple command
- shell: echo "hello"

# Multi-line
- shell: |
    echo "Line 1"
    echo "Line 2"

# With variables
- shell: echo "Running on {{os}}"

# With execution control
- shell: curl -O https://example.com/file.tar.gz
  timeout: 10m
  retries: 3
  retry_delay: 30s
```

[See complete shell documentation in Actions Guide →](actions.md#shell)

---

### command

Execute commands directly without shell interpolation (safer, faster).

**Basic form**:
```yaml
- command:
    argv: ["git", "status"]
```

**Properties**:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `command.argv` | array | Yes | Command and arguments as list |
| `command.stdin` | string | No | Input to pipe to command |
| `command.capture` | boolean | No | Capture output (default: true) |

**Works with**: All [universal properties](#universal-properties) + [shell-specific properties](#shell-specific-properties)

**Example**:
```yaml
- command:
    argv: ["git", "clone", "{{repo_url}}", "/opt/app"]
  timeout: 5m
  retries: 3
```

**Difference from shell**: No shell interpretation (no pipes, redirects, wildcards), arguments passed directly to executable.

[See complete command documentation in Actions Guide →](actions.md#command)

---

### file

Create and manage files, directories, and links.

**Basic form**:
```yaml
- file:
    path: /tmp/test
    state: directory
```

**Properties**:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `file.path` | string | Yes | File or directory path |
| `file.state` | string | No | `file`, `directory`, `absent`, `touch`, `link`, `hardlink`, `perms` |
| `file.content` | string | No | Content to write (for `state: file`) |
| `file.mode` | string | No | Permissions (e.g., "0644", "0755") |
| `file.owner` | string | No | Owner (username or UID) |
| `file.group` | string | No | Group (group name or GID) |
| `file.src` | string | No | Source for links (required for `link`/`hardlink`) |
| `file.force` | boolean | No | Force overwrite |
| `file.recurse` | boolean | No | Apply permissions recursively (with `state: perms`) |
| `file.backup` | boolean | No | Create .bak backup |

**Works with**: [Universal properties](#universal-properties) only (NOT shell-specific properties)

**Examples**:
```yaml
# Create directory
- file:
    path: ~/.config/myapp
    state: directory
    mode: "0755"

# Create file with content
- file:
    path: /tmp/config.txt
    state: file
    content: "app_name: myapp"
    mode: "0644"

# Create symbolic link
- file:
    path: /usr/local/bin/myapp
    src: /opt/myapp/bin/myapp
    state: link
```

[See complete file documentation in Actions Guide →](actions.md#file)

---

### copy

Copy files with checksum verification and backup support.

**Properties**:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `copy.src` | string | Yes | Source file path |
| `copy.dest` | string | Yes | Destination file path |
| `copy.mode` | string | No | Permissions (e.g., "0644") |
| `copy.owner` | string | No | Owner (username or UID) |
| `copy.group` | string | No | Group (group name or GID) |
| `copy.backup` | boolean | No | Create .bak backup before overwrite |
| `copy.force` | boolean | No | Force overwrite |
| `copy.checksum` | string | No | Expected SHA256 or MD5 checksum |

**Works with**: [Universal properties](#universal-properties)

**Example**:
```yaml
- copy:
    src: ./configs/app.yml
    dest: /etc/app/config.yml
    mode: "0644"
    owner: app
    group: app
    backup: true
```

[See complete copy documentation in Actions Guide →](actions.md#copy)

---

### download

Download files from remote URLs with checksum verification and retry support.

**Properties**:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `download.url` | string | Yes | Remote URL |
| `download.dest` | string | Yes | Destination file path |
| `download.checksum` | string | No | Expected SHA256 (64 chars) or MD5 (32 chars) |
| `download.mode` | string | No | File permissions |
| `download.timeout` | string | No | Maximum download time (e.g., "5m") |
| `download.retries` | integer | No | Retry attempts (0-100) |
| `download.force` | boolean | No | Force re-download |
| `download.backup` | boolean | No | Create .bak backup |
| `download.headers` | object | No | Custom HTTP headers |

**Works with**: [Universal properties](#universal-properties)

**Example**:
```yaml
- download:
    url: "https://go.dev/dl/go1.21.5.linux-amd64.tar.gz"
    dest: "/tmp/go.tar.gz"
    checksum: "e2bc0b3e4b64111ec117295c088bde5f00eeed1567999ff77bc859d7df70078e"
    timeout: "10m"
    retries: 3
```

**Idempotency**: Downloads skipped when destination exists with matching checksum.

[See complete download documentation in Actions Guide →](actions.md#download)

---

### package

Manage system packages with automatic package manager detection.

**Properties**:

| Property | Type | Description |
|----------|------|-------------|
| `name` | string | Single package name |
| `names` | array | Multiple package names |
| `state` | string | `present` (default), `absent`, `latest` |
| `manager` | string | Package manager (auto-detected if not specified) |
| `update_cache` | boolean | Update package cache before operation |
| `upgrade` | boolean | Upgrade all packages |
| `extra` | array | Extra arguments for package manager |

**Works with**: `name`, `when`, `become`, `tags`, `register`, `with_items`, `with_filetree`

**Supported managers**: apt, dnf, yum, pacman, zypper, apk, brew, port, choco, scoop

**Example**:
```yaml
- name: Install packages
  package:
    names: [git, curl, vim]
    state: present
    update_cache: true
  become: true
```

[See complete package documentation in Actions Guide →](actions.md#package)

---

### unarchive

Extract archive files with automatic format detection and security protections.

**Properties**:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `unarchive.src` | string | Yes | Path to archive file |
| `unarchive.dest` | string | Yes | Destination directory |
| `unarchive.strip_components` | integer | No | Strip leading path components (default: 0) |
| `unarchive.creates` | string | No | Skip if this path exists (idempotency) |
| `unarchive.mode` | string | No | Directory permissions |

**Works with**: [Universal properties](#universal-properties)

**Supported formats**: `.tar`, `.tar.gz`, `.tgz`, `.zip` (auto-detected)

**Example**:
```yaml
- unarchive:
    src: /tmp/node-v20.tar.gz
    dest: /opt/node
    strip_components: 1
    creates: /opt/node/bin/node
    mode: "0755"
```

**Security**: Automatically blocks path traversal attacks and validates all paths.

[See complete unarchive documentation in Actions Guide →](actions.md#unarchive)

---

### template

Render configuration files from templates using pongo2 syntax.

**Properties**:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `template.src` | string | Yes | Source template file path |
| `template.dest` | string | Yes | Destination file path |
| `template.vars` | object | No | Additional variables for template |
| `template.mode` | string | No | Permissions (e.g., "0644") |

**Works with**: [Universal properties](#universal-properties)

**Example**:
```yaml
- template:
    src: ./config.yml.j2
    dest: /etc/app/config.yml
    mode: "0644"
    vars:
      port: 8080
      debug: true
```

**Template syntax**: Variables `{{ var }}`, conditionals `{% if %}`, loops `{% for %}`, filters `{{ path | expanduser }}`

[See complete template documentation in Actions Guide →](actions.md#template)

---

### service

Manage system services (systemd on Linux, launchd on macOS).

**Properties**:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `service.name` | string | Yes | Service name |
| `service.state` | string | No | `started`, `stopped`, `restarted`, `reloaded` |
| `service.enabled` | boolean | No | Enable on boot |
| `service.daemon_reload` | boolean | No | Reload daemon (systemd only) |
| `service.unit` | object | No | Unit/plist file config |
| `service.dropin` | object | No | Drop-in config (systemd only) |

**Works with**: [Universal properties](#universal-properties)

**Example**:
```yaml
- service:
    name: nginx
    state: started
    enabled: true
  become: true
```

[See complete service documentation in Actions Guide →](actions.md#service)

---

### assert

Verify system state, command results, file properties, or HTTP responses.

**Properties** (one type required):

**Command Assertion**:
```yaml
assert:
  command:
    cmd: docker --version
    exit_code: 0
```

**File Assertion**:
```yaml
assert:
  file:
    path: /tmp/test
    exists: true
    mode: "0644"
```

**HTTP Assertion**:
```yaml
assert:
  http:
    url: https://api.example.com/health
    status: 200
```

**Works with**: [Universal properties](#universal-properties)

**Key behaviors**: Always returns `changed: false`, fails fast on verification failure.

[See complete assert documentation in Actions Guide →](actions.md#assert)

---

### preset

Invoke reusable, parameterized workflows.

**Simple form**:
```yaml
- preset: my-preset
```

**With parameters**:
```yaml
- preset: ollama
  with:
    state: present
    service: true
    pull: [llama3.1:8b]
```

**Works with**: [Universal properties](#universal-properties)

**Example**:
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
  register: ollama_result
```

[See Presets Guide →](../presets.md) | [See complete preset documentation in Actions Guide →](actions.md#preset)

---

### include

Load and execute steps from other configuration files.

**Basic form**:
```yaml
- include: ./tasks/common.yml
```

**With conditional**:
```yaml
- include: ./tasks/linux.yml
  when: os == "linux"
```

**Works with**: `name`, `when`, `tags`, `with_items` (NOT shell-specific properties or `register`)

[See complete include documentation in Actions Guide →](actions.md#include)

---

### include_vars

Load variables from external YAML files.

**Basic form**:
```yaml
- include_vars: ./vars/production.yml
```

**Dynamic path**:
```yaml
- include_vars: ./vars/{{environment}}.yml
```

**Works with**: `name`, `when`, `tags` (NOT loops, shell-specific properties, or `register`)

[See complete include_vars documentation in Actions Guide →](actions.md#include-vars)

---

### vars

Define inline variables.

**Basic form**:
```yaml
- vars:
    app_name: MyApp
    version: "1.0.0"
    environment: production
```

**Nested variables**:
```yaml
- vars:
    app:
      name: MyApp
      version: "1.0.0"
    ports:
      web: 8080
      api: 8081
```

[See Variables Guide →](variables.md)

---

## Universal Properties

These properties work with **all** actions (unless noted otherwise).

### name

**Type**: `string`
**Applies to**: All actions
**Required**: No (but recommended)

Human-readable description displayed in output.

```yaml
- name: Install dependencies
  shell: npm install
```

---

### when

**Type**: `string` (expression)
**Applies to**: All actions
**Required**: No

Conditional execution - step runs only if expression evaluates to `true`.

**Operators**: `==`, `!=`, `>`, `<`, `>=`, `<=`, `&&`, `||`, `!`, `in`, `not in`

```yaml
# OS check
- shell: brew install git
  when: os == "darwin"

# Multiple conditions
- shell: install-package
  when: os == "linux" && memory_total_mb >= 8000

# Using register results
- shell: which docker
  register: docker_check

- shell: echo "Docker not found"
  when: docker_check.rc != 0
```

[See Control Flow Guide →](control-flow.md#conditionals)

---

### creates

**Type**: `string` (path)
**Applies to**: All actions
**Required**: No

Skip step if the specified file path exists (idempotency check).

```yaml
# Skip if binary already exists
- shell: go build -o myapp
  creates: ./myapp

# Skip if installation marker exists
- unarchive:
    src: /tmp/app.tar.gz
    dest: /opt/app
  creates: /opt/app/.installed
```

**How it works**: Before executing, Mooncake checks if file exists. If yes, step is skipped.

[See Idempotency Examples →](../../examples/idempotency.md)

---

### unless

**Type**: `string` (command)
**Applies to**: All actions
**Required**: No

Skip step if the given shell command succeeds (returns exit code 0).

```yaml
# Skip if service is already enabled
- shell: systemctl enable nginx
  unless: "systemctl is-enabled nginx"

# Skip if user exists
- shell: useradd myuser
  unless: "id myuser"
```

**How it works**: Command is executed silently before the step. If exit code is 0 (success), step is skipped.

[See Idempotency Examples →](../../examples/idempotency.md)

---

### tags

**Type**: `array[string]`
**Applies to**: All actions
**Required**: No

Tags for filtering step execution via `--tags` flag.

```yaml
- shell: npm test
  tags: [test, dev]

- shell: deploy-production
  tags: [prod, deploy]
```

**Usage**:
```bash
# Run only dev steps
mooncake run --config config.yml --tags dev

# Multiple tags (OR logic)
mooncake run --config config.yml --tags dev,test
```

**Behavior**:
- **No tags specified**: All steps run
- **Tags specified**: Only steps with matching tags run

[See Control Flow Guide →](control-flow.md#tags)

---

### with_items

**Type**: `string` (variable reference)
**Applies to**: All actions
**Required**: No

Iterate over list. Step executes once for each item.

**Loop variables available**:
- `{{item}}` - Current item value
- `{{index}}` - Zero-based iteration index
- `{{first}}` - Boolean, true for first iteration
- `{{last}}` - Boolean, true for last iteration

```yaml
# List literal
- shell: echo "{{item}}"
  with_items: [a, b, c]

# Variable reference
- vars:
    packages: [git, vim, tmux]

- shell: brew install {{item}}
  with_items: "{{packages}}"
```

[See Control Flow Guide →](control-flow.md#loops)

---

### with_filetree

**Type**: `string` (path)
**Applies to**: All actions
**Required**: No

Iterate over files in directory tree. Step executes for each file.

**Item properties**:
- `{{item.name}}` - File name
- `{{item.src}}` - Full source path
- `{{item.is_dir}}` - Boolean, true if directory

**Loop variables**:
- `{{index}}` - Zero-based iteration index
- `{{first}}` - Boolean, true for first iteration
- `{{last}}` - Boolean, true for last iteration

```yaml
- shell: cp "{{item.src}}" "/backup/{{item.name}}"
  with_filetree: ./dotfiles
  when: item.is_dir == false
```

[See Control Flow Guide →](control-flow.md#loops)

---

### become

**Type**: `boolean`
**Applies to**: shell, command, file, template, service, preset
**Required**: No
**Default**: `false`

Execute step with sudo privileges.

```yaml
- shell: apt update
  become: true

- file:
    path: /opt/myapp
    state: directory
  become: true
```

**Password methods**:
1. Interactive: `mooncake run --config config.yml --ask-become-pass` or `-K`
2. File: `mooncake run --config config.yml --sudo-pass-file ~/.mooncake/sudo_pass`
3. Environment: `export SUDO_ASKPASS=/usr/bin/ssh-askpass`
4. CLI (insecure): `mooncake run --config config.yml --sudo-pass mypassword --insecure-sudo-pass`

**Platform support**: Linux and macOS only

---

### register

**Type**: `string`
**Applies to**: All actions
**Required**: No

Variable name to store step execution result.

**Result properties**:
- `rc` - Exit code (0 = success)
- `stdout` - Standard output (shell/command only)
- `stderr` - Standard error (shell/command only)
- `failed` - Boolean, true if step failed
- `changed` - Boolean, true if step made changes

```yaml
- shell: whoami
  register: current_user

- shell: echo "User is {{current_user.stdout}}"

# Use in conditions
- shell: which docker
  register: docker_check

- shell: echo "Docker not installed"
  when: docker_check.rc != 0
```

---

## Shell-Specific Properties

These properties **only work with shell and command actions**. They are ignored for file, template, include, etc.

### become_user

**Type**: `string`
**Applies to**: shell, command
**Required**: No
**Default**: `root`

Specify which user to become when using `become`.

```yaml
- shell: psql -c "SELECT version()"
  become: true
  become_user: postgres
```

---

### env

**Type**: `object` (string keys and values)
**Applies to**: shell, command
**Required**: No

Environment variables for command execution. Values support template rendering.

```yaml
- shell: make build
  env:
    CC: gcc-11
    CFLAGS: "-O2 -Wall"
    PATH: "/custom/bin:$PATH"
    PROJECT: "{{project_name}}"
```

---

### cwd

**Type**: `string`
**Applies to**: shell, command
**Required**: No

Working directory for command execution. Supports template rendering.

```yaml
- shell: npm install
  cwd: /opt/myproject

- shell: ./build.sh
  cwd: "/home/{{user}}/projects/{{project}}"
```

---

### timeout

**Type**: `string` (duration)
**Applies to**: shell, command
**Required**: No

Maximum execution time. Command terminates with exit code 124 on timeout.

**Format**: Number + unit (`ns`, `us`, `µs`, `ms`, `s`, `m`, `h`)

```yaml
- shell: ./slow-script.sh
  timeout: 30s

- shell: npm run build
  timeout: 10m
```

---

### retries

**Type**: `integer`
**Applies to**: shell, command
**Required**: No
**Default**: `0`
**Range**: 0-100

Number of times to retry on failure. Total attempts = retries + 1.

```yaml
- shell: curl https://api.example.com/data
  retries: 3
  retry_delay: 5s
```

---

### retry_delay

**Type**: `string` (duration)
**Applies to**: shell, command
**Required**: No
**Default**: No delay

Delay between retry attempts. Only used when `retries` is set.

**Format**: Number + unit (`ns`, `us`, `µs`, `ms`, `s`, `m`, `h`)

```yaml
- shell: nc -z localhost 8080
  retries: 10
  retry_delay: 2s
```

---

### changed_when

**Type**: `string` (expression)
**Applies to**: shell, command
**Required**: No

Expression to override changed status. Evaluated after command execution.

**Available variables**:
- `result.rc` - Exit code
- `result.stdout` - Standard output
- `result.stderr` - Standard error
- `result.failed` - Boolean failure status

```yaml
# Never changed
- shell: cat /etc/os-release
  changed_when: false

# Git pull - only changed if updated
- shell: git pull
  changed_when: "'Already up to date' not in result.stdout"

# Based on exit code
- shell: check-update
  changed_when: "result.rc == 0"
```

---

### failed_when

**Type**: `string` (expression)
**Applies to**: shell, command
**Required**: No

Expression to override failure status. Evaluated after command execution.

**Available variables**: Same as `changed_when`

```yaml
# Grep - 0=found, 1=not found, 2+=error
- shell: grep "pattern" file.txt
  failed_when: "result.rc >= 2"

# Multiple acceptable exit codes
- shell: ./script.sh
  failed_when: "result.rc not in [0, 2]"

# Check stderr
- shell: ./command
  failed_when: "'ERROR' in result.stderr or 'FATAL' in result.stderr"

# Never fail
- shell: best-effort-command
  failed_when: false
```

---

## System Facts Reference

Available automatically in all steps. View with `mooncake facts` or `mooncake facts --format json`.

### Basic Facts

| Variable | Type | Example | Description |
|----------|------|---------|-------------|
| `os` | string | `"linux"`, `"darwin"`, `"windows"` | Operating system |
| `arch` | string | `"amd64"`, `"arm64"` | CPU architecture |
| `hostname` | string | `"server01"` | System hostname |
| `username` | string | `"admin"` | Current username |
| `user_home` | string | `"/home/admin"` | Current user's home directory |
| `kernel_version` | string | `"6.5.0-14"` | Kernel/Darwin version |

### Distribution Info

| Variable | Type | Example | Description |
|----------|------|---------|-------------|
| `distribution` | string | `"ubuntu"`, `"macos"` | OS distribution |
| `distribution_version` | string | `"22.04"`, `"15.7"` | Full version |
| `distribution_major` | string | `"22"`, `"15"` | Major version number |

### CPU Facts

| Variable | Type | Example | Description |
|----------|------|---------|-------------|
| `cpu_cores` | integer | `8` | Number of CPU cores |
| `cpu_model` | string | `"Intel Core i7-10700K"` | CPU model name |
| `cpu_flags` | array | `["avx", "avx2", "sse4_2"]` | CPU feature flags |
| `cpu_flags_string` | string | `"avx avx2 sse4_2"` | CPU flags as string |

### Memory Facts

| Variable | Type | Example | Description |
|----------|------|---------|-------------|
| `memory_total_mb` | integer | `16384` | Total RAM in MB |
| `memory_free_mb` | integer | `8192` | Available RAM in MB |
| `swap_total_mb` | integer | `4096` | Total swap space in MB |
| `swap_free_mb` | integer | `2048` | Available swap space in MB |

### Network Facts

| Variable | Type | Example | Description |
|----------|------|---------|-------------|
| `ip_addresses` | array | `["192.168.1.100"]` | List of IP addresses |
| `ip_addresses_string` | string | `"192.168.1.100"` | IP addresses as string |
| `default_gateway` | string | `"192.168.1.1"` | Default network gateway |
| `dns_servers` | array | `["8.8.8.8", "1.1.1.1"]` | DNS servers |
| `dns_servers_string` | string | `"8.8.8.8, 1.1.1.1"` | DNS servers as string |
| `network_interfaces` | array | See below | Network interface details |

**NetworkInterface Structure**:
```yaml
name: "eth0"
mac_address: "00:11:22:33:44:55"
mtu: 1500
addresses: ["192.168.1.100/24"]
up: true
```

### Storage Facts

| Variable | Type | Example | Description |
|----------|------|---------|-------------|
| `disks` | array | See below | Disk/mount information |

**Disk Structure**:
```yaml
device: "/dev/sda1"
mount_point: "/"
filesystem: "ext4"
size_gb: 500
used_gb: 250
avail_gb: 250
used_pct: 50
```

### GPU Facts

| Variable | Type | Example | Description |
|----------|------|---------|-------------|
| `gpus` | array | See below | GPU information |

**GPU Structure**:
```yaml
vendor: "nvidia"           # nvidia, amd, intel, apple
model: "GeForce RTX 4090"
memory: "24GB"
driver: "535.54.03"
cuda_version: "12.3"       # NVIDIA only
```

### Software Detection

| Variable | Type | Example | Description |
|----------|------|---------|-------------|
| `package_manager` | string | `"apt"`, `"brew"`, `"yum"` | Detected package manager |
| `python_version` | string | `"3.11.5"` | Python version (if installed) |
| `docker_version` | string | `"24.0.7"` | Docker version (if installed) |
| `git_version` | string | `"2.43.0"` | Git version (if installed) |
| `go_version` | string | `"1.21.5"` | Go version (if installed) |

---

## File Mode Reference

Common permission values for `mode` property:

| Mode | Permissions | Use Case |
|------|-------------|----------|
| `"0755"` | `rwxr-xr-x` | Directories, executables |
| `"0644"` | `rw-r--r--` | Regular files, configs |
| `"0600"` | `rw-------` | Private files, secrets |
| `"0700"` | `rwx------` | Private directories |
| `"0777"` | `rwxrwxrwx` | World-writable (avoid!) |

---

## Expression Syntax Reference

Used in `when`, `changed_when`, `failed_when`.

### Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `==` | Equals | `os == "linux"` |
| `!=` | Not equals | `arch != "arm64"` |
| `>` | Greater than | `memory_total_mb > 8000` |
| `<` | Less than | `cpu_cores < 4` |
| `>=` | Greater or equal | `distribution_major >= "20"` |
| `<=` | Less or equal | `cpu_cores <= 16` |
| `&&` | Logical AND | `os == "linux" && arch == "amd64"` |
| `||` | Logical OR | `os == "linux" || os == "darwin"` |
| `!` | Logical NOT | `!(os == "windows")` |
| `in` | Contains (lists) | `result.rc in [0, 2]` |
| `not in` | Not contains | `result.rc not in [1, 2]` |

### Special Values

| Value | Description |
|-------|-------------|
| `true` | Boolean true |
| `false` | Boolean false |
| `"string"` | String literal (single or double quotes) |
| `123` | Number literal |

---

## See Also

- [Actions Guide](actions.md) - Detailed action documentation with examples
- [Control Flow Guide](control-flow.md) - Conditionals, loops, tags
- [Variables Guide](variables.md) - Variable management
- [Examples](../../examples/index.md) - Practical examples
