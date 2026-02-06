# Creating Presets

This guide shows you how to create your own mooncake presets for sharing complex workflows and configurations.

## Preset Structure

### Flat Structure (Simple)

A preset is a YAML file with this structure:

```yaml
preset:
  name: my-preset
  description: What this preset does
  version: 1.0.0

  parameters:
    param1:
      type: string
      required: true
      description: Description of param1

    param2:
      type: bool
      default: false
      description: Description of param2

  steps:
    - name: First step
      shell: echo "{{ parameters.param1 }}"

    - name: Second step
      file:
        path: /tmp/flag
        state: file
      when: parameters.param2
```

### Directory Structure (Advanced)

For complex presets with multiple files, use a directory structure:

```
presets/
└── my-preset/
    ├── preset.yml           # Main preset definition
    ├── tasks/               # Modular task files
    │   ├── install.yml
    │   ├── configure.yml
    │   └── cleanup.yml
    ├── templates/           # Configuration templates
    │   ├── config.j2
    │   └── service.j2
    └── README.md            # Documentation
```

The main preset file uses `include` to organize steps:

```yaml
# presets/my-preset/preset.yml
preset:
  name: my-preset
  description: Modular preset with includes
  version: 1.0.0

  parameters:
    state:
      type: string
      enum: [present, absent]

  steps:
    - name: Install
      include: tasks/install.yml
      when: parameters.state == "present"

    - name: Configure
      include: tasks/configure.yml
      when: parameters.state == "present"

    - name: Cleanup
      include: tasks/cleanup.yml
      when: parameters.state == "absent"
```

## Minimal Example

The simplest preset:

```yaml
preset:
  name: hello
  description: Print hello message
  version: 1.0.0

  steps:
    - name: Say hello
      shell: echo "Hello from preset!"
```

Usage:
```yaml
- preset: hello
```

## Parameters

### Defining Parameters

```yaml
parameters:
  environment:
    type: string
    required: true
    enum: [dev, staging, production]
    description: Deployment environment

  replicas:
    type: number
    required: false
    default: 3
    description: Number of replicas

  features:
    type: array
    required: false
    default: []
    description: Feature flags to enable

  config:
    type: object
    required: false
    description: Additional configuration
```

### Parameter Types

| Type | Go Type | YAML Example |
|------|---------|--------------|
| `string` | `string` | `"value"` |
| `bool` | `bool` | `true` / `false` |
| `array` | `[]interface{}` | `[item1, item2]` |
| `object` | `map[string]interface{}` | `{key: value}` |

### Accessing Parameters

Parameters are available under the `parameters` namespace:

```yaml
steps:
  - name: Use string parameter
    shell: echo "Env{{ ":" }} {{ parameters.environment }}"

  - name: Use boolean parameter
    file:
      path: /tmp/feature
      state: file
    when: parameters.enable_feature

  - name: Loop over array parameter
    shell: echo "Feature{{ ":" }} {{ item }}"
    with_items: "{{ parameters.features }}"

  - name: Access object parameter
    shell: echo "DB{{ ":" }} {{ parameters.config.database_url }}"
```

## Includes

### Using Includes for Modularity

Break large presets into smaller, focused files using `include`:

```yaml
# preset.yml
steps:
  - name: Run installation tasks
    include: tasks/install.yml

  - name: Run configuration tasks
    include: tasks/configure.yml
```

```yaml
# tasks/install.yml
- name: Check if already installed
  shell: command -v myapp
  register: check
  failed_when: false

- name: Install if not present
  shell: ./install.sh
  when: check.rc != 0
```

### Path Resolution

**All paths in presets resolve relative to the file they're written in** (Node.js-style):

```
presets/my-preset/
├── preset.yml
├── tasks/
│   └── configure.yml
└── templates/
    └── config.j2
```

From `tasks/configure.yml`, reference the template:

```yaml
# tasks/configure.yml
- name: Render config
  template:
    src: ../templates/config.j2  # Relative to tasks/ directory
    dest: /etc/myapp/config
```

From `preset.yml`, reference the template directly:

```yaml
# preset.yml
- name: Render config
  template:
    src: templates/config.j2  # Relative to preset.yml
    dest: /etc/myapp/config
```

**Key principle**: Paths are always relative to the YAML file containing them, not the preset root.

