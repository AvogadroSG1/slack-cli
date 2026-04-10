#!/usr/bin/env bash
set -euo pipefail

BINARY_NAME="slack-cli"

info() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33mwarning:\033[0m %s\n' "$*"; }
error() { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

# Pick install directory: explicit override > GOPATH/bin > /usr/local/bin
if [ -n "${INSTALL_DIR:-}" ]; then
    : # use what the user set
elif [ -n "${GOPATH:-}" ] && echo "$PATH" | grep -q "$GOPATH/bin"; then
    INSTALL_DIR="$GOPATH/bin"
elif echo "$PATH" | grep -q "$(go env GOPATH)/bin"; then
    INSTALL_DIR="$(go env GOPATH)/bin"
else
    INSTALL_DIR="/usr/local/bin"
fi

# Check prerequisites
command -v go >/dev/null 2>&1 || error "Go is not installed. Install it from https://go.dev/dl/"

GO_VERSION=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | sed 's/go//')
MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
MINOR=$(echo "$GO_VERSION" | cut -d. -f2)
if [ "$MAJOR" -lt 1 ] || { [ "$MAJOR" -eq 1 ] && [ "$MINOR" -lt 21 ]; }; then
    error "Go 1.21+ is required (found $GO_VERSION)"
fi

# Build
info "Building $BINARY_NAME..."
cd "$(dirname "$0")"

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS="-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}"

go build -ldflags "$LDFLAGS" -o "bin/$BINARY_NAME" ./cmd/slack-cli

# Install
mkdir -p "$INSTALL_DIR"
info "Installing to $INSTALL_DIR/$BINARY_NAME..."
if [ -w "$INSTALL_DIR" ]; then
    cp "bin/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
else
    info "Requires sudo to write to $INSTALL_DIR"
    sudo cp "bin/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
fi

# Verify
if command -v "$BINARY_NAME" >/dev/null 2>&1; then
    info "Installed successfully:"
    "$BINARY_NAME" version
else
    warn "$INSTALL_DIR is not in your PATH."
    SHELL_NAME=$(basename "$SHELL")
    case "$SHELL_NAME" in
        zsh)  RC_FILE="$HOME/.zshrc" ;;
        bash) RC_FILE="$HOME/.bashrc" ;;
        *)    RC_FILE="your shell rc file" ;;
    esac
    echo ""
    echo "  Add this to $RC_FILE:"
    echo "    export PATH=\"$INSTALL_DIR:\$PATH\""
    echo ""
    echo "  Then reload:"
    echo "    source $RC_FILE"
    echo ""
    info "Binary is at: $INSTALL_DIR/$BINARY_NAME"
fi

echo ""
info "Set SLACK_TOKEN to get started:"
echo "  export SLACK_TOKEN=xoxp-your-token-here"
echo "  $BINARY_NAME conversations list --pretty"
