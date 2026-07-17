#!/usr/bin/env bash
# Lower kernel.lg -> native Go (via lg-compile), then build ./mandel-native.
#
# Needs a let-go checkout with the ^double AOT param hints (#357) + int/float
# widening (#534) — i.e. >= 1.12. Defaults to the repo's ../../let-go symlink;
# override with:  LG=/path/to/let-go ./build.sh
set -euo pipefail
DEMO="$(cd "$(dirname "$0")" && pwd)"
LG="${LG:-$DEMO/../../let-go}"
LG="$(cd "$LG" && pwd -P)"

echo "lowering kernel.lg via $LG/lg ..."
# lg-compile runs from the let-go checkout so it resolves gogen's classpath
# source (gogen isn't self-contained yet — nooga/let-go#425 Item 2).
( cd "$LG" && ./lg scripts/lg-compile "$DEMO/gen" mandelbrot-aot/gen "$DEMO/kernel.lg" )

# Version guard: on a pre-1.12 let-go the float params lower to `int` and truncate.
if ! grep -h 'func Escape' "$DEMO"/gen/aot/kernel/*.go 2>/dev/null | grep -q 'float64'; then
  echo "error: escape() did not lower with float64 params." >&2
  echo "  the let-go at $LG lacks ^double AOT support (#357/#534, needs >= 1.12)." >&2
  echo "  point LG at a 1.12+ checkout:  LG=/path/to/let-go ./build.sh" >&2
  exit 1
fi

( cd "$DEMO" && GOFLAGS=-mod=mod go build -ldflags '-s -w' -o mandel-native ./native )
echo "built $DEMO/mandel-native"
echo "try:  ./mandel-native zoom 240 0 0   |   ./mandel-native ascii   |   ./mandel-native bench"
