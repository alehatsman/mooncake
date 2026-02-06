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

After registering a result, you can access:

**For shell commands:**
- `register_name.stdout` - Standard output
- `register_name.stderr` - Standard error
- `register_name.rc` - Return/exit code (0 = success)
- `register_name.failed` - Boolean, true if rc != 0
- `register_name.changed` - Boolean, always true for shell

**For file operations:**
- `register_name.rc` - 0 for success, 1 for failure
- `register_name.failed` - Boolean, true if operation failed
- `register_name.changed` - Boolean, true if file created/modified

**For template operations:**
- `register_name.rc` - 0 for success, 1 for failure
- `register_name.failed` - Boolean, true if rendering failed
- `register_name.changed` - Boolean, true if output file changed

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
