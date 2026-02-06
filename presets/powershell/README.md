# PowerShell - Cross-Platform Automation Shell

Task automation and configuration management framework with a command-line shell, scripting language, and .NET integration.

## Quick Start

```yaml
- preset: powershell
```

## Features

- **Cross-Platform**: Runs on Windows, Linux, and macOS
- **Object-Based**: Work with .NET objects, not just text
- **Cmdlet Ecosystem**: 1000+ built-in commands with consistent syntax
- **Pipeline**: Pass complex objects between commands
- **Scripting**: Full-featured language with functions, classes, modules
- **Remote Management**: SSH and WinRM remoting built-in

## Basic Usage

```bash
# Check version
pwsh --version

# Start interactive shell
pwsh

# Run command
pwsh -Command "Get-Process"

# Run script
pwsh -File script.ps1

# Run as administrator (Linux/macOS)
sudo pwsh
```

## Advanced Configuration

```yaml
# Basic installation
- preset: powershell
  with:
    state: present

# Uninstall
- preset: powershell
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support

- ✅ Linux (apt, dnf, yum, zypper, snap)
- ✅ macOS (Homebrew, pkg)
- ✅ Windows (MSI installer)

## Configuration

- **Config directory**:
  - Linux: `~/.config/powershell/`
  - macOS: `~/.config/powershell/`
  - Windows: `$HOME\Documents\PowerShell\`
- **Profile script**:
  - Linux/macOS: `~/.config/powershell/Microsoft.PowerShell_profile.ps1`
  - Windows: `$HOME\Documents\PowerShell\Microsoft.PowerShell_profile.ps1`
- **Module path**: `~/.local/share/powershell/Modules/`

## Real-World Examples

### System Administration

```powershell
# Get system information
Get-ComputerInfo

# List running processes
Get-Process | Sort-Object -Property CPU -Descending | Select-Object -First 10

# Monitor disk space
Get-PSDrive | Where-Object {$_.Used -gt 0}

# Get network configuration
Get-NetIPConfiguration

# Check open ports
Get-NetTCPConnection | Where-Object {$_.State -eq "Listen"}
```

### File Operations

```powershell
# Find files by pattern
Get-ChildItem -Path /var/log -Recurse -Filter "*.log"

# Get file hashes
Get-FileHash -Path file.txt -Algorithm SHA256

# Search file content
Select-String -Path "*.log" -Pattern "ERROR"

# Copy with progress
Copy-Item -Path source -Destination dest -Recurse -Verbose

# Archive files
Compress-Archive -Path folder/* -DestinationPath backup.zip
```

### Automation Scripts

```yaml
# Deploy with PowerShell script
- name: Install PowerShell
  preset: powershell

- name: Run automation script
  shell: |
    pwsh -File deploy.ps1 -Environment production -Verbose
  register: deploy_result

- name: Check deployment status
  assert:
    command:
      cmd: "[ {{ deploy_result.rc }} -eq 0 ]"
      exit_code: 0
```

## Agent Use

- Automate system configuration across Windows, Linux, and macOS
- Query and manage cloud resources (Azure, AWS via modules)
- Parse structured data (JSON, XML, CSV) with native cmdlets
- Remote system management via SSH or WinRM
- CI/CD pipeline scripting with error handling
- Configuration management with DSC (Desired State Configuration)
- Log analysis and security auditing

## Troubleshooting

### Command not found

Ensure PowerShell is in PATH:
```bash
which pwsh
pwsh --version
```

### Execution policy errors (Windows)

```powershell
# Check current policy
Get-ExecutionPolicy

# Allow scripts (run as administrator)
Set-ExecutionPolicy RemoteSigned -Scope CurrentUser
```

### Module import fails

```powershell
# Check module path
$env:PSModulePath -split ':'

# Install to user scope (no sudo needed)
Install-Module -Name ModuleName -Scope CurrentUser -Force
```

## Uninstall

```yaml
- preset: powershell
  with:
    state: absent
```

## Resources

- Official docs: https://learn.microsoft.com/en-us/powershell/
- GitHub: https://github.com/PowerShell/PowerShell
- Module gallery: https://www.powershellgallery.com/
- Search: "powershell tutorial", "powershell cmdlets", "powershell cross-platform"
