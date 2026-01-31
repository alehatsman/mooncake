# Mooncake

[![CI](https://github.com/alehatsman/mooncake/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/alehatsman/mooncake/actions/workflows/ci.yml)
[![Security](https://github.com/alehatsman/mooncake/actions/workflows/security.yml/badge.svg?branch=master)](https://github.com/alehatsman/mooncake/actions/workflows/security.yml)
[![codecov](https://codecov.io/gh/alehatsman/mooncake/branch/master/graph/badge.svg)](https://codecov.io/gh/alehatsman/mooncake)

Space fighters provisioning tool, **Chookity!**

## Installation

```bash
go install github.com/alehatsman/mooncake@latest
```

## Usage

Mooncake features an animated TUI (Text User Interface) by default that shows real-time progress with an animated character. Use `--raw` to disable the animation.

```bash
# Run configuration
mooncake run --config config.yml

# With variables file
mooncake run --config config.yml --vars vars.yml

# With sudo password
mooncake run --config config.yml --sudo-pass <password>

# Filter by tags
mooncake run --config config.yml --tags dev
mooncake run --config config.yml --tags dev,prod,test

# Preview what would be executed (dry-run)
mooncake run --config config.yml --dry-run

# Disable animated UI (use raw console output)
mooncake run --config config.yml --raw

# With debug logging
mooncake run --config config.yml --log-level debug
```

### CLI Flags

- `--config, -c`: Path to configuration file (required)
- `--vars, -v`: Path to variables file
- `--log-level, -l`: Log level - debug, info, or error (default: info)
- `--sudo-pass, -s`: Sudo password for steps with `become: true`
- `--tags, -t`: Filter steps by tags (comma-separated)
- `--dry-run`: Preview what would be executed without making any changes (validates and shows preview)
- `--raw, -r`: Disable animated TUI and use raw console output

## Features

- Execute shell commands with templating
- Create files and directories with configurable permissions
- Render templates using pongo2
- Conditional execution with expressions
- Include other configuration files
- Load variables from external files
- File tree iteration with with_filetree
- List iteration with with_items
- Relative path resolution
- Comprehensive system facts (OS, distribution, package manager, hardware, network)
- Animated TUI with live progress tracking
- Dry-run mode for safe preview and validation

## Dry-Run Mode

Preview what would be executed and validate your configuration without making any changes to your system:

```bash
mooncake run --config config.yml --dry-run
```

**What it does:**
- Validates YAML syntax and step structure
- Checks that required files exist (template sources, included configs, variable files)
- Verifies paths can be expanded and variables resolved
- Shows commands that would be executed
- Shows files and directories that would be created
- Shows templates that would be rendered
- Shows variables that would be set
- Processes includes recursively to show all steps

**Example output:**
```
[1/3] Create directory
  [DRY-RUN] Would create directory: /home/user/.config (mode: 0755)
[2/3] Render config
  [DRY-RUN] Would template: ./template.j2 -> /home/user/.config/app.conf (mode: 0644)
[3/3] Run setup
  [DRY-RUN] Would execute: apt install neovim
  [DRY-RUN] With sudo privileges
```

**Use cases:**
- Test and validate configurations before applying them
- Preview changes in production environments
- Debug complex configurations with conditionals and includes
- Verify variable substitution and template rendering
- Check that all required files exist

## File Structure

### YAML Format

Configuration files are YAML arrays of steps. Each step must have exactly one action:

```yaml
- name: First step
  shell: echo "hello"

- name: Second step
  file:
    path: /tmp/test
    state: directory
```

### Path Resolution

Mooncake uses Node.js-like relative path resolution. All relative paths are resolved relative to the **current configuration file**, not the working directory.

```
project/
├── main.yml
├── configs/
│   ├── neovim.yml
│   └── templates/
│       └── init.lua.j2
└── dotfiles/
    └── .bashrc
```

**main.yml:**
```yaml
- name: Include neovim config
  include: ./configs/neovim.yml  # Relative to main.yml
```

**configs/neovim.yml:**
```yaml
- name: Render init.lua
  template:
    src: ./templates/init.lua.j2  # Relative to neovim.yml
    dest: ~/.config/nvim/init.lua
```

### Organizing Configurations

You can organize your configuration in multiple ways:

**Flat structure:**
```
provisioning/
├── main.yml
├── linux.yml
├── macos.yml
└── common.yml
```

**Nested structure:**
```
provisioning/
├── main.yml
├── os/
│   ├── linux.yml
│   └── macos.yml
├── apps/
│   ├── neovim.yml
│   ├── tmux.yml
│   └── zsh.yml
└── templates/
    ├── init.lua.j2
    ├── tmux.conf.j2
    └── zshrc.j2
```

**main.yml:**
```yaml
- vars:
    config_dir: ~/.config

- name: Load OS-specific configuration
  include: ./os/linux.yml
  when: os == "linux"

- name: Load OS-specific configuration
  include: ./os/macos.yml
  when: os == "darwin"

- name: Setup neovim
  include: ./apps/neovim.yml

- name: Setup tmux
  include: ./apps/tmux.yml
```

### Path Types

**Relative paths** (start with `./` or `../`):
```yaml
include: ./configs/app.yml
src: ../templates/config.j2
```

**Absolute paths**:
```yaml
dest: /etc/nginx/nginx.conf
path: /var/log/app
```

**Home directory paths** (use `~` or template filter):
```yaml
dest: ~/.config/nvim/init.lua
dest: "{{ '~/.config' | expanduser }}/nvim/init.lua"
```

## Configuration

### Variables

Define and use variables throughout your configuration:

```yaml
- vars:
    config_dir: ~/.config
    nvim_dir: "{{config_dir}}/nvim"

- name: Create nvim directory
  file:
    path: "{{nvim_dir}}"
    state: directory
```

### Include Variables

Load variables from external YAML files:

```yaml
- name: Load environment variables
  include_vars: ./env.yml
```

### Global Variables (System Facts)

Mooncake automatically collects system facts and makes them available in all steps:

**Basic Facts:**
- `os`: Operating system (linux | darwin | windows)
- `arch`: Architecture (amd64 | arm64 | etc)
- `hostname`: System hostname
- `user_home`: Current user's home directory
- `cpu_cores`: Number of CPU cores
- `memory_total_mb`: Total system memory in megabytes

**Distribution (Linux/macOS):**
- `distribution`: Distribution name (ubuntu | debian | centos | rhel | fedora | arch | macos)
- `distribution_version`: Full version (e.g., "22.04", "15.7.3")
- `distribution_major`: Major version number (e.g., "22", "15")

**Software:**
- `package_manager`: Detected package manager (apt | yum | dnf | brew | pacman | zypper | apk)
- `python_version`: Installed Python version (e.g., "3.11.4")

**Network:**
- `ip_addresses`: Array of all non-loopback IP addresses
- `ip_addresses_string`: Comma-separated string of IP addresses

**Example usage:**
```yaml
- name: Install package using detected package manager
  shell: "{{ package_manager }} install neovim"
  become: true
  when: os == "linux"

- name: Configure for high-memory systems
  shell: echo "Using high memory settings"
  when: memory_total_mb >= 16000

- name: Ubuntu-specific configuration
  shell: apt update
  become: true
  when: distribution == "ubuntu" and distribution_major >= "20"
```

### File

Create files or directories with optional permissions:

```yaml
- name: Create directory
  file:
    path: ~/.config/nvim
    state: directory
    mode: "0755"

- name: Create empty file
  file:
    path: ~/.config/nvim/init.lua
    state: file
    mode: "0644"

- name: Create file with content
  file:
    path: /tmp/test.txt
    state: file
    content: "Hello World"
```

### Template

Render templates using pongo2 syntax:

```yaml
- name: Render config file
  template:
    src: ./init.lua.j2
    dest: ~/.config/nvim/init.lua
    mode: "0644"
    vars:
      port: 8080
      debug: true
```

Template supports pongo2 features:

```jinja
# Variables
{{ variable_name }}

# Conditionals
{% if debug %}
debug_mode = true
{% endif %}

# Loops
{% for item in items %}
- {{ item }}
{% endfor %}

# Filters
{{ path|expanduser }}  # Expands ~ to home directory
{{ text|upper }}
```

### Shell

Execute shell commands:

```yaml
- name: Install packages
  shell: brew install neovim ripgrep

- name: Run multiple commands
  shell: |
    echo "Starting setup"
    mkdir -p ~/.local/bin
    echo "Setup complete"
```

### Include

Include other configuration files:

```yaml
- name: Include Linux configuration
  include: ./linux.yml

- name: Include with relative path
  include: ./configs/neovim.yml
```

### Conditional Execution

Use `when` to conditionally execute steps:

```yaml
- name: Install Linux packages
  shell: apt install neovim
  when: os == "linux"

- name: Install macOS packages
  shell: brew install neovim
  when: os == "darwin"

- name: Complex condition
  shell: echo "ARM Mac"
  when: os == "darwin" && arch == "arm64"
```

Supported operators:
- Comparison: `==`, `!=`, `>`, `<`, `>=`, `<=`
- Logical: `&&`, `||`, `!`
- Arithmetic: `+`, `-`, `*`, `/`, `%`

### File Tree Iteration

Iterate over files in a directory:

```yaml
- name: Copy dotfiles
  template:
    src: "{{ item.src }}"
    dest: "~/.config/{{ item.name }}"
  with_filetree: ./dotfiles
```

Each iteration provides:
- `item.src`: Source file path
- `item.name`: File name
- `item.is_dir`: Boolean indicating if item is directory

### Register - Capture Command Output

Capture output from commands and use it in subsequent steps:

```yaml
- name: Check if git is installed
  shell: which git
  register: git_check

- name: Show git location (nested access)
  shell: echo "Git is at {{ git_check.stdout }}"
  when: git_check.rc == 0

- name: Get current user
  shell: whoami
  register: current_user

- name: Create user-specific config
  file:
    path: "/tmp/{{ current_user.stdout }}_config.txt"
    state: file
    content: "Config for {{ current_user.stdout }}"
```

**Available fields:**

Use nested access (recommended):
- `{name}.stdout`: Standard output from the command
- `{name}.stderr`: Standard error from the command
- `{name}.rc`: Return code (exit status)
- `{name}.failed`: Boolean indicating if step failed
- `{name}.changed`: Boolean indicating if step made changes

Or flat access (also supported):
- `{name}_stdout`, `{name}_stderr`, `{name}_rc`, `{name}_failed`, `{name}_changed`

**Works with:**
- `shell`: Captures command output
- `file`: Detects if file was created/modified
- `template`: Detects if template output changed

**Change detection:**
- Shell commands: Always `changed=true`
- File operations: `changed=true` only if file created or content modified
- Templates: `changed=true` only if rendered output differs from existing file

**Expression features:**

Thanks to [expr-lang](https://github.com/expr-lang/expr), you can use powerful expressions:

```yaml
# Complex conditions
- name: Check multiple conditions
  shell: echo "Valid"
  when: user.name == "admin" and git_check.rc == 0

# Built-in functions
- name: Check list length
  shell: echo "Many items"
  when: len(packages) > 5

# String operations
- name: Check string contains
  shell: echo "Found"
  when: '"docker" in git_check.stdout'
```

### List Iteration

Iterate over a list of items using `with_items`:

```yaml
- vars:
    packages:
      - neovim
      - ripgrep
      - tmux
      - fzf

- name: Install packages
  shell: brew install {{ item }}
  with_items: "{{ packages }}"
```

You can also iterate over inline lists:

```yaml
- vars:
    users:
      - alice
      - bob
      - charlie

- name: Create user directories
  file:
    path: "/home/{{ item }}"
    state: directory
    mode: "0755"
  with_items: "{{ users }}"
```

Each iteration provides the current item in the `item` variable

### Tags

Filter execution by tags. When tags filter is specified, only steps with matching tags are executed:

```yaml
- name: Step without tags
  shell: echo "Always runs when no filter specified"

- name: Install development tools
  shell: brew install neovim ripgrep
  tags:
    - dev
    - tools

- name: Setup production
  shell: setup-production.sh
  tags:
    - prod

- name: Deploy to staging
  shell: deploy-staging.sh
  tags:
    - deploy
    - staging
```

**Behavior:**
- **No tags filter**: All steps execute (including untagged steps)
- **With tags filter**: Only steps with matching tags execute; untagged steps are skipped
- **Multiple tags**: Step executes if it has ANY of the specified tags

Run with tags:
```bash
# Run only dev-tagged steps
mooncake run --config config.yml --tags dev

# Run dev OR prod tagged steps
mooncake run --config config.yml --tags dev,prod

# Run steps tagged with deploy OR staging
mooncake run --config config.yml --tags deploy,staging
```

### Sudo/Become

Execute commands with sudo:

```yaml
- name: Install system package
  shell: apt install neovim
  become: true
```

Provide sudo password:
```bash
mooncake run --config config.yml --sudo-pass <password>
```

Note: Only steps with `become: true` will use sudo.

### File Permissions

Specify file permissions in octal format:

```yaml
- name: Create executable script
  file:
    path: ~/.local/bin/script.sh
    state: file
    content: "#!/bin/bash\necho hello"
    mode: "0755"

- name: Create private config
  template:
    src: ./secret.yml.j2
    dest: ~/.config/secret.yml
    mode: "0600"
```

## Examples

See the [examples/](examples/) directory for complete working examples:
- **Basic examples**: Hello world, files, conditionals, tags
- **Advanced examples**: Multi-file configurations, includes, variables

## Example Configuration

```yaml
- vars:
    config_dir: ~/.config
    nvim_dir: "{{config_dir}}/nvim"
    nvim_config: "{{nvim_dir}}/init.lua"

- name: Ensure neovim config directory exists
  file:
    path: "{{nvim_dir}}"
    state: directory
    mode: "0755"

- name: Render neovim configuration
  template:
    src: ./init.lua.j2
    dest: "{{nvim_config}}"
    mode: "0644"

- name: Install neovim on macOS
  shell: brew install neovim
  when: os == "darwin"

- name: Install neovim on Linux
  shell: apt install neovim
  become: true
  when: os == "linux"

- name: Copy plugin configs
  template:
    src: "{{item.src}}"
    dest: "{{nvim_dir}}/lua/{{item.name}}"
  with_filetree: ./nvim/lua
  when: item.is_dir == false
```
