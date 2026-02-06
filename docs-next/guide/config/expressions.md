# Expression Syntax Reference

Mooncake uses **expr-lang** for expressions in conditionals (`when`), custom change detection (`changed_when`), and custom failure detection (`failed_when`).

!!! info "Quick Start"
    Expressions are string values evaluated at runtime:
    ```yaml
    when: "os == 'darwin' && cpu_cores > 4"
    changed_when: "'installed' in result.stdout"
    failed_when: "result.rc != 0 || 'error' in result.stderr"
    ```

---

## Operators

### Comparison Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `==` | Equal | `os == "linux"` |
| `!=` | Not equal | `arch != "arm64"` |
| `>` | Greater than | `cpu_cores > 4` |
| `>=` | Greater or equal | `memory_total_mb >= 8192` |
| `<` | Less than | `disk_free_percent < 20` |
| `<=` | Less or equal | `cpu_load <= 2.0` |

**Examples:**

```yaml
- name: Install on Linux
  shell: apt install -y neovim
  when: os == "linux"

- name: Check high CPU systems
  print: "High-performance system detected"
  when: cpu_cores >= 16
```

### Logical Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `&&` | Logical AND | `os == "linux" && arch == "amd64"` |
| `\|\|` | Logical OR | `os == "darwin" \|\| os == "linux"` |
| `!` | Logical NOT | `!is_docker_installed` |

**Examples:**

```yaml
- name: Install on Ubuntu/Debian
  shell: apt install -y docker.io
  become: true
  when: os == "linux" && (distribution == "ubuntu" || distribution == "debian")

- name: Warn if not root
  print: "Warning: May need sudo"
  when: !(user == "root")
```

### String Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `in` | Contains substring | `"error" in result.stderr` |
| `not in` | Does not contain | `"success" not in output` |
| `matches` | Regex match | `version matches "^v?[0-9]+"` |
| `contains` | Same as `in` | `stderr contains "warning"` |
| `startsWith` | Prefix check | `path startsWith "/usr"` |
| `endsWith` | Suffix check | `file endsWith ".yml"` |
| `+` | Concatenation | `"/usr/bin/" + app_name` |

**Examples:**

```yaml
- name: Check for errors
  shell: ./run-tests.sh
  register: test_result
  failed_when: "'FAILED' in test_result.stdout"

- name: Detect version format
  print: "Valid version detected"
  when: app_version matches "^[0-9]+\\.[0-9]+\\.[0-9]+$"

- name: Process YAML files only
  shell: validate-config {{item}}
  with_filetree: ./configs
  when: item.name endsWith ".yml" || item.name endsWith ".yaml"
```

### Arithmetic Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `+` | Addition | `cpu_cores + 2` |
| `-` | Subtraction | `memory_total_mb - 1024` |
| `*` | Multiplication | `timeout * 2` |
| `/` | Division | `disk_total_gb / 1024` |
| `%` | Modulo | `port % 1000` |
| `**` | Exponentiation | `2 ** 10` |

**Examples:**

```yaml
- name: Check if half memory is free
  assert:
    command: test {{memory_free_mb}} -gt $(({{memory_total_mb}} / 2))

- name: Use exponential backoff
  shell: retry-command.sh
  retry_delay: "{{2 ** retry_count}}s"
```

---

## Variable Access

### Direct Variables

Access variables defined in `vars` blocks:

```yaml
- vars:
    app_name: myapp
    version: "1.0.0"

- name: Show version
  print: "{{app_name}} v{{version}}"
  when: version != ""
```

### System Facts

Access auto-detected system information:

```yaml
- name: Platform-specific action
  shell: echo "Running on {{os}}/{{arch}}"
  when: os == "darwin" && arch == "arm64"

- name: Check resources
  print: "CPU: {{cpu_cores}} cores, RAM: {{memory_total_mb}}MB"
  when: cpu_cores >= 4 && memory_total_mb >= 8192
```

**Common facts:**

- `os`, `distribution`, `arch`, `hostname`, `user`
- `cpu_cores`, `cpu_model`, `memory_total_mb`, `memory_free_mb`
- `package_manager`, `python_version`, `go_version`, `docker_version`

See [Facts Reference](../../api/facts.md) for complete list.

### Result Variables

Access results from previous steps with `register`:

```yaml
- name: Check service status
  shell: systemctl is-active nginx
  register: nginx_status
  ignore_errors: true

- name: Start if not running
  service:
    name: nginx
    state: started
  when: nginx_status.rc != 0
  become: true

- name: Show service output
  print: "Service was: {{nginx_status.stdout}}"
```

**Result fields:**

- `result.rc` - Exit code (0 = success)
- `result.stdout` - Standard output
- `result.stderr` - Standard error
- `result.changed` - Whether step changed state
- `result.failed` - Whether step failed

### Nested Access

Access nested structures with dot notation:

