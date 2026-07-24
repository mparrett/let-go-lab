package main

import (
	"fmt"
	"strings"
)

func Disassemble(p *Proto, indent string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%sproto %q (params=%d, maxstack=%d, upvals=%d)\n", indent, p.name, p.numParams, p.maxStack, len(p.upvals))
	for i, uv := range p.upvals {
		src := "upval"
		if uv.fromLocal {
			src = "local"
		}
		fmt.Fprintf(&b, "%s  upval[%d] %s <- parent %s[%d]\n", indent, i, uv.name, src, uv.index)
	}
	for i, instr := range p.code {
		op := instr.Op()
		switch op {
		case OpLoadK:
			fmt.Fprintf(&b, "%s  %3d  %-11s A=%d Bx=%d  ; K=%s\n", indent, i, opNames[op], instr.A(), instr.Bx(), p.consts[instr.Bx()].String())
		case OpGetGlobal, OpSetGlobal:
			fmt.Fprintf(&b, "%s  %3d  %-11s A=%d Bx=%d  ; %q\n", indent, i, opNames[op], instr.A(), instr.Bx(), p.consts[instr.Bx()].AsStr())
		case OpJmp, OpJmpIfFalse:
			fmt.Fprintf(&b, "%s  %3d  %-11s A=%d ->%d\n", indent, i, opNames[op], instr.A(), instr.Bx())
		case OpClosure:
			fmt.Fprintf(&b, "%s  %3d  %-11s A=%d proto=%d\n", indent, i, opNames[op], instr.A(), instr.Bx())
		default:
			fmt.Fprintf(&b, "%s  %3d  %-11s A=%d B=%d C=%d\n", indent, i, opNames[op], instr.A(), instr.B(), instr.C())
		}
	}
	for _, child := range p.protos {
		b.WriteString(Disassemble(child, indent+"  "))
	}
	return b.String()
}
