# Typed arrays under AOT lowering: the sum-arr ladder

The array half of [nooga/let-go#358](https://github.com/nooga/let-go/issues/358):
what `aget`/`aset` over a `double-array` costs today when lowered to Go, and
what a kind-aware lowering would recover. `sum_arr.lg` is the kernel;
`arrays_test.go` holds the three Go shapes it brackets.

## Measured 2026-10-10

let-go `main` at `ff1e6dac`, Go 1.27.1, Apple M2, 1M `double-array`, 50 reps,
three runs. Ratios are the point; absolutes are this machine.

| Variant | ms/rep | vs AOT today |
|---|---|---|
| Bytecode VM (`lg -e "(require 'sum-arr) (sum-arr/-main)"`) | 232-263 | |
| AOT today (`lg compile`: `rt.CoreAgetf`, `rt.AddValue`) | 66-67 | 1x |
| Go, `arr.Get(i)` direct, still boxed (`BenchmarkGetBoxed`) | 18-23 | ~3x |
| Go, kind asserted once, slice hoisted (`BenchmarkHoisted`) | 1.6-2.3 | ~30-40x |
| Hand-written Go over `[]float64` (`BenchmarkPlain`) | 1.4-1.8 | ~40x |

Reproduce the Go rows with `go test -bench=. -benchtime=2s` here; the module
pins the let-go release in `go.mod` and needs no checkout. The first two rows
need an `lg` built from the commit above: run the `.lg` as shown, and
`lg compile -work <dir> sum_arr.lg` keeps the generated Go in `<dir>`.

## What the lowered Go looks like today

```go
func SumArr(ec *vm.ExecContext, arg0 vm.Value, arg1 int64) (vm.Value, error) {
	var s vm.Value
	...
	arg__1, callErr = rt.CoreAgetf(arg0, vm.Int(i))
	...
	s = rt.AddValue(s, arg__1)
```

The array is `vm.Value`, the index boxes per access, the element comes back
boxed, and so the accumulator and its add fall back to runtime calls. A
`^doubles a` hint is rejected with a warning. The hoisted shape in
`arrays_test.go` is what the lowering could emit instead, over the same
`*vm.TypedArray`: its kind check and slice assertion are loop-invariant, and
`TypedArray.Get` re-runs both per element.
