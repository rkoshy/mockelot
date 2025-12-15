#!/bin/bash
# Build Mockelot as AppImage for maximum Linux compatibility
# This bundles all dependencies including libwebkit2gtk
#
# NOTE: AppImage must be built on the OLDEST supported distro to ensure
# compatibility with newer distros. Build on Debian 12 for best compatibility.

set -e

echo "Mockelot AppImage Builder"
echo "========================"
echo ""

# Check current platform
CURRENT_PLATFORM=""
WEBKIT_VERSION=""
if [ -x ./detect-platform.sh ]; then
    CURRENT_PLATFORM=$(./detect-platform.sh --platform)
    WEBKIT_VERSION=$(./detect-platform.sh --webkit)
    echo "Current platform: $CURRENT_PLATFORM (WebKit $WEBKIT_VERSION)"
fi

# Warn if building on webkit 4.1 system (unless running in Docker)
if [ "$WEBKIT_VERSION" = "4.1" ] && [ -z "$SKIP_PLATFORM_CHECK" ]; then
    echo ""
    echo "⚠ WARNING: Building AppImage on webkit 4.1 system"
    echo "  AppImages should be built on the OLDEST supported distro"
    echo "  Recommendation: Build on Debian 12 for maximum compatibility"
    echo ""
    echo "Options:"
    echo "  1. Build on Debian 12 system (recommended)"
    echo "  2. Use Docker to build: make appimage-debian12 or make appimage-debian13"
    echo "  3. Continue anyway (creates Debian 13+ only AppImage)"
    echo ""
    read -p "Continue anyway? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 1
    fi
fi

echo ""
echo "Building Mockelot AppImage..."

# Detect webkit and get build tags
WAILS_TAGS=""
if [ -x ./detect-platform.sh ]; then
    WAILS_TAGS=$(./detect-platform.sh --wails-tags)
fi

# Build the binary first with correct webkit version
echo "Step 1: Building binary..."
if [ -n "$WAILS_TAGS" ]; then
    echo "Using build tags: $WAILS_TAGS"
    ~/go/bin/wails build -platform linux/amd64 $WAILS_TAGS
else
    ~/go/bin/wails build -platform linux/amd64
fi

# Create AppDir structure
echo ""
echo "Step 2: Creating AppDir structure..."
APPDIR="build/appimage/Mockelot.AppDir"
rm -rf build/appimage
mkdir -p "$APPDIR/usr/bin"
mkdir -p "$APPDIR/usr/lib"
mkdir -p "$APPDIR/usr/share/applications"
mkdir -p "$APPDIR/usr/share/icons/hicolor/256x256/apps"

# Copy binary
cp build/bin/mockelot "$APPDIR/usr/bin/"

# Bundle webkit and GTK libraries
echo ""
echo "Step 3: Bundling dependencies..."

# Function to copy library and its dependencies
copy_lib_deps() {
    local lib=$1
    local destdir=$2

    if [ ! -f "$lib" ]; then
        return
    fi

    # Copy the library
    cp -L "$lib" "$destdir/" 2>/dev/null || true

    # Get dependencies
    ldd "$lib" 2>/dev/null | grep "=> /" | awk '{print $3}' | while read dep; do
        if [ -f "$dep" ]; then
            local depname=$(basename "$dep")
            if [ ! -f "$destdir/$depname" ]; then
                cp -L "$dep" "$destdir/" 2>/dev/null || true
            fi
        fi
    done
}

# Determine webkit library to bundle
WEBKIT_LIB=""
if [ "$WEBKIT_VERSION" = "4.1" ]; then
    WEBKIT_LIB="/usr/lib/x86_64-linux-gnu/libwebkit2gtk-4.1.so.0"
else
    WEBKIT_LIB="/usr/lib/x86_64-linux-gnu/libwebkit2gtk-4.0.so.37"
fi

echo "Bundling webkit library: $WEBKIT_LIB"

# Copy webkit and its dependencies
if [ -f "$WEBKIT_LIB" ]; then
    copy_lib_deps "$WEBKIT_LIB" "$APPDIR/usr/lib"
else
    echo "⚠ Warning: WebKit library not found: $WEBKIT_LIB"
    echo "  AppImage may not work on systems without webkit installed"
fi

# Bundle GTK libraries
for lib in /usr/lib/x86_64-linux-gnu/libgtk-3.so.* \
           /usr/lib/x86_64-linux-gnu/libgdk-3.so.* \
           /usr/lib/x86_64-linux-gnu/libgio-2.0.so.*; do
    if [ -f "$lib" ]; then
        copy_lib_deps "$lib" "$APPDIR/usr/lib"
    fi
done

echo "Bundled $(ls "$APPDIR/usr/lib" | wc -l) libraries"

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

# Create a simple icon (placeholder)
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
export GDK_PIXBUF_MODULEDIR="${HERE}/usr/lib/gdk-pixbuf-2.0/2.10.0/loaders"
export GDK_PIXBUF_MODULE_FILE="${HERE}/usr/lib/gdk-pixbuf-2.0/2.10.0/loaders.cache"
exec "${HERE}/usr/bin/mockelot" "$@"
EOF
chmod +x "$APPDIR/AppRun"

# Symlink desktop file and icon to root
ln -sf usr/share/applications/mockelot.desktop "$APPDIR/"
ln -sf usr/share/icons/hicolor/256x256/apps/mockelot.png "$APPDIR/"

# Download appimagetool if not present
if [ ! -f "/tmp/appimagetool-x86_64.AppImage" ]; then
    echo ""
    echo "Step 4: Downloading appimagetool..."
    wget -q -O /tmp/appimagetool-x86_64.AppImage \
        "https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage"
    chmod +x /tmp/appimagetool-x86_64.AppImage
fi

# Build AppImage
echo ""
echo "Step 5: Building AppImage..."
ARCH=x86_64 /tmp/appimagetool-x86_64.AppImage "$APPDIR" build/bin/mockelot-x86_64.AppImage 2>&1 | grep -v "WARNING: desktop-file-validate"

echo ""
echo "✓ AppImage created: build/bin/mockelot-x86_64.AppImage"
echo ""

# Show what webkit version is bundled
if [ -f "$APPDIR/usr/lib/$(basename $WEBKIT_LIB)" ]; then
    echo "Bundled WebKit version: $(basename $WEBKIT_LIB)"
    echo ""
fi

echo "This AppImage will run on most Linux distributions including:"
if [ "$WEBKIT_VERSION" = "4.1" ]; then
    echo "  - Debian 13+"
    echo "  - Ubuntu 24.04+"
    echo "  ⚠ May not work on older distros (Debian 12, Ubuntu 22.04)"
else
    echo "  - Debian 11, 12, 13"
    echo "  - Ubuntu 20.04, 22.04, 24.04"
    echo "  - Fedora, RHEL, CentOS"
    echo "  - Arch, Manjaro"
fi
echo ""
echo "To run: chmod +x build/bin/mockelot-x86_64.AppImage && ./build/bin/mockelot-x86_64.AppImage"
