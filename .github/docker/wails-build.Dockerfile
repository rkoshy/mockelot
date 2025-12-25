FROM ubuntu:22.04

# Set non-interactive mode for apt
ENV DEBIAN_FRONTEND=noninteractive

# Install build dependencies in a single layer
RUN apt-get update -qq && \
    apt-get install -y -qq \
      build-essential \
      pkg-config \
      libgtk-3-dev \
      libwebkit2gtk-4.0-dev \
      curl \
      wget \
      git \
      sudo \
      ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Install Go 1.23
RUN wget -q https://go.dev/dl/go1.23.4.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.23.4.linux-amd64.tar.gz && \
    rm go1.23.4.linux-amd64.tar.gz

# Install Node.js 20
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && \
    apt-get install -y nodejs && \
    rm -rf /var/lib/apt/lists/*

# Set up Go environment
ENV PATH="/usr/local/go/bin:/root/go/bin:${PATH}"
ENV GOPATH="/root/go"

# Install Wails CLI
RUN go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Install AppImage tools (for Linux builds)
RUN wget -q -O /usr/local/bin/appimagetool \
    https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage && \
    chmod +x /usr/local/bin/appimagetool

# Set working directory
WORKDIR /workspace

# Verify installations
RUN go version && \
    node --version && \
    npm --version && \
    wails version && \
    pkg-config --modversion gtk+-3.0 && \
    echo "Build environment ready!"

# Add labels
LABEL org.opencontainers.image.description="Wails build environment for Mockelot"
LABEL org.opencontainers.image.source="https://github.com/rkoshy/mockelot"
