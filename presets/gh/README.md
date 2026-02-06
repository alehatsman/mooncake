# gh - GitHub CLI

Manage GitHub repos, PRs, issues, and Actions from the command line. Native GitHub integration for CI/CD workflows.

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
# Interactive login
gh auth login

# With token
gh auth login --with-token < token.txt

# Check status
gh auth status

# Switch accounts
gh auth switch

# Logout
gh auth logout
```

## Repository Operations
```bash
# Create repo
gh repo create myproject --public
gh repo create myorg/myproject --private

# Create and initialize
gh repo create myproject --public --source=. --push

# Clone
gh repo clone owner/repo
gh repo clone owner/repo target-dir

# View repo
gh repo view
gh repo view owner/repo
gh repo view owner/repo --web

# Fork
gh repo fork owner/repo
gh repo fork owner/repo --clone

# Archive/delete
gh repo archive owner/repo
gh repo delete owner/repo --yes
```

## Pull Requests
```bash
# Create PR
gh pr create --title "Fix bug" --body "Description"
gh pr create --draft
gh pr create --web

# Create with labels/reviewers
gh pr create --label bug --reviewer @alice --assignee @me

# List PRs
gh pr list
gh pr list --state closed
gh pr list --author @me
gh pr list --label bug

# View PR
gh pr view 123
gh pr view 123 --web
gh pr diff 123
gh pr checks 123

# Checkout PR
gh pr checkout 123

# Review PR
gh pr review 123 --approve
gh pr review 123 --request-changes --body "Needs tests"
gh pr review 123 --comment --body "LGTM"

# Merge PR
gh pr merge 123 --merge
gh pr merge 123 --squash
gh pr merge 123 --rebase
gh pr merge 123 --auto --squash

# Close/reopen
gh pr close 123
gh pr reopen 123

# PR status
gh pr status
```

## Issues
```bash
# Create issue
gh issue create --title "Bug report" --body "Description"
gh issue create --web
gh issue create --template bug_report

# Add metadata
gh issue create --label bug --assignee @me --milestone v1.0

# List issues
gh issue list
gh issue list --state closed
gh issue list --author @me
gh issue list --label bug

# View issue
gh issue view 456
gh issue view 456 --web

# Edit issue
gh issue edit 456 --add-label priority
gh issue edit 456 --title "New title"

# Close/reopen
gh issue close 456
gh issue reopen 456

# Comment
gh issue comment 456 --body "Working on this"

# Status
gh issue status
```

## GitHub Actions
```bash
# List workflows
gh workflow list

# Run workflow
gh workflow run build
gh workflow run deploy --ref main
gh workflow run build --field environment=prod

# View workflow
gh workflow view build
gh workflow view build --web

# List runs
gh run list
gh run list --workflow=build
gh run list --branch=main

# View run
gh run view 789
gh run view 789 --log

# Watch run
gh run watch 789

# Download artifacts
gh run download 789
gh run download 789 --name build-artifacts

# Rerun
gh run rerun 789
gh run rerun 789 --failed

