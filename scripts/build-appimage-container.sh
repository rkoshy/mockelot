#!/bin/bash
set -e

DEBIAN_VERSION=${1:-debian12}

echo "Mockelot AppImage Builder (Temp Build)"
echo "======================================="
echo "Target: ${DEBIAN_VERSION}"

# Get version from environment or git
VERSION=${VERSION:-$(cd /workspace && git describe --tags 2>/dev/null | sed 's/^v//' || echo "dev")}
echo "Version: $VERSION"

# Copy source to temp directory
echo "Step 1: Copying source to build directory..."
BUILD_DIR="/tmp/mockelot-build"
rm -rf "$BUILD_DIR"
cp -r /workspace "$BUILD_DIR"
cd "$BUILD_DIR"

# Build the binary
echo "Step 2: Building binary..."
echo "Installing frontend dependencies..."
cd frontend && npm ci && cd ..

# Determine webkit build tags based on Debian version
if [ "$DEBIAN_VERSION" = "debian13" ]; then
    echo "Building for Debian 13 with webkit2gtk-4.1..."
    wails build -platform linux/amd64 -tags webkit2_41 -o mockelot -ldflags "-X main.version=${VERSION}"
else
    echo "Building for Debian 12 with webkit2gtk-4.0..."
    wails build -platform linux/amd64 -o mockelot -ldflags "-X main.version=${VERSION}"
fi

# Create AppDir structure
echo "Step 3: Creating AppDir structure..."
APPDIR="AppDir"
rm -rf "$APPDIR"
mkdir -p "$APPDIR/usr/bin"
mkdir -p "$APPDIR/usr/lib"
mkdir -p "$APPDIR/usr/share/applications"
mkdir -p "$APPDIR/usr/share/icons/hicolor/256x256/apps"

# Copy binary
cp build/bin/mockelot "$APPDIR/usr/bin/"

# Bundle webkit helper processes
echo "Step 4: Bundling WebKit helper processes..."
mkdir -p "$APPDIR/usr/lib/webkit2gtk-4.0"

if [ "$DEBIAN_VERSION" = "debian13" ]; then
    WEBKIT_BASE="/usr/lib/x86_64-linux-gnu/webkit2gtk-4.1"
else
    WEBKIT_BASE="/usr/lib/x86_64-linux-gnu/webkit2gtk-4.0"
fi

if [ -d "$WEBKIT_BASE" ]; then
    cp "$WEBKIT_BASE/WebKitWebProcess" "$APPDIR/usr/lib/webkit2gtk-4.0/" || true
    cp "$WEBKIT_BASE/WebKitNetworkProcess" "$APPDIR/usr/lib/webkit2gtk-4.0/" || true
    cp "$WEBKIT_BASE/WebKitGPUProcess" "$APPDIR/usr/lib/webkit2gtk-4.0/" 2>/dev/null || true
    cp -r "$WEBKIT_BASE/injected-bundle" "$APPDIR/usr/lib/webkit2gtk-4.0/" 2>/dev/null || true
    echo "WebKit helper processes bundled from $WEBKIT_BASE"
fi

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

# Copy icon
if [ -f "build/appicon.png" ]; then
    cp build/appicon.png "$APPDIR/usr/share/icons/hicolor/256x256/apps/mockelot.png"
elif [ -f "frontend/src/assets/images/logo-universal.png" ]; then
    cp frontend/src/assets/images/logo-universal.png "$APPDIR/usr/share/icons/hicolor/256x256/apps/mockelot.png"
fi

# Create AppRun script
cat > "$APPDIR/AppRun" << 'EOF'
#!/bin/bash
SELF=$(readlink -f "$0")
HERE=${SELF%/*}
export WEBKIT_EXEC_PATH="${HERE}/usr/lib/webkit2gtk-4.0"
export LD_LIBRARY_PATH="${HERE}/usr/lib:${LD_LIBRARY_PATH}"
export PATH="${HERE}/usr/bin:${PATH}"
exec "${HERE}/usr/bin/mockelot" "$@"
EOF
chmod +x "$APPDIR/AppRun"

# Create symlinks
ln -sf usr/share/applications/mockelot.desktop "$APPDIR/" 2>/dev/null || true
ln -sf usr/share/icons/hicolor/256x256/apps/mockelot.png "$APPDIR/" 2>/dev/null || true

# Run linuxdeploy to bundle dependencies
echo "Step 5: Bundling dependencies with linuxdeploy..."
linuxdeploy --appdir "$APPDIR" --output appimage 2>&1 | grep -v "WARNING"

# Rename to include version
if [ -f "Mockelot-x86_64.AppImage" ]; then
    mv Mockelot-x86_64.AppImage "mockelot-${VERSION}-x86_64.AppImage"
elif [ -f "Mockelot-${VERSION}-x86_64.AppImage" ]; then
    # Already has version in name
    mv "Mockelot-${VERSION}-x86_64.AppImage" "mockelot-${VERSION}-x86_64.AppImage"
fi

# Copy ONLY the final artifact back to workspace (with Debian version suffix)
echo "Step 6: Copying artifact to workspace..."
mkdir -p /workspace/releases

# Name the file with the Debian version for clarity
if [ "$DEBIAN_VERSION" = "debian13" ]; then
    OUTPUT_FILE="mockelot-${VERSION}-debian13-x86_64.AppImage"
else
    OUTPUT_FILE="mockelot-${VERSION}-debian12-x86_64.AppImage"
fi

cp mockelot-${VERSION}-x86_64.AppImage "/workspace/releases/${OUTPUT_FILE}"
chmod 755 "/workspace/releases/${OUTPUT_FILE}"
# Fix ownership to match the host user
chown ${HOST_UID:-1000}:${HOST_GID:-1000} "/workspace/releases/${OUTPUT_FILE}" 2>/dev/null || true

# Build directory will be cleaned automatically when container exits
echo "✓ AppImage created: /workspace/releases/${OUTPUT_FILE}"
echo "Build complete for: ${DEBIAN_VERSION}"