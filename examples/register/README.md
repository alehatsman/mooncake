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

After registering a result, the envelope shape (proposal-01) is the
same for every action:

**Top-level envelope (always present):**
- `register_name.changed` - Boolean, true if the action mutated state
- `register_name.failed` - Boolean, true if the action failed
- `register_name.operation` - One of `create` / `update` / `delete` /
  `noop` / `query` / `reverted` — what this step did
- `register_name.target` - The primary thing acted on (path, package,
  URL, etc.)
- `register_name.error` - Diagnostic string when `failed` is true;
  empty string when `failed` is false (proposal-06)
- `register_name.rc` - Return/exit code (shell-family only; 0 = success)
- `register_name.stdout` / `register_name.stderr` - Captured output
  (shell-family only)
- `register_name.skipped` - Boolean, true if the step did not run
- `register_name.cancelled` - Boolean, true if the step was interrupted
- `register_name.duration_ms` - Wall-clock time in milliseconds
- `register_name.status` - One-word summary (`ok`, `changed`, `failed`,
  `skipped`, `cancelled`, `reverted`)

**Action-specific payload (nested under `.data`):**
- `register_name.data.<field>` - Per-action typed fields (e.g.
  `register_name.data.found` for `observe.process`,
  `register_name.data.value.cores` for `observe.cpu`, etc.)

Pre-proposal-01 (legacy) flattened the action-specific fields into the
top level — `register_name.found` would have worked. The envelope
moves them under `.data` so envelope keys can't be shadowed by
handler-set fields.

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
