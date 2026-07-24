# reglisp

`let-go`'s job (small embedded VM language, written in Go), Lua 5.0's
architecture. A prototype, not a rewrite of let-go — same s-expression
surface as the let-go/wallisp family, but every implementation decision
underneath follows *The Implementation of Lua 5.0* (Ierusalimschy, de
Figueiredo, Celes, JUCS 2005) instead of let-go's own design.

## Run it

```
go run .
```

Prints disassembly for a closure-capturing program, then runs three demos:
independent closures, the hybrid table, and fib(28) timed.

## The three things taken from the paper

- **Register VM, not stack VM** (`opcode.go`, `compile.go`) — 32-bit packed
  instructions (`OP:6 A:8 B:9 C:9`, same layout as the paper's Figure 6), a
  one-pass compiler that emits directly from the AST with no separate IR,
  and a `while`-`switch` dispatch loop instead of computed goto (`vm.go`),
  same portability tradeoff Lua makes in C.

- **Hybrid array+hash table** (`value.go`, `Table`) — dense integer keys
  live in a plain slice with the key implicit; everything else goes in a
  Go map. Simplified vs. real Lua (no resize-to-optimal-n heuristic, no
  shrinking), but the shape — no per-slot key overhead when a table is
  actually used as an array — is the same.

- **Open/closed upvalues for closures** (`vm.go`, `Upvalue`) — a captured
  variable aliases its stack slot for free until the frame that owns the
  slot returns; only then does it get copied to its own heap cell. This is
  the one most worth reading if you only read one part: `findOrCreateUpval`,
  `closeUpvalsFrom`, and the `OpClose` emission in `compileLet`.

## The one thing this fixes that let-go currently can't

CALL and RETURN in `vm.go`'s `Run` loop push/pop a `CallInfo` and keep
running the *same* Go `for` loop — they never recurse into Go's own call
stack. `TestDeepRecursion` runs 200,000 levels of genuinely non-tail
recursion (`sumto`) without touching Go's native stack depth at all; only
the flat `vm.calls`/`vm.stack` slices grow, via `ensureStack`, and note
that it fixes up every open upvalue's stack pointer on growth (same
obligation as real Lua's `luaD_reallocstack` — an easy thing to silently
get wrong). That's exactly the property the earlier let-go conversation
flagged as missing: a VM whose entire execution state is data on the Go
heap, not smeared across native stack frames, is a much shorter walk to
"steppable" or "snapshottable" than a recursive tree-walker wearing a
bytecode VM's clothes.

## Deliberate simplifications (i.e., don't grade this against real Lua)

- Jump targets are absolute PCs, not signed relative offsets — real Lua's
  `sBx` buys position-independent code; we don't need that, and absolute
  targets remove a whole backpatching bug class.
- No coroutines, no varargs, no metatables, arithmetic/comparison ops are
  binary/left-folded rather than Lua's `RK` const-or-register operands.
- Register allocation always frees back to a saved high-water mark rather
  than Lua's tight register-reuse discipline — wastes some registers,
  never reuses one within a live expression.

## Files

| file | paper section |
|---|---|
| `value.go` | Section 3 (values), Section 4 (tables) |
| `opcode.go` | Section 7 (instruction encoding) |
| `reader.go` | s-expression reader (no paper parallel — Lua's own hand-written recursive-descent parser is Section 2) |
| `compile.go` | Section 5 (closures/upvalue resolution), Section 7 (codegen) |
| `vm.go` | Section 5 (upvalue open/close), Section 7 (dispatch loop, call convention) |
| `disasm.go` | — |
| `main.go` | demos |
