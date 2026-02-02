# Examples

Learn Mooncake through practical, hands-on examples. This page contains all examples from beginner to advanced, organized as a step-by-step learning path.

---

## Running Examples

All examples are in the `examples/` directory of the repository:

```bash
# Clone the repo
git clone https://github.com/alehatsman/mooncake.git
cd mooncake

# Run an example
mooncake run --config examples/01-hello-world/config.yml
```

---

## Beginner Examples

### 01 - Hello World

**Start here!** This is the simplest possible Mooncake configuration.

#### What You'll Learn

- Running basic shell commands
- Using global system variables
- Multi-line shell commands

#### Quick Start

```bash
cd examples/01-hello-world
mooncake run --config config.yml
```

#### What It Does

1. Prints a hello message
2. Runs system commands to show OS info
3. Uses Mooncake's global variables to display OS and architecture

#### Key Concepts

**Shell Commands**

Execute commands with the `shell` action:
```yaml
- name: Print message
  shell: echo "Hello!"
```

**Multi-line Commands**

Use `|` for multiple commands:
```yaml
- name: Multiple commands
  shell: |
    echo "First command"
    echo "Second command"
```

**Global Variables**

Mooncake automatically provides system information:
- `{{os}}` - Operating system (linux, darwin, windows)
- `{{arch}}` - Architecture (amd64, arm64, etc.)

#### Output Example

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

---

### 02 - Variables and System Facts

Learn how to define custom variables and use Mooncake's comprehensive system facts.

#### What You'll Learn

- Defining custom variables with `vars`
- Using all available system facts
- Combining custom variables with system facts
- Using variables in file operations

#### Quick Start

```bash
cd examples/02-variables-and-facts
mooncake run --config config.yml
```

#### What It Does

1. Defines custom application variables
2. Displays all system facts (OS, hardware, network, software)
3. Creates files using both custom variables and system facts

#### Key Concepts

**Custom Variables**

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

**System Facts**

Mooncake automatically collects system information:

| Category | Variable | Description |
|----------|----------|-------------|
| **Basic** | `os`, `arch`, `hostname`, `user_home` | System basics |
| **Hardware** | `cpu_cores`, `memory_total_mb` | Hardware info |
| **Distribution** | `distribution`, `distribution_version`, `distribution_major` | OS distribution |
| **Software** | `package_manager`, `python_version` | Installed software |
| **Network** | `ip_addresses`, `ip_addresses_string` | Network info |

**Variable Substitution**

Variables work everywhere:
```yaml
- file:
    path: "/tmp/{{app_name}}-{{version}}-{{os}}"
    state: directory
```

**Seeing All Facts**

Run `mooncake facts` to see all facts for your system:
```bash
mooncake facts
# or as JSON
mooncake facts --format json
```

---

### 03 - Files and Directories

Learn how to create and manage files and directories with Mooncake.

#### What You'll Learn

- Creating directories with `state: directory`
- Creating files with `state: file`
- Setting file permissions with `mode`
- Adding content to files

#### Quick Start

```bash
cd examples/03-files-and-directories
mooncake run --config config.yml
```

#### What It Does

1. Creates application directory structure
2. Creates files with specific content
3. Sets appropriate permissions (755 for directories, 644 for files)
4. Creates executable scripts

#### Key Concepts

**Creating Directories**

```yaml
- name: Create directory
  file:
    path: /tmp/myapp
    state: directory
    mode: "0755"  # rwxr-xr-x
```

**Creating Empty Files**

```yaml
- name: Create empty file
  file:
    path: /tmp/file.txt
    state: file
    mode: "0644"  # rw-r--r--
```

**Creating Files with Content**

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

**File Permissions**

| Mode | Meaning | Use Case |
|------|---------|----------|
| 0755 | rwxr-xr-x | Directories, executable scripts |
| 0644 | rw-r--r-- | Regular files, configs |
| 0600 | rw------- | Private files, secrets |
| 0700 | rwx------ | Private directories |

---

### 04 - Conditionals

Learn how to conditionally execute steps based on system properties or variables.

#### What You'll Learn

- Using `when` for conditional execution
- OS and architecture detection
- Complex conditions with logical operators
- Combining conditionals with tags

#### Quick Start

```bash
cd examples/04-conditionals

# Run all steps (only matching conditions will execute)
mooncake run --config config.yml

# Run only dev-tagged steps
mooncake run --config config.yml --tags dev
```