### Nested Includes

Includes can include other files (but avoid deep nesting):

```yaml
# preset.yml
steps:
  - include: tasks/setup.yml

# tasks/setup.yml
- include: common/dependencies.yml
- include: common/permissions.yml
```

### Include Conditions

Apply conditions to entire include blocks:

```yaml
steps:
  - name: Linux setup
    include: tasks/linux.yml
    when: os == "linux"

  - name: macOS setup
    include: tasks/macos.yml
    when: os == "darwin"
```

## Steps

### Using Built-in Actions

Presets can use any mooncake action **except other presets** (no nesting):

```yaml
steps:
  # Shell commands
  - name: Run script
    shell: ./install.sh
    become: true

  # File operations
  - name: Create config
    file:
      path: /etc/myapp/config.yml
      state: file
      content: |
        port: {{ parameters.port }}

  # Template rendering
  - name: Render template
    template:
      src: ./templates/config.j2
      dest: /etc/myapp/config
      vars:
        port: "{{ parameters.port }}"

  # Service management
  - name: Start service
    service:
      name: myapp
      state: started
      enabled: true
```

### Conditionals

Use `when` to execute steps conditionally:

```yaml
steps:
  - name: Install on Ubuntu
    shell: apt-get install -y myapp
    when: os == "linux" and apt_available
    become: true

  - name: Install on macOS
    shell: brew install myapp
    when: os == "darwin" and brew_available

  - name: Configure if parameter set
    file:
      path: /etc/myapp/config
      state: file
    when: parameters.configure == true
```

### Variables and Facts

Presets have access to:

**Parameters** (via `parameters` namespace):
```yaml
{{ parameters.my_param }}
```

**Variables** (playbook-level):
```yaml
{{ my_variable }}
```

**Facts** (system information):
```yaml
{{ os }}
{{ arch }}
{{ hostname }}
```

**Step Results** (via `register`):
```yaml
steps:
  - name: Check something
    shell: which myapp
    register: check_result
    failed_when: false

  - name: Use result
    shell: echo "Found at {{ check_result.stdout }}"
    when: check_result.rc == 0
```

## Platform Handling

### Detect Package Managers

Use facts to detect available package managers:

```yaml
steps:
  - name: Install via apt
    shell: apt-get install -y {{ parameters.package }}
    when: apt_available
    become: true

  - name: Install via dnf
    shell: dnf install -y {{ parameters.package }}
    when: dnf_available
    become: true

  - name: Install via brew
    shell: brew install {{ parameters.package }}
    when: brew_available
```

Available package manager facts:

- `apt_available` (Debian/Ubuntu)
- `dnf_available` (Fedora/RHEL 8+)
- `yum_available` (RHEL/CentOS 7)
- `pacman_available` (Arch)
- `zypper_available` (openSUSE)
- `apk_available` (Alpine)
- `brew_available` (macOS/Linux)

### Operating System Detection

```yaml
steps:
  - name: Linux-specific step
    shell: systemctl start myapp
    when: os == "linux"

  - name: macOS-specific step
    shell: launchctl load ~/Library/LaunchAgents/myapp.plist
    when: os == "darwin"
```

## Service Configuration

### systemd (Linux)

```yaml
steps:
  - name: Configure systemd service
    service:
      name: myapp
      state: started
      enabled: true
      daemon_reload: true
      dropin:
        name: 10-preset.conf
        content: |
          [Service]
          {% if parameters.host %}
          Environment="HOST={{ parameters.host }}"
          {% endif %}
          {% if parameters.port %}
          Environment="PORT={{ parameters.port }}"
          {% endif %}
    become: true
    when: os == "linux"
```

### launchd (macOS)

```yaml
steps:
  - name: Configure launchd service
    service:
      name: com.example.myapp
      state: started
      enabled: true
      unit:
        content: |
          <?xml version="1.0" encoding="UTF-8"?>
          <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
          <plist version="1.0">
          <dict>
            <key>Label</key>
            <string>com.example.myapp</string>
            <key>ProgramArguments</key>
            <array>
              <string>/usr/local/bin/myapp</string>
            </array>
            {% if parameters.host or parameters.port %}
            <key>EnvironmentVariables</key>
            <dict>
              {% if parameters.host %}
              <key>HOST</key>
              <string>{{ parameters.host }}</string>
              {% endif %}
              {% if parameters.port %}
              <key>PORT</key>
              <string>{{ parameters.port }}</string>
              {% endif %}
            </dict>
            {% endif %}
            <key>RunAtLoad</key>
            <true/>
            <key>KeepAlive</key>
            <true/>
          </dict>
          </plist>
    when: os == "darwin"
```

