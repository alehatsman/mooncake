# Documentation Website Setup

Complete guide to set up and deploy the Mooncake documentation website.

## 🎯 Overview

Your documentation will be hosted at: **https://mooncake.alehatsman.com**

**Tech Stack:**
- **MkDocs** - Static site generator
- **Material for MkDocs** - Beautiful theme
- **Cloudflare Pages** - Fast global hosting
- **pipenv** - Python dependency management (no global installs!)

## 🚀 Quick Setup (5 minutes)

### 1. Install pipenv

pipenv keeps dependencies isolated (no global installs):

```bash
pip install --user pipenv
```

Or on macOS with Homebrew:
```bash
brew install pipenv
```

### 2. Install Dependencies

```bash
pipenv install
```

This creates a virtual environment and installs MkDocs Material.

Or use the automated setup script:
```bash
./scripts/setup-docs.sh
```

### 3. Preview Locally

```bash
pipenv run mkdocs serve
```

Open http://127.0.0.1:8000 in your browser

> 💡 **Tip:** All MkDocs commands must be prefixed with `pipenv run` to use the virtual environment

### 4. Deploy to Cloudflare Pages

Just push to master:
```bash
git add .
git commit -m "Add documentation site"
git push origin master
```

Cloudflare Pages will automatically:
1. Build the documentation
2. Deploy to your custom domain
3. Your site will be live at https://mooncake.alehatsman.com

### 5. Configure Cloudflare Pages

Go to Cloudflare Dashboard:
1. Workers & Pages → Create a project → Connect to Git
2. Select your repository
3. Build command: `pip install pipenv && pipenv install && pipenv run mkdocs build`
4. Build output directory: `site`
5. Deploy

**Done!** Your site will be live in ~2 minutes.

## 📁 Structure

```
mooncake/
├── Pipfile                 # Python dependencies (pipenv)
├── Pipfile.lock            # Locked versions
├── mkdocs.yml              # Site configuration
├── docs/                   # Documentation source
│   ├── index.md            # Homepage
│   ├── getting-started/    # Getting started guides
│   ├── guide/              # User guide
│   ├── examples/           # Example documentation
│   ├── development/        # Development docs
│   └── stylesheets/        # Custom CSS
├── .github/workflows/
│   └── docs.yml            # Auto-deploy workflow
└── scripts/
    └── setup-docs.sh       # Setup script
```

## 🔧 Common Commands

All commands use `pipenv run` to use the virtual environment:

```bash
# Start development server
pipenv run mkdocs serve

# Build static site
pipenv run mkdocs build

# Build with strict mode (fail on warnings)
pipenv run mkdocs build --strict

# Update dependencies
pipenv update

# Install new dependency
pipenv install package-name

# Show dependency graph
pipenv graph
```

## 📝 Adding Content

### Create New Page

1. Create markdown file:
```bash
touch docs/guide/new-feature.md
```

2. Add to navigation in `mkdocs.yml`:
```yaml
nav:
  - User Guide:
    - New Feature: guide/new-feature.md
```

3. Write content:
```markdown
# New Feature

Description here...
```

4. Preview:
```bash
pipenv run mkdocs serve
```

### Reuse Existing Markdown

The setup already links:
- ✅ CONTRIBUTING.md → docs/development/contributing.md
- ✅ ROADMAP.md → docs/development/roadmap.md
- ✅ docs/DEVELOPMENT.md → docs/development/development.md

### Add Examples

For each example directory (e.g., `examples/01-hello-world/`):

1. Create doc page: `docs/examples/01-hello-world.md`
2. Copy/adapt the README content
3. Add to navigation

## 🎨 Customization

### Colors

Edit `mkdocs.yml`:
```yaml
theme:
  palette:
    primary: indigo  # Change this
    accent: blue     # Change this
```

Colors: red, pink, purple, indigo, blue, cyan, teal, green, lime, yellow, amber, orange, deep orange

### Logo

1. Add `docs/assets/logo.png`
2. Update `mkdocs.yml`:
```yaml
theme:
  logo: assets/logo.png
```

### Custom CSS

Edit `docs/stylesheets/extra.css` for custom styles.

## 🧩 Advanced Features

