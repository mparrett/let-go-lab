# AOT-compiling the mandelbrot kernel to native Go

A spike on let-go's `lg-compile` AOT path (`lower-all-ns-to-go`): how fast the
mandelbrot demo's hot path gets when its `.lg` is lowered to native Go instead of
interpreted, and where the lowering stops. Originally cut against let-go 1.11.0;
re-validated on current tip (see Caveats).

For the polished, interactive version of this — the escape kernel lowered with
`^double` param hints, a live native-vs-VM zoom, and a GIF — see the
[`mandelbrot-aot`](../../demos/mandelbrot-aot) demo. This directory is the
underlying spike: the shaped micro-kernel, the standalone native port, and the
measurement harness.

## Result

Same workload (160×120 grid, maxiter 96, the demo's home view), three ways:

| Phase | Interpreted (VM) | Native Go | Speedup | Lowers under `lg-compile`? |
|---|---|---|---|---|
| Compute (escape-time) | ~97 ms | 1.2 ms | ~79× | yes |
| Encode (sixel) | ~120 ms | 1.4 ms | ~83× | no — boxes to `vm.Value` |
| Frame | ~217 ms | 2.7 ms | ~81× | partial |

The compute kernel lowers to a native `float64` loop, and the AOT output benches
within noise of the hand-written port (~1.3 ms, ~70× over interpreted) — for pure
arithmetic the lowering is near-optimal. The encoder can't lower today, but the
native port shows the boxing-heavy half is the bigger prize: ~83× sits unrealized
there. Two upstream findings came out of the spike:
[nooga/let-go#357](https://github.com/nooga/let-go/issues/357) (float params typed
`int`) — since **fixed**, so hinted float params now lower — and
[#358](https://github.com/nooga/let-go/issues/358) (native homogeneous
collections), still open, which is what caps the encoder.

## Layout

```
kernel/  mandel_kernel.lg   shaped-to-lower bench kernel (escape-time sum)
         mandel_drive.lg    runs it interpreted, for the baseline
         aot_bench_test.go  bench dropped into the lg-compile output package
         regen-aot.sh       lower + bench the kernel against a let-go checkout
native/  native.go          hand-written Go port of compute + sixel encode
         native_test.go     byte-equivalence check + benchmarks (standalone module)
```

## Reproduce

Native port — standalone, no let-go needed:

```sh
cd native
go test -run=TestFrameMatchesDemo -v   # byte-equivalent to the demo (~27695)
go test -bench=. -benchtime=2s         # the native numbers above
```

Interpreted baseline — needs an `lg` (the repo symlink, or on PATH):

```sh
lg -source-paths kernel kernel/mandel_drive.lg
```

AOT lowering + bench — needs a let-go ≥ 1.12 checkout with a built `lg` (the
bench template calls the exported `MandelBench`; `lg-compile` switched to exported
CamelCase names after 1.11.0):

```sh
kernel/regen-aot.sh /path/to/let-go   # or set LETGO=, or rely on the ./let-go symlink
```

It lowers `mandel_kernel.lg` with `lg-compile`, drops `aot_bench_test.go` into the
generated package, and benches it — the AOT output lands ~1.3 ms, within noise of
the hand-written native port.

`mandel_kernel.lg` is shaped as a params-free micro-kernel — the view is baked as
float literals and the per-pixel position is carried as float accumulators — so it
lowers on any let-go ≥ 1.11.0. (The float-param gap it originally sidestepped,
[#357](https://github.com/nooga/let-go/issues/357), is now fixed upstream, so the
natural `(defn escape [^double cx ^double cy mi] …)` lowers too — that's the
[`mandelbrot-aot`](../../demos/mandelbrot-aot) demo's kernel.) The generated escape
loop comes out as native `float64`:

```go
for {
    ...
    zx = arg__7 + px
    zy = arg__9 + im
    k = k + 1
}
```

## What doesn't lower yet

The spike surfaced two gaps. The first — **float params typed as `int`**
([#357](https://github.com/nooga/let-go/issues/357)), where `(defn escape [cx cy mi] …)`
inferred `cx`/`cy` as `int` and emitted non-compiling `float64 + int` Go — is now
**fixed upstream**: `^double` param hints lower to native `float64` (see the
`mandelbrot-aot` demo). The second still stands:

- **Collections and strings box to `vm.Value`** ([#358](https://github.com/nooga/let-go/issues/358)).
  The sixel encoder is `transient`/`assoc!`/`nth`/`str`/`subs` over vectors and
  strings, none of which have a native Go type in the lowering, so it stays on
  runtime trampolines. The `native/` port is what a collection-lowering would
  emit; the ~83× gap is the measured prize.

## Caveats

- AOT targets native Go, so this speeds the **native** demo, not the
  browser/WASM build (which runs on the VM).
- The port's iteration sum (450174) differs from the demo's (450584) by ~0.09%:
  the demo's coordinate setup uses Clojure-style single-precision `(float …)`
  while the port is float64, flipping a handful of escape-boundary pixels. Output
  stays byte-equivalent (27794 vs 27695).
- Measured on let-go v1.11.0 (`f9048d8`) and re-validated on current tip
  (`f154c7f`): interpreted compute ~93 ms, AOT ~1.27 ms, native compute 1.23 ms —
  within noise of the original. One machine — read as ratios, not absolutes.
