# kube-hunter - Kubernetes Penetration Testing Tool

Hunts for security weaknesses in Kubernetes clusters by testing for known vulnerabilities and misconfigurations.

## Quick Start
```yaml
- preset: kube-hunter
```

## Features
- **Active and passive scanning**: Network probing or pod-based internal scanning
- **CVE detection**: Tests for known Kubernetes vulnerabilities
- **Automated reporting**: JSON/YAML output with severity ratings
- **Multiple scan modes**: Remote, pod, CIDR range scanning
- **Authentication testing**: Tests for anonymous access and weak credentials

## Basic Usage
```bash
# Interactive mode (prompts for options)
kube-hunter

# Scan remote cluster
kube-hunter --remote some.node.com

# Scan pod network (from within cluster)
kube-hunter --pod

# Scan CIDR range
kube-hunter --cidr 192.168.0.0/24

# Active hunting (attempts exploits)
kube-hunter --active

# Output as JSON
kube-hunter --report json

# List all tests
kube-hunter --list
```

## Advanced Configuration
```yaml
- preset: kube-hunter
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove kube-hunter |

## Platform Support
- ✅ Linux (pip install)
- ✅ macOS (pip install)
- ✅ Docker (official container image)
- ❌ Windows (use Docker or WSL)

## Configuration
- **No config file**: All options via CLI flags
- **Runs as**: Python script or container
- **Network access**: Requires connectivity to target cluster

## Real-World Examples

### CI/CD Security Scan
```bash
# Scan cluster before deployment
kube-hunter --remote k8s.example.com --report json > security-report.json

# Check for critical issues
if jq '.vulnerabilities[] | select(.severity == "high")' security-report.json | grep -q .; then
  echo "Critical vulnerabilities found!"
  exit 1
fi
```

### In-Cluster Security Audit
```yaml
# Deploy as Kubernetes job
apiVersion: batch/v1
kind: Job
metadata:
  name: kube-hunter
spec:
  template:
    spec:
      containers:
      - name: kube-hunter
        image: aquasec/kube-hunter
        command: ["kube-hunter", "--pod", "--report", "json"]
      restartPolicy: Never
```

### Network Range Scan
```bash
# Scan entire cluster network
kube-hunter --cidr 10.96.0.0/12 --dispatch remote --report json
```

### Active Exploitation Testing
```bash
# WARNING: Only run in test environments
kube-hunter --remote test-cluster.example.com --active
```

## Agent Use
- Automated security testing in staging environments
- Pre-production vulnerability scanning
- Scheduled security assessments
- Red team exercises and penetration testing
- Security validation after cluster updates

## Troubleshooting

### No vulnerabilities found
May indicate:
- Cluster is well-secured
- Network issues preventing detection
- Need to run with `--active` flag for deeper testing

### Connection refused
Check cluster accessibility:
```bash
# Verify API server is reachable
kubectl cluster-info

# Test direct connection
curl -k https://k8s.example.com:6443
```

### Permission denied
Running as pod requires appropriate ServiceAccount permissions:
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kube-hunter
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kube-hunter
rules:
- apiGroups: [""]
  resources: ["*"]
  verbs: ["get", "list"]
```

## Uninstall
```yaml
- preset: kube-hunter
  with:
    state: absent
```

**Note**: Python-based tool, removes via pip.

## Resources
- Official docs: https://github.com/aquasecurity/kube-hunter
- Documentation: https://aquasecurity.github.io/kube-hunter/
- Search: "kube-hunter kubernetes security", "kubernetes penetration testing"
