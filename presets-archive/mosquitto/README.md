# Mosquitto - Lightweight MQTT Message Broker

Open-source MQTT message broker ideal for IoT, edge computing, and real-time messaging scenarios with minimal resource footprint and high performance.

## Quick Start

```yaml
- preset: mosquitto
```

## Features

- **Lightweight**: Minimal resource consumption, runs on embedded systems
- **MQTT 3.1.1 & 5.0**: Full protocol support with modern features
- **High Performance**: Handles thousands of concurrent connections
- **Security**: TLS/SSL encryption, username/password authentication, ACL support
- **Bridging**: Connect multiple brokers for distributed deployments
- **Persistence**: Optional message persistence to disk
- **Cross-platform**: Linux, macOS, Windows support

## Basic Usage

```bash
# Start mosquitto with default configuration
mosquitto

# Start as daemon (background)
mosquitto -d

# Use specific config file
mosquitto -c /etc/mosquitto/mosquitto.conf

# Check version
mosquitto -v

# Run with verbose logging
mosquitto -v -v

# Listen on specific port
mosquitto -p 1883

# Use specific IP address
mosquitto -p 1883 -l 0.0.0.0
```

## Publish and Subscribe (after installation)

```bash
# Terminal 1: Subscribe to topic
mosquitto_sub -h localhost -t "sensors/temperature"

# Terminal 2: Publish message
mosquitto_pub -h localhost -t "sensors/temperature" -m "22.5"

# Subscribe to all topics under sensors/
mosquitto_sub -h localhost -t "sensors/#"

# Publish with QoS level
mosquitto_pub -h localhost -t "sensors/humidity" -m "65" -q 1

# Subscribe with username/password
mosquitto_sub -h mqtt.example.com -u admin -P password -t "alerts/#"
```

## Advanced Configuration

```yaml
- preset: mosquitto
  with:
    state: present
```

Note: Mosquitto is installed without service configuration by default. To enable as a system service or add custom configuration, see the Configuration section.

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install (present) or remove (absent) mosquitto |

## Platform Support

- ✅ Linux (apt, dnf, yum, pacman, zypper)
- ✅ macOS (Homebrew)
- ✅ Windows (with official installer or WSL)

## Configuration

- **Config file**: `/etc/mosquitto/mosquitto.conf` (Linux), `~/.mosquitto.conf` (user)
- **Data directory**: `/var/lib/mosquitto/` (Linux), `/var/log/mosquitto/` (logs)
- **Default port**: 1883 (MQTT), 8883 (MQTT over TLS)
- **Default address**: Listens on all interfaces (0.0.0.0)

## Real-World Examples

### IoT Sensor Network

```bash
# Device 1: Temperature sensor publishes every 30 seconds
while true; do
  temp=$(cat /sys/class/thermal/thermal_zone0/temp)
  mosquitto_pub -h mqtt.home.local -t "home/bedroom/temperature" -m "$((temp / 1000))"
  sleep 30
done

# Collector: Subscribe to all sensor data
mosquitto_sub -h mqtt.home.local -t "home/+/+" -v
```

### Smart Home Automation

```bash
# Dashboard publishes commands to devices
mosquitto_pub -h mqtt.local -t "home/living_room/lights" -m "on"

# Lights subscribe and respond
mosquitto_sub -h mqtt.local -t "home/living_room/lights" | while read msg; do
  if [ "$msg" = "on" ]; then
    echo "Turning lights on..."
  fi
done
```

### Log Aggregation

```bash
# Application publishes logs
mosquitto_pub -h logs.example.com -t "app/production/error" -m "Database connection failed"

# Log aggregator subscribes
mosquitto_sub -h logs.example.com -t "app/+/+" > /var/log/mqtt-events.log
```

### Health Check Monitoring

```bash
# Health check script publishes status
if systemctl is-active --quiet myapp; then
  mosquitto_pub -h monitor.local -t "status/myapp" -m "healthy"
else
  mosquitto_pub -h monitor.local -t "status/myapp" -m "down"
fi
```

## Agent Use

- Monitor distributed IoT device states in real-time
- Aggregate sensor data from multiple sources for analytics
- Publish control commands to edge devices in deployment workflows
- Implement health checks across containerized systems
- Log aggregation and event streaming for observability
- Bridge between cloud services and on-premises MQTT brokers
- Implement pub/sub messaging in CI/CD automation scripts

## Troubleshooting

### Port already in use

Check what's using port 1883:

```bash
# Find process using the port
lsof -i :1883  # Linux/macOS
netstat -ano | findstr :1883  # Windows

# Change port in config or start on different port
mosquitto -p 1884
```

### Connection refused

Verify mosquitto is running:

```bash
# Check process
ps aux | grep mosquitto
pgrep mosquitto

# Verify listening
netstat -an | grep 1883
ss -ltn | grep 1883
```

### Authentication failed

Verify password file exists and user is configured:

```bash
# Create user with password
mosquitto_passwd -c /etc/mosquitto/passwd username

# Set permissions in config
# listener 1883
# password_file /etc/mosquitto/passwd
```

### No subscribers receiving messages

Verify subscription and queueing:

```bash
# Subscribe before publishing
mosquitto_sub -h localhost -t "test/topic" &
sleep 1
mosquitto_pub -h localhost -t "test/topic" -m "hello"

# Check for persistence issues
mosquitto -c /etc/mosquitto/mosquitto.conf -v
```

## Uninstall

```yaml
- preset: mosquitto
  with:
    state: absent
```

## Resources

- **Official Docs**: https://mosquitto.org/documentation/
- **GitHub**: https://github.com/eclipse/mosquitto
- **MQTT Standard**: https://mqtt.org/
- **Man Pages**: `man mosquitto`, `man mosquitto_pub`, `man mosquitto_sub`
- **Search**: "MQTT tutorial", "mosquitto broker setup", "IoT messaging patterns"
