# CLAUDE.md — let-go-lab

A lab for small experiments built on **let-go** (a Clojure dialect on a Go
bytecode VM): sixel graphics, terminal UI, wasm-in-the-browser. If a demo needs
only the language — not any particular application — it lives here, kept separate
from the let-go runtime repo so experiments don't churn it.

## Layout

```
demos/<name>/        one experiment each: <name>.lg + shell.html (its browser shell)
harness/             reusable browser-shell-for-lg backbone:
                       inject-shell.sh  — bake a client shell into a `lg -w` bundle
                       inline-assets.sh — inline vendored xterm into the shell (offline)
                       vendor/          — pinned xterm assets (committed; see its README)
                       serve.json       — COOP/COEP headers (cross-origin isolation)
scripts/             serve.sh (browser, HTTPS/LAN) · serve.py (stdlib static server)
                       play.sh (native TUI) · vendor-xterm.sh (re-fetch pins)
let-go/              symlink to your lg checkout (gitignored; create it or set LETGO)
dist/                built bundles (gitignored)
certs/               local TLS certs (gitignored; generate your own — see below)
```

## How it gets `lg`

This repo never builds lg — it only runs it. Point it at one of:
- `ln -s /path/to/let-go let-go` (the default `LETGO=./let-go`), or
- `LETGO=/path/to/let-go just serve …` to override per-invocation.

The demos need an lg carrying the client-owned shell (the `-w-shell` flag, #245)
and SGR mouse input (#313) — both are on let-go's `main`, so a stock build works.

## Recipes (`just`)

- `just play [demo]` — native TUI (needs a sixel-capable terminal)
- `just serve [demo] [--port N] [--http]` — browser; HTTPS/LAN if a cert is found
- `just reserve [demo]` — re-inject shell + serve, no WASM rebuild (fast shell loop)
- `just env` — lg version + which let-go this points at
- `just clean` — remove built bundles

## Serving needs cross-origin isolation

let-go's wasm input ring (`read-key`) is `SharedArrayBuffer`-backed, so the page
must be a secure context with COOP/COEP headers. `scripts/serve.py` (stdlib, no
npm) serves the bundle and applies the headers from `harness/serve.json`.
`localhost` over HTTP IS a secure context (works); a LAN IP is NOT — use HTTPS
for a phone. Generate your own certs (this repo ships none):

- **Tailscale (no phone-side trust needed):** `tailscale cert <host>.<tailnet>.ts.net`,
  then place `cert.pem` / `key.pem` in `${XDG_DATA_HOME:-~/.local/share}/let-go-lab/le/`.
- **Local:** `mkcert localhost <lan-ip>` → drop `cert.pem` / `key.pem` in `certs/`.

`serve.sh` searches `…/let-go-lab/le/`, `…/let-go-lab/`, then `certs/`; with no
cert it falls back to localhost HTTP.

## Adding a demo

1. `demos/<name>/<name>.lg` — the program (let-go stdlib: `term`, `math`, …).
2. `demos/<name>/shell.html` — copy mandelbrot's and adjust `IMAGE_W` /
   `IMAGE_H` / `CHROME_ROWS` to your image size, or write a minimal shell that
   binds `window.LetGoHost`.
3. `just serve <name>` (browser) or `just play <name>` (native).
