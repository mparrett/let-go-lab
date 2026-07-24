package main

// Opcode is Lua's register-based instruction set (paper Section 7), trimmed
// to what this toy language needs. Real Lua 5.0 has 35; we have fewer
// because we don't have varargs, methods, or the SETLIST/FORLOOP family.
type Opcode uint8

const (
	OpMove Opcode = iota
	OpLoadK
	OpLoadNil
	OpLoadBool
	OpGetGlobal
	OpSetGlobal
	OpGetUpval
	OpSetUpval
	OpNewTable
	OpGetTable
	OpSetTable
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpUnm
	OpNot
	OpLt
	OpLe
	OpEq
	OpJmp
	OpJmpIfFalse
	OpCall
	OpReturn
	OpClosure
	OpClose
)

var opNames = map[Opcode]string{
	OpMove: "MOVE", OpLoadK: "LOADK", OpLoadNil: "LOADNIL", OpLoadBool: "LOADBOOL",
	OpGetGlobal: "GETGLOBAL", OpSetGlobal: "SETGLOBAL", OpGetUpval: "GETUPVAL",
	OpSetUpval: "SETUPVAL", OpNewTable: "NEWTABLE", OpGetTable: "GETTABLE",
	OpSetTable: "SETTABLE", OpAdd: "ADD", OpSub: "SUB", OpMul: "MUL", OpDiv: "DIV",
	OpUnm: "UNM", OpNot: "NOT", OpLt: "LT", OpLe: "LE", OpEq: "EQ", OpJmp: "JMP",
	OpJmpIfFalse: "JMPIFFALSE", OpCall: "CALL", OpReturn: "RETURN",
	OpClosure: "CLOSURE", OpClose: "CLOSE",
}

// Instr packs an opcode plus operands into a single 32-bit word, exactly the
// layout in the paper's Figure 6: 6 bits OP, 8 bits A, 9+9 bits B/C (which
// double as one 18-bit Bx field for constant indices and jump targets).
//
// One deliberate deviation from real Lua: jump instructions here store an
// *absolute* target PC in Bx rather than a signed offset (sBx). Real Lua
// uses relative offsets so bytecode chunks stay position-independent and
// relocatable; we don't need that property, and absolute targets remove a
// whole class of off-by-one/sign bugs in the backpatcher. Correctness and
// simplicity over strict fidelity here.
type Instr uint32

func encodeABC(op Opcode, a, b, c int) Instr {
	return Instr(uint32(op) | uint32(a)<<6 | uint32(b)<<14 | uint32(c)<<23)
}

func encodeABx(op Opcode, a, bx int) Instr {
	return Instr(uint32(op) | uint32(a)<<6 | uint32(bx)<<14)
}

func (i Instr) Op() Opcode { return Opcode(i & 0x3F) }
func (i Instr) A() int     { return int((i >> 6) & 0xFF) }
func (i Instr) B() int     { return int((i >> 14) & 0x1FF) }
func (i Instr) C() int     { return int((i >> 23) & 0x1FF) }
func (i Instr) Bx() int    { return int(i >> 14) }