## Error Handling

### Validation

Validate parameters at the start:

```yaml
steps:
  - name: Validate port range
    shell: test {{ parameters.port }} -ge 1024 && test {{ parameters.port }} -le 65535
    when: parameters.port is defined

  - name: Validate required files
    shell: test -f {{ parameters.config_file }}
    when: parameters.config_file is defined
```

### Idempotency

Make steps idempotent:

```yaml
steps:
  # Check before installing
  - name: Check if already installed
    shell: command -v myapp
    register: check
    failed_when: false

  - name: Install only if not present
    shell: ./install.sh
    when: check.rc != 0

  # Use 'creates' for idempotency
  - name: Download archive
    shell: curl -L -o /tmp/myapp.tar.gz {{ parameters.url }}
    creates: /tmp/myapp.tar.gz
```

### Failed When

Control when steps should fail:

```yaml
steps:
  - name: Try package manager install
    shell: apt-get install -y myapp
    register: apt_install
    failed_when: false

  - name: Fallback to script install
    shell: curl -fsSL https://get.myapp.com | sh
    when: apt_install.rc != 0
```

## Complete Example: Custom Application Preset

```yaml
preset:
  name: deploy-webapp
  description: Deploy a web application with service management
  version: 1.0.0

  parameters:
    app_name:
      type: string
      required: true
      description: Application name

    version:
      type: string
      required: true
      description: Version to deploy (e.g., v1.2.3)

    port:
      type: number
      default: 8080
      description: Application port

    environment:
      type: string
      default: production
      enum: [development, staging, production]
      description: Deployment environment

    enable_service:
      type: bool
      default: true
      description: Configure and start systemd/launchd service

  steps:
    # Step 1: Create application directory
    - name: Create app directory
      file:
        path: "/opt/{{ parameters.app_name }}"
        state: directory
        mode: "0755"
      become: true

    # Step 2: Download application binary
    - name: Download application
      shell: |
        curl -L -o /opt/{{ parameters.app_name }}/app \
          https://releases.example.com/{{ parameters.app_name }}/{{ parameters.version }}/app
        chmod +x /opt/{{ parameters.app_name }}/app
      become: true
      creates: "/opt/{{ parameters.app_name }}/app"

    # Step 3: Create configuration file
    - name: Create config file
      file:
        path: "/etc/{{ parameters.app_name }}/config.yml"
        state: file
        mode: "0644"
        content: |
          app_name: {{ parameters.app_name }}
          version: {{ parameters.version }}
          port: {{ parameters.port }}
          environment: {{ parameters.environment }}
      become: true

    # Step 4: Configure systemd service (Linux)
    - name: Configure systemd service
      service:
        name: "{{ parameters.app_name }}"
        state: started
        enabled: true
        unit:
          content: |
            [Unit]
            Description={{ parameters.app_name }} service
            After=network.target

            [Service]
            Type=simple
            User=www-data
            WorkingDirectory=/opt/{{ parameters.app_name }}
            ExecStart=/opt/{{ parameters.app_name }}/app
            Restart=always
            RestartSec=10
            Environment="PORT={{ parameters.port }}"
            Environment="ENV={{ parameters.environment }}"

            [Install]
            WantedBy=multi-user.target
      become: true
      when: parameters.enable_service and os == "linux"

    # Step 5: Configure launchd service (macOS)
    - name: Configure launchd service
      service:
        name: "com.example.{{ parameters.app_name }}"
        state: started
        enabled: true
        unit:
          content: |
            <?xml version="1.0" encoding="UTF-8"?>
            <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
            <plist version="1.0">
            <dict>
              <key>Label</key>
              <string>com.example.{{ parameters.app_name }}</string>
              <key>ProgramArguments</key>
              <array>
                <string>/opt/{{ parameters.app_name }}/app</string>
              </array>
              <key>WorkingDirectory</key>
              <string>/opt/{{ parameters.app_name }}</string>
              <key>EnvironmentVariables</key>
              <dict>
                <key>PORT</key>
                <string>{{ parameters.port }}</string>
                <key>ENV</key>
                <string>{{ parameters.environment }}</string>
              </dict>
              <key>RunAtLoad</key>
              <true/>
              <key>KeepAlive</key>
              <true/>
              <key>StandardOutPath</key>
              <string>/var/log/{{ parameters.app_name }}.log</string>
              <key>StandardErrorPath</key>
              <string>/var/log/{{ parameters.app_name }}-error.log</string>
            </dict>
            </plist>
      become: true
      when: parameters.enable_service and os == "darwin"

    # Step 6: Wait for service to be ready
    - name: Wait for service
      assert:
        http:
          url: "http://localhost:{{ parameters.port }}/health"
          status: 200
          timeout: "5s"
      retries: 10
      retry_delay: "3s"
      when: parameters.enable_service
```

