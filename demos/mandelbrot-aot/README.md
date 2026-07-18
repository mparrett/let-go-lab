# mandelbrot-aot — a let-go kernel lowered to native Go

Companion to the sixel [`mandelbrot`](../mandelbrot) demo, but from the other
direction: instead of running the fractal on the **bytecode VM**, this lowers the
hot `escape` kernel from `.lg` to **native Go** (`lg-compile` → typed Go → `go
build`) and runs it as a compiled binary. Same math, ~**72×** faster, bit-for-bit
identical output.

It's a worked example of the AOT-native path — what works today, and where it's
still hand-wired (see nooga/let-go#425).

![mandelbrot-aot: the AOT-lowered kernel diving uncapped (delay=0), ending on the frames/sec summary](../../docs/img/mandelbrot-aot-zoom.gif)

*Uncapped (`zoom … 0`): the AOT-lowered `escape` runs as fast as the terminal will take it — ~2ms compute/frame. The on-screen fps is bounded by this headless capture's terminal; on a real terminal the same run clears ~470 fps. The bytecode VM does the identical kernel at ~48ms/frame (`zoom-vm.lg`).*

## What it shows

- **`kernel.lg`** — the escape-time kernel with `^double` param hints
  (`(defn escape [^double cx ^double cy mi] …)`) plus the grid coords
  `(* 0.03125 col)` (int × float). The hints are load-bearing: they're why the
  params lower to native `float64` instead of `int`.
- **`gen/aot/kernel/kernel.go`** (generated) — the lowered result:
  `func Escape(ec *vm.ExecContext, cx float64, cy float64, mi int) int` — a raw
  `for` loop on unboxed `float64`, no VM dispatch, no boxing. `CxOf(col int)
  float64` widens the int column (nooga/let-go#534).
- **`native/main.go`** — the Go driver that calls the lowered funcs directly (no
  VM boot needed). It renders truecolor ANSI cells and runs a live, keyboard-driven
  fractal (`interactive` mode) — the same home view and key bindings as the VM
  [`mandelbrot`](../mandelbrot) demo, but on the native kernel, so panning and
  zooming are instant (~1–3 ms/frame vs the VM's ~48 ms).
- **`vm.lg` / `zoom-vm.lg`** — the same workload on the bytecode VM, for the
  side-by-side comparison.

## Build & run

Needs a **let-go ≥ 1.12** checkout (the `^double` AOT param hints, #357/#534).
`build.sh` defaults to the repo's `../../let-go` symlink; override with `LG=`.

```sh
./build.sh                      # or: LG=/path/to/let-go-1.12 ./build.sh
./mandel-native interactive     # live, keyboard-driven fractal — pan/zoom it yourself
./mandel-native zoom 240 0 0    # scripted uncapped zoom (delay=0) — feel the speed
./mandel-native ascii           # static plain-ASCII fractal
./mandel-native bench           # native-vs-VM timing (checksums must match)
```

`interactive` (needs a real TTY) drives the native kernel live. Keys, matching the
VM demo:

| Key | Action |
|---|---|
| `+` / `-` | zoom in / out |
| `h j k l` or arrows | pan left / down / up / right |
| `,` / `.` | fewer / more iterations (detail vs speed) |
| `r` | reset to the home view |
| `q` or Ctrl-C | quit |

`zoom` params: `zoom [frames] [startFrame] [delayMs] [stepPct]`. The status line
prints `frame / span / mi / compute / delay`; an fps summary goes to stderr at
exit. Compare against the VM:

```sh
../../let-go/lg zoom-vm.lg       # same zoom, interpreted (~48ms/frame vs ~2ms native)
```

## Two things this demo taught (worth knowing)

1. **The speed is in the typed args.** `escape` lowers to unboxed `float64` and
   loses the VM's per-op dispatch + boxing — that's the ~72×. Unhinted, the same
   kernel lowers to `int` params and silently truncates (the open half of #357,
   tracked as float-param inference).
2. **Rapid-redraw ANSI tears without synchronized output.** The color zoom is
   ~88KB/frame; without DEC private mode 2026 (begin/end synchronized update) the
   terminal composites half-drawn frames — a "copy-paste" glitch that self-heals.
   Wrapping each frame in `\e[?2026h … \e[?2026l` fixes it (same technique as
   xsofy's render pipeline). Plain ASCII (~9KB/frame) doesn't tear.

## Status / caveats

- **Hand-wired, by design.** `build.sh` `cd`s into the let-go checkout so
  `lg-compile` can resolve gogen's classpath source — gogen isn't self-contained
  yet (nooga/let-go#425 Item 2). A turnkey `lg build` would remove that.
- **`gen/` and `mandel-native` are build artifacts** (gitignored); run `build.sh`
  to regenerate.
