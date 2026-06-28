#!/usr/bin/env bash
# scripts/build-pages.sh — build the mandelbrot demo into a GitHub Pages site dir.
#
# Produces a self-contained, offline, header-less-hostable bundle:
#   _site/index.html            the demo (wasm + xterm inlined, COI bootstrap in <head>)
#   _site/coi-serviceworker.js  re-adds COOP/COEP so the page is cross-origin isolated
#   _site/.nojekyll             tell Pages not to run Jekyll (serve files verbatim)
#
# Pages can't set COOP/COEP, which let-go's SharedArrayBuffer input ring needs;
# the service worker supplies them (see harness/coi-serviceworker.js). The bundle
# is single-page at the site root, so it works under a project subpath
# (https://<user>.github.io/<repo>/) with the SW registered by a relative path.
#
# Usage: build-pages.sh [demo] [out-dir]   (defaults: mandelbrot, _site)
# LETGO=<path> selects the lg checkout (default ./let-go).

set -euo pipefail

LAB="$(cd "$(dirname "$0")/.." && pwd)"
LETGO="${LETGO:-$LAB/let-go}"
LG="$LETGO/lg"
DEMO="${1:-mandelbrot}"
OUT="${2:-$LAB/_site}"

SRC="$LAB/demos/$DEMO"
SHELL_HTML="$SRC/shell.html"
LGFILE="$SRC/$DEMO.lg"
BUILD="$LAB/dist/$DEMO"

[[ -f "$LGFILE" ]]     || { echo "build-pages: no demo at $LGFILE" >&2; exit 1; }
[[ -f "$SHELL_HTML" ]] || { echo "build-pages: no shell at $SHELL_HTML" >&2; exit 1; }
[[ -x "$LG" ]]         || { echo "build-pages: no lg at $LG (set LETGO=<path>)" >&2; exit 1; }

echo "==> build: $LG -w -w-shell none $DEMO.lg"
mkdir -p "$BUILD"
(cd "$SRC" && LETGO_SRC="$LETGO" "$LG" -w "$BUILD" -w-shell none "$DEMO.lg")

# Inline the vendored xterm assets into the shell (offline), inject it, then
# inject the COI bootstrap into <head>. The resolved shell is a temp.
RESOLVED_SHELL="$(mktemp "${TMPDIR:-/tmp}/lg-shell-XXXXXX")"
trap 'rm -f "$RESOLVED_SHELL"' EXIT
"$LAB/harness/inline-assets.sh" "$SHELL_HTML" > "$RESOLVED_SHELL"
"$LAB/harness/inject-shell.sh" "$BUILD/index.html" "$RESOLVED_SHELL"
"$LAB/harness/inject-coi.sh"   "$BUILD/index.html" "$LAB/harness/coi-bootstrap.html"

# Assemble the site dir: demo at root + the SW + .nojekyll.
mkdir -p "$OUT"
cp "$BUILD/index.html" "$OUT/index.html"
cp "$LAB/harness/coi-serviceworker.js" "$OUT/coi-serviceworker.js"
touch "$OUT/.nojekyll"

echo "==> Pages site ready in $OUT/"
for f in .nojekyll index.html coi-serviceworker.js; do
  [[ -e "$OUT/$f" ]] && echo "    $f" || echo "    MISSING: $f"
done
echo "    index.html: $(wc -c < "$OUT/index.html") bytes"
