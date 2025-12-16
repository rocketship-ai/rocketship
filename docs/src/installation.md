# Installation

This guide will help you install Rocketship on your computer. We provide ready-to-use versions for macOS and Linux.

**What you'll need:**
- A macOS or Linux computer (Windows users can use Docker or WSL)
- Internet connection to download the software
- Terminal/command line access

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

Temporal is a tool that helps Rocketship manage long-running tests reliably.

## macOS (recommended via Homebrew)

```bash
brew tap rocketship-ai/tap
brew install rocketship
```

**Benefits of Homebrew:**
- Easy updates: run `brew upgrade rocketship` to get new versions
- Automatic dependency management
- Keeps everything organized in one place

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
- [Examples](examples.md) for ready-made specs
- [Test specification reference](yaml-reference/plugin-reference.md) when you need exact syntax
