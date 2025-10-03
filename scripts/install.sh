#!/usr/bin/env bash
set -euo pipefail

REPO="${ROCKETSHIP_REPO:-rocketship-ai/rocketship}"
BINARY_NAME="rocketship"
DEFAULT_BIN_DIR="$HOME/.local/bin"
BIN_DIR="${ROCKETSHIP_BIN_DIR:-$DEFAULT_BIN_DIR}"
REQUESTED_VERSION="${ROCKETSHIP_VERSION:-${1:-latest}}"

log() {
  printf '%s\n' "$1"
}

err() {
  printf 'Error: %s\n' "$1" >&2
}

normalize_tag() {
  local tag="$1"
  if [[ "$tag" == "latest" ]]; then
    echo "latest"
    return
  fi
  if [[ "$tag" != v* ]]; then
    tag="v${tag}"
  fi
  echo "$tag"
}

detect_asset() {
  local os arch
  os="$(uname -s)"
  arch="$(uname -m)"
  case "$os" in
    Darwin) os="darwin" ;;
    Linux) os="linux" ;;
    *) err "Unsupported OS: $os"; exit 1 ;;
  esac

  case "$arch" in
    arm64|aarch64) arch="arm64" ;;
    x86_64|amd64) arch="amd64" ;;
    *) err "Unsupported architecture: $arch"; exit 1 ;;
  esac

  echo "${BINARY_NAME}-${os}-${arch}"
}

latest_tag() {
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep -oE '"tag_name"\s*:\s*"([^"]+)"' \
    | sed -E 's/.*"([^"]+)".*/\1/'
}

choose_checksum_tool() {
  if command -v shasum >/dev/null 2>&1; then
    echo "shasum"
    return
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    echo "sha256sum"
    return
  fi
  err "Neither shasum nor sha256sum is available on PATH"
  exit 1
}

ensure_bin_dir() {
  mkdir -p "$BIN_DIR"
}

bin_dir_in_path() {
  case ":$PATH:" in
    *":$BIN_DIR:") return 0 ;;
    *) return 1 ;;
  esac
}

append_path_hint() {
  local target_dir="$1"
  local rc_file
  local shell_path="${SHELL:-}"

  if [[ "$shell_path" == *"zsh"* ]]; then
    rc_file="$HOME/.zshrc"
  elif [[ "$shell_path" == *"bash"* ]]; then
    if [[ "$OSTYPE" == darwin* ]]; then
      rc_file="$HOME/.bash_profile"
    else
      rc_file="$HOME/.bashrc"
    fi
  else
    rc_file="$HOME/.profile"
  fi

  mkdir -p "$(dirname "$rc_file")"
  if [[ ! -f "$rc_file" ]]; then
    touch "$rc_file"
  fi

  local export_line
  if [[ "$target_dir" == "$DEFAULT_BIN_DIR" ]]; then
    export_line='export PATH="$HOME/.local/bin:$PATH"'
  else
    export_line="export PATH=\"${target_dir}:\$PATH\""
  fi
  if ! grep -F "$export_line" "$rc_file" >/dev/null 2>&1; then
    printf '%s\n' "$export_line" >> "$rc_file"
    log "Added $target_dir to PATH in $rc_file. Open a new shell to pick up the change."
  fi
}

clear_quarantine() {
  if [[ "$(uname -s)" == "Darwin" ]] && command -v xattr >/dev/null 2>&1; then
    xattr -d com.apple.quarantine "$1" 2>/dev/null || true
  fi
}

main() {
  ensure_bin_dir

  local tag
  tag="$(normalize_tag "$REQUESTED_VERSION")"
  if [[ "$tag" == "latest" ]]; then
    tag="$(latest_tag)"
    if [[ -z "$tag" ]]; then
      err "Could not determine latest release tag"
      exit 1
    fi
    log "Detected latest release: $tag"
  fi

  local asset
  asset="$(detect_asset)"
  local base="https://github.com/${REPO}/releases/download/${tag}"
  local url="${base}/${asset}"
  local sums_url="${base}/checksums.txt"

  local tmp
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT

  log "Downloading ${asset}"
  curl -fL "$url" -o "$tmp/${asset}"
  log "Downloading checksums.txt"
  curl -fL "$sums_url" -o "$tmp/checksums.txt"

  local checksum_line
  checksum_line="$(grep -F "  ${asset}" "$tmp/checksums.txt" || true)"
  if [[ -z "$checksum_line" ]]; then
    err "Checksum for ${asset} not found in checksums.txt"
    exit 1
  fi

  local tool
  tool="$(choose_checksum_tool)"
  log "Verifying checksum"
  if [[ "$tool" == "shasum" ]]; then
    (cd "$tmp" && printf '%s\n' "$checksum_line" | shasum -a 256 --check >/dev/null) || {
      err "Checksum verification failed"
      exit 1
    }
  else
    (cd "$tmp" && printf '%s\n' "$checksum_line" | sha256sum --check --status) || {
      err "Checksum verification failed"
      exit 1
    }
  fi

  install -m 0755 "$tmp/${asset}" "$BIN_DIR/${BINARY_NAME}"
  clear_quarantine "$BIN_DIR/${BINARY_NAME}"

  if [[ "$BIN_DIR" == "$DEFAULT_BIN_DIR" ]] && ! bin_dir_in_path; then
    append_path_hint "$DEFAULT_BIN_DIR"
  fi

  log "Installed to $BIN_DIR/${BINARY_NAME}"
  "$BIN_DIR/${BINARY_NAME}" --version || true
}

main "$@"