```yaml
- vars:
    config:
      server:
        host: localhost
        port: 8080
      features:
        ssl: true
        auth: true

- name: Configure server
  template:
    src: server.conf.j2
    dest: /etc/server.conf
  when: config.features.ssl == true && config.server.port > 1024
```

---

## Functions

### String Functions

| Function | Description | Example |
|----------|-------------|---------|
| `len(s)` | String length | `len(password) >= 8` |
| `trim(s)` | Remove whitespace | `trim(result.stdout) == "ok"` |
| `upper(s)` | Uppercase | `upper(env) == "PRODUCTION"` |
| `lower(s)` | Lowercase | `lower(os) == "linux"` |
| `split(s, sep)` | Split string | `split(result.stdout, ",")` |

**Examples:**

```yaml
- name: Validate password
  assert:
    command: test {{len(password)}} -ge 8
  when: password is defined

- name: Normalize environment
  vars:
    normalized_env: "{{lower(environment)}}"
  when: environment is defined

- name: Process CSV output
  shell: echo {{item}}
  with_items: "{{split(csv_data, ',')}}"
```

### Collection Functions

| Function | Description | Example |
|----------|-------------|---------|
| `len(arr)` | Array length | `len(packages) > 0` |
| `all(arr, pred)` | All match | `all(results, .changed)` |
| `any(arr, pred)` | Any match | `any(checks, .failed)` |
| `filter(arr, pred)` | Filter items | `filter(files, .size > 1000)` |
| `map(arr, expr)` | Transform items | `map(users, .name)` |

**Examples:**

```yaml
- name: Check all services started
  print: "All services running"
  when: all(service_results, .rc == 0)

- name: Alert if any checks failed
  print: "Some checks failed!"
  when: any(health_checks, .failed == true)

- name: Process large files only
  shell: compress {{item.path}}
  with_items: "{{filter(files, .size > 10000000)}}"
```

### Type Checking

| Function | Description | Example |
|----------|-------------|---------|
| `type(v)` | Get type name | `type(value) == "string"` |
| `int(v)` | Convert to int | `int(port) > 1024` |
| `float(v)` | Convert to float | `float(version) >= 1.5` |
| `string(v)` | Convert to string | `string(port) + ":8080"` |

**Examples:**

```yaml
- name: Validate port number
  assert:
    command: test {{int(port)}} -gt 1024 -a {{int(port)}} -lt 65535
  when: port is defined

- name: Compare versions
  shell: upgrade-app.sh
  when: float(current_version) < float(target_version)
```

---

## Conditionals (when)

### Basic Usage

```yaml
- name: Linux only
  shell: apt update
  when: os == "linux"
  become: true

- name: macOS only
  shell: brew update
  when: os == "darwin"
```

### Complex Conditions

```yaml
- name: Install on Ubuntu 20.04+
  shell: apt install -y neovim
  when: |
    os == "linux" &&
    distribution == "ubuntu" &&
    float(distribution_version) >= 20.04
  become: true

- name: High-performance systems only
  shell: enable-turbo-mode.sh
  when: |
    cpu_cores >= 16 &&
    memory_total_mb >= 32768 &&
    (arch == "amd64" || arch == "arm64")
```

### Result-Based Conditionals

```yaml
- name: Check if file exists
  shell: test -f /etc/config.yml
  register: config_check
  ignore_errors: true

- name: Create config if missing
  file:
    path: /etc/config.yml
    state: file
    content: "default: config"
  when: config_check.rc != 0

- name: Validate config content
  shell: validate-config.sh
  when: config_check.rc == 0 && "error" not in config_check.stderr
```

---

## Custom Change Detection (changed_when)

By default, shell commands always report `changed: true`. Use `changed_when` to detect changes based on output:

```yaml
- name: Install package
  shell: apt install -y neovim
  become: true
  register: install_result
  changed_when: "'is already the newest version' not in install_result.stdout"

- name: Reload service
  shell: systemctl reload nginx
  become: true
  register: reload_result
  changed_when: "'Reloaded' in reload_result.stdout"

- name: Update configuration
  shell: config-tool set key value
  register: config_result
  changed_when: config_result.stdout contains "Updated"
```

### Multiple Conditions

```yaml
- name: Deploy application
  shell: ./deploy.sh
  register: deploy_result
  changed_when: |
    deploy_result.rc == 0 &&
    ("deployed" in deploy_result.stdout || "updated" in deploy_result.stdout)
```

### Never Changed

```yaml
- name: Check status only
  shell: systemctl status nginx
  register: status_check
  changed_when: false  # Never report as changed
```

---

## Custom Failure Detection (failed_when)

Override default failure detection (non-zero exit code):

```yaml
- name: Check service health
  shell: curl -f http://localhost:8080/health
  register: health_check
  failed_when: |
    health_check.rc != 0 ||
    "unhealthy" in health_check.stdout

- name: Run tests
  shell: npm test
  register: test_result
  failed_when: |
    test_result.rc != 0 ||
    "FAILED" in test_result.stdout ||
    int(test_result.stdout.split("passed")[0]) < 100
```

