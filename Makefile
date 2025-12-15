# Mockelot Build Makefile
# Builds for Linux and Windows platforms

.PHONY: all linux windows clean dev help

# Default target
all: linux windows

# Build for Linux (amd64)
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

# Clean build artifacts
clean:
	@echo "Cleaning build directory..."
	rm -rf build/bin/*
	@echo "Clean complete"

# Run in development mode
dev:
	@echo "Starting development mode..."
	~/go/bin/wails dev

# Show help
help:
	@echo "Mockelot Build Targets:"
	@echo "  make          - Build for both Linux and Windows (default)"
	@echo "  make linux    - Build for Linux only"
	@echo "  make windows  - Build for Windows only"
	@echo "  make both     - Build for both platforms"
	@echo "  make clean    - Remove build artifacts"
	@echo "  make dev      - Run in development mode"
	@echo "  make help     - Show this help message"
