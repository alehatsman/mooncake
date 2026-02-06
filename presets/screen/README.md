# screen - Terminal Multiplexer

Classic terminal multiplexer. Run multiple shell sessions in one terminal, detach and reattach, survive disconnections.

## Quick Start
```yaml
- preset: screen
```

## Basic Usage
```bash
# Start new session
screen

# Start named session
screen -S mysession

# List sessions
screen -ls

# Reattach to session
screen -r
screen -r mysession

# Detach from session
Ctrl+A D
```

## Key Bindings

All commands start with **Ctrl+A** (prefix key):

### Windows
```bash
Ctrl+A C     # Create new window
Ctrl+A N     # Next window
Ctrl+A P     # Previous window
Ctrl+A 0-9   # Switch to window 0-9
Ctrl+A "     # List all windows
Ctrl+A A     # Rename current window
Ctrl+A K     # Kill current window
```

### Splitting (vertical splits require screen 4.1+)
```bash
Ctrl+A S     # Split horizontally
Ctrl+A |     # Split vertically (4.1+)
Ctrl+A Tab   # Switch between splits
Ctrl+A X     # Close current split
Ctrl+A Q     # Close all splits except current
```

### Sessions
```bash
Ctrl+A D     # Detach session
Ctrl+A [     # Enter copy mode (scroll back)
Ctrl+A ]     # Paste buffer
Ctrl+A ?     # Show help
Ctrl+A :     # Enter command mode
```

### Copy Mode
```bash
Ctrl+A [     # Enter copy mode
Space        # Start selection
Enter        # Copy selection
Ctrl+A ]     # Paste
```

## Session Management
```bash
# Start session
screen -S development

# Detach (keeps running)
Ctrl+A D

# List all sessions
screen -ls

# Reattach to session
screen -r development

# Reattach even if attached elsewhere
screen -x development

# Force detach and reattach
screen -d -r development

# Kill session
screen -S development -X quit
```

## Configuration

### ~/.screenrc
```bash
# Scrollback buffer
defscrollback 10000

# Turn off startup message
startup_message off

# Use 256 colors
term screen-256color

# Fix for residual editor text
altscreen on

# Status bar
hardstatus alwayslastline
hardstatus string '%{= kG}[ %{G}%H %{g}][%= %{= kw}%?%-Lw%?%{r}(%{W}%n*%f%t%?(%u)%?%{r})%{w}%?%+Lw%?%?%= %{g}][%{B} %Y-%m-%d %{W}%c %{g}]'

# Mouse support (scroll)
termcapinfo xterm* ti@:te@

# Caption for window list
caption always "%{= kw}%-w%{= BW}%n %t%{-}%+w %-= @%H - %LD %d %LM - %c"

# Bind F1 and F2 for switching
bindkey -k k1 prev
bindkey -k k2 next

# Split navigation
bind j focus down
bind k focus up
bind h focus left
bind l focus right
```

## Common Workflows

### Long-Running Processes
```bash
# Start screen
screen -S build

# Run long process
./build.sh
make install

# Detach
Ctrl+A D

# Continue later
screen -r build
```

### Remote Server Work
```bash
# SSH to server
ssh user@server

# Start screen
screen -S work

# Do work...

# Detach
Ctrl+A D

# Logout (session keeps running)
exit

# Later: Reconnect
ssh user@server
screen -r work
```

### Multiple Windows
```bash
# Start screen
screen -S dev

# Create windows
Ctrl+A C    # editor
Ctrl+A C    # server
Ctrl+A C    # database
Ctrl+A C    # logs

# Rename windows
Ctrl+A A
# Type name: "editor"

# Switch between
Ctrl+A 0    # First window
Ctrl+A 1    # Second window
Ctrl+A N    # Next
Ctrl+A P    # Previous
```

## Advanced Features

### Logging
```bash
# Enable logging for current window
Ctrl+A H

# Log all output to file
# Creates screenlog.0 in current directory

# Log commands in .screenrc
logfile /tmp/screenlog-%t.txt
deflog on
```

### Monitoring
```bash
# Monitor window for activity
Ctrl+A M

# Gets notification when output appears
# Useful for long-running commands

# Monitor for silence (30 seconds)
Ctrl+A _
```

### Multiuser Sessions
```bash
# Enable multiuser mode
screen -S shared
Ctrl+A :multiuser on
Ctrl+A :acladd otheruser

# Other user connects
screen -x youruser/shared

# Grant write access
Ctrl+A :aclchg otheruser +w "#"
```

### Screen in Screen
```bash
# Outer screen: Ctrl+A
# Inner screen: Ctrl+A A

# Send command to inner screen
Ctrl+A A D    # Detach inner screen
```

