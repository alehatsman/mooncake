# k3sup - k3s Installer

Bootstrap Kubernetes with k3s over SSH in under 60 seconds. Install, join nodes, and configure kubectl with a single command.

## Quick Start
```yaml
- preset: k3sup
```

## Features
- **Fast**: Install k3s on any Linux host in under 60 seconds
- **SSH-based**: No need to SSH manually, k3sup handles everything
- **kubectl config**: Automatically merges kubeconfig for immediate access
- **Multi-node**: Join additional nodes with a single command
- **ARM support**: Works on Raspberry Pi, AWS Graviton, and other ARM devices
- **Cross-platform client**: Run k3sup from Linux, macOS, or Windows

## Basic Usage
```bash
# Install k3s on a remote server
k3sup install --ip 192.168.1.100 --user ubuntu

# Join a worker node
k3sup join --ip 192.168.1.101 --server-ip 192.168.1.100 --user ubuntu

# Install with custom k3s version
k3sup install --ip 192.168.1.100 --user ubuntu --k3s-version v1.28.4+k3s1

# Install locally
k3sup install --local

# Get merged kubeconfig
k3sup install --ip 192.168.1.100 --user ubuntu --merge --local-path ~/.kube/config
```

## Advanced Configuration
```yaml
# Basic installation
- preset: k3sup

# Install and verify
- preset: k3sup
  register: k3sup_result

- name: Show k3sup version
  shell: k3sup version
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove k3sup |

## Platform Support
- ✅ Linux (curl install script)
- ✅ macOS (Homebrew)
- ❌ Windows (manual install via GitHub releases)

**Note**: k3sup is the installer tool. The actual k3s cluster runs on Linux hosts.

## Configuration
- **Binary location**: `/usr/local/bin/k3sup`
- **No config file**: k3sup is a CLI tool with no persistent configuration
- **Kubeconfig**: Saved to current directory as `kubeconfig` unless `--merge` or `--local-path` specified

## Real-World Examples

### Homelab Cluster Setup
```bash
# Install k3s on master node
k3sup install \
  --ip 192.168.1.10 \
  --user pi \
  --k3s-extra-args '--disable traefik' \
  --merge \
  --local-path ~/.kube/config \
  --context homelab

# Join worker nodes
k3sup join --ip 192.168.1.11 --server-ip 192.168.1.10 --user pi
k3sup join --ip 192.168.1.12 --server-ip 192.168.1.10 --user pi
```

### Development Environment
```bash
# Install k3s locally for testing
k3sup install --local --k3s-extra-args '--disable traefik'

# Test your manifests
kubectl apply -f deployment.yaml
kubectl get pods
```

### CI/CD Pipeline
```yaml
# Provision test cluster in CI
- name: Setup k3s test cluster
  shell: |
    k3sup install --local --k3s-version v1.28.4+k3s1
    export KUBECONFIG=$(pwd)/kubeconfig
    kubectl wait --for=condition=Ready nodes --all --timeout=60s

- name: Run integration tests
  shell: |
    export KUBECONFIG=$(pwd)/kubeconfig
    kubectl apply -f test/fixtures/
    ./run-tests.sh
```

### Multi-Node Production Cluster
```bash
# High-availability control plane (3 servers)
k3sup install --ip 10.0.1.10 --user ubuntu --cluster --k3s-version v1.28.4+k3s1
k3sup join --ip 10.0.1.11 --server-ip 10.0.1.10 --user ubuntu --server
k3sup join --ip 10.0.1.12 --server-ip 10.0.1.10 --user ubuntu --server

# Add worker nodes
for ip in 10.0.2.{20..25}; do
  k3sup join --ip $ip --server-ip 10.0.1.10 --user ubuntu
done
```

## Agent Use
- Provision ephemeral Kubernetes clusters for integration testing
- Automate homelab and edge cluster setup
- Create isolated test environments in CI/CD pipelines
- Bootstrap development Kubernetes clusters
- Deploy k3s to IoT and edge devices at scale
- Set up multi-tenant testing environments

## Common Flags
```bash
--ip              # Target server IP
--user            # SSH user
--ssh-key         # Path to SSH private key (default: ~/.ssh/id_rsa)
--k3s-version     # Specific k3s version
--k3s-extra-args  # Additional k3s server args
--local           # Install on local machine
--merge           # Merge kubeconfig into ~/.kube/config
--local-path      # Custom kubeconfig path
--context         # kubectl context name
--cluster         # Initialize HA cluster
--server          # Join as server node (HA)
```

## Troubleshooting

### SSH Connection Failed
```bash
# Verify SSH access
ssh user@host

# Specify custom SSH key
k3sup install --ip 192.168.1.100 --user ubuntu --ssh-key ~/.ssh/custom_key

# Use SSH port other than 22
k3sup install --ip 192.168.1.100 --user ubuntu --ssh-port 2222
```

### Kubeconfig Not Working
```bash
# Check kubeconfig file
cat kubeconfig

# Use explicit path
export KUBECONFIG=$(pwd)/kubeconfig
kubectl get nodes

# Merge into existing config
k3sup install --ip 192.168.1.100 --user ubuntu --merge --local-path ~/.kube/config
```

### k3s Version Mismatch
```bash
# List available versions
curl -s https://api.github.com/repos/k3s-io/k3s/releases | jq -r '.[].tag_name' | head

# Install specific version
k3sup install --ip 192.168.1.100 --user ubuntu --k3s-version v1.28.4+k3s1
```

## Uninstall
```yaml
# Remove k3sup tool
- preset: k3sup
  with:
    state: absent
```

**Note**: This removes k3sup tool only. To uninstall k3s from remote servers:
```bash
# On remote server
sudo /usr/local/bin/k3s-uninstall.sh
```

## Resources
- Official docs: https://github.com/alexellis/k3sup
- k3s docs: https://docs.k3s.io/
- Search: "k3sup tutorial", "k3s bootstrap ssh", "k3sup homelab"
