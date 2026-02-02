# Changelog

## v0.3 - Current

### Features
- Animated TUI with progress tracking
- Dry-run mode for safe preview
- System information (explain command)
- Improved error messages
- Service management (systemd on Linux, launchd on macOS)
  - Full lifecycle control (start, stop, restart, reload)
  - Enable/disable services on boot
  - Unit/plist file management with templates
  - Drop-in configuration files (systemd)
  - Idempotent operations with change detection

## v0.2

### Features
- Loop iteration (with_items, with_filetree)
- Tag filtering
- Register for capturing output

## v0.1 - Initial Release

### Features
- Shell command execution
- File and directory operations
- Template rendering (pongo2)
- Variables and system facts
- Conditionals (when)
- Include files and variables
- Sudo/privilege escalation
