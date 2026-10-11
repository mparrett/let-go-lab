// Three Go shapes of the sum-arr kernel in ../arrays/sum_arr.lg, over the
// runtime's real *vm.TypedArray. They bracket what an array-aware AOT
// lowering would emit (nooga/let-go#358, array half):
//
//	sumGetBoxed  what reaching TypedArray.Get directly buys: no var lookup,
//	             no trampoline, but the kind switch, slice assert and boxing
//	             still run per element.
//	sumHoisted   the kind asserted once, the slice hoisted once, a[i] in the
//	             loop; the param is still a vm.Value, so this is the shape a
//	             guarded lowering can emit.
//	sumPlain     hand-written Go over []float64, the floor.
package arrays

import (
	"fmt"
	"testing"

	"github.com/nooga/let-go/pkg/vm"
)

const n = 1_000_000

func sumPlain(a []float64, n int64) float64 {
	s := 0.0
	for i := int64(0); i < n; i++ {
		s += a[i]
	}
	return s
}

func sumHoisted(arg0 vm.Value, arg1 int64) (vm.Value, error) {
	arr, ok := arg0.(*vm.TypedArray)
	if !ok || arr.Kind() != vm.ArrayFloat {
		return nil, fmt.Errorf("not a double-array: %v", arg0)
	}
	a := arr.Unbox().([]float64)
	s := 0.0
	for i := int64(0); i < arg1; i++ {
		s += a[i]
	}
	return vm.Float(s), nil
}

func sumGetBoxed(arg0 vm.Value, arg1 int64) (vm.Value, error) {
	arr, ok := arg0.(*vm.TypedArray)
	if !ok {
		return nil, fmt.Errorf("not an array: %v", arg0)
	}
	var s vm.Value = vm.Float(0)
	for i := int64(0); i < arg1; i++ {
		s = vm.Float(float64(s.(vm.Float)) + float64(arr.Get(int(i)).(vm.Float)))
	}
	return s, nil
}

func filled() *vm.TypedArray {
	arr := vm.NewFloatArray(n)
	for i := 0; i < n; i++ {
		if err := arr.Set(i, vm.Float(0.5*float64(i))); err != nil {
			panic(err)
		}
	}
	return arr
}

// One rep; the .lg kernel prints 50 reps summed, 12499987500000.0.
const want = 249999750000.0

func TestShapesAgree(t *testing.T) {
	arr := filled()
	for name, f := range map[string]func(vm.Value, int64) (vm.Value, error){
		"hoisted": sumHoisted, "getBoxed": sumGetBoxed,
	} {
		v, err := f(arr, n)
		if err != nil {
			t.Fatal(name, err)
		}
		if got := float64(v.(vm.Float)); got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	if got := sumPlain(arr.Unbox().([]float64), n); got != want {
		t.Errorf("plain = %v, want %v", got, want)
	}
	if _, err := sumHoisted(vm.Int(3), 1); err == nil {
		t.Error("hoisted accepted a non-array; the guard must refuse so the lowering can trampoline")
	}
}

func BenchmarkPlain(b *testing.B) {
	a := filled().Unbox().([]float64)
	for b.Loop() {
		sumPlain(a, n)
	}
}

func BenchmarkHoisted(b *testing.B) {
	arr := filled()
	for b.Loop() {
		if _, err := sumHoisted(arr, n); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetBoxed(b *testing.B) {
	arr := filled()
	for b.Loop() {
		if _, err := sumGetBoxed(arr, n); err != nil {
			b.Fatal(err)
		}
	}
}