#### What It Does

1. Demonstrates steps that always run
2. Shows OS-specific steps (macOS vs Linux)
3. Shows architecture-specific steps
4. Demonstrates tag filtering

#### Key Concepts

**Basic Conditionals**

Use `when` to conditionally execute steps:
```yaml
- name: Linux only
  shell: echo "Running on Linux"
  when: os == "linux"
```

**Available System Variables**

- `os` - darwin, linux, windows
- `arch` - amd64, arm64, 386, etc.
- `distribution` - ubuntu, debian, centos, macos, etc.
- `distribution_major` - major version number
- `package_manager` - apt, yum, brew, pacman, etc.

**Comparison Operators**

- `==` - equals
- `!=` - not equals
- `>`, `<`, `>=`, `<=` - comparisons
- `&&` - logical AND
- `||` - logical OR
- `!` - logical NOT

**Complex Conditions**

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

**Tags vs Conditionals**

- **Conditionals (`when`)**: Evaluated at runtime based on system facts
- **Tags**: User-controlled filtering via CLI `--tags` flag

---

## Intermediate Examples

### 05 - Templates

Learn how to render configuration files from templates using pongo2 syntax.

#### What You'll Learn

- Rendering `.j2` template files
- Using variables in templates
- Template conditionals (`{% if %}`)
- Template loops (`{% for %}`)
- Passing additional vars to templates

#### Quick Start

```bash
cd examples/05-templates
mooncake run --config config.yml

# Check the rendered files
ls -lh /tmp/mooncake-templates/
cat /tmp/mooncake-templates/config.yml
```

#### What It Does

1. Defines variables for application, server, and database config
2. Renders application config with loops and conditionals
3. Renders nginx config with optional SSL
4. Creates executable script from template
5. Renders same template with different variables

#### Key Concepts

**Template Action**

```yaml
- name: Render config
  template:
    src: ./templates/config.yml.j2
    dest: /tmp/config.yml
    mode: "0644"
```

**Template Syntax (pongo2)**

Variables:
```jinja
{{ variable_name }}
{{ nested.property }}
```

Conditionals:
```jinja
{% if debug %}
  debug: true
{% else %}
  debug: false
{% endif %}
```

Loops:
```jinja
{% for item in items %}
  - {{ item }}
{% endfor %}
```

Filters:
```jinja
{{ path | expanduser }}  # Expands ~ to home directory
{{ text | upper }}       # Convert to uppercase
```

**Passing Additional Vars**

Override variables for specific templates:
```yaml
- template:
    src: ./templates/config.yml.j2
    dest: /tmp/prod-config.yml
    vars:
      environment: production
      debug: false
```

**Common Use Cases**

- Config files (app.yml, nginx.conf, etc.)
- Shell scripts (deployment, setup)
- Systemd units (service files)
- Dotfiles (.bashrc, .vimrc with customization)

---

### 06 - Loops

Learn how to iterate over lists and files to avoid repetition.

#### What You'll Learn

- Iterating over lists with `with_items`
- Iterating over files with `with_filetree`
- Using the `{{ item }}` variable
- Accessing file properties in loops

#### Quick Start

```bash
cd examples/06-loops

# Run list iteration example
mooncake run --config with-items.yml

# Run file tree iteration example
mooncake run --config with-filetree/config.yml
```

#### Examples Included

**1. with-items.yml - List Iteration**

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

**2. with-filetree/ - File Tree Iteration**

Iterate over files in a directory:
```yaml
- name: Copy dotfile
  shell: cp "{{ item.src }}" "/tmp/backup/{{ item.name }}"
  with_filetree: ./files
```

#### Key Concepts

**List Iteration (with_items)**

```yaml
- vars:
    users: [alice, bob, charlie]

- name: Create user directory
  file:
    path: "/home/{{ item }}"
    state: directory
  with_items: "{{ users }}"
```

This creates `/home/alice`, `/home/bob`, `/home/charlie`

**File Tree Iteration (with_filetree)**

```yaml
- name: Process file
  shell: echo "Processing {{ item.name }}"
  with_filetree: ./dotfiles
  when: item.is_dir == false
```

Available properties:

- `item.src` - Full source path
- `item.name` - File name
- `item.is_dir` - Boolean, true if directory

**Filtering in Loops**

