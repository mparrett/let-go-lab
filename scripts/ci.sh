#!/usr/bin/env bash
# scripts/ci.sh — let-go-lab smoke suite (#10). One entry point for local runs
# and CI (.github/workflows/ci.yml). It orchestrates the focused regressions that
# live with their features under test/ — protocol, lifecycle, and browser
# boundaries that have already produced bugs (#6/#7/#8).
#
# Strict vs lenient. LENIENT (local default): a missing test file or tool is a
# visible SKIP, so the suite runs before every test-bearing PR has merged. STRICT
# (CI, or CI_STRICT=1): a missing expected test/tool is a FAILURE, so once this is
# the required gate an omitted test/feature can't merge silently green. GitHub
# sets CI=true, so the workflow is authoritative without extra config.
#
# Usage: scripts/ci.sh                 (LETGO=<path> to point at an lg checkout)
#        CI_STRICT=1 scripts/ci.sh     (force strict locally)

set -uo pipefail

LAB="$(cd "$(dirname "$0")/.." && pwd)"
cd "$LAB" || exit 1
LETGO="${LETGO:-$LAB/let-go}"
export LETGO
LG="$LETGO/lg"
PORT="${CI_PORT:-8252}"

STRICT="${CI_STRICT:-}"
[[ -z "$STRICT" && -n "${CI:-}" ]] && STRICT=1

fails=0
group() { echo; echo "==> $*"; }
run() { local name="$1"; shift; if "$@"; then echo "PASS: $name"; else echo "FAIL: $name"; fails=$((fails + 1)); fi; }
# A required-but-absent test/tool: hard fail in strict, soft skip otherwise.
absent() {
  local name="$1" reason="$2"
  if [[ -n "$STRICT" ]]; then
    echo "FAIL: $name MISSING ($reason) — required in strict mode"
    fails=$((fails + 1))
  else
    echo "SKIP: $name ($reason)"
  fi
}

group "shell lint (bash -n + shellcheck)"
sh_files=()
for f in scripts/*.sh harness/*.sh; do [[ -f "$f" ]] && sh_files+=("$f"); done
run "bash -n" bash -n "${sh_files[@]}"
if command -v shellcheck >/dev/null; then
  run "shellcheck" shellcheck "${sh_files[@]}"
else
  absent "shellcheck" "not installed"
fi

group "serve.json is valid JSON"
if command -v jq >/dev/null; then
  run "jq serve.json" jq empty harness/serve.json
else
  absent "jq" "not installed"
fi

group "shell injector (#8)"
if [[ -f test/inject_shell_test.sh ]]; then
  run "inject-shell" bash test/inject_shell_test.sh
else
  absent "injector" "test/inject_shell_test.sh not present"
fi

group "read-key EOF exit (#7)"
if [[ ! -f test/eof_exit_test.py ]]; then
  absent "eof" "test/eof_exit_test.py not present"
elif [[ ! -x "$LG" ]]; then
  absent "eof" "no lg at $LG"
else
  run "eof-exit" env LG="$LG" python3 test/eof_exit_test.py
fi

group "build + browser viewport/click (#6)"
if [[ ! -f test/viewport_fit_test.py ]]; then
  absent "viewport" "test/viewport_fit_test.py not present"
elif [[ ! -x "$LG" ]]; then
  absent "viewport" "no lg at $LG"
else
  # Build + serve in the background, wait for the port, run the browser test,
  # then stop the server (serve.sh execs the server, so $! is that process).
  scripts/serve.sh mandelbrot --http --port "$PORT" > /tmp/ci-serve.log 2>&1 &
  serve_pid=$!
  up=0
  for _ in $(seq 1 150); do
    if curl -fsS "http://localhost:$PORT/" >/dev/null 2>&1; then up=1; break; fi
    sleep 1
  done
  if [[ "$up" == 1 ]]; then
    # The test exits 77 when playwright isn't importable — treat that as an
    # absent required tool (skip lenient / fail strict), not a pass.
    python3 test/viewport_fit_test.py "http://localhost:$PORT/"; rc=$?
    if [[ "$rc" -eq 0 ]]; then echo "PASS: viewport-fit"
    elif [[ "$rc" -eq 77 ]]; then absent "viewport-fit" "playwright not installed"
    else echo "FAIL: viewport-fit"; fails=$((fails + 1)); fi
  else
    echo "FAIL: server did not come up"; fails=$((fails + 1)); cat /tmp/ci-serve.log
  fi
  kill "$serve_pid" 2>/dev/null || true
fi

echo
if [[ "$fails" -eq 0 ]]; then
  echo "ci: all checks passed${STRICT:+ (strict)}"
  exit 0
fi
echo "ci: $fails check(s) failed"
exit 1
