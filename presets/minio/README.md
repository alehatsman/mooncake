# MinIO Preset

**Status:** ✓ Installed successfully

## Quick Start

```bash
# Access Console UI
open http://localhost:9001  # macOS
xdg-open http://localhost:9001  # Linux

# Default credentials (from installation output)
Username: minioadmin  # (or as specified)
Password: minioadmin  # (or as specified)

# S3 API endpoint
http://localhost:9000
```

## Configuration

- **Data directory:** `/var/lib/minio` (default)
- **Console UI port:** 9001 (default)
- **S3 API port:** 9000 (default)
- **Config file:** Set via environment variables

## MinIO Client (mc)

```bash
# Configure mc client
mc alias set myminio http://localhost:9000 minioadmin minioadmin

# Create bucket
mc mb myminio/mybucket

# List buckets
mc ls myminio/

# Upload file
mc cp file.txt myminio/mybucket/

# Download file
mc cp myminio/mybucket/file.txt .

# List objects
mc ls myminio/mybucket/

# Remove object
mc rm myminio/mybucket/file.txt
```

## AWS CLI Usage

```bash
# Configure AWS CLI to use MinIO
export AWS_ACCESS_KEY_ID=minioadmin
export AWS_SECRET_ACCESS_KEY=minioadmin
export AWS_ENDPOINT_URL=http://localhost:9000

# List buckets
aws s3 ls --endpoint-url http://localhost:9000

# Create bucket
aws s3 mb s3://mybucket --endpoint-url http://localhost:9000

# Upload file
aws s3 cp file.txt s3://mybucket/ --endpoint-url http://localhost:9000
```

## Python SDK Example

```python
from minio import Minio

client = Minio(
    "localhost:9000",
    access_key="minioadmin",
    secret_key="minioadmin",
    secure=False
)

# Create bucket
client.make_bucket("mybucket")

# Upload file
client.fput_object("mybucket", "myfile.txt", "/path/to/file.txt")

# Download file
client.fget_object("mybucket", "myfile.txt", "/path/to/save.txt")
```

## Common Operations

```bash
# Restart MinIO
sudo systemctl restart minio  # Linux
pkill minio && [start command]  # macOS

# Check server status
mc admin info myminio

# Create user
mc admin user add myminio newuser newpassword

# Create access policy
mc admin policy create myminio readonly readonly.json
```

## Uninstall

```yaml
- preset: minio
  with:
    state: absent
```

**Note:** Data directory is preserved after uninstall.
