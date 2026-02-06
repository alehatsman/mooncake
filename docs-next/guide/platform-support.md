# Platform Support

Mooncake is designed to work across multiple operating systems with platform-aware action validation.

## Overview

Actions in Mooncake declare their platform support through metadata. The planner validates platform compatibility at plan-time, failing fast if an unsupported action is detected on the current platform.

## Supported Platforms

Mooncake supports the following platforms:

- **Linux** - All major distributions (Ubuntu, Debian, RHEL, Fedora, Arch, etc.)
- **macOS** (Darwin) - All recent versions
- **Windows** - Windows 10/11, Windows Server
- **FreeBSD** - Limited support (package manager only)

## Platform Support Matrix

The table below shows which actions are supported on which platforms:

| Action | Linux | macOS | Windows | FreeBSD | Notes |
|--------|-------|-------|---------|---------|-------|
| **assert** | ✓ | ✓ | ✓ | ✓ | Verification action, platform-agnostic |
| **command** | ✓ | ✓ | ✓ | ✓ | Direct command execution |
| **copy** | ✓ | ✓ | ✓ | ✓ | File operations with checksums |
| **download** | ✓ | ✓ | ✓ | ✓ | HTTP/HTTPS downloads |
| **file** | ✓ | ✓ | ✓ | ✓ | File, directory, and link management |
| **include_vars** | ✓ | ✓ | ✓ | ✓ | YAML variable loading |
| **package** | ✓ | ✓ | ✓ | ✓ | Multiple package managers (apt, dnf, yum, pacman, zypper, apk, brew, port, choco, scoop) |
| **preset** | ✓ | ✓ | ✓ | ✓ | Meta-action, platform support depends on constituent steps |
| **print** | ✓ | ✓ | ✓ | ✓ | Output messages |
| **service** | ✓ | ✓ | ✓ | ✗ | systemd (Linux), launchd (macOS), Windows Services |
| **shell** | ✓ | ✓ | ✓ | ✓ | Shell command execution (bash, sh, pwsh, cmd) |
| **template** | ✓ | ✓ | ✓ | ✓ | Template rendering |
| **unarchive** | ✓ | ✓ | ✓ | ✓ | Archive extraction (tar, tar.gz, zip) |
| **vars** | ✓ | ✓ | ✓ | ✓ | Variable management |

### Action Capabilities

Each action also declares additional capabilities:

| Action | Requires Sudo | Implements Idempotency Check |
|--------|---------------|------------------------------|
| **assert** | No | N/A (verification only) |
| **command** | Depends | No |
| **copy** | Depends | Yes (checksums) |
| **download** | Depends | Yes (file existence + checksum) |
| **file** | Depends | Yes (existence, permissions, ownership) |
| **include_vars** | No | No |
| **package** | Yes | Yes (package installation status) |
| **preset** | Depends | Depends (delegates to steps) |
| **print** | No | No |
| **service** | Yes | Yes (service state) |
| **shell** | Depends | No |
| **template** | Depends | Yes (content comparison) |
| **unarchive** | Depends | Yes (creates marker) |
| **vars** | No | No |

