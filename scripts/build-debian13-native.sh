#!/bin/bash

# Redirect all output to build log files
BUILD_LOG="build-logs/debian13-build.log"
ERROR_LOG="build-logs/debian13-error.log"

# Ensure build-logs directory exists
mkdir -p build-logs

# Build the Docker image
echo "Building Docker image..." | tee -a "$BUILD_LOG"
if ! docker build -t mockelot-debian13 -f Dockerfile.debian13 . > "$BUILD_LOG" 2> "$ERROR_LOG"; then
    echo "Docker image build failed. Check $ERROR_LOG for details." >&2
    exit 1
fi

# Run build inside Docker
echo "Running build inside Docker container..." | tee -a "$BUILD_LOG"
if ! docker run --rm --network=host -v "$(pwd)":/build mockelot-debian13 /bin/bash -c "cd /build/frontend && npm install && cd .. && go mod tidy && wails build -platform linux/amd64" >> "$BUILD_LOG" 2>> "$ERROR_LOG"; then
    echo "Build inside Docker container failed. Check $ERROR_LOG for details." >&2
    exit 1
fi

# Create tar.gz archive
echo "Creating tar.gz archive..." | tee -a "$BUILD_LOG"
if ! tar -czf mockelot-linux-debian13-amd64.tar.gz -C build/bin mockelot >> "$BUILD_LOG" 2>> "$ERROR_LOG"; then
    echo "Archive creation failed. Check $ERROR_LOG for details." >&2
    exit 1
fi

# Verify artifact
if [ -f mockelot-linux-debian13-amd64.tar.gz ]; then
    echo "Build successful. Artifact created: mockelot-linux-debian13-amd64.tar.gz" | tee -a "$BUILD_LOG"
    exit 0
else
    echo "Artifact creation failed." >&2
    exit 1
fi