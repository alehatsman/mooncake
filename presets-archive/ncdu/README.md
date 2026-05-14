# ncdu - Fast Interactive Disk Usage Analyzer

ncdu is a fast, interactive disk usage analyzer that helps you find large files and directories on your system. With an ncurses interface, you can drill down through directory trees and quickly identify storage bottlenecks.

## Quick Start

```yaml
- preset: ncdu
```

## Features

- **Interactive Navigation**: Explore directory trees with keyboard controls (arrow keys, enter, delete)
- **Fast Scanning**: Written in C for speed on large filesystems
- **Live Drill-Down**: Instantly navigate from summary to specific files
- **Cross-Platform**: Works on Linux, macOS, and BSD
- **Minimal Dependencies**: Standalone binary with no heavy requirements
- **Disk Visualization**: Clear directory sizes with percentage bars

## Basic Usage

```bash
# Analyze current directory
ncdu

# Analyze specific directory
ncdu /var

# Analyze system root (usually requires sudo)
ncdu /

# Show hidden files
ncdu -H

# Export scan results
ncdu -o- > scan.json

# Load previous scan
ncdu -f scan.json
```

## Advanced Configuration

```yaml
# Basic installation with defaults
- preset: ncdu

# Install and verify
- name: Ensure ncdu is ready
  preset: ncdu
  with:
    state: present

# Uninstall ncdu
- preset: ncdu
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove ncdu |

## Platform Support

- ✅ Linux (apt, dnf, yum, pacman, zypper)
- ✅ macOS (Homebrew)
- ✅ BSD (ports)

## Configuration

- **Binary location**: `/usr/bin/ncdu` (Linux), `/usr/local/bin/ncdu` (macOS)
- **No configuration files**: ncdu is self-contained with no config directory
- **Data files**: Scan results saved as JSON when using `-o` flag

## Real-World Examples

### Find Large Directories Before Cleanup

```bash
# Analyze /home to find large user directories
ncdu /home

# Use 'q' to quit, 'd' to delete files in ncdu UI
# Or export for analysis:
ncdu -o- /home | jq '.[].dsize | max'
```

### Monitor Disk Usage in CI/CD Pipeline

```bash
# Check if any directory exceeds 50GB threshold
size_mb=$(ncdu -0 /var/log | tail -c +12 | sort -rn | head -1 | awk '{print int($1/1024/1024)}')
if [ "$size_mb" -gt 51200 ]; then
  echo "ERROR: /var/log exceeds 50GB: ${size_mb}MB"
  exit 1
fi
```

### Storage Audit Report

```bash
# Export directory structure and sizes for reporting
ncdu -0 -o- /opt/app | \
  jq -r '.[] | select(.dsize > 1048576) | "\(.name): \(.dsize / 1024 / 1024 | floor)MB"' \
  > disk-report.txt
```

## Agent Use

- Automated disk space audits for infrastructure
- Find storage bottlenecks before they cause outages
- Generate disk usage reports across multiple systems
- Validate disk cleanup procedures with pre/post measurements
- Monitor application log directory growth
- Identify stale file caches and temporary directories
- CI/CD pipeline disk space assertions

## Troubleshooting

### Permission Denied

If you get permission errors when analyzing certain directories:

```bash
# Run with elevated privileges
sudo ncdu /

# Analyze only accessible directories
ncdu /home/username
```

### Out of Memory on Large Filesystems

For very large directory trees (10M+ files), ncdu may use significant memory. Export to disk first:

```bash
# Use minimal memory mode
ncdu -e /very/large/path

# Or let it run in background
nice ncdu -o- / > full-scan.json &
```

### Terminal Display Issues

If the ncurses interface appears corrupted:

```bash
# Reset terminal
reset

# Check terminal support
echo $TERM  # Should be "xterm", "screen", etc.
```

## Uninstall

```yaml
- preset: ncdu
  with:
    state: absent
```

## Resources

- Official documentation: https://dev.yorhel.nl/ncdu
- GitHub repository: https://github.com/rofl0r/ncdu
- Man page: `man ncdu` (after installation)
- Search: "ncdu tutorial", "ncdu disk analysis", "ncdu export json"
