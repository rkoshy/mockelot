#!/bin/bash
# Mockelot Release Script
# Usage: ./release.sh <version> [release-notes]
# Example: ./release.sh v0.3.6 'Bug fixes and improvements'
#
# Builds all platforms via Laminar CI, creates .deb packages,
# generates checksums, tags, and creates a GitHub release.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/_common.sh"

# --- Validate arguments ---
if [ $# -lt 1 ]; then
    echo "Usage: $0 <version> [release-notes]"
    echo "Example: $0 v0.3.6"
    exit 1
fi

VERSION_TAG="$1"
RELEASE_NOTES="${2:-}"

if [[ ! "$VERSION_TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-.*)?$ ]]; then
    log_error "Invalid version format: $VERSION_TAG (expected vX.Y.Z or vX.Y.Z-suffix)"
    exit 1
fi

export VERSION="${VERSION_TAG#v}"

cd "$PROJECT_DIR"

# --- Check tree ---
if ! git diff --quiet || ! git diff --cached --quiet; then
    log_warn "Uncommitted changes:"
    git status --short
    read -p "Continue anyway? (y/N) " -n 1 -r; echo
    [[ $REPLY =~ ^[Yy]$ ]] || { log_error "Aborted."; exit 1; }
fi

if git tag -l "$VERSION_TAG" | grep -q "$VERSION_TAG"; then
    log_error "Tag $VERSION_TAG already exists!"
    exit 1
fi

# --- Create local tag BEFORE building so git-describe returns the correct version ---
# (Laminar CI jobs use `git describe --tags` which needs the tag to exist)
# The tag is only pushed after a successful build + user confirmation.
log_info "Creating local tag ${VERSION_TAG} (for build versioning)..."
git tag -a "$VERSION_TAG" -m "Release ${VERSION_TAG}"

# --- Build all platforms ---
log_info "Starting builds for ${VERSION_TAG}..."
if ! "${SCRIPT_DIR}/build-all.sh" --laminar; then
    log_error "Build failed — removing local tag ${VERSION_TAG}"
    git tag -d "$VERSION_TAG" 2>/dev/null
    exit 1
fi

# --- Build .deb packages ---
log_info "Building .deb packages..."
"${SCRIPT_DIR}/build-deb.sh" "${VERSION}-debian12" "releases/mockelot-${VERSION}-debian12-x86_64.AppImage"
"${SCRIPT_DIR}/build-deb.sh" "${VERSION}-debian13" "releases/mockelot-${VERSION}-debian13-x86_64.AppImage"

mv -f mockelot_*-debian12_amd64.deb "${DIST_DIR}/linux/" 2>/dev/null || true
mv -f mockelot_*-debian13_amd64.deb "${DIST_DIR}/linux/" 2>/dev/null || true
log_success ".deb packages built."

# --- Checksums ---
log_info "Generating checksums..."
cd "${DIST_DIR}/linux"
sha256sum *.tar.gz *.AppImage *.deb 2>/dev/null > checksums.txt
cd "$PROJECT_DIR"

# --- Collect release artifacts ---
RELEASE_ARTIFACTS=(
    "${DIST_DIR}/linux/mockelot-linux-amd64.tar.gz"
    "${DIST_DIR}/windows/mockelot-windows-amd64.zip"
    "${DIST_DIR}/macos/mockelot-darwin-universal.zip"
    "${DIST_DIR}/linux/mockelot-${VERSION}-debian12-x86_64.AppImage"
    "${DIST_DIR}/linux/mockelot-${VERSION}-debian13-x86_64.AppImage"
)
for f in "${DIST_DIR}"/linux/*.deb; do
    [ -f "$f" ] && RELEASE_ARTIFACTS+=("$f")
done

echo ""
log_info "Release artifacts:"
for artifact in "${RELEASE_ARTIFACTS[@]}"; do
    ls -lh "$artifact"
done

# --- Generate release notes if not provided ---
if [ -z "$RELEASE_NOTES" ]; then
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

# --- Confirm ---
read -p "Create release ${VERSION_TAG}? (y/N) " -n 1 -r; echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    log_warn "Release aborted."
    exit 0
fi

# --- Push tag and code ---
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