Skip directories:
```yaml
- name: Copy files only
  shell: cp "{{ item.src }}" "/tmp/{{ item.name }}"
  with_filetree: ./files
  when: item.is_dir == false
```

**Real-World Use Cases**

with_items:

- Installing multiple packages
- Creating multiple users/groups
- Setting up multiple services

with_filetree:

- Managing dotfiles
- Deploying configuration directories
- Backing up files

---

### 07 - Register

Learn how to capture command output and use it in subsequent steps.

#### What You'll Learn

- Capturing output with `register`
- Accessing stdout, stderr, and return codes
- Using captured data in conditionals
- Detecting if operations made changes

#### Quick Start

```bash
cd examples/07-register
mooncake run --config config.yml
```

#### What It Does

1. Checks if git is installed and captures the result
2. Uses return code to conditionally show messages
3. Captures username and uses it in file paths
4. Captures OS version and displays it
5. Detects if file operations made changes

#### Key Concepts

**Basic Registration**

```yaml
- name: Check if git exists
  shell: which git
  register: git_check

- name: Use the result
  shell: echo "Git is at {{ git_check.stdout }}"
  when: git_check.rc == 0
```

**Available Fields**

For shell commands:

- `register_name.stdout` - Standard output
- `register_name.stderr` - Standard error
- `register_name.rc` - Return/exit code (0 = success)
- `register_name.failed` - Boolean, true if rc != 0
- `register_name.changed` - Boolean, always true for shell

For file operations:

- `register_name.rc` - 0 for success, 1 for failure
- `register_name.failed` - Boolean
- `register_name.changed` - Boolean, true if file created/modified

**Using in Conditionals**

```yaml
- shell: test -f /tmp/file.txt
  register: file_check

- shell: echo "File exists"
  when: file_check.rc == 0

- shell: echo "File not found"
  when: file_check.rc != 0
```

**Using in Templates**

```yaml
- shell: whoami
  register: current_user

- file:
    path: "/tmp/{{ current_user.stdout }}_config.txt"
    state: file
    content: "User: {{ current_user.stdout }}"
```

**Change Detection**

```yaml
- file:
    path: /tmp/test.txt
    state: file
    content: "test"
  register: result

- shell: echo "File was created or modified"
  when: result.changed == true
```

**Common Patterns**

Checking for command existence:
```yaml
- shell: which docker
  register: docker_check

- shell: echo "Docker not installed"
  when: docker_check.rc != 0
```

Conditional installation:
```yaml
- shell: python3 --version
  register: python_check

- shell: apt install python3
  become: true
  when: python_check.rc != 0
```

---

## Advanced Examples

### 08 - Tags

Learn how to use tags to selectively run parts of your configuration.

#### What You'll Learn

- Adding tags to steps
- Filtering execution with `--tags` flag
- Organizing workflows with tags
- Combining tags with conditionals

#### Quick Start

```bash
cd examples/08-tags

# Run all steps (no tag filter)
mooncake run --config config.yml

# Run only development steps
mooncake run --config config.yml --tags dev

# Run only production steps
mooncake run --config config.yml --tags prod

# Run multiple tag categories
mooncake run --config config.yml --tags dev,test
```

#### What It Does

Demonstrates different tagged workflows:

- Development setup
- Production deployment
- Testing
- Security audits
- Staging deployment

#### Key Concepts

**Adding Tags**

```yaml
- name: Install dev tools
  shell: echo "Installing tools"
  tags:
    - dev
    - tools
```

**Tag Filtering Behavior**

- **No tags specified**: All steps run (including untagged steps)
- **Tags specified (`--tags dev`)**: Only steps with matching tags run
- **Multiple tags (`--tags dev,prod`)**: Steps run if they have ANY of the specified tags (OR logic)

**Tag Organization Strategies**

By Environment:
```yaml
tags: [dev, staging, prod]
```

By Phase:
```yaml
tags: [setup, deploy, test, cleanup]
```

By Component:
```yaml
tags: [database, webserver, cache]
```

By Role:
```yaml
tags: [install, configure, security]
```

**Multiple Tags Per Step**

```yaml
- name: Security audit
  shell: run-security-scan
  tags:
    - test
    - prod
    - security
```

This runs with `--tags test`, `--tags prod`, or `--tags security`

**Combining Tags and Conditionals**

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

**Best Practices**

