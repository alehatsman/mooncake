# argo - Workflow Engine for Kubernetes

Container-native workflow engine. Define complex workflows, parallel execution, DAGs, parameters, and artifacts.

## Quick Start
```yaml
- preset: argo-workflows
```

## Features
- **DAG workflows**: Define complex workflows with dependencies
- **Container-native**: Each step runs in a container
- **Parallel execution**: Run multiple steps concurrently
- **Artifacts**: Pass data between steps via S3/GCS/Artifactory
- **Cron scheduling**: Time-based workflow triggers
- **Event-driven**: Trigger workflows from events (with Argo Events)
- **Parameterization**: Template workflows with parameters
- **Retry strategies**: Automatic retries with backoff

## Basic Usage
```bash
# Submit workflow
argo submit workflow.yaml

# List workflows
argo list

# Get workflow status
argo get my-workflow

# View logs
argo logs my-workflow

# Delete workflow
argo delete my-workflow
```

## Submitting Workflows
```bash
# Submit from file
argo submit workflow.yaml

# Submit with parameters
argo submit workflow.yaml -p message="hello"

# Submit and watch
argo submit --watch workflow.yaml

# Submit with name
argo submit --name my-run workflow.yaml

# Dry run
argo submit --dry-run workflow.yaml

# From URL
argo submit https://raw.githubusercontent.com/org/repo/main/workflow.yaml
```

## Listing & Viewing
```bash
# List all workflows
argo list

# List with status filter
argo list --status Succeeded
argo list --status Running
argo list --status Failed

# Recent workflows
argo list --since 1h
argo list --since 24h

# JSON output
argo list -o json

# Wide output (more columns)
argo list -o wide
```

## Workflow Status
```bash
# Get workflow
argo get my-workflow

# YAML output
argo get my-workflow -o yaml

# JSON output
argo get my-workflow -o json

# Watch status
argo watch my-workflow

# Wait for completion
argo wait my-workflow
```

## Logs
```bash
# View logs
argo logs my-workflow

# Follow logs
argo logs -f my-workflow

# Specific step
argo logs my-workflow step-name

# All steps
argo logs my-workflow --no-color | less

# Previous run
argo logs my-workflow --previous
```

## Artifacts
```bash
# Download artifacts
argo artifact get my-workflow output-artifact

# Download all artifacts
argo artifact get my-workflow

# Download to directory
argo artifact get my-workflow output-artifact -o /tmp/artifacts/
```

## Workflow Management
```bash
# Stop workflow
argo stop my-workflow

# Terminate (force stop)
argo terminate my-workflow

# Retry failed workflow
argo retry my-workflow

# Resubmit workflow
argo resubmit my-workflow

# Resume suspended workflow
argo resume my-workflow

# Suspend running workflow
argo suspend my-workflow
```

## Deletion
```bash
# Delete workflow
argo delete my-workflow

# Delete all completed
argo delete --completed

# Delete older than
argo delete --older 7d

# Delete by label
argo delete -l app=myapp

# Force delete
argo delete my-workflow --force
```

## Templates
```bash
# List workflow templates
argo template list

# Get template
argo template get my-template

# Create template
argo template create template.yaml

# Submit from template
argo submit --from workflowtemplate/my-template

# With parameters
argo submit --from workflowtemplate/my-template \
  -p param1=value1 -p param2=value2
```

## Cron Workflows
```bash
# List cron workflows
argo cron list

# Get cron workflow
argo cron get my-cron

# Create cron workflow
argo cron create cron.yaml

# Suspend cron
argo cron suspend my-cron

# Resume cron
argo cron resume my-cron

# Delete cron
argo cron delete my-cron
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Parameters
```bash
# Submit with parameters
argo submit workflow.yaml \
  -p name=value \
  -p count=5

# Parameter file
argo submit workflow.yaml --parameter-file params.json

# View parameters
argo get my-workflow -o json | jq .spec.arguments.parameters
```

## CI/CD Integration
```bash
# Submit and wait
argo submit workflow.yaml --wait

# Check exit code
if argo submit --wait workflow.yaml; then
  echo "Workflow succeeded"
else
  echo "Workflow failed"
  exit 1
fi

# Get workflow status
STATUS=$(argo get my-workflow -o json | jq -r .status.phase)
if [ "$STATUS" != "Succeeded" ]; then
  echo "Workflow failed with status: $STATUS"
  exit 1
fi

# Submit with dynamic parameters
argo submit ci-workflow.yaml \
  -p git-sha=$CI_COMMIT_SHA \
  -p environment=production \
  --wait
```

## GitHub Actions Example
```yaml
- name: Run Argo Workflow
  run: |
    argo submit .argo/deploy.yaml \
      -p image-tag=${{ github.sha }} \
      -p environment=production \
      --wait \
      --log
```

## Example Workflows
```yaml
# Simple workflow
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: hello-world-
spec:
  entrypoint: whalesay
  templates:
  - name: whalesay
    container:
      image: docker/whalesay
      command: [cowsay]
      args: ["hello world"]

# DAG workflow
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: dag-
spec:
  entrypoint: diamond
  templates:
  - name: diamond
    dag:
      tasks:
      - name: A
        template: echo
      - name: B
        dependencies: [A]
        template: echo
      - name: C
        dependencies: [A]
        template: echo
      - name: D
        dependencies: [B, C]
        template: echo
  - name: echo
    container:
      image: alpine:3.7
      command: [echo]
      args: ["{{tasks.A.outputs.result}}"]

# With artifacts
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: artifact-passing-
spec:
  entrypoint: main
  templates:
  - name: main
    steps:
    - - name: generate
        template: generate-artifact
    - - name: consume
        template: consume-artifact
        arguments:
          artifacts:
          - name: message
            from: "{{steps.generate.outputs.artifacts.message}}"
```

## Debugging
```bash
# Verbose logs
argo logs -f my-workflow --timestamps

# View workflow events
kubectl get events --field-selector involvedObject.name=my-workflow

# Describe workflow
kubectl describe workflow my-workflow

# View pod logs directly
kubectl logs my-workflow-xxxxx

# Debug failed step
argo get my-workflow -o yaml | grep -A 20 failed
```

## Resource Management
```bash
# Archive workflow
argo archive my-workflow

# List archived
argo archive list

# Resubmit archived
argo archive resubmit uid-xxx

# Delete archived
argo archive delete uid-xxx

# Archive all completed
argo archive --completed
```

## Best Practices
- **Use WorkflowTemplates** for reusable workflows
- **Set resource limits** on containers
- **Use artifacts** for data passing between steps
- **Enable archiving** for audit trail
- **Use parameters** for flexibility
- **Set retry policies** for transient failures
- **Use DAGs** for complex dependencies
- **Enable workflow GC** (garbage collection)

## Tips
- Workflows run as Kubernetes CRDs
- Supports parallel execution
- Built-in artifact management (S3, GCS, etc.)
- Suspend/resume for human approval
- Works with any container
- Event-driven triggers
- Great for ML pipelines

## Agent Use
- CI/CD pipeline automation
- Data processing workflows
- ML training pipelines
- ETL job orchestration
- Testing automation
- Deployment workflows

## Uninstall
```yaml
- preset: argo-workflows
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/argoproj/argo-workflows
- Docs: https://argoproj.github.io/argo-workflows/
- Search: "argo workflows examples", "argo dag"
