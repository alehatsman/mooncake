# kubescape - Kubernetes Security Platform

Open-source Kubernetes security platform that scans clusters, YAML files, and Helm charts for security misconfigurations and compliance violations.

## Quick Start
```yaml
- preset: kubescape
```

## Features
- **Multiple frameworks**: NSA, MITRE ATT&CK, CIS, DevOpsBest
- **Cluster scanning**: Live cluster security assessment
- **YAML scanning**: Pre-deployment manifest validation
- **Helm chart scanning**: Template security analysis
- **Risk scoring**: Prioritized findings with severity levels
- **Remediation guidance**: Detailed fix instructions

## Basic Usage
```bash
# Scan cluster with NSA framework
kubescape scan framework nsa

# Scan with MITRE ATT&CK
kubescape scan framework mitre

# Scan with CIS benchmark
kubescape scan framework cis

# Scan YAML files
kubescape scan *.yaml

# Scan Helm chart
kubescape scan --helm-chart ./mychart

# Scan specific namespace
kubescape scan --namespace kube-system

# Output as JSON
kubescape scan framework nsa --format json

# Generate HTML report
kubescape scan framework nsa --format html --output report.html
```

## Advanced Configuration
```yaml
- preset: kubescape
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove kubescape |

## Platform Support
- ✅ Linux (binary download)
- ✅ macOS (Homebrew)
- ✅ Windows (binary download)
- ✅ Docker (official container)

## Configuration
- **Config file**: `~/.kubescape/config.json`
- **Results cache**: `~/.kubescape/results/`
- **Kubeconfig**: Uses `~/.kube/config` or `$KUBECONFIG`

## Real-World Examples

### CI/CD Security Gate
```bash
# Scan manifests before deployment
kubescape scan *.yaml --format json --output results.json

# Fail pipeline on high-severity issues
if jq '.summaryDetails.riskScore > 50' results.json | grep -q true; then
  echo "Security risk too high!"
  exit 1
fi
```

### Multi-Framework Compliance Check
```bash
# Check against multiple frameworks
kubescape scan framework nsa,mitre --format json > compliance.json

# Extract failed controls
jq '.results[].controls[] | select(.failedResources > 0)' compliance.json
```

### Helm Chart Validation
```yaml
# Validate Helm chart before release
- name: Scan Helm chart
  shell: kubescape scan --helm-chart ./charts/myapp --format json
  register: scan

- name: Check for critical issues
  assert:
    command:
      cmd: echo "{{ scan.stdout }}" | jq '.summaryDetails.riskScore < 30'
      exit_code: 0
```

### Scheduled Cluster Scans
```yaml
# Run daily security scans
- name: Scan production cluster
  shell: |
    kubescape scan framework nsa --format json \
      --output /var/log/kubescape-$(date +%Y%m%d).json
  register: scan

- name: Send alert if issues found
  shell: mail -s "Security scan results" security@example.com < {{ scan.stdout }}
  when: scan.rc != 0
```

## Agent Use
- Automated security scanning in CI/CD pipelines
- Pre-deployment manifest validation
- Continuous compliance monitoring
- Security drift detection
- Multi-cluster security assessments

## Troubleshooting

### Cannot connect to cluster
Verify kubeconfig:
```bash
kubectl cluster-info
export KUBECONFIG=/path/to/kubeconfig
kubescape scan framework nsa
```

### High memory usage
Scan specific namespaces:
```bash
kubescape scan framework nsa --namespace production
```

### Permission errors
Kubescape needs read access to cluster resources:
```bash
# Check permissions
kubectl auth can-i get pods --all-namespaces
```

## Uninstall
```yaml
- preset: kubescape
  with:
    state: absent
```

## Resources
- Official docs: https://github.com/kubescape/kubescape
- Documentation: https://kubescape.io/docs
- Frameworks: https://hub.armosec.io/docs
- Search: "kubescape kubernetes security", "kubescape nsa mitre"
