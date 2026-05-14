# crossplane - Universal Control Plane

Crossplane is a framework for building cloud-native control planes without needing to write code, transforming Kubernetes into a universal control plane.

## Quick Start
```yaml
- preset: crossplane
```

## Features
- **Infrastructure as Code**: Define cloud resources as K8s manifests
- **Provider ecosystem**: AWS, Azure, GCP, and 80+ providers
- **Composition**: Build platforms from reusable components
- **GitOps ready**: Declarative infrastructure management
- **Self-service**: Enable teams to provision their own infrastructure
- **Policy enforcement**: OPA, Kyverno integration

## Basic Usage
```bash
# Install Crossplane on Kubernetes
kubectl create namespace crossplane-system
helm repo add crossplane-stable https://charts.crossplane.io/stable
helm install crossplane crossplane-stable/crossplane --namespace crossplane-system

# Install kubectl plugin
curl -sL https://raw.githubusercontent.com/crossplane/crossplane/master/install.sh | sh

# Install AWS provider
kubectl crossplane install provider crossplane/provider-aws:latest

# Check provider status
kubectl get providers

# Create cloud resources
kubectl apply -f my-s3-bucket.yaml
```

## Advanced Configuration
```yaml
- preset: crossplane
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove crossplane CLI |

## Platform Support
- ✅ Linux (binary download, package managers)
- ✅ macOS (Homebrew, binary download)
- ❌ Windows (not yet supported)

## Configuration
- **Namespace**: `crossplane-system` (default for Crossplane)
- **CRDs**: Installed automatically with providers
- **Providers**: Installed as Kubernetes packages
- **Compositions**: Define platform abstractions

## Real-World Examples

### Install AWS Provider
```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-aws
spec:
  package: xpkg.upbound.io/crossplane-contrib/provider-aws:v0.40.0
```

### Configure AWS Credentials
```yaml
apiVersion: aws.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: default
spec:
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: aws-creds
      key: creds
```

```bash
# Create secret from AWS credentials
kubectl create secret generic aws-creds \
  -n crossplane-system \
  --from-file=creds=./aws-credentials.txt

# aws-credentials.txt format:
# [default]
# aws_access_key_id = AKIAIOSFODNN7EXAMPLE
# aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

### Create S3 Bucket
```yaml
apiVersion: s3.aws.crossplane.io/v1beta1
kind: Bucket
metadata:
  name: my-crossplane-bucket
spec:
  forProvider:
    region: us-east-1
    acl: private
    versioning:
      status: Enabled
    serverSideEncryptionConfiguration:
      rules:
        - applyServerSideEncryptionByDefault:
            sseAlgorithm: AES256
  providerConfigRef:
    name: default
```

### Create RDS Instance
```yaml
apiVersion: rds.aws.crossplane.io/v1alpha1
kind: DBInstance
metadata:
  name: my-postgres-db
spec:
  forProvider:
    region: us-east-1
    dbInstanceClass: db.t3.micro
    engine: postgres
    engineVersion: "13.7"
    masterUsername: admin
    allocatedStorage: 20
    storageType: gp2
    publiclyAccessible: false
    masterUserPasswordSecretRef:
      namespace: default
      name: db-password
      key: password
  providerConfigRef:
    name: default
  writeConnectionSecretToRef:
    namespace: default
    name: db-connection
```

### Composition (Platform Abstraction)
```yaml
# Define CompositeResourceDefinition
apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: xdatabases.example.com
spec:
  group: example.com
  names:
    kind: XDatabase
    plural: xdatabases
  claimNames:
    kind: Database
    plural: databases
  versions:
  - name: v1alpha1
    served: true
    referenceable: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              size:
                type: string
                enum: [small, medium, large]
            required: [size]

---
# Define Composition
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: aws-postgres
spec:
  compositeTypeRef:
    apiVersion: example.com/v1alpha1
    kind: XDatabase
  resources:
  - name: rds-instance
    base:
      apiVersion: rds.aws.crossplane.io/v1alpha1
      kind: DBInstance
      spec:
        forProvider:
          region: us-east-1
          engine: postgres
          engineVersion: "13.7"
    patches:
    - fromFieldPath: spec.size
      toFieldPath: spec.forProvider.dbInstanceClass
      transforms:
      - type: map
        map:
          small: db.t3.micro
          medium: db.t3.small
          large: db.t3.medium
```

### Claim Infrastructure
```yaml
# Developers request database with simple claim
apiVersion: example.com/v1alpha1
kind: Database
metadata:
  name: my-app-db
  namespace: app-team
spec:
  size: medium
  compositionSelector:
    matchLabels:
      provider: aws
```

### Multi-Cloud Setup
```yaml
# Install multiple providers
---
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-aws
spec:
  package: xpkg.upbound.io/crossplane-contrib/provider-aws:v0.40.0
---
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-gcp
spec:
  package: xpkg.upbound.io/crossplane-contrib/provider-gcp:v0.22.0
---
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-azure
spec:
  package: xpkg.upbound.io/crossplane-contrib/provider-azure:v0.19.0
```

## Kubectl Plugin Commands
```bash
# Install provider
kubectl crossplane install provider <package>

# Install configuration package
kubectl crossplane install configuration <package>

# Update provider
kubectl crossplane update provider <name>

# Build configuration package
kubectl crossplane build configuration

# Push configuration to registry
kubectl crossplane push configuration <registry>/<org>/<name>:<tag>
```

## Monitoring
```bash
# Check Crossplane status
kubectl get pods -n crossplane-system

# View managed resources
kubectl get managed

# Check specific resource
kubectl describe bucket my-crossplane-bucket

# View events
kubectl get events -n crossplane-system --sort-by='.lastTimestamp'
```

## Agent Use
- Platform engineering and self-service infrastructure
- Multi-cloud resource provisioning
- GitOps-based infrastructure automation
- Standardized environment creation
- Developer-friendly infrastructure abstractions
- Policy-enforced resource management

## Troubleshooting

### Provider not ready
Check provider status:
```bash
# View provider details
kubectl get providers
kubectl describe provider provider-aws

# Check provider pod logs
kubectl logs -n crossplane-system -l pkg.crossplane.io/provider=provider-aws
```

### Resource stuck creating
Inspect resource status:
```bash
# Check resource conditions
kubectl describe bucket my-bucket

# View detailed status
kubectl get bucket my-bucket -o yaml

# Check provider credentials
kubectl get providerconfig default -o yaml
```

### Composition not working
Validate composition:
```bash
# Check XRD and Composition
kubectl get xrd
kubectl get composition

# View claim status
kubectl describe database my-app-db

# Check composite resource
kubectl get composite
```

## Uninstall
```yaml
- preset: crossplane
  with:
    state: absent
```

To remove from Kubernetes:
```bash
# Delete all managed resources first
kubectl delete managed --all

# Uninstall providers
kubectl delete provider --all

# Uninstall Crossplane
helm uninstall crossplane -n crossplane-system
kubectl delete namespace crossplane-system
```

## Resources
- Official docs: https://docs.crossplane.io/
- Provider docs: https://marketplace.upbound.io/
- Compositions guide: https://docs.crossplane.io/latest/concepts/compositions/
- Search: "crossplane tutorial", "crossplane composition examples"
