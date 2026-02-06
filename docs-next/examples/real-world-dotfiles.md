# Real-World Example: Dotfiles Manager

A complete example showing how to manage and deploy dotfiles using Mooncake.

## Features Demonstrated

- Multi-file organization
- Template rendering for dynamic configs
- File tree iteration
- Conditional deployment by OS
- Variable management
- Backup functionality
- Tag-based workflows

## Quick Start

```bash
cd examples/real-world/dotfiles-manager

# Deploy all dotfiles
mooncake run --config setup.yml

# Deploy only shell configs
mooncake run --config setup.yml --tags shell

# Preview what would be deployed
mooncake run --config setup.yml --dry-run
```

## Directory Structure

```
dotfiles-manager/
├── setup.yml              # Main entry point
├── vars.yml               # User configuration
├── dotfiles/              # Your actual dotfiles
│   ├── shell/
│   │   ├── .bashrc
│   │   └── .zshrc
│   ├── vim/
│   │   └── .vimrc
│   └── git/
│       └── .gitconfig
└── templates/             # Dynamic config templates
    ├── .tmux.conf.j2
    └── .config/
        └── nvim/
            └── init.lua.j2
```

## What It Does

1. Backs up existing dotfiles
2. Creates necessary directories
3. Deploys static dotfiles
4. Renders dynamic configs from templates
5. Sets appropriate permissions
6. OS-specific configuration

## Configuration

Edit `vars.yml` to customize:
```yaml
user_email: your@email.com
user_name: Your Name
editor: nvim
shell: zsh
color_scheme: gruvbox
```

## Usage

### Full Deployment
```bash
mooncake run --config setup.yml
```

### Selective Deployment
```bash
# Only shell configs
mooncake run --config setup.yml --tags shell

# Only vim/neovim
mooncake run --config setup.yml --tags vim

# Only git config
mooncake run --config setup.yml --tags git
```

### Backup Only
```bash
mooncake run --config setup.yml --tags backup
```

## Extending

### Adding New Dotfiles

1. Add file to `dotfiles/` directory
2. Add deployment step in `setup.yml`:
```yaml
- name: Deploy new config
  shell: cp {{ item.src }} ~/{{ item.name }}
  with_filetree: ./dotfiles/new-app
  tags:
    - new-app
```

### Adding Templates

1. Create template in `templates/`
2. Add rendering step:
```yaml
- name: Render new config
  template:
    src: ./templates/new-config.j2
    dest: ~/.config/new-app/config
  tags:
    - new-app
```

## Real-World Tips

1. **Version control** - Keep this in git
2. **Test first** - Use `--dry-run` before applying
3. **Incremental** - Add configs gradually
4. **Backup** - The example includes backup steps
5. **Document** - Add comments for custom settings

## See Also

This example combines concepts from:

- [06-loops](06-loops.md) - File iteration
- [05-templates](05-templates.md) - Config rendering
- [08-tags](08-tags.md) - Selective deployment
- [10-multi-file-configs](10-multi-file-configs.md) - Organization
