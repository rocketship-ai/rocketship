# Installation

Rocketship ships prebuilt binaries for macOS, Linux, and Windows. Use the Homebrew tap on macOS for the smoothest experience, or the portable installer script everywhere else. This page walks through the supported options, prerequisites, and post-install checks.

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

## Windows

1. Download `rocketship-windows-amd64.exe` from the [latest release](https://github.com/rocketship-ai/rocketship/releases/latest)
2. Rename it to `rocketship.exe`
3. Place it somewhere on your `PATH` (e.g. `C:\\Users\\<you>\\AppData\\Local\\Microsoft\\WindowsApps`)

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
rocketship doctor
```

`rocketship doctor` inspects your `PATH`, config directory permissions, file ownership, and (on macOS) quarantine state, printing exact remediation steps if anything is off.

Configuration lives under the path returned by `os.UserConfigDir()`—for example `~/Library/Application Support/Rocketship` on macOS or `~/.config/rocketship` on Linux. Directories are created with `0700` permissions and the config file with `0600`.

Rocketship refuses to run as root unless you set `ROCKETSHIP_ALLOW_ROOT=1`. This avoids leaving behind root-owned config files that can break normal usage.

## Optional quality-of-life tweaks

Alias the command if you prefer a shorter entry point:

```bash
# macOS/Linux shell rc
alias rs="rocketship"
```

```powershell
# Windows PowerShell profile
Set-Alias -Name rs -Value rocketship
```

## Next steps

- [Quickstart](quickstart.md) to run your first suite
- [Examples](examples.md) for ready-made specs
- [Test specification reference](yaml-reference/index.md) when you need exact syntax
