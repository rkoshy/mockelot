#!/bin/bash
# Build Debian package from AppImage
set -e

VERSION=${1:-}
APPIMAGE_PATH=${2:-}

# Validation
if [ -z "$VERSION" ]; then
    echo "Error: VERSION not provided"
    echo "Usage: $0 VERSION APPIMAGE_PATH"
    exit 1
fi

if [ -z "$APPIMAGE_PATH" ] || [ ! -f "$APPIMAGE_PATH" ]; then
    echo "Error: AppImage not found at '$APPIMAGE_PATH'"
    echo "Usage: $0 VERSION APPIMAGE_PATH"
    exit 1
fi

echo "Building Debian package for Mockelot ${VERSION}..."

# Create package structure
PKGDIR="mockelot_${VERSION}_amd64"
rm -rf "$PKGDIR"
mkdir -p "$PKGDIR/DEBIAN"
mkdir -p "$PKGDIR/opt/mockelot/bin"
mkdir -p "$PKGDIR/usr/share/applications"
mkdir -p "$PKGDIR/usr/share/icons/hicolor/256x256/apps"

# Copy AppImage with versioned name
echo "  Copying AppImage..."
cp "$APPIMAGE_PATH" "$PKGDIR/opt/mockelot/bin/mockelot-${VERSION}.AppImage"
chmod 755 "$PKGDIR/opt/mockelot/bin/mockelot-${VERSION}.AppImage"

# Generate control file with version substitution
echo "  Generating control file..."
sed "s/\${VERSION}/$VERSION/g" debian/control > "$PKGDIR/DEBIAN/control"

# Copy scripts (substitute version placeholder in postinst)
echo "  Copying postinst and prerm scripts..."
sed "s/@VERSION@/${VERSION}/g" debian/postinst > "$PKGDIR/DEBIAN/postinst"
cp debian/prerm "$PKGDIR/DEBIAN/"
chmod 755 "$PKGDIR/DEBIAN/postinst"
chmod 755 "$PKGDIR/DEBIAN/prerm"

# Create desktop file
echo "  Creating desktop file..."
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

# Copy icon - try multiple sources
echo "  Copying icon..."
ICON_COPIED=false

# Try build/appicon.png
if [ -f "build/appicon.png" ]; then
    cp build/appicon.png "$PKGDIR/usr/share/icons/hicolor/256x256/apps/mockelot.png"
    ICON_COPIED=true
    echo "    ✓ Copied icon from build/appicon.png"
# Try frontend logo
elif [ -f "frontend/src/assets/images/logo-universal.png" ]; then
    cp frontend/src/assets/images/logo-universal.png "$PKGDIR/usr/share/icons/hicolor/256x256/apps/mockelot.png"
    ICON_COPIED=true
    echo "    ✓ Copied icon from frontend/src/assets/images/logo-universal.png"
fi

if [ "$ICON_COPIED" = false ]; then
    echo "    ⚠ Warning: No icon found, package will be built without icon"
fi

# Build the package
echo "  Building .deb package..."
dpkg-deb --build "$PKGDIR"

# Clean up
echo "  Cleaning up temporary files..."
rm -rf "$PKGDIR"

echo ""
echo "✓ Debian package created successfully:"
echo "  mockelot_${VERSION}_amd64.deb"
ls -lh "mockelot_${VERSION}_amd64.deb"
