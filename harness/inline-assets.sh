#!/usr/bin/env bash
# harness/inline-assets.sh — resolve a shell's external asset references into a
# self-contained shell by inlining the committed vendor files (issue #9).
#
# shell.html carries a single marker line, <!--LETGO-LAB-XTERM-ASSETS-->, where
# the xterm CSS/JS used to be CDN <link>/<script> tags. This replaces that marker
# with <style>/<script> blocks holding the vendored file contents, and writes the
# resolved shell to stdout. serve.sh pipes the result into inject-shell.sh, so
# the built index.html needs no network.
#
# Usage: inline-assets.sh <shell.html> [vendor-dir] > resolved-shell.html

set -euo pipefail

SHELL_HTML="${1:?usage: inline-assets.sh <shell.html> [vendor-dir]}"
LAB="$(cd "$(dirname "$0")/.." && pwd)"
VDIR="${2:-$LAB/harness/vendor/xterm@5.5.0}"
MARKER="<!--LETGO-LAB-XTERM-ASSETS-->"

[[ -f "$SHELL_HTML" ]] || { echo "inline-assets: no $SHELL_HTML" >&2; exit 1; }
for f in xterm.min.css xterm.min.js addon-fit.min.js addon-image.min.js; do
  [[ -f "$VDIR/$f" ]] || { echo "inline-assets: missing $VDIR/$f (run scripts/vendor-xterm.sh)" >&2; exit 1; }
done
grep -qF "$MARKER" "$SHELL_HTML" || {
  echo "inline-assets: no $MARKER in $SHELL_HTML" >&2; exit 1; }

awk -v marker="$MARKER" -v vdir="$VDIR" '
  index($0, marker) {
    print "<style>"
    while ((getline l < (vdir "/xterm.min.css")) > 0) print l
    close(vdir "/xterm.min.css")
    print "</style>"
    n = split("xterm.min.js addon-fit.min.js addon-image.min.js", js, " ")
    for (i = 1; i <= n; i++) {
      print "<script>"
      while ((getline l < (vdir "/" js[i])) > 0) print l
      close(vdir "/" js[i])
      print "</script>"
    }
    next
  }
  { print }
' "$SHELL_HTML"
