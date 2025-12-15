# Mockelot Build Guide

This guide explains how to build Mockelot for different Linux distributions, particularly addressing the libwebkit compatibility issues across Debian versions.

## The Problem: WebKit Across Distributions

Wails uses WebKit2GTK for rendering. Different distributions ship different versions:
- **Debian 12 (Bookworm)**: libwebkit2gtk-4.0
- **Debian 13 (Trixie)**: libwebkit2gtk-4.1
- **Ubuntu 22.04**: libwebkit2gtk-4.0
- **Ubuntu 24.04**: libwebkit2gtk-4.1

Binaries built on one version won't run on systems with a different version without the matching libraries installed.

## Solution Options

### Option 1: AppImage (Recommended) ⭐

**Best for**: Maximum compatibility across all Linux distributions

AppImage bundles all dependencies into a single executable file. Due to webkit ABI incompatibilities, **separate AppImages are required for different webkit versions**.

```bash
# Build AppImage for Debian 12 / Ubuntu 22.04 (webkit 4.0) - RECOMMENDED
make appimage-debian12
# Result: build/bin/mockelot-debian12-x86_64.AppImage
# Runs on: Debian 12, Ubuntu 22.04, Linux Mint 21.x, and other webkit 4.0 systems

# Build AppImage for Debian 13 / Ubuntu 24.04+ (webkit 4.1)
make appimage-debian13
# Result: build/bin/mockelot-debian13-x86_64.AppImage
# Runs on: Debian 13, Ubuntu 24.04+, Linux Mint 22.x+, and other webkit 4.1 systems

# Build both AppImage variants
make all-appimages
```

**CRITICAL - WebKit Version Compatibility:**
Wails applications depend on system webkit, which has **incompatible ABIs** between versions:
- **webkit 4.0** (Debian 12, Ubuntu 22.04, Mint 21.x) ↔️ **NOT compatible with** ↔️ **webkit 4.1** (Debian 13, Ubuntu 24.04+, Mint 22.x+)
- AppImages built on one webkit version will **not run** on systems with a different webkit version
- Users must download the AppImage matching their system's webkit version

**Docker-based Build:**
Both AppImage builds use Docker to ensure correct webkit version:
- `appimage-debian12`: Builds in Debian 12 container with webkit 4.0
- `appimage-debian13`: Builds in Debian 13 container with webkit 4.1
- Requires Docker installed on build system
- Ensures reproducible builds with correct dependencies

**Advantages:**
- ✅ Single file, no installation needed for end users
- ✅ Bundles all dependencies including libwebkit
- ✅ Users just need to `chmod +x` and run
- ✅ Reproducible builds via Docker

**Disadvantages:**
- ⚠️ Larger file size (~110-130MB with bundled webkit)
- ⚠️ Requires Docker for building
- ⚠️ Two separate files needed to cover all distros
- ⚠️ Users must choose correct variant for their system

**When to use**: Distributing to end users when you want easy installation without package management.

---

### Option 2: Docker Builds (Distro-Specific) - Smart Detection

**Best for**: Building binaries optimized for specific distributions

Build in Docker containers with exact target distro libraries. The build system automatically detects if your current platform matches the target and uses a native build if possible.

```bash
# Build for Debian 12 (Bookworm)
make debian12
# Result: build/bin/mockelot-debian12
# If you're on Debian 12: uses native build (fast!)
# If you're on Debian 13: uses Docker build

# Build for Debian 13 (Trixie)
make debian13
# Result: build/bin/mockelot-debian13
# If you're on Debian 13: uses native build (fast!)
# If you're on Debian 12: uses Docker build

# Build for all Debian versions
make all-debian
```

**Smart Platform Detection:**
The build system checks:
1. Current OS (Debian/Ubuntu version)
2. Installed libwebkit version (4.0 vs 4.1)
3. Automatically chooses native or Docker build

**Advantages:**
- ✅ Smaller binary size (~50MB)
- ✅ Native performance
- ✅ Matches target distro exactly
- ✅ Reproducible builds
- ✅ Auto-detects when Docker isn't needed (faster!)

**Disadvantages:**
- ⚠️ Requires Docker installed (only when cross-building)
- ⚠️ Must distribute correct binary for each distro
- ⚠️ Users need matching libwebkit version

**When to use**: Building for internal deployment where you control the distro versions.

---

### Option 3: Native Build

**Best for**: Development on your current system

