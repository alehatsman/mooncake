# GitHub CLI Preset

Work with GitHub from the command line. Manage repos, PRs, issues, and workflows without leaving the terminal.

## Quick Start

```yaml
# Install only
- preset: gh

# Install and authenticate
- preset: gh
  with:
    configure_auth: true
```

## Authentication

```bash
gh auth login
```

## Common Commands

```bash
# Repository
gh repo create myproject --public
gh repo clone owner/repo
gh repo view

# Pull Requests
gh pr create --title "Fix bug" --body "Description"
gh pr list
gh pr view 123
gh pr checkout 123
gh pr merge 123

# Issues
gh issue create --title "Bug report"
gh issue list
gh issue view 456
gh issue close 456

# Workflows
gh workflow list
gh workflow run build
gh run list
gh run watch

# Releases
gh release create v1.0.0
gh release list
```

## Examples

```bash
# Create repo and push
gh repo create myproject --public --source=. --push

# View PR diff
gh pr diff 123

# Open PR in browser
gh pr view --web

# Create issue from template
gh issue create --template bug_report

# Download release assets
gh release download v1.0.0

# Fork and clone
gh repo fork owner/repo --clone
```

## Resources
- Docs: https://cli.github.com/manual/
- Examples: https://github.com/cli/cli/blob/trunk/docs/examples.md
