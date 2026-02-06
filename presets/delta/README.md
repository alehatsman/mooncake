# delta Preset

Install delta - a syntax-highlighting pager for git, diff, and grep output that makes code reviews and diffs beautiful and easy to read.

## Quick Start

```yaml
# Basic installation with git configuration
- preset: delta

# Installation without git configuration
- preset: delta
  with:
    configure_git: false

# With custom theme and options
- preset: delta
  with:
    theme: "GitHub"
    line_numbers: true
    side_by_side: true
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | Install (`present`) or uninstall (`absent`) |
| `configure_git` | bool | `true` | Configure git to use delta automatically |
| `theme` | string | `Monokai Extended` | Color theme |
| `line_numbers` | bool | `true` | Show line numbers in diffs |
| `side_by_side` | bool | `false` | Display diffs side-by-side |

## Basic Usage
```bash
# View changes (automatic with git)
git diff

# Compare files directly
delta file1.txt file2.txt

# Pipe diff output
diff -u old.txt new.txt | delta

# View commit
git show HEAD

# Compare branches
git diff main..feature-branch
```

## Usage

delta works automatically with git when configured. It enhances:

- `git diff`
- `git show`
- `git log -p`
- `git stash show -p`
- `git reflog -p`
- `git blame`

### Basic Examples

```bash
# View changes in working directory
git diff

# View changes in specific file
git diff README.md

# View commit
git show HEAD

# View commit history with diffs
git log -p

# Compare branches
git diff main..feature-branch

# View staged changes
git diff --staged
```

### Direct Usage

```bash
# Compare files
delta file1.txt file2.txt

# Pipe diff output
diff -u old.txt new.txt | delta

# Compare directories
diff -ur dir1/ dir2/ | delta

# With grep output
grep -r "pattern" . | delta
```

## Agent Use
- Enhance code review processes with better diff visualization
- Improve CI/CD log readability for deployment reviews
- Generate visual diff reports for documentation
- Automate code quality checks with syntax-highlighted comparisons
- Create readable change summaries for automated pull requests

## Features

### Syntax Highlighting

delta provides beautiful syntax highlighting for:
- **Languages**: Python, JavaScript, Rust, Go, Java, C++, and 100+ more
- **File formats**: JSON, YAML, XML, Markdown, SQL
- **Config files**: Dockerfile, nginx.conf, etc.

### Diff Improvements

1. **Intra-line changes**: Highlights exact characters that changed
2. **Line numbers**: Shows both old and new line numbers
3. **Commit decorations**: Beautiful commit headers with metadata
4. **Merge conflicts**: Enhanced 3-way merge conflict display
5. **Hunk headers**: Function names in diff hunks

### Navigation

- **Search**: `/pattern` to search within delta output
- **Navigate hunks**: `n` for next, `N` for previous (with `navigate` enabled)
- **Scroll**: Use terminal scrollback or pager commands

## Configuration

### Git Integration

When `configure_git: true`, delta is set as the default pager:

```bash
[core]
    pager = delta

[interactive]
    diffFilter = delta --color-only

[delta]
    navigate = true
    light = false
    line-numbers = true
    syntax-theme = Monokai Extended
    side-by-side = false

[merge]
    conflictstyle = diff3

[diff]
    colorMoved = default
```

### Manual Configuration

Add to `~/.gitconfig`:

```ini
[core]
    pager = delta

[interactive]
    diffFilter = delta --color-only

[delta]
    # Features
    navigate = true
    line-numbers = true
    side-by-side = false

    # Themes
    syntax-theme = Monokai Extended

    # Line numbers
    line-numbers-left-format = "{nm:>4}│"
    line-numbers-right-format = "{np:>4}│"

    # Styling
    file-style = bold yellow ul
    file-decoration-style = none
    hunk-header-decoration-style = cyan box

    # Merge conflicts
    merge-conflict-begin-symbol = ⚔
    merge-conflict-end-symbol = ⚔
    merge-conflict-ours-diff-header-style = yellow bold
    merge-conflict-theirs-diff-header-style = yellow bold
```

## Themes

### Available Themes

Popular themes:
- **Monokai Extended** (default) - Dark theme with vibrant colors
- **GitHub** - Light theme matching GitHub's colors
- **Nord** - Cool, frost-inspired palette
- **Dracula** - Dark theme with purple accents
- **Solarized (dark/light)** - Classic solarized color scheme
- **gruvbox** - Retro groove colors
- **OneHalfDark/Light** - Atom's One theme
- **Base16** - Multiple Base16 variants

### Change Theme

```bash
# Set theme in git config
git config --global delta.syntax-theme "GitHub"

# Or use delta directly
delta --syntax-theme "Nord"

# List available themes
delta --list-syntax-themes
```

### Theme Examples

```bash
# Dark themes
delta --syntax-theme "Monokai Extended"
delta --syntax-theme "Dracula"
delta --syntax-theme "Nord"

# Light themes
delta --syntax-theme "GitHub"
delta --syntax-theme "Solarized (light)"
delta --syntax-theme "OneHalfLight"
```

## Advanced Features

### Side-by-Side Mode

```bash
# Enable in config
git config --global delta.side-by-side true

# Or use flag
delta --side-by-side file1.txt file2.txt

# Adjust width
git config --global delta.side-by-side-width 100
```

### Custom Features

Create feature presets in `~/.gitconfig`:

```ini
[delta "decorations"]
    commit-decoration-style = bold yellow box ul
    file-style = bold yellow ul
    file-decoration-style = none
    hunk-header-decoration-style = cyan box ul

