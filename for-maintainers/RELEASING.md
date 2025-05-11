# Releasing Rocketship

This document describes the process for developing and releasing new versions of Rocketship.

## Development Process

1. **Create a Feature Branch**

   ```bash
   git checkout -b feature-name
   ```

2. **Set Up Development Environment**

   ```bash
   make dev-setup
   ```

   This will:

   - Set up Git hooks for linting and testing
   - Build initial binaries
   - Prepare your environment for development

3. **Development Guidelines**

   - All changes must pass linting and tests (`make lint test`)
   - Pre-commit hooks will automatically run these checks
   - It's helpful for reviewers to use conventional commit messages:
     - `feat:` for new features
     - `fix:` for bug fixes
     - `chore:` for maintenance tasks
     - `docs:` for documentation updates
     - `refactor:` for code refactoring
     - `test:` for adding tests

4. **Testing Your Changes**

   - Run unit tests: `make test`
   - Run linting: `make lint`
   - Test local changes:

     ```bash
     # Build and install your local changes
     make install    # Builds all binaries and installs CLI to /usr/local/bin

     # Test the your CLI changes
     rocketship start    # Start required services
     rocketship run      # Execute your tests
     ```

   - Test with Docker: `make compose-up`

## Release Process

1. **Prepare for Release**

   - Ensure all changes are merged to main
   - Pull latest changes:
     ```bash
     git checkout main
     git pull origin main
     ```
   - UPDATE VERSION (versionTagToRelease) IN `internal/embedded/binaries.go`
   - Commit version update:
     ```bash
     git add internal/embedded/binaries.go
     git commit -m "chore: update version to vX.Y.Z"
     git push origin main
     ```

2. **Create Release**

   ```bash
   ./scripts/release.sh vX.Y.Z
   ```

   This will:

   - Create and push a Git tag
   - Trigger GitHub Actions workflow
   - Build platform-specific binaries
   - Create GitHub release

3. **Monitor Release**

   - Watch the GitHub Actions workflow at:
     https://github.com/rocketship-ai/rocketship/actions
   - Verify the release is created at:
     https://github.com/rocketship-ai/rocketship/releases

4. **Test Installation**

   ```bash
   # Remove existing installation
   rm -f $(which rocketship)
   rm -rf ~/.cache/rocketship

   # Install new version
   go install github.com/rocketship-ai/rocketship/cmd/rocketship@vX.Y.Z

   # Test basic functionality
   rocketship version
   ```

## Release Artifacts

Each release includes:

- CLI binaries for multiple platforms
- Worker binaries for multiple platforms
- Engine binaries for multiple platforms

Supported platforms:

- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

## Troubleshooting

1. **Failed GitHub Actions**

   - Check workflow logs for errors
   - Common issues:
     - Linting failures
     - Test failures
     - Build errors
     - Missing permissions

2. **Installation Issues**

   - Verify the release exists on GitHub
   - Check binary permissions
   - Clear cache directory: `rm -rf ~/.cache/rocketship`

3. **Binary Download Issues**
   - Check URL format in `internal/embedded/binaries.go`
   - Verify binary names match release artifacts
   - Check platform-specific binary naming

## Version Numbering

We follow semantic versioning (MAJOR.MINOR.PATCH):

- MAJOR: Breaking changes
- MINOR: New features, backward compatible
- PATCH: Bug fixes, backward compatible

Example: v1.2.3

- 1: Major version
- 2: Minor version
- 3: Patch version