```bash
# Build for your current system
make linux

# Result: build/bin/mockelot
```

**Advantages:**
- ✅ Fastest build
- ✅ No additional tools needed
- ✅ Good for development

**Disadvantages:**
- ⚠️ Only works on systems with same libwebkit version
- ⚠️ May not work on other distros

**When to use**: Local development and testing.

---

## Platform Detection

The build system includes smart platform detection that automatically chooses between native and Docker builds.

### Check Your Current Platform
```bash
# Make script executable
chmod +x detect-platform.sh

# Detect platform
./detect-platform.sh --verbose

# Example output:
# OS: debian 12 (bookworm)
# WebKit: 4.0
# Platform: debian12
```

This helps you understand:
- What builds will be native (fast)
- What builds will need Docker
- If your system is compatible with target distro

---

## Quick Start - Install Dependencies

The easiest way to install all required build dependencies:

```bash
# Automatically detects your platform and installs correct packages
./install-deps.sh
```

This script automatically installs the correct webkit version for your distro.

---

## Installation Requirements

### Automated Installation (Recommended)
```bash
chmod +x install-deps.sh
./install-deps.sh
```

The script automatically detects your platform and installs:
- Debian 12: `libwebkit2gtk-4.0-dev`
- Debian 13: `libwebkit2gtk-4.1-dev`
- Ubuntu 22.04: `libwebkit2gtk-4.0-dev`
- Ubuntu 24.04: `libwebkit2gtk-4.1-dev`

### Manual Installation

#### For AppImage Builds
```bash
# Install appimagetool dependencies
sudo apt-get install wget fuse
```

#### For Docker Builds
```bash
# Install Docker
sudo apt-get install docker.io
sudo usermod -aG docker $USER
# Log out and back in for group changes
```

#### For Native Builds
```bash
# Debian 12 / Ubuntu 22.04
sudo apt-get install build-essential libgtk-3-dev libwebkit2gtk-4.0-dev pkg-config

# Debian 13 / Ubuntu 24.04
sudo apt-get install build-essential libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config
```

---

## Build Commands Quick Reference

```bash
# See all options
make help

# Development
make dev              # Run in development mode
make clean            # Clean build artifacts

# Production builds
make appimage         # Universal Linux binary (RECOMMENDED)
make debian12         # Debian 12 specific
make debian13         # Debian 13 specific
make all-debian       # Build for all Debian versions
make windows          # Windows executable
```

---

## Distribution Recommendations

### For Public Release
```bash
# Build BOTH AppImage variants using Docker
make all-appimages

# This creates:
# - build/bin/mockelot-debian12-x86_64.AppImage (for webkit 4.0 systems)
# - build/bin/mockelot-debian13-x86_64.AppImage (for webkit 4.1 systems)

# Distribute both files with clear instructions:
# - Debian 12 / Ubuntu 22.04 users: Download mockelot-debian12-x86_64.AppImage
# - Debian 13 / Ubuntu 24.04+ users: Download mockelot-debian13-x86_64.AppImage
```

**Distribution Instructions for Users:**
Provide this guidance with your releases:
```
How to choose the correct AppImage:

Debian/Ubuntu users:
  - Debian 12 or Ubuntu 22.04, 23.04, 23.10: Use mockelot-debian12-x86_64.AppImage
  - Debian 13 or Ubuntu 24.04+: Use mockelot-debian13-x86_64.AppImage

If unsure, check your webkit version:
  $ pkg-config --modversion webkit2gtk-4.0 2>/dev/null && echo "Use debian12 AppImage"
  $ pkg-config --modversion webkit2gtk-4.1 2>/dev/null && echo "Use debian13 AppImage"

To run:
  $ chmod +x mockelot-*-x86_64.AppImage
  $ ./mockelot-*-x86_64.AppImage
```

### For Internal IT Department
```bash
# Build specific versions
make debian12
make debian13

# Distribute matching binary:
# - Debian 12 systems: mockelot-debian12
# - Debian 13 systems: mockelot-debian13
```

### For Developers
```bash
# Use native build for speed
make linux

# Or run directly
make dev
```

---

## Testing Builds

### Test on Target System
```bash
# Copy binary to target system
scp build/bin/mockelot-debian12 user@target:/tmp/

# On target system
ssh user@target
chmod +x /tmp/mockelot-debian12
/tmp/mockelot-debian12

# Check for library errors
ldd /tmp/mockelot-debian12 | grep "not found"
```

