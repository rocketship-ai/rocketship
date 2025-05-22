# Contributing to Rocketship

Thank you for your interest in contributing to Rocketship! We're excited to have you join our community. This document provides guidelines and instructions for contributing to the project.

## Getting Started

1. Fork the repository on GitHub
2. Clone your fork locally:

```bash
git clone https://github.com/YOUR_USERNAME/rocketship.git
cd rocketship
```

Set up your development environment:

```bash
make dev-setup
```

## Development Workflow

Create a new branch for your feature/fix:

```bash
git checkout -b feature/your-feature-name
```

Make your changes and ensure tests pass:

```bash
make test
make lint
```

Build and install your local changes:

```bash
make install    # Removes old executable and go installs the local version
```

Test your changes:

```bash
# OPTION 1: this will start the local rocketship server, run all tests in the examples directory, and then stop the server
rocketship run -ad examples

# OPTION 2: run the test server in a separate session and then run the tests
rocketship start server --local
# in another session, run the test(s)
rocketship run -f <path/to/rocketship.yaml>
```

Test Server for Development:

Inside [for-contributors/](https://github.com/rocketship-ai/rocketship/blob/main/for-contributors), you'll find a test HTTP server that you can run as an in-memory store to test changes. Make sure your rocketship.yaml files point to this server (localhost:8080):

```bash
cd ./for-contributors/test-server && go run .
```

This will help you test your changes with an in-memory store that can preserve resources.

## Creating Plugins

Rocketship's plugin system allows you to add support for new APIs and protocols. To create a new plugin:

1. Add your plugin in `internal/plugins/`
2. Implement the required plugin interface
3. Register your plugin in the plugin registry
4. Add tests for your plugin
5. Document your plugin's usage

## Documentation

If you're updating features or adding new ones, please update the documentation:

- Documentation is written in Markdown under `docs/src/`
- Run the documentation server locally:

```bash
make docs-serve
```

View your changes at `http://localhost:8000`

## Code Style

- Follow Go best practices and conventions
- Use `gofmt` to format your code
- Add comments for non-obvious code sections
- Write meaningful commit messages

## Development Tips

- Set the `ROCKETSHIP_LOG` env var to `DEBUG` to see more verbose logging
- Pre-commit hooks will automatically run linting and tests
- Always test your changes locally before submitting a PR

## Release Process

**Update Default Version**

Before creating a release, update the default version in `internal/embedded/binaries.go`:

```bash
# Example: For releasing v1.2.3
sed -i '' 's/defaultVersion *= *".*"/defaultVersion = "v1.2.3"/' internal/embedded/binaries.go
git add internal/embedded/binaries.go
git commit -m "chore: update default version to v1.2.3"
git push
```

**Create a Release**

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

**Test Installation**

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

## Getting Help

- Open an issue on GitHub
- Reach out to me on [LinkedIn](https://www.linkedin.com/in/magiusdarrigo)

## License

By contributing to Rocketship, you agree that your contributions will be licensed under the MIT License.
