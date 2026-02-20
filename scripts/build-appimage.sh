#!/bin/bash
# Build Mockelot AppImage for a specific Debian variant
# Usage: ./build-appimage.sh debian12|debian13
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/_common.sh"

VARIANT="${1:-}"
if [[ "$VARIANT" != "debian12" && "$VARIANT" != "debian13" ]]; then
    log_error "Usage: $0 debian12|debian13"
    exit 1
fi

IMAGE="scorussolutions/wails-appimage-builder-${VARIANT/debian/d}:latest"
VERSION=$(get_version)

log_info "=== Mockelot AppImage Build: ${VARIANT} (v${VERSION}) ==="
ensure_dist_dir linux

cd "$PROJECT_DIR"
mkdir -p releases

docker run --rm --privileged \
    -e VERSION="${VERSION}" \
    -v "${PROJECT_DIR}:/workspace" \
    -w /workspace \
    "${IMAGE}" \
    -c "chmod +x scripts/build-appimage-container.sh && ./scripts/build-appimage-container.sh ${VARIANT}"

cp "releases/mockelot-${VERSION}-${VARIANT}-x86_64.AppImage" "${DIST_DIR}/linux/"

log_success "AppImage (${VARIANT}): ${DIST_DIR}/linux/mockelot-${VERSION}-${VARIANT}-x86_64.AppImage"