### Test AppImage
```bash
# AppImage should just work
chmod +x build/bin/mockelot-x86_64.AppImage
./build/bin/mockelot-x86_64.AppImage
```

---

## Troubleshooting

### "Package 'gtk+-3.0' was not found" or "Package 'webkit2gtk-4.0' was not found"
**Problem**: Build dependencies not installed

**Solution**:
```bash
# Run the automated installer
./install-deps.sh

# Or install manually for your distro:
# Debian 12 / Ubuntu 22.04:
sudo apt-get install build-essential libgtk-3-dev libwebkit2gtk-4.0-dev pkg-config

# Debian 13 / Ubuntu 24.04:
sudo apt-get install build-essential libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config
```

### "Package 'webkit2gtk-4.0' was not found" (but you have webkit2gtk-4.1 installed)
**Problem**: Wails is looking for webkit2gtk-4.0 but your system has webkit2gtk-4.1 (Debian 13/Ubuntu 24.04)

**Solution**: The build system automatically detects this and uses the correct build tags. If you're building manually:
```bash
# Build with webkit2_41 tag (for Debian 13 / Ubuntu 24.04)
~/go/bin/wails build -platform linux/amd64 -tags webkit2_41

# Or just use make (automatically detects)
make linux
```

**Automatic Detection**: When you use `make linux` or `make debian13`, the build system:
1. Detects your installed webkit version
2. Automatically adds `-tags webkit2_41` if needed
3. No manual configuration required

### "libwebkit2gtk-4.0.so.37: cannot open shared object file"
**Problem**: Binary built for Debian 12 running on Debian 13 (or vice versa)

**Solutions**:
1. Use AppImage instead: `make appimage`
2. Build for correct distro: `make debian13`
3. Install compat library (not recommended)

### "Error: cannot build AppImage"
**Problem**: appimagetool not available

**Solution**:
```bash
# The script auto-downloads it, but you can manually install:
wget https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage
chmod +x appimagetool-x86_64.AppImage
sudo mv appimagetool-x86_64.AppImage /usr/local/bin/appimagetool
```

### "Docker: permission denied"
**Problem**: User not in docker group

**Solution**:
```bash
sudo usermod -aG docker $USER
# Log out and back in
```

### "How do I know if my build will use Docker?"
**Problem**: Want to know before building if Docker is needed

**Solution**:
```bash
# Check your platform
./detect-platform.sh --verbose

# If output shows "Platform: debian12" and you run "make debian12",
# it will use native build (no Docker needed)

# If output shows "Platform: debian12" and you run "make debian13",
# it will use Docker build
```

---

## File Sizes

Typical build sizes:
- Native binary: ~40-50 MB
- Debian-specific Docker build: ~45-55 MB
- AppImage: ~110-130 MB (includes all dependencies)
- Windows .exe: ~35-45 MB

---

## CI/CD Integration

### GitHub Actions Example
```yaml
name: Build Releases

on:
  release:
    types: [created]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Build AppImage
        run: make appimage

      - name: Build Debian 12
        run: make debian12

      - name: Build Debian 13
        run: make debian13

      - name: Upload Artifacts
        uses: actions/upload-artifact@v3
        with:
          name: mockelot-builds
          path: build/bin/*
```

---

## Additional Resources

- [Wails Documentation](https://wails.io/docs/introduction)
- [AppImage Documentation](https://docs.appimage.org/)
- [WebKit2GTK](https://webkitgtk.org/)

---

## Summary

**Quick Decision Guide:**

| Use Case | Build Command | Output |
|----------|--------------|---------|
| 🎯 Distribute to Debian 12/Ubuntu 22.04 users | `make appimage-debian12` | AppImage for webkit 4.0 systems |
| 🎯 Distribute to Debian 13/Ubuntu 24.04+ users | `make appimage-debian13` | AppImage for webkit 4.1 systems |
| 🎯 Distribute to all Linux users | `make all-appimages` | Both AppImage variants |
| 🏢 Internal Debian 12 | `make debian12` | Native binary for Debian 12 |
| 🏢 Internal Debian 13 | `make debian13` | Native binary for Debian 13 |
| 💻 Local development | `make dev` | Development mode |
| 🪟 Windows users | `make windows` | Windows executable |

**TL;DR**:
- For distribution: `make all-appimages` (requires Docker)
- For development: `make dev` or `make linux`
