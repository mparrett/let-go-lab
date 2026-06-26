# let-go-lab

Experiments on [let-go](https://github.com/nooga/let-go) — sixel graphics,
terminal UI, wasm-in-the-browser — decoupled from any one client app.

## Quick start

```sh
just play            # native TUI (needs a sixel-capable terminal)
just serve           # build + serve in the browser; open the printed URL
```

Both default to the `mandelbrot` demo and to `./let-go/lg` (the symlinked
checkout). See [CLAUDE.md](CLAUDE.md) for the lg requirement, cert setup for
LAN/phone serving, and how to add a demo.

## Demos

- **mandelbrot** — an escape-time Mandelbrot rendered as sixel graphics in
  xterm.js (and any sixel-capable native terminal): interactive zoom / pan /
  maxiter, tap-to-recenter-zoom, a dim-while-computing cue, live phase timing,
  and a single-pass sixel encoder.

  Controls: `+/-` zoom · `hjkl` / arrows pan · `,/.` maxiter · `r` reset ·
  `q` quit · tap/click to recenter · wheel to zoom.
