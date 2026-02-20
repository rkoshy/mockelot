#!/bin/bash
# Build Mockelot for macOS (universal binary via remote SSH to macOS host)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/_common.sh"

REMOTE_HOST="10.100.102.102"
REMOTE_USER="renny"
REMOTE_BUILD_DIR="~/repositories/tools/mockelot"
SIGNING_IDENTITY="Developer ID Application: Renny Koshy (YET2EWBPK9)"
CI_KEYCHAIN="~/Library/Keychains/ci-signing.keychain-db"
VERSION=$(get_version)

# CI_KEYCHAIN_PASSWORD must be set by caller (Laminar job injects it)
if [ -z "${CI_KEYCHAIN_PASSWORD:-}" ]; then
    log_error "CI_KEYCHAIN_PASSWORD not set. Export it before running this script."
    exit 1
fi

log_info "=== Mockelot macOS Build (v${VERSION}) ==="
log_info "Remote: ${REMOTE_USER}@${REMOTE_HOST}"
ensure_dist_dir macos

# Sync source to remote
log_info "Syncing source to macOS builder..."
rsync -avz --delete \
    --exclude '.git' \
    --exclude 'dist' \
    --exclude 'node_modules' \
    --exclude 'build' \
    --exclude '*.tar.gz' \
    --exclude '*.zip' \
    --exclude '*.AppImage' \
    "${PROJECT_DIR}/" \
    "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_BUILD_DIR}/"

# Build arm64 + amd64, combine with lipo, sign
log_info "Building universal binary (arm64 + amd64)..."
ssh "${REMOTE_USER}@${REMOTE_HOST}" "bash -lc 'cd ${REMOTE_BUILD_DIR} && \
    rm -rf build/bin/Mockelot*.app && \
    ~/go/bin/wails build -platform darwin/arm64 && \
    mv build/bin/Mockelot.app build/bin/Mockelot-arm64.app && \
    ~/go/bin/wails build -platform darwin/amd64 && \
    mv build/bin/Mockelot.app build/bin/Mockelot-amd64.app && \
    mkdir -p build/bin/Mockelot.app/Contents/MacOS && \
    lipo -create \
        build/bin/Mockelot-arm64.app/Contents/MacOS/mockelot \
        build/bin/Mockelot-amd64.app/Contents/MacOS/mockelot \
        -output build/bin/Mockelot.app/Contents/MacOS/mockelot && \
    cp -r build/bin/Mockelot-arm64.app/Contents/Resources build/bin/Mockelot.app/Contents/ && \
    cp build/bin/Mockelot-arm64.app/Contents/Info.plist build/bin/Mockelot.app/Contents/ && \
    echo \"=== Unlocking CI keychain ===\" && \
    security unlock-keychain -p \"${CI_KEYCHAIN_PASSWORD}\" ${CI_KEYCHAIN} && \
    echo \"=== Signing app bundle ===\" && \
    codesign --force --deep --sign \"${SIGNING_IDENTITY}\" --keychain ${CI_KEYCHAIN} --options runtime build/bin/Mockelot.app && \
    echo \"=== Verifying signature ===\" && \
    codesign -dvvv build/bin/Mockelot.app 2>&1 | grep -E \"Identifier|Authority|TeamIdentifier\" && \
    spctl -a -vv build/bin/Mockelot.app && \
    cd build/bin && \
    rm -f mockelot-darwin-universal.zip && \
    zip -r mockelot-darwin-universal.zip Mockelot.app'"

# Retrieve artifact
log_info "Retrieving artifact from macOS..."
rsync -avz \
    "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_BUILD_DIR}/build/bin/mockelot-darwin-universal.zip" \
    "${DIST_DIR}/macos/"

log_success "macOS build complete: ${DIST_DIR}/macos/mockelot-darwin-universal.zip"
ls -lh "${DIST_DIR}/macos/"
