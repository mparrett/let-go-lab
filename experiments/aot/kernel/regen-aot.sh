#!/usr/bin/env bash
# regen-aot.sh — reproduce the AOT-lowered compute number (~70x vs interpreted).
#
# Lowers mandel_kernel.lg to Go with lg-compile, drops aot_bench_test.go into the
# generated package, and benches it. The generated Go imports the let-go runtime,
# so it must build inside the let-go module — hence the scratch package below.
#
# Needs a let-go >= 1.12 checkout with a built ./lg (the bench template calls the
# exported `MandelBench`; lg-compile switched to exported CamelCase names after
# 1.11.0). Resolution order:
#   ./regen-aot.sh /path/to/let-go   |   LETGO=/path/to/let-go ./regen-aot.sh
#   (default: the repo's ./let-go symlink)
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
LETGO="${1:-${LETGO:-$HERE/../../../let-go}}"
LG="$LETGO/lg"
[[ -x "$LG" ]] || { echo "regen-aot: no lg at $LG — build it or pass a checkout path" >&2; exit 1; }

SCRATCH="aot_mandel_gen"                       # scratch package dir inside the let-go module
PKG="github.com/nooga/let-go/$SCRATCH"

( cd "$LETGO"
  "./lg" scripts/lg-compile "$SCRATCH" "$PKG" "$HERE/mandel_kernel.lg"
  cp "$HERE/aot_bench_test.go" "$SCRATCH/mandel_kernel/"
  echo "==> AOT-lowered kernel (native float64 loop):"
  go test -run=TestMandelAOTValue -v "./$SCRATCH/mandel_kernel/"
  go test -run='^$' -bench=BenchmarkMandelAOT -benchtime=2s "./$SCRATCH/mandel_kernel/" )

echo "scratch package left at $LETGO/$SCRATCH (delete when done)"
