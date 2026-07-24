package main

import "fmt"

// Proto is a compiled function prototype: its code, its constants, its
// nested function prototypes, and the upvalue descriptors that say how to
// build a closure's upvalue array out of its *parent's* frame when a
// CLOSURE instruction runs. This mirrors the paper's Section 5 exactly.
type Proto struct {
	code      []Instr
	consts    []Value
	protos    []*Proto
	upvals    []upvalDesc
	numParams int
	maxStack  int
	name      string
}

// upvalDesc says where an upvalue comes from *in the enclosing function*:
// either one of its live locals (a register) or one of its own upvalues.
// Chaining these across nested closures is exactly Cardelli's "flat
// closures" technique the paper cites in Section 5 -- every function only
// ever reaches one level up, never further, so no closure needs to know
// about anything beyond its immediate parent.
type upvalDesc struct {
	name      string
	fromLocal bool
	index     int
}

type localVar struct {
	name string
	reg  int
}

type FuncState struct {
	parent  *FuncState
	proto   *Proto
	locals  []localVar
	freereg int
}

func (fs *FuncState) emit(i Instr) int {
	fs.proto.code = append(fs.proto.code, i)
	return len(fs.proto.code) - 1
}

func (fs *FuncState) patch(idx int, target int) {
	old := fs.proto.code[idx]
	fs.proto.code[idx] = encodeABx(old.Op(), old.A(), target)
}

func (fs *FuncState) here() int { return len(fs.proto.code) }

func (fs *FuncState) addConst(v Value) int {
	for i, c := range fs.proto.consts {
		if c.Type == v.Type && c.rawKey() == v.rawKey() {
			return i
		}
	}
	fs.proto.consts = append(fs.proto.consts, v)
	return len(fs.proto.consts) - 1
}

func (fs *FuncState) reserveReg() int {
	r := fs.freereg
	fs.freereg++
	if fs.freereg > fs.proto.maxStack {
		fs.proto.maxStack = fs.freereg
	}
	return r
}

// resolveVar walks up the FuncState chain looking for name as a local. If
// it's found in an ancestor (not the immediate function), an upvalDesc is
// threaded through *every* intervening function -- the flat-closures move.
func resolveVar(fs *FuncState, name string) (kind string, idx int, ok bool) {
	for i := len(fs.locals) - 1; i >= 0; i-- {
		if fs.locals[i].name == name {
			return "local", fs.locals[i].reg, true
		}
	}
	for i, uv := range fs.proto.upvals {
		if uv.name == name {
			return "upval", i, true
		}
	}
	if fs.parent == nil {
		return "", 0, false
	}
	pkind, pidx, ok := resolveVar(fs.parent, name)
	if !ok {
		return "", 0, false
	}
	fs.proto.upvals = append(fs.proto.upvals, upvalDesc{name: name, fromLocal: pkind == "local", index: pidx})
	return "upval", len(fs.proto.upvals) - 1, true
}

var binOps = map[string]Opcode{"+": OpAdd, "-": OpSub, "*": OpMul, "/": OpDiv}
var cmpOps = map[string]Opcode{"<": OpLt, "<=": OpLe, "=": OpEq}

func Compile(forms []*Node) (proto *Proto, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	proto = &Proto{name: "main"}
	fs := &FuncState{proto: proto}
	compileBody(fs, forms)
	return proto, nil
}

// compileBody compiles a sequence of statements, discarding all but the
// last value, and emits the function's RETURN.
func compileBody(fs *FuncState, forms []*Node) {
	saved := fs.freereg
	var last int = -1
	for i, f := range forms {
		if i == len(forms)-1 {
			r := fs.reserveReg()
			compileInto(fs, f, r)
			last = r
		} else {
			r := fs.reserveReg()
			compileInto(fs, f, r)
			fs.freereg = saved
		}
	}
	if last == -1 {
		fs.emit(encodeABC(OpReturn, 0, 1, 0))
	} else {
		fs.emit(encodeABC(OpReturn, last, 2, 0))
	}
}

func compileFunc(parent *FuncState, params []string, body []*Node, name string) *Proto {
	proto := &Proto{name: name, numParams: len(params)}
	fs := &FuncState{parent: parent, proto: proto}
	for _, p := range params {
		reg := fs.reserveReg()
		fs.locals = append(fs.locals, localVar{name: p, reg: reg})
	}
	compileBody(fs, body)
	return proto
}

