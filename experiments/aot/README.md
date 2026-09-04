# AOT-compiling the mandelbrot kernel to native Go

A spike on let-go's `lg-compile` AOT path (`lower-all-ns-to-go`): how fast the
mandelbrot demo's hot path gets when its `.lg` is lowered to native Go instead of
interpreted, and where the lowering stops. Originally cut against let-go 1.11.0;
re-measured on current tip 2026-09-04 (see Caveats).

For the polished, interactive version of this — the escape kernel lowered with
`^double` param hints, a live native-vs-VM zoom, and a GIF — see the
[`mandelbrot-aot`](../../demos/mandelbrot-aot) demo. This directory is the
underlying spike: the shaped micro-kernel, the standalone native port, and the
measurement harness.

## Result

Same workload (160×120 grid, maxiter 96, the demo's home view), three ways.
Measured 2026-09-04 on let-go tip `bdd8268c` (`v1.12.2-95`), one machine
(Apple M2) — read as ratios, not absolutes.

| Phase | VM | AOT-lowered | Native Go | VM→AOT | VM→native |
|---|---|---|---|---|---|
| Compute (escape-time) | 115 ms | 10 ms | 1.348 ms | 11.5× | 85× |
| Encode (sixel) | 151 ms | 57–69 ms | 1.635 ms | 2.2–2.6× | 92× |
| Frame | 266 ms | 79 ms | 3.038 ms | 3.4× | 88× |

AOT figures are the whole demo built through `lg-compile --entry-frame`; frame
is the sum of its two phases. **The compute column needs `^double` hints** — the
demo's own unhinted `escape` lowers to `int` params, and the call site's type
guard then falls back to the VM, so compute stays at 115 ms with no diagnostic
([#551](https://github.com/nooga/let-go/issues/551)).

For pure arithmetic the lowering is near-optimal: the AOT micro-kernel benches
at 1.343 ms against the hand-written port's 1.348 ms. The encoder is the
opposite story — it *does* lower now, but to boxed `vm.Value` Go, which buys
2.2–2.6×. The hand-written port shows what the same code is worth unboxed, so
**35–42× remains unrealized** behind
[#358](https://github.com/nooga/let-go/issues/358) (native homogeneous
collections), still open. The spike's other finding,
[#357](https://github.com/nooga/let-go/issues/357) (float params typed `int`),
is **fixed** for hinted params; the inference half is #551 above.

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

## Where the lowering stops

The spike surfaced two gaps. The first — **float params typed as `int`**
([#357](https://github.com/nooga/let-go/issues/357)), where `(defn escape [cx cy mi] …)`
inferred `cx`/`cy` as `int` and emitted non-compiling `float64 + int` Go — is now
**fixed upstream**: `^double` param hints lower to native `float64` (see the
`mandelbrot-aot` demo). The second still stands:

- **Collections and strings box to `vm.Value`** ([#358](https://github.com/nooga/let-go/issues/358)).
  The sixel encoder is `transient`/`assoc!`/`nth`/`str`/`subs` over vectors and
  strings, none of which have a native Go type in the lowering. The encoder does
  lower — but into Go that keeps every element access on a boxed `ec.Invoke`, so
  the inner loop still pays VM dispatch. The `native/` port is what a
  collection-lowering would emit; the 35–42× between AOT and that port is the
  measured prize.

## Caveats

- AOT targets native Go, so this speeds the **native** demo, not the
  browser/WASM build (which runs on the VM).
- The port's iteration sum (450174) differs from the demo's (450584) by ~0.09%:
  the demo's coordinate setup uses Clojure-style single-precision `(float …)`
  while the port is float64, flipping a handful of escape-boundary pixels. Output
  stays byte-equivalent (27794 vs 27695).
- Numbers above are 2026-09-04 on tip `bdd8268c`. Earlier revisions of this
  table carried a v1.11.0 run (`f9048d8`, compute ~97 ms / encode ~120 ms /
  native encode ~1.4 ms). Those are not reconciled with today's and should not
  be mixed with them: the AOT and native ratios shift materially depending on
  which baseline you pair with which port measurement.
- Building the full demo through `--entry-frame` currently needs three `math/*`
  sites hand-unboxed to compile at all
  ([#562](https://github.com/nooga/let-go/issues/562)), and the resulting binary
  runs `-main` twice — once interpreted, once natively. The AOT column above is
  the native pass.
