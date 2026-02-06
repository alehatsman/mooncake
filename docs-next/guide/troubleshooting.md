# Troubleshooting Guide

Common issues and their solutions when working with Mooncake.

---

## Installation Issues

### "command not found: mooncake"

**Problem**: After installing with `go install`, the `mooncake` command is not found.

**Solution**: Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is in your PATH:

```bash
# Add to ~/.bashrc, ~/.zshrc, or equivalent
export PATH="$PATH:$HOME/go/bin"

# Or find your GOPATH
go env GOPATH

# Verify
which mooncake
```

### Permission denied when installing

**Problem**: `go install` fails with permission errors.

**Solution**: Don't use `sudo` with `go install`. Install to your user directory:

```bash
# Correct way
go install github.com/alehatsman/mooncake@latest

# Wrong way (don't do this)
sudo go install github.com/alehatsman/mooncake@latest
```

---

## Configuration Errors

### "invalid configuration: unknown field"

**Problem**: YAML contains a typo or invalid field name.

```
Error: invalid configuration: unknown field 'shel' in step 1
```

**Solution**: Check spelling of action names and fields. Common typos:

- `shel` → `shell`
- `comand` → `command`
- `templete` → `template`

Use schema validation to catch these early:
```bash
mooncake plan --config config.yml  # Validates without running
```

### "yaml: unmarshal errors"

**Problem**: Invalid YAML syntax.

**Solution**: Check YAML formatting:

- Proper indentation (use spaces, not tabs)
- Quoted strings containing special characters
- Proper list syntax

```yaml
# Wrong
- name: Test
shell: echo "hello"

# Right
- name: Test
  shell: echo "hello"

# Wrong - mixed indentation
- name: Test
  shell: |
    echo "line 1"
      echo "line 2"  # Too much indent

# Right
- name: Test
  shell: |
    echo "line 1"
    echo "line 2"
```

Use a YAML validator:
```bash
# Install yamllint
pip install yamllint

# Validate
yamllint config.yml
```

### "failed to expand template"

**Problem**: Template variable is undefined or template syntax is invalid.

```
Error: failed to expand template: variable 'my_var' is not defined
```

**Solution**:

1. Define the variable before using it:
```yaml
- vars:
    my_var: value

- name: Use variable
  shell: echo "{{my_var}}"
```

2. Use conditional to check if variable exists:
```yaml
- name: Optional variable
  shell: echo "{{my_var}}"
  when: my_var is defined
```

3. Check template syntax:
```yaml
# Wrong
- shell: "{{ variable }"  # Missing closing brace

# Right
- shell: "{{ variable }}"
```

---

## Execution Errors

### "permission denied"

**Problem**: Trying to access a file or directory without sufficient permissions.

**Solution**: Use `become: true` for operations requiring root:

```yaml
- name: Install system package
  shell: apt install neovim
  become: true
```

Then run with sudo password:
```bash
mooncake run --config config.yml -K
```

### "sudo: no password provided"

**Problem**: Step requires sudo but no password method was specified.

**Solution**: Provide sudo password using one of these methods:

```bash
# Interactive prompt
mooncake run --config config.yml --ask-become-pass
mooncake run --config config.yml -K  # shorthand

# Password file
echo "your_password" > ~/.mooncake/sudo_pass
chmod 600 ~/.mooncake/sudo_pass
mooncake run --config config.yml --sudo-pass-file ~/.mooncake/sudo_pass

# SSH askpass (for GUI environments)
export SUDO_ASKPASS=/usr/bin/ssh-askpass
mooncake run --config config.yml
```

### "command not found"

**Problem**: Shell command doesn't exist on the system.

**Solution**:

1. Check if command is installed:
```yaml
- name: Check if docker exists
  shell: which docker
  register: docker_check
  ignore_errors: true

- name: Install docker
  preset: docker
  when: docker_check.rc != 0
  become: true
```

2. Use OS-specific commands:
```yaml
- name: Install on macOS
  shell: brew install neovim
  when: os == "darwin"

- name: Install on Linux
  shell: apt install neovim
  when: os == "linux" && package_manager == "apt"
```

### "timeout: command took too long"

**Problem**: Command exceeds default 2-minute timeout.

**Solution**: Increase timeout:

```yaml
- name: Long-running task
  shell: ./build-script.sh
  timeout: 30m  # 30 minutes
```

---

## File Operation Errors

### "file already exists"

**Problem**: Trying to create file/directory that already exists.

