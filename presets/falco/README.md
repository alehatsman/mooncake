# Falco - Cloud Native Runtime Security

Cloud-native runtime security tool for threat detection and behavioral monitoring. Detect anomalous activity in applications, containers, and Kubernetes clusters.

## Quick Start
```yaml
- preset: falco
  become: true
```

## Features
- **Runtime threat detection**: Real-time monitoring of system calls and kernel events
- **Container security**: Native Docker and Kubernetes integration
- **Custom rules**: Flexible rule engine for defining suspicious behaviors
- **eBPF or kernel module**: Multiple deployment options for syscall capture
- **CNCF graduated**: Production-ready, vendor-neutral security standard

## Basic Usage
```bash
# Start Falco
sudo falco

# Load custom rules
sudo falco -r /etc/falco/custom_rules.yaml

# Test with sample events
sudo falco -A

# Check configuration
falco --version
cat /etc/falco/falco.yaml
```

## Advanced Configuration
```yaml
- preset: falco
  with:
    state: present
  become: true
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Falco |

## Platform Support
- ✅ Linux (apt, dnf, yum, zypper - kernel module or eBPF required)
- ❌ macOS (not supported - requires Linux kernel features)
- ❌ Windows (not supported)

## Configuration
- **Config file**: `/etc/falco/falco.yaml`
- **Rules directory**: `/etc/falco/rules.d/`
- **Default rules**: `/etc/falco/falco_rules.yaml`
- **Log output**: `/var/log/falco/` or stdout
- **Driver**: Kernel module (`falco-probe`) or eBPF probe

## Real-World Examples

### Kubernetes Security Monitoring
```yaml
# Deploy Falco as DaemonSet
- name: Install Falco on K8s nodes
  preset: falco
  become: true

# Monitor for suspicious container activity
# Falco will alert on:
# - Shell spawned in container
# - Sensitive file access (/etc/shadow)
# - Privilege escalation attempts
# - Network connections from unexpected processes
```

### Custom Security Rules
```yaml
# /etc/falco/rules.d/custom_rules.yaml
- rule: Detect Cryptocurrency Mining
  desc: Detect cryptocurrency mining processes
  condition: spawned_process and proc.name in (xmrig, ethminer, cgminer)
  output: "Crypto mining detected (user=%user.name command=%proc.cmdline)"
  priority: WARNING
```

### CI/CD Integration
```bash
# Run Falco in audit mode during deployment
sudo falco --dry-run -r /etc/falco/falco_rules.yaml

# Export alerts to SIEM
sudo falco -o json_output=true -o file_output.filename=/var/log/falco/events.json
```

## Agent Use
- Detect runtime threats in containerized applications
- Monitor Kubernetes clusters for security policy violations
- Alert on privilege escalation and container breakout attempts
- Audit compliance with security baselines (PCI-DSS, HIPAA)
- Integrate with SIEM for centralized security monitoring
- Generate forensic data for incident response

## Troubleshooting

### Falco driver not loaded
```bash
# Check driver status
sudo falco-driver-loader

# Install kernel headers (required for kernel module)
sudo apt-get install linux-headers-$(uname -r)  # Debian/Ubuntu
sudo dnf install kernel-devel-$(uname -r)       # Fedora/RHEL

# Use eBPF instead of kernel module
sudo falco --modern-bpf
```

### No events detected
```bash
# Verify Falco is capturing syscalls
sudo falco --list-syscall

# Generate test event
sudo sh -c "cat /etc/shadow"  # Should trigger alert

# Check configuration
sudo falco --validate /etc/falco/falco.yaml
```

### High CPU usage
```bash
# Reduce verbosity and filter rules
# Edit /etc/falco/falco.yaml
log_level: warning
rate_limiter:
  enabled: true
  max_events: 5000
```

## Uninstall
```yaml
- preset: falco
  with:
    state: absent
  become: true
```

## Resources
- Official docs: https://falco.org/docs/
- GitHub: https://github.com/falcosecurity/falco
- Rules repository: https://github.com/falcosecurity/rules
- Kubernetes deployment: https://falco.org/docs/getting-started/running/#kubernetes
- Search: "falco runtime security", "falco kubernetes rules", "falco threat detection"
