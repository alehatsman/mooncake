# Neovim - Modern Vim-based Text Editor

Hyperextensible Vim-based text editor with built-in LSP, tree-sitter, and modern plugin architecture.

## Quick Start

```yaml
- preset: neovim
```

## Features

- **Built-in LSP**: Native Language Server Protocol support for code intelligence
- **Tree-sitter**: Advanced syntax highlighting and code parsing
- **Lua configuration**: Modern configuration with Lua instead of VimScript
- **Async I/O**: Non-blocking operations for better performance
- **Better defaults**: Sensible defaults out of the box
- **Plugin managers**: Includes Packer.nvim or lazy.nvim setup
- **Cross-platform**: Linux, macOS, BSD, Windows

## Basic Usage

```bash
# Start Neovim
nvim

# Open file
nvim file.txt

# Open at specific line
nvim +10 file.txt

# Check version
nvim --version

# Run health check
nvim +checkhealth

# Open file explorer
nvim .
```

## Advanced Configuration

```yaml
# Install with plugin manager and LSP support
- preset: neovim
  with:
    install_packer: true       # Install Packer.nvim
    install_lsp: true           # Install LSP dependencies
    create_basic_config: true   # Create starter init.lua
```

```yaml
# Install with lazy.nvim (modern plugin manager)
- preset: neovim
  with:
    install_packer: false
    install_lazy: true
    install_lsp: true
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Neovim |
| install_packer | bool | true | Install Packer.nvim plugin manager |
| install_lazy | bool | false | Install lazy.nvim plugin manager |
| install_lsp | bool | true | Install LSP dependencies (Node.js, tree-sitter-cli) |
| create_basic_config | bool | false | Create basic init.lua configuration |

## Platform Support

- ✅ Linux (apt, dnf, yum, pacman, zypper)
- ✅ macOS (Homebrew)
- ❌ Windows (install manually via winget or chocolatey)

## Configuration

- **Config directory**: `~/.config/nvim/` (Linux/macOS)
- **Init file**: `~/.config/nvim/init.lua` or `~/.config/nvim/init.vim`
- **Plugin directory**: `~/.local/share/nvim/site/pack/`
- **Data directory**: `~/.local/share/nvim/`

## Real-World Examples

### Basic Vim-style Configuration
```lua
-- ~/.config/nvim/init.lua
vim.opt.number = true           -- Show line numbers
vim.opt.relativenumber = true   -- Relative line numbers
vim.opt.tabstop = 2             -- 2 spaces for tabs
vim.opt.shiftwidth = 2          -- 2 spaces for indents
vim.opt.expandtab = true        -- Use spaces, not tabs
vim.opt.mouse = 'a'             -- Enable mouse
```

### LSP Setup for Python Development
```lua
-- Install nvim-lspconfig plugin first
require('lspconfig').pyright.setup{
  settings = {
    python = {
      analysis = {
        typeCheckingMode = "basic",
        autoSearchPaths = true
      }
    }
  }
}

-- Key mappings for LSP
vim.keymap.set('n', 'gd', vim.lsp.buf.definition)
vim.keymap.set('n', 'K', vim.lsp.buf.hover)
vim.keymap.set('n', 'gr', vim.lsp.buf.references)
```

### Development Environment Setup
```yaml
# Install Neovim with all development tools
- preset: neovim
  with:
    install_lazy: true
    install_lsp: true
    create_basic_config: true

# Create custom init.lua
- name: Configure Neovim for Go development
  template:
    src_template: configs/nvim-go.lua.j2
    dest: ~/.config/nvim/init.lua
```

### Plugin Manager - Packer.nvim
```lua
-- ~/.config/nvim/lua/plugins.lua
return require('packer').startup(function(use)
  use 'wbthomason/packer.nvim'  -- Packer manages itself
  use 'neovim/nvim-lspconfig'   -- LSP configurations
  use 'nvim-treesitter/nvim-treesitter'
  use 'nvim-telescope/telescope.nvim'
  use 'folke/tokyonight.nvim'   -- Theme
end)
```

### Plugin Manager - lazy.nvim
```lua
-- ~/.config/nvim/init.lua
local lazypath = vim.fn.stdpath("data") .. "/lazy/lazy.nvim"
require("lazy").setup({
  "neovim/nvim-lspconfig",
  "nvim-treesitter/nvim-treesitter",
  "nvim-telescope/telescope.nvim",
})
```

## Keyboard Shortcuts

### Normal Mode
```
i       Insert mode
v       Visual mode
V       Visual line mode
:w      Save file
:q      Quit
:wq     Save and quit
:q!     Quit without saving
u       Undo
Ctrl+r  Redo
```

### Navigation
```
h j k l    Left, down, up, right
w          Next word
b          Previous word
0          Start of line
$          End of line
gg         Start of file
G          End of file
{          Previous paragraph
}          Next paragraph
Ctrl+u     Page up
Ctrl+d     Page down
```

### Editing
```
dd      Delete line
yy      Copy line
p       Paste after cursor
P       Paste before cursor
x       Delete character
cw      Change word
ciw     Change inner word
ci"     Change inside quotes
```

### Windows and Tabs
```
:split      Horizontal split
:vsplit     Vertical split
Ctrl+w h/j/k/l   Navigate splits
:tabnew     New tab
gt          Next tab
gT          Previous tab
```

## Popular Neovim Distributions

Pre-configured Neovim setups for different workflows:

- **LazyVim**: Fast IDE-like setup - https://www.lazyvim.org/
- **NvChad**: Beautiful, blazing fast - https://nvchad.com/
- **AstroNvim**: Feature-rich, community-driven - https://astronvim.com/
- **LunarVim**: IDE layer for Neovim - https://www.lunarvim.org/

## Agent Use

- Set up development environments with consistent editor configuration
- Automate plugin installation and LSP configuration
- Deploy standardized Neovim configs across teams
- Configure language-specific editing environments
- Create reproducible development setups in CI/CD
- Provision remote development environments with Neovim

## Troubleshooting

### Plugin manager not loading
```bash
# For Packer
nvim +PackerSync

# For lazy.nvim
nvim +Lazy sync
```

### LSP not working
```bash
# Check LSP status
nvim +checkhealth lsp

# Install language server manually
npm install -g pyright        # Python
npm install -g typescript-language-server  # TypeScript
```

### Performance issues
```lua
-- Disable slow plugins on large files
vim.api.nvim_create_autocmd("BufReadPre", {
  pattern = "*",
  callback = function()
    local ok, stats = pcall(vim.loop.fs_stat, vim.api.nvim_buf_get_name(0))
    if ok and stats and stats.size > 1000000 then
      vim.cmd("syntax off")
    end
  end,
})
```

## Uninstall

```yaml
- preset: neovim
  with:
    state: absent
```

**Note**: Configuration files in `~/.config/nvim/` are preserved after uninstall.

## Resources

- Official docs: https://neovim.io/doc/
- GitHub: https://github.com/neovim/neovim
- Wiki: https://github.com/neovim/neovim/wiki
- Awesome Neovim: https://github.com/rockerBOO/awesome-neovim
- Search: "neovim lua config", "neovim lsp setup", "neovim plugin tutorial"