# Cancel
gh run cancel 789
```

## Releases
```bash
# Create release
gh release create v1.0.0
gh release create v1.0.0 --notes "Release notes"
gh release create v1.0.0 --draft
gh release create v1.0.0 dist/* --generate-notes

# List releases
gh release list
gh release list --limit 10

# View release
gh release view v1.0.0
gh release view latest

# Download assets
gh release download v1.0.0
gh release download v1.0.0 --pattern '*.tar.gz'

# Upload assets
gh release upload v1.0.0 dist/*

# Delete release
gh release delete v1.0.0 --yes
```

## CI/CD Integration
```bash
# Check PR status in CI
if gh pr view $PR_NUMBER --json state -q .state | grep -q OPEN; then
  echo "PR is open"
fi

# Auto-merge when CI passes
gh pr merge $PR_NUMBER --auto --squash

# Create release in CI
gh release create v${VERSION} \
  --title "Release ${VERSION}" \
  --generate-notes \
  dist/*

# Comment on PR from CI
gh pr comment $PR_NUMBER --body "✅ Tests passed"

# Set PR status
gh pr ready 123  # Mark ready for review
gh pr ready 123 --undo  # Convert to draft

# Trigger workflow from CI
gh workflow run deploy --field version=$VERSION
```

## GitHub Actions YAML
```yaml
# Create PR workflow
- name: Create Pull Request
  shell: |
    gh pr create \
      --title "Auto-update dependencies" \
      --body "Automated PR from workflow" \
      --label dependencies

# Check workflow status
- name: Wait for CI
  shell: |
    gh run watch $(gh run list --branch $BRANCH --limit 1 --json databaseId -q '.[0].databaseId')

# Release workflow
- name: Create Release
  shell: |
    gh release create v${{ env.VERSION }} \
      --generate-notes \
      ./dist/*
```

## GitHub API
```bash
# Get API response
gh api repos/owner/repo

# Pagination
gh api --paginate repos/owner/repo/issues

# POST request
gh api repos/owner/repo/issues \
  -f title="Bug report" \
  -f body="Description"

# GraphQL
gh api graphql -f query='
  query {
    viewer {
      login
    }
  }
'

# With jq
gh api repos/owner/repo/pulls --jq '.[] | .title'
```

## Gists
```bash
# Create gist
gh gist create file.txt
gh gist create file.txt --public
gh gist create file1.txt file2.txt --desc "My gist"

# List gists
gh gist list

# View gist
gh gist view abc123

# Edit gist
gh gist edit abc123

# Delete gist
gh gist delete abc123
```

## Projects
```bash
# List projects
gh project list

# View project
gh project view 1

# Add item to project
gh project item-add 1 --owner @me --url https://github.com/owner/repo/issues/123
```

## Advanced Workflows
```bash
# PR review workflow
gh pr list --author @me --json number,title | \
  jq -r '.[] | "\(.number): \(.title)"' | \
  fzf | cut -d: -f1 | \
  xargs gh pr view

# Bulk label assignment
gh issue list --label needs-triage --json number -q '.[].number' | \
  xargs -I {} gh issue edit {} --add-label triaged

# Release notes from PRs
gh pr list --state merged --search "merged:>2024-01-01" \
  --json number,title,author \
  --jq '.[] | "- \(.title) (#\(.number)) @\(.author.login)"'

# Check PR conflicts
gh pr list --json number,mergeable | \
  jq -r '.[] | select(.mergeable=="CONFLICTING") | .number' | \
  xargs -I {} gh pr view {}
```

## Configuration
```bash
# Set default editor
gh config set editor vim

# Set default protocol
gh config set git_protocol ssh

# View config
gh config list

# Set aliases
gh alias set pv 'pr view'
gh alias set co 'pr checkout'

# Environment variables
export GH_PAGER=less
export GH_EDITOR=vim
export GH_TOKEN=ghp_xxx
```

## Comparison
| Feature | gh | Web UI | git CLI |
|---------|-----|--------|---------|
| Speed | Fast | Slow | N/A |
| CI/CD | Excellent | Poor | No |
| Offline | Limited | No | Yes |
| PRs/Issues | Native | Native | No |
| Scripting | Easy | No | N/A |

## Best Practices
- Use `--json` for scripting
- Set `GH_TOKEN` in CI/CD
- Use `--auto` for auto-merge
- Combine with `jq` for filtering
- Use aliases for common commands
- Enable `--generate-notes` for releases
- Use `gh api` for advanced automation

## Tips
- `gh pr status` shows your PRs
- `--web` opens in browser
- Use `gh browse` to open repo
- `gh run watch` shows live logs
- JSON output is stable for scripts
- Works with GitHub Enterprise
- Supports multiple accounts

## Agent Use
- Automated PR creation/review
- CI/CD pipeline integration
- Release automation
- Issue triage workflows
- Workflow triggering
- Repository management

## Uninstall
```yaml
- preset: gh
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/cli/cli
- Docs: https://cli.github.com/manual/
- Search: "gh cli examples", "gh automation"
