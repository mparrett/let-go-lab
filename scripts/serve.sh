#!/usr/bin/env bash
# scripts/serve.sh — build a let-go-lab demo to WASM and serve it in a browser
# with a client-owned shell (xterm.js). Generic over demos: demos/<name>/ holds
# <name>.lg + shell.html.
#
# Cross-origin isolation: let-go's wasm input ring is SharedArrayBuffer-backed,
# so the page must be a secure context for read-key/term/size to work. HTTPS on
# 0.0.0.0 when a cert is found (LAN/phone reachable); localhost HTTP otherwise
# (the COOP/COEP headers in harness/serve.json still isolate localhost). A LAN IP
# over plain HTTP is NOT a secure context — use HTTPS for the phone.
#
# Usage: serve.sh [demo] [--port N] [--no-build] [--http]
#   demo         demo name under demos/ (default: mandelbrot)
#   --port N     listen port (default 8249)
#   --no-build   re-inject shell + serve, skipping the WASM rebuild (fast loop)
#   --http       force localhost-only HTTP (no cert; LAN unreachable)
#
# LETGO defaults to ./let-go (symlink to your let-go checkout). Override
# LETGO=<path> to point at any lg with #245 (-w-shell) + #313 (SGR mouse).

set -euo pipefail

LAB="$(cd "$(dirname "$0")/.." && pwd)"
LETGO="${LETGO:-$LAB/let-go}"
LG="$LETGO/lg"
DEMO=mandelbrot
PORT=8249
BUILD=1
HTTPS=1

while [[ $# -gt 0 ]]; do
  case "$1" in
    --port)     PORT="${2:?--port needs a value}"; shift 2 ;;
    --no-build) BUILD=0; shift ;;
    --http)     HTTPS=0; shift ;;
    -h|--help)  sed -n '2,/^set -e/p' "$0" | sed 's/^# \{0,1\}//;s/^set -e.*//'; exit 0 ;;
    --*)        echo "serve: unknown flag: $1" >&2; exit 1 ;;
    *)          DEMO="$1"; shift ;;
  esac
done

SRC="$LAB/demos/$DEMO"
DIST="$LAB/dist/$DEMO"
SHELL_HTML="$SRC/shell.html"
LGFILE="$SRC/$DEMO.lg"

[[ -f "$LGFILE" ]]     || { echo "serve: no demo at $LGFILE" >&2; exit 1; }
[[ -f "$SHELL_HTML" ]] || { echo "serve: no shell at $SHELL_HTML" >&2; exit 1; }
[[ -x "$LG" ]]         || { echo "serve: no lg at $LG (set LETGO=<path-to-let-go>)" >&2; exit 1; }
"$LG" -h 2>&1 | grep -q -- '-w-shell' || {
  echo "serve: $LG predates -w-shell (#245); point LETGO at upstream main or later" >&2; exit 1; }

mkdir -p "$DIST"
if [[ "$BUILD" == "1" ]]; then
  echo "==> build: $LG -w -w-shell none $DEMO.lg"
  (cd "$SRC" && LETGO_SRC="$LETGO" "$LG" -w "$DIST" -w-shell none "$DEMO.lg")
fi

# Inline the vendored xterm assets into the shell (offline, self-contained —
# #9), then inject the resolved shell into the bundle. The temp resolved shell
# is cleaned on exit.
RESOLVED_SHELL="$(mktemp "${TMPDIR:-/tmp}/lg-shell-XXXXXX")"   # trailing X's: BSD mktemp needs them last
trap 'rm -f "$RESOLVED_SHELL"' EXIT
"$LAB/harness/inline-assets.sh" "$SHELL_HTML" > "$RESOLVED_SHELL"
"$LAB/harness/inject-shell.sh" "$DIST/index.html" "$RESOLVED_SHELL"
rm -f "$RESOLVED_SHELL"   # consumed (baked into index.html); exec below skips the trap

# Cert discovery (lab-specific; this repo ships none — generate your own, see
# CLAUDE.md). Tailscale LE preferred (no phone-side trust needed).
find_certs() {
  local xdg="${XDG_DATA_HOME:-$HOME/.local/share}/let-go-lab"
  for prefix in "$xdg/le" "$xdg" "$LAB/certs"; do
    if [[ -f "$prefix/cert.pem" && -f "$prefix/key.pem" ]]; then
      CERT="$prefix/cert.pem"; KEY="$prefix/key.pem"; return 0
    fi
  done
  return 1
}

print_urls() {
  local scheme="$1"
  echo "    ${scheme}://localhost:${PORT}/"
  for iface in en0 en1; do
    local ip; ip="$(ipconfig getifaddr "$iface" 2>/dev/null || true)"
    [[ -n "$ip" ]] && echo "    ${scheme}://${ip}:${PORT}/"
  done
  if command -v tailscale >/dev/null && command -v jq >/dev/null; then
    local ts; ts="$(tailscale status --json 2>/dev/null | jq -r '.Self.DNSName // empty' | sed 's/\.$//')"
    [[ -n "$ts" ]] && echo "    ${scheme}://${ts}:${PORT}/   (Tailscale — no phone-side cert trust needed)"
  fi
}

SERVE_PY="$LAB/scripts/serve.py"
HEADERS="$LAB/harness/serve.json"
if [[ "$HTTPS" == "1" ]] && find_certs; then
  echo "==> HTTPS serve on 0.0.0.0:${PORT} (cert: $CERT) — reachable at:"
  print_urls https
  exec python3 "$SERVE_PY" --dir "$DIST" --port "$PORT" --host 0.0.0.0 \
       --cert "$CERT" --key "$KEY" --headers "$HEADERS"
else
  [[ "$HTTPS" == "1" ]] && echo "==> no cert in \${XDG_DATA_HOME:-~/.local/share}/let-go-lab/ or certs/ — localhost HTTP only"
  echo "==> HTTP serve on localhost:${PORT} (LAN unreachable — SharedArrayBuffer needs a secure context):"
  echo "    http://localhost:${PORT}/"
  # Bind localhost-only: 0.0.0.0 would expose insecure HTTP on the LAN — an
  # origin that isn't cross-origin isolated, so the demo can't run there anyway.
  exec python3 "$SERVE_PY" --dir "$DIST" --port "$PORT" --host localhost --headers "$HEADERS"
fi
