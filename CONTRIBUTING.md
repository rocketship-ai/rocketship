# Contributing to Rocketship

This document describes the process for contributing to Rocketship.

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

   - Set the ROCKETSHIP_LOG env var to DEBUG to see more verbose logging
   - All changes must pass linting and tests (`make lint test`)
   - Pre-commit hooks will automatically run these checks
   - Using conventional commit messages would be nice:
     - `feat:` for new features
     - `fix:` for bug fixes
     - `chore:` for maintenance tasks
     - `docs:` for documentation updates
     - `refactor:` for code refactoring
     - `test:` for adding tests

4. **Testing Your Changes**

   ```bash
   # Build and install your local changes
   make install    # Removes old executable and go installs the local version

   # Test your changes
   rocketship start server --local    # Start required services
   # in another session, run the test
   rocketship run --file <path/to/rocketship.yaml> --engine localhost:7700
   ```

5. **BONUS! Run a test server**

   Inside [for-contributors/](https://github.com/rocketship-ai/rocketship/blob/main/for-contributors), you'll a test server. That you can run with

   ```bash
   go run for-contributors/test-server/main.go
   ```

   This will help you test your changes. It has an in-memory store, so it can preserve resources.

## Release Process

1. **Update Default Version**

   Before creating a release, update the default version in `internal/embedded/binaries.go`:

   ```bash
   # Example: For releasing v1.2.3
   sed -i '' 's/defaultVersion *= *".*"/defaultVersion = "v1.2.3"/' internal/embedded/binaries.go
   git add internal/embedded/binaries.go
   git commit -m "chore: update default version to v1.2.3"
   git push
   ```

2. **Create a Release**

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

3. **Test Installation**

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
