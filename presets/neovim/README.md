# Neovim Preset

**Status:** ✓ Installed successfully

## Quick Start

```bash
# Start Neovim
nvim

# Open file
nvim file.txt

# Open at specific line
nvim +10 file.txt

# Check version
nvim --version

# Check health
nvim +checkhealth
```

## Configuration

- **Config directory:** `~/.config/nvim/`
- **Init file:** `~/.config/nvim/init.vim` or `~/.config/nvim/init.lua`
- **Plugin directory:** `~/.local/share/nvim/`
- **Data directory:** `~/.local/share/nvim/`

## Basic Usage

**Normal Mode:**
- `i` - Insert mode
- `v` - Visual mode
- `V` - Visual line mode
- `:w` - Save
- `:q` - Quit
- `:wq` - Save and quit
- `:q!` - Quit without saving
- `u` - Undo
- `Ctrl+r` - Redo

**Navigation:**
- `h j k l` - Left, down, up, right
- `w` - Next word
- `b` - Previous word
- `0` - Start of line
- `$` - End of line
- `gg` - Start of file
- `G` - End of file

**Editing:**
- `dd` - Delete line
- `yy` - Copy line
- `p` - Paste
- `x` - Delete character
- `r` - Replace character
- `cw` - Change word

## Plugin Management

Using **vim-plug**:

```vim
" ~/.config/nvim/init.vim
call plug#begin()
Plug 'nvim-treesitter/nvim-treesitter'
Plug 'neovim/nvim-lspconfig'
Plug 'nvim-telescope/telescope.nvim'
call plug#end()
```

```bash
# Install vim-plug
sh -c 'curl -fLo "${XDG_DATA_HOME:-$HOME/.local/share}"/nvim/site/autoload/plug.vim --create-dirs \
       https://raw.githubusercontent.com/junegunn/vim-plug/master/plug.vim'

# Install plugins
nvim +PlugInstall +qall

# Update plugins
nvim +PlugUpdate +qall
```

## LSP (Language Server Protocol)

```lua
-- ~/.config/nvim/init.lua
require('lspconfig').pyright.setup{}  -- Python
require('lspconfig').tsserver.setup{} -- TypeScript
require('lspconfig').gopls.setup{}    -- Go
```

## Popular Configurations

- **LazyVim** - `https://www.lazyvim.org/`
- **NvChad** - `https://nvchad.com/`
- **AstroNvim** - `https://astronvim.com/`
- **LunarVim** - `https://www.lunarvim.org/`

## Useful Commands

```vim
" Search and replace
:%s/old/new/g

" Open file explorer
:Explore

" Split windows
:split file.txt   " Horizontal
:vsplit file.txt  " Vertical

" Navigate splits
Ctrl+w h/j/k/l

" Run shell command
:!ls

" Open terminal
:terminal
```

## Neovim vs Vim

- Better Lua support
- Built-in LSP client
- Tree-sitter integration
- Better plugin architecture
- Async I/O

## Learning Resources

```bash
# Built-in tutorial
nvim +Tutor

# Help
:help
:help navigation
```

## Uninstall

```yaml
- preset: neovim
  with:
    state: absent
```

**Note:** Configuration in `~/.config/nvim/` preserved after uninstall.
