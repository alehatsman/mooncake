# Azure CLI - Microsoft Azure Command-Line Interface

Official command-line interface for managing Microsoft Azure cloud resources and services.

## Quick Start
```yaml
- preset: azure-cli
```

## Features
- **Complete Azure management**: Control all Azure services from command line
- **Cross-platform**: Linux, macOS, Windows support
- **Multiple authentication**: Interactive login, service principals, managed identities
- **Script-friendly**: JSON output, query support with JMESPath
- **Extension system**: Extend functionality with community extensions
- **Cloud Shell integration**: Pre-installed in Azure Cloud Shell

## Basic Usage
```bash
# Login to Azure
az login

# List subscriptions
az account list

# Set active subscription
az account set --subscription "My Subscription"

# List resource groups
az group list

# Create resource group
az group create --name mygroup --location eastus

# List VMs
az vm list

# Get resource details in JSON
az vm show --resource-group mygroup --name myvm

# Use JMESPath queries
az vm list --query "[].{name:name, location:location}"
```

## Advanced Configuration

```yaml
# Install Azure CLI
- preset: azure-cli
  register: az_result

# Verify installation
- name: Check Azure CLI version
  shell: az version
  register: version

- name: Display version
  print: "Azure CLI version {{ version.stdout }}"

# Install with extensions
- preset: azure-cli

- name: Install Azure DevOps extension
  shell: az extension add --name azure-devops

- name: Install AKS extension
  shell: az extension add --name aks-preview
```

## Authentication Methods

### Interactive Login
```bash
# Browser-based login
az login

# Login with specific tenant
az login --tenant contoso.onmicrosoft.com

# Login and select subscription
az login
az account set --subscription "Production"
```

### Service Principal
```bash
# Login with service principal
az login --service-principal \
  --username $APP_ID \
  --password $PASSWORD \
  --tenant $TENANT_ID

# Or use certificate
az login --service-principal \
  --username $APP_ID \
  --tenant $TENANT_ID \
  --password /path/to/cert.pem
```

### Managed Identity (Azure VMs)
```bash
# Login with system-assigned managed identity
az login --identity

# Login with user-assigned managed identity
az login --identity --username $CLIENT_ID
```

## Common Service Operations

### Virtual Machines
```bash
# Create VM
az vm create \
  --resource-group mygroup \
  --name myvm \
  --image Ubuntu2204 \
  --size Standard_B2s \
  --admin-username azureuser \
  --generate-ssh-keys

# Start/stop VM
az vm start --resource-group mygroup --name myvm
az vm stop --resource-group mygroup --name myvm

# Deallocate VM (stop billing)
az vm deallocate --resource-group mygroup --name myvm
```

### Storage
```bash
# Create storage account
az storage account create \
  --name mystorageaccount \
  --resource-group mygroup \
  --location eastus \
  --sku Standard_LRS

# Create blob container
az storage container create \
  --name mycontainer \
  --account-name mystorageaccount

# Upload blob
az storage blob upload \
  --container-name mycontainer \
  --name myfile.txt \
  --file /local/path/myfile.txt \
  --account-name mystorageaccount
```

### Azure Kubernetes Service (AKS)
```bash
# Create AKS cluster
az aks create \
  --resource-group mygroup \
  --name myakscluster \
  --node-count 3 \
  --enable-addons monitoring \
  --generate-ssh-keys

# Get credentials
az aks get-credentials --resource-group mygroup --name myakscluster

# Scale cluster
az aks scale --resource-group mygroup --name myakscluster --node-count 5
```

## Configuration

### Config File Locations
- **Linux/macOS**: `~/.azure/config`
- **Windows**: `%USERPROFILE%\.azure\config`
- **Cloud directory**: `~/.azure/` (credentials, logs, telemetry)

### Configuration Commands
```bash
# Set default location
az configure --defaults location=eastus

# Set default resource group
az configure --defaults group=mygroup

# Set output format (json, table, tsv, yaml)
az config set core.output=table

# Disable telemetry
az config set core.collect_telemetry=no
```

## Output Formatting

