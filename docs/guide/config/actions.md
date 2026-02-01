# Actions

Actions are the operations Mooncake performs. Each step in your configuration uses one action type.

## Shell

Execute shell commands.

```yaml
- name: Run command
  shell: echo "Hello"
```

### Multi-line Commands

```yaml
- name: Multiple commands
  shell: |
    echo "First"
    echo "Second"
    cd /tmp && ls -la
```

### With Variables

```yaml
- vars:
    package: neovim

- name: Install package
  shell: "{{package_manager}} install {{package}}"
```

## File

Create and manage files and directories.

### Create Directory

```yaml
- name: Create directory
  file:
    path: /tmp/myapp
    state: directory
    mode: "0755"
```

### Create File

```yaml
- name: Create empty file
  file:
    path: /tmp/config.txt
    state: file
    mode: "0644"
```

### Create File with Content

```yaml
- name: Create config
  file:
    path: /tmp/config.txt
    state: file
    mode: "0644"
    content: |
      key: value
      debug: true
```

### File Permissions

Common permission modes:
- `"0755"` - rwxr-xr-x (directories, executables)
- `"0644"` - rw-r--r-- (regular files)
- `"0600"` - rw------- (private files)
- `"0700"` - rwx------ (private directories)

## Template

Render templates with variables and logic.

```yaml
- name: Render config
  template:
    src: ./templates/config.yml.j2
    dest: /tmp/config.yml
    mode: "0644"
```

### With Additional Variables

```yaml
- template:
    src: ./templates/nginx.conf.j2
    dest: /etc/nginx/nginx.conf
    mode: "0644"
    vars:
      port: 8080
      ssl_enabled: true
```

### Template Syntax (pongo2)

**Variables:**
```jinja
server_name: {{ hostname }}
port: {{ port }}
```

**Conditionals:**
```jinja
{% if ssl_enabled %}
ssl on;
ssl_certificate {{ ssl_cert }};
{% endif %}
```

**Loops:**
```jinja
{% for server in servers %}
upstream {{ server.name }} {
    server {{ server.host }}:{{ server.port }};
}
{% endfor %}
```

**Filters:**
```jinja
home: {{ "~/.config" | expanduser }}
name: {{ app_name | upper }}
```

## Include

Load and execute tasks from other files.

```yaml
- name: Run common tasks
  include: ./tasks/common.yml
```

### Conditional Include

```yaml
- name: Run Linux tasks
  include: ./tasks/linux.yml
  when: os == "linux"
```

## Include Vars

Load variables from external files.

```yaml
- name: Load environment variables
  include_vars: ./vars/development.yml
```

### Dynamic Include

```yaml
- vars:
    env: production

- name: Load env-specific vars
  include_vars: ./vars/{{env}}.yml
```

## Action Properties

All actions support these properties:

### name

Human-readable description:
```yaml
- name: Install dependencies
  shell: npm install
```

### when

Conditional execution:
```yaml
- shell: brew install git
  when: os == "darwin"
```

### tags

Filter execution:
```yaml
- shell: npm test
  tags: [test, dev]
```

### become

Run with sudo:
```yaml
- shell: apt update
  become: true
```

### register

Capture output:
```yaml
- shell: whoami
  register: current_user
```

### with_items

Iterate over list:
```yaml
- shell: echo "{{item}}"
  with_items: ["a", "b", "c"]
```

### with_filetree

Iterate over files:
```yaml
- shell: cp "{{item.src}}" "/backup/{{item.name}}"
  with_filetree: ./dotfiles
```

## See Also

- [Control Flow](control-flow.md) - Conditionals, loops, tags
- [Variables](variables.md) - Variable usage and system facts
- [Examples](../../examples/index.md) - Practical examples
