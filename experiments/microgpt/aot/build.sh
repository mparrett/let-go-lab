#!/usr/bin/env bash
# Build microgpt.lg to a native binary via `lg compile`.
#
#   experiments/microgpt/aot/build.sh [out]        # PROFILE=1 adds the profiling hook
#
# lg comes from the lab's let-go checkout like the rest of the repo
# (LETGO=./let-go by default, see CLAUDE.md); LG and LETGO_SRC override it. lg
# compile needs a Go toolchain and the let-go source LG was built from.
#
# Needs let-go main at or after bb36063 (nooga/let-go#1044, not yet in a
# release): before it, the generated Go doesn't build (#562: math/* results
# land in float64 slots boxed).
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
# Both builds must write to the same file, even after the profiling build cd.
case "$out" in
  /*) ;;
  *) out=$PWD/$out ;;
esac
gen=$here/gen
rm -rf "$gen"
LETGO_SRC=$LETGO_SRC "$LG" compile -work "$gen" -o "$out" "$here/../microgpt.lg"
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
  (cd "$gen" && go build -o "$out" .)
fi
echo "built $out"
