#!/usr/bin/env bash
# scripts/vendor-xterm.sh — (re)fetch the pinned xterm browser assets into
# harness/vendor/ and print their SHA-256s (issue #9). Run after bumping the
# versions below; update harness/vendor/README.md with the new checksums.
#
# This is the ONLY step that needs network. Day-to-day builds inline the
# committed files (harness/inline-assets.sh) and never touch the CDN.

set -euo pipefail

XTERM_VER="5.5.0"
FIT_VER="0.10.0"
IMAGE_VER="0.8.0"

LAB="$(cd "$(dirname "$0")/.." && pwd)"
DEST="$LAB/harness/vendor/xterm@${XTERM_VER}"
BASE="https://cdn.jsdelivr.net/npm"

mkdir -p "$DEST"
echo "==> fetching xterm@${XTERM_VER} (+ addon-fit@${FIT_VER}, addon-image@${IMAGE_VER})"
curl -fsSL "$BASE/@xterm/xterm@${XTERM_VER}/css/xterm.min.css"           -o "$DEST/xterm.min.css"
curl -fsSL "$BASE/@xterm/xterm@${XTERM_VER}/lib/xterm.min.js"            -o "$DEST/xterm.min.js"
curl -fsSL "$BASE/@xterm/addon-fit@${FIT_VER}/lib/addon-fit.min.js"      -o "$DEST/addon-fit.min.js"
curl -fsSL "$BASE/@xterm/addon-image@${IMAGE_VER}/lib/addon-image.min.js" -o "$DEST/addon-image.min.js"

echo "==> SHA-256 (update harness/vendor/README.md if these changed):"
( cd "$DEST" && shasum -a 256 xterm.min.css xterm.min.js addon-fit.min.js addon-image.min.js )
