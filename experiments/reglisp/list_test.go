package main

import "testing"

// builtins pulls the installed list primitives back out of a VM so tests can
// call them directly at the Go level (where pointer identity is observable).
func builtins(t *testing.T) map[string]Builtin {
	t.Helper()
	vm := NewVM()
	installList(vm)
	m := map[string]Builtin{}
	for _, name := range []string{"list", "cons", "first", "rest", "count", "empty?", "list?"} {
		g := vm.globals[name]
		if g.Type != TBuiltin {
			t.Fatalf("%s not installed", name)
		}
		m[name] = g.Obj.(Builtin)
	}
	return m
}

func TestListBuildAccessPrint(t *testing.T) {
	b := builtins(t)
	xs := b["list"]([]Value{Num(1), Num(2), Num(3)})
	if xs.String() != "(1 2 3)" {
		t.Fatalf("print: got %q, want (1 2 3)", xs.String())
	}
	if n := b["count"]([]Value{xs}).Num; n != 3 {
		t.Fatalf("count: got %v, want 3", n)
	}
	if f := b["first"]([]Value{xs}).Num; f != 1 {
		t.Fatalf("first: got %v, want 1", f)
	}
	if b["rest"]([]Value{xs}).String() != "(2 3)" {
		t.Fatalf("rest: got %q, want (2 3)", b["rest"]([]Value{xs}).String())
	}
}

// The headline property: cons prepends without copying, and rest returns the
// SAME cells. Verified by pointer identity, which only Go-level access exposes.
func TestConsStructuralSharingAndImmutability(t *testing.T) {
	b := builtins(t)
	xs := b["list"]([]Value{Num(1), Num(2), Num(3)})
	ys := b["cons"]([]Value{Num(0), xs})

	if ys.String() != "(0 1 2 3)" {
		t.Fatalf("cons: got %q, want (0 1 2 3)", ys.String())
	}
	// rest(ys) must BE xs's cells, not a copy of them.
	shared := b["rest"]([]Value{ys})
	if shared.Obj != xs.Obj {
		t.Fatal("rest(cons(0,xs)) is not the same cells as xs -- tail was copied, not shared")
	}
	// xs is untouched by the cons (immutability / persistence).
	if xs.String() != "(1 2 3)" || b["count"]([]Value{xs}).Num != 3 {
		t.Fatalf("xs was mutated by cons: now %q", xs.String())
	}
}

func TestEmptyListSemantics(t *testing.T) {
	b := builtins(t)
	e := b["list"](nil)
	if !isEmptyList(e) || e.String() != "()" {
		t.Fatalf("empty list: isEmpty=%v str=%q", isEmptyList(e), e.String())
	}
	if !b["empty?"]([]Value{e}).Truthy() {
		t.Fatal("empty? of () should be true")
	}
	if b["first"]([]Value{e}).Type != TNil {
		t.Fatal("first of () should be nil")
	}
	if !isEmptyList(b["rest"]([]Value{e})) {
		t.Fatal("rest of () should be ()")
	}
	if !e.Truthy() {
		t.Fatal("empty list should be truthy (only nil/false are falsey)")
	}
}

// Integration: lists flow through the compiler + VM, not just Go-level calls.
func TestListThroughVM(t *testing.T) {
	got := run(newDemoVM(), `(count (rest (cons 9 (list 1 2 3))))`)
	if got.Type != TNum || got.Num != 3 {
		t.Fatalf("got %v, want 3", got.Num)
	}
}

// Documents the tier-1 equality gotcha: empty lists are equal (rawKey nil),
// but non-empty lists compare by pointer identity, NOT structurally. Matching
// Clojure's structural = would be a VM change, deliberately out of scope here.
func TestListEqualityIsIdentityForNonEmpty(t *testing.T) {
	if !run(newDemoVM(), `(= (list) (list))`).Truthy() {
		t.Fatal("empty lists should be equal")
	}
	if run(newDemoVM(), `(= (list 1) (list 1))`).Truthy() {
		t.Fatal("non-empty lists compare by identity in tier 1; structural = is out of scope")
	}
}
