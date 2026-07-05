#!/bin/sh
# install.sh installs or updates cc-select from GitHub Releases.
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/matiastang/cc-select/main/scripts/install.sh | sh
#   curl -fsSL ... | sh -s -- --dir /usr/local/bin

set -eu

REPO="matiastang/cc-select"
API_URL="https://api.github.com/repos/${REPO}/releases/latest"

# Print an error message and exit.
err() {
  printf 'error: %s\n' "$1" >&2
  exit 1
}

# Print an informational message.
info() {
  printf '%s\n' "$1"
}

# Detect OS.
detect_os() {
  case "$(uname -s)" in
    Darwin) printf 'darwin' ;;
    Linux)  printf 'linux' ;;
    *)      err "unsupported OS: $(uname -s)" ;;
  esac
}

# Detect architecture.
detect_arch() {
  case "$(uname -m)" in
    x86_64 | amd64) printf 'amd64' ;;
    arm64 | aarch64) printf 'arm64' ;;
    *)               err "unsupported architecture: $(uname -m)" ;;
  esac
}

# Fetch the latest release tag from GitHub API.
# Prefer jq for robust JSON parsing; fall back to grep/sed.
fetch_latest_tag() {
  if command -v jq >/dev/null 2>&1; then
    tag=$(curl -fsSL "${API_URL}" | jq -r '.tag_name // empty')
  else
    tag=$(curl -fsSL "${API_URL}" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
  fi
  if [ -z "$tag" ]; then
    err "failed to fetch latest release tag from ${API_URL}"
  fi
  printf '%s' "$tag"
}

# Download a URL to a destination file.
download() {
  url=$1
  dest=$2
  curl -fSL --retry 3 --retry-delay 1 --progress-bar "$url" -o "$dest" || err "failed to download ${url}"
}

# Verify a file's SHA256 checksum against checksums.txt.
verify_checksum() {
  asset=$1
  checksums=$2
  expected=$(awk -v asset="$asset" '$2 == asset { print $1; exit }' "$checksums")
  if [ -z "$expected" ]; then
    err "could not find checksum for ${asset}"
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "$asset" | awk '{print $1}')
  else
    actual=$(shasum -a 256 "$asset" | awk '{print $1}')
  fi
  if [ "$expected" != "$actual" ]; then
    err "checksum mismatch for ${asset}: expected ${expected}, got ${actual}"
  fi
}

# Check whether a directory is in the user's PATH.
in_path() {
  case ":${PATH}:" in
    *":$1:"*) return 0 ;;
    *)         return 1 ;;
  esac
}

# Determine the installation directory.
resolve_install_dir() {
  # If a command-line --dir argument was provided, use it.
  if [ -n "${INSTALL_DIR:-}" ]; then
    printf '%s' "$INSTALL_DIR"
    return
  fi

  # If cc-select already exists in PATH, install into the same directory.
  existing=$(command -v cc-select 2>/dev/null || true)
  if [ -n "$existing" ]; then
    dir=$(dirname "$existing")
    info "updating existing installation in ${dir}"
    printf '%s' "$dir"
    return
  fi

  # Prefer ~/.local/bin when it exists or can be created and is/will be in PATH.
  home_bin="${HOME}/.local/bin"
  if in_path "$home_bin" || mkdir -p "$home_bin" 2>/dev/null; then
    printf '%s' "$home_bin"
    return
  fi

  # Fall back to /usr/local/bin.
  printf '%s' '/usr/local/bin'
}

# Ensure the target directory exists and is writable; use sudo if needed.
ensure_dir() {
  dir=$1
  if [ -d "$dir" ]; then
    if [ ! -w "$dir" ]; then
      return 1
    fi
    return 0
  fi
  if mkdir -p "$dir" 2>/dev/null; then
    return 0
  fi
  return 1
}

# Parse optional --dir argument.
INSTALL_DIR=""
while [ $# -gt 0 ]; do
  case "$1" in
    --dir)
      if [ -z "${2:-}" ]; then err "--dir requires a value"; fi
      INSTALL_DIR="$2"
      shift 2
      ;;
    --dir=*)
      INSTALL_DIR="${1#*=}"
      shift
      ;;
    -h | --help)
      info "Usage: install.sh [--dir /path/to/bin]"
      exit 0
      ;;
    *)
      err "unknown argument: $1"
      ;;
  esac
done

OS=$(detect_os)
ARCH=$(detect_arch)
TAG=$(fetch_latest_tag)
VERSION=${TAG#v}
ASSET="cc-select_${VERSION}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${TAG}/checksums.txt"

info "Installing cc-select ${TAG} for ${OS}/${ARCH}..."

TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/cc-select-install.XXXXXX")
trap 'rm -rf "$TMP_DIR"' EXIT

cd "$TMP_DIR"

info "Downloading ${ASSET}..."
download "$DOWNLOAD_URL" "$ASSET"

info "Downloading checksums..."
download "$CHECKSUMS_URL" checksums.txt

info "Verifying checksum..."
verify_checksum "$ASSET" checksums.txt

info "Extracting..."
tar -xzf "$ASSET"
if [ ! -f cc-select ]; then
  err "archive did not contain cc-select binary"
fi

INSTALL_DIR=$(resolve_install_dir)

if ! ensure_dir "$INSTALL_DIR"; then
  info "Directory ${INSTALL_DIR} requires elevated permissions."
  if command -v sudo >/dev/null 2>&1; then
    sudo mkdir -p "$INSTALL_DIR"
    sudo install -m 755 cc-select "${INSTALL_DIR}/cc-select"
  else
    err "cannot write to ${INSTALL_DIR} and sudo is not available"
  fi
else
  # Back up existing binary if present.
  if [ -f "${INSTALL_DIR}/cc-select" ]; then
    cp "${INSTALL_DIR}/cc-select" "${INSTALL_DIR}/cc-select.bak"
    info "Backed up existing binary to ${INSTALL_DIR}/cc-select.bak"
  fi
  install -m 755 cc-select "${INSTALL_DIR}/cc-select"
fi

if ! command -v cc-select >/dev/null 2>&1; then
  case ":${PATH}:" in
    *":${INSTALL_DIR}:") ;;
    *)
      info ""
      info "NOTE: ${INSTALL_DIR} is not in your PATH."
      info "Add the following line to your shell profile:"
      info "  export PATH=\"${INSTALL_DIR}:\$PATH\""
      ;;
  esac
fi

installed_version=$("${INSTALL_DIR}/cc-select" --version 2>/dev/null || true)
info ""
info "${installed_version:-cc-select} installed to ${INSTALL_DIR}/cc-select"
info ""
info "To enable shell integration, run one of:"
info "  cc-select init >> ~/.zshrc"
info "  cc-select init >> ~/.bashrc"
info "  cc-select init | Out-File -Append -Encoding utf8 \$PROFILE   # PowerShell"
info ""
info "Then reload your shell or open a new terminal."
