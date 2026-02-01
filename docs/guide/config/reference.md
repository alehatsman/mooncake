# Configuration Reference

Complete reference for all configuration properties.

## Step Properties

Every step in your configuration can use these properties. Properties are grouped by function.

### Quick Reference Table

| Property | Type | Applies To | Description |
|----------|------|------------|-------------|
| **Identification** ||||
| `name` | string | All | Human-readable step name |
| **Actions** (one required) ||||
| `shell` | string | Shell | Shell command to execute |
| `file` | object | File | File/directory operation |
| `template` | object | Template | Template rendering |
| `include` | string | Include | Include steps from another file |
| `include_vars` | string | Variables | Load variables from file |
| `vars` | object | Variables | Define inline variables |
| **Control Flow** ||||
| `when` | string | All | Conditional expression |
| `creates` | string | All | Skip if file exists (idempotency) |
| `unless` | string | All | Skip if command succeeds (idempotency) |
| `tags` | array[string] | All | Tags for filtering |
| **Loops** ||||
| `with_items` | string | All | Iterate over list |
| `with_filetree` | string | All | Iterate over directory |
| **Privilege** ||||
| `become` | boolean | shell, file, template | Execute with sudo |
| `become_user` | string | shell, file, template | User for sudo (e.g., 'postgres') |
| **Shell Execution Control** ||||
| `env` | object | shell only | Environment variables |
| `cwd` | string | shell only | Working directory |
| `timeout` | string | shell only | Maximum execution time |
| `retries` | integer | shell only | Number of retry attempts |
| `retry_delay` | string | shell only | Delay between retries |
| **Result Control** ||||
| `changed_when` | string | shell only | Override changed status |
| `failed_when` | string | shell only | Override failure status |
| `register` | string | All | Variable name to store result |

## Property Details

### name

**Type:** `string`
**Applies to:** All actions
**Required:** No (but recommended)

Human-readable description displayed in output.

```yaml
- name: Install dependencies
  shell: npm install
```

---

### shell

**Type:** `string`
**Applies to:** Shell action
**Required:** When using shell action

Command to execute in bash shell.

```yaml
- shell: echo "Hello"

# Multi-line
- shell: |
    echo "Line 1"
    echo "Line 2"

# With variables
- shell: echo "{{message}}"
```

**Supports:**
- Template variable substitution
- Multi-line commands with `|`
- All shell-specific fields

---

### file

**Type:** `object`
**Applies to:** File action
**Required:** When using file action

File or directory operation.

**Properties:**

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `path` | string | Yes | File/directory path |
| `state` | string | No | `file`, `directory`, or `absent` |
| `content` | string | No | Content to write to file |
| `mode` | string | No | Permissions (e.g., `"0644"`, `"0755"`) |

```yaml
# Create directory
- file:
    path: /tmp/myapp
    state: directory
    mode: "0755"

# Create file with content
- file:
    path: /tmp/config.txt
    state: file
    content: "key: value"
    mode: "0644"
```

---

### template

**Type:** `object`
**Applies to:** Template action
**Required:** When using template action

Render pongo2 template file.

**Properties:**

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `src` | string | Yes | Source template file path |
| `dest` | string | Yes | Destination file path |
| `vars` | object | No | Additional variables for template |
| `mode` | string | No | Permissions (e.g., `"0644"`) |

```yaml
- template:
    src: ./config.yml.j2
    dest: /etc/app/config.yml
    mode: "0644"
    vars:
      port: 8080
      debug: true
```

---

### include

**Type:** `string`
**Applies to:** Include action
**Required:** When using include action

Path to YAML file containing steps to include.

```yaml
- include: ./tasks/common.yml

# With condition
- include: ./tasks/linux.yml
  when: os == "linux"

# With variables
- include: ./tasks/{{env}}.yml
```

---

### include_vars

**Type:** `string`
**Applies to:** Variable loading action
**Required:** When using include_vars action

Path to YAML file containing variables to load.

```yaml
- include_vars: ./vars/production.yml

# Dynamic
- include_vars: ./vars/{{environment}}.yml
```

---

### vars

**Type:** `object`
**Applies to:** Variable definition action
**Required:** When using vars action

