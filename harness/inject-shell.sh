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
# Transactional (issue #8): the input is validated and the new file is built in a
# temp, validated again, and only then moved into place. A malformed prior
# injection (e.g. a start sentinel with no end — which the old in-place awk
# silently turned into whole-file truncation) is rejected and the original is
# left byte-for-byte unchanged.
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
# </body> to inject into). Catches an accidental `-w-shell xterm` build. Checked
# before any work, so a bad input is never modified.
grep -q "window.LetGoHost" "$INDEX" || {
  echo "inject-shell: no window.LetGoHost in $INDEX — build with 'lg -w -w-shell none' first" >&2
  exit 1
}
grep -q "</body>" "$INDEX" || { echo "inject-shell: no </body> in $INDEX" >&2; exit 1; }

# A well-formed file has either no prior injection (0 start / 0 end) or exactly
# one matched pair. Anything else — a lone sentinel, a count mismatch, or
# duplicates — means a previous run was interrupted or the file was hand-edited;
# refuse rather than risk truncating (lone start) or stacking (duplicates).
sc=$(grep -cF "$START" "$INDEX" || true)
ec=$(grep -cF "$END" "$INDEX" || true)
if ! { [[ "$sc" -eq 0 && "$ec" -eq 0 ]] || [[ "$sc" -eq 1 && "$ec" -eq 1 ]]; }; then
  echo "inject-shell: malformed sentinels in $INDEX (start=$sc end=$ec); refusing to modify" >&2
  exit 1
fi

# Build in temps next to INDEX (same filesystem → atomic final mv). The trap
# clears them on any exit, so a failure leaves no partial files behind.
strip_tmp="$(mktemp "${INDEX}.strip.XXXXXX")"
out_tmp="$(mktemp "${INDEX}.out.XXXXXX")"
trap 'rm -f "$strip_tmp" "$out_tmp"' EXIT

# Strip any previously-injected block (sentinels inclusive) so a re-run replaces
# rather than stacks. With no prior block this is a passthrough copy.
if [[ "$sc" -eq 1 ]]; then
  awk -v s="$START" -v e="$END" '
    index($0, s) { skip = 1 }
    !skip { print }
    index($0, e) { skip = 0 }
  ' "$INDEX" > "$strip_tmp"
else
  cp "$INDEX" "$strip_tmp"
fi

# Inject the current shell (wrapped in sentinels) before the first </body>.
awk -v start="$START" -v end="$END" -v shellfile="$SHELL_HTML" '
  /<\/body>/ && !done {
    print start
    while ((getline line < shellfile) > 0) print line
    print end
    done = 1
  }
  { print }
' "$strip_tmp" > "$out_tmp"

# Validate the built result before it replaces anything: exactly one matched
# sentinel pair, in order, plus a surviving </body>. Catches an injection that
# silently no-op'd (e.g. a malformed strip that ate the closing body tag).
osc=$(grep -cF "$START" "$out_tmp" || true)
oec=$(grep -cF "$END" "$out_tmp" || true)
sl=$(grep -nF "$START" "$out_tmp" | head -1 | cut -d: -f1)
el=$(grep -nF "$END" "$out_tmp" | head -1 | cut -d: -f1)
if [[ "$osc" -ne 1 || "$oec" -ne 1 ]] || ! grep -q "</body>" "$out_tmp" \
   || [[ -z "$sl" || -z "$el" || "$sl" -ge "$el" ]]; then
  echo "inject-shell: built output failed validation (start=$osc end=$oec); $INDEX unchanged" >&2
  exit 1
fi

# Preserve the original file's mode: mktemp made out_tmp 0600, so a bare mv would
# leave the (typically 0644) generated index.html private — breaking copy/publish
# or serving it under another account. Carry the original mode onto the result.
orig_mode="$(stat -f '%Lp' "$INDEX" 2>/dev/null || stat -c '%a' "$INDEX" 2>/dev/null)"
mv "$out_tmp" "$INDEX"
[[ -n "$orig_mode" ]] && chmod "$orig_mode" "$INDEX"
echo "inject-shell: injected $SHELL_HTML into $INDEX"