## Command Line Options
```bash
# Start with command
screen -S build make all

# Start detached
screen -dmS background ./script.sh

# List sessions
screen -ls
screen -list

# Resume any session
screen -R

# Create or reattach
screen -D -RR mysession

# Force new session if exists
screen -D -m -S newsession

# Wipe dead sessions
screen -wipe
```

## Scripting with Screen
```bash
# Send commands to screen session
screen -S mysession -X stuff "ls\n"

# Create window with command
screen -S mysession -X screen -t "logs" tail -f /var/log/app.log

# Kill all windows
screen -S mysession -X quit

# Script example
#!/bin/bash
screen -dmS build
screen -S build -X screen -t "compile" make
screen -S build -X screen -t "test" make test
screen -S build -X screen -t "logs" tail -f build.log
```

## Comparison with tmux
| Feature | screen | tmux |
|---------|--------|------|
| Age | 1987 | 2007 |
| Config | ~/.screenrc | ~/.tmux.conf |
| Prefix | Ctrl+A | Ctrl+B |
| Vertical split | 4.1+ | Yes |
| Plugins | No | Yes (TPM) |
| Scripting | Basic | Advanced |
| Status bar | Basic | Advanced |
| Mouse | Limited | Full |
| Popularity | Declining | Growing |

## Troubleshooting
```bash
# Permission denied
chmod 700 /var/run/screen
sudo /etc/init.d/screen-cleanup start

# Can't reattach (attached elsewhere)
screen -d -r mysession

# Screen freezes (Ctrl+S was pressed)
Ctrl+Q    # Resume

# List all sessions including dead
screen -ls
screen -wipe    # Clean up dead sessions

# Terminal messed up after exit
reset    # Reset terminal
```

## Use Cases

### Development
```bash
# Window 1: Editor
vim project.c

# Window 2: Compiler
make watch

# Window 3: Tests
npm test --watch

# Window 4: Server
./run-server.sh
```

### Server Administration
```bash
# Window 1: Logs
tail -f /var/log/syslog

# Window 2: Monitoring
htop

# Window 3: Shell
# Work here

# Window 4: Database
mysql -u root
```

### Training/Demos
```bash
# Share session with trainee
screen -S training
Ctrl+A :multiuser on
Ctrl+A :acladd trainee

# Both can type and see output
```

## CI/CD Integration
```bash
# GitHub Actions
- name: Run in screen
  run: |
    screen -dmS ci
    screen -S ci -X stuff "make test\n"
    screen -S ci -X stuff "exit\n"

# Long-running builds
#!/bin/bash
screen -dmS build make all
while screen -list | grep -q build; do
  sleep 10
done
screen -S build -X hardcopy build.log
```

## Tips & Tricks
```bash
# Quick detach and reattach
Ctrl+A D D    # Detach
screen -r     # Reattach

# Copy between windows
# Window 1:
echo "text" > /tmp/buffer
# Window 2:
cat /tmp/buffer

# Hardcopy (screenshot) current window
Ctrl+A H    # Saves to hardcopy.0

# Lock screen
Ctrl+A X    # Requires password to unlock

# Zombie windows (command finished)
Ctrl+A :zombie xy
# x = key to resurrect, y = key to kill
```

## Best Practices
- **Name your sessions** for easy identification
- **Use meaningful window names** (Ctrl+A A)
- **Keep .screenrc simple** to avoid issues
- **Detach before closing terminal** to preserve sessions
- **Use logging** for important sessions
- **Clean up dead sessions** regularly (screen -wipe)
- **Consider tmux** for new projects (more features)

## Migration to tmux
```bash
# Screen habits → tmux equivalents
Ctrl+A C  → Ctrl+B C    # New window
Ctrl+A N  → Ctrl+B N    # Next window
Ctrl+A D  → Ctrl+B D    # Detach
Ctrl+A [  → Ctrl+B [    # Copy mode
Ctrl+A "  → Ctrl+B W    # List windows

# Or change tmux prefix to Ctrl+A
echo "set -g prefix C-a" >> ~/.tmux.conf
```

## Agent Use
- Long-running automated tasks
- Remote server management
- Build and deployment automation
- Multi-window monitoring setups
- Training and demonstration sessions
- Session persistence across disconnections

## Uninstall
```yaml
- preset: screen
  with:
    state: absent
```

## Resources
- Manual: `man screen`
- Help: `Ctrl+A ?` (in screen)
- Wiki: https://www.gnu.org/software/screen/
- Search: "screen tutorial", "screen vs tmux"
