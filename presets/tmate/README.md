# tmate - Instant Terminal Sharing

Share your terminal session instantly with others. Fork of tmux designed for terminal sharing and pair programming.

## Quick Start
```yaml
- preset: tmate
```

## Features
- **Instant Sharing**: Generate shareable SSH/web links
- **No Configuration**: Works out of the box
- **Secure**: End-to-end encryption
- **Read-only Mode**: Share without giving control
- **tmux Compatible**: Same commands as tmux
- **Cross-platform**: Linux and macOS support

## Basic Usage
```bash
# Start tmate session
tmate

# Session URL appears automatically:
# SSH: ssh session@tmate.io
# Web: https://tmate.io/t/session-id

# Get session URLs
tmate show-messages

# Read-only URLs
tmate show-messages | grep "read only"

# Detach from session
Ctrl-b d
```

## Session Sharing

### Pair Programming
```bash
# Host starts session
tmate

# Get SSH connection string
tmate show-messages | grep "ssh session"

# Output:
# Colleague can connect with: ssh xyz123@tmate.io

# Coworker connects
ssh xyz123@tmate.io

# Both can type and see changes in real-time
```

### Read-Only Sharing
```bash
# Start tmate
tmate

# Get read-only connection
tmate show-messages | grep "read only"

# Output:
# Share read-only: ssh ro-xyz123@tmate.io

# Viewer connects (can see, cannot type)
ssh ro-xyz123@tmate.io
```

### Web Sharing
```bash
# Get web URL
tmate show-messages | grep "web session"

# Output:
# Web session: https://tmate.io/t/xyz123

# Share URL - viewers can watch in browser
```

## Real-World Examples

### Remote Debugging
```bash
# Developer starts debug session
tmate

# Get connection strings
tmate show-messages

# Share with colleague
echo "Debug session: ssh xyz123@tmate.io"

# Both can investigate issue together
```

### Live Demo/Tutorial
```bash
# Instructor starts session
tmate

# Get read-only web URL
WEB_URL=$(tmate show-messages | grep "web session" | awk '{print $NF}')

# Share with students
echo "Watch live demo: $WEB_URL"

# Students watch in browser, instructor has full control
```

### CI/CD Debug
```bash
# In failing CI job
if [ "$CI_DEBUG" = "true" ]; then
  tmate -F  # Foreground mode
  # Session URL printed to logs
  # Developer can connect and investigate
fi
```

### Support Session
```bash
# Support agent starts session
tmate

# Send connection to customer
echo "Please connect: ssh xyz123@tmate.io"

# Walk customer through solution
# Or take control to fix issue
```

## Advanced Options
```bash
# Custom socket
tmate -S /tmp/tmate.sock

# Foreground mode (no fork)
tmate -F

# Custom server
tmate -s tmate-server.company.com

# With specific session name
tmate new-session -s debug

# Attach to existing session
tmate attach -t debug
```

## tmux Compatibility
```bash
# All tmux commands work
tmate split-window -h
tmate split-window -v
tmate new-window
tmate select-window -t 1

# Key bindings are the same
Ctrl-b c  # New window
Ctrl-b %  # Split horizontal
Ctrl-b "  # Split vertical
Ctrl-b d  # Detach
```

## Configuration

### ~/.tmate.conf
```bash
# Use custom server
set -g tmate-server-host tmate.company.com
set -g tmate-server-port 22

# Custom status bar
set -g status-right "tmate session"

# Set prefix key (like tmux)
set -g prefix C-a
unbind C-b
```

### Self-Hosted Server
```bash
# Install tmate server
# (requires separate setup)

# Point client to custom server
echo "set -g tmate-server-host tmate.company.com" >> ~/.tmate.conf
```

## Scripting Examples

### Automated Support Script
```bash
#!/bin/bash
# support-session.sh

# Start tmate in background
tmate -F > /tmp/tmate.log 2>&1 &
TMATE_PID=$!

# Wait for session URL
sleep 3

# Extract URLs
SSH_URL=$(tmate show-messages | grep "ssh session" | awk '{print $NF}')
WEB_URL=$(tmate show-messages | grep "web session" | awk '{print $NF}')

# Send to support system
echo "Support session created:"
echo "  SSH: $SSH_URL"
echo "  Web: $WEB_URL"

# Keep session alive
wait $TMATE_PID
```

### Temporary Share Script
```bash
#!/bin/bash
# quick-share.sh

# Start tmate
tmate -F &
sleep 2

# Show connection info
echo "=== Share this terminal ==="
tmate show-messages
echo "==========================="

# Wait for Enter to close
read -p "Press Enter to end session..."
pkill tmate
```

## Security Notes
- End-to-end encryption
- Session IDs are random and secure
- Sessions expire after disconnection
- Use read-only mode for public sharing
- Self-host server for sensitive work
- Close sessions when done

## Advanced Configuration
```yaml
- preset: tmate
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove tmate |

## Platform Support
- ✅ Linux (package managers, source)
- ✅ macOS (Homebrew, source)
- ❌ Windows (use WSL)

## Comparison with tmux
| Feature | tmux | tmate |
|---------|------|-------|
| Local sessions | ✅ | ✅ |
| Remote sharing | ❌ | ✅ |
| Configuration | Complex | Simple |
| Setup required | Yes | No |
| Web access | ❌ | ✅ |
| Security | Local only | Encrypted |

## Troubleshooting

### Cannot connect to session
```bash
# Check tmate is running
ps aux | grep tmate

# Verify session exists
tmate show-messages

# Try new session
tmate kill-server
tmate
```

### Slow connection
```bash
# Use custom tmate server
echo "set -g tmate-server-host tmate.company.com" >> ~/.tmate.conf

# Or self-host
# See: https://github.com/tmate-io/tmate-ssh-server
```

## Agent Use
- Remote pair programming sessions
- Live debugging with team members
- Customer support with terminal access
- Code review and collaboration
- Teaching and demonstrations
- CI/CD pipeline debugging

## Uninstall
```yaml
- preset: tmate
  with:
    state: absent
```

## Resources
- Official site: https://tmate.io/
- GitHub: https://github.com/tmate-io/tmate
- Search: "tmate pair programming", "tmate vs tmux", "tmate self-hosted"
