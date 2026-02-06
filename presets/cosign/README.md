# cosign - Container Signing

Sign and verify container images. Part of Sigstore project for supply chain security.

## Quick Start
```yaml
- preset: cosign
```

## Features
- **Container Image Signing**: Sign OCI container images with cryptographic signatures
- **Keyless Signing**: OIDC-based signing without managing private keys
- **Attestation Support**: Create and verify SLSA provenance and SBOM attestations
- **Multiple Key Backends**: Support for local keys, KMS (Google, AWS, Azure, HashiCorp)
- **Policy Enforcement**: Verify signatures before deployment with admission controllers
- **Transparency Logs**: Automatic logging to Rekor for tamper-proof audit trails
- **Multi-Platform**: Works with any OCI registry (Docker Hub, GCR, ECR, ACR)

## Basic Usage
```bash
# Generate key pair
cosign generate-key-pair

# Sign container image
cosign sign --key cosign.key registry.io/image:tag

# Verify signature
cosign verify --key cosign.pub registry.io/image:tag

# Sign with keyless (OIDC)
COSIGN_EXPERIMENTAL=1 cosign sign registry.io/image:tag

# Create attestation
cosign attest --key cosign.key --predicate attestation.json registry.io/image:tag

# Verify attestation
cosign verify-attestation --key cosign.pub registry.io/image:tag
```

## Key Management
```bash
# Generate key pair
cosign generate-key-pair
# Creates: cosign.key (private), cosign.pub (public)

# Store in environment
export COSIGN_PASSWORD=your-password
export COSIGN_KEY=path/to/cosign.key

# Generate with KMS (Google/AWS/Azure/HashiCorp)
cosign generate-key-pair --kms gcpkms://projects/PROJECT/locations/LOCATION/keyRings/RING/cryptoKeys/KEY
```

## Signing Images
```bash
# Sign with key
cosign sign --key cosign.key registry.io/image:tag

# Sign all tags
cosign sign --key cosign.key -a registry.io/image

# Sign with annotations
cosign sign --key cosign.key -a build=1234 -a git-sha=abc123 registry.io/image:tag

# Keyless signing (OIDC)
cosign sign registry.io/image:tag
# Opens browser for OIDC authentication

# Sign with specific identity
COSIGN_EXPERIMENTAL=1 cosign sign --oidc-issuer=https://token.actions.githubusercontent.com registry.io/image:tag
```

## Verification
```bash
# Verify with public key
cosign verify --key cosign.pub registry.io/image:tag

# Verify keyless signature
COSIGN_EXPERIMENTAL=1 cosign verify registry.io/image:tag

# Verify with certificate
cosign verify --certificate cert.pem --certificate-chain chain.pem registry.io/image:tag

# Verify annotations
cosign verify --key cosign.pub -a build=1234 registry.io/image:tag

# JSON output
cosign verify --key cosign.pub --output=json registry.io/image:tag
```

## Attestations
```bash
# Create SLSA attestation
cosign attest --key cosign.key --predicate=attestation.json --type slsaprovenance registry.io/image:tag

# Verify attestation
cosign verify-attestation --key cosign.pub --type slsaprovenance registry.io/image:tag

# Sign SBOM
syft registry.io/image:tag -o json > sbom.json
cosign attest --key cosign.key --predicate sbom.json --type spdx registry.io/image:tag

# Verify SBOM
cosign verify-attestation --key cosign.pub --type spdx registry.io/image:tag
```

## CI/CD Integration
```bash
# GitHub Actions
- name: Sign image
  env:
    COSIGN_PASSWORD: ${{ secrets.COSIGN_PASSWORD }}
  run: |
    echo "${{ secrets.COSIGN_KEY }}" > cosign.key
    cosign sign --key cosign.key ${{ env.IMAGE }}:${{ github.sha }}

# GitLab CI
sign:
  script:
    - echo "$COSIGN_KEY" > cosign.key
    - cosign sign --key cosign.key $CI_REGISTRY_IMAGE:$CI_COMMIT_SHA

# Keyless (no secrets needed)
- name: Sign with keyless
  run: |
    COSIGN_EXPERIMENTAL=1 cosign sign $IMAGE:$TAG
```

## Policy Enforcement
```bash
# Verify before deploy
if cosign verify --key cosign.pub registry.io/image:tag; then
  kubectl apply -f deployment.yaml
else
  echo "Image not signed or verification failed"
  exit 1
fi

# Check signature exists
cosign triangulate registry.io/image:tag
```

## Multi-Platform Images
```bash
# Sign all platforms
cosign sign --key cosign.key --recursive registry.io/multi-arch:tag

# Verify specific platform
cosign verify --key cosign.pub --platform=linux/amd64 registry.io/multi-arch:tag
```

## Advanced Usage
```bash
# Sign with timestamp
cosign sign --key cosign.key --timestamp-server=http://timestamp.digicert.com registry.io/image:tag

# Copy signatures
cosign copy source.io/image:tag dest.io/image:tag

# Clean signatures
cosign clean registry.io/image:tag

# Upload signature to transparency log
cosign sign --key cosign.key --upload=true registry.io/image:tag
```

## Best Practices
- **Protect private keys**: Use KMS or vault
- **Automate verification**: In admission controllers
- **Use keyless**: For public images
- **Attest builds**: Include SLSA provenance
- **Timestamp signatures**: Prove signing time
- **Verify in pipelines**: Before deployment

## Admission Control
```yaml
# Example policy (Kyverno/OPA)
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: verify-images
spec:
  rules:
  - name: verify-signature
    match:
      resources:
        kinds:
        - Pod
    verifyImages:
    - image: "registry.io/*"
      key: |-
        -----BEGIN PUBLIC KEY-----
        ...
        -----END PUBLIC KEY-----
```

## Tips
- Signatures stored as OCI artifacts
- Works with any OCI registry
- Keyless uses Fulcio CA
- Transparency via Rekor log
- Compatible with Docker/Podman

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Automated image signing in CI/CD
- Policy enforcement gates
- Supply chain security
- Build provenance tracking


## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install cosign
  preset: cosign

- name: Use cosign in automation
  shell: |
    # Custom configuration here
    echo "cosign configured"
```
## Uninstall
```yaml
- preset: cosign
  with:
    state: absent
```

## Resources
- Docs: https://docs.sigstore.dev/cosign/overview/
- GitHub: https://github.com/sigstore/cosign
