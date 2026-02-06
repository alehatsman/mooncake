# Oracle Cloud CLI - OCI Command Line Interface

Official command-line interface for Oracle Cloud Infrastructure to manage resources and services.

## Quick Start

```yaml
- preset: oci-cli
```

## Features

- **Comprehensive coverage**: Manage all OCI services from CLI
- **JSON output**: Machine-readable output for automation
- **Configuration profiles**: Multiple account support
- **Instance principals**: Authenticate from OCI compute instances
- **Query and filter**: JMESPath queries for response filtering
- **Bulk operations**: Manage multiple resources efficiently
- **Cross-platform**: Linux, macOS, Windows

## Basic Usage

```bash
# Configure CLI (interactive)
oci setup config

# List compartments
oci iam compartment list --all

# List compute instances
oci compute instance list --compartment-id ocid1.compartment...

# Create compute instance
oci compute instance launch \
  --availability-domain AD-1 \
  --compartment-id ocid1.compartment... \
  --shape VM.Standard.E4.Flex \
  --image-id ocid1.image... \
  --subnet-id ocid1.subnet...

# List storage buckets
oci os bucket list --compartment-id ocid1.compartment...

# List databases
oci db database list --compartment-id ocid1.compartment...
```

## Advanced Configuration

```yaml
# Install OCI CLI
- preset: oci-cli

# Setup configuration
- name: Configure OCI CLI
  shell: |
    oci setup config \
      --config-location ~/.oci/config \
      --tenancy-id {{ oci_tenancy_id }} \
      --user-id {{ oci_user_id }} \
      --region {{ oci_region }} \
      --key-file ~/.oci/oci_api_key.pem

# Deploy API key
- name: Install OCI API key
  copy:
    content: "{{ oci_api_key }}"
    dest: ~/.oci/oci_api_key.pem
    mode: "0600"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove OCI CLI |

## Platform Support

- ✅ Linux (installer script, package managers)
- ✅ macOS (Homebrew, installer script)
- ✅ Windows (installer)

## Configuration

- **Config file**: `~/.oci/config`
- **API keys**: `~/.oci/` (private keys for authentication)
- **Profiles**: Multiple profiles for different tenancies

## Real-World Examples

### Infrastructure Automation
```yaml
# Provision OCI infrastructure
- name: Create VCN
  shell: |
    oci network vcn create \
      --compartment-id {{ compartment_id }} \
      --cidr-block 10.0.0.0/16 \
      --display-name "prod-vcn" \
      --wait-for-state AVAILABLE \
      --output json
  register: vcn

- name: Create subnet
  shell: |
    oci network subnet create \
      --compartment-id {{ compartment_id }} \
      --vcn-id {{ vcn.stdout | from_json | json_query('data.id') }} \
      --cidr-block 10.0.1.0/24 \
      --display-name "prod-subnet"
```

### Compute Management
```bash
# Start all stopped instances in compartment
oci compute instance list \
  --compartment-id ocid1.compartment... \
  --lifecycle-state STOPPED \
  --query 'data[*].id' \
  --output json \
  | jq -r '.[]' \
  | xargs -I {} oci compute instance action --instance-id {} --action START

# Create instance from backup
oci compute instance launch \
  --availability-domain AD-1 \
  --compartment-id ocid1.compartment... \
  --shape VM.Standard.E4.Flex \
  --source-details file://instance-source.json
```

### Object Storage Operations
```bash
# Upload file to bucket
oci os object put \
  --bucket-name mybucket \
  --file localfile.txt \
  --name remotefile.txt

# Sync directory to bucket
oci os object bulk-upload \
  --bucket-name mybucket \
  --src-dir ./dist/ \
  --overwrite

# Generate pre-authenticated request
oci os preauth-request create \
  --bucket-name mybucket \
  --name myfile.txt \
  --access-type ObjectRead \
  --time-expires 2026-12-31T23:59:59Z
```

### Database Management
```bash
# Create autonomous database
oci db autonomous-database create \
  --compartment-id ocid1.compartment... \
  --db-name mydb \
  --display-name "Production DB" \
  --admin-password MySecurePass123 \
  --cpu-core-count 1 \
  --data-storage-size-in-tbs 1

# Backup database
oci db backup create \
  --database-id ocid1.database... \
  --display-name "Daily Backup"
```

### CI/CD Integration
```yaml
# Deploy application to OCI
- name: Build application
  shell: docker build -t myapp:{{ version }} .

- name: Tag for OCIR
  shell: docker tag myapp:{{ version }} {{ ocir_region }}/{{ ocir_namespace }}/myapp:{{ version }}

- name: Push to Oracle Container Registry
  shell: docker push {{ ocir_region }}/{{ ocir_namespace }}/myapp:{{ version }}

- name: Update container instance
  shell: |
    oci container-instances container-instance update \
      --container-instance-id {{ instance_id }} \
      --containers file://containers.json
```

## Query and Filter

```bash
# Use JMESPath queries
oci compute instance list \
  --compartment-id ocid1.compartment... \
  --query 'data[?lifecycle-state==`RUNNING`].{Name:display-name,IP:primary-public-ip}'

# Filter with grep and jq
oci os bucket list --all | jq '.data[] | select(.name | contains("prod"))'

# Count resources
oci compute instance list --all --query 'length(data)'
```

## Agent Use

- Automate Oracle Cloud infrastructure provisioning
- Manage compute, storage, and networking resources programmatically
- Deploy and scale applications on OCI
- Perform backups and disaster recovery operations
- Monitor and audit cloud resources
- Integrate OCI with CI/CD pipelines

## Troubleshooting

### Authentication errors
```bash
# Verify configuration
cat ~/.oci/config

# Test connection
oci iam region list

# Check API key permissions
ls -la ~/.oci/oci_api_key.pem
```

### Rate limiting
```bash
# Add delays between calls
oci compute instance list && sleep 1

# Use --wait-for-state for long operations
oci compute instance launch ... --wait-for-state RUNNING
```

## Uninstall

```yaml
- preset: oci-cli
  with:
    state: absent
```

## Resources

- Official docs: https://docs.oracle.com/en-us/iaas/tools/oci-cli/latest/
- GitHub: https://github.com/oracle/oci-cli
- API reference: https://docs.oracle.com/en-us/iaas/api/
- Search: "oci cli tutorial", "oracle cloud cli examples", "oci automation"
