# IBM Cloud CLI - IBM Cloud Command Line Interface

Unified command-line interface for managing IBM Cloud resources, services, and deployments.

## Quick Start
```yaml
- preset: ibmcloud-cli
```

## Features
- **Unified interface**: Manage all IBM Cloud services from one CLI
- **Resource management**: VMs, containers, databases, AI services
- **Kubernetes**: IBM Cloud Kubernetes Service (IKS) and Red Hat OpenShift
- **Plugins**: Extend functionality with official and community plugins
- **Multi-account**: Switch between multiple IBM Cloud accounts
- **Cross-platform**: Linux, macOS, and Windows support

## Basic Usage
```bash
# Login
ibmcloud login
ibmcloud login --sso  # Single sign-on

# List regions
ibmcloud regions

# Set target region and resource group
ibmcloud target -r us-south -g default

# List resources
ibmcloud resource service-instances

# Kubernetes cluster management
ibmcloud ks clusters
ibmcloud ks cluster config --cluster my-cluster

# Container registry
ibmcloud cr images
ibmcloud cr namespaces

# Cloud Functions
ibmcloud fn action list
ibmcloud fn action invoke my-function

# Plugins
ibmcloud plugin list
ibmcloud plugin install container-service
```

## Configuration
- **Config file**: `~/.bluemix/config.json`
- **Credentials**: `~/.bluemix/.cf/config.json`
- **Default region**: Configurable per session

## Real-World Examples

### Deploy Application to Cloud Foundry
```bash
# Target Cloud Foundry org and space
ibmcloud target --cf

# Push application
ibmcloud cf push my-app -m 512M

# Check status
ibmcloud cf apps

# View logs
ibmcloud cf logs my-app --recent
```

### Kubernetes Cluster Operations
```yaml
- name: Create IKS cluster
  shell: |
    ibmcloud ks cluster create classic \
      --name production \
      --zone dal10 \
      --flavor b3c.4x16 \
      --workers 3

- name: Get cluster config
  shell: ibmcloud ks cluster config --cluster production

- name: Verify cluster
  shell: kubectl get nodes
```

### Manage Object Storage
```bash
# Create COS instance
ibmcloud resource service-instance-create my-cos \
  cloud-object-storage standard global

# Create bucket
ibmcloud cos bucket-create --bucket my-bucket \
  --ibm-service-instance-id <instance-id>

# Upload file
ibmcloud cos object-put --bucket my-bucket \
  --key myfile.txt --body ./local-file.txt
```

## Agent Use
- Automate IBM Cloud resource provisioning and management
- Deploy applications to Cloud Foundry, Kubernetes, or VMs
- Manage multi-cloud deployments with IBM Cloud integration
- Implement infrastructure as code for IBM Cloud services
- Monitor and manage cloud costs and usage
- Integrate IBM Watson AI services into workflows

## Advanced Configuration
```yaml
- preset: ibmcloud-cli
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove IBM Cloud CLI |

## Troubleshooting

### Authentication Issues
```bash
# Clear credentials and re-login
rm -rf ~/.bluemix
ibmcloud login

# Check current user
ibmcloud target

# Use API key
ibmcloud login --apikey @/path/to/key.json
```

### Plugin Problems
```bash
# Update plugins
ibmcloud plugin update --all

# Reinstall plugin
ibmcloud plugin uninstall container-service
ibmcloud plugin install container-service
```

## Platform Support
- ✅ Linux (script installation)
- ✅ macOS (installer)
- ✅ Windows (installer)

## Uninstall
```yaml
- preset: ibmcloud-cli
  with:
    state: absent
```

## Resources
- Official docs: https://cloud.ibm.com/docs/cli
- GitHub: https://github.com/IBM-Cloud/ibm-cloud-cli-release
- Plugins: https://cloud.ibm.com/docs/cli?topic=cli-plug-ins
- Search: "ibmcloud cli tutorial", "ibm cloud kubernetes", "ibmcloud commands"
