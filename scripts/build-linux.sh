#!/bin/bash
# Build Mockelot for Linux amd64 using Docker
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/_common.sh"

BUILDER_IMAGE="scorussolutions/wails-appimage-builder-d12"
VERSION=$(get_version)

log_info "=== Mockelot Linux Build (v${VERSION}) ==="
ensure_dist_dir linux

cd "$PROJECT_DIR"
rm -rf build/bin/mockelot-linux-amd64* "${DIST_DIR}/linux/"*.tar.gz || true

log_info "Building in Docker (${BUILDER_IMAGE})..."
docker run --rm --privileged \
    -v "${PROJECT_DIR}:/workspace" \
    -e VERSION="${VERSION}" \
    "${BUILDER_IMAGE}" \
    -c "
        set -e
        cp -r /workspace /tmp/build
        cd /tmp/build
        rm -rf frontend/node_modules frontend/dist build/bin
        wails build -platform linux/amd64 -o mockelot-linux-amd64 -ldflags \"-X main.version=${VERSION}\"
        mkdir -p /workspace/build/bin
        cp /tmp/build/build/bin/mockelot-linux-amd64 /workspace/build/bin/
        chown -R ${HOST_UID}:${HOST_GID} /workspace/build
    "

log_info "Packaging artifact..."
cd "${PROJECT_DIR}/build/bin"
tar -czf mockelot-linux-amd64.tar.gz mockelot-linux-amd64
mv mockelot-linux-amd64.tar.gz "${DIST_DIR}/linux/"

log_success "Linux build complete: ${DIST_DIR}/linux/mockelot-linux-amd64.tar.gz"
ls -lh "${DIST_DIR}/linux/"
