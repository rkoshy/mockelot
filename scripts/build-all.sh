#!/bin/bash
# Build all Mockelot platform artifacts
# Usage: ./build-all.sh [--laminar]
#   --laminar: dispatch to Laminar CI jobs (used by mockelot-build-all.run)
#   (default): run build scripts directly in parallel subshells
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/_common.sh"

export VERSION=$(get_version)
log_info "=== Mockelot Build All (v${VERSION}) ==="

if [[ "${1:-}" == "--laminar" ]]; then
    log_info "Dispatching to Laminar CI jobs..."
    laminarc run mockelot-linux-build VERSION=${VERSION} &
    PID_LINUX=$!
    laminarc run mockelot-windows-build VERSION=${VERSION} &
    PID_WIN=$!
    laminarc run mockelot-macos-build VERSION=${VERSION} &
    PID_MAC=$!

    log_info "Waiting for builds (monitor: http://localhost:9000)..."
    FAILED=0
    wait $PID_LINUX || { log_error "Linux build failed!"; FAILED=1; }
    wait $PID_WIN   || { log_error "Windows build failed!"; FAILED=1; }
    wait $PID_MAC   || { log_error "macOS build failed!"; FAILED=1; }
else
    log_info "Running build scripts directly..."
    FAILED=0
    "${SCRIPT_DIR}/build-linux.sh" &
    PID_LINUX=$!
    "${SCRIPT_DIR}/build-windows.sh" &
    PID_WIN=$!

    # macOS runs sequentially (SSH to single host)
    "${SCRIPT_DIR}/build-macos.sh" || { log_error "macOS build failed!"; FAILED=1; }

    wait $PID_LINUX || { log_error "Linux build failed!"; FAILED=1; }
    wait $PID_WIN   || { log_error "Windows build failed!"; FAILED=1; }
fi

if [ $FAILED -eq 1 ]; then
    log_error "One or more builds failed."
    exit 1
fi

# Verify artifacts
EXPECTED=(
    "${DIST_DIR}/linux/mockelot-linux-amd64.tar.gz"
    "${DIST_DIR}/windows/mockelot-windows-amd64.zip"
    "${DIST_DIR}/macos/mockelot-darwin-universal.zip"
)

for artifact in "${EXPECTED[@]}"; do
    if [ ! -f "$artifact" ]; then
        log_error "Missing artifact: $artifact"
        exit 1
    fi
done

log_success "All 3 platform artifacts verified!"
