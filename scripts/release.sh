#!/bin/bash
# Mockelot Release Script
# Usage: ./release.sh <version>
# Example: ./release.sh v0.5.0
#
# Builds Linux/Windows/macOS via Laminar CI, builds a native Debian 13 .deb
# locally, generates checksums, tags, and creates a GitHub release.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/_common.sh"

# --- Validate arguments ---
if [ $# -lt 1 ]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 v0.5.0"
    exit 1
fi

VERSION_TAG="$1"

if [[ ! "$VERSION_TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-.*)?$ ]]; then
    log_error "Invalid version format: $VERSION_TAG (expected vX.Y.Z)"
    exit 1
fi

export VERSION="${VERSION_TAG#v}"

cd "$PROJECT_DIR"

# --- Check tree is clean ---
if ! git diff --quiet || ! git diff --cached --quiet; then
    log_warn "Uncommitted changes:"
    git status --short
    log_error "Please commit or stash before releasing."
    exit 1
fi

if git tag -l "$VERSION_TAG" | grep -q "$VERSION_TAG"; then
    log_error "Tag $VERSION_TAG already exists!"
    exit 1
fi

# --- Create local tag BEFORE building so git-describe returns the right version ---
log_info "Creating local tag ${VERSION_TAG} (for build versioning)..."
git tag -a "$VERSION_TAG" -m "Release ${VERSION_TAG}"

# --- Build Linux / Windows / macOS via Laminar ---
log_info "Starting platform builds for ${VERSION_TAG}..."
if ! "${SCRIPT_DIR}/build-all.sh" --laminar; then
    log_error "Build failed — removing local tag ${VERSION_TAG}"
    git tag -d "$VERSION_TAG" 2>/dev/null
    exit 1
fi

# --- Build native Debian 13 .deb locally ---
log_info "Building native Debian 13 .deb..."
if ! "${SCRIPT_DIR}/build-deb-native.sh"; then
    log_error "Native .deb build failed — removing local tag ${VERSION_TAG}"
    git tag -d "$VERSION_TAG" 2>/dev/null
    exit 1
fi
DEB_FILE="${PROJECT_DIR}/mockelot_${VERSION}-native-debian13_amd64.deb"
ensure_dist_dir linux
mv -f "$DEB_FILE" "${DIST_DIR}/linux/"
DEB_ARTIFACT="${DIST_DIR}/linux/mockelot_${VERSION}-native-debian13_amd64.deb"
log_success "Debian package: $(du -sh "$DEB_ARTIFACT" | cut -f1)"

# --- Checksums ---
log_info "Generating checksums..."
cd "${DIST_DIR}/linux"
sha256sum *.tar.gz *.deb > checksums.txt
cd "$PROJECT_DIR"

# --- Collect artifacts ---
RELEASE_ARTIFACTS=(
    "${DIST_DIR}/linux/mockelot-linux-amd64.tar.gz"
    "${DIST_DIR}/windows/mockelot-windows-amd64.zip"
    "${DIST_DIR}/macos/mockelot-darwin-universal.zip"
    "$DEB_ARTIFACT"
    "${DIST_DIR}/linux/checksums.txt"
)

echo ""
log_info "Release artifacts:"
for artifact in "${RELEASE_ARTIFACTS[@]}"; do
    ls -lh "$artifact"
done

# --- Release notes ---
PREV_TAG=$(git describe --tags --abbrev=0 "${VERSION_TAG}^" 2>/dev/null || echo "")
if [ -n "$PREV_TAG" ]; then
    RELEASE_NOTES=$(git log --pretty=format:"- %s" "${PREV_TAG}..${VERSION_TAG}" | grep -v "^- Merge" || true)
else
    RELEASE_NOTES="Initial release"
fi

echo ""
log_info "Release notes:"
echo "----------------------------------------"
echo "$RELEASE_NOTES"
echo "----------------------------------------"
echo ""

# --- Confirm ---
read -p "Create release ${VERSION_TAG}? (y/N) " -n 1 -r; echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    log_warn "Release aborted."
    exit 0
fi

# --- Push ---
log_info "Pushing ${VERSION_TAG}..."
git push origin main 2>/dev/null || git push origin HEAD
git push origin "$VERSION_TAG"
log_success "Tag ${VERSION_TAG} pushed."

# --- GitHub release ---
log_info "Creating GitHub release..."
gh release create "$VERSION_TAG" \
    "${RELEASE_ARTIFACTS[@]}" \
    --title "$VERSION_TAG" \
    --notes "## Changes

${RELEASE_NOTES}
"

log_success "Release ${VERSION_TAG} created!"
echo "View: https://github.com/rkoshy/mockelot/releases/tag/${VERSION_TAG}"
