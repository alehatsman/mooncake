# syft - SBOM Generator

Generate Software Bill of Materials (SBOM) for container images, filesystems, and archives. Essential for supply chain transparency and vulnerability scanning.

## Quick Start
```yaml
- preset: syft
```

## Features
- **Multiple formats**: CycloneDX, SPDX, JSON, SARIF for industry-standard SBOMs
- **Comprehensive cataloging**: Detects packages from 40+ ecosystems (npm, pip, gem, go, cargo, maven, etc.)
- **Fast scanning**: Multi-threaded analysis of container images and filesystems
- **Deep inspection**: All layers, including squashed and distroless images
- **CI/CD friendly**: JSON/SARIF output for automated pipelines
- **Vulnerability feed**: Direct integration with Grype scanner
- **Cross-platform**: Linux, macOS, Windows support

## Basic Usage
```bash
# Scan container image
syft nginx:latest

# Scan filesystem
syft dir:.
syft dir:/path/to/project

# Scan archive
syft file:image.tar

# Specific registry
syft registry:ghcr.io/org/image:tag
```

## Advanced Configuration
```yaml
# Install syft (default)
- preset: syft

# Uninstall syft
- preset: syft
  with:
    state: absent
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ✅ Windows (scoop, choco)

## Output Formats
```bash
# CycloneDX (industry standard)
syft nginx:latest -o cyclonedx-json
syft nginx:latest -o cyclonedx-xml

# SPDX (Linux Foundation)
syft nginx:latest -o spdx-json
syft nginx:latest -o spdx-tag-value

# JSON (programmatic access)
syft nginx:latest -o json

# Table (human readable)
syft nginx:latest -o table

# Text (simple list)
syft nginx:latest -o text

# GitHub JSON (Dependency Graph)
syft nginx:latest -o github-json

# SARIF (security tools)
syft nginx:latest -o sarif
```

## Source Types
```bash
# Docker image
syft docker:nginx:latest
syft docker://nginx:latest

# Container registry
syft registry:ghcr.io/org/image:tag

# OCI layout
syft oci-dir:path/to/layout
syft oci-archive:image.tar

# Directory
syft dir:.
syft dir:/usr/local

# Container archive
syft docker-archive:image.tar

# Git repository
syft git:https://github.com/user/repo
```

## Cataloging Options
```bash
# Scan specific ecosystems
syft nginx:latest --catalogers pip,gem,npm

# List available catalogers
syft catalogers list

# Skip catalogers
syft nginx:latest --skip-catalogers java,python

# All layers (including intermediate)
syft --scope all-layers nginx:latest

# Squashed (final image only, default)
syft --scope squashed nginx:latest
```

## Configuration
- **Config file**: `.syft.yaml` in current directory or `~/.syft.yaml`
- **Cache**: `~/.cache/syft/` for downloaded images and databases
- **Output**: STDOUT by default, use `-o format=file` for file output

## Configuration File
```yaml
# .syft.yaml
output:
  - cyclonedx-json
  - spdx-json=sbom.spdx.json

scope: all-layers

catalogers:
  enabled:
    - python
    - javascript
    - go
    - ruby
    - java

exclude:
  - "**/test/**"
  - "**/node_modules/**"
  - "**/.git/**"

log:
  level: "warn"
```

## Real-World Examples

### CI/CD Pipeline
```yaml
# GitHub Actions
- name: Generate SBOM
  run: |
    syft ${{ env.IMAGE }}:${{ github.sha }} -o cyclonedx-json > sbom.json

- name: Upload SBOM
  uses: actions/upload-artifact@v3
  with:
    name: sbom
    path: sbom.json
```

### GitLab CI
```yaml
sbom:
  image: anchore/syft:latest
  script:
    - syft ${CI_REGISTRY_IMAGE}:${CI_COMMIT_SHA} -o cyclonedx-json > sbom.json
  artifacts:
    paths:
      - sbom.json
```

### Docker Build Integration
```bash
# Generate SBOM during build
docker build -t myapp:latest .
syft myapp:latest -o cyclonedx-json > sbom.json

