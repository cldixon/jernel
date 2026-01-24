#!/bin/sh
set -e

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
    darwin|linux) ;;
    *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Check for supported combinations
if [ "$OS" = "darwin" ] && [ "$ARCH" = "amd64" ]; then
    echo "Intel Macs are not currently supported. Please build from source."
    exit 1
fi

if [ "$OS" = "linux" ] && [ "$ARCH" = "arm64" ]; then
    echo "Linux arm64 is not currently supported. Please build from source."
    exit 1
fi

# Get latest version from GitHub
REPO="cldixon/jernel"
VERSION=$(curl -sI "https://github.com/$REPO/releases/latest" | grep -i "location:" | sed 's/.*tag\///' | tr -d '\r\n')

if [ -z "$VERSION" ]; then
    echo "Failed to fetch latest version"
    exit 1
fi

# Download
URL="https://github.com/$REPO/releases/download/$VERSION/jernel-${OS}-${ARCH}"
echo "Downloading jernel $VERSION for ${OS}-${ARCH}..."

if ! curl -sfL "$URL" -o /tmp/jernel; then
    echo "Failed to download from $URL"
    exit 1
fi

# Install
chmod +x /tmp/jernel
sudo mv /tmp/jernel /usr/local/bin/jernel

echo "Installed jernel $VERSION to /usr/local/bin/jernel"
jernel --version
