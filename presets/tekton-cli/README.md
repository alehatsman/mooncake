# Tekton CLI (tkn) - Kubernetes-native CI/CD

Command-line tool for Tekton Pipelines. Create, run, and manage CI/CD pipelines on Kubernetes.

## Quick Start
```yaml
- preset: tekton-cli
```

## Features
- **Pipeline management**: Create and manage Tekton Pipelines
- **Task execution**: Run tasks and view logs in real-time
- **Resource inspection**: View pipeline runs, task runs, and logs
- **Interactive**: Start pipelines with parameter prompts
- **Kubernetes-native**: Works with existing Tekton installations
- **Plugin support**: Extend with custom commands

## Basic Usage
```bash
# Check version
tkn version

# List pipelines
tkn pipeline list

# Start pipeline
tkn pipeline start build-app

# List pipeline runs
tkn pipelinerun list

# View logs
tkn pipelinerun logs build-app-run-123 -f

# List tasks
tkn task list
```

## Pipelines

### List and Describe
```bash
# List all pipelines
tkn pipeline list

# List in namespace
tkn pipeline list -n dev

# Describe pipeline
tkn pipeline describe build-app

# Show YAML
tkn pipeline describe build-app -o yaml
```

### Start Pipeline
```bash
# Interactive start (prompts for parameters)
tkn pipeline start build-app

# With parameters
tkn pipeline start build-app \
  --param repo-url=https://github.com/example/app \
  --param image-tag=v1.0.0

# With workspace
tkn pipeline start build-app \
  --workspace name=source,claimName=source-pvc

# With service account
tkn pipeline start build-app \
  --serviceaccount=pipeline-sa

# Dry run (show YAML without creating)
tkn pipeline start build-app --dry-run
```

### Delete Pipeline
```bash
# Delete pipeline
tkn pipeline delete build-app

# Delete with runs
tkn pipeline delete build-app --pipelineruns
```

## Pipeline Runs

### List and Describe
```bash
# List pipeline runs
tkn pipelinerun list

# Show last 5 runs
tkn pipelinerun list --limit 5

# Describe run
tkn pipelinerun describe build-app-run-123

# Get YAML
tkn pipelinerun describe build-app-run-123 -o yaml
```

### Logs
```bash
# View logs
tkn pipelinerun logs build-app-run-123

# Follow logs (real-time)
tkn pipelinerun logs build-app-run-123 -f

# Show last pipeline run logs
tkn pipelinerun logs --last

# Show specific task logs
tkn pipelinerun logs build-app-run-123 -t build
```

### Cancel and Delete
```bash
# Cancel running pipeline
tkn pipelinerun cancel build-app-run-123

# Delete run
tkn pipelinerun delete build-app-run-123

# Delete all runs for pipeline
tkn pipelinerun delete --pipeline build-app

# Keep last N runs
tkn pipelinerun delete --keep 5 --pipeline build-app
```

## Tasks

### List and Describe
```bash
# List tasks
tkn task list

# Describe task
tkn task describe git-clone

# Show YAML
tkn task describe git-clone -o yaml
```

### Start Task
```bash
# Start task directly
tkn task start git-clone \
  --param url=https://github.com/example/repo \
  --param revision=main

# With workspace
tkn task start git-clone \
  --workspace name=output,claimName=source-pvc
```

### Task Runs
```bash
# List task runs
tkn taskrun list

# View logs
tkn taskrun logs git-clone-run-456 -f

# Delete task run
tkn taskrun delete git-clone-run-456
```

## Cluster Tasks

### Catalog Tasks
```bash
# List cluster tasks
tkn clustertask list

# Describe cluster task
tkn clustertask describe git-clone

# Start cluster task
tkn clustertask start git-clone \
  --param url=https://github.com/example/repo
```

## Resources

### Pipeline Resources (Deprecated)
```bash
# List resources
tkn resource list

# Describe resource
tkn resource describe git-source
```

## Triggers

### Event Listeners
```bash
# List event listeners
tkn eventlistener list

# Describe event listener
tkn eventlistener describe github-webhook

# View logs
tkn eventlistener logs github-webhook
```

### Trigger Bindings and Templates
```bash
# List trigger bindings
tkn triggerbinding list

# List trigger templates
tkn triggertemplate list

# Describe
tkn triggerbinding describe github-push
tkn triggertemplate describe build-pipeline
```

## CI/CD Integration

### GitHub Actions
```yaml
- name: Install Tekton CLI
  run: |
    curl -LO https://github.com/tektoncd/cli/releases/latest/download/tkn_0.33.0_Linux_x86_64.tar.gz
    tar xvzf tkn_0.33.0_Linux_x86_64.tar.gz
    sudo mv tkn /usr/local/bin/

- name: Start pipeline
  env:
    KUBECONFIG: ${{ secrets.KUBECONFIG }}
  run: |
    tkn pipeline start build-app \
      --param repo-url=${{ github.repository }} \
      --param commit-sha=${{ github.sha }} \
      --showlog
```

### GitLab CI
```yaml
deploy:
  image: gcr.io/tekton-releases/dogfooding/tkn
  script:
    - tkn pipeline start deploy \
        --param environment=production \
        --param version=$CI_COMMIT_TAG
  only:
    - tags
```

## Real-World Examples

