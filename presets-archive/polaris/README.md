# Polaris - Kubernetes Configuration Validation

Validate Kubernetes resource configurations against policy-as-code best practices with 30+ built-in checks.

## Quick Start

```yaml
- preset: polaris
```

## Features

- **30+ Built-in Checks**: Security, efficiency, and reliability best practices
- **Policy-as-Code**: Define and enforce custom policies with JSON Schema
- **Multiple Modes**: CLI, dashboard, admission controller, or CI/CD integration
- **Auto-Remediation**: Automatically fix issues based on policy criteria
- **Multi-Tenant**: Supports namespace isolation and access control
- **Zero Runtime Impact**: Validates configurations without affecting running workloads

## Basic Usage

```bash
# Audit Kubernetes cluster
polaris audit --format=pretty

# Audit specific namespace
polaris audit --namespace production

# Validate local YAML files
polaris audit --audit-path ./k8s-manifests/

# Generate JSON report
polaris audit --format=json > report.json

# Check specific resources
polaris audit --resource deployment/myapp

# Show version
polaris version
```

## Advanced Configuration

```yaml
# Install with custom configuration
- preset: polaris
  with:
    state: present

# Uninstall
- preset: polaris
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support

- ✅ Linux (apt, dnf, yum, pacman, brew)
- ✅ macOS (Homebrew, binary)
- ❌ Windows (not directly supported)

## Configuration

Polaris uses a configuration file to define policies. Default checks include:

**Security Checks:**
- Host network/IPC/PID usage
- Privileged containers
- ReadOnlyRootFilesystem
- RunAsNonRoot
- Capabilities

**Efficiency Checks:**
- Resource requests and limits
- LimitRange configuration

**Reliability Checks:**
- Health checks (liveness/readiness probes)
- Multiple replicas
- PodDisruptionBudget

Create a custom config file:

```yaml
# polaris-config.yaml
checks:
  cpuRequestsMissing: warning
  cpuLimitsMissing: warning
  memoryRequestsMissing: warning
  memoryLimitsMissing: warning
  hostNetworkSet: danger
  hostPortSet: warning
  notReadOnlyRootFilesystem: warning
  privilegeEscalationAllowed: danger
  runAsRootAllowed: warning
  runAsPrivileged: danger
  dangerousCapabilities: danger
  insecureCapabilities: warning
```

Run with custom config:
```bash
polaris audit --config polaris-config.yaml
```

## Real-World Examples

### CI/CD Pipeline Validation

```yaml
# .github/workflows/k8s-validation.yml
- name: Validate Kubernetes manifests
  preset: polaris

- name: Run Polaris audit
  shell: |
    polaris audit --audit-path ./k8s/ \
      --format=json \
      --set-exit-code-on-danger \
      --set-exit-code-below-score 90
  register: polaris_result

- name: Fail on policy violations
  assert:
    command:
      cmd: "[ {{ polaris_result.rc }} -eq 0 ]"
      exit_code: 0
```

### Pre-Deployment Validation

```bash
# Validate before applying to cluster
polaris audit --audit-path ./deployment.yaml --format=pretty

# Check current cluster configuration
polaris audit --namespace production --format=pretty

# Generate report for security team
polaris audit --format=json > security-audit-$(date +%Y%m%d).json
```

### Admission Controller Mode

Deploy Polaris as a webhook to block non-compliant resources:

```bash
# Polaris will reject deployments that fail critical checks
kubectl apply -f deployment.yaml
# Error: deployment violates policy: runAsPrivileged
```

### Dashboard Mode

View cluster-wide policy violations in a web UI:

```bash
# Install dashboard (separate from this preset)
kubectl apply -f https://github.com/FairwindsOps/polaris/releases/latest/download/dashboard.yaml

# Port forward to access
kubectl port-forward -n polaris svc/polaris-dashboard 8080:80

# Open http://localhost:8080
```

## Agent Use

- Validate Kubernetes configurations in CI/CD before deployment
- Enforce security policies across multi-tenant clusters
- Generate compliance reports for audit purposes
- Identify misconfigurations that could cause production issues
- Automate remediation of common configuration problems
- Compare configurations against organizational best practices
- Validate infrastructure-as-code templates (Helm, Kustomize)

## Troubleshooting

### Command not found: polaris

After installation, ensure the binary is in your PATH:
```bash
which polaris
polaris version
```

### Permission denied errors

Some audit operations require cluster access:
```bash
# Ensure kubectl is configured
kubectl cluster-info

# Check permissions
kubectl auth can-i get deployments --all-namespaces
```

### Custom checks not working

Verify your configuration file syntax:
```bash
polaris audit --config polaris-config.yaml --format=json | jq .
```

## Uninstall

```yaml
- preset: polaris
  with:
    state: absent
```

## Resources

- Official docs: https://polaris.docs.fairwinds.com/
- GitHub: https://github.com/FairwindsOps/polaris
- Fairwinds website: https://www.fairwinds.com/polaris
- Search: "polaris kubernetes validation", "polaris admission controller", "kubernetes policy enforcement"
