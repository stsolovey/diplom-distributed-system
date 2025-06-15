#!/bin/bash
set -e

echo "=== Installing k6 Load Testing Tool ==="

# Detect OS
OS=$(uname -s)
ARCH=$(uname -m)

if [ "$OS" = "Linux" ]; then
    if [ "$ARCH" = "x86_64" ]; then
        echo "Installing k6 for Linux AMD64..."
        
        # Download k6
        curl -LO https://github.com/grafana/k6/releases/latest/download/k6-linux-amd64.tar.gz
        tar -xzf k6-linux-amd64.tar.gz
        sudo mv k6-linux-amd64/k6 /usr/local/bin/k6
        rm -rf k6-linux-amd64*
        
    elif [ "$ARCH" = "aarch64" ]; then
        echo "Installing k6 for Linux ARM64..."
        
        curl -LO https://github.com/grafana/k6/releases/latest/download/k6-linux-arm64.tar.gz
        tar -xzf k6-linux-arm64.tar.gz
        sudo mv k6-linux-arm64/k6 /usr/local/bin/k6
        rm -rf k6-linux-arm64*
    fi
    
elif [ "$OS" = "Darwin" ]; then
    echo "Installing k6 for macOS..."
    
    if command -v brew &> /dev/null; then
        brew install k6
    else
        if [ "$ARCH" = "arm64" ]; then
            curl -LO https://github.com/grafana/k6/releases/latest/download/k6-macos-arm64.zip
            unzip k6-macos-arm64.zip
            sudo mv k6-macos-arm64/k6 /usr/local/bin/k6
            rm -rf k6-macos-arm64*
        else
            curl -LO https://github.com/grafana/k6/releases/latest/download/k6-macos-amd64.zip
            unzip k6-macos-amd64.zip
            sudo mv k6-macos-amd64/k6 /usr/local/bin/k6
            rm -rf k6-macos-amd64*
        fi
    fi
else
    echo "Unsupported OS: $OS"
    exit 1
fi

# Verify installation
echo "Verifying k6 installation..."
k6 version

echo "✅ k6 installed successfully!"
echo ""
echo "Usage examples:"
echo "  k6 run k6/scenarios/smoke.js"
echo "  k6 run k6/scenarios/load.js"
echo "  k6 run k6/scenarios/spike.js" 