[delta "line-numbers"]
    line-numbers = true
    line-numbers-minus-style = "#444444"
    line-numbers-zero-style = "#444444"
    line-numbers-plus-style = "#444444"
    line-numbers-left-format = "{nm:>4}┊"
    line-numbers-right-format = "{np:>4}│"
    line-numbers-left-style = blue
    line-numbers-right-style = blue

[delta]
    features = decorations line-numbers
```

### Hyperlinks

Enable clickable file paths in terminals that support hyperlinks:

```ini
[delta]
    hyperlinks = true
    hyperlinks-file-link-format = "vscode://file/{path}:{line}"
```

### Blame Integration

```bash
# Configure git blame to use delta
git config --global pager.blame delta

# Use with git blame
git blame file.txt

# With line range
git blame -L 10,20 file.txt
```

## Integration with Tools

### With lazygit

Add to `~/.config/lazygit/config.yml`:

```yaml
git:
  paging:
    colorArg: always
    pager: delta --dark --paging=never
```

### With tig

Add to `~/.tigrc`:

```bash
set diff-view-options = --minimal --color=always
set pager = delta --paging=never
```

### With Vim/Neovim

```vim
" Use delta for git diff in vim
let g:gitgutter_git_args = '--diff-algorithm=minimal'
```

### With GitHub CLI

```bash
# Set delta as pager for gh
export GH_PAGER="delta"

# View PR diff
gh pr diff 123
```

## Real-World Workflows

### Code Review

```bash
# Review branch changes
git diff main..feature-branch | delta

# Review with side-by-side
git diff main..feature-branch | delta --side-by-side

# Review specific files
git diff main..feature-branch -- src/ | delta
```

### Commit History

```bash
# View recent commits with diffs
git log -p -10

# View changes by author
git log -p --author="John" --since="1 week ago"

# Search commits
git log -p -S "function_name"
```

### Merge Conflict Resolution

```bash
# When merge conflict occurs
git diff  # Shows 3-way diff with delta

# delta highlights:
# - Common ancestor
# - Your changes
# - Their changes
```

### Release Comparison

```bash
# Compare releases
git diff v1.0.0..v2.0.0 | delta

# Generate changelog
git log --oneline v1.0.0..v2.0.0
git diff v1.0.0..v2.0.0 -- CHANGELOG.md | delta
```

## Customization Examples

### Minimal Setup

```ini
[delta]
    syntax-theme = GitHub
    line-numbers = true
    navigate = true
```

### Detailed Setup

```ini
[delta]
    # Core features
    features = decorations line-numbers
    syntax-theme = Monokai Extended
    navigate = true

    # Styling
    minus-style = syntax "#3a273a"
    minus-emph-style = syntax "#6b2e43"
    plus-style = syntax "#273849"
    plus-emph-style = syntax "#305f6f"

    # Line numbers
    line-numbers = true
    line-numbers-minus-style = "#B10036"
    line-numbers-plus-style = "#03a4ff"

    # Whitespace
    whitespace-error-style = 22 reverse

    # Commit decorations
    commit-decoration-style = bold yellow box ul
    commit-style = raw

    # File decorations
    file-style = omit
    hunk-header-decoration-style = blue box
    hunk-header-file-style = red
    hunk-header-line-number-style = "#067a00"
    hunk-header-style = file line-number syntax
```

### Side-by-Side with Wrapping

```ini
[delta]
    side-by-side = true
    wrap-max-lines = unlimited
    wrap-left-symbol = "↵ "
    wrap-right-symbol = " ↵"
    wrap-right-prefix-symbol = "…"
```

## Tips and Tricks

1. **Quick toggle side-by-side**: Create git alias
   ```bash
   git config --global alias.ds 'diff --color=always'
   git ds | delta --side-by-side
   ```

2. **Per-repository config**: Use `.git/config` for project-specific themes

3. **Disable delta temporarily**:
   ```bash
   git --no-pager diff
   # or
   GIT_PAGER=cat git diff
   ```

4. **Compare with original**:
   ```bash
   git diff --no-ext-diff  # Use git's built-in diff
   ```

5. **Export diffs**:
   ```bash
   git diff > changes.diff
   delta < changes.diff > changes.html
   ```

## Troubleshooting

### delta not working

```bash
# Check git config
git config --global core.pager

# Should output: delta

# Test delta manually
echo "test" | delta
```

### Colors not showing

```bash
# Check terminal supports color
echo $TERM

# Force color output
git config --global color.ui always
git config --global delta.color-only false
```

### Side-by-side too narrow

```bash
# Adjust width
git config --global delta.side-by-side-width 120

# Or use percentage
git config --global delta.side-by-side-width "80%"
```

### Slow performance

```bash
# Disable heavy features
git config --global delta.syntax-theme none
git config --global delta.side-by-side false

# Use simpler theme
git config --global delta.syntax-theme "GitHub"
```


## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install delta
  preset: delta

- name: Use delta in automation
  shell: |
    # Custom configuration here
    echo "delta configured"
```
## Uninstall

```yaml
- preset: delta
  with:
    state: absent
```

This removes delta and cleans up git configuration.

## Resources

- **GitHub**: https://github.com/dandavison/delta
- **Docs**: https://dandavison.github.io/delta/
- **Themes**: https://github.com/dandavison/delta#syntax-highlighting-themes
- **Configuration**: https://dandavison.github.io/delta/configuration.html
