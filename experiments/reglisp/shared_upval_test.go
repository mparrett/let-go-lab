package main

import "testing"

func runProg(t *testing.T, src string) Value {
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

// Two closures capturing the SAME slot must share one variable. findOrCreateUpval
// returns the same *Upvalue for a given slot so this holds; if it ever minted a
// second upvalue for the slot, the two closures would drift apart once the frame
// closes and each cell became independent.

// OPEN case: reader and writer share while the owning frame is still live.
// (Trivially true -- both alias the same stack slot -- but it pins the wiring.)
func TestSharedUpvalueOpen(t *testing.T) {
	got := runProg(t, `
(define (open-share)
  (let ((n 1))
    (let ((rd (lambda () n))
          (wr (lambda (v) (set! n v))))
      (wr 8)      ; write through the writer
      (rd))))     ; read through the reader -> 8 iff shared
(open-share)
`)
	if got.Type != TNum || got.Num != 8 {
		t.Fatalf("open sharing broken: got %v, want 8", got.Num)
	}
	t.Logf("open: write via one closure visible through the other (8)")
}

// CLOSED case -- the discriminating one. make-pair returns, closing n's upvalue
// (both closures captured n at value 0). Then a write THROUGH the writer must be
// visible THROUGH the reader. Shared cell -> 7; two separate closed cells -> 0.
func TestSharedUpvalueClosed(t *testing.T) {
	got := runProg(t, `
(define (make-pair)
  (let ((n 0))
    (let ((t (table)))
      (tset! t 1 (lambda () n))
      (tset! t 2 (lambda (v) (set! n v)))
      t)))
(define p (make-pair))   ; n's frame has returned -> upvalue closed
(define rd (tget p 1))
(define wr (tget p 2))
(wr 7)                    ; write through the writer closure
(rd)                      ; read through the reader closure
`)
	if got.Type != TNum || got.Num != 7 {
		t.Fatalf("closed sharing broken: got %v, want 7 "+
			"(0 => reader and writer ended up with separate closed cells)", got.Num)
	}
	t.Logf("closed: after the frame returned, write via writer still visible through reader (7)")
}
