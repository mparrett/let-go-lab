#!/usr/bin/env bash
# Compile a full demo with the audited upstream SHA, without patching generated Go.
# Usage: scripts/build-native.sh [mandelbrot|pathtrace|microgpt]
# LETGO_NATIVE selects an existing checkout of that SHA; otherwise cache one locally.
set -euo pipefail
LAB="$(cd "$(dirname "$0")/.." && pwd)"
PIN=$(cat "$LAB/config/let-go-native.sha")
DEMO=${1:-mandelbrot}
case "$DEMO" in
  mandelbrot|pathtrace|microgpt) ;;
  *) echo "build-native: supported demos: mandelbrot, pathtrace, microgpt" >&2; exit 1 ;;
esac
[[ "$PIN" =~ ^[0-9a-f]{40}$ ]] || { echo "build-native: invalid native SHA pin" >&2; exit 1; }
RUNTIME=${LETGO_NATIVE:-$LAB/.cache/let-go-native/$PIN}
if [[ ! -d "$RUNTIME" ]]; then
  [[ -z "${LETGO_NATIVE:-}" ]] || { echo "build-native: no checkout at $RUNTIME" >&2; exit 1; }
  [[ ! -e "$RUNTIME" ]] || { echo "build-native: not a directory: $RUNTIME" >&2; exit 1; }
  mkdir -p "$(dirname "$RUNTIME")"
  # Publish the cache only after fetching succeeds, so a failed download can retry.
  STAGING=$(mktemp -d "$RUNTIME.tmp.XXXXXX")
  trap 'rm -rf "$STAGING"' EXIT
  git -C "$STAGING" init -q
  git -C "$STAGING" remote add origin https://github.com/nooga/let-go.git
  git -C "$STAGING" fetch --depth 1 origin "$PIN"
  git -C "$STAGING" checkout --detach -q FETCH_HEAD
  mv "$STAGING" "$RUNTIME"
  trap - EXIT
fi
RUNTIME=$(cd "$RUNTIME" && pwd -P)
ACTUAL=$(git -C "$RUNTIME" rev-parse HEAD)
[[ "$ACTUAL" == "$PIN" ]] || {
  echo "build-native: checkout is $ACTUAL; expected $PIN" >&2; exit 1; }
# Canonical upstream build promotes bin/lg only after its smoke gates pass.
make -C "$RUNTIME" build
# shellcheck source=scripts/lib/lg-path.sh
. "$LAB/scripts/lib/lg-path.sh"
LG=$(lg_path "$RUNTIME")
OUT="$LAB/dist/native"
mkdir -p "$OUT"
if [[ "$DEMO" == microgpt ]]; then
  "$LAB/experiments/microgpt/fetch-reference.sh"
  LETGO="$RUNTIME" LETGO_SRC="$RUNTIME" LG="$LG" \
    "$LAB/experiments/microgpt/aot/build.sh" "$OUT/$DEMO"
else
  LETGO_SRC="$RUNTIME" "$LG" compile -work "$OUT/$DEMO-gen" \
    -o "$OUT/$DEMO" "$LAB/demos/$DEMO/$DEMO.lg"
fi
echo "native demo: $OUT/$DEMO (let-go $PIN)"
