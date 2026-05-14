# croc - Secure File Transfer

Croc is a tool that allows any two computers to simply and securely transfer files and folders directly between them.

## Quick Start
```yaml
- preset: croc
```

## Features
- **Easy to use**: Simple command-line interface
- **Secure**: End-to-end encryption using PAKE
- **Resume transfers**: Continue interrupted transfers
- **Cross-platform**: Works on any platform (Linux, macOS, Windows, BSD)
- **No setup**: No port forwarding or server configuration needed
- **Direct transfer**: P2P via relay servers

## Basic Usage
```bash
# Send file
croc send myfile.txt
# Output: Code is: 1234-code-word

# Receive file (on another machine)
croc 1234-code-word

# Send multiple files
croc send file1.txt file2.txt folder/

# Send with custom code
croc send --code my-secret-code document.pdf

# Use local relay (same network)
croc send --relay local myfile.txt

# Custom relay server
croc send --relay myrelay.example.com:9009 file.txt
```

## Advanced Configuration
```yaml
- preset: croc
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove croc |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, snap, binary download)
- ✅ macOS (Homebrew, binary download)
- ❌ Windows (not yet supported by preset, but binary available)

## Configuration
- **Default relay**: croc.schollz.com:9009
- **Config file**: None required
- **Transfer port**: Dynamic (NAT traversal)
- **Data**: Encrypted end-to-end with PAKE

## Real-World Examples

### Send Directory
```bash
# Send entire directory
croc send /path/to/directory/

# Receive (preserves structure)
croc code-phrase
```

### CI/CD Artifact Transfer
```bash
# In CI pipeline, send build artifact
croc send --code $CI_JOB_ID build/app.tar.gz

# On deployment server
croc $CI_JOB_ID
```

### Custom Relay Server
```bash
# Run your own relay (on server)
croc relay

# Send using custom relay
croc send --relay your-server.com:9009 file.zip

# Receive using custom relay
croc --relay your-server.com:9009 code-word
```

### Transfer with Progress
```bash
# Send (shows progress bar)
croc send large-file.iso

# Output shows:
# Sending 'large-file.iso' (4.7 GB)
# Code is: 7421-alpha-beta
# [=================>        ] 65% | 3.1 GB/4.7 GB | 45 MB/s | 0:00:36
```

### Secure Server-to-Server Transfer
```bash
# Server A (sender)
croc send --code production-deploy-2024 /var/www/app.tar.gz

# Server B (receiver)
croc production-deploy-2024
tar -xzf app.tar.gz -C /var/www/
```

### Compress Before Sending
```bash
# Croc handles compression automatically for folders
# For manual control:
tar -czf archive.tar.gz /large/folder
croc send archive.tar.gz
```

## Options
```bash
# Send options
croc send [options] [file/folder...]
  --code CODE         Use specific code word
  --relay RELAY       Use custom relay server
  --ports PORTS       Ports to use (comma-separated)
  --no-compress       Disable compression
  --overwrite         Overwrite existing files
  --yes               Accept all prompts

# Receive options
croc [options] CODE
  --yes               Accept transfer automatically
  --out FOLDER        Save to specific folder
  --overwrite         Overwrite without asking
```

## Agent Use
- Automated file transfers between machines
- CI/CD artifact distribution
- Backup transfers to remote locations
- Log file collection from servers
- Configuration file deployment
- Emergency data recovery transfers

## Troubleshooting

### Transfer stuck or slow
Try different relay or direct mode:
```bash
# Use different relay
croc send --relay croc3.schollz.com:9009 file.txt

# Local network (faster)
croc send --relay local file.txt
```

### Connection refused
Check firewall and network:
```bash
# Test relay connectivity
nc -zv croc.schollz.com 9009

# Use custom ports
croc send --ports 9010,9011,9012 file.txt
```

### Code doesn't work
Verify code phrase carefully:
```bash
# Codes are case-sensitive and time-limited
# Make sure both sender and receiver use exact code
# Codes expire after 10 minutes of inactivity
```

## Uninstall
```yaml
- preset: croc
  with:
    state: absent
```

## Resources
- Official site: https://schollz.com/croc
- GitHub: https://github.com/schollz/croc
- Documentation: https://github.com/schollz/croc/blob/master/README.md
- Search: "croc file transfer", "croc secure send"
