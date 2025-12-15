#!/bin/bash
# Build Mockelot in Docker containers for specific Debian versions
# This ensures binary compatibility with target distro's libwebkit version

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

echo "Building Mockelot for $DISTRO..."
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
echo "✓ Build complete: build/bin/$OUTPUT_NAME"
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