```bash
# JSON output (default)
az vm list

# Table format
az vm list --output table

# TSV (tab-separated values)
az vm list --output tsv

# YAML format
az vm list --output yaml

# Query with JMESPath
az vm list --query "[?location=='eastus'].{Name:name, Size:hardwareProfile.vmSize}"

# Get single value
az vm show --resource-group mygroup --name myvm --query "powerState" --output tsv
```

## Real-World Examples

### CI/CD Pipeline Deployment
```yaml
# Deploy application to Azure App Service
- preset: azure-cli

- name: Login with service principal
  shell: |
    az login --service-principal \
      --username ${{ secrets.AZURE_CLIENT_ID }} \
      --password ${{ secrets.AZURE_CLIENT_SECRET }} \
      --tenant ${{ secrets.AZURE_TENANT_ID }}
  no_log: true

- name: Deploy web app
  shell: |
    az webapp deploy \
      --resource-group production \
      --name mywebapp \
      --src-path ./app.zip \
      --type zip
```

### Infrastructure Provisioning
```bash
# Create complete infrastructure
az group create --name production --location eastus

az network vnet create \
  --resource-group production \
  --name myvnet \
  --address-prefix 10.0.0.0/16

az network nsg create \
  --resource-group production \
  --name mynsg

az vm create \
  --resource-group production \
  --name webserver \
  --image Ubuntu2204 \
  --vnet-name myvnet \
  --nsg mynsg \
  --size Standard_B2ms
```

### Resource Cleanup Script
```bash
# List and delete old resources
# Find resource groups older than 30 days
az group list --query "[?tags.environment=='test']" --output tsv | \
while read -r name location; do
  echo "Deleting resource group: $name"
  az group delete --name "$name" --yes --no-wait
done
```

### Cost Management
```bash
# Get cost analysis
az consumption usage list \
  --start-date 2024-01-01 \
  --end-date 2024-01-31 \
  --query "sum([].pretaxCost)" \
  --output tsv

# List resources by cost
az resource list \
  --query "sort_by([], &sku.tier)" \
  --output table
```

## Extensions

```bash
# List available extensions
az extension list-available --output table

# Install extension
az extension add --name azure-devops
az extension add --name aks-preview
az extension add --name ai-examples

# Update all extensions
az extension update --name azure-devops

# Remove extension
az extension remove --name azure-devops
```

## Troubleshooting

### Authentication Issues
```bash
# Clear cached credentials
az account clear

# Re-login
az login

# Check current account
az account show

# Verify subscription access
az account list --output table
```

### Network Connectivity
```bash
# Test connectivity to Azure
az rest --url "https://management.azure.com/subscriptions?api-version=2020-01-01"

# Use specific cloud (China, Government)
az cloud set --name AzureChinaCloud
az login
```

### Command Timeout
```bash
# Increase HTTP timeout (seconds)
export AZURE_HTTP_TIMEOUT=300

# Or configure permanently
az config set core.http_timeout=300
```

### Debug Mode
```bash
# Enable debug logging
az vm list --debug

# Enable verbose output
az vm list --verbose
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, Homebrew)
- ✅ macOS (Homebrew, native installer)
- ✅ Windows (MSI installer, winget, Chocolatey)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Provision Azure infrastructure in CI/CD pipelines
- Automate resource deployments across environments
- Query cloud resource state for monitoring
- Manage AKS clusters and container deployments
- Automate cost reporting and resource cleanup
- Configure Azure services via Infrastructure as Code
- Integrate with GitHub Actions, Azure DevOps, Jenkins

## Uninstall
```yaml
- preset: azure-cli
  with:
    state: absent
```

## Resources
- Official docs: https://learn.microsoft.com/en-us/cli/azure/
- Command reference: https://learn.microsoft.com/en-us/cli/azure/reference-index
- GitHub: https://github.com/Azure/azure-cli
- Extensions: https://learn.microsoft.com/en-us/cli/azure/azure-cli-extensions-list
- Search: "azure cli tutorial", "azure cli examples", "azure cli best practices"
