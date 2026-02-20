#!/bin/bash
set -euo pipefail

# Mockelot Build and Release Script
# Usage: ./build-and-release.sh <version>
# Example: ./build-and-release.sh v0.3.1
#
# This script uses Laminar CI for multi-platform builds:
#   - mockelot-linux-build    : Linux amd64 binary (Docker)
#   - mockelot-appimage-build : AppImages + .deb packages for Debian 12/13 (Docker)
#   - mockelot-macos-build    : macOS universal binary (remote SSH)
#   - mockelot-windows-build  : Windows amd64 binary (Docker cross-compile)
#
# Prerequisites:
#   - Laminar CI running (http://localhost:9000)
#   - SSH access to macOS machine (for macOS builds)
#   - gh CLI authenticated with GitHub
#
# CI job scripts: /opt/laminar/var/cfg/jobs/mockelot-*.run

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIST_DIR="${SCRIPT_DIR}/dist"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check arguments
if [ $# -lt 1 ]; then
    echo "Usage: $0 <version> [release-notes]"
    echo "Example: $0 v0.3.1"
    echo "Example: $0 v0.3.1 'Bug fixes and improvements'"
    exit 1
fi

VERSION="$1"
RELEASE_NOTES="${2:-}"

# Validate version format
if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-.*)?$ ]]; then
    log_error "Invalid version format: $VERSION"
    echo "Version must match pattern: vX.Y.Z or vX.Y.Z-suffix"
    exit 1
fi

log_info "Preparing release ${VERSION}"

# Check for uncommitted changes
cd "$SCRIPT_DIR"
if ! git diff --quiet || ! git diff --cached --quiet; then
    log_warn "You have uncommitted changes:"
    git status --short
    echo ""
    read -p "Do you want to continue anyway? (y/N) " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_error "Aborted. Please commit or stash your changes first."
        exit 1
    fi
fi

# Check if tag already exists
if git tag -l "$VERSION" | grep -q "$VERSION"; then
    log_error "Tag $VERSION already exists!"
    exit 1
fi

# Step 1: Trigger multi-platform builds
log_info "Triggering multi-platform builds..."
echo "  - Linux (amd64)"
echo "  - Linux AppImages + .deb (Debian 12 & 13)"
echo "  - macOS (universal)"
echo "  - Windows (amd64)"
echo ""

log_info "Starting Linux build..."
laminarc run mockelot-linux-build &
LINUX_PID=$!

log_info "Starting AppImage build..."
laminarc run mockelot-appimage-build &
APPIMAGE_PID=$!

log_info "Starting macOS build..."
laminarc run mockelot-macos-build &
MACOS_PID=$!

log_info "Starting Windows build..."
laminarc run mockelot-windows-build &
WINDOWS_PID=$!

log_info "Waiting for all builds to complete..."

# Wait for all builds and check exit status
FAILED=0

wait $LINUX_PID || { log_error "Linux build failed!"; FAILED=1; }
wait $APPIMAGE_PID || { log_error "AppImage build failed!"; FAILED=1; }
wait $MACOS_PID || { log_error "macOS build failed!"; FAILED=1; }
wait $WINDOWS_PID || { log_error "Windows build failed!"; FAILED=1; }

if [ $FAILED -eq 1 ]; then
    log_error "One or more builds failed. Aborting release."
    exit 1
fi

log_success "All builds completed successfully!"

# Step 2: Verify artifacts exist
log_info "Verifying build artifacts..."

LINUX_ARTIFACT="${DIST_DIR}/linux/mockelot-linux-amd64.tar.gz"
MACOS_ARTIFACT="${DIST_DIR}/macos/mockelot-darwin-universal.zip"
WINDOWS_ARTIFACT="${DIST_DIR}/windows/mockelot-windows-amd64.zip"

# Required artifacts
for artifact in "$LINUX_ARTIFACT" "$MACOS_ARTIFACT" "$WINDOWS_ARTIFACT"; do
    if [ ! -f "$artifact" ]; then
        log_error "Missing artifact: $artifact"
        exit 1
    fi
done

# Collect AppImage and .deb artifacts (version in filename uses git describe without v prefix)
VER_NO_V="${VERSION#v}"
RELEASE_ARTIFACTS=("$LINUX_ARTIFACT" "$MACOS_ARTIFACT" "$WINDOWS_ARTIFACT")

# Look for AppImage artifacts in dist/linux
for f in "${DIST_DIR}"/linux/*.AppImage; do
    [ -f "$f" ] && RELEASE_ARTIFACTS+=("$f")
done

# Look for .deb artifacts in dist/linux
for f in "${DIST_DIR}"/linux/*.deb; do
    [ -f "$f" ] && RELEASE_ARTIFACTS+=("$f")
done

# Generate checksums
log_info "Generating checksums..."
cd "${DIST_DIR}/linux"
sha256sum *.tar.gz *.AppImage *.deb 2>/dev/null > checksums.txt || true
cd "$SCRIPT_DIR"

log_success "All artifacts verified!"
echo ""
echo "Release artifacts:"
for artifact in "${RELEASE_ARTIFACTS[@]}"; do
    ls -lh "$artifact"
done
echo ""

# Step 3: Generate release notes if not provided
if [ -z "$RELEASE_NOTES" ]; then
    log_info "Generating release notes from git log..."

    # Get previous tag
    PREV_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")

    if [ -n "$PREV_TAG" ]; then
        RELEASE_NOTES=$(git log --pretty=format:"- %s" "${PREV_TAG}..HEAD" | grep -v "^- Merge" || true)
    else
        RELEASE_NOTES="Initial release"
    fi
fi

echo ""
log_info "Release notes:"
echo "----------------------------------------"
echo "$RELEASE_NOTES"
echo "----------------------------------------"
echo ""

# Step 4: Confirm release
read -p "Create release ${VERSION}? (y/N) " -n 1 -r
echo ""
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    log_warn "Release aborted."
    exit 0
fi

# Step 5: Create and push tag
log_info "Creating tag ${VERSION}..."
git tag -a "$VERSION" -m "Release ${VERSION}

${RELEASE_NOTES}"

log_info "Pushing commits and tag to origin..."
git push origin main 2>/dev/null || git push origin HEAD
git push origin "$VERSION"

log_success "Tag ${VERSION} pushed to origin!"

# Step 6: Create GitHub release with all artifacts
log_info "Creating GitHub release..."
gh release create "$VERSION" \
    "${RELEASE_ARTIFACTS[@]}" \
    --title "$VERSION" \
    --notes "## Changes

${RELEASE_NOTES}
"

log_success "Release ${VERSION} created successfully!"
echo ""
echo "View release at: https://github.com/rkoshy/mockelot/releases/tag/${VERSION}"
