# Installation

Rocketship ships prebuilt binaries for macOS and Linux. Use the Homebrew tap on macOS for the smoothest experience, or the portable installer script everywhere else. This page walks through the supported options, prerequisites, and post-install checks.

## Prerequisites

**Do you need Temporal?**

- **Yes, if** you'll run tests on your own computer using the local engine
- **No, if** you'll connect to a remote Rocketship server (cloud or team server)

**To install Temporal (if needed):**

```bash
# macOS
brew install temporal

# Linux
# Follow Temporal's installation guide: https://docs.temporal.io/cli#install
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

**What the installer does:**
- Detects your operating system and downloads the right version
- Verifies the download is safe (checksums)
- Installs Rocketship to `~/.local/bin/rocketship`
- Sets up your PATH so you can run `rocketship` from anywhere

**To update later:** Just run the installer script again - it will download the latest version.

**To install a specific version:** Set `ROCKETSHIP_VERSION=v0.5.23` (for example) before running the script.

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
- [Test specification reference](yaml-reference/plugin-reference.md) when you need exact syntax
