#!/usr/bin/env bash
# harness/inject-shell.sh — inject a client shell into a `lg -w -w-shell none`
# bundle's index.html, just before </body>.
#
# The shell-less bundle exposes window.LetGoHost; the shell binds to the runtime
# only through that contract. Baking it into the single self-contained index.html
# at build time replaces let-go's retired slot-fetch model (a host.html fragment
# fetched at boot via loadShell).
#
# Replace-idempotent: the injected block is wrapped in sentinel comments, so
# re-running swaps in the current shell rather than appending a duplicate — this
# keeps the fast shell-edit loop alive (`serve.sh --no-build` re-injects the
# edited shell without a WASM rebuild).
#
# Usage: inject-shell.sh <index.html> <shell.html>

set -euo pipefail

INDEX="${1:?usage: inject-shell.sh <index.html> <shell.html>}"
SHELL_HTML="${2:?usage: inject-shell.sh <index.html> <shell.html>}"

START="<!--LETGO-LAB-SHELL-START-->"
END="<!--LETGO-LAB-SHELL-END-->"

[[ -f "$INDEX" ]] || { echo "inject-shell: no $INDEX" >&2; exit 1; }
[[ -f "$SHELL_HTML" ]] || { echo "inject-shell: no $SHELL_HTML" >&2; exit 1; }

# Guard: the bundle must be the shell-less variant (exposes LetGoHost, has a
# </body> to inject into). Catches an accidental `-w-shell xterm` build.
grep -q "window.LetGoHost" "$INDEX" || {
  echo "inject-shell: no window.LetGoHost in $INDEX — build with 'lg -w -w-shell none' first" >&2
  exit 1
}
grep -q "</body>" "$INDEX" || { echo "inject-shell: no </body> in $INDEX" >&2; exit 1; }

# Strip any previously-injected block (sentinels inclusive) so a re-run
# replaces rather than stacks.
if grep -qF "$START" "$INDEX"; then
  awk -v s="$START" -v e="$END" '
    index($0, s) { skip = 1 }
    !skip { print }
    index($0, e) { skip = 0 }
  ' "$INDEX" > "$INDEX.tmp" && mv "$INDEX.tmp" "$INDEX"
fi

# Inject the current shell (wrapped in sentinels) before </body>.
awk -v start="$START" -v end="$END" -v shellfile="$SHELL_HTML" '
  /<\/body>/ && !done {
    print start
    while ((getline line < shellfile) > 0) print line
    print end
    done = 1
  }
  { print }
' "$INDEX" > "$INDEX.tmp" && mv "$INDEX.tmp" "$INDEX"

echo "inject-shell: injected $SHELL_HTML into $INDEX"
