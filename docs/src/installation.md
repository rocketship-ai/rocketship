# Installation

Rocketship ships prebuilt binaries for macOS and Linux. Use the Homebrew tap on macOS for the smoothest experience, or the portable installer script everywhere else. This page walks through the supported options, prerequisites, and post-install checks.

## Prerequisites

To run the Rocketship engine locally you need Temporal:

```bash
brew install temporal
```

On Linux follow Temporal's [official installation guide](https://docs.temporal.io/cli#install). If you only connect to a remote Rocketship deployment, Temporal is optional.

## macOS (recommended via Homebrew)

```bash
brew tap rocketship-ai/tap
brew install rocketship
```

The formula installs the latest tagged CLI, handles upgrades with `brew upgrade rocketship`, and keeps the binary inside your Homebrew prefix.

## Linux and macOS (portable installer)

For environments without Homebrew run the installer script:

```bash
curl -fsSL https://raw.githubusercontent.com/rocketship-ai/rocketship/main/scripts/install.sh | bash
```

The script:

- Detects your OS/architecture and downloads the matching release asset
- Verifies the binary against the published `checksums.txt`
- Installs to `~/.local/bin/rocketship` (override via `ROCKETSHIP_BIN_DIR`)
- Removes macOS quarantine attributes when needed
- Appends `~/.local/bin` to your shell `PATH` if it isn't already there

Re-run the script to pick up future releases. To pin a version, set `ROCKETSHIP_VERSION=v0.5.23` (for example) before invoking the script.

## Docker

```bash
docker pull rocketshipai/rocketship:latest
docker run --rm -it rocketshipai/rocketship:latest --help
```

Docker images are useful for CI jobs or ephemeral runs where you don't want to manage binaries.

## Post-install checks

After installing, confirm the CLI works:

```bash
rocketship --version
```

## Next steps

- [Quickstart](quickstart.md) to run your first suite
- [Examples](examples.md) for ready-made specs
- [Test specification reference](yaml-reference/index.md) when you need exact syntax
