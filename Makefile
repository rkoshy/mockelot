# Mockelot Build Makefile
# Local builds only - use Laminar CI for cross-platform builds
# For remote macOS/Windows builds: laminarc queue mockelot-build-all

.PHONY: all build clean dev help
.PHONY: appimage appimage-debian12 appimage-debian13 all-appimages
.PHONY: debian12 debian13 all-debian
.PHONY: deb deb-docker clean-appimage clean-debian all-local

# Version detection (from git tags)
VERSION ?= $(shell git describe --tags 2>/dev/null | sed 's/^v//' || echo "dev")

# Build directories
BUILD_DIR := build/bin
DIST_DIR := dist

# Shared Docker images for AppImage builds
APPIMAGE_BUILDER_D12 := scorussolutions/wails-appimage-builder-d12:latest
APPIMAGE_BUILDER_D13 := scorussolutions/wails-appimage-builder-d13:latest

# Default target - build for current platform
all: build

# ============================================================================
# Main Builds
# ============================================================================

# Build Mockelot for Linux amd64 (current platform)
build:
	@echo "Building Mockelot for Linux (amd64)..."
	@cd frontend && npm ci
	@wails build -platform linux/amd64 -o mockelot -ldflags "-X main.version=$(VERSION)"
	@echo "Linux build complete: $(BUILD_DIR)/mockelot"

# ============================================================================
# Debian-Specific Native Builds
# ============================================================================

# Build native binary for Debian 12 (Bookworm) - webkit 4.0
debian12:
	@echo "Building native binary for Debian 12 (Bookworm)..."
	@chmod +x scripts/build-debian-native.sh
	@./scripts/build-debian-native.sh debian12

# Build native binary for Debian 13 (Trixie) - webkit 4.1
debian13:
	@echo "Building native binary for Debian 13 (Trixie)..."
	@chmod +x scripts/build-debian-native.sh
	@./scripts/build-debian-native.sh debian13

# Build native binaries for all Debian versions
all-debian: debian12 debian13
	@echo "All Debian native builds complete!"

# ============================================================================
# AppImage Packaging (Using Shared Docker Images)
# ============================================================================

# Build AppImage for Debian 12 using temp build
appimage-debian12:
	@echo "Building AppImage for Debian 12 / Ubuntu 22.04 (webkit 4.0)..."
	@echo "Using shared Docker image: $(APPIMAGE_BUILDER_D12)"
	@mkdir -p releases
	@docker run --rm --privileged \
		-e VERSION=$(VERSION) \
		-e HOST_UID=$$(id -u) \
		-e HOST_GID=$$(id -g) \
		-v "$(PWD):/workspace" \
		-w /workspace \
		$(APPIMAGE_BUILDER_D12) \
		-c "chmod +x build-appimage-container.sh && ./build-appimage-container.sh debian12 && chown -R $$(id -u):$$(id -g) /workspace/releases /workspace/frontend/node_modules /workspace/frontend/dist /workspace/build 2>/dev/null || true"
	@echo "✓ AppImage created: releases/mockelot-$(VERSION)-debian12-x86_64.AppImage"

# Build AppImage for Debian 13 using temp build
appimage-debian13:
	@echo "Building AppImage for Debian 13 / Ubuntu 24.04 (webkit 4.1)..."
	@echo "Using shared Docker image: $(APPIMAGE_BUILDER_D13)"
	@mkdir -p releases
	@docker run --rm --privileged \
		-e VERSION=$(VERSION) \
		-e HOST_UID=$$(id -u) \
		-e HOST_GID=$$(id -g) \
		-v "$(PWD):/workspace" \
		-w /workspace \
		$(APPIMAGE_BUILDER_D13) \
		-c "chmod +x build-appimage-container.sh && ./build-appimage-container.sh debian13 && chown -R $$(id -u):$$(id -g) /workspace/releases /workspace/frontend/node_modules /workspace/frontend/dist /workspace/build 2>/dev/null || true"
	@echo "✓ AppImage created: releases/mockelot-$(VERSION)-debian13-x86_64.AppImage"

# Build both AppImage variants
all-appimages: appimage-debian12 appimage-debian13
	@echo "All AppImage builds complete!"
	@echo "Artifacts in releases/ directory"
	@ls -lah releases/

# Universal AppImage (Debian 12 base for maximum compatibility)
appimage: appimage-debian12

# ============================================================================
# Debian Package
# ============================================================================

# Build Debian package from Debian 12 AppImage
deb: appimage-debian12
	@echo "Building Debian package for Debian 12..."
	@chmod +x scripts/build-deb-package.sh
	@./scripts/build-deb-package.sh $(VERSION)-debian12 releases/mockelot-$(VERSION)-debian12-x86_64.AppImage

# Build Debian package using Docker
deb-docker: appimage-debian12
	@echo "Building Debian package in Docker..."
	@docker run --rm \
		-v "$(PWD):/workspace" \
		-w /workspace \
		$(APPIMAGE_BUILDER_D12) \
		-c "chmod +x scripts/build-deb-package.sh && ./scripts/build-deb-package.sh $(VERSION)-debian12 releases/mockelot-$(VERSION)-debian12-x86_64.AppImage && chown -R $$(id -u):$$(id -g) /workspace/*.deb 2>/dev/null || true"

# ============================================================================
# Development
# ============================================================================

# Run in development mode with hot reload
dev:
	@echo "Starting Mockelot in development mode..."
	@wails dev

