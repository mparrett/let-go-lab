#!/usr/bin/env bash
# Build the pretrained sampler for header-free hosting, at /microgpt/.
# Usage: build-microgpt-pages.sh [out-dir] (default: _site/microgpt)
set -euo pipefail
LAB="$(cd "$(dirname "$0")/.." && pwd)"
OUT="${1:-$LAB/_site/microgpt}"
SRC="$LAB/experiments/microgpt"
"$SRC/fetch-reference.sh"
(cd "$SRC" && ./wasm/build.sh --weights weights.txt)
"$LAB/harness/inject-coi.sh" "$SRC/wasm/out/index.html" "$LAB/harness/coi-bootstrap.html"
mkdir -p "$OUT"
cp "$SRC/wasm/out/index.html" "$OUT/index.html"
cp "$LAB/harness/coi-serviceworker.js" "$OUT/coi-serviceworker.js"
touch "$OUT/.nojekyll"
echo "microgpt Pages site ready in $OUT/"