### Build and Deploy Pipeline
```yaml
- name: Install tkn
  shell: |
    curl -LO https://github.com/tektoncd/cli/releases/latest/download/tkn_0.33.0_Linux_x86_64.tar.gz
    tar xvzf tkn_0.33.0_Linux_x86_64.tar.gz
    sudo mv tkn /usr/local/bin/
  become: true

- name: Start build pipeline
  shell: |
    tkn pipeline start build-app \
      --param git-url={{ repo_url }} \
      --param git-revision={{ git_branch }} \
      --param image-tag={{ image_tag }} \
      --workspace name=source,claimName=build-pvc \
      --serviceaccount=pipeline-runner \
      --showlog
  register: pipeline_run

- name: Wait for pipeline completion
  shell: |
    tkn pipelinerun logs {{ pipeline_run.stdout | regex_search('PipelineRun started: (.+)', '\\1') | first }} -f
```

### Automated Testing
```yaml
- name: Run tests via Tekton
  shell: |
    RUN_ID=$(tkn pipeline start test-suite \
      --param test-type=integration \
      --param parallel=true \
      --output name)

    tkn pipelinerun logs $RUN_ID -f

    STATUS=$(tkn pipelinerun describe $RUN_ID -o jsonpath='{.status.conditions[0].reason}')

    if [ "$STATUS" != "Succeeded" ]; then
      echo "Tests failed"
      exit 1
    fi
```

### Cleanup Old Runs
```yaml
- name: Cleanup old pipeline runs
  shell: |
    # Keep last 10 runs per pipeline
    for pipeline in $(tkn pipeline list -o name); do
      tkn pipelinerun delete --pipeline $pipeline --keep 10 -f
    done

    # Delete failed runs older than 7 days
    tkn pipelinerun delete --all \
      --label tekton.dev/pipeline=build-app \
      --older-than 168h \
      --status failed -f
```

## Output Formats

### JSON
```bash
# Pipeline as JSON
tkn pipeline describe build-app -o json

# Pipeline runs as JSON
tkn pipelinerun list -o json

# Extract specific fields
tkn pipelinerun list -o jsonpath='{.items[*].metadata.name}'
```

### YAML
```bash
# Pipeline as YAML
tkn pipeline describe build-app -o yaml

# Use with kubectl
tkn pipeline describe build-app -o yaml | kubectl apply -f -
```

### Name Only
```bash
# List names only
tkn pipeline list -o name
tkn pipelinerun list -o name

# Use in scripts
for run in $(tkn pipelinerun list -o name --limit 10); do
  tkn pipelinerun describe $run
done
```

## Hub Integration

### Tekton Hub
```bash
# Search hub for tasks
tkn hub search git

# Get task info
tkn hub info task git-clone

# Install task from hub
tkn hub install task git-clone

# Install specific version
tkn hub install task git-clone --version 0.9
```

## Configuration

### Kubeconfig
```bash
# Use specific kubeconfig
tkn pipeline list --kubeconfig ~/.kube/prod-config

# Use specific context
tkn pipeline list --context prod-cluster

# Use specific namespace
tkn pipeline list -n production
```

### Plugins
```bash
# List plugins
tkn plugin list

# Example plugins:
# - tkn-pac (Pipelines as Code)
# - tkn-results (store results)
```

## Troubleshooting

### Debug Pipeline Failures
```bash
# View failed pipeline run
tkn pipelinerun describe build-app-run-123

# Check conditions
tkn pipelinerun describe build-app-run-123 -o jsonpath='{.status.conditions[*]}'

# View specific task logs
tkn pipelinerun logs build-app-run-123 -t failing-task

# Check events
kubectl get events --sort-by='.lastTimestamp' | grep build-app-run-123
```

### Common Issues
```bash
# Pipeline not starting
kubectl describe pipeline build-app
kubectl get serviceaccount pipeline-sa

# No logs appearing
tkn pipelinerun logs --last -f

# Permission errors
kubectl auth can-i create pipelineruns --as=system:serviceaccount:default:pipeline-sa
```

## Comparison with Alternatives
| Feature | Tekton CLI | Jenkins X | Argo CD |
|---------|-----------|-----------|---------|
| K8s-native | Yes | Yes | Yes |
| GitOps | Via triggers | Built-in | Built-in |
| CLI complexity | Low | Medium | Medium |
| Pipeline as Code | YAML | YAML | YAML/UI |
| Learning curve | Easy | Moderate | Moderate |

## Best Practices
- Use `--showlog` for synchronous pipeline starts
- Store pipeline definitions in git
- Use namespaces to organize pipelines
- Clean up old runs regularly with `--keep`
- Use service accounts with minimal permissions
- Add labels for filtering and cleanup
- Use workspaces for persistent data
- Monitor pipeline runs with `describe` before debugging

## Platform Support
- ✅ Linux (amd64, arm64)
- ✅ macOS (Intel, Apple Silicon)
- ✅ Windows (amd64)
- ✅ Requires Kubernetes cluster with Tekton installed

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove tool |

## Agent Use
- Automated CI/CD pipeline orchestration
- Pipeline run monitoring and management
- Resource cleanup automation
- Pipeline testing and validation
- Integration with other tools (Argo, Flux)
- Multi-cluster pipeline management

## Advanced Configuration
```yaml
- preset: tekton-cli
  with:
    state: present
```

## Uninstall
```yaml
- preset: tekton-cli
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/tektoncd/cli
- Documentation: https://tekton.dev/docs/cli/
- Tekton: https://tekton.dev/
- Hub: https://hub.tekton.dev/
- Search: "tekton pipeline", "tkn cli", "tekton tutorial"
