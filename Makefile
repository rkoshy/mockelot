# Mockelot Build Makefile
# Builds for Linux and Windows platforms with multiple distribution options

.PHONY: all linux windows clean dev help appimage appimage-debian12 appimage-debian13 all-appimages debian12 debian13 docker-debian12 docker-debian13 all-local

# Default target
all: linux windows

# Build for Linux (amd64) - native
linux:
	@echo "Building for Linux (amd64)..."
	@if [ -x ./detect-platform.sh ]; then \
		WAILS_TAGS=$$(./detect-platform.sh --wails-tags); \
		if [ -n "$$WAILS_TAGS" ]; then \
			echo "Using build tags: $$WAILS_TAGS"; \
			~/go/bin/wails build $$WAILS_TAGS; \
		else \
			~/go/bin/wails build; \
		fi \
	else \
		~/go/bin/wails build; \
	fi
	@echo "Linux build complete: build/bin/mockelot"

# Build for Windows (amd64)
windows:
	@echo "Building for Windows (amd64)..."
	~/go/bin/wails build -platform windows/amd64
	@echo "Windows build complete: build/bin/mockelot.exe"

# Build for both platforms
both: linux windows
	@echo "All builds complete!"

# Build AppImage (portable Linux binary) - native build
appimage:
	@echo "Building AppImage for universal Linux compatibility..."
	@chmod +x build-appimage.sh
	@./build-appimage.sh

# Build AppImage for Debian 12 / Ubuntu 22.04 (webkit 4.0) using Docker
appimage-debian12:
	@echo "Building AppImage for Debian 12 / Ubuntu 22.04 (webkit 4.0) using Docker..."
	@chmod +x build-appimage-docker.sh
	@./build-appimage-docker.sh debian12

# Build AppImage for Debian 13 / Ubuntu 24.04 (webkit 4.1) using Docker
appimage-debian13:
	@echo "Building AppImage for Debian 13 / Ubuntu 24.04 (webkit 4.1) using Docker..."
	@chmod +x build-appimage-docker.sh
	@./build-appimage-docker.sh debian13

# Build both AppImage variants
all-appimages: appimage-debian12 appimage-debian13
	@echo "All AppImage builds complete!"

# Build for Debian 12 using Docker
debian12: docker-debian12

docker-debian12:
	@echo "Building for Debian 12 (Bookworm) using Docker..."
	@chmod +x build-docker.sh
	@./build-docker.sh debian12

# Build for Debian 13 using Docker
debian13: docker-debian13

docker-debian13:
	@echo "Building for Debian 13 (Trixie) using Docker..."
	@chmod +x build-docker.sh
	@./build-docker.sh debian13

# Build for all Debian versions
all-debian: debian12 debian13
	@echo "All Debian builds complete!"

# Clean build artifacts
clean:
	@echo "Cleaning build directory..."
	rm -rf build/bin/*
	rm -rf build/appimage
	@echo "Clean complete"

# Run in development mode
dev:
	@echo "Starting development mode..."
	~/go/bin/wails dev

# Show help
help:
	@echo "Mockelot Build Targets:"
	@echo ""
	@echo "Basic Builds:"
	@echo "  make              - Build for both Linux and Windows (default)"
	@echo "  make linux        - Build for Linux only (native system)"
	@echo "  make windows      - Build for Windows only"
	@echo "  make both         - Build for both platforms"
	@echo ""
	@echo "Distribution-Specific Builds:"
	@echo "  make appimage          - Build AppImage (native, current system webkit)"
	@echo "  make appimage-debian12 - Build AppImage for Debian 12/Ubuntu 22.04 (webkit 4.0, Docker)"
	@echo "  make appimage-debian13 - Build AppImage for Debian 13/Ubuntu 24.04 (webkit 4.1, Docker)"
	@echo "  make all-appimages     - Build both AppImage variants (Docker)"
	@echo "  make debian12          - Build native binary for Debian 12 (Bookworm) using Docker"
	@echo "  make debian13          - Build native binary for Debian 13 (Trixie) using Docker"
	@echo "  make all-debian        - Build native binaries for all Debian versions"
	@echo ""
	@echo "Development:"
	@echo "  make dev          - Run in development mode"
	@echo "  make clean        - Remove build artifacts"
	@echo "  make help         - Show this help message"
	@echo ""
	@echo "Recommendations:"
	@echo "  - For distribution: Use 'make appimage-debian12' or 'make appimage-debian13'"
	@echo "  - For development: Use 'make dev' or 'make linux'"
	@echo "  - AppImages bundle all dependencies including libwebkit"
	@echo "  - Debian 12 AppImage works on webkit 4.0 systems (Debian 12, Ubuntu 22.04)"
	@echo "  - Debian 13 AppImage works on webkit 4.1 systems (Debian 13, Ubuntu 24.04+)"

# Build locally and copy artifacts to DESTDIR (for CI/CD)
all-local:
	@echo "Building for local platform and copying to ${DESTDIR}..."
	@mkdir -p "${DESTDIR}"
	@if [ -d "build/bin" ]; then \
		cp -r build/bin/* "${DESTDIR}/" 2>/dev/null || true; \
	fi
	@if [ -f "build/bin/mockelot" ]; then \
		echo "Copied: mockelot (Linux binary)"; \
	fi
	@if [ -f "build/bin/mockelot.exe" ]; then \
		echo "Copied: mockelot.exe (Windows binary)"; \
	fi
	@if [ -f "build/bin/mockelot-x86_64.AppImage" ]; then \
		cp build/bin/mockelot-x86_64.AppImage "${DESTDIR}/" 2>/dev/null || true; \
		echo "Copied: mockelot-x86_64.AppImage"; \
	fi
	@echo "Build artifacts copied to ${DESTDIR}"
