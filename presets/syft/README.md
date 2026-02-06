# syft - SBOM Generator

Generate Software Bill of Materials (SBOM) for container images, filesystems, and archives. Supply chain transparency.

## Quick Start
```yaml
- preset: syft
```

## Basic Usage
```bash
# Scan container image
syft nginx:latest
syft alpine:3.18

# Scan filesystem
syft dir:.
syft dir:/path/to/project

# Scan archive
syft file:///path/to/image.tar

# Scan specific path
syft docker:nginx:latest
```

## Output Formats
```bash
# JSON (default)
syft nginx:latest -o json

# CycloneDX (industry standard)
syft nginx:latest -o cyclonedx-json
syft nginx:latest -o cyclonedx-xml

# SPDX (Linux Foundation)
syft nginx:latest -o spdx-json
syft nginx:latest -o spdx-tag-value

# Table (human readable)
syft nginx:latest -o table

# Text (simple list)
syft nginx:latest -o text

# GitHub JSON
syft nginx:latest -o github-json
```

## Source Types
```bash
# Docker image
syft docker:nginx:latest
syft registry:ghcr.io/org/image:tag

# OCI layout directory
syft oci-dir:path/to/layout
syft oci-archive:image.tar

# Directory
syft dir:.
syft dir:/usr/local

# Container archive
syft docker-archive:image.tar
syft oci-archive:image.tar

# Git repository
syft git:https://github.com/user/repo
```

## Cataloging
```bash
# Scan specific ecosystems
syft nginx:latest --catalogers pip,gem,npm

# Available catalogers
syft catalogers list

# Skip catalogers
syft nginx:latest --skip-catalogers java,python

# Scan for secrets
syft --scope all-layers nginx:latest
```

## CI/CD Integration
```bash
# Generate SBOM in CI
syft nginx:latest -o cyclonedx-json > sbom.json

# GitHub Actions
- name: Generate SBOM
  run: |
    syft ${{ env.IMAGE }}:${{ github.sha }} -o spdx-json > sbom.json

- name: Upload SBOM
  uses: actions/upload-artifact@v3
  with:
    name: sbom
    path: sbom.json

# GitLab CI
sbom:
  script:
    - syft ${CI_REGISTRY_IMAGE}:${CI_COMMIT_SHA} -o cyclonedx-json > sbom.json
  artifacts:
    paths:
      - sbom.json
```

## Attestation Integration
```bash
# Generate SBOM
syft nginx:latest -o spdx-json > sbom.json

# Sign with cosign
cosign attest --predicate sbom.json --type spdx nginx:latest

# Verify
cosign verify-attestation --type spdx nginx:latest
```

## Filtering
```bash
# Filter by package type
syft nginx:latest -o json | jq '.artifacts[] | select(.type=="rpm")'

# Count packages
syft nginx:latest -o json | jq '.artifacts | length'

# Find specific package
syft nginx:latest -o json | jq '.artifacts[] | select(.name=="openssl")'

# List unique types
syft nginx:latest -o json | jq '[.artifacts[].type] | unique'
```

## Comparison
```bash
# Generate SBOMs for two versions
syft nginx:1.21 -o json > v1.json
syft nginx:1.22 -o json > v2.json

# Compare packages
diff <(jq -r '.artifacts[].name' v1.json | sort) \
     <(jq -r '.artifacts[].name' v2.json | sort)

# Find added packages
comm -13 <(jq -r '.artifacts[].name' v1.json | sort) \
         <(jq -r '.artifacts[].name' v2.json | sort)
```

## Package Types Detected
- **OS packages**: rpm, deb, apk
- **Language packages**: npm, pip, gem, go, cargo, maven
- **Archives**: jar, war, zip
- **Binary analysis**: executables, shared libraries

## Configuration
```yaml
# .syft.yaml
output:
  - cyclonedx-json
scope: all-layers
catalogers:
  enabled:
    - python
    - javascript
    - go
exclude:
  - "**/test/**"
  - "**/node_modules/**"
```

## Advanced Usage
```bash
# Multi-platform images
syft --platform linux/amd64 nginx:latest
syft --platform linux/arm64 nginx:latest

# Include layers info
syft --scope all-layers nginx:latest

# Custom output file
syft nginx:latest -o cyclonedx-json=sbom.json

# Quiet mode
syft -q nginx:latest -o json

# Verbose logging
syft -v nginx:latest
```

## Package Metadata
```json
{
  "artifacts": [{
    "name": "openssl",
    "version": "1.1.1",
    "type": "deb",
    "foundBy": "dpkgdb-cataloger",
    "locations": [{
      "path": "/var/lib/dpkg/status"
    }],
    "licenses": ["Apache-2.0"],
    "language": "",
    "cpes": [
      "cpe:2.3:a:openssl:openssl:1.1.1:*:*:*:*:*:*:*"
    ],
    "purl": "pkg:deb/debian/openssl@1.1.1"
  }]
}
```

## Compliance Workflows
```bash
# Generate CycloneDX for compliance
syft nginx:latest -o cyclonedx-json > sbom.json

# Validate required packages
if ! syft nginx:latest -o json | jq -e '.artifacts[] | select(.name=="openssl")' > /dev/null; then
  echo "ERROR: Required package openssl not found"
  exit 1
fi

# Check for GPL licenses
syft nginx:latest -o json | jq '.artifacts[] | select(.licenses[] | contains("GPL"))'
```

## License Analysis
```bash
# Extract all licenses
syft nginx:latest -o json | jq -r '.artifacts[].licenses[]' | sort -u

# Find GPL packages
syft nginx:latest -o json | \
  jq '.artifacts[] | select(.licenses[] | test("GPL"))'

# Count by license
syft nginx:latest -o json | \
  jq -r '.artifacts[].licenses[]' | sort | uniq -c | sort -rn
```

## Integration with Grype
```bash
# Generate SBOM
syft nginx:latest -o json > sbom.json

# Scan for vulnerabilities
grype sbom:sbom.json

# Combined workflow
syft nginx:latest -o json | grype
```

## Best Practices
- **Generate early**: Create SBOMs during build
- **Store artifacts**: Archive SBOMs with releases
- **Attest**: Sign SBOMs with cosign
- **Automate**: Generate in CI/CD pipelines
- **Compare**: Track changes between versions
- **Scan**: Feed to vulnerability scanners

## Tips
- Use CycloneDX for modern tooling
- SPDX for compliance/legal
- JSON for programmatic access
- Combine with grype for security
- Store SBOMs with container tags
- Include in security gates

## Agent Use
- Automated SBOM generation
- Supply chain transparency
- License compliance
- Dependency tracking
- Vulnerability analysis prep

## Uninstall
```yaml
- preset: syft
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/anchore/syft
- Docs: https://github.com/anchore/syft#readme
- Search: "syft sbom examples", "cyclonedx sbom"
