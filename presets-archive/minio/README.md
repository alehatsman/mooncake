# MinIO - S3-Compatible Object Storage Server

High-performance, Kubernetes-native object storage server compatible with Amazon S3. Perfect for private cloud deployments, AI/ML workflows, and data lakes.

## Quick Start

```yaml
- preset: minio
```

## Features

- **S3 Compatible**: Full AWS S3 API compatibility for seamless integration
- **High Performance**: Written in Go, optimized for distributed deployments
- **Web Console**: Intuitive UI for bucket and user management
- **Multi-node**: Distributed architecture for scalability and reliability
- **Security**: IAM policies, encryption, versioning, and audit logging
- **Cross-platform**: Linux, macOS with Homebrew support

## Basic Usage

```bash
# Check MinIO version
minio --version

# Access web console
open http://localhost:9001          # macOS
xdg-open http://localhost:9001      # Linux

# Configure MinIO client alias
mc alias set myminio http://localhost:9000 minioadmin minioadmin

# Create a bucket
mc mb myminio/my-data

# Upload a file
mc cp data.json myminio/my-data/

# List bucket contents
mc ls myminio/my-data

# Check server info
mc admin info myminio
```

## Advanced Configuration

```yaml
- preset: minio
  with:
    state: present
    data_dir: /mnt/data          # Custom storage location
    api_port: 9000               # S3 API port
    console_port: 9001           # Web UI port
    root_user: s3admin           # Admin username
    root_password: MySecret123!   # Admin password (min 8 chars)
    start_service: true
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |
| data_dir | string | /var/lib/minio | Data storage directory |
| api_port | string | 9000 | S3 API service port |
| console_port | string | 9001 | Web console UI port |
| root_user | string | minioadmin | Root access key ID |
| root_password | string | minioadmin | Root secret key (min 8 chars) |
| start_service | bool | true | Start service after installation |

## Configuration

- **Data directory**: `/var/lib/minio` (stores buckets, objects, metadata)
- **Config location**: Environment variables (MINIO_ROOT_USER, MINIO_ROOT_PASSWORD)
- **API port**: 9000 (S3-compatible object operations)
- **Console port**: 9001 (Web UI for management)
- **Service name**: minio (systemd/launchd)

## Platform Support

- ✅ Linux (apt, dnf, yum, pacman via script download)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

## Real-World Examples

### Development S3-Compatible Storage

Use MinIO locally instead of AWS S3 during development:

```bash
# Configure AWS CLI to use local MinIO
export AWS_ACCESS_KEY_ID=minioadmin
export AWS_SECRET_ACCESS_KEY=minioadmin
export AWS_ENDPOINT_URL=http://localhost:9000

# Create bucket with AWS CLI
aws s3 mb s3://dev-bucket --endpoint-url http://localhost:9000

# Upload files for testing
aws s3 cp test-data/ s3://dev-bucket/ --recursive --endpoint-url http://localhost:9000

# List objects
aws s3 ls s3://dev-bucket/ --endpoint-url http://localhost:9000
```

### Python Application Integration

Access MinIO from Python applications:

```python
from minio import Minio

# Connect to MinIO
client = Minio(
    "localhost:9000",
    access_key="minioadmin",
    secret_key="minioadmin",
    secure=False
)

# Create bucket
client.make_bucket("ml-models")

# Upload model file
client.fput_object("ml-models", "model.pkl", "/tmp/model.pkl")

# Generate presigned URL for sharing
url = client.get_presigned_download_url(
    "ml-models",
    "model.pkl",
    expires=timedelta(hours=24)
)
print(f"Download at: {url}")
```

### CI/CD Pipeline with Artifact Storage

```bash
# Store build artifacts in MinIO
mc mb "${CI_MINIO_ALIAS}/${CI_PIPELINE_ID}" || true
mc cp build/output/* "${CI_MINIO_ALIAS}/${CI_PIPELINE_ID}/"

# Retrieve artifacts for deployment
mc cp --recursive "${CI_MINIO_ALIAS}/${CI_PIPELINE_ID}/" ./artifacts/
```

## Agent Use

- **Data pipeline testing**: Store and retrieve test datasets during automation workflows
- **Model versioning**: Archive ML models and training data with timestamped buckets
- **Log aggregation**: Centralize application and system logs from distributed systems
- **Backup automation**: Store database backups and snapshots for disaster recovery
- **Multi-tenant deployments**: Create isolated buckets for different tenants/environments

## Troubleshooting

### Service fails to start

Check logs and disk space:

```bash
# Linux: View systemd logs
sudo journalctl -u minio -n 50 -f

# macOS: View launchd logs
log stream --predicate 'process == "minio"' --level debug

# Check data directory exists and has write permissions
ls -la /var/lib/minio
```

### Permission errors

MinIO requires appropriate directory permissions:

```bash
# Ensure user owns data directory
sudo chown minio:minio /var/lib/minio
sudo chmod 0755 /var/lib/minio
```

### Cannot access web console

Verify ports are available:

```bash
# Check if ports are in use
lsof -i :9000
lsof -i :9001

# Use custom ports in configuration
```

### Bucket operations fail

Verify credentials and access:

```bash
# Test S3 API connectivity
mc alias set test http://localhost:9000 minioadmin minioadmin --api S3v4

# Create test bucket
mc mb test/verify-test

# List to confirm
mc ls test/
```

## Uninstall

```yaml
- preset: minio
  with:
    state: absent
```

**Note:** Data directory (`/var/lib/minio`) is preserved after uninstall to prevent accidental data loss. Remove manually if no longer needed.

## Resources

- Official docs: https://docs.min.io/
- GitHub: https://github.com/minio/minio
- Client library: https://min.io/docs/minio/linux/developers/minio-client.html
- Search: "MinIO deployment", "MinIO S3 tutorial", "MinIO Kubernetes"
