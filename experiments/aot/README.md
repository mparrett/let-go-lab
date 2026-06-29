# AOT-compiling the mandelbrot kernel to native Go

A spike on let-go 1.11.0's `lg-compile` AOT path (`lower-all-ns-to-go`): how fast
the mandelbrot demo's hot path gets when its `.lg` is lowered to native Go
instead of interpreted, and where the lowering stops.

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
[nooga/let-go#357](https://github.com/nooga/let-go/issues/357) and
[#358](https://github.com/nooga/let-go/issues/358).

## Layout

```
kernel/  mandel_kernel.lg   shaped-to-lower bench kernel (escape-time sum)
         mandel_drive.lg    runs it interpreted, for the baseline
         aot_bench_test.go  bench dropped into the lg-compile output package
         regen-aot.sh       lower + bench the kernel against a let-go checkout
native/  native.go          hand-written Go port of compute + sixel encode
         native_test.go     byte-equivalence check + benchmarks (standalone module)
repro/   repro.lg           8-line repro for the #357 param-typing bug
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

AOT lowering + bench — needs a let-go ≥ 1.11.0 checkout with a built `lg`:

```sh
kernel/regen-aot.sh /path/to/let-go   # or set LETGO=, or rely on the ./let-go symlink
```

It lowers `mandel_kernel.lg` with `lg-compile`, drops `aot_bench_test.go` into the
generated package, and benches it — the AOT output lands ~1.3 ms, within noise of
the hand-written native port.

`mandel_kernel.lg` is shaped to lower cleanly — the view is baked as float
literals and the per-pixel position is carried as float accumulators — so no
float param or `(float i)` cast trips the gaps below. The generated escape loop
comes out as native `float64`:

```go
for {
    ...
    zx = arg__7 + px
    zy = arg__9 + im
    k = k + 1
}
```

## What doesn't lower yet

1. **Float params typed as `int`** ([#357](https://github.com/nooga/let-go/issues/357)).
   The natural `(defn escape [cx cy mi] …)` infers `cx`/`cy` as `int`, emitting
   `float64 + int` Go that won't compile. `repro/repro.lg` is the minimal case;
   the kernel here dodges it with float literals and accumulators.
2. **Collections and strings box to `vm.Value`** ([#358](https://github.com/nooga/let-go/issues/358)).
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
- Measured on let-go v1.11.0 (`f9048d8`), one machine — read as ratios, not
  absolutes.
