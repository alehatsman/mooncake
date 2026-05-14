# mcfly - Neural Network-Powered Shell History Search

A lightning-fast command-line history search tool that uses neural networks to learn from your shell history and predict the commands you want to run.

## Quick Start

```yaml
- preset: mcfly
```

This installs mcfly and integrates it with your shell for intelligent history search.

## Features

- **Neural Network Search**: Uses machine learning to intelligently rank shell history based on context and patterns
- **Lightning Fast**: Written in Rust with minimal overhead, searches thousands of commands instantly
- **Cross-Shell Support**: Works with Bash, Zsh, and Fish shells
- **Learning Algorithm**: Learns from your command patterns and adapts predictions over time
- **Keyboard Shortcuts**: Intuitive Ctrl+R integration for seamless history navigation
- **Session Context**: Understands directory changes and command relationships
- **Offline**: All processing happens locally, no data collection or network calls

## Basic Usage

```bash
# Show version
mcfly --version

# Display help
mcfly --help

# Search shell history with neural network ranking
# (typically triggered with Ctrl+R in your shell)
mcfly search "query"

# View recent commands with scores
mcfly search
```

## Advanced Configuration

```yaml
# Basic installation with default settings
- preset: mcfly
  with:
    state: present

# Specify desired version
- preset: mcfly
  with:
    state: present
    version: "0.9.0"  # or "latest" for newest version
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install (present) or remove (absent) mcfly |

## Configuration

- **Shell Integration**: mcfly integrates automatically with Bash, Zsh, and Fish
- **History Database**: `~/.local/share/mcfly/` (Linux), `~/Library/Application Support/mcfly/` (macOS)
- **Configuration**: Auto-detected per-shell, no manual config required
- **Learning Database**: Stored locally in standard XDG directories

## Platform Support

- ✅ Linux (via script installation from official source)
- ✅ macOS (Homebrew)
- ❌ Windows (not supported)

## Real-World Examples

### Searching Recent Docker Commands

```bash
# In your shell, press Ctrl+R and type:
docker

# mcfly shows your most frequently used docker commands ranked by context
# Select with arrow keys, execute with Enter
```

### Finding Commands by Directory Context

```bash
# Navigate to project directory
cd ~/projects/my-app

# Press Ctrl+R and search
npm test

# mcfly learns that "npm test" is common in this directory
# Future searches will rank these commands higher when in similar contexts
```

### Workflow Pattern Recognition

```bash
# Typical development workflow
git checkout feature-branch
npm install
npm run dev

# Later, press Ctrl+R - mcfly learns these command sequences
# It can predict "npm run dev" when you type "npm"
# because it learned the pattern from your history
```

## Agent Use

- **Command Prediction**: Agents can analyze shell history patterns to understand user workflows and predict next steps
- **Automation Workflows**: AI agents can leverage learned patterns to generate efficient command sequences
- **DevOps Optimization**: Identify frequently-used deployment or infrastructure commands and suggest optimizations
- **Training & Onboarding**: Extract common command patterns from team histories to inform documentation and training materials
- **Performance Analysis**: Use command frequency and context data to identify optimization opportunities in development workflows
- **Shell Script Generation**: Learn from command patterns to generate accurate shell scripts that match user preferences

## Troubleshooting

### mcfly not integrating with shell

mcfly requires shell integration in your rc file. Check that your shell's configuration includes mcfly initialization:

```bash
# For Bash/Zsh, check ~/.bashrc or ~/.zshrc
# Should contain:
eval "$(mcfly init bash)"  # or zsh for Zsh

# For Fish, check ~/.config/fish/config.fish
# Should contain:
mcfly init fish | source
```

### Search results not improving over time

mcfly's neural network learns from your command history. Ensure:
- You've used commands multiple times for learning to be effective
- mcfly has been initialized in your shell (`eval "$(mcfly init bash)"`)
- History database isn't being cleared frequently

### Performance issues

If searches are slow, verify your shell history isn't corrupted:

```bash
# Check history file size
wc -l ~/.bash_history  # or ~/.zsh_history

# mcfly works best with 5,000-50,000 commands
# Very large histories (100k+) may need trimming
```

## Uninstall

```yaml
- preset: mcfly
  with:
    state: absent
```

This removes mcfly from your system. Your command history remains untouched.

## Resources

- **Official Repository**: https://github.com/cantino/mcfly
- **Documentation**: https://github.com/cantino/mcfly#readme
- **Shell Integration**: https://github.com/cantino/mcfly#installation
- **Search Terms**: "mcfly shell history", "neural network history search", "interactive shell history"
