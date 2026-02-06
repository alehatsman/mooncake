# Idempotency Patterns

Mooncake provides several features to help you write idempotent playbooks that can be safely run multiple times without unintended side effects.

## Table of Contents

- [Using `creates`](#using-creates)
- [Using `unless`](#using-unless)
- [Using `changed_when`](#using-changed_when)
- [Combining Strategies](#combining-strategies)
- [Result Timing](#result-timing)

## Using `creates`

The `creates` field skips a step if the specified file path already exists. This is useful for one-time installation or setup tasks.

### One-time installation

```yaml
- name: Download installer
  shell: wget https://example.com/installer.sh -O /tmp/installer.sh
  creates: /tmp/installer.sh

- name: Run installer
  shell: bash /tmp/installer.sh
  creates: /opt/myapp/bin/myapp
```

On the first run, both steps execute. On subsequent runs, both steps are skipped because the files exist.

### Compilation steps

```yaml
- name: Compile binary
  shell: go build -o myapp
  creates: ./myapp
```

The compilation only runs if the binary doesn't exist yet.

### Template variables

The `creates` path is rendered through the template engine, so you can use variables:

```yaml
- name: Set build directory
  vars:
    build_dir: /opt/myproject

- name: Compile project
  shell: make build
  creates: "{{ build_dir }}/myapp"
```

## Using `unless`

The `unless` field skips a step if the given command succeeds (returns exit code 0). This provides more flexibility than `creates` for conditional execution.

### Database initialization

```yaml
- name: Initialize database
  shell: psql -f schema.sql mydb
  unless: "psql -c '\\dt' mydb | grep users"
```

The initialization only runs if the `users` table doesn't exist.

### Service configuration

```yaml
- name: Configure service
  shell: systemctl enable myservice
  unless: "systemctl is-enabled myservice"
```

The service is only enabled if it's not already enabled.

### Version checks

```yaml
- name: Install package
  shell: apt-get install -y mypackage
  unless: "dpkg -s mypackage | grep -q 'Version: 2.0'"
```

The package is only installed if version 2.0 is not currently installed.

### Template variables

Like `creates`, the `unless` command is rendered through the template engine:

```yaml
- name: Set database name
  vars:
    db_name: production

- name: Create database
  shell: createdb {{ db_name }}
  unless: "psql -l | grep {{ db_name }}"
```

**Note**: The `unless` command is executed silently (no output logged) to avoid cluttering the logs.

## Using `changed_when`

The `changed_when` field allows you to override whether a shell command is marked as "changed". By default, all shell commands are marked as changed.

### Commands with predictable output

```yaml
- name: Install package
  shell: apt-get install -y package
  register: install_result
  changed_when: "'is already the newest version' not in result.stdout"
```

The step is only marked as changed if the package was actually installed or upgraded.

### Always-safe commands

```yaml
- name: Set sysctl value (idempotent)
  shell: sysctl -w net.ipv4.ip_forward=1
  changed_when: false
```

Setting `changed_when: false` indicates this command is idempotent and doesn't make changes.

### Conditional based on exit code

```yaml
- name: Check and update config
  shell: diff config.new /etc/config && cp config.new /etc/config
  register: config_result
  changed_when: result.rc == 0
  failed_when: false
```

Only mark as changed if the files were different (diff returns 0) and the copy succeeded.

## Combining Strategies

You can combine `creates`, `unless`, `when`, and `changed_when` for sophisticated idempotency control.

### Smart package installation

```yaml
- name: Install package
  shell: apt-get install -y mypackage
  creates: /usr/bin/mypackage
  register: pkg_install
  changed_when: "result.rc == 0 and 'already installed' not in result.stdout"
  failed_when: "result.rc != 0 and 'Unable to locate package' not in result.stderr"
```

This step:

- Skips if `/usr/bin/mypackage` already exists
- Only marks as changed if the package was actually installed
- Only fails if there's a real error (not just "package not found")

### Conditional with multiple checks

```yaml
- name: Install development tools
  shell: apt-get install -y build-essential
  when: ansible_os_family == "Debian"
  unless: "dpkg -l build-essential | grep '^ii'"
  creates: /usr/bin/gcc
```

This step only runs if:

- The OS family is Debian (via `when`)
- The package is not already installed (via `unless`)
- The compiler doesn't exist (via `creates`)

**Evaluation order**: `when` → `creates` → `unless` → execute

### Database setup with safeguards

```yaml
- name: Create database user
  shell: |
    psql -c "CREATE USER myapp WITH PASSWORD '{{ db_password }}';"
  unless: "psql -c '\\du' | grep myapp"
  register: user_created
  changed_when: result.rc == 0

- name: Grant privileges
  shell: |
    psql -c "GRANT ALL PRIVILEGES ON DATABASE mydb TO myapp;"
  when: user_created.changed
  unless: "psql -c '\\l' mydb | grep myapp | grep -q PRIVILEGES"
```

The second step only runs if the user was just created or if privileges aren't already granted.

## Result Timing

All step results now include timing information that can be accessed in registered results:

```yaml
- name: Run expensive operation
  shell: make build
  register: build_result

- name: Show build duration
  shell: echo "Build took {{ build_result.duration_ms }}ms"
```

### Available timing fields

When you register a result, the following timing fields are available:

- `result.duration_ms`: Duration in milliseconds (integer)
- `result.status`: String status ("ok", "changed", "failed", "skipped")

### Example: Performance monitoring

```yaml
- name: Compile project
  shell: make -j4 build
  register: compile

- name: Run tests
  shell: make test
  register: tests

- name: Report performance
  shell: |
    echo "Compilation: {{ compile.duration_ms }}ms"
    echo "Tests: {{ tests.duration_ms }}ms"
    echo "Total: {{ compile.duration_ms + tests.duration_ms }}ms"
```

### Example: Conditional based on performance

```yaml
- name: Run optimization
  shell: optimize-database
  register: optimize_result

- name: Alert if slow
  shell: |
    echo "Warning: Optimization took {{ optimize_result.duration_ms }}ms" | \
    mail -s "Slow optimization" admin@example.com
  when: optimize_result.duration_ms > 60000
```

This sends an alert if optimization takes more than 60 seconds (60000ms).

## Best Practices

### 1. Prefer `creates` for file-based idempotency

Use `creates` when you're creating files or installing software that produces files:

```yaml
# Good
- name: Download file
  shell: wget https://example.com/file.tar.gz
  creates: file.tar.gz

# Less efficient
- name: Download file
  shell: wget https://example.com/file.tar.gz
  unless: "test -f file.tar.gz"
```

`creates` is more efficient because it uses a simple filesystem check.

### 2. Use `unless` for state checks

Use `unless` when idempotency depends on system state rather than file existence:

```yaml
- name: Enable firewall rule
  shell: ufw allow 22/tcp
  unless: "ufw status | grep '22/tcp.*ALLOW'"
```

### 3. Combine with `register` for dependent steps

```yaml
- name: Install package
  shell: apt-get install -y nginx
  creates: /usr/sbin/nginx
  register: nginx_installed

- name: Start nginx
  shell: systemctl start nginx
  when: nginx_installed.changed
```

The service is only started if nginx was just installed.

### 4. Document non-obvious idempotency

```yaml
- name: Apply database migrations (idempotent via migration tracking table)
  shell: ./migrate.sh
  changed_when: result.stdout | contains('Applied migrations')
```

Add comments when idempotency isn't immediately obvious from the command.

### 5. Test your idempotency

Always run your playbook at least twice to verify it's truly idempotent:

```bash
# First run - should make changes
mooncake run -c playbook.yml

# Second run - should skip most steps
mooncake run -c playbook.yml
```

## Common Patterns

### Package management

```yaml
- name: Install package
  shell: apt-get install -y package-name
  creates: /usr/bin/package-name

# Or with unless
- name: Install package
  shell: apt-get install -y package-name
  unless: "dpkg -l package-name | grep '^ii'"
```

### File downloads

```yaml
- name: Download archive
  shell: wget https://example.com/archive.tar.gz
  creates: archive.tar.gz

- name: Extract archive
  shell: tar xzf archive.tar.gz
  creates: archive/
```

### Service management

```yaml
- name: Enable service
  shell: systemctl enable myservice
  unless: "systemctl is-enabled myservice"

- name: Start service
  shell: systemctl start myservice
  unless: "systemctl is-active myservice"
```

### Configuration management

```yaml
- name: Update config
  template:
    src: config.j2
    dest: /etc/myapp/config.yml
  register: config_updated

- name: Restart service if config changed
  shell: systemctl restart myapp
  when: config_updated.changed
```

### Database operations

```yaml
- name: Create database
  shell: createdb mydb
  unless: "psql -l | grep mydb"

- name: Load schema
  shell: psql mydb < schema.sql
  unless: "psql mydb -c '\\dt' | grep users"
```

## Troubleshooting

### Step is not being skipped

1. **Check file paths are correct**:
   ```yaml
   # Wrong - uses relative path that might change
   creates: ./myapp

   # Better - use absolute path
   creates: /opt/myapp/myapp
   ```

2. **Check command exit codes**:
   ```bash
   # Test your unless command manually
   test -f /tmp/marker && echo "Skip" || echo "Run"
   ```

3. **Use debug mode**:
   ```bash
   mooncake run -c playbook.yml --log-level debug
   ```

### Step is being skipped incorrectly

1. **Verify the condition**:
   - For `creates`: Is the file being deleted elsewhere?
   - For `unless`: Is the command returning the wrong exit code?

2. **Check for template variables**:
   ```yaml
   # Make sure variables are set
   - name: Debug variable
     shell: echo "Checking {{ file_path }}"

   - name: Do work
     shell: create-file
     creates: "{{ file_path }}"
   ```

## See Also

- [Configuration Reference](../guide/config/reference.md) - Full field documentation
- [Control Flow](../guide/config/control-flow.md) - Conditionals and when expressions
- [Variables](../guide/config/variables.md) - Template syntax and variables
