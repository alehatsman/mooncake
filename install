#!/bin/sh
# Mooncake installer — POSIX sh, no bash-isms required
# Usage:
#   curl -sSL https://raw.githubusercontent.com/alehatsman/mooncake/master/install.sh | sh
# Environment variables:
#   VERSION     — pin a specific release tag, e.g. VERSION=v1.2.0
#   INSTALL_DIR — override install location (default: /usr/local/bin)

set -e

REPO="alehatsman/mooncake"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# ──────────────────────────────────────────────
# Helpers
# ──────────────────────────────────────────────

info()  { printf '\033[1;34m==> \033[0m%s\n' "$*"; }
ok()    { printf '\033[1;32m ✓  \033[0m%s\n' "$*"; }
die()   { printf '\033[1;31mERROR: \033[0m%s\n' "$*" >&2; exit 1; }

need() {
    command -v "$1" >/dev/null 2>&1 || die "Required command not found: $1"
}

download() {
    url="$1"
    dest="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -sSL --fail -o "$dest" "$url"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "$dest" "$url"
    else
        die "Neither curl nor wget is available. Please install one and retry."
    fi
}

# ──────────────────────────────────────────────
# Detect OS
# ──────────────────────────────────────────────

detect_os() {
    _uname="$(uname -s)"
    case "$_uname" in
        Linux*)   echo "Linux"   ;;
        Darwin*)  echo "Darwin"  ;;
        MINGW*|MSYS*|CYGWIN*)
                  echo "Windows" ;;
        *)        die "Unsupported operating system: $_uname" ;;
    esac
}

# ──────────────────────────────────────────────
# Detect architecture
# ──────────────────────────────────────────────

detect_arch() {
    _machine="$(uname -m)"
    case "$_machine" in
        x86_64|amd64)        echo "x86_64" ;;
        aarch64|arm64)       echo "arm64"  ;;
        armv7l)              echo "armv7"  ;;
        i386|i686)           echo "i386"   ;;
        *)                   die "Unsupported architecture: $_machine" ;;
    esac
}

# ──────────────────────────────────────────────
# Resolve version
# ──────────────────────────────────────────────

resolve_version() {
    if [ -n "$VERSION" ]; then
        echo "$VERSION"
        return
    fi

    info "Fetching latest release version..."
    _api_url="https://api.github.com/repos/${REPO}/releases/latest"

    if command -v curl >/dev/null 2>&1; then
        _tag=$(curl -sSL "$_api_url" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
    elif command -v wget >/dev/null 2>&1; then
        _tag=$(wget -qO- "$_api_url" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
    else
        die "Neither curl nor wget is available. Please install one and retry."
    fi

    [ -n "$_tag" ] || die "Could not determine latest release. Set VERSION= to install a specific version."
    echo "$_tag"
}

# ──────────────────────────────────────────────
# Main
# ──────────────────────────────────────────────

main() {
    OS="$(detect_os)"
    ARCH="$(detect_arch)"
    VERSION="$(resolve_version)"

    info "Installing mooncake ${VERSION} (${OS}/${ARCH})"

    # Build asset name matching goreleaser defaults
    if [ "$OS" = "Windows" ]; then
        ARCHIVE="${VERSION}/mooncake_${OS}_${ARCH}.zip"
        BINARY="mooncake.exe"
    else
        ARCHIVE="${VERSION}/mooncake_${OS}_${ARCH}.tar.gz"
        BINARY="mooncake"
    fi

    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${ARCHIVE}"

    # Temp directory for download/extraction
    TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t mooncake-install)"
    trap 'rm -rf "$TMP_DIR"' EXIT

    ARCHIVE_FILE="${TMP_DIR}/mooncake-archive"

    info "Downloading ${DOWNLOAD_URL}..."
    download "$DOWNLOAD_URL" "$ARCHIVE_FILE"
    ok "Download complete"

    info "Extracting archive..."
    if [ "$OS" = "Windows" ]; then
        need unzip
        unzip -qo "$ARCHIVE_FILE" -d "$TMP_DIR"
    else
        need tar
        tar -xzf "$ARCHIVE_FILE" -C "$TMP_DIR"
    fi

    BINARY_SRC="${TMP_DIR}/${BINARY}"
    [ -f "$BINARY_SRC" ] || die "Binary not found in archive (looked for: ${BINARY})"

    info "Installing to ${INSTALL_DIR}/${BINARY}..."
    # Create install dir if missing (may require sudo — user's responsibility)
    mkdir -p "$INSTALL_DIR" 2>/dev/null || true
    cp "$BINARY_SRC" "${INSTALL_DIR}/${BINARY}"
    chmod +x "${INSTALL_DIR}/${BINARY}"

    ok "mooncake ${VERSION} installed to ${INSTALL_DIR}/${BINARY}"

    # Sanity check
    if command -v mooncake >/dev/null 2>&1; then
        ok "Installation verified: $(mooncake --version 2>/dev/null || echo 'OK')"
    else
        printf '\n  NOTE: %s is not on your PATH.\n' "$INSTALL_DIR"
        printf '  Add this to your shell profile and restart your terminal:\n'
        printf '    export PATH="%s:$PATH"\n\n' "$INSTALL_DIR"
    fi
}

main
