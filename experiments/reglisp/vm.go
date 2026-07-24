package main

import "fmt"

type Closure struct {
	proto  *Proto
	upvals []*Upvalue
}

type Builtin func(args []Value) Value

// Upvalue starts "open" (aliasing a live slot in the shared stack) and gets
// "closed" (copied into its own cell) when the frame that owns that slot
// goes away. This open->closed split is the paper's headline trick for
// closures (Section 5, Figure 4): the common case -- a local that's never
// captured, or is read by a closure before its frame returns -- costs
// nothing extra; you only pay for the heap cell when a variable actually
// outlives its stack frame.
type Upvalue struct {
	stack  []Value
	idx    int
	closed Value
	isOpen bool
	next   *Upvalue
}

func (u *Upvalue) Get() Value {
	if u.isOpen {
		return u.stack[u.idx]
	}
	return u.closed
}

func (u *Upvalue) Set(v Value) {
	if u.isOpen {
		u.stack[u.idx] = v
	} else {
		u.closed = v
	}
}

type CallInfo struct {
	closure   *Closure
	base      int
	retPC     int
	retTarget int
}

// VM holds a single flat value stack shared by every call frame -- a
// "register window" model, per the paper's Section 7. Frames don't nest by
// recursing into Go's call stack; CALL and RETURN just push/pop a CallInfo
// and keep running the same dispatch loop. That's a deliberate departure
// from a naive tree-walker (and from a VM that recurses into its host
// language's stack for every user call): the whole machine's state --
// call stack + registers + PC -- lives in Go-level data you could in
// principle snapshot and resume, rather than being smeared across native Go
// stack frames.
type VM struct {
	stack   []Value
	globals map[string]Value
	openUV  *Upvalue
	calls   []CallInfo
}

func NewVM() *VM {
	return &VM{
		stack:   make([]Value, 1<<12),
		globals: make(map[string]Value),
	}
}

func (vm *VM) SetGlobal(name string, v Value) { vm.globals[name] = v }

const maxStackSlots = 1 << 24 // ~16M Values; a hard ceiling, same spirit as Lua's own stack-overflow guard

// ensureStack grows the shared stack when a deep call needs more room than
// currently allocated. Growing it is the one place a naive port would break
// silently: any *open* upvalue holds a slice header (vm.stack at the time it
// was created) pointing at the *old* backing array. Reallocating without
// fixing those up would leave existing closures reading/writing a stack no
// one else can see anymore. Real Lua's luaD_reallocstack has the exact same
// obligation for the same reason.
func (vm *VM) ensureStack(need int) {
	if need <= len(vm.stack) {
		return
	}
	if need > maxStackSlots {
		panic("stack overflow")
	}
	newSize := len(vm.stack) * 2
	for newSize < need {
		newSize *= 2
	}
	if newSize > maxStackSlots {
		newSize = maxStackSlots
	}
	newStack := make([]Value, newSize)
	copy(newStack, vm.stack)
	vm.stack = newStack
	for uv := vm.openUV; uv != nil; uv = uv.next {
		uv.stack = newStack
	}
}

func (vm *VM) findOrCreateUpval(absIdx int) *Upvalue {
	var prev *Upvalue
	cur := vm.openUV
	for cur != nil && cur.idx > absIdx {
		prev = cur
		cur = cur.next
	}
	if cur != nil && cur.idx == absIdx {
		return cur
	}
	nu := &Upvalue{isOpen: true, stack: vm.stack, idx: absIdx, next: cur}
	if prev == nil {
		vm.openUV = nu
	} else {
		prev.next = nu
	}
	return nu
}

func (vm *VM) closeUpvalsFrom(fromAbsIdx int) {
	for vm.openUV != nil && vm.openUV.idx >= fromAbsIdx {
		uv := vm.openUV
		uv.closed = vm.stack[uv.idx]
		uv.isOpen = false
		vm.openUV = uv.next
		uv.next = nil
	}
}

