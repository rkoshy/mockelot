#!/bin/bash
# Build Mockelot AppImage using Docker for specific Debian versions
# This ensures the AppImage is built with the correct webkit version

set -e

DISTRO=${1:-debian12}

case $DISTRO in
    debian12)
        DOCKERFILE="Dockerfile.appimage-debian12"
        TAG="mockelot-appimage-build:debian12"
        OUTPUT_NAME="mockelot-debian12-x86_64.AppImage"
        WEBKIT_VERSION="4.0"
        ;;
    debian13)
        DOCKERFILE="Dockerfile.appimage-debian13"
        TAG="mockelot-appimage-build:debian13"
        OUTPUT_NAME="mockelot-debian13-x86_64.AppImage"
        WEBKIT_VERSION="4.1"
        ;;
    *)
        echo "Usage: $0 [debian12|debian13]"
        echo ""
        echo "Examples:"
        echo "  $0 debian12    # Build AppImage for Debian 12 / Ubuntu 22.04 (webkit 4.0)"
        echo "  $0 debian13    # Build AppImage for Debian 13 / Ubuntu 24.04 (webkit 4.1)"
        exit 1
        ;;
esac

echo "Building Mockelot AppImage for $DISTRO (webkit $WEBKIT_VERSION)..."
echo ""

# Build the Docker image
echo "Step 1: Building Docker image..."
docker build -f "$DOCKERFILE" -t "$TAG" .

# Create output directory
mkdir -p build/bin

# Run build in container
echo ""
echo "Step 2: Building AppImage in container..."
docker run --rm \
    -v "$(pwd):/build" \
    -w /build \
    --privileged \
    "$TAG" \
    -c "chmod +x build-appimage.sh && SKIP_PLATFORM_CHECK=1 ./build-appimage.sh"

# Rename output to include distro version
if [ -f "build/bin/mockelot-x86_64.AppImage" ]; then
    mv build/bin/mockelot-x86_64.AppImage "build/bin/$OUTPUT_NAME"
fi

echo ""
echo "✓ AppImage build complete: build/bin/$OUTPUT_NAME"
echo ""
echo "This AppImage will run on:"
if [ "$DISTRO" = "debian12" ]; then
    echo "  - Debian 12 (Bookworm)"
    echo "  - Ubuntu 22.04, 23.04, 23.10"
    echo "  - Linux Mint 21.x"
    echo "  - Other distros with webkit2gtk-4.0"
else
    echo "  - Debian 13 (Trixie)"
    echo "  - Ubuntu 24.04+"
    echo "  - Linux Mint 22.x+"
    echo "  - Other distros with webkit2gtk-4.1"
fi
echo ""
echo "To test: chmod +x build/bin/$OUTPUT_NAME && ./build/bin/$OUTPUT_NAME"