# Attach SBOM to image
cosign attach sbom --sbom sbom.json myapp:latest
```

## Filtering and Analysis
```bash
# Filter by package type
syft nginx:latest -o json | jq '.artifacts[] | select(.type=="rpm")'

# Count packages
syft nginx:latest -o json | jq '.artifacts | length'

# Find specific package
syft nginx:latest -o json | jq '.artifacts[] | select(.name=="openssl")'

# List unique types
syft nginx:latest -o json | jq '[.artifacts[].type] | unique'

# Extract licenses
syft nginx:latest -o json | jq -r '.artifacts[].licenses[]' | sort -u
```

## Package Types Detected
- **OS packages**: rpm, deb, apk, portage, pacman
- **JavaScript**: npm, yarn, pnpm
- **Python**: pip, poetry, pipenv
- **Ruby**: gem, bundler
- **Go**: go.mod, go.sum
- **Rust**: cargo
- **Java**: maven, gradle, jar
- **PHP**: composer
- **.NET**: nuget
- **C/C++**: conan, vcpkg

## Version Comparison
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

# Find removed packages
comm -23 <(jq -r '.artifacts[].name' v1.json | sort) \
         <(jq -r '.artifacts[].name' v2.json | sort)
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

# Packages without licenses
syft nginx:latest -o json | \
  jq '.artifacts[] | select(.licenses | length == 0)'
```

## Integration with Grype
```bash
# Generate SBOM and scan for vulnerabilities
syft nginx:latest -o json > sbom.json
grype sbom:sbom.json

# Direct pipeline
syft nginx:latest -o json | grype

# SBOM + vulnerability report
syft nginx:latest -o cyclonedx-json > sbom.json
grype sbom:sbom.json -o sarif > vulnerabilities.sarif
```

## Attestation with Cosign
```bash
# Generate SBOM
syft nginx:latest -o spdx-json > sbom.json

# Sign with Cosign
cosign attest --predicate sbom.json --type spdx nginx:latest

# Verify attestation
cosign verify-attestation --type spdx nginx:latest
```

## Multi-Platform Images
```bash
# Scan specific platform
syft --platform linux/amd64 nginx:latest
syft --platform linux/arm64 nginx:latest

# Scan all platforms
for platform in linux/amd64 linux/arm64; do
  echo "Platform: $platform"
  syft --platform $platform nginx:latest -o table
done
```

## Advanced Options
```bash
# Custom output file
syft nginx:latest -o cyclonedx-json=sbom.json

# Quiet mode (no progress)
syft -q nginx:latest -o json

# Verbose logging
syft -v nginx:latest

# Debug output
syft -vv nginx:latest

# Multiple outputs
syft nginx:latest -o cyclonedx-json -o spdx-json=sbom.spdx.json
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
if syft nginx:latest -o json | jq '.artifacts[] | select(.licenses[] | contains("GPL"))' | grep -q GPL; then
  echo "WARNING: GPL-licensed packages detected"
fi
```

## Best Practices
- **Generate during build**: Create SBOMs as part of CI/CD pipeline
- **Store with releases**: Archive SBOMs alongside container images
- **Sign SBOMs**: Use Cosign to attest authenticity
- **Automate scanning**: Integrate with vulnerability scanners like Grype
- **Track changes**: Compare SBOMs between versions
- **Use standard formats**: CycloneDX or SPDX for interoperability

## Tips
- Use CycloneDX for modern tooling and vulnerability scanners
- SPDX for compliance and legal requirements
- JSON for programmatic access and filtering
- Include SBOM generation in CI/CD pipelines
- Store SBOMs with container tags or git releases
- Feed SBOMs to security gates and compliance tools

## Agent Use
- Automated SBOM generation in CI/CD
- Supply chain transparency and tracking
- License compliance verification
- Dependency inventory management
- Vulnerability analysis preparation
- Security gate integration

## Uninstall
```yaml
- preset: syft
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/anchore/syft
- Docs: https://github.com/anchore/syft#readme
- CycloneDX: https://cyclonedx.org/
- SPDX: https://spdx.dev/
- Search: "syft sbom examples", "cyclonedx sbom", "spdx sbom"
