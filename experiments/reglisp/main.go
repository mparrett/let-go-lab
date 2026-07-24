package main

import (
	"fmt"
	"strings"
	"time"
)

func run(vm *VM, src string) Value {
	forms, err := Read(src)
	if err != nil {
		panic(err)
	}
	proto, err := Compile(forms)
	if err != nil {
		panic(err)
	}
	main := &Closure{proto: proto}
	return vm.Call(main, nil)
}

func newDemoVM() *VM {
	vm := NewVM()
	vm.SetGlobal("print", Value{Type: TBuiltin, Obj: Builtin(func(args []Value) Value {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = a.String()
		}
		fmt.Println(strings.Join(parts, " "))
		return Nil
	})})
	installList(vm)
	return vm
}

func main() {
	fmt.Println("=== reglisp: let-go's surface, Lua 5.0's architecture ===")

	fmt.Println("\n--- bytecode for a small closure-capturing program ---")
	src := `
(define (make-adder n)
  (lambda (x) (+ x n)))
(define add5 (make-adder 5))
(print (add5 10))
`
	forms, _ := Read(src)
	proto, _ := Compile(forms)
	fmt.Println(strings.TrimRight(Disassemble(proto, ""), "\n"))
	vm := newDemoVM()
	main := &Closure{proto: proto}
	vm.Call(main, nil)

	fmt.Println("\n--- closures: two independent counters share no state ---")
	vm2 := newDemoVM()
	run(vm2, `
(define (make-counter)
  (let ((n 0))
    (lambda ()
      (set! n (+ n 1))
      n)))
(define c1 (make-counter))
(define c2 (make-counter))
(print "c1:" (c1) (c1) (c1))
(print "c2:" (c2) (c2))
(print "c1 again:" (c1))
`)

	fmt.Println("\n--- hybrid array+hash table ---")
	vm3 := newDemoVM()
	run(vm3, `
(define t (table))
(tset! t 1 "one")
(tset! t 2 "two")
(tset! t 3 "three")
(tset! t "name" "reglisp")
(print "t[2] =" (tget t 2))
(print "t[name] =" (tget t "name"))
`)

	fmt.Println("\n--- immutable lists (cons cells) ---")
	vmL := newDemoVM()
	run(vmL, `
(define xs (list 1 2 3))
(define ys (cons 0 xs))
(print "xs      =" xs)
(print "ys      =" ys "  ; cons prepends in O(1)")
(print "first ys=" (first ys))
(print "rest ys =" (rest ys) "  ; shares xs's cells, no copy")
(print "xs still=" xs "  ; ys never mutated xs")
(print "count ys=" (count ys) " empty? (list)=" (empty? (list)))
`)

	fmt.Println("\n--- recursive fib(28), flat non-recursive Go dispatch loop ---")
	vm4 := newDemoVM()
	vm4.SetGlobal("clock_ms", Value{Type: TBuiltin, Obj: Builtin(func(args []Value) Value {
		return Num(float64(time.Now().UnixNano()) / 1e6)
	})})
	start := time.Now()
	run(vm4, `
(define (fib n)
  (if (< n 2)
      n
      (+ (fib (- n 1)) (fib (- n 2)))))
(print "fib(28) =" (fib 28))
`)
	fmt.Printf("elapsed: %s\n", time.Since(start))
}