1. Use consistent naming - Pick a scheme and stick to it
2. Multiple tags per step - Makes filtering more flexible
3. Document your tags - In README or comments
4. Combine with conditionals - For environment + OS filtering

---

### 09 - Sudo / Privilege Escalation

Learn how to execute commands and operations with elevated privileges.

#### What You'll Learn

- Using `become: true` for sudo operations
- Providing sudo password via CLI
- System-level operations
- OS-specific privileged operations

#### Quick Start

```bash
cd examples/09-sudo

# Requires sudo password
mooncake run --config config.yml --sudo-pass <your-password>

# Preview what would run with sudo
mooncake run --config config.yml --sudo-pass <password> --dry-run
```

 **Warning:** This example contains commands that require root privileges. Review the config before running!

#### What It Does

1. Runs regular command (no sudo)
2. Runs privileged command with sudo
3. Updates package list (Linux)
4. Installs system packages
5. Creates system directories and files

#### Key Concepts

**Basic Sudo**

Add `become: true` to run with sudo:
```yaml
- name: System operation
  shell: apt update
  become: true
```

**Providing Password**

Three ways to provide sudo password:

1. Command line (recommended):
```bash
mooncake run --config config.yml --sudo-pass mypassword
```

2. Environment variable:
```bash
export MOONCAKE_SUDO_PASS=mypassword
mooncake run --config config.yml
```

**Which Operations Need Sudo?**

Typically require sudo:

- Package management (`apt`, `yum`, `dnf`)
- System file operations (`/etc`, `/opt`, `/usr/local`)
- Service management (`systemctl`)
- User/group management
- Mounting filesystems
- Network configuration

Don't require sudo:

- User-space operations
- Home directory files
- `/tmp` directory
- Homebrew on macOS (usually)

**File Operations with Sudo**

```yaml
- name: Create system directory
  file:
    path: /opt/myapp
    state: directory
    mode: "0755"
  become: true
```

**OS-Specific Sudo**

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

**Security Considerations**

1. Review before running - Check what commands will execute with sudo
2. Use dry-run - Preview with `--dry-run` first
3. Minimize sudo usage - Only use on steps that require it
4. Specific commands - Don't use `become: true` on untrusted commands
5. Password handling - Be careful with password in shell history

---

### 10 - Multi-File Configurations

Learn how to organize large configurations into multiple files.

#### What You'll Learn

- Splitting configuration into multiple files
- Using `include` to load other configs
- Using `include_vars` to load variables
- Organizing by environment (dev/prod)
- Organizing by platform (Linux/macOS)
- Relative path resolution

#### Quick Start

```bash
cd examples/10-multi-file-configs

# Run with development environment (default)
mooncake run --config main.yml

# Run with specific tags
mooncake run --config main.yml --tags install
```

#### Directory Structure

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

#### What It Does

1. Sets project variables
2. Loads environment-specific variables
3. Runs common setup tasks
4. Runs OS-specific tasks (Linux or macOS)
5. Conditionally runs dev tools setup

#### Key Concepts

**Entry Point (main.yml)**

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

**Including Variable Files**

```yaml
- name: Load development vars
  include_vars: ./vars/development.yml
```

**Including Task Files**

```yaml
- name: Run common setup
  include: ./tasks/common.yml
```

**Relative Path Resolution**

Paths are relative to the **current file**, not the working directory:

```
main.yml:
  include: ./tasks/common.yml  # Relative to main.yml

tasks/common.yml:
  template:
    src: ./templates/config.j2  # Relative to common.yml
```

**Organization Strategies**

By Environment:
```
vars/
  development.yml
  staging.yml
  production.yml
```

By Platform:
```
tasks/
  linux.yml
  macos.yml
  windows.yml
```

By Component:
```
tasks/
  database.yml
  webserver.yml
  cache.yml
```

By Phase:
```
tasks/
  00-prepare.yml
  01-install.yml
  02-configure.yml
  03-deploy.yml
```

**Benefits of Multi-File Organization**

1. Maintainability - Easier to find and update specific parts
2. Reusability - Share tasks across projects
3. Collaboration - Team members can work on different files
4. Testing - Test components independently
5. Clarity - Clear separation of concerns

---

### 11 - Shell Execution Control

Learn advanced execution control for shell commands with timeouts, retries, environment variables, and custom result evaluation.

**Important:** These features are shell-specific and don't apply to file, template, or include operations.