### Multiple Failure Conditions

```yaml
- name: Deploy with validation
  shell: ./deploy.sh --validate
  register: deploy_result
  failed_when: |
    deploy_result.rc != 0 ||
    "error" in lower(deploy_result.stderr) ||
    "failed" in lower(deploy_result.stdout) ||
    len(deploy_result.stderr) > 0
```

### Never Fail

```yaml
- name: Optional cleanup
  shell: rm -rf /tmp/cache
  failed_when: false  # Never fail (similar to ignore_errors)
```

---

## Common Patterns

### Platform Detection

```yaml
# Linux distributions
when: os == "linux" && distribution == "ubuntu"
when: os == "linux" && package_manager == "apt"
when: os == "linux" && (distribution == "debian" || distribution == "ubuntu")

# macOS versions
when: os == "darwin" && float(os_version) >= 13.0

# Architecture
when: arch == "amd64" || arch == "arm64"
when: arch in ["amd64", "arm64", "x86_64"]
```

### Resource Checks

```yaml
# CPU cores
when: cpu_cores >= 4
when: cpu_cores > 1 && cpu_cores <= 8

# Memory
when: memory_total_mb >= 8192
when: memory_free_mb > (memory_total_mb / 2)

# Disk space
when: disk_free_gb >= 10
```

### Version Comparisons

```yaml
# Semantic versions
when: float(python_version) >= 3.8
when: go_version matches "^1\\.(2[0-9]|[3-9][0-9])"

# Package versions
when: int(split(version, ".")[0]) >= 2
```

### String Matching

```yaml
# Exact match
when: environment == "production"

# Contains
when: "'staging' in environment || 'prod' in environment"

# Regex
when: hostname matches "^web-[0-9]+-prod$"

# Prefix/suffix
when: path startsWith "/usr/local/"
when: filename endsWith ".yml" || filename endsWith ".yaml"
```

### Result Inspection

```yaml
# Exit code
when: result.rc == 0
failed_when: result.rc != 0 && result.rc != 2

# Output contains
when: "'success' in result.stdout"
failed_when: "'error' in result.stderr || 'failed' in result.stdout"

# Changed state
when: result.changed == true
changed_when: result.stdout contains "Updated"

# Empty output
when: len(result.stdout) == 0
failed_when: len(result.stderr) > 0
```

### Collection Operations

```yaml
# Check if list has items
when: len(packages) > 0
when: len(filter(services, .status == "active")) == len(services)

# All/any
when: all(health_checks, .rc == 0)
when: any(warnings, .severity == "high")
```

---

## Escape Sequences

### Quotes in Strings

```yaml
# Single quotes (no escaping needed)
when: 'result.stdout == "value"'

# Double quotes (escape with backslash)
when: "result.stdout == \"value\""

# Mixed quotes
when: "result.stdout contains 'error message'"
```

### Special Characters

```yaml
# Newlines
when: "result.stdout contains '\\n'"

# Tabs
when: "result.stdout contains '\\t'"

# Backslashes
when: "path == 'C:\\\\Program Files\\\\App'"
```

---

## Debugging Expressions

### Print Variable Values

```yaml
- name: Debug variables
  print: |
    os: {{os}}
    arch: {{arch}}
    cpu_cores: {{cpu_cores}}
    Expression result: {{os == "linux" && cpu_cores > 4}}
```

### Test Expressions in Shell

```yaml
- name: Test condition
  shell: |
    echo "os={{os}}"
    echo "condition result=$(({{os}} == "linux"))"
  register: debug_output

- name: Show result
  print: "{{debug_output.stdout}}"
```

### Dry-Run Mode

```bash
# See which steps would execute
mooncake run config.yml --dry-run

# See conditions evaluated
mooncake run config.yml --dry-run --verbose
```

---

## Performance Tips

1. **Simple expressions are faster**:
   ```yaml
   # Fast
   when: os == "linux"

   # Slower (regex compilation)
   when: os matches "^(linux|darwin)$"
   ```

2. **Cache expensive operations**:
   ```yaml
   - vars:
       is_prod: "{{environment == 'production'}}"

   - name: Production task 1
     shell: task1.sh
     when: is_prod

   - name: Production task 2
     shell: task2.sh
     when: is_prod
   ```

3. **Use short-circuit evaluation**:
   ```yaml
   # Checks is_defined first (fast), then expensive regex
   when: is_defined && version matches "^[0-9]+\\.[0-9]+"
   ```

---

## See Also

- **[Control Flow](control-flow.md)** - Conditionals, loops, tags
- **[Variables](variables.md)** - Working with variables
- **[Facts Reference](../../api/facts.md)** - All available system facts
- **[Actions](actions.md)** - Complete actions reference
