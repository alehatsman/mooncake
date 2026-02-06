# magic-wormhole - Secure Peer-to-Peer File Transfer

Secure, simple file transfer tool that creates one-time codes for transferring files and directories between computers.

## Quick Start
```yaml
- preset: magic-wormhole
```

## Features
- **Zero configuration**: No accounts, no servers to set up
- **Secure**: End-to-end encryption with one-time codes
- **NAT traversal**: Works behind firewalls and NAT
- **Cross-platform**: Linux, macOS, Windows
- **Simple UX**: Just type the code on the receiving end
- **Progress indicators**: Shows transfer speed and ETA

## Basic Usage
```bash
# Send file
wormhole send myfile.txt
# Outputs code like: 7-crossover-clockwork

# Receive file (on another machine)
wormhole receive 7-crossover-clockwork

# Send directory
wormhole send mydirectory/

# Send with custom code
wormhole send --code 5-custom-code myfile.txt

# Send text message
wormhole send --text "Hello, World!"
echo "Secret message" | wormhole send --text

# Receive to specific directory
wormhole receive --output-dir /path/to/dest
```

## Advanced Configuration
```yaml
- preset: magic-wormhole
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove magic-wormhole |

## Platform Support
- ✅ Linux (apt, dnf, pip)
- ✅ macOS (Homebrew, pip)
- ✅ Windows (pip install)

## Configuration
- **No config file**: Works out of the box
- **Relay server**: Default public relay or self-hosted
- **Code length**: Configurable word count

## Real-World Examples

### Deployment Artifact Transfer
```yaml
# On build server
- name: Send build artifact
  shell: wormhole send --code {{ transfer_code }} /path/to/artifact.tar.gz
  async: 600
  register: send_job

# On deployment server
- name: Receive build artifact
  shell: wormhole receive --accept-file --output-dir /tmp {{ transfer_code }}
  register: receive
```

### Secure Config Transfer
```bash
# Send encrypted config
wormhole send production-config.yml
# Code: 3-hamburger-carnival

# Receive on production server
wormhole receive 3-hamburger-carnival
```

### Quick File Sharing Between Developers
```bash
# Developer A sends debug logs
wormhole send debug-logs.tar.gz
# Outputs: 5-gazelle-paperclip

# Developer B receives
wormhole receive 5-gazelle-paperclip
```

### Temporary File Bridge
```bash
# Transfer large file without cloud storage
# Machine A:
wormhole send large-dataset.zip

# Machine B (immediately):
wormhole receive <code-from-A>
```

## Agent Use
- Secure file transfer in CI/CD pipelines
- Temporary data exchange between systems
- Emergency config/log transfer
- Development environment synchronization
- Secure credential transfer (one-time)

## Troubleshooting

### Transfer hangs
Check connectivity:
```bash
# Test relay server
ping -c 3 wormhole.xfer.sh

# Use verbose mode
wormhole --verbose send myfile.txt
```

### NAT/Firewall issues
Try different relay:
```bash
wormhole --relay-url ws://custom-relay.example.com send file.txt
```

### Code expired
Codes expire after use. Generate new code:
```bash
wormhole send myfile.txt
```

### Slow transfer speed
Transfers use direct peer-to-peer connection when possible. Speed depends on:
- Network bandwidth
- NAT traversal success
- Relay server location

## Security Notes
- **One-time codes**: Each code works once, then expires
- **End-to-end encryption**: Files encrypted before leaving sender
- **No storage**: Files never stored on relay server
- **Code length**: Longer codes = more secure
- **Verification**: Codes provide mutual authentication

## Uninstall
```yaml
- preset: magic-wormhole
  with:
    state: absent
```

## Resources
- Official site: https://magic-wormhole.readthedocs.io/
- GitHub: https://github.com/magic-wormhole/magic-wormhole
- Search: "magic wormhole file transfer", "wormhole secure sharing"
