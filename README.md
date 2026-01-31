# Mooncake [![build](https://github.com/alehatsman/mooncake/actions/workflows/build_test.yml/badge.svg?branch=master)](https://github.com/alehatsman/mooncake/actions/workflows/build_test.yml)

Space fighters provisioning tool, **Chookity!**

## Installation

```bash
go install github.com/alehatsman/mooncake@latest
```

## Usage

```bash
mooncake run config.yml
mooncake run config.yml --vars vars.yml
mooncake run config.yml --sudo-pass <password>
```

## Features

- Execute shell commands with templating
- Create files and directories with configurable permissions
- Render templates using pongo2
- Conditional execution with expressions
- Include other configuration files
- Load variables from external files
- File tree iteration
- Relative path resolution
- Global system facts (os, arch)

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

### Global Variables

Available in all steps:

- `os`: linux | darwin | windows
- `arch`: amd64 | arm64 | etc

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

### Tags

Filter steps by tags:

```yaml
- name: Install development tools
  shell: brew install neovim ripgrep
  tags:
    - dev
    - tools

- name: Setup production
  shell: setup-production.sh
  tags:
    - prod
```

Run with tags:
```bash
mooncake run config.yml --tags dev
mooncake run config.yml --tags prod,tools
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
mooncake run config.yml --sudo-pass <password>
```

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
