#!/usr/bin/env bash
# harness/inject-coi.sh — inject the COI bootstrap (harness/coi-bootstrap.html)
# into a bundle's <head>, just before </head>, so it runs before the wasm boot.
#
# Needed only for header-less hosting (GitHub Pages): the bootstrap registers
# coi-serviceworker.js, which re-adds COOP/COEP so the page is cross-origin
# isolated (SharedArrayBuffer / read-key). The caller must also place
# coi-serviceworker.js next to index.html. See scripts/build-pages.sh.
#
# Transactional + idempotent, mirroring inject-shell.sh: sentinel-wrapped, built
# in a temp, validated, atomically moved, original mode preserved.
#
# Usage: inject-coi.sh <index.html> <coi-bootstrap.html>

set -euo pipefail

INDEX="${1:?usage: inject-coi.sh <index.html> <coi-bootstrap.html>}"
BOOT="${2:?usage: inject-coi.sh <index.html> <coi-bootstrap.html>}"

START="<!--LETGO-LAB-COI-START-->"
END="<!--LETGO-LAB-COI-END-->"

[[ -f "$INDEX" ]] || { echo "inject-coi: no $INDEX" >&2; exit 1; }
[[ -f "$BOOT" ]]  || { echo "inject-coi: no $BOOT" >&2; exit 1; }
grep -q "</head>" "$INDEX" || { echo "inject-coi: no </head> in $INDEX" >&2; exit 1; }

# Accept only an un-injected file (0/0) or a single matched pair (1/1).
sc=$(grep -cF "$START" "$INDEX" || true)
ec=$(grep -cF "$END" "$INDEX" || true)
if ! { [[ "$sc" -eq 0 && "$ec" -eq 0 ]] || [[ "$sc" -eq 1 && "$ec" -eq 1 ]]; }; then
  echo "inject-coi: malformed sentinels in $INDEX (start=$sc end=$ec); refusing" >&2
  exit 1
fi

strip_tmp="$(mktemp "${INDEX}.cstrip.XXXXXX")"
out_tmp="$(mktemp "${INDEX}.cout.XXXXXX")"
trap 'rm -f "$strip_tmp" "$out_tmp"' EXIT

if [[ "$sc" -eq 1 ]]; then
  awk -v s="$START" -v e="$END" '
    index($0, s) { skip = 1 }
    !skip { print }
    index($0, e) { skip = 0 }
  ' "$INDEX" > "$strip_tmp"
else
  cp "$INDEX" "$strip_tmp"
fi

# Insert the bootstrap before the first </head>.
awk -v start="$START" -v end="$END" -v bootfile="$BOOT" '
  /<\/head>/ && !done {
    print start
    while ((getline line < bootfile) > 0) print line
    print end
    done = 1
  }
  { print }
' "$strip_tmp" > "$out_tmp"

osc=$(grep -cF "$START" "$out_tmp" || true)
oec=$(grep -cF "$END" "$out_tmp" || true)
if [[ "$osc" -ne 1 || "$oec" -ne 1 ]] || ! grep -q "</head>" "$out_tmp"; then
  echo "inject-coi: built output failed validation (start=$osc end=$oec); $INDEX unchanged" >&2
  exit 1
fi

# GNU stat (-c) first, BSD/macOS (-f) fallback — on Linux `stat -f '%Lp'` is
# --file-system and "succeeds" with garbage, so it must not be tried first.
orig_mode="$(stat -c '%a' "$INDEX" 2>/dev/null || stat -f '%Lp' "$INDEX" 2>/dev/null)"
mv "$out_tmp" "$INDEX"
[[ -n "$orig_mode" ]] && chmod "$orig_mode" "$INDEX"
echo "inject-coi: injected COI bootstrap into $INDEX"
