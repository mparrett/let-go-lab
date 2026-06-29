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

The compute kernel lowers to a native `float64` loop: ~56× as actual AOT output,
~79× as hand-written Go (the ceiling). The encoder can't lower today, but the
hand-written native port shows the boxing-heavy half is the bigger prize — ~83×
sits unrealized there. Two upstream findings came out of the spike:
[nooga/let-go#357](https://github.com/nooga/let-go/issues/357) and
[#358](https://github.com/nooga/let-go/issues/358).

## Layout

```
kernel/  mandel_kernel.lg  shaped-to-lower bench kernel (escape-time sum)
         mandel_drive.lg   runs it interpreted, for the baseline
native/  native.go         hand-written Go port of compute + sixel encode
         native_test.go    byte-equivalence check + benchmarks (standalone module)
repro/   repro.lg          8-line repro for the #357 param-typing bug
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

AOT lowering — needs a let-go ≥ 1.11.0 checkout, with `mandel_kernel.lg` copied in:

```sh
./lg scripts/lg-compile out github.com/nooga/let-go/out mandel_kernel.lg
go build ./out/...
```

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