Usage:
```yaml
- name: Deploy my web app
  preset: deploy-webapp
  with:
    app_name: mywebapp
    version: v1.2.3
    port: 8080
    environment: production
    enable_service: true
  become: true
  register: deploy_result
```

## Best Practices

### 1. Single Responsibility

Each preset should do one thing well:

**Good**: `install-postgres`, `configure-postgres`, `backup-postgres`

**Avoid**: `setup-everything` (monolithic preset)

### 2. Sensible Defaults

Choose defaults that work for 80% of users:

```yaml
parameters:
  port:
    type: number
    default: 8080  # Common default

  enabled:
    type: bool
    default: true  # Most users want this enabled
```

### 3. Clear Documentation

Document every parameter:

```yaml
parameters:
  timeout:
    type: number
    default: 30
    description: Connection timeout in seconds (1-300)
```

### 4. Platform Detection

Use facts, don't hardcode:

```yaml
# Good
when: apt_available

# Bad
when: os == "linux"  # Not all Linux distros have apt
```

### 5. Fail Fast

Validate inputs early:

```yaml
steps:
  - name: Validate version format
    shell: echo "{{ parameters.version }}" | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$'
```

### 6. Idempotent Operations

Every step should be safe to run multiple times:

```yaml
- name: Create directory (idempotent)
  file:
    path: /opt/myapp
    state: directory

- name: Download if not exists (idempotent)
  shell: curl -o /tmp/file https://example.com/file
  creates: /tmp/file
```

### 7. Version Your Presets

Use semantic versioning:

```yaml
preset:
  version: 1.2.3  # Breaking.Feature.Fix
```

## Testing Presets

### Dry Run

Always test with `--dry-run` first:

```bash
mooncake run -c test-preset.yml --dry-run
```

### Multiple Platforms

Test on different operating systems:

```yaml
# test-preset.yml
- name: Test on current OS
  preset: my-preset
  with:
    state: present
```

### Parameter Validation

Test with missing/invalid parameters:

```yaml
# Should fail
- preset: my-preset
  # Missing required parameter

# Should fail
- preset: my-preset
  with:
    invalid_param: value
```

## Distribution

### Local Presets

Place in playbook directory:

```
my-project/
├── playbook.yml
└── presets/
    └── custom.yml
```

### User Presets

Install to user directory:

```bash
mkdir -p ~/.mooncake/presets
cp my-preset.yml ~/.mooncake/presets/
```

### System Presets

Install system-wide:

```bash
sudo mkdir -p /usr/share/mooncake/presets
sudo cp my-preset.yml /usr/share/mooncake/presets/
```

### Sharing

Share presets via:

- Git repositories
- Package managers
- Direct file distribution

## Limitations

Current architectural constraints:

1. **No Nesting**: Presets cannot call other presets (architectural decision for simplicity)
2. **Sequential Execution**: Steps execute in order, not parallel (may be relaxed in future)
3. **Parameter Types**: Only string, bool, array, object types supported

**Note**: Preset steps fully support includes, loops (with_items, with_filetree), and conditionals (when). The preset definition file itself must be static YAML, but the steps within can be dynamically expanded.

## Next Steps

- [Using Presets Guide](presets.md)
- [Built-in Presets](#) <!-- TODO -->
- [Community Presets](#) <!-- TODO -->
