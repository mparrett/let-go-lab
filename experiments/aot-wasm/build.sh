#!/usr/bin/env bash
# Build the mandelbrot canvas page twice with TinyGo, through the same entry
# path: a control, and one that links the demo's lg-compile-lowered Go package
# into the wasm bundle. See README.md.
# Usage: experiments/aot-wasm/build.sh [OUT_DIR]
# LETGO (default ./let-go) is a built let-go checkout; tinygo must be on PATH.
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
LAB="$(cd "$HERE/../.." && pwd)"
LETGO="$(cd "${LETGO:-$LAB/let-go}" && pwd -P)"
OUT="$(mkdir -p "${1:-$LAB/dist/aot-wasm}" && cd "${1:-$LAB/dist/aot-wasm}" && pwd)"
# shellcheck source=../../scripts/lib/lg-path.sh
. "$LAB/scripts/lib/lg-path.sh"
LG=$(lg_path "$LETGO")
DEMO="$LAB/demos/mandelbrot"
REAL_TINYGO=$(command -v tinygo) || { echo "aot-wasm: tinygo not on PATH" >&2; exit 1; }

rm -rf "${OUT:?}/gen" "${OUT:?}/mod-aot" "${OUT:?}/mod-ctl" "${OUT:?}/bin" "${OUT:?}/ctl" "${OUT:?}/aot"

# 1. Lower the demo to Go. Only the generated package is used; its init()
#    registers the lowered functions as overrides on the mandelbrot vars.
(cd "$LAB" && LETGO_SRC="$LETGO" "$LG" compile -work "$OUT/gen" -o "$OUT/gen/native" \
  demos/mandelbrot/mandelbrot.lg)

# 2. Two modules with identical go.mod/go.sum: mod-aot also holds that package.
for arm in aot ctl; do
  mod="$OUT/mod-$arm"
  mkdir -p "$mod/lgprogram"
  touch "$mod/lgprogram/.lg-generated"      # lets lg -w-module write lgprogram/
  cp "$OUT/gen/go.sum" "$mod/"
  { echo "module example.com/mb$arm"; echo; echo "go 1.27"; echo
    sed -n '/^require (/,/^)/p' "$OUT/gen/go.mod" | sed 's| // indirect||'; echo
    echo "replace github.com/nooga/let-go => $LETGO"; } > "$mod/go.mod"
done
mkdir -p "$OUT/mod-aot/aot"
cp "$OUT/gen/mandelbrot/mandelbrot.go" "$OUT/mod-aot/aot/"

# 3. lg -w's generated main replays the program chunk, whose top-level
#    (-main) call starts the render loop before any override is installed.
#    A tinygo shim patches that main to the lg compile entry order first
#    (patch_main.py); both arms get the patch, so only the import differs.
mkdir -p "$OUT/bin"
cat > "$OUT/bin/tinygo" <<EOF
#!/usr/bin/env bash
# lg runs "tinygo build" inside lgprogram/ (and other subcommands, such as
# "tinygo env", elsewhere); refuse to build an unpatched main.
if [[ "\${1:-}" == build ]]; then
  grep -q 'unit.NSOrder\|aot-wasm entry' main.go || { echo "aot-wasm shim: no lg -w main.go in \$PWD" >&2; exit 1; }
  python3 "$HERE/patch_main.py" main.go
  grep -q 'aot-wasm entry' main.go || { echo "aot-wasm shim: patch did not apply" >&2; exit 1; }
fi
exec "$REAL_TINYGO" "\$@"
EOF
chmod +x "$OUT/bin/tinygo"

build() {   # build <arm> [lg -import args...]
  local arm=$1; shift
  (cd "$DEMO" && PATH="$OUT/bin:$PATH" LETGO_USE_TINYGO=1 LETGO_TINYGO_OPT=2 LETGO_SRC="$LETGO" \
    "$LG" -w "$OUT/$arm" -w-shell none -w-module "$OUT/mod-$arm" "$@" mandelbrot.lg)
  "$LAB/harness/inject-shell.sh" "$OUT/$arm/index.html" "$DEMO/shell.html"
}
build ctl
build aot -import example.com/mbaot/aot
echo "aot-wasm: built $OUT/ctl and $OUT/aot"
