#!/usr/bin/env bash
# Build microgpt.lg to a native binary via `lg compile`.
#
#   experiments/microgpt/aot/build.sh [out]        # PROFILE=1 adds the profiling hook
#
# lg comes from the lab's let-go checkout like the rest of the repo
# (LETGO=./let-go by default, see CLAUDE.md); LG and LETGO_SRC override it. lg
# compile needs a Go toolchain and the let-go source LG was built from.
#
# Until nooga/let-go#1044 lands, the generated Go doesn't build (#562: math/*
# results land in float64 slots boxed); patch562.py unboxes the rejected sites
# and is a no-op ("patched 0 sites") on an lg that has the fix.
#
# Output: aot/microgpt-native (default); the generated Go module is kept in
# aot/gen/ (gitignored) for inspection.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
lab=$(cd "$here/../../.." && pwd)
LETGO=${LETGO:-$lab/let-go}
# shellcheck source=../../../scripts/lib/lg-path.sh
. "$lab/scripts/lib/lg-path.sh"
LG=${LG:-$(lg_path "$LETGO")}
LETGO_SRC=${LETGO_SRC:-$LETGO}
[ -x "$LG" ] || { echo "no lg at $LG (set LETGO or LG)" >&2; exit 1; }
out=${1:-$here/microgpt-native}
gen=$here/gen
rm -rf "$gen"
# The first build fails on #562; that's expected, the patch loop fixes it.
LETGO_SRC=$LETGO_SRC "$LG" compile -work "$gen" -o "$out" "$here/../microgpt.lg" >/dev/null 2>&1 || true
cp "$here/unbox562.go" "$gen/microgpt/"
if [ "${PROFILE:-0}" = 1 ]; then
  # Profiling hook (LG_CPUPROFILE / LG_MEMPROFILE); see profile_hook.go.
  cp "$here/profile_hook.go" "$gen/"
  python3 -I - "$gen/main.go" <<'PY'
import sys
p = sys.argv[1]; s = open(p).read()
a = "func main() {\n"
assert s.count(a) == 1, "main.go shape changed"
open(p, "w").write(s.replace(a, a + "\tdefer startProfiles()()\n"))
PY
fi
(cd "$gen" && python3 -I "$here/patch562.py" . && go build -o "$out" .)
echo "built $out"
