# AWS CDK - Cloud Development Kit

Define cloud infrastructure using familiar programming languages like TypeScript, Python, Java, Go, or C#.

## Quick Start
```yaml
- preset: cdk
```

## Features
- **Infrastructure as Code**: Define AWS resources in code
- **Multiple languages**: TypeScript, Python, Java, Go, C#, JavaScript
- **Type safety**: Compile-time checks for infrastructure
- **Reusable components**: Create and share constructs
- **AWS best practices**: Built-in security and compliance
- **CloudFormation integration**: Synthesizes to CloudFormation

## Basic Usage
```bash
# Initialize new project
cdk init app --language typescript

# List stacks
cdk list

# Synthesize CloudFormation
cdk synth

# Deploy stack
cdk deploy

# Destroy stack
cdk destroy

# Diff against deployed
cdk diff
```

## Project Structure
```typescript
// lib/my-stack.ts
import * as cdk from 'aws-cdk-lib';
import * as s3 from 'aws-cdk-lib/aws-s3';

export class MyStack extends cdk.Stack {
  constructor(scope: cdk.App, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    new s3.Bucket(this, 'MyBucket', {
      versioned: true,
      encryption: s3.BucketEncryption.S3_MANAGED,
    });
  }
}
```

## Real-World Examples

### Deploy Infrastructure
```yaml
- name: Install CDK
  preset: cdk

- name: Bootstrap CDK
  shell: cdk bootstrap aws://ACCOUNT/REGION
  environment:
    AWS_PROFILE: production

- name: Deploy stacks
  shell: cdk deploy --all --require-approval never
  cwd: /infrastructure
```

### CI/CD Pipeline
```yaml
- name: Synthesize stack
  shell: cdk synth
  cwd: /app

- name: Run CDK diff
  shell: cdk diff
  cwd: /app
  register: diff

- name: Deploy if approved
  shell: cdk deploy --require-approval never
  cwd: /app
  when: deployment_approved
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
- Define AWS infrastructure as code
- Deploy cloud resources programmatically
- Manage multi-stack applications
- Implement infrastructure CI/CD
- Create reusable infrastructure components


## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install cdk
  preset: cdk

- name: Use cdk in automation
  shell: |
    # Custom configuration here
    echo "cdk configured"
```
## Uninstall
```yaml
- preset: cdk
  with:
    state: absent
```

## Resources
- Official site: https://aws.amazon.com/cdk/
- Documentation: https://docs.aws.amazon.com/cdk/
- Examples: https://github.com/aws-samples/aws-cdk-examples
- Search: "aws cdk tutorial", "cdk typescript", "aws cdk examples"
