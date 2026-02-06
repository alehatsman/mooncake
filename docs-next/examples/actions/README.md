# Mooncake Actions - Comprehensive Examples

This directory contains extensive examples for every Mooncake action, demonstrating both basic and advanced usage patterns.

## Overview

Each file focuses on a single action type with 20-50+ examples covering:

- Basic usage
- Advanced features
- Real-world scenarios
- Error handling
- Best practices

## Quick Start

Run any example file:
```bash
cd examples/actions
mooncake run --config shell.yml
mooncake run --config file.yml --tags basics
```

## Available Actions

### Core Actions

| File | Action | Description | Examples |
|------|--------|-------------|----------|
| **[shell.yml](shell.yml)** | `shell` | Execute shell commands | 50+ examples |
| **[print.yml](print.yml)** | `print` | Print messages | 60+ examples |
| **[vars.yml](vars.yml)** | `vars` | Define variables | 36+ examples |

### File Operations

| File | Action | Description | Examples |
|------|--------|-------------|----------|
| **[file.yml](file.yml)** | `file` | Create/manage files and directories | 50+ examples |
| **[copy.yml](copy.yml)** | `copy` | Copy files with verification | 23+ examples |
| **[template.yml](template.yml)** | `template` | Render Jinja2 templates | 27+ examples |

### Network Operations

| File | Action | Description | Examples |
|------|--------|-------------|----------|
| **[download.yml](download.yml)** | `download` | Download files from URLs | 26+ examples |
| **[unarchive.yml](unarchive.yml)** | `unarchive` | Extract archives (.tar, .zip, etc.) | 25+ examples |

### System Management

| File | Action | Description | Examples |
|------|--------|-------------|----------|
| **[service.yml](service.yml)** | `service` | Manage systemd/launchd services | 24+ examples |

### Validation & Control

| File | Action | Description | Examples |
|------|--------|-------------|----------|
| **[assert.yml](assert.yml)** | `assert` | Verify system state | 48+ examples |
| **[include.yml](include.yml)** | `include` | Load tasks from files | 23+ examples |

### Advanced

| File | Action | Description | Examples |
|------|--------|-------------|----------|
| **[preset.yml](preset.yml)** | `preset` | Use reusable workflows | 20+ examples |

## Running Examples

### Run All Examples
```bash
mooncake run --config shell.yml
```

### Run Specific Tags
```bash
mooncake run --config shell.yml --tags basics
mooncake run --config file.yml --tags permissions
mooncake run --config template.yml --tags real-world
```

### Run with Cleanup
```bash
mooncake run --config file.yml --tags cleanup
```

## Action Details

### shell.yml - Shell Commands
Execute commands with full shell capabilities:

- Basic commands and multi-line scripts
- Output capture with `register`
- Environment variables
- Working directory changes
- Timeouts and retries
- Custom failure/change conditions
- Different shell interpreters
- stdin input

**Example:**
```yaml
- name: Complex deployment
  shell: |
    echo "Deploying..."
    npm install
    npm run build
  env:
    NODE_ENV: production
  cwd: /opt/app
  timeout: 10m
  retries: 3
  register: deploy_result
```

### print.yml - Print Messages
Simple message output without shell:

- Basic messages
- Variable interpolation
- Multi-line output
- Conditional printing
- Loops
- Debug messages
- Progress indicators
- Formatted output

**Example:**
```yaml
- name: Deployment status
  print: |
    Deployed {{ app_name }} v{{ version }}
    Status: Complete
    Platform: {{ os }}/{{ arch }}
```

### file.yml - File Management
Create and manage files, directories, and links:

- Create files with content
- Create directories (nested)
- Set permissions (0644, 0755, 0600, etc.)
- Set ownership (owner/group)
- Create symlinks and hardlinks
- Remove files/directories
- Touch files (update timestamp)
- Recursive operations
- Backups

**Example:**
```yaml
- name: Create application config
  file:
    path: /opt/app/config.yml
    state: file
    content: |
      app: {{ app_name }}
      port: {{ port }}
    mode: "0644"
```

### copy.yml - Copy Files
Copy files with integrity verification:

- Simple file copy
- Copy with permissions
- Backup before overwrite
- Force overwrite
- Checksum verification
- Loops for multiple files

**Example:**
```yaml
- name: Deploy configuration
  copy:
    src: ./configs/production.yml
    dest: /opt/app/config.yml
    mode: "0600"
    backup: true
```

### template.yml - Template Rendering
Render Jinja2 templates with variables:

- Basic template rendering
- Variables and system facts
- Conditionals and loops
- Filters (upper, lower, default, etc.)
- Executable script generation
- Configuration files
- Service definitions

**Example:**
```yaml
- name: Render nginx config
  template:
    src: ./templates/nginx.conf.j2
    dest: /etc/nginx/nginx.conf
    mode: "0644"
    vars:
      port: 80
      server_name: example.com
```

### download.yml - Download Files
Download from URLs with retry support:

- Simple downloads
- Checksum verification (SHA256/MD5)
- Timeouts and retries
- Custom headers
- Authentication
- Force re-download
- Backups
- Integration with unarchive

**Example:**
```yaml
- name: Download Node.js
  download:
    url: "https://nodejs.org/dist/v20.11.0/node-v20.11.0-linux-x64.tar.gz"
    dest: "/tmp/node.tar.gz"
    checksum: "SHA256_HERE"
    timeout: "5m"
    retries: 3
```

### unarchive.yml - Extract Archives
Extract .tar, .tar.gz, .tgz, .zip files:

- Basic extraction
- Strip path components
- Idempotency with markers
- Permission management
- Security features (path traversal protection)
- Integration with download

