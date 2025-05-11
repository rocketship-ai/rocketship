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
   - Use conventional commit messages:
     - `feat:` for new features
     - `fix:` for bug fixes
     - `chore:` for maintenance tasks
     - `docs:` for documentation updates
     - `refactor:` for code refactoring
     - `test:` for adding tests

4. **Testing Your Changes**

   ```bash
   # Build and install your local changes
   make install    # Builds all binaries and installs CLI

   # Test your changes
   rocketship start server --local    # Start required services
   rocketship run --file <path/to/rocketship.yaml> --engine localhost:7700
   ```

## Release Process

1. **Create a Release**

   Once changes are merged to main, a maintainer can create a new release:

   - Go to GitHub Releases: https://github.com/rocketship-ai/rocketship/releases
   - Click "Draft a new release"
   - Create a new tag (e.g., `v1.2.3`) following semantic versioning
   - Write release notes
   - Publish release

   This will automatically:

   - Create and push a Git tag
   - Trigger the release workflow
   - Build platform-specific binaries
   - Attach binaries to the release

2. **Test Installation**

   ```bash
   # Install released version
   go install github.com/rocketship-ai/rocketship/cmd/rocketship@v1.2.3

   # Test basic functionality
   rocketship version
   ```

## Release Artifacts

Each release includes platform-specific binaries for:

- CLI (rocketship)
- Worker
- Engine

Supported platforms:

- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

## Version Numbering

We follow semantic versioning (MAJOR.MINOR.PATCH):

- MAJOR: Breaking changes
- MINOR: New features, backward compatible
- PATCH: Bug fixes, backward compatible

Example: v1.2.3