Define inline variables.

```yaml
- vars:
    app_name: MyApp
    version: "1.0.0"
    environment: production
    ports:
      web: 8080
      api: 8081
```

---

### when

**Type:** `string`
**Applies to:** All actions
**Required:** No

Conditional expression. Step executes only if expression evaluates to `true`.

**Operators:** `==`, `!=`, `>`, `<`, `>=`, `<=`, `&&`, `||`, `!`

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

---

### creates

**Type:** `string`
**Applies to:** All actions
**Required:** No

Skip step if the specified file path exists. Useful for idempotency - prevents re-running steps that have already completed.

```yaml
# Skip if binary already exists
- name: Compile application
  shell: go build -o myapp
  creates: ./myapp

# Skip if installation marker exists
- name: Install package
  shell: apt-get install -y package
  creates: /usr/bin/package
```

**Path rendering:**
The path is rendered through the template engine, so variables are supported:

```yaml
- name: Set output directory
  vars:
    output_dir: /opt/myproject

- name: Build project
  shell: make build
  creates: "{{ output_dir }}/myapp"
```

**How it works:**
- Before executing the step, Mooncake checks if the file exists using `os.Stat()`
- If the file exists, the step is skipped with reason `"idempotency:creates: /path/to/file"`
- If the file doesn't exist, the step executes normally

**Evaluation order:**
Idempotency conditions are evaluated after `when` and `tags` filters:
1. `when` expression (if present)
2. `tags` filter (if specified)
3. `creates` check (if specified)
4. `unless` command (if specified)
5. Execute step

**See also:** [Idempotency Examples](../../examples/idempotency.md)

---

### unless

**Type:** `string`
**Applies to:** All actions
**Required:** No

Skip step if the given shell command succeeds (returns exit code 0). Provides flexible idempotency control based on system state.

```yaml
# Skip if service is already enabled
- name: Enable nginx
  shell: systemctl enable nginx
  unless: "systemctl is-enabled nginx"

# Skip if database table exists
- name: Initialize database
  shell: psql -f schema.sql mydb
  unless: "psql -c '\\dt' mydb | grep users"
```

**Command rendering:**
The command is rendered through the template engine:

```yaml
- name: Set database name
  vars:
    db_name: production

- name: Create database
  shell: createdb {{ db_name }}
  unless: "psql -l | grep {{ db_name }}"
```

**How it works:**
- Before executing the step, Mooncake runs the `unless` command with `sh -c`
- The command is executed silently (no output logged)
- If the command exits with code 0 (success), the step is skipped with reason `"idempotency:unless: command"`
- If the command exits with non-zero code (failure), the step executes normally

**Important notes:**
- The `unless` command is run silently to avoid cluttering logs
- Only the exit code is checked - stdout/stderr are discarded
- Use simple, fast commands to avoid performance impact
- The command runs in a shell (`sh -c`), so shell features like pipes and redirects work

**Common patterns:**
```yaml
# Check if file exists
unless: "test -f /path/to/file"

# Check if package installed
unless: "dpkg -l package-name | grep '^ii'"

# Check if user exists
unless: "id username"

# Check if service running
unless: "systemctl is-active service"

# Check for specific content
unless: "grep 'pattern' /etc/config"
```

**See also:** [Idempotency Examples](../../examples/idempotency.md)

---

### tags

**Type:** `array[string]`
**Applies to:** All actions
**Required:** No

Tags for filtering step execution via `--tags` flag.

```yaml
- shell: npm test
  tags: [test, dev]

- shell: deploy-production
  tags: [prod, deploy]
```

**Usage:**
```bash
# Run only dev steps
mooncake run --config config.yml --tags dev

# Multiple tags (OR logic)
mooncake run --config config.yml --tags dev,test
```

---

### with_items

**Type:** `string`
**Applies to:** All actions
**Required:** No

Iterate over list. Step executes once for each item.

**Loop variables available:**
- `{{item}}` - Current item value
- `{{index}}` - Zero-based iteration index (0, 1, 2, ...)
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

# Using loop variables
- name: "Package {{index + 1}}/{{packages|length}}: {{item}}"
  shell: brew install {{item}}
  with_items: "{{packages}}"