func (vm *VM) Call(cl *Closure, args []Value) Value {
	base := 0
	for i, a := range args {
		if i < len(vm.stack) {
			vm.stack[base+i] = a
		}
	}
	vm.calls = []CallInfo{{closure: cl, base: base, retPC: -1, retTarget: -1}}
	pc := 0
	for {
		cur := &vm.calls[len(vm.calls)-1]
		instr := cur.closure.proto.code[pc]
		switch instr.Op() {
		case OpMove:
			vm.stack[cur.base+instr.A()] = vm.stack[cur.base+instr.B()]
			pc++
		case OpLoadK:
			vm.stack[cur.base+instr.A()] = cur.closure.proto.consts[instr.Bx()]
			pc++
		case OpLoadNil:
			vm.stack[cur.base+instr.A()] = Nil
			pc++
		case OpLoadBool:
			vm.stack[cur.base+instr.A()] = Bool(instr.B() != 0)
			pc++
		case OpGetGlobal:
			name := cur.closure.proto.consts[instr.Bx()].AsStr()
			vm.stack[cur.base+instr.A()] = vm.globals[name]
			pc++
		case OpSetGlobal:
			name := cur.closure.proto.consts[instr.Bx()].AsStr()
			vm.globals[name] = vm.stack[cur.base+instr.A()]
			pc++
		case OpGetUpval:
			vm.stack[cur.base+instr.A()] = cur.closure.upvals[instr.B()].Get()
			pc++
		case OpSetUpval:
			cur.closure.upvals[instr.B()].Set(vm.stack[cur.base+instr.A()])
			pc++
		case OpNewTable:
			vm.stack[cur.base+instr.A()] = Value{Type: TTable, Obj: NewTable()}
			pc++
		case OpGetTable:
			t := vm.stack[cur.base+instr.B()].Obj.(*Table)
			key := vm.stack[cur.base+instr.C()]
			vm.stack[cur.base+instr.A()] = t.Get(key)
			pc++
		case OpSetTable:
			t := vm.stack[cur.base+instr.A()].Obj.(*Table)
			key := vm.stack[cur.base+instr.B()]
			val := vm.stack[cur.base+instr.C()]
			t.Set(key, val)
			pc++
		case OpAdd, OpSub, OpMul, OpDiv:
			a := vm.stack[cur.base+instr.B()].Num
			b := vm.stack[cur.base+instr.C()].Num
			var r float64
			switch instr.Op() {
			case OpAdd:
				r = a + b
			case OpSub:
				r = a - b
			case OpMul:
				r = a * b
			case OpDiv:
				r = a / b
			}
			vm.stack[cur.base+instr.A()] = Num(r)
			pc++
		case OpUnm:
			vm.stack[cur.base+instr.A()] = Num(-vm.stack[cur.base+instr.B()].Num)
			pc++
		case OpNot:
			vm.stack[cur.base+instr.A()] = Bool(!vm.stack[cur.base+instr.B()].Truthy())
			pc++
		case OpLt:
			vm.stack[cur.base+instr.A()] = Bool(vm.stack[cur.base+instr.B()].Num < vm.stack[cur.base+instr.C()].Num)
			pc++
		case OpLe:
			vm.stack[cur.base+instr.A()] = Bool(vm.stack[cur.base+instr.B()].Num <= vm.stack[cur.base+instr.C()].Num)
			pc++
		case OpEq:
			l, r := vm.stack[cur.base+instr.B()], vm.stack[cur.base+instr.C()]
			vm.stack[cur.base+instr.A()] = Bool(l.Type == r.Type && l.rawKey() == r.rawKey())
			pc++
		case OpJmp:
			pc = instr.Bx()
		case OpJmpIfFalse:
			if vm.stack[cur.base+instr.A()].Truthy() {
				pc++
			} else {
				pc = instr.Bx()
			}
		case OpClose:
			vm.closeUpvalsFrom(cur.base + instr.A())
			pc++
		case OpClosure:
			childProto := cur.closure.proto.protos[instr.Bx()]
			ncl := &Closure{proto: childProto, upvals: make([]*Upvalue, len(childProto.upvals))}
			for i, uvd := range childProto.upvals {
				if uvd.fromLocal {
					ncl.upvals[i] = vm.findOrCreateUpval(cur.base + uvd.index)
				} else {
					ncl.upvals[i] = cur.closure.upvals[uvd.index]
				}
			}
			vm.stack[cur.base+instr.A()] = Value{Type: TClosure, Obj: ncl}
			pc++
		case OpCall:
			calleeReg := instr.A()
			nargs := instr.B() - 1
			callee := vm.stack[cur.base+calleeReg]
			argsBase := cur.base + calleeReg + 1
			switch callee.Type {
			case TClosure:
				childCl := callee.Obj.(*Closure)
				vm.ensureStack(argsBase + childCl.proto.maxStack)
				for i := nargs; i < childCl.proto.numParams; i++ {
					vm.stack[argsBase+i] = Nil
				}
				vm.calls = append(vm.calls, CallInfo{
					closure:   childCl,
					base:      argsBase,
					retPC:     pc + 1,
					retTarget: cur.base + calleeReg,
				})
				pc = 0
			case TBuiltin:
				fn := callee.Obj.(Builtin)
				argVals := make([]Value, nargs)
				copy(argVals, vm.stack[argsBase:argsBase+nargs])
				vm.stack[cur.base+calleeReg] = fn(argVals)
				pc++
			default:
				panic(fmt.Sprintf("attempt to call a non-function value (%v)", callee))
			}
		case OpReturn:
			var retVal Value = Nil
			if instr.B() >= 2 {
				retVal = vm.stack[cur.base+instr.A()]
			}
			vm.closeUpvalsFrom(cur.base)
			retPC, retTarget := cur.retPC, cur.retTarget
			vm.calls = vm.calls[:len(vm.calls)-1]
			if len(vm.calls) == 0 {
				return retVal
			}
			vm.stack[retTarget] = retVal
			pc = retPC
		default:
			panic(fmt.Sprintf("unhandled opcode %v", instr.Op()))
		}
	}
}
