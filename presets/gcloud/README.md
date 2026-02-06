# gcloud - Google Cloud SDK

Manage Google Cloud Platform resources from the command line. Complete toolset for GCP services including Compute, Storage, Kubernetes, and more.

## Quick Start
```yaml
- preset: gcloud
```

## Features
- **Complete GCP management**: Command-line access to all Google Cloud services
- **Multiple tools**: gcloud, gsutil (Cloud Storage), bq (BigQuery)
- **Authentication**: OAuth2 and service account support
- **Shell completion**: bash, zsh, fish support
- **Cross-platform**: Linux, macOS, Windows support

## Basic Usage
```bash
# Authenticate
gcloud auth login

# Set project
gcloud config set project my-project-id

# List compute instances
gcloud compute instances list

# Create GKE cluster
gcloud container clusters create my-cluster --num-nodes=3

# Deploy to Cloud Run
gcloud run deploy my-service --source .

# Copy to Cloud Storage
gsutil cp file.txt gs://my-bucket/

# Query BigQuery
bq query "SELECT * FROM dataset.table LIMIT 10"
```

## Advanced Configuration
```yaml
- preset: gcloud
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Google Cloud SDK |

## Platform Support
- ✅ Linux (apt, yum, snap, tar.gz)
- ✅ macOS (Homebrew, tar.gz)
- ✅ Windows (installer)

## Configuration
- **Config directory**: `~/.config/gcloud/`
- **Credentials**: `~/.config/gcloud/credentials.db`
- **Active config**: `~/.config/gcloud/configurations/config_default`
- **Components**: Install additional tools with `gcloud components install`

## Real-World Examples

### CI/CD with Service Account
```yaml
# Authenticate with service account in pipeline
- name: Install gcloud
  preset: gcloud

- name: Authenticate
  shell: |
    echo "${{ secrets.GCP_SA_KEY }}" | gcloud auth activate-service-account --key-file=-
    gcloud config set project ${{ secrets.GCP_PROJECT }}

- name: Deploy to Cloud Run
  shell: gcloud run deploy api --image gcr.io/project/api:latest --region us-central1
```

### Kubernetes Cluster Management
```bash
# Create GKE cluster
gcloud container clusters create prod-cluster \
  --zone us-central1-a \
  --num-nodes 3 \
  --machine-type n1-standard-2 \
  --enable-autoscaling --min-nodes 1 --max-nodes 10

# Get credentials for kubectl
gcloud container clusters get-credentials prod-cluster --zone us-central1-a

# Manage cluster
kubectl get nodes
```

### Cloud Storage Operations
```bash
# Create bucket
gsutil mb -l us-central1 gs://my-backup-bucket

# Sync directory to Cloud Storage
gsutil -m rsync -r ./data gs://my-backup-bucket/data

# Set lifecycle policy
gsutil lifecycle set lifecycle.json gs://my-bucket

# Make objects public
gsutil iam ch allUsers:objectViewer gs://my-bucket
```

### Infrastructure as Code
```bash
# Deploy with gcloud CLI
gcloud compute instances create web-server \
  --image-family debian-11 \
  --image-project debian-cloud \
  --machine-type e2-medium \
  --tags http-server \
  --metadata startup-script='#!/bin/bash
    apt-get update
    apt-get install -y nginx'
```

## Agent Use
- Provision and manage GCP infrastructure from automation scripts
- Deploy applications to Cloud Run, App Engine, or GKE
- Manage Cloud Storage buckets and objects programmatically
- Query BigQuery datasets for analytics pipelines
- Configure IAM policies and service accounts
- Monitor and manage cloud resources at scale

## Troubleshooting

### Authentication failed
```bash
# Re-authenticate
gcloud auth login

# List auth accounts
gcloud auth list

# Switch account
gcloud config set account user@example.com

# Service account authentication
gcloud auth activate-service-account --key-file=key.json
```

### Project not set
```bash
# Set default project
gcloud config set project PROJECT_ID

# Create new configuration
gcloud config configurations create dev
gcloud config set project dev-project-id

# Switch configurations
gcloud config configurations activate prod
```

### Component not found
```bash
# Update components
gcloud components update

# Install specific component
gcloud components install kubectl
gcloud components install beta

# List available components
gcloud components list
```

### Permission denied
```bash
# Check current permissions
gcloud projects get-iam-policy PROJECT_ID

# Add IAM role
gcloud projects add-iam-policy-binding PROJECT_ID \
  --member='user:email@example.com' \
  --role='roles/editor'
```

## Uninstall
```yaml
- preset: gcloud
  with:
    state: absent
```

## Resources
- Official docs: https://cloud.google.com/sdk/docs
- GitHub: https://github.com/GoogleCloudPlatform/google-cloud-sdk
- gcloud reference: https://cloud.google.com/sdk/gcloud/reference
- gsutil docs: https://cloud.google.com/storage/docs/gsutil
- Search: "gcloud tutorial", "google cloud sdk", "gcloud authentication"
