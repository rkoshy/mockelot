# Mockelot Build Makefile
# Builds for Linux and Windows platforms with multiple distribution options

.PHONY: all linux windows clean dev help appimage debian12 debian13 docker-debian12 docker-debian13

# Default target
all: linux windows

# Build for Linux (amd64) - native
linux:
	@echo "Building for Linux (amd64)..."
	~/go/bin/wails build -platform linux/amd64
	@echo "Linux build complete: build/bin/mockelot"

# Build for Windows (amd64)
windows:
	@echo "Building for Windows (amd64)..."
	~/go/bin/wails build -platform windows/amd64
	@echo "Windows build complete: build/bin/mockelot.exe"

# Build for both platforms
both: linux windows
	@echo "All builds complete!"

# Build AppImage (portable Linux binary)
appimage:
	@echo "Building AppImage for universal Linux compatibility..."
	@chmod +x build-appimage.sh
	@./build-appimage.sh

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
	@echo "  make appimage     - Build AppImage (runs on all Linux distros)"
	@echo "  make debian12     - Build for Debian 12 (Bookworm) using Docker"
	@echo "  make debian13     - Build for Debian 13 (Trixie) using Docker"
	@echo "  make all-debian   - Build for all Debian versions"
	@echo ""
	@echo "Development:"
	@echo "  make dev          - Run in development mode"
	@echo "  make clean        - Remove build artifacts"
	@echo "  make help         - Show this help message"
	@echo ""
	@echo "Recommendations:"
	@echo "  - Use 'make appimage' for widest compatibility"
	@echo "  - Use 'make debian12' or 'make debian13' for distro-specific builds"
	@echo "  - AppImage bundles all dependencies including libwebkit"