# First/last checks
- shell: echo "Processing {{item}}"
  with_items: [a, b, c]
  when: first == true  # Only first iteration
```

---

### with_filetree

**Type:** `string`
**Applies to:** All actions
**Required:** No

Iterate over files in directory tree. Step executes for each file in deterministic (sorted) order.

**Item properties:**
- `{{item.name}}` - File name
- `{{item.src}}` - Full source path
- `{{item.is_dir}}` - Boolean, true if directory

**Loop variables available:**
- `{{index}}` - Zero-based iteration index
- `{{first}}` - Boolean, true for first iteration
- `{{last}}` - Boolean, true for last iteration

```yaml
- shell: cp "{{item.src}}" "/backup/{{item.name}}"
  with_filetree: ./dotfiles
  when: item.is_dir == false

# Using loop variables
- name: "[{{index + 1}}] Copying {{item.name}}"
  shell: cp "{{item.src}}" "~/{{item.name}}"
  with_filetree: ./dotfiles
```

---

### become

**Type:** `boolean`
**Applies to:** shell, file, template actions
**Required:** No
**Default:** `false`

Execute step with sudo privileges.

```yaml
- shell: apt update
  become: true

- file:
    path: /opt/myapp
    state: directory
  become: true

- template:
    src: nginx.conf.j2
    dest: /etc/nginx/nginx.conf
  become: true
```

**Password Input Methods:**

You must provide a sudo password using one of these methods (mutually exclusive):

1. **Interactive prompt (recommended):**
   ```bash
   mooncake run --config config.yml --ask-become-pass
   # or
   mooncake run --config config.yml -K
   ```

2. **File-based (secure):**
   ```bash
   echo "mypassword" > ~/.mooncake/sudo_pass
   chmod 0600 ~/.mooncake/sudo_pass
   mooncake run --config config.yml --sudo-pass-file ~/.mooncake/sudo_pass
   ```
   ⚠️ File must have 0600 permissions and be owned by current user

3. **Environment variable (password manager integration):**
   ```bash
   export SUDO_ASKPASS=/usr/bin/ssh-askpass
   mooncake run --config config.yml
   ```

4. **CLI flag (insecure, not recommended):**
   ```bash
   mooncake run --config config.yml --sudo-pass mypassword --insecure-sudo-pass
   ```
   ⚠️ **WARNING:** Password visible in shell history and process list. Requires `--insecure-sudo-pass` flag.

**Security:**
- Passwords are automatically redacted from all log output
- Platform support: Linux and macOS only
- User must have sudo privileges

---

### become_user

**Type:** `string`
**Applies to:** shell, file, template actions
**Required:** No
**Default:** `root`

Specify which user to become when using `become`. Works with shell commands, file operations, and template rendering.

```yaml
- name: Run as postgres user
  shell: psql -c "SELECT version()"
  become: true
  become_user: postgres

- name: Create file owned by app user
  file:
    path: /opt/myapp/config.json
    content: '{"key": "value"}'
    state: file
  become: true
  become_user: appuser

- name: Deploy config as nginx user
  template:
    src: site.conf.j2
    dest: /etc/nginx/sites-enabled/mysite.conf
  become: true
  become_user: nginx
```

---

### env

**Type:** `object` (string keys and values)
**Applies to:** shell only
**Required:** No

⚠️ **Shell commands only** - Ignored for file/template/include.

Environment variables for shell command execution. Values support template rendering.

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

**Type:** `string`
**Applies to:** shell only
**Required:** No

⚠️ **Shell commands only** - Ignored for file/template/include.

Working directory for shell command execution. Supports template rendering.

```yaml
- shell: npm install
  cwd: /opt/myproject

- shell: ./build.sh
  cwd: "/home/{{user}}/projects/{{project}}"
```

---

### timeout

**Type:** `string` (duration)
**Applies to:** shell only
**Required:** No

⚠️ **Shell commands only** - Ignored for file/template/include.

Maximum execution time. Command terminates with exit code 124 on timeout.

**Format:** Number + unit (`ns`, `us`, `µs`, `ms`, `s`, `m`, `h`)

```yaml
- shell: ./slow-script.sh
  timeout: 30s

