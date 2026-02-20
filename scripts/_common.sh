#!/bin/bash
# _common.sh — Shared library for all Mockelot build scripts
# Source this at the top of every build script:
#   SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
#   source "${SCRIPT_DIR}/_common.sh"

set -euo pipefail
trap 'log_error "Failed at line $LINENO"' ERR

# --- Paths ---
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
DIST_DIR="${PROJECT_DIR}/dist"

# --- Host identity (for Docker chown) ---
HOST_UID=$(id -u)
HOST_GID=$(id -g)

# --- Version ---
get_version() {
    if [ -n "${VERSION:-}" ]; then
        echo "$VERSION"
        return
    fi
    cd "$PROJECT_DIR"
    git describe --tags 2>/dev/null | sed 's/^v//' || echo "dev"
}

# --- Logging ---
_RED='\033[0;31m'
_GREEN='\033[0;32m'
_YELLOW='\033[1;33m'
_BLUE='\033[0;34m'
_NC='\033[0m'

log_info()    { echo -e "${_BLUE}[INFO]${_NC} $1"; }
log_success() { echo -e "${_GREEN}[OK]${_NC} $1"; }
log_warn()    { echo -e "${_YELLOW}[WARN]${_NC} $1"; }
log_error()   { echo -e "${_RED}[ERROR]${_NC} $1"; }

# --- Helpers ---
ensure_dist_dir() {
    local platform="$1"
    mkdir -p "${DIST_DIR}/${platform}"
}