#### What You'll Learn

- Setting command timeouts
- Retrying failed commands
- Configuring retry delays
- Setting environment variables
- Changing working directory
- Custom change detection with `changed_when`
- Custom failure detection with `failed_when`
- Running as different users with `become_user`

#### Quick Start

```bash
cd examples/11-execution-control
mooncake run --config config.yml
```

#### Key Concepts

**Timeouts**

Prevent commands from running indefinitely:
```yaml
- name: Command with timeout
  shell: ./slow-script.sh
  timeout: 30s
```

**Retries**

Automatically retry failed operations:
```yaml
- name: Download file
  shell: curl -O https://example.com/file.tar.gz
  retries: 3
  retry_delay: 5s
```

**Environment Variables**

Set custom environment for commands:
```yaml
- name: Build with custom env
  shell: make build
  env:
    CC: gcc-11
    CFLAGS: "-O2"
```

**Working Directory**

Execute commands in specific directories:
```yaml
- name: Build project
  shell: npm run build
  cwd: /opt/myproject
```

**Custom Change Detection**

Override when a step counts as "changed":
```yaml
- name: Git pull
  shell: git pull
  changed_when: "'Already up to date' not in result.stdout"
```

**Custom Failure Detection**

Override when a step is considered failed:
```yaml
- name: Grep with acceptable exit codes
  shell: grep "pattern" file.txt
  failed_when: "result.rc >= 2"  # 0=found, 1=not found, 2+=error
```

**Different Users**

Run commands as specific users:
```yaml
- name: Run as postgres
  shell: psql -c "SELECT version()"
  become: true
  become_user: postgres
```

#### Real-World Example

Robust service deployment with retries and validation:
```yaml
- name: Download release
  shell: curl -O https://releases.example.com/app-{{version}}.tar.gz
  timeout: 10m
  retries: 3
  retry_delay: 30s

- name: Install application
  shell: pip install -r requirements.txt
  cwd: /opt/myapp
  become: true
  become_user: appuser
  timeout: 5m
  env:
    PIP_INDEX_URL: "{{pip_mirror}}"

- name: Run migrations
  shell: ./manage.py migrate
  cwd: /opt/myapp
  become_user: appuser
  timeout: 10m
  register: migrate_result
  changed_when: "'No migrations to apply' not in result.stdout"

- name: Wait for service
  shell: curl -sf http://localhost:8080/health
  retries: 30
  retry_delay: 2s
  failed_when: "result.rc != 0"
```

See [complete example](11-execution-control.md) for detailed deployment workflow.

---

### 12 - Unarchive / Extract Archives

Learn how to extract archive files with automatic format detection and security protections.

#### What You'll Learn

- Extracting tar, tar.gz, tgz, and zip archives
- Using `strip_components` to remove leading directories
- Idempotency with `creates` parameter
- Security protections against path traversal

#### Quick Start

```bash
cd examples/12-unarchive
mooncake run --config config.yml
```

 **Note:** This example demonstrates archive extraction patterns. You'll need to provide your own test archives or download sample ones.

#### What It Does

1. Extracts various archive formats
2. Demonstrates path stripping with `strip_components`
3. Shows idempotent extraction with `creates`
4. Extracts to system directories with sudo

#### Key Concepts

**Basic Extraction**

```yaml
- name: Extract Node.js
  unarchive:
    src: /tmp/node-v20.tar.gz
    dest: /opt/node
    mode: "0755"
```

**Supported Formats**

Auto-detected from extension:
- `.tar` - Uncompressed tar archives
- `.tar.gz`, `.tgz` - Gzip compressed tar
- `.zip` - ZIP archives

**Strip Components**

