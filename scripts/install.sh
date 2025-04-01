#!/bin/bash

# upctl installation script
# Usage: curl -fsSL https://raw.githubusercontent.com/uptime-labs/upctl/main/scripts/install.sh | bash

set -e

# Functions
download_upctl() {
  echo "Downloading upctl..."
  
  # Determine OS and architecture
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
  ARCH=$(uname -m)
  
  if [ "$ARCH" = "x86_64" ]; then
    ARCH="amd64"
  elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
    ARCH="arm64"
  else
    echo "Unsupported architecture: $ARCH"
    exit 1
  fi
  
  # Get latest version if not specified
  if [ -z "$VERSION" ]; then
    VERSION=$(curl -s https://api.github.com/repos/uptime-labs/upctl/releases/latest | grep -oP '"tag_name": "\K(.*)(?=")')
    VERSION=${VERSION#v}  # Remove 'v' prefix if present
  fi
  
  DOWNLOAD_URL="https://github.com/uptime-labs/upctl/releases/download/v${VERSION}/upctl_${VERSION}_${OS}_${ARCH}"
  
  echo "Downloading upctl version ${VERSION} for ${OS} ${ARCH}..."
  
  # Download binary
  curl -L -o "/tmp/upctl" "$DOWNLOAD_URL"
  chmod +x "/tmp/upctl"
}

install_upctl() {
  echo "Installing upctl to /usr/local/bin..."
  
  # Install binary
  sudo mv "/tmp/upctl" "/usr/local/bin/upctl"
  
  # Create config directory if it doesn't exist
  if [ ! -f "$HOME/.upctl.yaml" ]; then
    echo "Creating initial config file..."
    mkdir -p "$HOME/.upctl"
    
    # Download sample config if not exists
    curl -L -o "$HOME/.upctl.yaml" "https://raw.githubusercontent.com/uptime-labs/upctl/main/upctl.yaml"
  fi
}

# Main script
echo "upctl installer"
echo "---------------"

# Allow VERSION override
VERSION=${VERSION:-""}

download_upctl
install_upctl

echo "Installation complete! upctl has been installed to /usr/local/bin/upctl"
echo "You can get started with: upctl --help"