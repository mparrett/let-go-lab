# reglisp — a register-VM take on let-go's job (verified)

A prototype by a collaborator: **let-go's job** (a small embedded VM language
written in Go), rebuilt on **Lua 5.0's architecture** instead of let-go's own.
Same s-expression surface as the let-go/wallisp family; every implementation
decision underneath follows *The Implementation of Lua 5.0* (Ierusalimschy,
de Figueiredo, Celes, JUCS 2005) — a register VM, a hybrid array+hash table,
and open/closed upvalues for closures.

The design doc (the prototype's own notes, paper-section mapping, and the list
of deliberate simplifications) is preserved verbatim in [`NOTES.md`](./NOTES.md).
This README is the **experiment**: it takes the three load-bearing claims from
those notes and checks whether they actually hold, with reproducible tests —
then adds one small feature (immutable lists) to probe how the architecture
takes extension.

Run everything with [`./verify.sh`](./verify.sh) (add `mutate` to also run the
mutation check described below).

## What was verified

| Claim (from `NOTES.md`) | Verdict | How |
|---|---|---|
| CALL/RETURN never recurse into Go's stack; the whole call stack is heap data | ✅ holds | `TestDeepRecursionSurvivesTinyNativeStack`, `TestOneMillionDeep` |
| `ensureStack` re-points every **open** upvalue on stack growth (the "easy thing to silently get wrong") | ✅ holds, and is load-bearing | `TestOpenUpvalueSurvivesStackRegrow` + a mutation check |
| Two closures over the **same** variable share one cell — open *and* after the frame closes | ✅ holds, and is load-bearing | `TestSharedUpvalueOpen`, `TestSharedUpvalueClosed` + a mutation check |

### 1. Execution state lives on the heap, not Go's stack

`vm.go`'s `Run` loop is a single native frame. `OpCall` on a closure appends a
`CallInfo` and sets `pc = 0` — it never calls `vm.Call` again — and `OpReturn`
slices the `CallInfo` off. So reglisp recursion depth grows the heap-side
`vm.calls`/`vm.stack` slices, not Go's goroutine stack.

Tested with genuinely non-tail recursion (`(+ n (sumto (- n 1)))`, which must
keep frame `n` alive across the call):

| Test | Native stack cap | Depth | Result |
|---|---|---|---|
| `TestDeepRecursionSurvivesTinyNativeStack` | **512 KB** (clamped via `debug.SetMaxStack`) | 200,000 | `20000100000` ✓ |
| `TestOneMillionDeep` | default | 1,000,000 | `500000500000` ✓ (~3.5 s) |

The control: an ordinary Go recursive `func` at the same 200k depth under the
same 512 KB cap dies with `fatal error: stack overflow` (unrecoverable — a hard
process kill). reglisp doesn't notice, because none of those frames are native.
That's a property let-go's tree-walker doesn't have, and — per `NOTES.md` — what
makes step/snapshot/resume a short walk from here.

### 2. Open-upvalue fixup on stack growth

An **open** upvalue aliases a live stack slot by caching a slice header
(`uv.stack`). When `ensureStack` reallocates the backing array, every open
upvalue must be re-pointed at the new array, or it silently reads/writes a
stack nobody else can see. `TestOpenUpvalueSurvivesStackRegrow` is built to
detect exactly that: it keeps an upvalue open *across* a forced regrow, writes
`42` **through the upvalue**, then reads the same local **directly as a
register** — the two paths only agree if the fixup happened.

This test is only meaningful if it can actually fail, so we confirmed it does.
Deleting the three-line fixup loop from `ensureStack` (a throwaway copy; run
`./verify.sh mutate`) flips the probe from `42` to `0` — no crash, just a
quietly wrong answer:

| Build | `ensureStack` fixup loop | Probe returns |
|---|---|---|
| real code | present | **42** ✓ |
| mutant | removed | **0** ✗ |

Note this is the *open* case — the counter demo in `main.go` only exercises
*closed* upvalues (captured after the owning frame returned). This probe is the
first thing here to keep an upvalue open while the stack moves under it.

### 3. Two closures over one variable share a cell