# Run binary directly (after building)
run: build
	@echo "Running Mockelot..."
	@$(BUILD_DIR)/mockelot

# ============================================================================
# Cleaning
# ============================================================================

# Clean build artifacts
clean-build:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -rf frontend/dist
	@rm -rf frontend/node_modules
	@echo "Build clean complete"

# Clean AppImage build artifacts
clean-appimage:
	@echo "Cleaning AppImage artifacts..."
	@rm -rf AppDir
	@rm -rf releases/*.AppImage
	@rm -f mockelot-*.AppImage
	@rm -f Mockelot-*.AppImage
	@echo "AppImage clean complete"

# Clean Debian-specific build artifacts
clean-debian:
	@echo "Cleaning Debian-specific build artifacts..."
	@rm -f $(BUILD_DIR)/mockelot-debian12
	@rm -f $(BUILD_DIR)/mockelot-debian13
	@rm -f mockelot_*.deb
	@echo "Debian clean complete"

# Clean everything
clean: clean-build clean-appimage clean-debian
	@echo "Cleaning all build artifacts..."
	@rm -rf $(DIST_DIR)
	@echo "All clean complete"

# ============================================================================
# Distribution
# ============================================================================

# Build and copy artifacts to DESTDIR (for Laminar CI)
all-local: build
	@echo "Building for local platform and copying to ${DESTDIR}..."
	@if [ -z "${DESTDIR}" ]; then \
		echo "ERROR: DESTDIR not set"; \
		exit 1; \
	fi
	@mkdir -p "${DESTDIR}"
	@cp -r $(BUILD_DIR)/* "${DESTDIR}/" 2>/dev/null || true
	@echo "Build artifacts copied to ${DESTDIR}"
	@ls -lah "${DESTDIR}"

# ============================================================================
# Help
# ============================================================================

help:
	@echo "Mockelot Build Targets"
	@echo ""
	@echo "Quick Start:"
	@echo "  make              - Build for current platform"
	@echo "  make dev          - Run in development mode with hot reload"
	@echo ""
	@echo "Native Binary Builds:"
	@echo "  make build        - Build for current platform (auto-detects)"
	@echo "  make debian12     - Build native binary for Debian 12 (webkit 4.0)"
	@echo "  make debian13     - Build native binary for Debian 13 (webkit 4.1)"
	@echo "  make all-debian   - Build native binaries for all Debian versions"
	@echo ""
	@echo "AppImage Packaging (using shared Docker images):"
	@echo "  make appimage          - Build AppImage (Debian 12 base, max compatibility)"
	@echo "  make appimage-debian12 - Build AppImage for Debian 12/Ubuntu 22.04 (webkit 4.0)"
	@echo "  make appimage-debian13 - Build AppImage for Debian 13/Ubuntu 24.04 (webkit 4.1)"
	@echo "  make all-appimages     - Build both AppImage variants"
	@echo ""
	@echo "Debian Packaging:"
	@echo "  make deb               - Build .deb package (requires dpkg-deb)"
	@echo "  make deb-docker        - Build .deb package using Docker"
	@echo ""
	@echo "Laminar CI Integration:"
	@echo "  make all-local DESTDIR=/path  - Build and copy to DESTDIR (used by Laminar)"
	@echo ""
	@echo "Cross-Platform Builds (use Laminar CI):"
	@echo "  laminarc queue mockelot-build-all     - Build for all platforms"
	@echo "  laminarc queue mockelot-linux-build   - Build Linux binary"
	@echo "  laminarc queue mockelot-appimage-build - Build AppImages + .deb"
	@echo "  laminarc queue mockelot-macos-build   - Build macOS universal binary"
	@echo "  laminarc queue mockelot-windows-build - Build Windows binary"
	@echo ""
	@echo "Development:"
	@echo "  make dev               - Run in development mode"
	@echo "  make run               - Build and run"
	@echo ""
	@echo "Cleaning:"
	@echo "  make clean             - Remove all build artifacts"
	@echo "  make clean-build       - Remove build artifacts only"
	@echo "  make clean-appimage    - Remove AppImage artifacts only"
	@echo "  make clean-debian      - Remove Debian-specific artifacts only"
	@echo ""
	@echo "Version Control:"
	@echo "  VERSION=$(VERSION)"
	@echo "  Override with: make VERSION=0.3.5 <target>"
	@echo ""
	@echo "Docker Images Used:"
	@echo "  Debian 12: $(APPIMAGE_BUILDER_D12)"
	@echo "  Debian 13: $(APPIMAGE_BUILDER_D13)"
	@echo ""
	@echo "Requirements:"
	@echo "  - Go 1.22+ for builds"
	@echo "  - Wails v2 (go install github.com/wailsapp/wails/v2/cmd/wails@latest)"
	@echo "  - Node.js/npm for frontend builds"
	@echo "  - Docker for AppImage and Debian-specific builds"
	@echo "  - webkit2gtk-4.0 or 4.1 development libraries for local builds"
	@echo ""
	@echo "Recommendations:"
	@echo "  - For Debian 12 users: Use 'make appimage-debian12' or 'make debian12'"
	@echo "  - For Debian 13 users: Use 'make appimage-debian13' or 'make debian13'"
	@echo "  - For universal distribution: Use 'make appimage' (Debian 12 base)"
	@echo "  - AppImages bundle all dependencies including libwebkit"
