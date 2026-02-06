# kube-bench - Kubernetes CIS Benchmark Security Checker

Checks whether Kubernetes is deployed securely by running the checks documented in the CIS Kubernetes Benchmark.

## Quick Start
```yaml
- preset: kube-bench
```

## Features
- **CIS Benchmark compliance**: Automated security checks based on CIS standards
- **Multi-platform**: Works with various Kubernetes distributions
- **Detailed reporting**: JSON/YAML output with remediation advice
- **CI/CD integration**: Exit codes and structured output for automation
- **Master and node checks**: Validates both control plane and worker nodes

## Basic Usage
```bash
# Run all checks
sudo kube-bench

# Check specific target (master, node, etcd, policies)
sudo kube-bench --targets master
sudo kube-bench --targets node

# Output as JSON
sudo kube-bench --json

# Run specific benchmark version
sudo kube-bench --benchmark cis-1.6

# Check specific section
sudo kube-bench run --check 1.2.1
```

## Advanced Configuration
```yaml
- preset: kube-bench
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove kube-bench |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew, for development/testing)
- ❌ Windows (not supported)

## Configuration
- **Config files**: `/etc/kube-bench/cfg/` or `./cfg/`
- **Benchmark versions**: CIS 1.5, 1.6, 1.7, 1.8
- **Requires**: Root/sudo access to check system files

## Real-World Examples

### CI/CD Security Gate
```bash
# Run in CI pipeline and fail if critical issues found
sudo kube-bench --json > results.json
if jq '.Totals.fail > 0' results.json | grep -q true; then
  echo "Security issues detected!"
  exit 1
fi
```

### Scheduled Compliance Checks
```yaml
# Run daily via cron/systemd timer
- name: Run kube-bench security scan
  shell: kube-bench --json > /var/log/kube-bench-$(date +%Y%m%d).json
  become: true
  register: scan

- name: Alert on failures
  shell: mail -s "Kube-bench failures" security@example.com
  when: scan.rc != 0
```

### Specific Node Type Checks
```bash
# On master nodes
sudo kube-bench --targets master --json

# On worker nodes
sudo kube-bench --targets node --json

# etcd nodes
sudo kube-bench --targets etcd --json
```

## Agent Use
- Automated Kubernetes security auditing in CI/CD pipelines
- Compliance reporting and tracking over time
- Pre-deployment security validation
- Scheduled security scans with alerting
- Integration with security dashboards and SIEM systems

## Troubleshooting

### Permission denied errors
kube-bench needs root access to read Kubernetes config files:
```bash
sudo kube-bench
```

### Config files not found
Specify config location:
```bash
kube-bench --config-dir /path/to/cfg
```

### Wrong benchmark version
Specify the correct CIS version for your Kubernetes:
```bash
kube-bench --benchmark cis-1.7
```

## Uninstall
```yaml
- preset: kube-bench
  with:
    state: absent
```

## Resources
- Official docs: https://github.com/aquasecurity/kube-bench
- CIS Kubernetes Benchmark: https://www.cisecurity.org/benchmark/kubernetes
- Search: "kube-bench kubernetes security", "CIS benchmark kubernetes"
