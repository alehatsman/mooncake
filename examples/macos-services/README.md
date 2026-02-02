# macOS Service Management with Launchd

This guide demonstrates how to use Mooncake to manage macOS services using launchd.

## Table of Contents

1. [Quick Start](#quick-start)
2. [Service Types](#service-types)
3. [Complete Examples](#complete-examples)
4. [Common Patterns](#common-patterns)
5. [Plist Properties](#plist-properties)
6. [Troubleshooting](#troubleshooting)

---

## Quick Start

### Simple User Agent

```yaml
- name: Start my application
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
          <key>RunAtLoad</key>
          <true/>
        </dict>
        </plist>
```

### Using Templates

```yaml
- name: Deploy service from template
  service:
    name: com.example.{{ app_name }}
    state: started
    enabled: true
    unit:
      src_template: templates/service.plist.j2
```

---

## Service Types

### User Agents

User agents run in the user's context (no sudo required):
- **Path**: `~/Library/LaunchAgents/`
- **Domain**: `gui/<uid>`
- **Permissions**: Current user
- **When**: When user logs in

```yaml
- name: User agent
  service:
    name: com.example.myapp
    state: started
    enabled: true
    unit:
      content: |
        <!-- plist content here -->
```

### System Daemons

System daemons run as root (require sudo):
- **Path**: `/Library/LaunchDaemons/`
- **Domain**: `system`
- **Permissions**: root (requires `become: true`)
- **When**: At system boot

```yaml
- name: System daemon
  service:
    name: com.example.daemon
    state: started
    enabled: true
    unit:
      dest: /Library/LaunchDaemons/com.example.daemon.plist
      content: |
        <!-- plist content here -->
  become: true
```

---

## Complete Examples

### 1. Node.js Web Server

See: [`macos-nodejs-app.yml`](./macos-nodejs-app.yml)

Complete example showing:
- Directory setup
- Dependency installation
- Service configuration
- Logging
- Environment variables
- Health checks

Run with:
```bash
mooncake run examples/macos-services/macos-nodejs-app.yml
```

### 2. Service Management Operations

See: [`macos-service-management.yml`](./macos-service-management.yml)

Examples of:
- Starting/stopping services
- Restarting services
- Updating configuration
- Enabling/disabling
- Dry-run mode

Run with:
```bash
mooncake run examples/macos-services/macos-service-management.yml
```

### 3. Various Service Types

See: [`macos-launchd-service.yml`](./macos-launchd-service.yml)

Demonstrates:
- User agents
- System daemons
- Scheduled tasks
- Resource limits
- Keep-alive configuration

---

## Common Patterns

### Auto-Restart on Crash

```xml
<key>KeepAlive</key>
<dict>
  <key>SuccessfulExit</key>
  <false/>
  <key>Crashed</key>
  <true/>
</dict>
```

### Scheduled Task (Cron-like)

```xml
<!-- Run every hour -->
<key>StartCalendarInterval</key>
<dict>
  <key>Minute</key>
  <integer>0</integer>
</dict>
```

```xml
<!-- Run every day at 2:30 AM -->
<key>StartCalendarInterval</key>
<dict>
  <key>Hour</key>
  <integer>2</integer>
  <key>Minute</key>
  <integer>30</integer>
</dict>
```

### Environment Variables

```xml
<key>EnvironmentVariables</key>
<dict>
  <key>PORT</key>
  <string>8080</string>
  <key>NODE_ENV</key>
  <string>production</string>
</dict>
```

### Logging

```xml
<key>StandardOutPath</key>
<string>/var/log/myapp/stdout.log</string>
<key>StandardErrorPath</key>
<string>/var/log/myapp/stderr.log</string>
```

### Prevent Rapid Restarts

```xml
<!-- Wait 10 seconds before restarting -->
<key>ThrottleInterval</key>
<integer>10</integer>
```

---

## Plist Properties

### Essential Properties

| Key | Type | Description |
|-----|------|-------------|
| `Label` | String | Service identifier (required) |
| `ProgramArguments` | Array | Command and arguments to run (required) |

### Execution Control

| Key | Type | Description |
|-----|------|-------------|
| `RunAtLoad` | Boolean | Start when loaded |
| `KeepAlive` | Boolean/Dict | Auto-restart configuration |
| `StartCalendarInterval` | Dict | Schedule (cron-like) |
| `StartInterval` | Integer | Run every N seconds |

### Process Management

| Key | Type | Description |
|-----|------|-------------|
| `WorkingDirectory` | String | Working directory |
| `EnvironmentVariables` | Dict | Environment variables |
| `UserName` | String | Run as specific user |
| `GroupName` | String | Run as specific group |

### Logging

| Key | Type | Description |
|-----|------|-------------|
| `StandardOutPath` | String | Stdout log file |
| `StandardErrorPath` | String | Stderr log file |

### Resource Limits

| Key | Type | Description |
|-----|------|-------------|
| `SoftResourceLimits` | Dict | Soft resource limits |
| `HardResourceLimits` | Dict | Hard resource limits |
| `Nice` | Integer | Process priority (-20 to 20) |

### Network

| Key | Type | Description |
|-----|------|-------------|
| `Sockets` | Dict | Socket activation |

---

## Service States

### Available States

| State | Description | Action |
|-------|-------------|--------|
| `started` | Start the service | `launchctl bootstrap` (if not loaded)<br>`launchctl kickstart` (if loaded) |
| `stopped` | Stop the service | `launchctl kill SIGTERM` |
| `restarted` | Restart the service | `launchctl kickstart -k` |
| `reloaded` | Reload configuration | Same as `restarted` |

### Enabled Status

| Status | Description | Action |
|--------|-------------|--------|
| `enabled: true` | Load service (persistent) | `launchctl bootstrap` |
| `enabled: false` | Unload service | `launchctl bootout` |

---

## Idempotency

Mooncake automatically ensures idempotent operations:

1. **Plist Updates**: Only writes if content changed
2. **Service State**: Checks current state before changing
3. **Load Status**: Only loads/unloads if needed

Example:
```yaml
# First run: Creates plist, loads service, starts it
# Second run: No changes (plist unchanged, service already running)
- name: Deploy service
  service:
    name: com.example.app
    state: started
    enabled: true
    unit:
      content: |
        <!-- plist content -->
```

---

## Troubleshooting

### Check Service Status

```bash
# List all loaded services
launchctl list

# Check specific service
launchctl list | grep com.example.myapp

# Print service details
launchctl print gui/$(id -u)/com.example.myapp
```

### View Logs

```bash
# If using StandardOutPath/StandardErrorPath
tail -f /path/to/stdout.log
tail -f /path/to/stderr.log

# System logs
log stream --predicate 'processImagePath contains "myapp"' --info
```

### Unload Service Manually

```bash
# User agent
launchctl bootout gui/$(id -u)/com.example.myapp

# System daemon
sudo launchctl bootout system/com.example.daemon
```

### Load Service Manually

```bash
# User agent
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.example.myapp.plist

# System daemon
sudo launchctl bootstrap system /Library/LaunchDaemons/com.example.daemon.plist
```

### Common Issues

**Issue**: Service not starting
- Check plist syntax: `plutil -lint ~/Library/LaunchAgents/com.example.myapp.plist`
- Check logs: `tail -f /path/to/error.log`
- Verify program path exists

**Issue**: Permission denied
- User agents: Don't use `become: true`
- System daemons: Must use `become: true`

**Issue**: Service keeps restarting
- Check exit code: `launchctl print gui/$(id -u)/com.example.myapp`
- Review logs for errors
- Add `ThrottleInterval` to prevent rapid restarts

---

## Dry-Run Mode

Preview changes without applying them:

```bash
mooncake run --dry-run examples/macos-services/macos-launchd-service.yml
```

Output shows:
- What plist files would be created/updated
- What services would be started/stopped
- What operations would be performed

---

## Template Variables

Use variables for flexibility:

```yaml
vars:
  app_name: myapp
  app_path: /usr/local/bin/myapp
  port: 8080
  log_dir: /var/log/myapp

steps:
  - name: Deploy {{ app_name }}
    service:
      name: com.example.{{ app_name }}
      unit:
        content: |
          <?xml version="1.0" encoding="UTF-8"?>
          <!-- ... -->
          <key>ProgramArguments</key>
          <array>
            <string>{{ app_path }}</string>
          </array>
          <key>EnvironmentVariables</key>
          <dict>
            <key>PORT</key>
            <string>{{ port }}</string>
          </dict>
```

---

## References

- [launchd.info](http://www.launchd.info/) - Comprehensive launchd documentation
- [Apple Developer Documentation](https://developer.apple.com/library/archive/documentation/MacOSX/Conceptual/BPSystemStartup/Chapters/CreatingLaunchdJobs.html)
- [launchctl man page](https://ss64.com/osx/launchctl.html)
- [plist man page](https://www.manpagez.com/man/5/plist/)

---

## Testing

All launchd functionality is tested:
- ✅ Plist creation (inline and template)
- ✅ Service state management
- ✅ Load/unload operations
- ✅ Idempotency checks
- ✅ Platform detection
- ✅ Dry-run mode

Tests automatically skip on non-macOS platforms.

Run tests:
```bash
go test ./internal/executor -run "Launchd|Service"
```
