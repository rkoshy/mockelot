# Mockelot Build Scripts

This directory contains build and utility scripts for Mockelot.

## Build Scripts

### build-debian-native.sh
Builds native Mockelot binaries in Docker containers for Debian 12 and 13.
Usage: `./build-debian-native.sh debian12|debian13`

### build-debian13-native.sh
Specialized script for building on Debian 13.

### build-deb-package.sh
Creates Debian .deb packages from built AppImages.
Usage: `./build-deb-package.sh <version> <appimage-path>`

## Utility Scripts

### detect-platform.sh
Detects the current platform for appropriate build configuration.

### install-deps.sh
Installs build dependencies for local development.

### create-test-vm.sh
Creates a test VM for testing Mockelot builds.

### vm-setup.sh
Sets up the VM environment for testing.

### test-socks5.sh
Tests the SOCKS5 proxy functionality.

## Note

The main build processes are controlled by the Makefile in the parent directory.
Use `make help` for available build targets.