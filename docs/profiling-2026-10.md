# Profiling the demos, October 2026

Where mandelbrot, pathtrace and ptcanvas spend their time, natively and in the
browser, measured 2026-10-11. One machine, a 4-vCPU Linux box, so read the
ratios rather than the absolute numbers. (The September figures in
[`experiments/aot`](../experiments/aot) came from an Apple M2 and are 2–3×
faster across the board; that gap is the machine, see below.)

## Native VM: no regression since v1.13.0

Each demo's headless bench (`lg demos/<name>/<name>.lg </dev/null`), let-go
v1.13.0 against `main` at `49858bd`, alternated, two rounds each:

| bench | v1.13.0 | main |
|---|---|---|
| mandelbrot, median frame of 20 | compute 285–306 ms, encode 424–435 ms | compute 291–296 ms, encode 415–427 ms |
| pathtrace, 32 passes | 62–70 s | 63–65 s |
| ptcanvas, scalar tracer | 13.5 s | 14.2 s |
| ptcanvas, vec3 tracer | 46.0 s | 44.3 s |

The two versions are within noise of each other on every bench.

## Native VM: where the time goes

CPU profiles of the same benches with an `lg` built by `make build-profile`
(`lg-profile -cpuprofile out.prof demos/<name>/<name>.lg </dev/null`). The
three profiles look alike:

- **Bytecode dispatch** (`vm.(*Frame).runLoopInner`) is about 27% flat.
- **Boxing and GC**: `runtime.convT64` (a number boxed into an interface) is
  9–13%, `runtime.mallocgc` 11–15%, plus GC scanning.
- **Calls**: native function calls (`vm.(*NativeFn).Invoke`) are about 22%
  cumulative, and calls from native code back into let-go functions
  (`vm.(*ExecContext).Invoke`) about 25%.
- **The frame pool**: every call takes and releases a mutex in
  `acquireFrame`/`releaseFrame` (`pkg/vm/vm.go`), about 2–3% in pathtrace.
  The VM runs one goroutine here, so the lock is never contended; it is the
  bare cost of locking.

Per demo, the let-go functions that stand out:

- **pathtrace**: `nth` (7.5%) and `vector` (5.1%). It builds small `[r g b]`
  vectors per sample, which is what ptcanvas's allocation-free port avoids;
  ptcanvas's own bench puts the scalar tracer at 3.1–3.4× the vector one.
- **mandelbrot**: `nth` (4.4%), `assoc!` (3.4%) and `conj` (2.6%), mostly in
  the sixel encoder. Encoding costs more than computing the fractal
  (about 420 ms against 290 ms a frame).

## Browser: stock Go against TinyGo

Each page built with `lg -w` from `main` `49858bd`, once with the stock Go
toolchain and once with `LETGO_USE_TINYGO=1 LETGO_TINYGO_OPT=2` (TinyGo 0.42.0,
let-go CI's pin). Served by `scripts/serve.py`, one page at a time in headless
Chromium, timings read from the demos' own status lines with
[`scripts/browser-timing.py`](../scripts/browser-timing.py). Medians per frame
or pass:

| | stock Go | TinyGo | speedup | native VM |
|---|---|---|---|---|
| mandelbrot compute | 2.8–3.1 s | 1.3 s | 2.2× | 0.29 s |
| mandelbrot sixel emit | 3.9 s | 2.1–2.2 s | 1.8× | 0.42 s (encode only) |
| pathtrace compute | 15.3 s | 7.1 s | 2.2× | ~2 s |
| pathtrace PNG encode | 2.2 s | 1.4 s | 1.6× | |
| ptcanvas compute | 14.7 s | 7.9 s | 1.9× | |
| ptcanvas canvas present | 0.31 s | 0.17 s | 1.9× | |
| page size | 8.8 MB | 2.8–2.9 MB | | |

- The browser runs the bytecode VM compiled to wasm, so it is 8–10× slower than
  native with stock Go, and about half that with TinyGo. `lg compile` gains
  don't carry over to `lg -w`.
- **Getting pixels on screen**: ptcanvas hands RGBA to a `<canvas>` with
  `surface/present` in 0.17–0.31 s a frame. mandelbrot spends 2–4 s building
  and parsing a sixel string for a smaller image.
- **TinyGo needed a let-go fix.** All three demos time their frames with
  `(inst-ms (now))`, and under TinyGo that failed on the first frame: `inst-ms`
  calls `.UnixMilli` on a Go `time.Time`, and TinyGo only resolves boxed Go
  methods through a hand-written table that lacked it. Fixed by
  [nooga/let-go#1054](https://github.com/nooga/let-go/pull/1054). The TinyGo
  numbers above come from copies of the demos that read the clock with
  `System/currentTimeMillis` instead; mandelbrot unmodified renders the same
  way on that branch.

## Reproduce

```sh
# native benches and profiles (lg-profile: `make build-profile` in a let-go checkout)
(cd demos/mandelbrot && lg mandelbrot.lg </dev/null)
(cd demos/pathtrace  && lg-profile -cpuprofile /tmp/pt.prof pathtrace.lg </dev/null)
go tool pprof -top -nodecount=25 lg-profile /tmp/pt.prof

# browser: build a page, serve it, read its timings
(cd demos/ptcanvas && LETGO_USE_TINYGO=1 LETGO_TINYGO_OPT=2 lg -w ../../dist/ptcanvas-tinygo -w-shell none ptcanvas.lg)
harness/inline-assets.sh demos/ptcanvas/shell.html > /tmp/shell.html
harness/inject-shell.sh dist/ptcanvas-tinygo/index.html /tmp/shell.html
python3 scripts/serve.py --dir dist --port 8256 --host localhost --headers harness/serve.json &
python3 scripts/browser-timing.py http://localhost:8256/ptcanvas-tinygo/ --seconds 120
python3 scripts/browser-timing.py http://localhost:8256/mandelbrot/ --seconds 60 --rekey
```