**Legend:**
- **Requires Sudo**: Whether the action typically needs elevated privileges
- **Implements Check**: Whether the action verifies current state before making changes (idempotency)
- **Depends**: Capability depends on the specific operation (e.g., writing to `/etc` requires sudo, writing to `~` doesn't)

## Platform Detection

### At Plan Time

Platform validation occurs during the planning phase:

```yaml
# This will fail at plan-time on FreeBSD
steps:
  - name: "Start nginx"
    service:
      name: nginx
      state: started
```

**Error message:**
```
Error: platform validation failed for step "Start nginx":
  action 'service' is not supported on platform 'freebsd'
  (supported platforms: [linux darwin windows])
```

### Runtime Detection

Some actions adapt their behavior based on the platform:

- **service**: Uses systemd on Linux, launchd on macOS, Windows Services on Windows
- **shell**: Uses bash/sh on Unix-like systems, pwsh/cmd on Windows
- **package**: Auto-detects package manager (apt, dnf, yum, pacman, brew, choco, etc.)

## Platform-Specific Configurations

### Using Facts for Platform Detection

Mooncake provides system facts that can be used for conditional logic:

```yaml
vars:
  package_name: "{{ 'nginx' if os == 'linux' else 'nginx-full' if os == 'darwin' else 'nginx' }}"

steps:
  - name: "Install web server"
    package:
      name: "{{ package_name }}"
      state: present
    when: "os in ['linux', 'darwin']"
```

### Conditionals for Cross-Platform Scripts

```yaml
steps:
  # Linux-specific
  - name: "Configure systemd service"
    service:
      name: myapp
      state: started
    when: "os == 'linux'"

  # macOS-specific
  - name: "Configure launchd service"
    service:
      name: com.example.myapp
      state: started
    when: "os == 'darwin'"

  # Windows-specific
  - name: "Configure Windows service"
    service:
      name: myapp
      state: started
    when: "os == 'windows'"
```

### Multi-Platform Package Installation

```yaml
steps:
  - name: "Install Docker"
    preset:
      name: docker
      with:
        state: present
    # The docker preset handles platform-specific installation
```

## Checking Platform Support

### From CLI

List all actions with their platform support:

```bash
mooncake actions list
```

Output:
```
Action          Category  Platforms               Sudo    Check
assert          system    all                     no      n/a
command         command   all                     depends no
copy            file      all                     depends yes
download        network   all                     depends yes
file            file      all                     depends yes
include_vars    data      all                     no      no
package         system    linux,darwin,windows    yes     yes
preset          system    all                     depends depends
print           output    all                     no      no
service         system    linux,darwin,windows    yes     yes
shell           command   all                     depends no
template        file      all                     depends yes
unarchive       file      all                     depends yes
vars            data      all                     no      no
```

### Programmatic Access

Actions expose their metadata through the registry:

```go
import "github.com/alehatsman/mooncake/internal/actions"

handler, ok := actions.Get("service")
if ok {
    meta := handler.Metadata()
    fmt.Printf("Platforms: %v\n", meta.SupportedPlatforms)
    fmt.Printf("Requires Sudo: %v\n", meta.RequiresSudo)
    fmt.Printf("Implements Check: %v\n", meta.ImplementsCheck)
}
```

## Best Practices

### 1. Use Conditionals for Platform-Specific Steps

Always guard platform-specific actions with `when` conditions:

```yaml
- name: "Install with apt"
  package:
    name: nginx
    manager: apt
  when: "apt_available"  # Use facts
```

### 2. Leverage Presets for Cross-Platform Operations

Presets can encapsulate platform-specific logic:

```yaml
- name: "Install PostgreSQL"
  preset:
    name: postgresql
    with:
      version: "15"
  # Preset handles platform differences internally
```

### 3. Test on Target Platforms

Always test your configurations on the actual target platforms. Plan-time validation catches incompatible actions but doesn't guarantee behavior parity.

### 4. Document Platform Requirements

Include platform requirements in your configuration comments:

```yaml
# Supports: Linux, macOS
# Requirements: systemd (Linux), launchd (macOS)
steps:
  - name: "Start application service"
    service:
      name: myapp
      state: started
```

## Limitations

### Platform-Specific Behavior

Some actions have platform-specific behavior that may not be identical:

- **file**: Permission semantics differ (Unix vs Windows ACLs)
- **shell**: Command availability varies by platform
- **service**: Management interfaces differ (systemd, launchd, Windows Services)

### Package Managers

The **package** action supports multiple package managers but requires them to be installed:

- Linux: apt, dnf, yum, pacman, zypper, apk
- macOS: brew, port
- Windows: choco, scoop
- FreeBSD: pkg

### Service Management

The **service** action requires platform-specific service managers:

- Linux: systemd
- macOS: launchd
- Windows: Windows Service Control Manager

## Future Platform Support

Planned platform expansions:

- **AIX**: Full support for service and package actions
- **Solaris**: Full support for service and package actions
- **BSD variants**: Enhanced support for OpenBSD, NetBSD, DragonFly BSD

## Getting Help

If you encounter platform-specific issues:

1. Check this documentation for known limitations
2. Review the action's documentation in `/docs/guide/config/actions.md`
3. Report platform compatibility issues at https://github.com/anthropics/mooncake/issues
4. Use `mooncake actions list` to verify platform support

## See Also

- [Actions Reference](./config/actions.md) - Detailed action documentation
- [Facts System](./facts.md) - Using system facts for platform detection
- [Presets Guide](./presets.md) - Cross-platform presets
- [Conditionals](./conditionals.md) - Using `when` conditions for platform-specific logic
