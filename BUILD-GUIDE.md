# Mockelot Build Guide

## Overview

Mockelot now supports building for multiple Debian versions to ensure compatibility across different Linux distributions. The build system uses Docker to create binaries and AppImages for both Debian 12 (webkit 4.0) and Debian 13 (webkit 4.1).

## Build Types

### 1. Native Binaries (Distribution-Specific)

Native binaries are built against specific Debian version libraries:

**Debian 12 (Bookworm) - webkit 4.0:**
```bash
make debian12
```
- Output: `build/bin/mockelot-debian12`
- Compatible with: Debian 12, Ubuntu 22.04, 23.04, 23.10, Linux Mint 21.x

**Debian 13 (Trixie) - webkit 4.1:**
```bash
make debian13
```
- Output: `build/bin/mockelot-debian13`
- Compatible with: Debian 13, Ubuntu 24.04+, Linux Mint 22.x+

**Both versions:**
```bash
make all-debian
```

### 2. AppImage Builds (Portable, Self-Contained)

AppImages bundle all dependencies including webkit libraries:

**Debian 12 AppImage (Maximum Compatibility):**
```bash
make appimage-debian12
```
- Output: `build/bin/mockelot-debian12-x86_64.AppImage`
- Compatible with: Most Linux distros (Debian 11+, Ubuntu 20.04+, Fedora, Arch, etc.)
- Recommended for distribution

**Debian 13 AppImage:**
```bash
make appimage-debian13
```
- Output: `build/bin/mockelot-debian13-x86_64.AppImage`
- Compatible with: Debian 13+, Ubuntu 24.04+

**Both AppImages:**
```bash
make all-appimages
```

**Universal AppImage (Debian 12 base via wails-appimage-builder):**
```bash
make appimage
```
- Uses pre-built Docker image for consistent builds
- Output: `mockelot-{VERSION}-x86_64.AppImage`

### 3. Debian Packages (.deb)

Create installable .deb packages from AppImages:

```bash
make deb              # Build .deb from AppImage (local)
make deb-docker       # Build .deb in Docker
```

Output: `mockelot_{VERSION}_amd64.deb`

Installs to:
- Binary: `/opt/mockelot/bin/mockelot-{VERSION}.AppImage`
- Symlink: `/usr/bin/mockelot`
- Desktop file: `/usr/share/applications/mockelot.desktop`

## CI/CD Builds (Laminar)

### Build All Platforms

```bash
laminarc queue mockelot-build-all
```

Queues:
- Linux binary (native)
- macOS universal binary
- Windows binary
- AppImages for Debian 12 and 13
- .deb packages for Debian 12 and 13

### Individual Platform Builds

```bash
laminarc queue mockelot-linux-build        # Linux native binary
laminarc queue mockelot-macos-build        # macOS universal binary
laminarc queue mockelot-windows-build      # Windows binary
laminarc queue mockelot-appimage-build     # All AppImages + .deb packages
```

## Release Artifacts

After a full build (`mockelot-build-all`), the following artifacts are created in `dist/`:

### Linux (`dist/linux/`)

**Native Binaries:**
- `mockelot-debian12` - Native binary for Debian 12
- `mockelot-debian13` - Native binary for Debian 13
- `mockelot-debian12-linux-amd64.tar.gz` - Compressed Debian 12 binary
- `mockelot-debian13-linux-amd64.tar.gz` - Compressed Debian 13 binary

**AppImages:**
- `mockelot-debian12-x86_64.AppImage` - Debian 12 AppImage (recommended)
- `mockelot-debian13-x86_64.AppImage` - Debian 13 AppImage

**Debian Packages:**
- `mockelot_{VERSION}-debian12_amd64.deb` - Debian 12 package
- `mockelot_{VERSION}-debian13_amd64.deb` - Debian 13 package

### macOS (`dist/macos/`)
- `mockelot-darwin-universal.zip` - Universal binary (arm64 + amd64)

### Windows (`dist/windows/`)
- `mockelot-windows-amd64.zip` - Windows binary

## Distribution Recommendations

### For GitHub Releases

Include the following artifacts for maximum user compatibility:

**Required:**
1. `mockelot-debian12-x86_64.AppImage` - Primary Linux binary (works on most distros)
2. `mockelot-darwin-universal.zip` - macOS universal binary
3. `mockelot-windows-amd64.zip` - Windows binary

**Optional:**
4. `mockelot-debian13-x86_64.AppImage` - For newer distros (Debian 13+, Ubuntu 24.04+)
5. `mockelot_{VERSION}-debian12_amd64.deb` - For apt/dpkg users
6. `mockelot_{VERSION}-debian13_amd64.deb` - For newer distros
7. `mockelot-debian12-linux-amd64.tar.gz` - Native binary for advanced users
8. `mockelot-debian13-linux-amd64.tar.gz` - Native binary for newer distros

### For Users

**Debian 12 / Ubuntu 22.04 users:**
- Recommended: `mockelot-debian12-x86_64.AppImage` (portable, no install needed)
- Alternative: `mockelot_{VERSION}-debian12_amd64.deb` (system integration via apt)

**Debian 13 / Ubuntu 24.04+ users:**
- Recommended: `mockelot-debian13-x86_64.AppImage`
- Alternative: `mockelot_{VERSION}-debian13_amd64.deb`

**Other Linux distros:**
- Use: `mockelot-debian12-x86_64.AppImage` (maximum compatibility)

**macOS users:**
- Use: `mockelot-darwin-universal.zip`

**Windows users:**
- Use: `mockelot-windows-amd64.zip`

## Development Workflow

### Local Development
```bash
make dev              # Run with hot reload
make build            # Build for current platform
make run              # Build and run
```

### Clean Builds
```bash
make clean            # Clean all artifacts
make clean-build      # Clean build artifacts only
make clean-appimage   # Clean AppImage artifacts only
make clean-debian     # Clean Debian-specific artifacts only
```

### Testing Different Versions
```bash
# Build both Debian versions
make all-debian

# Test Debian 12 binary
./build/bin/mockelot-debian12

# Test Debian 13 binary
./build/bin/mockelot-debian13

# Build and test AppImages
make all-appimages
chmod +x build/bin/mockelot-debian12-x86_64.AppImage
./build/bin/mockelot-debian12-x86_64.AppImage
```

## Requirements

### Local Builds
- Go 1.22+
- Wails v2 (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
- Node.js/npm
- webkit2gtk-4.0 or 4.1 development libraries

### Docker Builds
- Docker (for AppImage and Debian-specific builds)
- No webkit dependencies needed (handled in containers)

### CI Builds
- Laminar CI running
- macOS accessible at `10.100.102.102` (for macOS builds)
- Docker available on build host

## Troubleshooting

### AppImage won't run
- Check FUSE is installed: `sudo apt install libfuse2`
- Make executable: `chmod +x mockelot-*.AppImage`
- Run from terminal to see errors: `./mockelot-*.AppImage`

### Debian 12 binary won't run on Debian 13
- Use Debian 13 binary instead: `./mockelot-debian13`
- Or use AppImage which works on both

### Build fails with webkit errors
- Use Docker builds instead of native builds
- Docker handles webkit dependencies automatically

## Version Information

Current version is automatically detected from git tags:
```bash
git describe --tags
# Output: v0.3.4
```

Override version for builds:
```bash
make VERSION=0.4.0 appimage-debian12
```
