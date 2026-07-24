package main

import (
	"fmt"
	"runtime/debug"
	"testing"
)

// sumto is GENUINELY non-tail recursive: (+ n (sumto (- n 1))) keeps frame n
// alive across the recursive call to finish the add on the way back up. No
// tail-call trick can flatten this.
func sumtoProgram(depth int) string {
	return fmt.Sprintf(`
(define (sumto n)
  (if (< n 1)
      0
      (+ n (sumto (- n 1)))))
(sumto %d)
`, depth)
}

func runSumto(depth int) Value {
	vm := NewVM()
	forms, err := Read(sumtoProgram(depth))
	if err != nil {
		panic(err)
	}
	proto, err := Compile(forms)
	if err != nil {
		panic(err)
	}
	return vm.Call(&Closure{proto: proto}, nil)
}

// The claim, in one test: clamp Go's per-goroutine stack to 512KB, then run
// 200k levels of non-tail reglisp recursion. If a user CALL recursed into
// Go's stack this panics with "goroutine stack exceeds". It completes because
// the 200k frames live in vm.calls on the heap, not native frames.
func TestDeepRecursionSurvivesTinyNativeStack(t *testing.T) {
	old := debug.SetMaxStack(512 * 1024)
	defer debug.SetMaxStack(old)

	got := runSumto(200000)
	want := float64(200000) * float64(200001) / 2
	if got.Type != TNum || got.Num != want {
		t.Fatalf("sumto(200000) = %v (type %v); want %v", got.Num, got.Type, want)
	}
	t.Logf("sumto(200000) = %.0f under a 512KB native-stack cap", got.Num)
}

// Headline: one MILLION levels of non-tail recursion on the default stack.
// A native tree-walker would need ~1M live Go frames; this needs one Go frame
// (the Run loop) plus a 1M-entry vm.calls slice on the heap.
func TestOneMillionDeep(t *testing.T) {
	got := runSumto(1000000)
	want := float64(1000000) * float64(1000001) / 2
	if got.Type != TNum || got.Num != want {
		t.Fatalf("sumto(1e6) = %v; want %v", got.Num, want)
	}
	t.Logf("sumto(1000000) = %.0f -- 1e6 non-tail frames, all on the heap", got.Num)
}
