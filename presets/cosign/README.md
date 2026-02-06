# cosign - Container Signing

Sign and verify container images with Sigstore.

## Quick Start
```yaml
- preset: cosign
```

## Usage
```bash
# Generate keypair
cosign generate-key-pair

# Sign image
cosign sign --key cosign.key registry.local/image:tag

# Verify
cosign verify --key cosign.pub registry.local/image:tag

# Keyless signing
cosign sign registry.local/image:tag
```

**Agent Use**: Automated signing, supply chain security
