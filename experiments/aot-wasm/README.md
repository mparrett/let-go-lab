# aot-wasm — lowered mandelbrot in the browser

Can the browser page run mandelbrot's hot loop as compiled Go instead of on the
let-go VM? `lg compile` already lowers the demo to Go for the native build
([native builds](../../docs/native-builds.md)); this links that same lowered
package into an `lg -w` bundle.

It works, but only with a patch to the `main.go` that `lg -w` generates (below).
The patch is the gap to close upstream.

## Result

2026-11-14, let-go `c0a93f0`, TinyGo 0.44 `-opt=2`, headless Chromium on an
Apple M2. Machine load was high (12–18), so the ratios are more trustworthy than
the absolute times. Both arms are the canvas page built through the same patched
entry; the only difference is the `-import` of the lowered package.

| Demo version | VM (control) | lowered Go | |
|---|---:|---:|---:|
| main at `0112cd5`/`c30537b` | 262.5 ms | 14.5 ms | ~18x |
| with the cardioid/bulb test (#59) | 109–111 ms | 14.5–15 ms | ~7.5x |

Median per-frame `compute` from the page's status, over 10 re-renders of the
home view (`bench.py`). The canvas pixels hash the same in both arms
(`pixels.py`). The lowered bundle is about 0.2 MB larger (2.9 vs 2.7 MB).

Once lowered, the cardioid test no longer moves the number: the escape loop
stops being the cost, and the remaining ~15 ms is elsewhere in the frame.

## The gap: `lg -w` never installs the overrides

`lg compile -work` emits a Go package whose `init()` calls
`rt.RegisterGoOverrides("mandelbrot", …)`. `lg -w -w-module <dir> -import <pkg>`
(let-go #1016, #1037) links it into the wasm build, and the registration runs.
But the overrides queue until `rt.ApplyGoOverrides` drains them, and the
generated wasm `main.go` replays the program chunks with its own loop and never
calls it. Worse, a single-file program's chunk ends in the top-level `(-main)`
call, so the render loop starts during replay, before anything could be
installed. The page runs bytecode and is no faster.

The native `lg compile` entry frame doesn't have this problem: it replays
through `rt.LoadProgramNamespaces` (which drains the overrides), and the
compiler drops the top-level entry call so the frame invokes `-main` itself.
`patch_main.py` gives the wasm `main.go` that order: replay with
`*compiling-aot*` true so `(when-not *compiling-aot* (-main))` is skipped,
`LoadProgramNamespaces`, `RunProgramMainChunk`, then `InvokeProgramEntry`
for `mandelbrot/-main`.

The generated wasm `main.go` also prints its decode and namespace-load errors
with `"%%v"`, so those messages show a literal `%v` instead of the error.

## Running it

```sh
experiments/aot-wasm/build.sh
python3 scripts/serve.py --dir dist/aot-wasm/ctl --port 8365 --headers harness/serve.json &
python3 scripts/serve.py --dir dist/aot-wasm/aot --port 8366 --headers harness/serve.json &
python3 experiments/aot-wasm/bench.py http://localhost:8365/ 10
python3 experiments/aot-wasm/bench.py http://localhost:8366/ 10
python3 experiments/aot-wasm/pixels.py http://localhost:8365/ http://localhost:8366/
```

`build.sh` needs a built let-go at `./let-go` (or `LETGO=`) and `tinygo` on
`PATH`. It writes `dist/aot-wasm/{ctl,aot}/index.html`, putting a `tinygo` shim
first on `PATH` that patches `lgprogram/main.go` before each `tinygo build` and
refuses to build if the patch didn't apply. The pages need cross-origin
isolation, which `serve.py` adds from `harness/serve.json`. The Python scripts
need Playwright.
