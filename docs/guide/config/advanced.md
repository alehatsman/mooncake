# Advanced Configuration

Advanced patterns and techniques for complex configurations.

## Multi-File Organization

Break large configurations into manageable pieces.

### Directory Structure

```
project/
├── main.yml              # Entry point
├── tasks/                # Task modules
│   ├── common.yml
│   ├── linux.yml
│   └── macos.yml
├── vars/                 # Variable files
│   ├── dev.yml
│   └── prod.yml
└── templates/            # Template files
    └── config.j2
```

### Main Configuration

```yaml
# main.yml
- vars:
    env: development

- include_vars: ./vars/{{env}}.yml

- include: ./tasks/common.yml

- include: ./tasks/{{os}}.yml
```

### Benefits

- **Maintainability** - Easier to find and update
- **Reusability** - Share modules across projects
- **Collaboration** - Work on different files
- **Testing** - Test modules independently

See [Example 10](../../examples/index.md#10-multi-file-configurations) for details.

## Complex Conditionals

### Multiple Conditions

```yaml
- name: Install on Ubuntu 20+ with enough RAM
  shell: install-heavy-package
  when: >
    distribution == "ubuntu" &&
    distribution_major >= "20" &&
    memory_total_mb >= 8000
```

### Using Register Results

```yaml
- shell: docker --version
  register: docker

- shell: curl --version
  register: curl

- name: Run if both installed
  shell: deploy-app
  when: docker.rc == 0 && curl.rc == 0
```

### Checking Changed State

```yaml
- file:
    path: /tmp/config
    state: file
    content: "data"
  register: result

- name: Restart service if config changed
  shell: systemctl restart myapp
  become: true
  when: result.changed == true
```

## Advanced Loops

### Nested Data

```yaml
- vars:
    servers:
      - name: web1
        port: 8080
      - name: web2
        port: 8081

- name: Configure {{item.name}}
  template:
    src: ./server.conf.j2
    dest: "/etc/{{item.name}}.conf"
    vars:
      server_port: "{{item.port}}"
  with_items: "{{servers}}"
```

### Filtering File Trees

```yaml
# Only process .conf files
- name: Copy config files
  shell: cp "{{item.src}}" "/backup/{{item.name}}"
  with_filetree: ./configs
  when: item.name.endswith(".conf") && item.is_dir == false
```

### Multiple Loops

```yaml
- vars:
    environments: [dev, prod]
    services: [web, api, worker]

# First loop
- name: Create env directory
  file:
    path: "/opt/{{item}}"
    state: directory
  with_items: "{{environments}}"

# Second loop
- name: Configure service
  shell: setup-{{item}}
  with_items: "{{services}}"
```

## Dynamic Templates

### Template Variables

```yaml
- vars:
    servers:
      - host: server1.com
        port: 443
      - host: server2.com
        port: 443

- template:
    src: ./load-balancer.conf.j2
    dest: /etc/nginx/nginx.conf
```

**load-balancer.conf.j2:**
```nginx
upstream backend {
    {% for server in servers %}
    server {{server.host}}:{{server.port}};
    {% endfor %}
}

server {
    {% if ssl_enabled %}
    listen 443 ssl;
    ssl_certificate {{ssl_cert}};
    {% else %}
    listen 80;
    {% endif %}

    location / {
        proxy_pass http://backend;
    }
}
```

### Conditional Sections

```jinja
{% if os == "linux" %}
# Linux-specific config
user www-data;
pid /var/run/nginx.pid;
{% elif os == "darwin" %}
# macOS-specific config
user _www;
pid /usr/local/var/run/nginx.pid;
{% endif %}
```

### Template Filters

```jinja
# Expand home directory
config_path: {{ "~/.config/app" | expanduser }}

# String manipulation
app_name: {{ name | upper }}
description: {{ desc | lower }}

# Default values
port: {{ port | default:"8080" }}
```

## Workflow Orchestration

### Phased Deployment

```yaml
# Phase 1: Preparation
- name: Backup current version
  shell: backup-app
  tags: [backup, phase1]

- name: Stop services
  shell: systemctl stop myapp
  become: true
  tags: [stop, phase1]

# Phase 2: Deploy
- name: Deploy new version
  shell: install-new-version
  tags: [deploy, phase2]

# Phase 3: Start
- name: Start services
  shell: systemctl start myapp
  become: true
  tags: [start, phase3]

# Phase 4: Verify
- name: Health check
  shell: curl localhost:8080/health
  register: health
  tags: [verify, phase4]
```

**Run specific phases:**
```bash
# Run only backup and stop
mooncake run --config deploy.yml --tags phase1

# Run only deployment
mooncake run --config deploy.yml --tags phase2

# Run all phases
mooncake run --config deploy.yml
```

### Environment-Specific Workflows

```yaml
- vars:
    env: "{{ lookup('env', 'ENVIRONMENT') or 'dev' }}"

- include_vars: ./vars/{{env}}.yml

# Dev-specific steps
- name: Enable debug logging
  shell: enable-debug
  when: env == "dev"
  tags: [dev]

# Prod-specific steps
- name: Configure monitoring
  shell: setup-monitoring
  when: env == "prod"
  tags: [prod]
```

## Error Handling

### Check Before Action

```yaml
- shell: which docker
  register: docker_check

- name: Fail if Docker missing
  shell: echo "Docker required but not installed" && exit 1
  when: docker_check.rc != 0
```

### Conditional Installation

```yaml
- shell: python3 --version
  register: python

- name: Install Python
  shell: apt install python3
  become: true
  when: python.rc != 0
```

### Verify Operations

```yaml
- file:
    path: /tmp/important-file
    state: file
    content: "data"
  register: file_result

- shell: test -f /tmp/important-file
  register: verify

- name: Alert if verification failed
  shell: echo "File creation failed!" && exit 1
  when: verify.rc != 0
```

## Performance Optimization

### Skip Unchanged Files

```yaml
- name: Deploy config
  template:
    src: ./app.conf.j2
    dest: /etc/app.conf
  register: config

- name: Restart only if config changed
  shell: systemctl restart myapp
  become: true
  when: config.changed == true
```

### Batch Operations

```yaml
# Instead of individual package installs
- vars:
    packages: [git, curl, vim, tmux, htop]

- name: Install all packages at once
  shell: apt install -y {{packages | join(' ')}}
  become: true
```

### Targeted Execution

```bash
# Run only what you need
mooncake run --config config.yml --tags deploy

# Skip expensive operations
mooncake run --config config.yml --tags quick
```

## Debugging

### Verbose Logging

```bash
# Debug level shows all details
mooncake run --config config.yml --log-level debug
```

### Dry Run

```bash
# See what would run without executing
mooncake run --config config.yml --dry-run
```

### Selective Debugging

```yaml
- name: Debug info
  shell: |
    echo "OS: {{os}}"
    echo "Arch: {{arch}}"
    echo "Home: {{user_home}}"
  tags: [debug]
```

Run only debug steps:
```bash
mooncake run --config config.yml --tags debug
```

## Best Practices

1. **Start Simple** - Begin with basic config, add complexity gradually
2. **Test Incrementally** - Use `--dry-run` after each change
3. **Document Decisions** - Add comments explaining non-obvious choices
4. **Version Control** - Keep configurations in git
5. **Environment Separation** - Use separate variable files for dev/prod
6. **Tag Consistently** - Use a clear tagging scheme
7. **Fail Fast** - Validate prerequisites early in the workflow
8. **Idempotency** - Make operations safe to run multiple times

## See Also

- [Multi-File Example](../../examples/index.md#10-multi-file-configurations) - Organization patterns
- [Real-World Example](../../examples/index.md#dotfiles-manager) - Complete application
- [Control Flow](control-flow.md) - Conditionals and loops
- [Variables](variables.md) - Variable management