// compileInto compiles n so its value ends up in register `target`. It is
// the single place that manages the freereg high-water mark: anything n
// needs internally (call args, operand temporaries) is reserved above
// target and released again before returning, per the "correctness and
// simplicity over squeezing out every register" tradeoff -- we don't do
// Lua's full register-reuse discipline, we just never reuse a register
// within the same expression twice.
func compileInto(fs *FuncState, n *Node, target int) {
	saved := fs.freereg
	if target >= fs.freereg {
		fs.freereg = target + 1
	}
	switch n.Kind {
	case NNum:
		fs.emit(encodeABx(OpLoadK, target, fs.addConst(Num(n.Num))))
	case NStr:
		fs.emit(encodeABx(OpLoadK, target, fs.addConst(Str(n.Str))))
	case NSym:
		compileVarRef(fs, n.Str, target)
	case NList:
		compileList(fs, n, target)
	}
	if target >= saved {
		fs.freereg = target + 1
	} else {
		fs.freereg = saved
	}
}

func compileVarRef(fs *FuncState, name string, target int) {
	if kind, idx, ok := resolveVar(fs, name); ok {
		switch kind {
		case "local":
			if idx != target {
				fs.emit(encodeABC(OpMove, target, idx, 0))
			}
		case "upval":
			fs.emit(encodeABC(OpGetUpval, target, idx, 0))
		}
		return
	}
	fs.emit(encodeABx(OpGetGlobal, target, fs.addConst(Str(name))))
}

func compileList(fs *FuncState, n *Node, target int) {
	if len(n.List) == 0 {
		fs.emit(encodeABC(OpLoadNil, target, 0, 0))
		return
	}
	head := n.List[0]
	if head.Kind == NSym {
		switch head.Str {
		case "if":
			compileIf(fs, n.List[1:], target)
			return
		case "lambda":
			compileLambda(fs, n.List[1], n.List[2:], "lambda", target)
			return
		case "define":
			compileDefine(fs, n.List[1:], target)
			return
		case "let":
			compileLet(fs, n.List[1], n.List[2:], target)
			return
		case "set!":
			compileSet(fs, n.List[1].Str, n.List[2], target)
			return
		case "begin", "do":
			compileSeq(fs, n.List[1:], target)
			return
		case "not":
			r := fs.reserveReg()
			compileInto(fs, n.List[1], r)
			fs.emit(encodeABC(OpNot, target, r, 0))
			return
		case "table":
			fs.emit(encodeABC(OpNewTable, target, 0, 0))
			return
		case "tget":
			tr := fs.reserveReg()
			compileInto(fs, n.List[1], tr)
			kr := fs.reserveReg()
			compileInto(fs, n.List[2], kr)
			fs.emit(encodeABC(OpGetTable, target, tr, kr))
			return
		case "tset!":
			tr := fs.reserveReg()
			compileInto(fs, n.List[1], tr)
			kr := fs.reserveReg()
			compileInto(fs, n.List[2], kr)
			vr := fs.reserveReg()
			compileInto(fs, n.List[3], vr)
			fs.emit(encodeABC(OpSetTable, tr, kr, vr))
			if target != tr {
				fs.emit(encodeABC(OpLoadNil, target, 0, 0))
			}
			return
		}
		if op, ok := binOps[head.Str]; ok && len(n.List) >= 3 {
			compileFold(fs, op, n.List[1:], target)
			return
		}
		if op, ok := cmpOps[head.Str]; ok && len(n.List) == 3 {
			l := fs.reserveReg()
			compileInto(fs, n.List[1], l)
			r := fs.reserveReg()
			compileInto(fs, n.List[2], r)
			fs.emit(encodeABC(op, target, l, r))
			return
		}
		if head.Str == ">" && len(n.List) == 3 {
			l := fs.reserveReg()
			compileInto(fs, n.List[1], l)
			r := fs.reserveReg()
			compileInto(fs, n.List[2], r)
			fs.emit(encodeABC(OpLt, target, r, l))
			return
		}
		if head.Str == ">=" && len(n.List) == 3 {
			l := fs.reserveReg()
			compileInto(fs, n.List[1], l)
			r := fs.reserveReg()
			compileInto(fs, n.List[2], r)
			fs.emit(encodeABC(OpLe, target, r, l))
			return
		}
	}
	compileCall(fs, n, target)
}

// compileFold handles variadic +, -, *, / by left-folding pairwise, e.g.
// (+ a b c) compiles as ((a+b)+c). Two-arg is the common case and needs no
// folding at all.
func compileFold(fs *FuncState, op Opcode, args []*Node, target int) {
	acc := fs.reserveReg()
	compileInto(fs, args[0], acc)
	for _, a := range args[1:] {
		r := fs.reserveReg()
		compileInto(fs, a, r)
		fs.emit(encodeABC(op, acc, acc, r))
	}
	if acc != target {
		fs.emit(encodeABC(OpMove, target, acc, 0))
	}
}

func compileIf(fs *FuncState, rest []*Node, target int) {
	cond, then := rest[0], rest[1]
	var els *Node
	if len(rest) > 2 {
		els = rest[2]
	}
	cr := fs.reserveReg()
	compileInto(fs, cond, cr)
	jf := fs.emit(encodeABx(OpJmpIfFalse, cr, 0))
	compileInto(fs, then, target)
	jend := fs.emit(encodeABx(OpJmp, 0, 0))
	fs.patch(jf, fs.here())
	if els != nil {
		compileInto(fs, els, target)
	} else {
		fs.emit(encodeABC(OpLoadNil, target, 0, 0))
	}
	fs.patch(jend, fs.here())
}

