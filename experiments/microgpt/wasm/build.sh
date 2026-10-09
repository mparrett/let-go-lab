#!/usr/bin/env bash
# Build microgpt.lg into a browser page via `lg -w`, which runs the bytecode VM
# compiled to wasm.
#
#   experiments/microgpt/wasm/build.sh [steps]           # train in the page (default 20), then sample
#   experiments/microgpt/wasm/build.sh --weights FILE    # interactive sampling page with trained weights
#   python3 scripts/serve.py --dir experiments/microgpt/wasm/out --headers harness/serve.json
#
# FILE comes from `lg microgpt.lg 1000 weights.txt` or the native build. With
# LETGO_USE_TINYGO=1 (tinygo on PATH) lg builds with TinyGo: a quarter of the
# page size and about twice the speed; this script then defaults
# LETGO_TINYGO_OPT to 2. The page needs cross-origin isolation (COOP/COEP),
# which scripts/serve.py sets.
#
# `lg -w` embeds no resources and the browser passes no argv, so this writes a
# copy of microgpt.lg to wasm/gen/ with input.txt inlined as a string and the
# entry point replaced. Run ../fetch-reference.sh first. lg comes from the
# lab's let-go checkout (LETGO=./let-go); LG and LETGO_SRC override it.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
lab=$(cd "$here/../../.." && pwd)
LETGO=${LETGO:-$lab/let-go}
# shellcheck source=../../../scripts/lib/lg-path.sh
. "$lab/scripts/lib/lg-path.sh"
LG=${LG:-$(lg_path "$LETGO")}
LETGO_SRC=${LETGO_SRC:-$LETGO}
[ -x "$LG" ] || { echo "no lg at $LG (set LETGO or LG)" >&2; exit 1; }
[ -f "$here/../input.txt" ] || { echo "no input.txt: run fetch-reference.sh first" >&2; exit 1; }
steps=20 weights=
case ${1:-} in
  --weights) weights=${2:?--weights needs a file}; [ -f "$weights" ] || { echo "no weights at $weights" >&2; exit 1; } ;;
  '') ;;
  *[!0-9]*) echo "usage: build.sh [steps] | --weights FILE" >&2; exit 1 ;;
  *) steps=$1 ;;
esac
[ "${LETGO_USE_TINYGO:-}" = 1 ] && export LETGO_TINYGO_OPT=${LETGO_TINYGO_OPT:-2}
rm -rf "$here/gen" "$here/out"
mkdir -p "$here/gen"
python3 -I - "$here/../microgpt.lg" "$here/../input.txt" "$steps" "$weights" "$here/gen/microgpt.lg" <<'PY'
import sys
src, data, steps, weights, out = sys.argv[1:]
s = open(src).read()
text = open(data).read()
lit = '"' + text.replace("\\", "\\\\").replace('"', '\\"') + '"'
if weights:
    ws = open(weights).read().split()
    [float(w) for w in ws]  # numbers only: they go into the source as a literal
    main = ('(when-not *compiling-aot*\n'
            '  (println "loaded %d trained weights")\n'
            '  (set-weights! [%s]))' % (len(ws), " ".join(ws)))
else:
    main = "(when-not *compiling-aot* (train! %s) (sample!))" % steps
for old, new in [('(slurp "input.txt")', lit),
                 ("(when-not *compiling-aot* (-main))", main)]:
    assert s.count(old) == 1, "microgpt.lg shape changed: " + old
    s = s.replace(old, new)
open(out, "w").write(s)
PY
if [ -n "$weights" ]; then
  # Keep the loaded image callable; the shell requests small batches of names.
  (cd "$here/gen" && LETGO_SRC=$LETGO_SRC "$LG" -w "$here/out" \
    -w-host-eval -w-shell "$here/shell.html" microgpt.lg)
else
  (cd "$here/gen" && LETGO_SRC=$LETGO_SRC "$LG" -w "$here/out" microgpt.lg)
fi
if [ -n "$weights" ]; then what="weights from $weights"; else what="$steps training steps"; fi
echo "built $here/out/index.html ($what)"
