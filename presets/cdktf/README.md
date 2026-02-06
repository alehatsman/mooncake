# CDK for Terraform - CDKTF

Define Terraform infrastructure using TypeScript, Python, Java, Go, or C# instead of HCL.

## Quick Start
```yaml
- preset: cdktf
```

## Features
- **Familiar languages**: Use TypeScript, Python, Java, Go, C#
- **Type safety**: Compile-time validation
- **Terraform ecosystem**: Access all Terraform providers
- **Code reuse**: Create and share constructs
- **Testing**: Unit test infrastructure code
- **IDE support**: Autocomplete and refactoring

## Basic Usage
```bash
# Initialize project
cdktf init --template typescript --local

# Get provider types
cdktf get

# Synthesize Terraform JSON
cdktf synth

# Deploy infrastructure
cdktf deploy

# Destroy infrastructure
cdktf destroy

# Show diff
cdktf diff
```

## Example Stack
```typescript
// main.ts
import { Construct } from "constructs";
import { App, TerraformStack } from "cdktf";
import { AwsProvider } from "./.gen/providers/aws/provider";
import { Instance } from "./.gen/providers/aws/instance";

class MyStack extends TerraformStack {
  constructor(scope: Construct, name: string) {
    super(scope, name);

    new AwsProvider(this, "aws", {
      region: "us-east-1",
    });

    new Instance(this, "compute", {
      ami: "ami-01456a894f71116f2",
      instanceType: "t2.micro",
    });
  }
}

const app = new App();
new MyStack(app, "my-stack");
app.synth();
```

## Real-World Examples

### Infrastructure Deployment
```yaml
- name: Install CDKTF
  preset: cdktf

- name: Get providers
  shell: cdktf get
  cwd: /infrastructure

- name: Deploy stack
  shell: cdktf deploy --auto-approve
  cwd: /infrastructure
```

### Multi-environment Deploy
```yaml
- name: Deploy to staging
  shell: cdktf deploy staging --auto-approve
  cwd: /infra
  environment:
    TF_VAR_environment: staging

- name: Deploy to production
  shell: cdktf deploy production --auto-approve
  cwd: /infra
  environment:
    TF_VAR_environment: production
  when: branch == "main"
```

## Platform Support
- ✅ Linux (npm)
- ✅ macOS (npm)
- ✅ Windows (npm)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Define Terraform infrastructure in TypeScript/Python
- Leverage type safety for infrastructure code
- Reuse infrastructure components across projects
- Test infrastructure code with unit tests
- Integrate Terraform with existing CI/CD


## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install cdktf
  preset: cdktf

- name: Use cdktf in automation
  shell: |
    # Custom configuration here
    echo "cdktf configured"
```
## Uninstall
```yaml
- preset: cdktf
  with:
    state: absent
```

## Resources
- Official site: https://developer.hashicorp.com/terraform/cdktf
- Documentation: https://developer.hashicorp.com/terraform/cdktf/concepts
- Examples: https://github.com/hashicorp/terraform-cdk-examples
- Search: "cdktf tutorial", "terraform cdk", "cdktf typescript"
