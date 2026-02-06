# Argo Events - Event-Driven Automation

Event-driven workflow automation for Kubernetes. Trigger workflows from 20+ event sources including webhooks, S3, and messaging systems.

## Quick Start
```yaml
- preset: argo-events
```

## Features
- **20+ event sources**: Webhook, S3, Kafka, NATS, SNS, SQS, etc.
- **Event filtering**: Filter and transform events before triggering
- **Multiple triggers**: Workflow, Kafka, NATS, HTTP, Slack triggers
- **Event dependencies**: Combine multiple events with logic gates
- **Sensors**: Define event dependencies and triggers
- **Scalable**: Handle millions of events
- **Cloud-native**: Kubernetes CRD-based configuration

## Basic Usage
```bash
# Create event source (webhook)
kubectl apply -f event-source.yaml

# Create sensor (trigger)
kubectl apply -f sensor.yaml

# List event sources
kubectl get eventsources

# List sensors
kubectl get sensors

# View logs
kubectl logs -l eventsource-name=webhook
```

## Advanced Configuration
```yaml
- preset: argo-events
  with:
    state: present
  become: true
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Automated deployment and configuration
- Infrastructure as code workflows
- CI/CD pipeline integration
- Development environment setup
- Production service management

## Uninstall
```yaml
- preset: argo-events
  with:
    state: absent
```

## Resources
- Search: "argo-events documentation", "argo-events tutorial"
