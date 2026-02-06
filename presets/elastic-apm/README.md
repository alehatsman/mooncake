# elastic-apm - Application Performance Monitoring

APM agent for monitoring application performance and errors with Elastic Stack.

## Quick Start
```yaml
- preset: elastic-apm
```

## Features
- **Real-time monitoring**: Track application performance
- **Error tracking**: Capture and analyze exceptions
- **Distributed tracing**: Follow requests across services
- **Metrics collection**: CPU, memory, response times
- **Integration**: Works with Elasticsearch and Kibana
- **Multi-language**: Python, Node.js, Java, Go, Ruby, .NET

## Basic Usage
```python
# Python
from elasticapm import Client

client = Client({
    'SERVICE_NAME': 'my-app',
    'SERVER_URL': 'http://localhost:8200',
})

# Django
INSTALLED_APPS = [
    'elasticapm.contrib.django',
]

ELASTIC_APM = {
    'SERVICE_NAME': 'my-django-app',
    'SERVER_URL': 'http://localhost:8200',
}

# Flask
from elasticapm.contrib.flask import ElasticAPM

app = Flask(__name__)
apm = ElasticAPM(app)
```

```javascript
// Node.js
const apm = require('elastic-apm-node').start({
  serviceName: 'my-app',
  serverUrl: 'http://localhost:8200'
});
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Real-World Examples

### Microservices Tracing
```python
# Service A
from elasticapm import Client
apm = Client({'SERVICE_NAME': 'api-gateway'})

@apm.capture_span()
def call_user_service():
    response = requests.get('http://user-service/users')
    return response.json()
```

### Error Tracking
```python
try:
    result = risky_operation()
except Exception as e:
    apm.capture_exception()
    raise
```

### Custom Metrics
```python
apm.capture_message('User checkout completed')

with apm.capture_span('database-query'):
    users = db.query(User).all()
```

## Agent Use
- Monitor application performance in production
- Track errors and exceptions
- Analyze slow requests
- Trace distributed transactions
- Collect custom metrics
- Debug performance bottlenecks

## Uninstall
```yaml
- preset: elastic-apm
  with:
    state: absent
```

## Resources
- Official docs: https://www.elastic.co/guide/en/apm/
- GitHub: https://github.com/elastic/apm-agent-python
- Search: "elastic apm tutorial"
