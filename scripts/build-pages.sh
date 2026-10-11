#!/usr/bin/env bash
# scripts/build-pages.sh — build every browser demo and the index into a Pages site dir.
#
# Produces a self-contained, offline, header-less-hostable site:
#   _site/index.html            the index: a screenshot tile per demo (site/index.html)
#   _site/img/                  its screenshots (site/img/)
#   _site/<demo>/index.html     each demo (wasm + xterm inlined, COI bootstrap in <head>)
#   _site/<demo>/coi-serviceworker.js  re-adds COOP/COEP within that demo's scope
#   _site/microgpt/             pretrained sampler + its scoped COI service worker
#   _site/coi-serviceworker.js  kept at the root: browsers that visited when
#                               mandelbrot lived there still have it registered
#   _site/.nojekyll             tell Pages not to run Jekyll (serve files verbatim)
#
# Pages can't set COOP/COEP, which let-go's SharedArrayBuffer input ring needs;
# the service worker supplies them (see harness/coi-serviceworker.js). Each demo
# is single-page in its own directory and registers its SW by a relative path, so
# the site works under a project subpath (https://<user>.github.io/<repo>/).
#
# Usage: build-pages.sh [out-dir]   (default: _site)
# LETGO=<path> selects the lg checkout (default ./let-go).

set -euo pipefail

LAB="$(cd "$(dirname "$0")/.." && pwd)"
LETGO="${LETGO:-$LAB/let-go}"
# shellcheck source=scripts/lib/lg-path.sh
. "$LAB/scripts/lib/lg-path.sh"
LG="$(lg_path "$LETGO")"
OUT="${1:-$LAB/_site}"
# Browser demos under demos/, each served at /<demo>/. Keep in step with the
# tiles in site/index.html.
DEMOS=(mandelbrot mandelbrot-sixel pathtrace ptcanvas)

[[ -x "$LG" ]] || { echo "build-pages: no lg at $LG (set LETGO=<path>)" >&2; exit 1; }
"$LG" -h 2>&1 | grep -q -- '-w-shell' || {
  echo "build-pages: $LG predates -w-shell (#245); point LETGO at upstream main or later" >&2; exit 1; }

RESOLVED_SHELL="$(mktemp "${TMPDIR:-/tmp}/lg-shell-XXXXXX")"
trap 'rm -f "$RESOLVED_SHELL"' EXIT

build_demo() {
  local demo=$1 src="$LAB/demos/$1" build="$LAB/dist/$1"
  [[ -f "$src/$demo.lg" ]]   || { echo "build-pages: no demo at $src/$demo.lg" >&2; exit 1; }
  [[ -f "$src/shell.html" ]] || { echo "build-pages: no shell at $src/shell.html" >&2; exit 1; }
  echo "==> build: $LG -w -w-shell none $demo.lg"
  mkdir -p "$build"
  (cd "$src" && LETGO_SRC="$LETGO" "$LG" -w "$build" -w-shell none "$demo.lg")
  # Inline the vendored xterm assets into the shell (offline), inject it, then
  # inject the COI bootstrap into <head>.
  "$LAB/harness/inline-assets.sh" "$src/shell.html" > "$RESOLVED_SHELL"
  "$LAB/harness/inject-shell.sh" "$build/index.html" "$RESOLVED_SHELL"
  "$LAB/harness/inject-coi.sh"   "$build/index.html" "$LAB/harness/coi-bootstrap.html"
  mkdir -p "$OUT/$demo"
  cp "$build/index.html" "$OUT/$demo/index.html"
  cp "$LAB/harness/coi-serviceworker.js" "$OUT/$demo/coi-serviceworker.js"
}

mkdir -p "$OUT"
for demo in "${DEMOS[@]}"; do build_demo "$demo"; done
LETGO="$LETGO" "$LAB/scripts/build-microgpt-pages.sh" "$OUT/microgpt"

cp "$LAB/site/index.html" "$OUT/index.html"
rm -rf "$OUT/img"
cp -R "$LAB/site/img" "$OUT/img"
cp "$LAB/harness/coi-serviceworker.js" "$OUT/coi-serviceworker.js"
touch "$OUT/.nojekyll"

echo "==> Pages site ready in $OUT/"
missing=0
for f in .nojekyll index.html coi-serviceworker.js "${DEMOS[@]/%//index.html}" microgpt/index.html; do
  if [[ -e "$OUT/$f" ]]; then echo "    $f ($(wc -c < "$OUT/$f") bytes)"
  else echo "    MISSING: $f" >&2; missing=1; fi
done
exit "$missing"
