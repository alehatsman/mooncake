# kubectl-argo-rollouts - Progressive Delivery

Advanced deployment strategies for Kubernetes. Canary, blue-green, analysis, traffic splitting, automated rollbacks.

## Quick Start
```yaml
- preset: argo-rollouts
```

## Basic Usage
```bash
# List rollouts
kubectl argo rollouts list rollouts

# Get rollout status
kubectl argo rollouts get rollout myapp

# Promote rollout
kubectl argo rollouts promote myapp

# Abort rollout
kubectl argo rollouts abort myapp

# Retry rollout
kubectl argo rollouts retry rollout myapp
```

## Viewing Status
```bash
# Get rollout details
kubectl argo rollouts get rollout myapp

# Watch rollout progress
kubectl argo rollouts get rollout myapp --watch

# Status of all rollouts
kubectl argo rollouts list rollouts -A

# YAML output
kubectl get rollout myapp -o yaml

# JSON output
kubectl get rollout myapp -o json
```

## Rollout Control
```bash
# Promote (advance to next step)
kubectl argo rollouts promote myapp

# Skip current step
kubectl argo rollouts promote myapp --skip-current-step

# Full promotion (skip all steps)
kubectl argo rollouts promote myapp --full

# Abort rollout
kubectl argo rollouts abort myapp

# Retry failed rollout
kubectl argo rollouts retry rollout myapp

# Pause rollout
kubectl argo rollouts pause myapp

# Resume rollout
kubectl argo rollouts resume myapp
```

## Rollout Restart
```bash
# Restart rollout
kubectl argo rollouts restart myapp

# Restart with new image
kubectl argo rollouts set image myapp mycontainer=myimage:v2
kubectl argo rollouts restart myapp
```

## Image Updates
```bash
# Set image
kubectl argo rollouts set image myapp \
  mycontainer=myimage:v1.2.3

# Set multiple images
kubectl argo rollouts set image myapp \
  container1=image1:tag1 \
  container2=image2:tag2

# Update with rollout restart
kubectl argo rollouts set image myapp app=myapp:v2 && \
  kubectl argo rollouts restart myapp
```

## Analysis
```bash
# List analysis runs
kubectl argo rollouts list experiments
kubectl argo rollouts list analysisruns

# Get analysis run
kubectl argo rollouts get analysisrun myanalysis

# Watch analysis
kubectl argo rollouts get analysisrun myanalysis --watch

# Terminate analysis
kubectl delete analysisrun myanalysis
```

## Dashboard
```bash
# Start dashboard
kubectl argo rollouts dashboard

# With port forward
kubectl argo rollouts dashboard -p 3100

# Access at http://localhost:3100
```

## Canary Deployment
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: myapp
spec:
  replicas: 5
  strategy:
    canary:
      steps:
      - setWeight: 20
      - pause: {duration: 5m}
      - setWeight: 40
      - pause: {duration: 5m}
      - setWeight: 60
      - pause: {duration: 5m}
      - setWeight: 80
      - pause: {duration: 5m}
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
      - name: myapp
        image: myapp:v1
```

## Blue-Green Deployment
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: myapp
spec:
  replicas: 3
  strategy:
    blueGreen:
      activeService: myapp-active
      previewService: myapp-preview
      autoPromotionEnabled: false
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
      - name: myapp
        image: myapp:v1
```

## Traffic Management
```bash
# With Istio
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: myapp
spec:
  strategy:
    canary:
      trafficRouting:
        istio:
          virtualService:
            name: myapp-vsvc
            routes:
            - primary
      steps:
      - setWeight: 10
      - pause: {duration: 1m}

# With NGINX Ingress
spec:
  strategy:
    canary:
      trafficRouting:
        nginx:
          stableIngress: myapp
      steps:
      - setWeight: 20
      - pause: {}

# With AWS ALB
spec:
  strategy:
    canary:
      trafficRouting:
        alb:
          ingress: myapp
          servicePort: 80
```