**Example:**
```yaml
- name: Extract application
  unarchive:
    src: /tmp/app.tar.gz
    dest: /opt/app
    strip_components: 1
    creates: /opt/app/.installed
    mode: "0755"
```

### service.yml - Service Management
Manage systemd (Linux) and launchd (macOS) services:

- Start/stop/restart services
- Enable/disable on boot
- Create service unit files
- Drop-in configurations
- Environment variables
- Dependencies
- Resource limits
- Timer units (scheduled tasks)

**Example:**
```yaml
- name: Deploy application service
  service:
    name: myapp
    unit:
      content: |
        [Unit]
        Description=My Application
        After=network.target

        [Service]
        Type=simple
        ExecStart=/opt/app/bin/server
        Restart=on-failure

        [Install]
        WantedBy=multi-user.target
    daemon_reload: true
    state: started
    enabled: true
  become: true
```

### assert.yml - Assertions
Verify system state (never changes, fails fast):

- Command assertions (exit codes)
- File assertions (exists, content, permissions)
- HTTP assertions (status, response body)
- Prerequisites checking
- Deployment validation
- Security checks
- Assertions with retries

**Example:**
```yaml
- name: Verify deployment
  assert:
    file:
      path: /opt/app/binary
      exists: true
      mode: "0755"

- name: Check health endpoint
  assert:
    http:
      url: http://localhost:8080/health
      status: 200
      contains: "healthy"
```

### include.yml - Include Tasks
Load and execute tasks from external files:

- Basic includes
- Conditional includes
- Include with tags
- Multi-level includes
- Environment-specific includes
- Reusable task libraries
- Platform-specific includes
- Modular configuration patterns

**Example:**
```yaml
- name: Run prerequisites
  include: ./tasks/prerequisites.yml

- name: Deploy application
  include: ./tasks/deploy.yml

- name: Run Linux tasks
  include: ./tasks/linux.yml
  when: os == "linux"
```

### preset.yml - Presets
Use reusable, parameterized workflows:

- Basic preset invocation
- Ollama preset (install/configure LLMs)
- Parameters and variables
- Conditional execution
- Registration
- Integration patterns
- Custom preset creation

**Example:**
```yaml
- name: Setup Ollama with models
  preset: ollama
  with:
    state: present
    service: true
    pull:
      - "llama3.1:8b"
      - "mistral:latest"
  become: true
```

### vars.yml - Variables
Define and manage variables:

- Simple variables
- Different types (string, number, boolean, list, dict)
- Nested structures
- System facts
- Conditional variables
- Default values
- Configuration management
- Multi-environment patterns

**Example:**
```yaml
- vars:
    app_name: "MyApp"
    version: "1.0.0"
    database:
      host: localhost
      port: 5432
      name: myapp_db

- name: Use variables
  print: "Deploying {{ app_name }} v{{ version }}"
```

## Tags Reference

Common tags used across examples:

- `basics` - Fundamental usage
- `advanced` - Complex scenarios
- `loops` - Using with_items
- `conditional` - Conditional execution
- `register` - Output capture
- `variables` - Variable usage
- `real-world` - Practical scenarios
- `best-practices` - Recommended patterns
- `cleanup` - Cleanup operations
- `always` - Always runs

## Tips

1. **Start with basics:**
   ```bash
   mooncake run --config shell.yml --tags basics
   ```

2. **Explore specific features:**
   ```bash
   mooncake run --config file.yml --tags permissions
   ```

3. **Learn from real-world examples:**
   ```bash
   mooncake run --config template.yml --tags real-world
   ```

4. **Clean up after testing:**
   ```bash
   mooncake run --config file.yml --tags cleanup
   ```

5. **Run examples safely:**
   - All examples use /tmp for testing
   - Cleanup tasks remove test files
   - sudo operations are marked with `become: true`

## Documentation

For complete action documentation, see:

- [Actions Reference](../../docs/guide/config/actions.md)
- [Control Flow](../../docs/guide/config/control-flow.md)
- [Variables](../../docs/guide/config/variables.md)
- [Complete Reference](../../docs/guide/config/reference.md)

## Structure

```
examples/actions/
├── README.md                 # This file
├── shell.yml                 # Shell command examples
├── print.yml                 # Print message examples
├── file.yml                  # File operations examples
├── copy.yml                  # Copy file examples
├── template.yml              # Template rendering examples
├── download.yml              # Download file examples
├── unarchive.yml             # Archive extraction examples
├── service.yml               # Service management examples
├── assert.yml                # Assertion examples
├── include.yml               # Include task examples
├── preset.yml                # Preset usage examples
├── vars.yml                  # Variable definition examples
├── templates/                # Template files for examples
│   ├── simple-config.yml.j2
│   ├── nginx.conf.j2
│   ├── script.sh.j2
│   └── systemd-service.j2
└── tasks/                    # Task files for include examples
    ├── common.yml
    ├── setup.yml
    ├── linux-tasks.yml
    ├── macos-tasks.yml
    └── cleanup.yml
```

## Contributing

When adding new examples:

1. Follow the existing format
2. Include clear descriptions
3. Add appropriate tags
4. Test examples work
5. Update this README

## Next Steps

After exploring these examples:

1. Check out the [numbered examples](../) (01-12) for complete workflows
2. See [scenarios](../scenarios/) for real-world setups
3. Read the [official documentation](../../docs/)
4. Build your own configurations!

## Getting Help

- Documentation: `docs/guide/config/actions.md`
- Examples: This directory and `examples/01-12`
- Issues: GitHub issues
- Community: Discussions on GitHub
