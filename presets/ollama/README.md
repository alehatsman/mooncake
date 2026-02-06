# Ollama Preset

Production-ready preset for installing and managing Ollama LLM runtime.

## Structure

```
ollama/
├── preset.yml              # Main preset definition (orchestration)
├── tasks/                  # Task modules
│   ├── install.yml        # Installation logic
│   ├── configure.yml      # Service configuration
│   ├── models.yml         # Model management
│   └── uninstall.yml      # Cleanup tasks
└── templates/              # Configuration templates
    ├── systemd-dropin.conf.j2   # Linux systemd configuration
    └── launchd.plist.j2         # macOS launchd configuration
```

## Features

- **Cross-platform**: Supports Linux (systemd) and macOS (launchd)
- **Flexible installation**: Package manager or official script
- **Service management**: Automatic service configuration and startup
- **Model management**: Pull and manage LLM models with idempotency
- **Configurable**: Custom bind address and models directory
- **Clean uninstall**: Complete removal with optional model cleanup

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | `present` to install, `absent` to uninstall |
| `pull` | array | `[]` | List of models to pull (e.g., `['llama3.1:8b']`) |
| `service` | bool | `true` | Enable and start Ollama service |
| `method` | string | `auto` | Installation method: `auto`, `script`, `package` |
| `host` | string | - | Server bind address (e.g., `0.0.0.0:11434`) |
| `models_dir` | string | - | Custom models directory path |
| `force` | bool | `false` | Force re-pull models, force remove data on uninstall |

## Quick Start

### Basic installation
```yaml
- name: Install Ollama
  preset:
    name: ollama
  become: true
```

### With model
```yaml
- name: Install Ollama with model
  preset:
    name: ollama
    with:
      pull: [tinyllama]
  become: true
```

### Production setup
```yaml
- name: Install Ollama for production
  preset:
    name: ollama
    with:
      service: true
      host: "0.0.0.0:11434"
      models_dir: "/opt/ollama/models"
      pull: ["llama3.1:8b", "mistral:latest"]
  become: true
```

### Uninstall
```yaml
- name: Remove Ollama
  preset:
    name: ollama
    with:
      state: absent
      force: true  # Also remove models
  become: true
```

## How It Works

### Installation Flow (state: present)
1. **Install**: Checks if Ollama exists, installs via package manager or script
2. **Configure**: Sets up systemd/launchd service with environment variables
3. **Models**: Pulls requested models (idempotent)

### Uninstallation Flow (state: absent)
1. Stop and disable service
2. Remove Ollama binary
3. Optionally remove models directory (if `force: true`)

## Customization

### Custom Host Binding
```yaml
with:
  host: "192.168.1.100:11434"
```
Sets `OLLAMA_HOST` environment variable via service configuration.

### Custom Models Directory
```yaml
with:
  models_dir: "/data/ollama"
```
Sets `OLLAMA_MODELS` environment variable via service configuration.

### Installation Methods

- **auto** (default): Tries package manager, falls back to script
- **package**: Uses system package manager only (apt, dnf, yum, brew)
- **script**: Uses official Ollama installation script

## Platform Support

### Linux (systemd)
- Creates drop-in configuration: `/etc/systemd/system/ollama.service.d/10-mooncake.conf`
- Manages service via `systemctl`
- Requires `sudo` for installation

### macOS (launchd)
- Creates launchd plist: `~/Library/LaunchAgents/com.ollama.ollama.plist`
- Manages service via `launchctl`
- Homebrew installation available

## Basic Usage

After installation, Ollama runs as a service and is available at `http://localhost:11434`.

### Command Line
```bash
# Pull a model
ollama pull llama3.1:8b

# Run interactive chat
ollama run llama3.1:8b

# Single prompt
ollama run llama3.1:8b "Explain quantum computing"

# List models
ollama list

# Show model info
ollama show llama3.1:8b

# Remove model
ollama rm llama3.1:8b

# Stop Ollama
systemctl stop ollama  # Linux
launchctl stop com.ollama.ollama  # macOS
```

### API Usage
```bash
# Generate completion
curl http://localhost:11434/api/generate -d '{
  "model": "llama3.1:8b",
  "prompt": "Why is the sky blue?"
}'

# Chat completion
curl http://localhost:11434/api/chat -d '{
  "model": "llama3.1:8b",
  "messages": [
    {"role": "user", "content": "Hello!"}
  ]
}'

# Embeddings
curl http://localhost:11434/api/embeddings -d '{
  "model": "llama3.1:8b",
  "prompt": "The quick brown fox"
}'
```

### Python SDK
```python
import ollama

# Generate
response = ollama.generate(
    model='llama3.1:8b',
    prompt='Why is the sky blue?'
)
print(response['response'])

# Chat
response = ollama.chat(
    model='llama3.1:8b',
    messages=[
        {'role': 'user', 'content': 'Hello!'}
    ]
)
print(response['message']['content'])

# Stream
for chunk in ollama.chat(
    model='llama3.1:8b',
    messages=[{'role': 'user', 'content': 'Tell me a story'}],
    stream=True
):
    print(chunk['message']['content'], end='')
```

## Advanced Configuration

### Remote Access
```yaml
with:
  host: "0.0.0.0:11434"  # Listen on all interfaces
```

