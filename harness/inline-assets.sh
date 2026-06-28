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
# Vendor dir: default to the lone committed harness/vendor/xterm@* (glob, so a
# version bump in vendor-xterm.sh needs no edit here). Error on 0 or >1 rather
# than silently inlining a stale version.
if [[ -n "${2:-}" ]]; then
  VDIR="$2"
else
  shopt -s nullglob; xdirs=("$LAB"/harness/vendor/xterm@*); shopt -u nullglob
  (( ${#xdirs[@]} == 1 )) || { echo "inline-assets: expected exactly one harness/vendor/xterm@* dir, found ${#xdirs[@]} (run scripts/vendor-xterm.sh)" >&2; exit 1; }
  VDIR="${xdirs[0]}"
fi
MARKER="<!--LETGO-LAB-XTERM-ASSETS-->"

[[ -f "$SHELL_HTML" ]] || { echo "inline-assets: no $SHELL_HTML" >&2; exit 1; }
for f in xterm.min.css xterm.min.js addon-fit.min.js addon-image.min.js \
         xterm.LICENSE.txt addon-fit.LICENSE.txt addon-image.LICENSE.txt addon-image.js.LICENSE.txt; do
  [[ -f "$VDIR/$f" ]] || { echo "inline-assets: missing $VDIR/$f (run scripts/vendor-xterm.sh)" >&2; exit 1; }
done
grep -qF "$MARKER" "$SHELL_HTML" || {
  echo "inline-assets: no $MARKER in $SHELL_HTML" >&2; exit 1; }

awk -v marker="$MARKER" -v vdir="$VDIR" '
  index($0, marker) {
    # Carry the vendored license notices into the self-contained bundle so
    # redistributing index.html also redistributes them (#9 review). MIT texts
    # contain no "--", so an HTML comment is safe.
    print "<!-- Bundled third-party licenses (xterm.js + addons, MIT):"
    nl = split("xterm.LICENSE.txt addon-fit.LICENSE.txt addon-image.LICENSE.txt addon-image.js.LICENSE.txt", lic, " ")
    for (i = 1; i <= nl; i++) {
      print ""; print "=== " lic[i] " ==="
      while ((getline l < (vdir "/" lic[i])) > 0) print l
      close(vdir "/" lic[i])
    }
    print "-->"
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
