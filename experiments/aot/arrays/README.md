# Typed arrays under AOT lowering: the sum-arr ladder

[nooga/let-go#1055](https://github.com/nooga/let-go/issues/1055), the array
half of [nooga/let-go#358](https://github.com/nooga/let-go/issues/358): what
`aget`/`aset` over a `double-array` costs when lowered to Go, with and without
a kind hint. `sum_arr.lg` is the kernel; `sum_arr_doubles.lg` is the same
kernel with `^doubles a` on both fns. `arrays_test.go` holds the three Go
shapes they bracket.

The kind-aware lowering landed in
[nooga/let-go#1057](https://github.com/nooga/let-go/pull/1057), with
`OP_AGET`/`OP_ASET` for the VM in
[nooga/let-go#1058](https://github.com/nooga/let-go/pull/1058) and qualified
core heads in [nooga/let-go#1060](https://github.com/nooga/let-go/pull/1060),
all merged 2026-10-17.

## Measured 2026-10-18

let-go `main` at `9981aa04`, Go 1.27.1, Apple M2, 1M `double-array`, 50 reps.
Two passes of four interleaved rounds, the first round of each dropped as
cold. Load was 6-9, so absolutes run about 20% above a quiet machine (the
hand-Go row was 1.4-1.8 on 2026-10-10); the ratios held across both passes.
These are indicative runs, not ratchet results.

| Variant | ms/rep | vs unhinted AOT |
|---|---|---|
| Bytecode VM (`lg -e "(require 'sum-arr) (sum-arr/-main)"`) | 125-187 | |
| AOT, no hint (`rt.CoreAgetf`, `rt.AddValue`) | 50-76 | 1x |
| Go, `arr.Get(i)` direct, still boxed (`BenchmarkGetBoxed`) | 19-22 | ~3x |
| Go, kind asserted once, slice hoisted (`BenchmarkHoisted`) | 2.1-2.4 | ~30x |
| Hand-written Go over `[]float64` (`BenchmarkPlain`) | 1.9-2.2 | ~30x |
| AOT, `^doubles` (`sum_arr_doubles.lg`) | 1.8-2.7 | ~25-30x |

The hinted kernel lands on the hand-written Go row. The VM row was 232-263 on
2026-10-10 at `ff1e6dac`, before `OP_AGET`; the hint does not change it.

Reproduce the Go rows with `go test -bench=. -benchtime=2s` here; the module
pins the let-go release in `go.mod` and needs no checkout. v1.13.0 predates
#1057, so the VM and AOT rows need an `lg` built from `main` at or after
`ff9f31a9`: run either `.lg` as shown (`sum-arr-doubles` for the hinted one),
and `lg compile -work <dir> sum_arr_doubles.lg` keeps the generated Go in
`<dir>`.

## What the lowered Go looks like

Without a hint, the array is `vm.Value`, the index boxes per access, the
element comes back boxed, and the accumulator and its add fall back to
runtime calls:

```go
func SumArr(ec *vm.ExecContext, arg0 vm.Value, arg1 int64) (vm.Value, error) {
	var s vm.Value
	...
	arg__1, callErr = rt.CoreAgetf(arg0, vm.Int(i))
	...
	s = rt.AddValue(s, arg__1)
```

With `^doubles a`, the parameter is `*vm.TypedArray`, the element and the
accumulator are `float64`, and the access is an inlinable accessor:

```go
func SumArr(ec *vm.ExecContext, arg0 *vm.TypedArray, arg1 int64) (float64, error) {
	var s float64
	...
	if rt.NativePrimsIntact() {
		arg__1, callErr = arg0.AtFloat64(i)
		...
	} else {
		tb16, callErr = ec.Invoke2(rt.CachedVarFnEC(ec, &__v_clojure_core_aget, "clojure.core", "aget"), arg0, vm.Int(i))
		...
	}
	i = i + 1
	s = s + arg__1
```

The `else` arm is the trampoline: if `aget` has been redefined, the lowered
code calls the var and checks the result's kind, so lowered and bytecode agree
on every input. `AtFloat64` returns an error on a wrong-kind array or a bad
index rather than panicking.
