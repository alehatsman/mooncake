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
