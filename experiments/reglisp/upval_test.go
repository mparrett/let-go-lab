package main

import "testing"

// discriminating program:
//   - n is a live local of `test`'s frame
//   - g captures n as an OPEN upvalue (test hasn't returned)
//   - grow() forces vm.stack to reallocate WHILE n's upvalue is open
//   - g writes 42 THROUGH the upvalue; then we read n DIRECTLY as a register
//
// fixup OK  -> upvalue points at the new backing array -> both see slot -> 42
// fixup BAD -> g writes the stale old array, direct read sees new array -> 0
const upvalProbe = `
(define (grow n)
  (if (< n 1) 0 (+ n (grow (- n 1)))))
(define (test)
  (let ((n 0))
    (let ((g (lambda (v) (set! n v))))
      (grow 100000)
      (g 42)
      n)))
(test)
`

func runSrc(t *testing.T, src string) Value {
	t.Helper()
	vm := NewVM()
	forms, err := Read(src)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	proto, err := Compile(forms)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return vm.Call(&Closure{proto: proto}, nil)
}

func TestOpenUpvalueSurvivesStackRegrow(t *testing.T) {
	// sanity: confirm the program actually triggers a regrow. Initial stack is
	// 1<<12 = 4096 slots; grow(100000) drives calls far past that.
	got := runSrc(t, upvalProbe)
	if got.Type != TNum {
		t.Fatalf("test returned non-number %v", got)
	}
	if got.Num != 42 {
		t.Fatalf("open upvalue read the WRONG cell after regrow: got %v, want 42 "+
			"(0 => ensureStack failed to re-point the open upvalue at the new backing array)", got.Num)
	}
	t.Logf("write-through-upvalue then direct-read agree (42) across a forced stack regrow -- fixup holds")
}
