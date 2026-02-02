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
