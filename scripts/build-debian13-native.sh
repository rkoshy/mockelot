#!/bin/bash
# Build Mockelot for Debian 13 using the scorussolutions Docker image
# This is a convenience wrapper around build-debian-native.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}/.."

exec ./scripts/build-debian-native.sh debian13
