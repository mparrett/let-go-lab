package main

import "fmt"

// Cons is the classic Lisp pair: two slots, `first` and `rest`. A list is a
// chain of cells linked through `rest`, terminated by the empty list. This is
// a deliberate departure from the Lua-5.0 architecture the rest of this
// prototype follows -- Lua's whole data story is the one hybrid Table (no cons
// cell exists), while the let-go/wallisp family this stands in for is
// Clojure-shaped: immutable, persistent, structurally shared lists. So this is
// a new axis grafted alongside TTable, not an un-simplification of Lua.
//
// Immutability is free here: nothing ever writes a Cons after construction, so
// `rest` just hands back the existing tail pointer (no copy), `cons` prepends
// in O(1) by allocating one cell that points at an unchanged tail, and many
// lists can share the same tail safely. Go's GC reclaims a cell once nothing
// references it. The tradeoff is the usual linked-list one: O(1) prepend and
// free tails, O(n) to reach the nth element or to count.
type Cons struct {
	first Value
	rest  Value
}

// EmptyList is the terminator. It is a TList whose Obj is nil (no *Cons), so
// every empty list compares equal (rawKey falls through to Obj == nil) and
// prints as "()". A single shared value; there is only ever one empty list.
var EmptyList = Value{Type: TList}

func consVal(head, tail Value) Value {
	return Value{Type: TList, Obj: &Cons{first: head, rest: tail}}
}

// asCons returns the underlying pair and whether v is a *non-empty* list.
func (v Value) asCons() (*Cons, bool) {
	c, ok := v.Obj.(*Cons)
	return c, ok && c != nil
}

func isEmptyList(v Value) bool { return v.Type == TList && v.Obj == nil }

// installList registers the list primitives as builtins. No VM, compiler, or
// reader changes are needed -- these operate on already-evaluated Values,
// exactly like `print`. (Literal syntax and quote would be a different story;
// that lives in a separate experiment.)
func installList(vm *VM) {
	reg := func(name string, fn Builtin) {
		vm.SetGlobal(name, Value{Type: TBuiltin, Obj: fn})
	}

	// (list a b c ...) -> (a b c ...); (list) -> the empty list.
	reg("list", func(args []Value) Value {
		out := EmptyList
		for i := len(args) - 1; i >= 0; i-- {
			out = consVal(args[i], out)
		}
		return out
	})

	// (cons x lst) -> a new cell whose first is x and whose rest is lst.
	reg("cons", func(args []Value) Value {
		if len(args) != 2 {
			panic(fmt.Sprintf("cons: expected 2 args, got %d", len(args)))
		}
		if args[1].Type != TList {
			panic(fmt.Sprintf("cons: rest must be a list, got %v", args[1]))
		}
		return consVal(args[0], args[1])
	})

	// (first lst) -> first element, or nil for the empty list (Clojure semantics).
	reg("first", func(args []Value) Value {
		if c, ok := args[0].asCons(); ok {
			return c.first
		}
		return Nil
	})

	// (rest lst) -> the tail (the same cells, not a copy), or () for the empty list.
	reg("rest", func(args []Value) Value {
		if c, ok := args[0].asCons(); ok {
			return c.rest
		}
		return EmptyList
	})

	// (count lst) -> number of elements (O(n), walks the rest chain).
	reg("count", func(args []Value) Value {
		n := 0
		for cur := args[0]; ; {
			c, ok := cur.asCons()
			if !ok {
				break
			}
			n++
			cur = c.rest
		}
		return Num(float64(n))
	})

	// (empty? lst) -> true for the empty list.
	reg("empty?", func(args []Value) Value {
		return Bool(isEmptyList(args[0]))
	})

	// (list? x) -> true if x is a list (empty or not).
	reg("list?", func(args []Value) Value {
		return Bool(args[0].Type == TList)
	})
}
