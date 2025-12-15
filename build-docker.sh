#!/bin/bash
# Build Mockelot in Docker containers for specific Debian versions
# This ensures binary compatibility with target distro's libwebkit version
# Automatically uses native build if current platform matches target

set -e

DISTRO=${1:-debian12}

case $DISTRO in
    debian12)
        DOCKERFILE="Dockerfile.debian12"
        TAG="mockelot-build:debian12"
        OUTPUT_NAME="mockelot-debian12"
        ;;
    debian13)
        DOCKERFILE="Dockerfile.debian13"
        TAG="mockelot-build:debian13"
        OUTPUT_NAME="mockelot-debian13"
        ;;
    *)
        echo "Usage: $0 [debian12|debian13]"
        echo ""
        echo "Examples:"
        echo "  $0 debian12    # Build for Debian 12 (Bookworm)"
        echo "  $0 debian13    # Build for Debian 13 (Trixie)"
        exit 1
        ;;
esac

# Detect current platform
CURRENT_PLATFORM=""
if [ -x ./detect-platform.sh ]; then
    CURRENT_PLATFORM=$(./detect-platform.sh --platform)
elif [ -f /etc/os-release ]; then
    source /etc/os-release
    if [ "$ID" = "debian" ]; then
        if [ "$VERSION_ID" = "12" ]; then
            CURRENT_PLATFORM="debian12"
        elif [ "$VERSION_ID" = "13" ]; then
            CURRENT_PLATFORM="debian13"
        fi
    elif [ "$ID" = "ubuntu" ]; then
        # Check webkit version to determine compatibility
        if command -v pkg-config &> /dev/null; then
            if pkg-config --exists webkit2gtk-4.1; then
                CURRENT_PLATFORM="debian13"
            elif pkg-config --exists webkit2gtk-4.0; then
                CURRENT_PLATFORM="debian12"
            fi
        fi
    fi
fi

echo "Building Mockelot for $DISTRO..."
echo "Current platform detected: ${CURRENT_PLATFORM:-unknown}"
echo ""

# Check if we can build natively
if [ "$CURRENT_PLATFORM" = "$DISTRO" ]; then
    echo "✓ Current platform matches target platform!"
    echo "  Using native build (faster, no Docker needed)"
    echo ""

    # Create output directory
    mkdir -p build/bin

    # Build natively
    echo "Building application natively..."
    ~/go/bin/wails build -platform linux/amd64

    # Copy to target name
    cp build/bin/mockelot "build/bin/$OUTPUT_NAME"

    echo ""
    echo "✓ Native build complete: build/bin/$OUTPUT_NAME"
else
    echo "⚠ Current platform ($CURRENT_PLATFORM) differs from target ($DISTRO)"
    echo "  Using Docker build for compatibility"
    echo ""

    # Build the Docker image
    echo "Step 1: Building Docker image..."
    docker build -f "$DOCKERFILE" -t "$TAG" .

    # Create output directory
    mkdir -p build/bin

    # Run build in container
    echo ""
    echo "Step 2: Building application in container..."
    docker run --rm \
        -v "$(pwd):/build" \
        -w /build \
        "$TAG" \
        bash -c "wails build -platform linux/amd64 && cp build/bin/mockelot build/bin/$OUTPUT_NAME"

    echo ""
    echo "✓ Docker build complete: build/bin/$OUTPUT_NAME"
fi

echo ""
echo "This binary is built against $DISTRO libraries and will run on:"
if [ "$DISTRO" = "debian12" ]; then
    echo "  - Debian 12 (Bookworm)"
    echo "  - Ubuntu 22.04, 23.04, 23.10"
    echo "  - Linux Mint 21.x"
else
    echo "  - Debian 13 (Trixie)"
    echo "  - Ubuntu 24.04+"
    echo "  - Linux Mint 22.x+"
fi
echo ""
echo "To test: ./build/bin/$OUTPUT_NAME"
