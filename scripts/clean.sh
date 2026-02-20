#!/bin/bash
# Clean all Mockelot build artifacts
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/_common.sh"

log_info "Cleaning build artifacts..."
cd "$PROJECT_DIR"
rm -rf build/ dist/ releases/ AppDir
rm -f *.AppImage *.deb
log_success "Clean complete."
