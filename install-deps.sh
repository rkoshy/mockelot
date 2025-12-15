#!/bin/bash
# Install build dependencies for Mockelot
# Automatically detects platform and installs correct packages

set -e

echo "Mockelot Dependency Installer"
echo "=============================="
echo ""

# Detect platform
if [ -x ./detect-platform.sh ]; then
    PLATFORM=$(./detect-platform.sh --platform)
    echo "Detected platform: $PLATFORM"
else
    # Fallback detection
    if [ -f /etc/os-release ]; then
        source /etc/os-release
        if [ "$ID" = "debian" ]; then
            if [ "$VERSION_ID" = "12" ]; then
                PLATFORM="debian12"
            elif [ "$VERSION_ID" = "13" ]; then
                PLATFORM="debian13"
            fi
        elif [ "$ID" = "ubuntu" ]; then
            # Try to detect based on webkit
            if command -v pkg-config &> /dev/null; then
                if pkg-config --exists webkit2gtk-4.1; then
                    PLATFORM="debian13"
                elif pkg-config --exists webkit2gtk-4.0; then
                    PLATFORM="debian12"
                else
                    # Guess based on Ubuntu version
                    MAJOR=$(echo "$VERSION_ID" | cut -d. -f1)
                    if [ "$MAJOR" -ge "24" ]; then
                        PLATFORM="debian13"
                    else
                        PLATFORM="debian12"
                    fi
                fi
            fi
        fi
    fi
fi

echo ""

# Install based on platform
case "$PLATFORM" in
    debian12)
        echo "Installing dependencies for Debian 12 / Ubuntu 22.04 (webkit2gtk-4.0)..."
        echo ""
        sudo apt-get update
        sudo apt-get install -y \
            build-essential \
            libgtk-3-dev \
            libwebkit2gtk-4.0-dev \
            pkg-config \
            git \
            curl \
            wget
        ;;
    debian13)
        echo "Installing dependencies for Debian 13 / Ubuntu 24.04 (webkit2gtk-4.1)..."
        echo ""
        sudo apt-get update
        sudo apt-get install -y \
            build-essential \
            libgtk-3-dev \
            libwebkit2gtk-4.1-dev \
            pkg-config \
            git \
            curl \
            wget
        ;;
    *)
        echo "Unknown platform: $PLATFORM"
        echo ""
        echo "Please install dependencies manually:"
        echo ""
        echo "For Debian 12 / Ubuntu 22.04:"
        echo "  sudo apt-get install build-essential libgtk-3-dev libwebkit2gtk-4.0-dev"
        echo ""
        echo "For Debian 13 / Ubuntu 24.04:"
        echo "  sudo apt-get install build-essential libgtk-3-dev libwebkit2gtk-4.1-dev"
        exit 1
        ;;
esac

echo ""
echo "✓ Dependencies installed successfully!"
echo ""
echo "Next steps:"
echo "  1. Install Go (if not already installed):"
echo "     wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz"
echo "     sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz"
echo "     export PATH=\$PATH:/usr/local/go/bin"
echo ""
echo "  2. Install Wails:"
echo "     go install github.com/wailsapp/wails/v2/cmd/wails@latest"
echo ""
echo "  3. Build Mockelot:"
echo "     make linux"
