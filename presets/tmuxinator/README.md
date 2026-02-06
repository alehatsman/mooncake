# tmuxinator - Tmux Session Manager

Manage complex tmux sessions with YAML configuration files. Define layouts, windows, panes, and commands once, then launch instantly.

## Quick Start
```yaml
- preset: tmuxinator
```

## Features
- **YAML Configuration**: Define tmux sessions as code
- **Multi-Window Support**: Multiple windows with custom layouts
- **Pane Splitting**: Complex pane layouts
- **Auto-Start Commands**: Run commands automatically in each pane
- **Project Management**: One config per project
- **Cross-platform**: Linux and macOS support

## Basic Usage
```bash
# Create new project
tmuxinator new project-name

# Start project
tmuxinator start project-name

# Stop project
tmuxinator stop project-name

# List projects
tmuxinator list

# Edit project
tmuxinator edit project-name

# Delete project
tmuxinator delete project-name

# Copy project
tmuxinator copy existing-project new-project
```

## Configuration Format

### Simple Project
```yaml
# ~/.config/tmuxinator/simple.yml
name: simple
root: ~/projects/myapp

windows:
  - editor: vim
  - server: npm start
  - logs: tail -f logs/development.log
```

Start with:
```bash
tmuxinator start simple
```

### Development Project
```yaml
# ~/.config/tmuxinator/dev.yml
name: dev
root: ~/projects/webapp

startup_window: editor

windows:
  - editor:
      layout: main-vertical
      panes:
        - vim
        - guard

  - server:
      panes:
        - npm run dev
        - redis-server

  - logs:
      layout: even-horizontal
      panes:
        - tail -f logs/development.log
        - tail -f logs/test.log

  - database:
      panes:
        - psql myapp_development
```

### Full-Stack Project
```yaml
# ~/.config/tmuxinator/fullstack.yml
name: fullstack
root: ~/projects/myapp

pre: docker-compose up -d

windows:
  - frontend:
      root: ~/projects/myapp/frontend
      layout: main-horizontal
      panes:
        - editor:
          - cd ~/projects/myapp/frontend
          - vim
        - server: npm run dev
        - test: npm run test:watch

  - backend:
      root: ~/projects/myapp/backend
      layout: main-horizontal
      panes:
        - editor: vim
        - server: go run main.go
        - logs: tail -f logs/api.log

  - database:
      panes:
        - psql myapp_dev
        - redis-cli

  - monitoring:
      layout: tiled
      panes:
        - htop
        - docker stats
        - tail -f /var/log/syslog
```

## Real-World Examples

### Web Development
```yaml
# ~/.config/tmuxinator/web.yml
name: web
root: ~/code/webapp

startup_window: code

windows:
  - code:
      layout: main-vertical
      panes:
        - code .  # VS Code
        - git status

  - frontend:
      root: ~/code/webapp/client
      panes:
        - npm run dev
        - npm run test:watch

  - backend:
      root: ~/code/webapp/server
      panes:
        - npm start
        - npm run db:migrate

  - tools:
      panes:
        - docker-compose ps
        - redis-cli
```

### DevOps Monitoring
```yaml
# ~/.config/tmuxinator/monitoring.yml
name: monitoring
root: ~

windows:
  - system:
      layout: tiled
      panes:
        - htop
        - iotop
        - nethogs
        - watch -n 1 df -h

  - kubernetes:
      panes:
        - kubectl get pods -A --watch
        - kubectl top nodes --watch
        - kubectl top pods -A --watch

  - logs:
      layout: even-vertical
      panes:
        - tail -f /var/log/syslog
        - journalctl -f
        - docker logs -f container-name
```

### Testing Environment
```yaml
# ~/.config/tmuxinator/test.yml
name: test
root: ~/projects/myapp

pre: docker-compose -f docker-compose.test.yml up -d

windows:
  - tests:
      layout: main-horizontal
      panes:
        - unit: npm run test:unit:watch
        - integration: npm run test:integration:watch
        - e2e: npm run test:e2e

  - coverage:
      panes:
        - npm run coverage:watch
        - python -m http.server 8000  # Serve coverage HTML

  - ci:
      panes:
        - act --watch  # GitHub Actions locally
```