func compileLambda(fs *FuncState, paramsNode *Node, body []*Node, name string, target int) {
	var params []string
	for _, p := range paramsNode.List {
		params = append(params, p.Str)
	}
	child := compileFunc(fs, params, body, name)
	idx := len(fs.proto.protos)
	fs.proto.protos = append(fs.proto.protos, child)
	fs.emit(encodeABx(OpClosure, target, idx))
}

func compileDefine(fs *FuncState, rest []*Node, target int) {
	nameNode := rest[0]
	var name string
	var valueNode *Node
	if nameNode.Kind == NList {
		// (define (name p1 p2 ...) body...) sugar for (define name (lambda (p1 p2 ...) body...))
		name = nameNode.List[0].Str
		params := &Node{Kind: NList, List: nameNode.List[1:]}
		valueNode = &Node{Kind: NList, List: append([]*Node{{Kind: NSym, Str: "lambda"}, params}, rest[1:]...)}
	} else {
		name = nameNode.Str
		valueNode = rest[1]
	}
	if fs.parent == nil {
		// top-level define: a global.
		tmp := fs.reserveReg()
		if valueNode.Kind == NList && len(valueNode.List) > 0 && valueNode.List[0].Kind == NSym && valueNode.List[0].Str == "lambda" {
			compileLambda(fs, valueNode.List[1], valueNode.List[2:], name, tmp)
		} else {
			compileInto(fs, valueNode, tmp)
		}
		fs.emit(encodeABx(OpSetGlobal, tmp, fs.addConst(Str(name))))
	} else {
		// nested define: a new local in the current function.
		reg := fs.reserveReg()
		fs.locals = append(fs.locals, localVar{name: name, reg: reg})
		if valueNode.Kind == NList && len(valueNode.List) > 0 && valueNode.List[0].Kind == NSym && valueNode.List[0].Str == "lambda" {
			compileLambda(fs, valueNode.List[1], valueNode.List[2:], name, reg)
		} else {
			compileInto(fs, valueNode, reg)
		}
	}
	fs.emit(encodeABC(OpLoadNil, target, 0, 0))
}

func compileLet(fs *FuncState, bindings *Node, body []*Node, target int) {
	savedLocals := len(fs.locals)
	firstReg := fs.freereg
	for _, b := range bindings.List {
		nameNode, valNode := b.List[0], b.List[1]
		reg := fs.reserveReg()
		compileInto(fs, valNode, reg)
		fs.locals = append(fs.locals, localVar{name: nameNode.Str, reg: reg})
	}
	compileSeq(fs, body, target)
	// Any closure created inside this let that captured one of its locals
	// holds an *open* upvalue pointing at these registers. Now that the
	// scope is ending and the registers are about to be reused, those
	// upvalues must be migrated to the heap -- exactly the CLOSE mechanism
	// from paper Figure 4.
	fs.emit(encodeABC(OpClose, firstReg, 0, 0))
	fs.locals = fs.locals[:savedLocals]
}

func compileSet(fs *FuncState, name string, valNode *Node, target int) {
	kind, idx, ok := resolveVar(fs, name)
	if !ok {
		tmp := fs.reserveReg()
		compileInto(fs, valNode, tmp)
		fs.emit(encodeABx(OpSetGlobal, tmp, fs.addConst(Str(name))))
	} else if kind == "local" {
		compileInto(fs, valNode, idx)
	} else {
		tmp := fs.reserveReg()
		compileInto(fs, valNode, tmp)
		fs.emit(encodeABC(OpSetUpval, tmp, idx, 0))
	}
	fs.emit(encodeABC(OpLoadNil, target, 0, 0))
}

func compileSeq(fs *FuncState, forms []*Node, target int) {
	saved := fs.freereg
	if len(forms) == 0 {
		fs.emit(encodeABC(OpLoadNil, target, 0, 0))
		return
	}
	for i, f := range forms {
		if i == len(forms)-1 {
			compileInto(fs, f, target)
		} else {
			r := fs.reserveReg()
			compileInto(fs, f, r)
			fs.freereg = saved
		}
	}
}

func compileCall(fs *FuncState, n *Node, target int) {
	saved := fs.freereg
	base := fs.reserveReg()
	compileInto(fs, n.List[0], base)
	nargs := 0
	for _, a := range n.List[1:] {
		r := fs.reserveReg()
		compileInto(fs, a, r)
		nargs++
	}
	fs.emit(encodeABC(OpCall, base, nargs+1, 2))
	if base != target {
		fs.emit(encodeABC(OpMove, target, base, 0))
	}
	fs.freereg = saved
	if target >= saved {
		fs.freereg = target + 1
	}
}