Remove leading directories (like tar's `--strip-components`):

```yaml
# Archive: project-1.0/src/main.go
- name: Extract without top-level directory
  unarchive:
    src: /tmp/project.tar.gz
    dest: /opt/project
    strip_components: 1
    # Result: /opt/project/src/main.go (without project-1.0/)
```

**Idempotency**

Skip extraction if marker file exists:

```yaml
- name: Extract application
  unarchive:
    src: /tmp/app.tar.gz
    dest: /opt/app
    creates: /opt/app/bin/app
```

Run again - extraction skipped because marker exists.

**Security Features**

Automatically blocks:
- Path traversal (`../` sequences)
- Absolute paths (`/etc/passwd`)
- Symlink escapes outside destination

**Extract Multiple Archives**

```yaml
- vars:
    archives:
      - {name: app, file: app.tar.gz, strip: 1}
      - {name: data, file: data.zip, strip: 0}

- name: Extract {{item.name}}
  unarchive:
    src: /tmp/{{item.file}}
    dest: /opt/{{item.name}}
    strip_components: "{{item.strip}}"
  with_items: "{{archives}}"
```

**Real-World Use Cases**

Software Installation:
```yaml
- name: Install Go
  unarchive:
    src: /tmp/go1.21.linux-amd64.tar.gz
    dest: /usr/local
    creates: /usr/local/go/bin/go
  become: true
```

Application Deployment:
```yaml
- name: Deploy release
  unarchive:
    src: /tmp/myapp-{{version}}.tar.gz
    dest: /opt/myapp
    strip_components: 1
  become: true
```

Backup Restoration:
```yaml
- name: Restore data
  unarchive:
    src: /backups/data-{{date}}.tar.gz
    dest: /var/lib/app
    creates: /var/lib/app/.restored
```

See [complete example](12-unarchive.md) for detailed archive handling patterns including Node.js installation workflow.

---

## Real-World Example

### Dotfiles Manager

A complete example showing how to manage and deploy dotfiles using Mooncake.

#### Features Demonstrated

- Multi-file organization
- Template rendering for dynamic configs
- File tree iteration
- Conditional deployment by OS
- Variable management
- Backup functionality
- Tag-based workflows

#### Quick Start

```bash
cd examples/real-world/dotfiles-manager

# Deploy all dotfiles
mooncake run --config setup.yml

# Deploy only shell configs
mooncake run --config setup.yml --tags shell

# Preview what would be deployed
mooncake run --config setup.yml --dry-run
```

#### Directory Structure

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

#### What It Does

1. Backs up existing dotfiles
2. Creates necessary directories
3. Deploys static dotfiles
4. Renders dynamic configs from templates
5. Sets appropriate permissions
6. OS-specific configuration

#### Configuration

Edit `vars.yml` to customize:
```yaml
user_email: your@email.com
user_name: Your Name
editor: nvim
shell: zsh
color_scheme: gruvbox
```

#### Usage

**Full Deployment:**
```bash
mooncake run --config setup.yml
```

**Selective Deployment:**
```bash
# Only shell configs
mooncake run --config setup.yml --tags shell

# Only vim/neovim
mooncake run --config setup.yml --tags vim

# Only git config
mooncake run --config setup.yml --tags git
```

**Backup Only:**
```bash
mooncake run --config setup.yml --tags backup
```

#### Extending

**Adding New Dotfiles:**

1. Add file to `dotfiles/` directory
2. Add deployment step in `setup.yml`:
```yaml
- name: Deploy new config
  shell: cp {{ item.src }} ~/{{ item.name }}
  with_filetree: ./dotfiles/new-app
  tags:
    - new-app
```

**Adding Templates:**

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

#### Real-World Tips

1. **Version control** - Keep this in git
2. **Test first** - Use `--dry-run` before applying
3. **Incremental** - Add configs gradually
4. **Backup** - The example includes backup steps
5. **Document** - Add comments for custom settings

---

## Summary

You've learned all core Mooncake features through these examples:

| Feature | Examples |
|---------|----------|
| **Shell Commands** | 01, 04, 06, 08, 11 |
| **File Operations** | 03, 06, Real-World |
| **Copy** | Real-World |
| **Unarchive** | 12 |
| **Templates** | 05, Real-World |
| **Variables** | 02, 05, 06, 10 |
| **Conditionals** | 04, 08, 09 |
| **Loops** | 06, 12, Real-World |
| **Register** | 07, 11 |
| **Tags** | 08, Real-World |
| **Sudo** | 09, 11, 12 |
| **Multi-file** | 10, Real-World |
| **Execution Control** | 11 |
| **Timeouts & Retries** | 11 |
| **Environment Variables** | 11 |

## Next Steps

- [:material-book-open: Read the Guide](../index.md) - Comprehensive documentation
- [:material-file-document: Reference](../guide/config/actions.md) - Detailed config reference
- [:fontawesome-brands-github: GitHub](https://github.com/alehatsman/mooncake) - View source code
- [:material-account-group: Contributing](../development/contributing.md) - Help improve Mooncake
