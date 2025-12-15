#!/bin/bash
# Build Mockelot as AppImage for maximum Linux compatibility
# This bundles all dependencies including libwebkit2gtk

set -e

echo "Building Mockelot AppImage..."

# Build the binary first
echo "Step 1: Building binary..."
~/go/bin/wails build -platform linux/amd64

# Create AppDir structure
echo "Step 2: Creating AppDir structure..."
APPDIR="build/appimage/Mockelot.AppDir"
rm -rf build/appimage
mkdir -p "$APPDIR/usr/bin"
mkdir -p "$APPDIR/usr/share/applications"
mkdir -p "$APPDIR/usr/share/icons/hicolor/256x256/apps"

# Copy binary
cp build/bin/mockelot "$APPDIR/usr/bin/"

# Create desktop file
cat > "$APPDIR/usr/share/applications/mockelot.desktop" << 'EOF'
[Desktop Entry]
Name=Mockelot
Exec=mockelot
Icon=mockelot
Type=Application
Categories=Development;Network;
Comment=HTTP Mock Server for Testing
Terminal=false
EOF

# Create icon (placeholder - you can replace with actual icon)
cat > "$APPDIR/usr/share/icons/hicolor/256x256/apps/mockelot.png" << 'EOF'
# Placeholder - add actual PNG icon here
EOF

# Create AppRun script
cat > "$APPDIR/AppRun" << 'EOF'
#!/bin/bash
SELF=$(readlink -f "$0")
HERE=${SELF%/*}
export PATH="${HERE}/usr/bin/:${PATH}"
export LD_LIBRARY_PATH="${HERE}/usr/lib/:${LD_LIBRARY_PATH}"
exec "${HERE}/usr/bin/mockelot" "$@"
EOF
chmod +x "$APPDIR/AppRun"

# Symlink desktop file and icon to root
ln -sf usr/share/applications/mockelot.desktop "$APPDIR/"
ln -sf usr/share/icons/hicolor/256x256/apps/mockelot.png "$APPDIR/"

# Download appimagetool if not present
if [ ! -f "/tmp/appimagetool-x86_64.AppImage" ]; then
    echo "Step 3: Downloading appimagetool..."
    wget -O /tmp/appimagetool-x86_64.AppImage \
        "https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage"
    chmod +x /tmp/appimagetool-x86_64.AppImage
fi

# Build AppImage
echo "Step 4: Building AppImage..."
ARCH=x86_64 /tmp/appimagetool-x86_64.AppImage "$APPDIR" build/bin/mockelot-x86_64.AppImage

echo ""
echo "✓ AppImage created: build/bin/mockelot-x86_64.AppImage"
echo ""
echo "This AppImage will run on most Linux distributions including:"
echo "  - Debian 11, 12, 13"
echo "  - Ubuntu 20.04+"
echo "  - Fedora, RHEL, CentOS"
echo "  - Arch, Manjaro"
echo ""
echo "To run: chmod +x build/bin/mockelot-x86_64.AppImage && ./build/bin/mockelot-x86_64.AppImage"