### Code Tabs

````markdown
=== "Linux"
    ```bash
    apt install mooncake
    ```

=== "macOS"
    ```bash
    brew install mooncake
    ```
````

### Admonitions

```markdown
!!! note
    This is a note

!!! tip "Pro Tip"
    Use dry-run first!

!!! warning
    This is destructive!
```

Types: note, abstract, info, tip, success, question, warning, failure, danger, bug, example, quote

### Collapsible Sections

```markdown
??? note "Click to expand"
    Hidden content here
```

## 🚢 Deployment

### Cloudflare Pages (Configured)

Cloudflare Pages automatically deploys when you push to master:
```bash
git push origin master
```

Cloudflare Pages will:
1. Detect the push
2. Run: `pip install pipenv && pipenv install && pipenv run mkdocs build`
3. Deploy the `site/` directory
4. Serve at https://mooncake.alehatsman.com

### Custom Subdomain

Already configured at **mooncake.alehatsman.com**!

To change the subdomain:
1. Go to Cloudflare Pages → Your project → Custom domains
2. Add new subdomain
3. Cloudflare auto-configures DNS
4. Update `site_url` in `mkdocs.yml`

## 🐛 Troubleshooting

### "mkdocs: command not found"

You forgot `pipenv run`:
```bash
# Wrong
mkdocs serve

# Correct
pipenv run mkdocs serve
```

### Dependencies Not Installing

```bash
# Remove and reinstall
pipenv --rm
pipenv install
```

### Build Fails

```bash
# Check for errors
pipenv run mkdocs build --strict

# Validate config
pipenv run mkdocs serve --strict
```

### Pages Not Updating

1. Clear browser cache (Cloudflare CDN cache)
2. Wait 2-3 minutes for deployment
3. Check Cloudflare Pages deployment logs
4. Purge cache in Cloudflare if needed

### Navigation Issues

Make sure paths in `nav` match actual files:
```yaml
nav:
  - Home: index.md  # Must exist as docs/index.md
```

### Theme Not Loading

Update dependencies:
```bash
pipenv update mkdocs-material
```

## 🔐 Virtual Environment

pipenv automatically manages the virtual environment:

```bash
# Show virtual environment location
pipenv --venv

# Activate shell in virtual environment (optional)
pipenv shell

# When in shell, no need for "pipenv run"
mkdocs serve

# Exit shell
exit
```

## 📚 Resources

- [MkDocs Documentation](https://www.mkdocs.org/)
- [Material Theme](https://squidfunk.github.io/mkdocs-material/)
- [pipenv Documentation](https://pipenv.pypa.io/)
- [Markdown Guide](https://www.markdownguide.org/)
- [GitHub Pages](https://pages.github.com/)

## ✅ Checklist

After setup:

- [ ] pipenv installed
- [ ] `pipenv install` completed
- [ ] `pipenv run mkdocs serve` works locally
- [ ] Cloudflare Pages project created
- [ ] Custom domain configured (mooncake.alehatsman.com)
- [ ] Site loads at https://mooncake.alehatsman.com
- [ ] Navigation works
- [ ] Search works
- [ ] Mobile responsive
- [ ] Dark mode works

## 🎉 Next Steps

1. **Fill in content** - Copy/adapt from README.md
2. **Add examples** - Create pages for each example
3. **Add screenshots** - Make docs visual
4. **Test on mobile** - Ensure responsive
5. **Share the link** - Add to README.md

## 💡 Tips

- **Use pipenv** - Keeps dependencies isolated
- **Always use `pipenv run`** - For all mkdocs commands
- **Commit Pipfile.lock** - Ensures reproducible builds
- **Reuse markdown** - Link to existing files
- **Keep it simple** - Don't over-organize
- **Update with code** - Keep docs in sync
- **Use automation** - Let GitHub Actions handle deployment

## 🚀 Quick Reference

```bash
# Setup
pipenv install

# Development
pipenv run mkdocs serve

# Build
pipenv run mkdocs build

# Deploy
git push  # Auto-deploys via Cloudflare Pages
```

---

**Questions?** Check the [MkDocs Material docs](https://squidfunk.github.io/mkdocs-material/) or open an issue!
