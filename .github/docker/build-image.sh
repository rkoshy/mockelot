#!/bin/bash
# Build the custom Wails build Docker image for local testing with act

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGE_NAME="mockelot-wails-build:latest"

echo "Building Wails build environment image..."
docker build -t "$IMAGE_NAME" -f "$SCRIPT_DIR/wails-build.Dockerfile" "$SCRIPT_DIR"

echo ""
echo "✅ Build complete!"
echo ""
echo "Image: $IMAGE_NAME"
echo ""
echo "To use with act, run:"
echo "  act -P ubuntu-22.04=$IMAGE_NAME"
echo ""
echo "Or create .actrc in project root with:"
echo "  -P ubuntu-22.04=$IMAGE_NAME"
