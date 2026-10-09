#!/usr/bin/env bash
# Build microgpt.lg into a browser page via `lg -w`, the bytecode VM compiled to
# wasm (about 7x slower than `lg microgpt.lg`; ~0.6 s/token in Chromium).
#
#   experiments/microgpt/wasm/build.sh [steps]     # default 20; ~90 s of training
#   python3 scripts/serve.py --dir experiments/microgpt/wasm/out --headers harness/serve.json
#
# The page needs cross-origin isolation (COOP/COEP), which scripts/serve.py sets.
# `lg -w` embeds no resources and the browser passes no argv, so this writes a
# copy of microgpt.lg to wasm/gen/ with input.txt inlined as a string and the
# default step count replaced. Run ../fetch-reference.sh first. lg comes from
# the lab's let-go checkout (LETGO=./let-go); LG and LETGO_SRC override it.
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
steps=${1:-20}
case $steps in ''|*[!0-9]*) echo "steps must be a number: $steps" >&2; exit 1 ;; esac
rm -rf "$here/gen" "$here/out"
mkdir -p "$here/gen"
python3 -I - "$here/../microgpt.lg" "$here/../input.txt" "$steps" "$here/gen/microgpt.lg" <<'PY'
import sys
src, data, steps, out = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
s = open(src).read()
text = open(data).read()
lit = '"' + text.replace("\\", "\\\\").replace('"', '\\"') + '"'
for old, new in [('(slurp "input.txt")', lit),
                 ("(parse-long (str a)) 1000)", "(parse-long (str a)) %s)" % steps)]:
    assert s.count(old) == 1, "microgpt.lg shape changed: " + old
    s = s.replace(old, new)
open(out, "w").write(s)
PY
(cd "$here/gen" && LETGO_SRC=$LETGO_SRC "$LG" -w "$here/out" microgpt.lg)
echo "built $here/out/index.html ($steps steps)"
