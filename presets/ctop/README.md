# ctop - Container Metrics Monitor

Top-like interface for container metrics. Real-time CPU, memory, network, and I/O monitoring for Docker and Kubernetes.

## Quick Start
```yaml
- preset: ctop
```

## Basic Usage
```bash
# Monitor all running containers
ctop

# Include stopped containers
ctop -a

# Compact view
ctop -compact

# Refresh interval
ctop -i 3  # 3 second refresh
```

## Filtering
```bash
# Filter by name
ctop -f "name=web"

# Filter by state
ctop -f "state=running"

# Multiple filters
ctop -f "name=api,state=running"

# Exclude containers
ctop -f "name!=test"

# Interactive filter (press 'f' in ctop)
# Then type: name=nginx
```

## Keyboard Shortcuts
```
Navigation:
  ↑/↓     - Move selection up/down
  PgUp/PgDn - Move page up/down
  Home/End - First/last container

Actions:
  Enter   - Container menu
  s       - Sort menu
  f       - Filter
  a       - Toggle all/running
  h       - Help
  r       - Reset stats
  q       - Quit

Container Menu (press Enter):
  l       - View logs
  e       - Exec shell
  s       - Stop container
  r       - Restart
  p       - Pause/unpause
  o       - Output logs to file
```

## Display Information
```
Columns shown:
- Name        Container name
- CID         Container ID (short)
- CPU         CPU usage %
- MEM         Memory usage (MB / GB)
- MEM %       Memory usage %
- NET RX/TX   Network received/transmitted
- IO R/W      Disk read/write
- PIDS        Number of processes
- UPTIME      Running duration
```

## Sorting
```bash
# Sort options (press 's' in ctop):
- Container name
- CPU usage
- Memory usage
- Memory percentage
- Network RX
- Network TX
- Block I/O
- PIDs
```

## Container Menu
```bash
# Press Enter on a container, then:

# View logs (real-time)
l

# Execute shell
e
# Drops into /bin/sh inside container

# Stop container
s

# Restart container
r

# Pause/unpause
p

# Save logs to file
o
# Saves to ./logs/container-name.log
```

## CI/CD Integration
```bash
# Check resource usage
ctop -compact | grep high-usage-container

# Alert on high CPU
ctop -i 1 | awk '/myapp/ && $3 > 80 {print "High CPU:", $3"%"; system("notify-admin")}'

# Export metrics
ctop -compact > /tmp/container-metrics.txt
```

## Monitoring Scenarios
```bash
# Watch specific service
ctop -f "name=myapp"

# Monitor all web containers
ctop -f "name=web"

# Find resource hogs
# Press 's' then select "CPU" to sort by CPU usage
# Press 's' then select "Memory" to sort by memory

# Check network activity
# Press 's' then select "NET RX" for download traffic
# Press 's' then select "NET TX" for upload traffic

# Identify memory leaks
# Watch MEM column over time for increasing values
```

## Docker Integration
```bash
# Works automatically with Docker socket
# Default: /var/run/docker.sock

# Custom Docker socket
DOCKER_HOST=tcp://remote-docker:2375 ctop

# Docker context support
docker context use production
ctop  # Uses production context
```

## Kubernetes Support
```bash
# Connect to K8s containers
# ctop automatically detects K8s containers via Docker

# Filter by namespace (via naming)
ctop -f "name=production"

# Use with kubectl port-forward
kubectl port-forward deployment/myapp 8080:8080 &
ctop -f "name=myapp"
```

## Performance Monitoring
```bash
# Baseline resources
ctop -compact > baseline.txt

# After load test
ctop -compact > under-load.txt

# Compare
diff baseline.txt under-load.txt

# Continuous monitoring
watch -n 5 'ctop -compact | head -20'

# Resource trends
while true; do
  echo "=== $(date) ===" >> metrics.log
  ctop -compact | grep myapp >> metrics.log
  sleep 60
done
```

## Troubleshooting
```bash
# Container using too much CPU
# 1. Press 's' to sort by CPU
# 2. Identify high CPU container
# 3. Press Enter, then 'l' for logs
# 4. Press 'e' to exec into container for debugging

# Memory leak detection
# 1. Press 's' to sort by Memory %
# 2. Watch MEM column increasing over time
# 3. Press Enter, then 'l' to check logs for errors

# Network bottleneck
# 1. Press 's' to sort by NET RX or NET TX
# 2. Identify high network container
# 3. Check if expected or anomaly

# Zombie processes
# 1. Check PIDS column
# 2. Unusually high PID count may indicate issue
# 3. Press Enter, then 'e' to investigate
```

## Comparison
| Feature | ctop | docker stats | htop | btop |
|---------|------|--------------|------|------|
| Container focus | Yes | Yes | No | No |
| Interactive | Yes | No | Yes | Yes |
| Filtering | Yes | No | Limited | Yes |
| Logs access | Yes | No | No | No |
| Exec shell | Yes | No | No | No |

## Advanced Usage
```bash
# Custom refresh rate
ctop -i 1  # 1 second (high CPU usage)
ctop -i 10 # 10 seconds (lower overhead)

# Scriptable output
ctop -compact | awk '{print $1, $3, $4}'  # Name, CPU, Memory

# Alert on threshold
#!/bin/bash
while true; do
  CPU=$(ctop -compact | grep myapp | awk '{print $3}' | tr -d '%')
  if [ $CPU -gt 80 ]; then
    echo "ALERT: myapp CPU at ${CPU}%"
    # Send alert
  fi
  sleep 30
done

# Export to monitoring system
ctop -compact | \
  awk '{print "container.cpu{name="$1"} "$3}' | \
  curl -X POST -d @- https://metrics.example.com
```

## Container Actions
```bash
# Quick restart
# 1. Navigate to container
# 2. Press Enter
# 3. Press 'r'

# Stop all containers matching filter
ctop -f "name=test"
# Then manually stop each with Enter -> 's'

# Collect logs before restart
# 1. Press Enter on container
# 2. Press 'o' to save logs
# 3. Press 'r' to restart
```

## Best Practices
- **Monitor during deployments** to catch resource issues
- **Set appropriate refresh interval** (`-i` flag)
- **Use filters** (`-f`) to focus on specific services
- **Sort by resource** to find bottlenecks quickly
- **Save logs** before stopping containers
- **Watch PIDS** for zombie process issues
- **Track network** for bandwidth problems

## Tips
- Press 'a' to toggle between running and all containers
- Use 'r' to reset statistics for fresh baseline
- 'h' shows help anytime
- Color coding indicates resource levels (green = ok, yellow = warning, red = critical)
- Works over SSH for remote monitoring
- Minimal performance overhead
- Great for quick health checks

## Agent Use
- Automated resource monitoring
- Anomaly detection in containers
- Capacity planning data
- Health check verification
- Resource usage trending
- Container lifecycle management

## Uninstall
```yaml
- preset: ctop
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/bcicen/ctop
- Search: "ctop docker monitoring", "ctop kubernetes"
