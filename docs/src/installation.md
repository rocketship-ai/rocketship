# Installation

Rocketship CLI is available for macOS, Linux, and Windows. Choose your platform below for installation instructions.

## Prerequisites

Before installing Rocketship, you need to install Temporal, which is required for the local engine:

```bash
brew install temporal  # macOS
```

For other platforms, please follow the [Temporal installation guide](https://docs.temporal.io/cli#install).

## macOS

### Apple Silicon

```bash
curl -Lo /usr/local/bin/rs https://github.com/rocketship-ai/rocketship/releases/latest/download/rocketship-darwin-arm64 && chmod +x /usr/local/bin/rs
```

### Intel

```bash
curl -Lo /usr/local/bin/rs https://github.com/rocketship-ai/rocketship/releases/latest/download/rocketship-darwin-amd64 && chmod +x /usr/local/bin/rs
```

## Linux

### AMD64

```bash
curl -Lo /usr/local/bin/rs https://github.com/rocketship-ai/rocketship/releases/latest/download/rocketship-linux-amd64 && chmod +x /usr/local/bin/rs
```

### ARM64

```bash
curl -Lo /usr/local/bin/rs https://github.com/rocketship-ai/rocketship/releases/latest/download/rocketship-linux-arm64 && chmod +x /usr/local/bin/rs
```

## Windows

1. Download the latest Windows executable from our [releases page](https://github.com/rocketship-ai/rocketship/releases/latest/download/rocketship-windows-amd64.exe)
2. Rename it to `rs.exe`
3. Move it to a directory in your PATH (e.g., `C:\Windows\System32\`)

## Docker

Rocketship is also available as a Docker image. It will run the tests you specify then exit:

```bash
docker pull rocketshipai/rocketship:latest
```

To run Rocketship using Docker:

```bash
docker run rocketshipai/rocketship:latest
```

## Verifying Installation

To verify your installation, run:

```bash
rs --version
```

## Next Steps

- Check out our [Quick Start Guide](quickstart.md) to run your first test
- Learn about [test specifications](test-specs.md)
- Explore our [examples](examples.md)
