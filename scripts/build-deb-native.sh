#!/bin/bash
# Build a Debian package from the native Wails binary (no AppImage).
# Requires webkit2gtk-4.1 on the target system (Debian 13+).
# Usage: ./build-deb-native.sh [version]
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/_common.sh"

VERSION=$(get_version)
PKGNAME="mockelot_${VERSION}-native-debian13_amd64"
BINARY="${PROJECT_DIR}/build/bin/mockelot"

log_info "=== Mockelot native .deb build (v${VERSION}) ==="

# Build the native binary for Debian 13 (webkit2gtk-4.1)
log_info "Building native binary..."
cd "$PROJECT_DIR"
~/go/bin/wails build -tags webkit2_41 -o mockelot

if [ ! -f "$BINARY" ]; then
    log_error "Binary not found at $BINARY after build"
    exit 1
fi
log_success "Binary built: $(du -sh "$BINARY" | cut -f1)"

# Package structure
PKGDIR="${PROJECT_DIR}/${PKGNAME}"
rm -rf "$PKGDIR"
mkdir -p "$PKGDIR/DEBIAN"
mkdir -p "$PKGDIR/usr/bin"
mkdir -p "$PKGDIR/usr/share/applications"
mkdir -p "$PKGDIR/usr/share/icons/hicolor/256x256/apps"

# Binary
cp "$BINARY" "$PKGDIR/usr/bin/mockelot"
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