**Solution**: This is usually fine - Mooncake operations are idempotent. If you see an error, check the `state` parameter:

```yaml
# Creates or ensures file exists (idempotent)
- name: Ensure file exists
  file:
    path: /tmp/myfile
    state: file

# Creates or ensures directory exists (idempotent)
- name: Ensure directory exists
  file:
    path: /tmp/mydir
    state: directory
```

### "no such file or directory"

**Problem**: Trying to operate on a file that doesn't exist, or parent directory doesn't exist.

**Solution**:

1. Create parent directories first:
```yaml
- name: Create parent directory
  file:
    path: ~/.config/myapp
    state: directory

- name: Create config file
  file:
    path: ~/.config/myapp/config.yml
    state: file
    content: "key: value"
```

2. Use `creates` to make operation conditional:
```yaml
- name: Extract only if doesn't exist
  unarchive:
    src: /tmp/archive.tar.gz
    dest: /opt/app
    creates: /opt/app/bin/app
```

### "checksum mismatch"

**Problem**: Downloaded file checksum doesn't match expected value.

**Solution**:

1. Verify the checksum value is correct
2. Re-download the file (might be corrupted)
3. Check if upstream changed the file

```yaml
- name: Download with correct checksum
  download:
    url: https://example.com/file.tar.gz
    dest: /tmp/file.tar.gz
    checksum: sha256:abc123def456...  # Verify this is correct
    retries: 3  # Retry on failure
```

Get correct checksum:
```bash
# Calculate SHA-256 checksum
sha256sum file.tar.gz
shasum -a 256 file.tar.gz  # macOS
```

---

## Variable & Template Issues

### "undefined variable"

**Problem**: Using a variable that hasn't been defined.

**Solution**:

1. Define variable before use:
```yaml
- vars:
    app_name: MyApp

- shell: echo "{{app_name}}"
```

2. Use system facts (automatically available):
```yaml
- shell: echo "Running on {{os}}/{{arch}}"
# No need to define os and arch
```

3. Check variable scope - variables defined in one config file aren't available in included files unless passed explicitly.

### "template rendering failed"

**Problem**: Invalid Jinja2 template syntax.

**Solution**: Check template syntax:

```yaml
# Wrong - spaces in variable name
{{ my var }}

# Right
{{ my_var }}

# Wrong - missing endif
{% if condition %}
  something

# Right
{% if condition %}
  something
{% endif %}

# Wrong - invalid filter
{{ value | badfilter }}

# Right - use valid filters
{{ path | expanduser }}
{{ text | upper }}
{{ file | basename }}
```

---

## Platform-Specific Issues

### macOS: "operation not permitted"

**Problem**: macOS security restrictions prevent file operations.

**Solution**:

1. Grant Full Disk Access to Terminal:
   - System Settings → Privacy & Security → Full Disk Access
   - Add Terminal.app or iTerm.app

2. Use `become: true` for system modifications

### Linux: "systemd service not found"

**Problem**: Trying to manage a service that doesn't exist.

**Solution**:

1. Verify service name:
```bash
systemctl list-units --type=service | grep myservice
```

2. Create service first, then manage it:
```yaml
- name: Create systemd unit
  service:
    name: myapp
    unit:
      dest: /etc/systemd/system/myapp.service
      content: |
        [Unit]
        Description=My App

        [Service]
        ExecStart=/usr/local/bin/myapp

        [Install]
        WantedBy=multi-user.target
    daemon_reload: true
  become: true

- name: Start service
  service:
    name: myapp
    state: started
    enabled: true
  become: true
```

### Windows: "command not supported"

**Problem**: Some actions work differently on Windows.

**Solution**: Use platform-specific conditionals:

```yaml
- name: Unix command
  shell: ls -la
  when: os != "windows"

- name: Windows command
  shell: dir
  when: os == "windows"
```

---

## Preset Issues

### "preset not found"

**Problem**: Trying to use a preset that doesn't exist.

**Solution**:

1. List available presets:
```bash
mooncake presets list
```

2. Check preset name spelling:
```yaml
# Wrong
- preset: postgress

# Right
- preset: postgres
```

3. Verify preset is installed (if using custom presets)

### "invalid preset parameters"

**Problem**: Preset parameters don't match schema.

**Solution**: Check preset documentation:

```bash
# Show preset details
mooncake presets status docker
```

