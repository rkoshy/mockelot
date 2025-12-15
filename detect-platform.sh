#!/bin/bash
# Detect current platform for smart build selection

set -e

# Detect Debian/Ubuntu version
if [ -f /etc/os-release ]; then
    source /etc/os-release
    OS_ID="$ID"
    OS_VERSION_ID="$VERSION_ID"
    OS_VERSION_CODENAME="${VERSION_CODENAME:-unknown}"
else
    OS_ID="unknown"
    OS_VERSION_ID="unknown"
    OS_VERSION_CODENAME="unknown"
fi

# Detect libwebkit version
WEBKIT_VERSION=""
if command -v pkg-config &> /dev/null; then
    if pkg-config --exists webkit2gtk-4.1; then
        WEBKIT_VERSION="4.1"
    elif pkg-config --exists webkit2gtk-4.0; then
        WEBKIT_VERSION="4.0"
    fi
fi

# Determine platform designation
PLATFORM="unknown"

if [ "$OS_ID" = "debian" ]; then
    case "$OS_VERSION_ID" in
        12)
            PLATFORM="debian12"
            ;;
        13)
            PLATFORM="debian13"
            ;;
    esac
elif [ "$OS_ID" = "ubuntu" ]; then
    # Map Ubuntu versions to Debian equivalents based on webkit version
    if [ "$WEBKIT_VERSION" = "4.0" ]; then
        PLATFORM="debian12"
    elif [ "$WEBKIT_VERSION" = "4.1" ]; then
        PLATFORM="debian13"
    fi
fi

# Output format based on argument
case "${1:-}" in
    --platform)
        echo "$PLATFORM"
        ;;
    --webkit)
        echo "$WEBKIT_VERSION"
        ;;
    --verbose)
        echo "OS: $OS_ID $OS_VERSION_ID ($OS_VERSION_CODENAME)"
        echo "WebKit: ${WEBKIT_VERSION:-not found}"
        echo "Platform: $PLATFORM"
        ;;
    *)
        echo "$PLATFORM"
        ;;
esac
