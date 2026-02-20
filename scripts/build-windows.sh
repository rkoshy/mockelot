#!/bin/bash
# Build Mockelot for Windows amd64 (cross-compile from Linux via Docker)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/_common.sh"

BUILDER_IMAGE="scorussolutions/wails-appimage-builder-d12"
VERSION=$(get_version)

log_info "=== Mockelot Windows Build (v${VERSION}) ==="
ensure_dist_dir windows

cd "$PROJECT_DIR"
rm -rf build/bin/mockelot*.exe "${DIST_DIR}/windows/"*.zip || true

log_info "Cross-compiling in Docker (${BUILDER_IMAGE})..."
docker run --rm --privileged \
    -v "${PROJECT_DIR}:/workspace" \
    -e VERSION="${VERSION}" \
    "${BUILDER_IMAGE}" \
    -c "
        set -e
        cp -r /workspace /tmp/build
        cd /tmp/build
        rm -rf frontend/node_modules frontend/dist build/bin
        wails build -platform windows/amd64 -o mockelot-windows-amd64.exe -ldflags \"-X main.version=${VERSION}\"
        mkdir -p /workspace/build/bin
        cp /tmp/build/build/bin/mockelot-windows-amd64.exe /workspace/build/bin/
        chown -R ${HOST_UID}:${HOST_GID} /workspace/build
    "

log_info "Packaging artifact..."
cd "${PROJECT_DIR}/build/bin"
zip -q mockelot-windows-amd64.zip mockelot-windows-amd64.exe
mv mockelot-windows-amd64.zip "${DIST_DIR}/windows/"

log_success "Windows build complete: ${DIST_DIR}/windows/mockelot-windows-amd64.zip"
ls -lh "${DIST_DIR}/windows/"
