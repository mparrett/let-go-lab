#!/usr/bin/env bash
# scripts/ci.sh — let-go-lab smoke suite (#10). One entry point for local runs
# and CI (.github/workflows/ci.yml). It orchestrates the focused regressions that
# live with their features under test/ — protocol, lifecycle, and browser
# boundaries that have already produced bugs (#6/#7/#8).
#
# A check whose test file isn't present yet is SKIPPED with a visible notice, so
# this can land independent of merge order; once the test-bearing PRs are in, the
# checks run for real. Tools that aren't installed (shellcheck/jq) are also
# skipped — CI installs them.
#
# Usage: scripts/ci.sh        (LETGO=<path> to point at an lg checkout)

set -uo pipefail

LAB="$(cd "$(dirname "$0")/.." && pwd)"
cd "$LAB" || exit 1
LETGO="${LETGO:-$LAB/let-go}"
export LETGO
LG="$LETGO/lg"
PORT="${CI_PORT:-8252}"

fails=0
group() { echo; echo "==> $*"; }
run() { local name="$1"; shift; if "$@"; then echo "PASS: $name"; else echo "FAIL: $name"; fails=$((fails + 1)); fi; }

group "shell lint (bash -n + shellcheck)"
sh_files=()
for f in scripts/*.sh harness/*.sh; do [[ -f "$f" ]] && sh_files+=("$f"); done
run "bash -n" bash -n "${sh_files[@]}"
if command -v shellcheck >/dev/null; then
  run "shellcheck" shellcheck "${sh_files[@]}"
else
  echo "SKIP: shellcheck (not installed)"
fi

group "serve.json is valid JSON"
if command -v jq >/dev/null; then
  run "jq serve.json" jq empty harness/serve.json
else
  echo "SKIP: jq (not installed)"
fi

group "shell injector (#8)"
if [[ -f test/inject_shell_test.sh ]]; then
  run "inject-shell" bash test/inject_shell_test.sh
else
  echo "SKIP: injector (test/inject_shell_test.sh not present yet)"
fi

group "read-key EOF exit (#7)"
if [[ -f test/eof_exit_test.py ]]; then
  if [[ -x "$LG" ]]; then run "eof-exit" env LG="$LG" python3 test/eof_exit_test.py; else echo "SKIP: eof (no lg at $LG)"; fi
else
  echo "SKIP: eof (test/eof_exit_test.py not present yet)"
fi

group "build + browser viewport/click (#6)"
if [[ -x "$LG" && -f test/viewport_fit_test.py ]]; then
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
    run "viewport-fit" python3 test/viewport_fit_test.py "http://localhost:$PORT/"
  else
    echo "FAIL: server did not come up"; fails=$((fails + 1)); cat /tmp/ci-serve.log
  fi
  kill "$serve_pid" 2>/dev/null || true
else
  echo "SKIP: viewport (need lg + test/viewport_fit_test.py)"
fi

echo
if [[ "$fails" -eq 0 ]]; then
  echo "ci: all checks passed"
  exit 0
fi
echo "ci: $fails check(s) failed"
exit 1
