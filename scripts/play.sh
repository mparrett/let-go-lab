#!/usr/bin/env bash
# scripts/play.sh — run a let-go-lab demo as a native TUI in the current
# terminal. The SAME <name>.lg as the browser build (serve.sh); only the input
# wiring differs (this path enters raw mode, the wasm shell feeds keys through
# xterm). Sixel output is terminal-agnostic.
#
# Sixel demos need a SIXEL-CAPABLE terminal: WezTerm, foot, iTerm2, mlterm,
# recent xterm (-ti vt340), or recent kitty. Without one you get the status text
# but no image (the DCS is silently ignored).
#
# Usage: play.sh [demo]      demo name under demos/ (default: mandelbrot)
#
# LETGO defaults to ./let-go (symlink to your let-go checkout). Override LETGO=<path>.

set -euo pipefail

LAB="$(cd "$(dirname "$0")/.." && pwd)"
LETGO="${LETGO:-$LAB/let-go}"
LG="$LETGO/lg"
DEMO="${1:-mandelbrot}"
LGFILE="$LAB/demos/$DEMO/$DEMO.lg"

[[ -f "$LGFILE" ]] || { echo "play: no demo at $LGFILE" >&2; exit 1; }
[[ -x "$LG" ]]     || { echo "play: no lg at $LG (set LETGO=<path-to-let-go>)" >&2; exit 1; }

exec env LETGO_SRC="$LETGO" "$LG" "$LGFILE"
