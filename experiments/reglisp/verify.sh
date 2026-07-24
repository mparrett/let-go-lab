#!/usr/bin/env bash
# Reproduce the verification in this experiment's README.
#
#   ./verify.sh          run the prototype's demos, then the claim tests
#   ./verify.sh mutate   ALSO run the mutation checks: each deletes one
#                        load-bearing line from a throwaway copy and shows the
#                        matching test flip from PASS to FAIL, proving the test
#                        is actually sensitive to that code.
set -euo pipefail
cd "$(dirname "$0")"

echo "=== demos (go run .) ==="
go run .

echo
echo "=== claim tests (verbose) ==="
go test -v ./...

if [[ "${1:-}" == "mutate" ]]; then
  # run one mutation: $1 = perl edit applied to vm.go, $2 = test to run,
  # $3 = human description. A FAIL is the expected, desired outcome.
  run_mutation() {
    local edit="$1" test="$2" desc="$3"
    echo
    echo "=== mutation: $desc ==="
    local tmp; tmp="$(mktemp -d)"
    cp ./*.go ./go.mod "$tmp"/
    perl -0pi -e "$edit" "$tmp/vm.go"
    ( cd "$tmp" && go test -run "$test" -v ./... 2>&1 \
        | grep -E 'PASS|FAIL|want|WRONG' ) || true
    rm -rf "$tmp"
    echo "(a FAIL above is the expected mutant behavior)"
  }

  # 1. drop the open-upvalue fixup in ensureStack -> regrow probe flips 42 -> 0
  run_mutation \
    's/\n\tfor uv := vm\.openUV.*?\n\t\}\n/\n\t\/\/ MUTANT: fixup removed\n/s' \
    'TestOpenUpvalueSurvivesStackRegrow' \
    'delete the open-upvalue fixup loop in ensureStack'

  # 2. drop the reuse check in findOrCreateUpval -> closed-sharing flips 7 -> 0
  run_mutation \
    's/\n\tif cur != nil && cur\.idx == absIdx \{\n\t\treturn cur\n\t\}\n/\n\t\/\/ MUTANT: reuse removed\n/s' \
    'TestSharedUpvalueClosed' \
    'delete the reuse check in findOrCreateUpval'
fi
