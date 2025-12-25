# Docker Build Environment for Local Testing

This directory contains a custom Docker image for testing Mockelot builds locally with `act`.

## Quick Start

### 1. Build the Docker Image

```bash
cd .github/docker
./build-image.sh
```

This creates a Docker image (`mockelot-wails-build:latest`) with all dependencies pre-installed:
- Go 1.23
- Node.js 20
- GTK 3 development libraries
- WebKit2GTK development libraries
- Wails CLI
- AppImage tools

### 2. Test Locally with Act

From the project root:

```bash
# Test Linux build only
~/go/bin/act workflow_dispatch -j build --matrix name:linux-amd64 \
  -P ubuntu-22.04=mockelot-wails-build:latest

# Test with auto-configuration (if .actrc exists)
~/go/bin/act workflow_dispatch -j build --matrix name:linux-amd64
```

### 3. Optional: Create .actrc

Create `.actrc` in project root for automatic image selection:

```
-P ubuntu-22.04=mockelot-wails-build:latest
```

Then you can simply run:
```bash
~/go/bin/act workflow_dispatch -j build --matrix name:linux-amd64
```

## Benefits

- **Faster local testing**: Dependencies are pre-installed, no need to download/install on each run
- **Consistent environment**: Same dependencies as CI/CD
- **Offline capable**: Once built, can test without internet (except for Go module downloads)

## Rebuilding

Rebuild the image after updating dependencies:

```bash
./build-image.sh
```

## Image Size

The built image is approximately 2-3GB (includes Go toolchain, Node.js, and all GTK libraries).