### Database Administration
```yaml
# ~/.config/tmuxinator/db-admin.yml
name: db-admin
root: ~

windows:
  - postgres:
      panes:
        - prod: psql $PROD_DB_URL
        - staging: psql $STAGING_DB_URL
        - dev: psql $DEV_DB_URL

  - monitoring:
      panes:
        - pg_top
        - watch -n 2 'psql -c "SELECT * FROM pg_stat_activity"'

  - backups:
      root: ~/backups
      panes:
        - ls -lh
```

## Advanced Configuration

### Environment Variables
```yaml
name: myproject
root: ~/projects/myproject

startup_window: code

# Pre commands (run before windows)
pre: docker-compose up -d

# Post commands (run after windows)
post: echo "Environment ready!"

# Pre window commands
pre_window: source .env

# Attach on start (true by default)
attach: true

# Set TMUX socket name
socket_name: myproject

# Kill session on stop
on_project_stop: tmux kill-session -t myproject

windows:
  - code:
      pre: nvm use
      panes:
        - vim
```

### Custom Layouts
```yaml
windows:
  - main:
      layout: main-horizontal  # Options: main-horizontal, main-vertical, even-horizontal, even-vertical, tiled
      panes:
        - pane1
        - pane2

  - custom:
      layout: 'd071,272x73,0,0{136x73,0,0,0,135x73,137,0[135x36,137,0,1,135x36,137,37,2]}'
      panes:
        - top-left
        - top-right
        - bottom-right
```

## Command Line Options
```bash
# Start with custom name
tmuxinator start project-name session=custom-session

# Debug mode
tmuxinator start project-name --debug

# Local config file
tmuxinator start -p ./custom-config.yml

# List in JSON
tmuxinator list --json

# Version
tmuxinator version
```

## Configuration Location
```bash
# Default config directory
~/.config/tmuxinator/

# Or use environment variable
export TMUXINATOR_CONFIG="$HOME/.tmuxinator"

# List config directory
tmuxinator doctor
```

## Advanced Configuration
```yaml
- preset: tmuxinator
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove tmuxinator |

## Platform Support
- ✅ Linux (gem install)
- ✅ macOS (Homebrew or gem install)
- ❌ Windows (not supported, use WSL)

## Tips and Tricks
```bash
# Auto-completion
echo 'source ~/.tmuxinator/tmuxinator.bash' >> ~/.bashrc  # Bash
echo 'source ~/.tmuxinator/tmuxinator.zsh' >> ~/.zshrc    # Zsh

# Alias for convenience
alias mux="tmuxinator"

# Start default project on terminal open
echo 'tmuxinator start dev' >> ~/.zshrc

# Use with SSH
ssh server "tmuxinator start project"
```

## Troubleshooting

### Config not found
```bash
# Check config location
tmuxinator doctor

# List all projects
tmuxinator list

# Use full path
tmuxinator start -p ~/path/to/config.yml
```

### Commands not running
```bash
# Check pre commands
pre: echo "Running..." && command

# Add delays
panes:
  - sleep 2 && command

# Check command exit codes
panes:
  - command1 || echo "Failed"
```

### Layout issues
```bash
# Use predefined layouts
layout: main-horizontal

# Or capture current layout
tmux list-windows -F "#{window_layout}"

# Paste into config
layout: 'd071,272x73...'
```

## Agent Use
- Consistent development environment setup
- One-command project initialization
- Multi-service application management
- Testing environment orchestration
- Monitoring dashboard creation
- Database administration workflows

## Uninstall
```yaml
- preset: tmuxinator
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/tmuxinator/tmuxinator
- Official docs: https://github.com/tmuxinator/tmuxinator/blob/master/README.md
- Search: "tmuxinator examples", "tmuxinator layouts", "tmuxinator configuration"
