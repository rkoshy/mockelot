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

AppImage bundles all dependencies into a single executable file that runs anywhere.

```bash
# Build AppImage
make appimage

# Result: build/bin/mockelot-x86_64.AppImage
# Runs on: Debian 11-13, Ubuntu 20.04+, Fedora, Arch, etc.
```

**Advantages:**
- ✅ Single file, no installation needed
- ✅ Works on virtually any Linux distro
- ✅ Bundles all dependencies including libwebkit
- ✅ Users just need to `chmod +x` and run

**Disadvantages:**
- ⚠️ Larger file size (~100MB+)
- ⚠️ First launch is slightly slower

**When to use**: Distributing to end users or when you don't know what distro they'll use.

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
# Build AppImage
make appimage

# Distribute: build/bin/mockelot-x86_64.AppImage
# Users run: chmod +x mockelot-x86_64.AppImage && ./mockelot-x86_64.AppImage
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
| 🎯 Distribute to users | `make appimage` | Universal Linux binary |
| 🏢 Internal Debian 12 | `make debian12` | Optimized for Debian 12 |
| 🏢 Internal Debian 13 | `make debian13` | Optimized for Debian 13 |
| 💻 Local development | `make dev` | Development mode |
| 🪟 Windows users | `make windows` | Windows executable |

**TL;DR**: Use `make appimage` unless you have a specific reason not to.
