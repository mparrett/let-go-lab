package main

import (
	"fmt"
	"strconv"
	"strings"
)

// The surface syntax is plain s-expressions (matching the let-go/wallisp
// family this is standing in for) so all compiler effort goes into the VM
// architecture rather than a parser. Lua's own compiler is a hand-written
// recursive-descent parser for the same reason: simplicity and portability
// (paper Section 2) -- ours just doesn't need precedence climbing at all.

type NodeKind uint8

const (
	NNum NodeKind = iota
	NStr
	NSym
	NList
)

type Node struct {
	Kind NodeKind
	Num  float64
	Str  string
	List []*Node
}

func Read(src string) ([]*Node, error) {
	toks := tokenize(src)
	p := &parser{toks: toks}
	var forms []*Node
	for p.pos < len(p.toks) {
		n, err := p.readForm()
		if err != nil {
			return nil, err
		}
		forms = append(forms, n)
	}
	return forms, nil
}

func tokenize(src string) []string {
	var toks []string
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == ';':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '(' || c == ')':
			toks = append(toks, string(c))
			i++
		case c == '"':
			j := i + 1
			for j < len(src) && src[j] != '"' {
				j++
			}
			toks = append(toks, src[i:j+1])
			i = j + 1
		default:
			j := i
			for j < len(src) && !strings.ContainsRune(" \t\n\r()", rune(src[j])) {
				j++
			}
			toks = append(toks, src[i:j])
			i = j
		}
	}
	return toks
}

type parser struct {
	toks []string
	pos  int
}

func (p *parser) readForm() (*Node, error) {
	if p.pos >= len(p.toks) {
		return nil, fmt.Errorf("unexpected eof")
	}
	tok := p.toks[p.pos]
	switch {
	case tok == "(":
		p.pos++
		var list []*Node
		for {
			if p.pos >= len(p.toks) {
				return nil, fmt.Errorf("unclosed (")
			}
			if p.toks[p.pos] == ")" {
				p.pos++
				break
			}
			n, err := p.readForm()
			if err != nil {
				return nil, err
			}
			list = append(list, n)
		}
		return &Node{Kind: NList, List: list}, nil
	case tok == ")":
		return nil, fmt.Errorf("unexpected )")
	case strings.HasPrefix(tok, "\""):
		p.pos++
		return &Node{Kind: NStr, Str: tok[1 : len(tok)-1]}, nil
	default:
		p.pos++
		if f, err := strconv.ParseFloat(tok, 64); err == nil {
			return &Node{Kind: NNum, Num: f}, nil
		}
		return &Node{Kind: NSym, Str: tok}, nil
	}
}
