# Release Process

Mooncake uses [GoReleaser](https://goreleaser.com/) for automated releases.

## Creating a Release

1. **Tag the release:**
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```

2. **Automated process:**
   - GitHub Actions automatically triggers
   - Runs tests
   - Builds binaries for all platforms
   - Creates GitHub release with changelog
   - Uploads all artifacts

## What Gets Built

GoReleaser automatically builds for:
- **Linux**: amd64, arm64, arm, 386
- **macOS**: amd64 (Intel), arm64 (Apple Silicon)
- **Windows**: amd64, arm, 386

## What Gets Published

Each release includes:
- ✓ Compiled binaries for all platforms
- ✓ Archived releases (`.tar.gz` for Linux/macOS, `.zip` for Windows)
- ✓ Checksums file for verification
- ✓ Auto-generated changelog
- ✓ Release notes

## Versioning

Use semantic versioning:
- `v1.0.0` - Major release
- `v1.1.0` - Minor release (new features)
- `v1.0.1` - Patch release (bug fixes)

## Testing Locally

Test the release process without publishing:

```bash
# Install goreleaser
go install github.com/goreleaser/goreleaser@latest

# Test the build
goreleaser build --snapshot --clean

# Test the full release (doesn't publish)
goreleaser release --snapshot --clean
```

## Commit Message Format

For better changelogs, use conventional commits:
- `feat: add new feature` → Features section
- `fix: resolve bug` → Bug fixes section
- `docs: update readme` → Excluded from changelog
- `chore: update deps` → Excluded from changelog

## Example Release

```bash
# Make your changes
git add .
git commit -m "feat: add dry-run mode"

# Create release
git tag -a v1.2.0 -m "Release v1.2.0: Add dry-run mode"
git push origin v1.2.0

# Wait for GitHub Actions to complete
# Release will appear at: https://github.com/alehatsman/mooncake/releases
```

## Rollback

If a release has issues:

```bash
# Delete the tag locally
git tag -d v1.0.0

# Delete the tag remotely
git push origin :refs/tags/v1.0.0

# Delete the GitHub release manually from the web interface
```

## Optional: Homebrew

To publish to Homebrew, uncomment the `brews` section in `.goreleaser.yml` and create a tap repository.
