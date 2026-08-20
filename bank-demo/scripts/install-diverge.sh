#!/usr/bin/env bash
set -euo pipefail

# Install the Diverge CLI from GitHub releases
# Usage: ./install-diverge.sh [version]
#   e.g. ./install-diverge.sh v0.6.1

VERSION="${1:-latest}"
REPO="divergedev/diverge"
INSTALL_DIR="${INSTALL_DIR:-./bin}"

# Detect OS and arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" && exit 1 ;;
esac

# Resolve latest version
if [ "$VERSION" = "latest" ]; then
  VERSION=$(gh release view --repo "$REPO" --json tagName -q .tagName 2>/dev/null || \
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | jq -er '.tag_name')
fi

TARBALL="diverge_${VERSION#v}_${OS}_${ARCH}.tar.gz"
echo "Installing diverge ${VERSION} (${OS}/${ARCH})..."

# Download and extract
mkdir -p "$INSTALL_DIR"
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

if command -v gh &>/dev/null; then
  gh release download "$VERSION" --repo "$REPO" --pattern "$TARBALL" -D "$TMPDIR"
  gh release download "$VERSION" --repo "$REPO" --pattern "checksums.txt" -D "$TMPDIR"
else
  curl -fsSL "https://github.com/${REPO}/releases/download/${VERSION}/${TARBALL}" -o "${TMPDIR}/${TARBALL}"
  curl -fsSL "https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt" -o "${TMPDIR}/checksums.txt"
fi

(cd "$TMPDIR" && sha256sum --ignore-missing -c checksums.txt) || {
  echo "Checksum verification failed!" >&2
  exit 1
}

tar xzf "${TMPDIR}/${TARBALL}" -C "$TMPDIR"
cp "${TMPDIR}/diverge" "${INSTALL_DIR}/diverge"
chmod +x "${INSTALL_DIR}/diverge"

echo "✅ Installed: ${INSTALL_DIR}/diverge ($("${INSTALL_DIR}/diverge" version 2>/dev/null || echo "$VERSION"))"
