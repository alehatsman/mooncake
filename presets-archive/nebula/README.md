# nebula - Overlay Networking for Scalable Private Networks

nebula is a scalable overlay network tool created by Slack. It enables you to create a fast, secure, private network that works across datacenters and clouds, regardless of network topology. Unlike traditional VPNs, nebula uses full-mesh networking for better performance and resilience.

## Quick Start

```yaml
- preset: nebula
```

## Features

- **Full-Mesh Network**: Direct peer-to-peer connections eliminate single points of failure
- **Scalable**: Tested with thousands of nodes across global infrastructure
- **Fast**: Written in Go, optimized for throughput and low latency
- **Portable**: Works on Linux, macOS, Windows, and FreeBSD
- **Secure**: TLS 1.3 encryption with certificate-based authentication
- **Cross-Cloud**: Creates unified networks across AWS, GCP, Azure, and private datacenters
- **Lighthouse Support**: Optional central coordination nodes for NAT traversal

## Basic Usage

```bash
# Show version
nebula -version

# Check binary location
which nebula

# Run nebula service (after configuration)
sudo nebula -config /etc/nebula/config.yml

# Verify network connectivity
ping <nebula-ip-of-peer>

# Check service status
sudo systemctl status nebula
```

## Advanced Configuration

```yaml
# Install nebula CLI tool
- preset: nebula

# Configure and start as service (requires manual config file setup)
- name: Install nebula networking
  preset: nebula
  with:
    state: present

# Complete uninstall
- preset: nebula
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove nebula |

## Platform Support

- ✅ Linux (via GitHub releases, package managers)
- ✅ macOS (Homebrew)
- ✅ Windows (binary releases)
- ✅ FreeBSD (ports)

## Configuration

- **Binary location**: `/usr/bin/nebula` (Linux), `/usr/local/bin/nebula` (macOS)
- **Certificate tool**: `nebula-cert` (comes with binary)
- **Config directory**: `/etc/nebula/` (Linux), `/usr/local/etc/nebula/` (macOS)
- **Config file**: `config.yml` (YAML format)
- **Certificates**: `ca.crt`, `<hostname>.crt`, `<hostname>.key`

## Real-World Examples

### Check Installed Version

```bash
# Display version information
nebula -version

# Verify both nebula and nebula-cert are available
command -v nebula && command -v nebula-cert
```

### Network Verification

```bash
# Ping another node through nebula network (after setup)
ping 192.168.1.100  # Where 192.168.1.x is your nebula network

# Check service running on Linux
sudo systemctl status nebula

# Check service on macOS
launchctl list | grep nebula
```

### Multi-Cloud Network Setup

```bash
# After installing nebula, manually:
# 1. Generate certificates: nebula-cert sign -name lighthouses -ip 192.168.1.1/32
# 2. Create /etc/nebula/config.yml with lighthouse settings
# 3. Start service: sudo systemctl start nebula
#
# This creates unified network across:
nebula-site:
  aws_ec2_instances
  gcp_compute_instances
  azure_vms
  datacenter_servers
```

### Generate Certificates for New Node

```bash
# Create root CA (one-time)
nebula-cert ca -name "Organization Name"

# Issue certificate for a node
nebula-cert sign -name "host-01" -ip "192.168.1.10/32"

# Verify certificate
nebula-cert print -path host-01.crt
```

## Agent Use

- Automated network setup for multi-cloud deployments
- Dynamic node provisioning across infrastructure
- Network topology discovery and monitoring
- Certificate generation and lifecycle management
- Network segmentation and policy enforcement
- Cross-region connectivity validation
- Infrastructure-as-code network configuration

## Troubleshooting

### Lighthouse Cannot Be Reached

Ensure lighthouse nodes are:
- Running and listening on configured ports (default: 4242)
- Reachable from all nodes (check firewall rules)
- Have valid certificates signed by same CA

```bash
# Test connectivity to lighthouse
nc -zv lighthouse-host 4242

# Check nebula logs
sudo journalctl -u nebula -f
```

### Certificate Errors

```bash
# Verify certificate is valid
nebula-cert print -path /etc/nebula/hostname.crt

# Check certificate matches private key
nebula-cert print -path /etc/nebula/hostname.key

# Regenerate if corrupted
nebula-cert sign -name "hostname" -ip "192.168.1.x/32"
```

### Network Not Forming

- Check all nodes have `config.yml` with correct lighthouse addresses
- Verify certificates are signed by same CA
- Ensure firewall allows UDP 4242 (default port)
- Check IP ranges don't conflict with existing networks

```bash
# Show interface info
ip addr show nebula0  # Linux
ifconfig utun0        # macOS
```

## Uninstall

```yaml
- preset: nebula
  with:
    state: absent
```

## Resources

- Official documentation: https://nebula.defined.net/
- GitHub repository: https://github.com/slackhq/nebula
- Certificate generation guide: https://nebula.defined.net/docs/references/cli/
- Configuration examples: https://nebula.defined.net/docs/guides/quick-start/
- Search: "nebula networking tutorial", "nebula overlay network setup", "nebula multi-cloud"