Use correct parameter names and types:
```yaml
# Wrong - state is string, not boolean
- preset: docker
  with:
    state: true

# Right
- preset: docker
  with:
    state: present
```

### "preset failed during execution"

**Problem**: Preset step failed.

**Solution**:

1. Run with debug logging:
```bash
mooncake run --config config.yml --log-level debug
```

2. Check preset source code:
```bash
# View preset definition
cat ~/.mooncake/presets/docker/preset.yml
```

3. Try manual installation to isolate issue

---

## Performance Issues

### "execution is very slow"

**Problem**: Configuration takes a long time to run.

**Solution**:

1. Use dry-run to identify slow steps:
```bash
mooncake run --config config.yml --dry-run
```

2. Reduce retries and timeouts where not needed:
```yaml
# Instead of this
- shell: echo "hello"
  timeout: 10m
  retries: 5

# Use this
- shell: echo "hello"
  timeout: 10s
```

3. Use tags to run only necessary steps:
```bash
mooncake run --config config.yml --tags quick
```

4. Check for unnecessary loops:
```yaml
# Inefficient - runs apt update 10 times
- shell: apt update && apt install {{item}}
  with_items: [vim, git, curl, ...]
  become: true

# Better - update once, then install
- shell: apt update
  become: true

- shell: apt install -y {{item}}
  with_items: [vim, git, curl, ...]
  become: true
```

---

## Debugging Techniques

### Enable debug logging

```bash
mooncake run --config config.yml --log-level debug
```

### Use dry-run mode

```bash
# See what would happen without making changes
mooncake run --config config.yml --dry-run
```

### Generate execution plan

```bash
# See the execution plan
mooncake plan --config config.yml

# Export as JSON for analysis
mooncake plan --config config.yml --format json --output plan.json
```

### Test individual steps

Use tags to isolate problematic steps:

```yaml
- name: Problematic step
  shell: complex-command.sh
  tags: [debug]
```

```bash
mooncake run --config config.yml --tags debug --dry-run
```

### Check system facts

```bash
# View all detected system information
mooncake facts

# Export as JSON
mooncake facts --format json > facts.json
```

### Register and inspect results

```yaml
- name: Run command
  shell: my-command.sh
  register: result

- name: Show result
  shell: echo "RC={{result.rc}} STDOUT={{result.stdout}}"
```

### Use ignore_errors

```yaml
- name: Optional step
  shell: might-fail.sh
  register: result
  ignore_errors: true

- name: Check if failed
  shell: echo "Previous step failed"
  when: result.rc != 0
```

---

## Getting Help

### Check documentation

- [Quick Reference](../quick-reference.md)
- [Actions Guide](config/actions.md)
- [Complete Reference](config/reference.md)
- [FAQ](../faq.md)

### Validate configuration

```bash
# Validate without executing
mooncake plan --config config.yml
```

### Report bugs

If you've found a bug:

1. Create minimal reproduction:
```yaml
# Simplest config that reproduces the issue
- name: Bug reproduction
  shell: echo "This fails"
```

2. Include system information:
```bash
mooncake facts > system-info.txt
mooncake --version
```

3. Report at [GitHub Issues](https://github.com/alehatsman/mooncake/issues)

---

## Common Patterns

### Safe file operations

```yaml
# Always create parent directories first
- name: Create config directory
  file:
    path: ~/.config/myapp
    state: directory

# Then create files
- name: Create config
  file:
    path: ~/.config/myapp/config.yml
    state: file
    content: "..."
```

### Idempotent commands

```yaml
# Use creates/removes for idempotency
- name: Extract tarball
  shell: tar xzf /tmp/app.tar.gz -C /opt
  args:
    creates: /opt/app/bin/app  # Only if doesn't exist

- name: Clean up
  shell: rm -rf /tmp/cache
  args:
    removes: /tmp/cache  # Only if exists
```

### Error handling

```yaml
- name: Try to download
  download:
    url: https://example.com/file.tar.gz
    dest: /tmp/file.tar.gz
  register: download_result
  ignore_errors: true

- name: Use fallback if download failed
  download:
    url: https://mirror.example.com/file.tar.gz
    dest: /tmp/file.tar.gz
  when: download_result.rc != 0
```

---

## See Also

- [Quick Reference](../quick-reference.md) - Common commands and patterns
- [FAQ](../faq.md) - Frequently asked questions
- [Examples](../../examples/) - Working examples
- [GitHub Issues](https://github.com/alehatsman/mooncake/issues) - Report bugs
