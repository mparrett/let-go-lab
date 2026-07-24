package main

import "fmt"

type ValueType uint8

const (
	TNil ValueType = iota
	TBool
	TNum
	TStr
	TTable
	TClosure
	TBuiltin
)

// Value is Lua's "tagged union" idea (paper Section 3), Go-flavored. In ANSI C
// the whole point of that section is that you can't rely on a GC-friendly fat
// interface type, so you hand-roll a tagged union to keep values small and
// copies cheap -- and can't use pointer-tagging tricks because ANSI C gives
// you no portable way to steal bits from a pointer. Go's runtime already
// gives us a real GC and a reasonably cheap boxed-interface type, so we don't
// have that specific portability constraint -- but we keep the explicit tag
// + unboxed float64 anyway, because avoiding an interface type-switch (and a
// heap escape) on every arithmetic op is still worth it.
type Value struct {
	Type ValueType
	Num  float64     // used directly when Type == TNum or TBool (0/1)
	Obj  interface{} // string, *Table, *Closure, *Builtin -- only touched for the boxed types
}

var Nil = Value{Type: TNil}

func Bool(b bool) Value {
	n := 0.0
	if b {
		n = 1.0
	}
	return Value{Type: TBool, Num: n}
}

func Num(f float64) Value { return Value{Type: TNum, Num: f} }
func Str(s string) Value  { return Value{Type: TStr, Obj: s} }

func (v Value) AsStr() string { return v.Obj.(string) }

// Truthy implements Lua semantics: everything is truthy except nil and false.
func (v Value) Truthy() bool {
	switch v.Type {
	case TNil:
		return false
	case TBool:
		return v.Num != 0
	default:
		return true
	}
}

// rawKey returns a Go-comparable representation of v suitable for use as a
// map key in Table's hash part.
func (v Value) rawKey() interface{} {
	switch v.Type {
	case TNum:
		return v.Num
	case TStr:
		return v.Obj
	case TBool:
		return v.Num != 0
	default:
		return v.Obj // pointer identity for tables/closures
	}
}

func (v Value) String() string {
	switch v.Type {
	case TNil:
		return "nil"
	case TBool:
		if v.Num != 0 {
			return "true"
		}
		return "false"
	case TNum:
		return fmt.Sprintf("%g", v.Num)
	case TStr:
		return v.Obj.(string)
	case TTable:
		return fmt.Sprintf("table: %p", v.Obj)
	case TClosure:
		return fmt.Sprintf("closure: %p", v.Obj)
	case TBuiltin:
		return "builtin"
	}
	return "?"
}

// Table is Lua 5.0's hybrid array+hash structure (paper Section 4). Dense
// integer keys starting at 1 live in a plain Go slice with no key stored at
// all; everything else falls into the hash part.
//
// This is a simplified version of the real algorithm: Lua computes an
// optimal array size on every resize (the largest n such that at least half
// of 1..n is occupied, per Section 4). We just grow the array by one
// whenever the next contiguous integer key arrives, migrating any hash
// entries that become contiguous as a result, and never shrink. It shows the
// shape of the idea -- amortized O(1) dense-array access with no per-slot key
// overhead -- without the exact resize heuristic.
type Table struct {
	arr  []Value
	hash map[interface{}]Value
}

func NewTable() *Table { return &Table{} }

func (t *Table) Get(key Value) Value {
	if key.Type == TNum {
		if i, ok := arrayIndex(key.Num); ok && i >= 1 && i <= len(t.arr) {
			return t.arr[i-1]
		}
	}
	if t.hash != nil {
		if v, ok := t.hash[key.rawKey()]; ok {
			return v
		}
	}
	return Nil
}

func (t *Table) Set(key, val Value) {
	if key.Type == TNum {
		if i, ok := arrayIndex(key.Num); ok && i >= 1 {
			if i <= len(t.arr) {
				t.arr[i-1] = val
				return
			}
			if i == len(t.arr)+1 {
				t.arr = append(t.arr, val)
				t.migrateFromHash()
				return
			}
		}
	}
	if t.hash == nil {
		t.hash = make(map[interface{}]Value)
	}
	t.hash[key.rawKey()] = val
}

func (t *Table) migrateFromHash() {
	if t.hash == nil {
		return
	}
	for {
		next := float64(len(t.arr) + 1)
		v, ok := t.hash[next]
		if !ok {
			return
		}
		t.arr = append(t.arr, v)
		delete(t.hash, next)
	}
}

func arrayIndex(f float64) (int, bool) {
	i := int(f)
	return i, float64(i) == f
}
