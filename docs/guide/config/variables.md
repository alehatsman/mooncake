# Variables

Variables make configurations reusable and dynamic. Use system facts and custom variables throughout your configuration.

## Defining Variables

### Inline Variables

```yaml
- vars:
    app_name: myapp
    version: "1.0.0"
    port: 8080
```

### External Variable Files

```yaml
- name: Load variables
  include_vars: ./vars/common.yml
```

**vars/common.yml:**
```yaml
app_name: myapp
environment: development
debug: true
```

### Dynamic Variable Loading

```yaml
- vars:
    env: production

- include_vars: ./vars/{{env}}.yml
```

## Using Variables

### In Shell Commands

```yaml
- vars:
    package: neovim

- shell: brew install {{package}}
```

### In File Paths

```yaml
- vars:
    app_dir: /opt/myapp

- file:
    path: "{{app_dir}}/config"
    state: directory
```

### In File Content

```yaml
- vars:
    api_key: secret123

- file:
    path: /tmp/config.txt
    state: file
    content: |
      api_key: {{api_key}}
      environment: production
```

### In Templates

**config.yml:**
```yaml
- template:
    src: ./nginx.conf.j2
    dest: /etc/nginx/nginx.conf
```

**nginx.conf.j2:**
```nginx
server {
    listen {{port}};
    server_name {{hostname}};
}
```

## System Facts

Mooncake automatically provides system information as variables.

### Basic Facts

```yaml
# Operating system
os: "linux"        # linux, darwin, windows
arch: "amd64"      # amd64, arm64, 386, etc.
hostname: "myserver"
user_home: "/home/user"
```

### Distribution Info

```yaml
distribution: "ubuntu"           # ubuntu, debian, centos, macos, etc.
distribution_version: "22.04"    # Full version
distribution_major: "22"         # Major version only
```

### Hardware Facts

```yaml
cpu_cores: 8                # Number of CPU cores
memory_total_mb: 16384      # Total RAM in megabytes
```

### Software Detection

```yaml
package_manager: "apt"      # apt, yum, brew, pacman, etc.
python_version: "3.10.0"    # Installed Python version
```

### Network Facts

```yaml
ip_addresses: ["192.168.1.100", "10.0.0.5"]
ip_addresses_string: "192.168.1.100, 10.0.0.5"
```

### GPU Facts

```yaml
gpus:
  - vendor: "NVIDIA"
    model: "RTX 4090"
  - vendor: "AMD"
    model: "RX 7900"
```

### Storage Facts

```yaml
storage_devices:
  - name: "sda"
    size_bytes: 500000000000
  - name: "nvme0n1"
    size_bytes: 1000000000000
```

### Network Interfaces

```yaml
network_interfaces:
  - name: "eth0"
    mac_address: "00:11:22:33:44:55"
  - name: "wlan0"
    mac_address: "AA:BB:CC:DD:EE:FF"
```

## Viewing System Facts

Run `mooncake explain` to see all available facts:

```bash
mooncake explain
```

Output shows:
- Operating system details
- CPU and memory
- GPUs
- Storage devices
- Network interfaces
- Package manager
- Python version

## Using System Facts

### OS Detection

```yaml
- shell: apt update
  when: os == "linux"

- shell: brew update
  when: os == "darwin"
```

### Distribution-Specific Commands

```yaml
- shell: apt install package
  when: distribution == "ubuntu" || distribution == "debian"

- shell: yum install package
  when: distribution == "centos" || distribution == "fedora"
```

### Architecture Detection

```yaml
- shell: install-amd64-binary
  when: arch == "amd64"

- shell: install-arm64-binary
  when: arch == "arm64"
```

### Memory-Based Decisions

```yaml
- name: Configure for high-memory system
  shell: set-large-buffers
  when: memory_total_mb >= 32000
```

### Package Manager Detection

```yaml
- shell: "{{package_manager}} install neovim"
  when: os == "linux"
```

## Variable Precedence

When the same variable is defined in multiple places:

1. **Template vars** (highest priority)
   ```yaml
   - template:
       vars:
         port: 9000
   ```

2. **Step-level vars**
   ```yaml
   - vars:
       port: 8080
   ```

3. **Included vars**
   ```yaml
   - include_vars: ./vars.yml
   ```

4. **System facts** (lowest priority)
   ```yaml
   # Automatically available
   os: "linux"
   ```

## Variable Scoping

Variables are available to all subsequent steps:

```yaml
# Step 1: Define
- vars:
    app_name: myapp

# Step 2: Use in same file
- shell: echo "{{app_name}}"

# Step 3: Use in included files
- include: ./tasks/setup.yml  # Can use app_name
```

## Register Variables

Capture command output as variables:

```yaml
- shell: whoami
  register: current_user

- shell: echo "User is {{current_user.stdout}}"

- file:
    path: "/home/{{current_user.stdout}}/config"
    state: file
```

## Loop Variables

Special `item` variable in loops:

```yaml
- vars:
    users: [alice, bob]

- name: Create directory for {{item}}
  file:
    path: "/home/{{item}}"
    state: directory
  with_items: "{{users}}"
```

## Best Practices

1. **Use descriptive names**
   ```yaml
   # Good
   database_host: "localhost"

   # Bad
   h: "localhost"
   ```

2. **Quote version strings**
   ```yaml
   # Good
   version: "1.0.0"

   # Bad (may be parsed as number)
   version: 1.0.0
   ```

3. **Group related variables**
   ```yaml
   - vars:
       # Database config
       db_host: localhost
       db_port: 5432
       db_name: myapp

       # App config
       app_port: 8080
       app_debug: false
   ```

4. **Use external files for environments**
   ```
   vars/
     development.yml
     staging.yml
     production.yml
   ```

5. **Use system facts when possible**
   ```yaml
   # Good - adapts to system
   - shell: "{{package_manager}} install curl"

   # Bad - hardcoded for one OS
   - shell: apt install curl
   ```

## See Also

- [Actions](actions.md) - Using variables in actions
- [Control Flow](control-flow.md) - Using variables in conditions
- [Examples](../../examples/index.md#02-variables-and-system-facts) - Variable examples
- [Commands](../../index.md#mooncake-explain) - View system facts
