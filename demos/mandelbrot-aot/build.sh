#!/usr/bin/env bash
# Lower kernel.lg -> native Go (via lg-compile), then build ./mandel-native.
#
# Needs a let-go checkout with the ^double AOT param hints (#357) + int/float
# widening (#534), from 1.12, AND the int64 lowered signatures (#906) that
# native/main.go calls — i.e. >= 1.13. Defaults to the repo's ../../let-go symlink;
# override with:  LG=/path/to/let-go ./build.sh
set -euo pipefail
DEMO="$(cd "$(dirname "$0")" && pwd)"
LAB="$(cd "$DEMO/../.." && pwd)"
LG="${LG:-$LAB/let-go}"
LG="$(cd "$LG" && pwd -P)"
# shellcheck source=scripts/lib/lg-path.sh
. "$LAB/scripts/lib/lg-path.sh"
# $LG is the CHECKOUT (lg-compile is run from inside it); $LGBIN is the binary,
# which lives at bin/lg on a current let-go and at the root on older ones.
LGBIN="$(lg_path "$LG")"
[[ -x "$LGBIN" ]] || { echo "build.sh: no lg under $LG (tried bin/lg and lg) — build it: make -C $LG build" >&2; exit 1; }

echo "lowering kernel.lg via $LGBIN ..."
# lg-compile runs from the let-go checkout so it resolves gogen's classpath
# source (gogen isn't self-contained yet — nooga/let-go#425 Item 2).
( cd "$LG" && "$LGBIN" scripts/lg-compile "$DEMO/gen" mandelbrot-aot/gen "$DEMO/kernel.lg" )

# Version guards, read off the generated signature rather than any version
# string — the lowering is what has to match, and a checkout can be any commit.

# Pre-1.12: the float params lower to `int` and truncate.
if ! grep -h 'func Escape' "$DEMO"/gen/aot/kernel/*.go 2>/dev/null | grep -q 'float64'; then
  echo "error: escape() did not lower with float64 params." >&2
  echo "  the let-go at $LG lacks ^double AOT support (#357/#534, needs >= 1.12)." >&2
  echo "  point LG at a 1.13+ checkout:  LG=/path/to/let-go ./build.sh" >&2
  exit 1
fi

# Pre-1.13: the integer params/returns lower to `int`, not `int64`. native/main.go
# calls the int64 signatures, so without this guard Go reports eight type errors
# from a file the user did not write and cannot obviously fix.
if ! grep -h 'func Escape' "$DEMO"/gen/aot/kernel/*.go 2>/dev/null | grep -q 'int64'; then
  echo "error: escape() lowered with int params, not int64." >&2
  echo "  the let-go at $LG predates the int64 lowered signatures (#906, needs >= 1.13)." >&2
  echo "  native/main.go calls the int64 signatures and will not compile against these." >&2
  echo "  point LG at a 1.13+ checkout:  LG=/path/to/let-go ./build.sh" >&2
  exit 1
fi

( cd "$DEMO" && GOFLAGS=-mod=mod go build -ldflags '-s -w' -o mandel-native ./native )
echo "built $DEMO/mandel-native"
echo "try:  ./mandel-native interactive   |   ./mandel-native zoom 240 0 0   |   ./mandel-native bench"
