# ctop - Container Metrics

Top-like interface for container metrics. Monitor Docker and Kubernetes containers.

## Quick Start
```yaml
- preset: ctop
```

## Usage
```bash
ctop                    # All containers
ctop -a                 # Include stopped
ctop -f "name=web"      # Filter by name
```

## Keys
- `s` - Sort
- `f` - Filter
- `Enter` - Container menu (logs, exec, stop)
- `q` - Quit

**Agent Use**: Monitor container resource usage, detect anomalies