## Analysis and Metrics
```yaml
# AnalysisTemplate
apiVersion: argoproj.io/v1alpha1
kind: AnalysisTemplate
metadata:
  name: success-rate
spec:
  metrics:
  - name: success-rate
    interval: 1m
    successCondition: result >= 0.95
    provider:
      prometheus:
        address: http://prometheus:9090
        query: |
          sum(rate(http_requests_total{status="200"}[1m])) /
          sum(rate(http_requests_total[1m]))

# Use in Rollout
spec:
  strategy:
    canary:
      analysis:
        templates:
        - templateName: success-rate
        startingStep: 2
      steps:
      - setWeight: 20
      - pause: {duration: 5m}
```

## Automated Rollback
```bash
# Rollback on failed analysis
spec:
  strategy:
    canary:
      analysis:
        templates:
        - templateName: success-rate
        args:
        - name: service-name
          value: myapp
      steps:
      - setWeight: 20
      - pause: {duration: 1m}
      - setWeight: 40
      - pause: {duration: 1m}
      # Analysis runs, rollback if fails
```

## CI/CD Integration
```bash
# Update image in CI
kubectl argo rollouts set image myapp \
  app=myapp:$CI_COMMIT_SHA

# Wait for healthy
kubectl argo rollouts status myapp --watch --timeout 10m

# Promote if manual approval needed
kubectl argo rollouts promote myapp

# Check status
STATUS=$(kubectl argo rollouts status myapp -o json | jq -r .status.phase)
if [ "$STATUS" != "Healthy" ]; then
  echo "Rollout not healthy"
  exit 1
fi
```

## GitHub Actions Example
```yaml
- name: Deploy with Argo Rollouts
  run: |
    kubectl argo rollouts set image myapp \
      app=myapp:${{ github.sha }}

    kubectl argo rollouts status myapp --watch --timeout 600s

    # Auto-promote or wait for manual approval
    if [ "${{ inputs.auto_promote }}" == "true" ]; then
      kubectl argo rollouts promote myapp --full
    fi
```

## Notifications
```yaml
# With Argo CD notifications
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  annotations:
    notifications.argoproj.io/subscribe.on-rollout-step-completed.slack: my-channel
spec:
  # ... rollout spec
```

## Experiments
```yaml
# A/B testing
apiVersion: argoproj.io/v1alpha1
kind: Experiment
metadata:
  name: myapp-experiment
spec:
  duration: 1h
  templates:
  - name: baseline
    replicas: 1
    selector:
      matchLabels:
        app: myapp
        version: baseline
    template:
      # pod template
  - name: canary
    replicas: 1
    selector:
      matchLabels:
        app: myapp
        version: canary
    template:
      # pod template with new version
  analyses:
  - name: success-rate
    templateName: success-rate
```

## Best Practices
- **Use analysis** for automated decisions
- **Start with small steps** (10%, 20%, 50%)
- **Set appropriate pause durations**
- **Enable autoPromotionEnabled: false** for manual approval
- **Use traffic splitting** with service mesh
- **Monitor metrics** during rollout
- **Set timeout** on kubectl argo rollouts commands
- **Use experiments** for A/B testing

## Tips
- Works with any service mesh (Istio, Linkerd, etc.)
- Supports AWS ALB, NGINX, Traefik, SMI
- Metrics from Prometheus, Datadog, New Relic, etc.
- Automated rollback on failed analysis
- Manual approval gates with pause
- Progressive delivery without code changes
- Dashboard for visualization

## Agent Use
- Automated deployment pipelines
- Progressive rollout automation
- Canary deployment orchestration
- A/B testing workflows
- Automated rollback on failures
- Traffic-based deployments

## Uninstall
```yaml
- preset: argo-rollouts
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/argoproj/argo-rollouts
- Docs: https://argoproj.github.io/argo-rollouts/
- Search: "argo rollouts canary", "argo rollouts examples"
