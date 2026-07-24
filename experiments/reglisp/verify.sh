#!/usr/bin/env bash
# Reproduce the verification in this experiment's README.
#
#   ./verify.sh          run the prototype's demos, then the claim tests
#   ./verify.sh mutate   ALSO run the ensureStack mutation check (deletes the
#                        open-upvalue fixup in a throwaway copy; expects the
#                        open-upvalue probe to flip 42 -> 0)
set -euo pipefail
cd "$(dirname "$0")"

echo "=== demos (go run .) ==="
go run .

echo
echo "=== claim tests (verbose) ==="
go test -v ./...

if [[ "${1:-}" == "mutate" ]]; then
  echo
  echo "=== mutation check: delete the open-upvalue fixup in ensureStack ==="
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT
  cp ./*.go ./go.mod "$tmp"/
  # strip the `for uv := vm.openUV ... { uv.stack = newStack }` loop
  perl -0pi -e 's/\n\tfor uv := vm\.openUV.*?\n\t\}\n/\n\t\/\/ MUTANT: fixup removed\n/s' "$tmp/vm.go"
  ( cd "$tmp"
    got="$(go run . 2>/dev/null >/dev/null; printf '')"  # demos still run
    # run only the open-upvalue probe and show it now reads the wrong cell
    go test -run TestOpenUpvalueSurvivesStackRegrow -v ./... 2>&1 | grep -E 'WRONG cell|FAIL|got [0-9]' || true )
  echo "(a FAIL above with 'got 0, want 42' is the expected mutant behavior)"
fi