- shell: npm run build
  timeout: 10m

- shell: integration-tests
  timeout: 1h
```

---

### retries

**Type:** `integer`
**Applies to:** shell only
**Required:** No
**Default:** `0`
**Range:** 0-100

⚠️ **Shell commands only** - Ignored for file/template/include.

Number of times to retry on failure. Total attempts = retries + 1.

```yaml
- shell: curl https://api.example.com/data
  retries: 3

- shell: docker pull myimage:latest
  retries: 5
  retry_delay: 10s
```

---

### retry_delay

**Type:** `string` (duration)
**Applies to:** shell only
**Required:** No
**Default:** No delay

⚠️ **Shell commands only** - Ignored for file/template/include.

Delay between retry attempts. Only used when `retries` is set.

**Format:** Number + unit (`ns`, `us`, `µs`, `ms`, `s`, `m`, `h`)

```yaml
- shell: nc -z localhost 8080
  retries: 10
  retry_delay: 2s
```

---

### changed_when

**Type:** `string` (expression)
**Applies to:** shell only
**Required:** No

⚠️ **Shell commands only** - Ignored for file/template/include.

Expression to override changed status. Evaluated after command execution.

**Available variables:**
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

**Type:** `string` (expression)
**Applies to:** shell only
**Required:** No

⚠️ **Shell commands only** - Ignored for file/template/include.

Expression to override failure status. Evaluated after command execution.

**Available variables:** Same as `changed_when`

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

# Never fail (ignore errors)
- shell: best-effort-command
  failed_when: false
```

---

### register

**Type:** `string`
**Applies to:** All actions
**Required:** No

Variable name to store step execution result.

**Result properties:**
- `rc` - Exit code (0 = success)
- `stdout` - Standard output (shell only)
- `stderr` - Standard error (shell only)
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

## System Facts Reference

Available automatically in all steps. View with `mooncake explain`.

### Basic Facts

| Variable | Type | Example | Description |
|----------|------|---------|-------------|
| `os` | string | `"linux"`, `"darwin"`, `"windows"` | Operating system |
| `arch` | string | `"amd64"`, `"arm64"` | CPU architecture |
| `hostname` | string | `"myserver"` | System hostname |
| `user_home` | string | `"/home/user"` | Current user's home directory |

### Distribution Info

| Variable | Type | Example | Description |
|----------|------|---------|-------------|
| `distribution` | string | `"ubuntu"`, `"macos"` | OS distribution |
| `distribution_version` | string | `"22.04"`, `"13.0"` | Full version |
| `distribution_major` | string | `"22"`, `"13"` | Major version number |

### Hardware Facts

| Variable | Type | Example | Description |
|----------|------|---------|-------------|
| `cpu_cores` | integer | `8` | Number of CPU cores |
| `memory_total_mb` | integer | `16384` | Total RAM in MB |

### Software Detection

| Variable | Type | Example | Description |
|----------|------|---------|-------------|
| `package_manager` | string | `"apt"`, `"brew"`, `"yum"` | Detected package manager |
| `python_version` | string | `"3.10.0"` | Python version (if installed) |

### Network Facts

| Variable | Type | Example | Description |
|----------|------|---------|-------------|
| `ip_addresses` | array | `["192.168.1.100"]` | List of IP addresses |
| `ip_addresses_string` | string | `"192.168.1.100"` | First IP address |

## File Mode Reference

Common permission values for `mode` property:

| Mode | Permissions | Use Case |
|------|-------------|----------|
| `"0755"` | `rwxr-xr-x` | Directories, executables |
| `"0644"` | `rw-r--r--` | Regular files, configs |
| `"0600"` | `rw-------` | Private files, secrets |
| `"0700"` | `rwx------` | Private directories |
| `"0777"` | `rwxrwxrwx` | World-writable (avoid!) |

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

## See Also

- [Actions Guide](actions.md) - Detailed action documentation
- [Control Flow Guide](control-flow.md) - Conditionals, loops, tags
- [Variables Guide](variables.md) - Variable management
- [Examples](../../examples/index.md) - Practical examples
