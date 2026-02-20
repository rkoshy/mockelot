#!/bin/bash
# Build Mockelot in Docker containers for specific Debian versions
# This ensures binary compatibility with target distro's libwebkit version
# Automatically uses native build if current platform matches target

set -e

DISTRO=${1:-debian12}

case $DISTRO in
    debian12)
        DOCKER_IMAGE="scorussolutions/wails-appimage-builder-d12:latest"
        OUTPUT_NAME="mockelot-debian12"
        ;;
    debian13)
        DOCKER_IMAGE="scorussolutions/wails-appimage-builder-d13:latest"
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

    # Detect webkit version and set build tags
    WAILS_TAGS=""
    if [ -x ./detect-platform.sh ]; then
        WAILS_TAGS=$(./detect-platform.sh --wails-tags)
    elif command -v pkg-config &> /dev/null; then
        if pkg-config --exists webkit2gtk-4.1; then
            WAILS_TAGS="-tags webkit2_41"
        fi
    fi

    # Build natively
    echo "Building application natively..."
    if [ -n "$WAILS_TAGS" ]; then
        echo "Using build tags: $WAILS_TAGS"
        ~/go/bin/wails build -platform linux/amd64 $WAILS_TAGS
    else
        ~/go/bin/wails build -platform linux/amd64
    fi

    # Copy to target name
    cp build/bin/mockelot "build/bin/$OUTPUT_NAME"

    echo ""
    echo "✓ Native build complete: build/bin/$OUTPUT_NAME"
else
    echo "⚠ Current platform ($CURRENT_PLATFORM) differs from target ($DISTRO)"
    echo "  Using Docker build with ${DOCKER_IMAGE}"
    echo ""

    HOST_UID=$(id -u)
    HOST_GID=$(id -g)

    # Create output directory
    mkdir -p build/bin

    # Determine build tags for target distro
    BUILD_CMD="wails build -platform linux/amd64"
    if [ "$DISTRO" = "debian13" ]; then
        echo "Using webkit2_41 build tag for Debian 13"
        BUILD_CMD="wails build -platform linux/amd64 -tags webkit2_41"
    fi

    echo "Building application in container (temp-dir-copy)..."
    docker run --rm --privileged \
        -v "$(pwd):/workspace" \
        "${DOCKER_IMAGE}" \
        -c "
            set -e
            cp -r /workspace /tmp/build
            cd /tmp/build
            rm -rf frontend/node_modules frontend/dist build/bin
            ${BUILD_CMD}
            mkdir -p /workspace/build/bin
            cp /tmp/build/build/bin/mockelot /workspace/build/bin/${OUTPUT_NAME}
            chown -R ${HOST_UID}:${HOST_GID} /workspace/build
        "

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