Then access from other machines:
```bash
OLLAMA_HOST=http://192.168.1.100:11434 ollama list
```

### Custom Models Directory
```yaml
with:
  models_dir: "/data/ollama/models"
```

Useful for:
- Storing models on external drive
- Shared network storage
- Disk space management

### GPU Configuration
Ollama automatically detects and uses GPUs (NVIDIA, AMD, Apple Silicon). Check logs:
```bash
journalctl -u ollama -f  # Linux
tail -f ~/Library/Logs/ollama.log  # macOS
```

### Service Management
```bash
# Linux (systemd)
systemctl status ollama
systemctl restart ollama
systemctl stop ollama
journalctl -u ollama -f

# macOS (launchd)
launchctl list | grep ollama
launchctl stop com.ollama.ollama
launchctl start com.ollama.ollama
```

### Environment Variables
Edit service configuration:
- Linux: `/etc/systemd/system/ollama.service.d/10-mooncake.conf`
- macOS: `~/Library/LaunchAgents/com.ollama.ollama.plist`

Available variables:
- `OLLAMA_HOST` - Bind address
- `OLLAMA_MODELS` - Models directory
- `OLLAMA_NUM_PARALLEL` - Parallel requests (default: 1)
- `OLLAMA_MAX_LOADED_MODELS` - Loaded models (default: 1)
- `OLLAMA_DEBUG` - Debug logging

## Agent Use

Ollama is ideal for AI agent systems:

### Autonomous Agents
```python
import ollama

def agent_think(task):
    """Agent reasoning loop"""
    response = ollama.generate(
        model='llama3.1:8b',
        prompt=f"Task: {task}\nThink step by step:"
    )
    return response['response']

# Use in agent loop
result = agent_think("Plan a web scraping project")
```

### Tool Calling
```python
# Define available tools
tools = [
    {
        "type": "function",
        "function": {
            "name": "get_weather",
            "description": "Get current weather",
            "parameters": {
                "type": "object",
                "properties": {
                    "location": {"type": "string"}
                }
            }
        }
    }
]

# Agent decides which tool to use
response = ollama.chat(
    model='llama3.1:8b',
    messages=[{'role': 'user', 'content': 'What is the weather in NYC?'}],
    tools=tools
)
```

### Multi-Agent Systems
```python
# Specialist agents with different models
planner = ollama.generate(model='llama3.1:70b', prompt='Plan: ...')
coder = ollama.generate(model='codellama', prompt='Implement: ...')
reviewer = ollama.generate(model='llama3.1:8b', prompt='Review: ...')
```

### RAG (Retrieval Augmented Generation)
```python
# Generate embeddings for search
def embed_documents(texts):
    embeddings = []
    for text in texts:
        result = ollama.embeddings(model='llama3.1:8b', prompt=text)
        embeddings.append(result['embedding'])
    return embeddings

# Query with context
context = search_documents(query)
response = ollama.generate(
    model='llama3.1:8b',
    prompt=f"Context: {context}\n\nQuestion: {query}"
)
```

### Local AI Pipeline
```yaml
# Install Ollama + models in agent environment
- preset: ollama
  with:
    service: true
    pull: ["llama3.1:8b", "codellama:7b", "mistral:latest"]
    host: "0.0.0.0:11434"
```

Benefits for agents:
- **Privacy** - No external API calls
- **Low latency** - Local inference
- **No rate limits** - Unlimited requests
- **Cost-free** - No API fees
- **Offline** - Works without internet

## Examples

See `examples/ollama/` for complete usage examples.

## Maintenance

### Adding a new installation method
Edit `tasks/install.yml` and add a new conditional step.

### Modifying service configuration
Edit templates:
- Linux: `templates/systemd-dropin.conf.j2`
- macOS: `templates/launchd.plist.j2`

### Adding model management features
Edit `tasks/models.yml` to add new model operations.

## Dependencies

- **Linux**: systemd, curl (for script installation)
- **macOS**: launchd, Homebrew (for package installation)
- **Both**: Internet connection for model downloads

## Troubleshooting

### Service won't start
Check logs:
- Linux: `journalctl -u ollama -f`
- macOS: `tail -f ~/Library/Logs/ollama.log`

### Models not pulling
Ensure Ollama service is running:
```bash
systemctl status ollama  # Linux
launchctl list | grep ollama  # macOS
```

### Permission errors
Most operations require `become: true` (sudo).

## Resources

- **Official Website**: https://ollama.com
- **GitHub**: https://github.com/ollama/ollama
- **Models Library**: https://ollama.com/library
- **API Documentation**: https://github.com/ollama/ollama/blob/main/docs/api.md
- **Python SDK**: `pip install ollama`
- **JavaScript SDK**: `npm install ollama`
- **Model Cards**: https://ollama.com/library (performance benchmarks)
- **Community Discord**: https://discord.gg/ollama

**Popular Models:**
- llama3.1:8b - Meta's latest (8B params)
- mistral:latest - Mistral AI (7B params)
- codellama:7b - Code generation
- llama3.1:70b - Large model (requires 40GB+ RAM)
- phi3:mini - Microsoft's compact model

**Search Terms:**
- "ollama install", "ollama run model"
- "ollama api", "ollama python"
- "ollama gpu", "ollama service"