`findOrCreateUpval` returns the *same* `Upvalue` for a given slot, so a reader
and a writer closing over the same variable stay one variable — not two copies.
The counter demo only shows *independent* counters (different slots), so it
never exercises sharing. Two probes do:

- `TestSharedUpvalueOpen` — reader and writer share while the owning frame is
  still live. Trivially true (both alias the same stack slot), but it pins the
  wiring.
- `TestSharedUpvalueClosed` — the discriminating one. After the frame returns
  (both closures captured the variable at `0`), a write **through the writer**
  must be visible **through the reader**.

| Build | `findOrCreateUpval` reuse check | Closed probe returns |
|---|---|---|
| real code | present | **7** ✓ |
| mutant | removed (mints a new upvalue per capture) | **0** ✗ |

The failure is the silent kind again: while open, two separate upvalues over one
slot still alias the same stack cell and agree — they only diverge once the
frame closes and each gets its own heap cell. Which is exactly why the *closed*
probe, not the open one, is what catches it.

## Extension: immutable cons-cell lists (`list.go`)

Beyond verifying the prototype, this experiment adds one small feature: a
Clojure/let-go-style immutable list, to see how far it reaches into the
architecture. Answer: barely at all. `list`, `cons`, `first`, `rest`, `count`,
`empty?`, and `list?` are ordinary **builtins** — they operate on
already-evaluated `Value`s, exactly like `print` — so nothing in the reader,
compiler, or VM changed. The whole addition is `list.go` plus a `TList` tag and
a `String()` case in `value.go`.

```lisp
(define xs (list 1 2 3))
(define ys (cons 0 xs))   ; O(1) prepend, one new cell
(rest ys)                 ; => (1 2 3) -- the SAME cells as xs, not a copy
xs                        ; => (1 2 3) -- untouched; ys never mutated it
```

Immutability is free (nothing writes a `Cons` after construction), so `rest`
hands back the tail pointer and lists share structure safely;
`TestConsStructuralSharingAndImmutability` pins the sharing by pointer identity.

Two things are deliberately **out of scope**, because they're not builtins —
they cross the code/data boundary and belong in a separate parallel experiment:

- **Literal syntax + `quote`** (`'(1 2 3)`). An unquoted `(1 2 3)` compiles to a
  *call*, so a literal list needs a `quote` special form — a reader + compiler
  change — and quoting symbols forces a runtime symbol type the VM doesn't have.
- **Structural equality.** `OpEq` compares by pointer identity, so
  `(= (list 1) (list 1))` is `false` (empty lists are equal via the nil key).
  Matching Clojure's value equality is a VM change.
  See `TestListEqualityIsIdentityForNonEmpty`.

## Run it

```
go run .          # demos: disassembly, closures, table, immutable lists, fib(28)
go test ./...     # the verification tests added by this experiment
./verify.sh       # both of the above, verbose
./verify.sh mutate  # + the ensureStack mutation check
```

## Layout

```
NOTES.md         the prototype's design doc (verbatim): paper mapping, simplifications
main.go          demos (make-adder disassembly, independent counters, hybrid table, fib)
reader.go        s-expression reader
compile.go       one-pass compiler: closure/upvalue resolution + codegen
opcode.go        32-bit packed instruction encoding (OP:6 A:8 B:9 C:9)
vm.go            register VM: dispatch loop, call convention, open/close upvalues, ensureStack
value.go         value repr + hybrid array+hash Table + TList printing
disasm.go        bytecode disassembler
list.go          immutable cons-cell lists + builtins (this experiment's extension)
claim_test.go        deep non-tail recursion under a clamped native stack (heap-state claim)
upval_test.go        open upvalue survives a stack regrow (ensureStack fixup claim)
shared_upval_test.go two closures over one variable share a cell, open and closed
list_test.go         cons/first/rest, structural sharing, empty + equality semantics
verify.sh            reproduce all of the above, incl. the optional mutation checks
```

## Status

Experimental and unmerged — a prototype spike kept for reference, not part of
any build or CI here. It's a standalone Go module (`go.mod` module `reglisp`)
with no dependency on the surrounding lab or on a local `lg` checkout; it builds
and runs with the Go toolchain alone.
