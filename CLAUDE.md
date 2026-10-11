# CLAUDE.md — let-go-lab

A lab for small experiments built on **let-go** (a Clojure dialect on a Go
bytecode VM): sixel graphics, terminal UI, wasm-in-the-browser. If a demo needs
only the language — not any particular application — it lives here, kept separate
from the let-go runtime repo so experiments don't churn it.

## Layout

```
demos/<name>/        one experiment each: <name>.lg + shell.html (its browser shell);
                       mandelbrot-sixel/ reuses mandelbrot.lg (symlink) with an xterm shell
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

The browser and VM recipes run an existing lg. Point them at one of:
- `ln -s /path/to/let-go let-go` (the default `LETGO=./let-go`), or
- `LETGO=/path/to/let-go just serve …` to override per-invocation.

The optional `just native <demo>` recipe builds and caches a separate upstream
checkout pinned by `config/let-go-native.sha`. It does not change `LETGO` or the
browser/Pages release pin. Set `LETGO_NATIVE=/path/to/checkout` to reuse a checkout
of that exact SHA. Native builds and parity tests have a separate CI job; see
[docs/native-builds.md](docs/native-builds.md) when updating the native pin.

**The supported let-go is the release in `config/let-go-release`.** Browser CI
and Pages build exactly that tag, and it is the only version the demos are
tested against — there is no separate, older floor. Docs point at the file
rather than naming a version, so a bump is a one-line change; CI takes the Go
toolchain from that release's `go.mod`. Keep it at a recent release: holding CI
at the oldest supported version is what let nooga/let-go#378 (wasm host mount
`#terminal` → `#app`, v1.12.0) break both browser demos for two minor versions
with CI green. A newer `main` usually works too. The build path feature-probes
for `-w-shell`, failing clearly if it's absent.

The canvas pages (mandelbrot, pathtrace, ptcanvas) draw with `surface/present`
(#572, in releases since v1.13.0). In the browser a program picks canvas or
terminal output with `surface/available?`, which is true only once the page
attaches a canvas sink (`LetGoHost.onSurface`). That is how
`demos/mandelbrot-sixel`, a symlink to the same `mandelbrot.lg` with the xterm
shell, draws sixel: it never attaches one.

`lg` itself is resolved by `scripts/lib/lg-path.sh`, which prefers `$LETGO/bin/lg`
(upstream's promoted copy) over a leftover `$LETGO/lg` at the checkout root.

## Recipes (`just`)

- `just play [demo]` — native TUI (needs a sixel-capable terminal)
- `just native [demo]` — compile mandelbrot, pathtrace, or microgpt to `dist/native/`
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

## Deploy (GitHub Pages)

`.github/workflows/pages.yml` builds every browser demo into its own directory
(`/mandelbrot/`, `/pathtrace/`, `/ptcanvas/`, plus the pretrained microgpt sampler at
`/microgpt/`) under an index page, and publishes them to
https://matt.parrett.us/let-go-lab/ on push to `main`. `scripts/build-pages.sh`
assembles `_site/`: the index (`site/index.html` and its screenshots in
`site/img/`), and per demo an offline single-file bundle + its own
`coi-serviceworker.js`. Adding a browser demo means adding it to `DEMOS` there and
in `test/pages_site_test.py`, and giving it a tile in `site/index.html`.
`test/pages_site_test.py` gates the deploy. Pages can't send COOP/COEP, which the `SharedArrayBuffer` input
ring needs, so `harness/coi-serviceworker.js` re-adds them and a `<head>`
bootstrap (`harness/coi-bootstrap.html`, injected by `inject-coi.sh`) registers
it and reloads once into the isolated context. Build locally with
`scripts/build-pages.sh` and serve `_site/` over plain HTTP to exercise the SW
path (a headered origin like `serve.py` is already isolated, so the SW no-ops).

`scripts/build-microgpt-pages.sh` also builds the sampler alone. Its custom
shell uses `-w-host-eval` to request eight-name batches from the same loaded
image. CI's `test/microgpt_pages_test.py` serves it without isolation headers
under `/let-go-lab/microgpt/` and checks both desktop and mobile. Draft PRs run
these checks; Pages still deploys only on pushes to `main` or an explicit
workflow dispatch.

First deploy: enable Pages with Source = GitHub Actions (the workflow's
`configure-pages` step auto-enables it when the token allows).
