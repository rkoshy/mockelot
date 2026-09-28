#!/bin/bash
# Build a Debian package from the native Wails binary (no AppImage).
# Requires webkit2gtk-4.1 on the target system (Debian 13+).
# Usage: ./build-deb-native.sh [version]
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/_common.sh"

VERSION=$(get_version)
PKGNAME="mockelot_${VERSION}-debian13_amd64"
# Use the already-built Linux binary from dist/ (produced by build-linux.sh via Laminar).
# Do NOT rebuild — the Laminar build already tagged the binary with the correct version.
BINARY="${DIST_DIR}/linux/mockelot-linux-amd64.tar.gz"
BINARY_EXTRACTED="/tmp/mockelot-deb-binary-$$"

log_info "=== Mockelot native .deb build (v${VERSION}) ==="

# Extract the binary from the Linux tarball built by Laminar
if [ ! -f "$BINARY" ]; then
    log_error "Linux tarball not found at $BINARY — run build-linux.sh first"
    exit 1
fi
log_info "Extracting binary from Linux tarball..."
mkdir -p "$BINARY_EXTRACTED"
tar -xzf "$BINARY" -C "$BINARY_EXTRACTED"
# The tarball contains mockelot-linux-amd64 (the platform-named binary)
EXTRACTED_BIN=$(find "$BINARY_EXTRACTED" -name "mockelot*" -type f | head -1)
if [ -z "$EXTRACTED_BIN" ]; then
    log_error "Could not find mockelot binary inside tarball"
    rm -rf "$BINARY_EXTRACTED"
    exit 1
fi
log_success "Binary extracted: $(du -sh "$EXTRACTED_BIN" | cut -f1)"

# Package structure
PKGDIR="${PROJECT_DIR}/${PKGNAME}"
rm -rf "$PKGDIR"
mkdir -p "$PKGDIR/DEBIAN"
mkdir -p "$PKGDIR/usr/bin"
mkdir -p "$PKGDIR/usr/share/applications"
mkdir -p "$PKGDIR/usr/share/icons/hicolor/256x256/apps"

# Binary
cp "$EXTRACTED_BIN" "$PKGDIR/usr/bin/mockelot"
chmod 755 "$PKGDIR/usr/bin/mockelot"

# Control file (depends on system webkit, not libfuse)
cat > "$PKGDIR/DEBIAN/control" << EOF
Package: mockelot
Version: ${VERSION}
Architecture: amd64
Maintainer: Renny Koshy <renny@scorussolutions.com>
Depends: libwebkit2gtk-4.1-0
Description: HTTP Mock Server for Testing
 Mockelot is a powerful HTTP mock server with support for:
  - Mock endpoints with static, template, and script responses
  - Proxy endpoints with transformation capabilities
  - Container endpoints for running Docker/Podman containers
  - SOCKS5 proxy for browser-based testing
  - OpenAPI/Swagger import
  - HTTPS with automatic certificate generation
Homepage: https://github.com/rkoshy/mockelot
Section: devel
Priority: optional
EOF

# postinst
cat > "$PKGDIR/DEBIAN/postinst" << 'EOF'
#!/bin/bash
set -e
case "$1" in
    configure)
        if command -v update-desktop-database > /dev/null 2>&1; then
            update-desktop-database /usr/share/applications 2>/dev/null || true
        fi
        if command -v gtk-update-icon-cache > /dev/null 2>&1; then
            gtk-update-icon-cache -f /usr/share/icons/hicolor 2>/dev/null || true
        fi
        echo "✓ Mockelot installed. Run: mockelot"
        ;;
esac
exit 0
EOF
chmod 755 "$PKGDIR/DEBIAN/postinst"

# prerm
cat > "$PKGDIR/DEBIAN/prerm" << 'EOF'
#!/bin/bash
set -e
exit 0
EOF
chmod 755 "$PKGDIR/DEBIAN/prerm"

# Desktop file
cat > "$PKGDIR/usr/share/applications/mockelot.desktop" << 'EOF'
[Desktop Entry]
Name=Mockelot
Exec=/usr/bin/mockelot
Icon=mockelot
Type=Application
Categories=Development;Network;
Terminal=false
Comment=HTTP Mock Server for Testing
EOF

# Icon
if [ -f "${PROJECT_DIR}/build/appicon.png" ]; then
    cp "${PROJECT_DIR}/build/appicon.png" "$PKGDIR/usr/share/icons/hicolor/256x256/apps/mockelot.png"
fi

# Build .deb
log_info "Building .deb..."
cd "$PROJECT_DIR"
dpkg-deb --build "$PKGDIR"
DEB="${PKGNAME}.deb"
log_success "Package ready: $(du -sh "$DEB" | cut -f1)  →  ${PROJECT_DIR}/${DEB}"

rm -rf "$PKGDIR"
rm -rf "$BINARY_EXTRACTED"
