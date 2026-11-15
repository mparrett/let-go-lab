"""Rewrite lg -w's generated main.go into lg compile's entry order: replay the
program with *compiling-aot* true (so its top-level (-main) call is skipped),
let LoadProgramNamespaces install the lowered overrides, then enter -main.
Idempotent; the namespace and entry are this demo's."""
import sys, pathlib
p = pathlib.Path(sys.argv[1]); t = p.read_text()
if "aot-wasm entry" in t:
    sys.exit(0)
start = t.index("\trun := func(chunk *vm.CodeChunk) error {")
end = t.rindex("}")  # closing brace of main
new = '''\t// aot-wasm entry: see experiments/aot-wasm/patch_main.py.
\taot := rt.CoreNS.Lookup("*compiling-aot*").(*vm.Var)
\taot.SetRoot(vm.TRUE)
\tif err := rt.LoadProgramNamespaces(unit); err != nil {
\t\tfmt.Fprintln(os.Stderr, err)
\t\treturn
\t}
\tif err := rt.RunProgramMainChunk(unit); err != nil {
\t\tfmt.Fprint(os.Stderr, vm.FormatError(err))
\t\treturn
\t}
\taot.SetRoot(vm.FALSE)
\tif err := rt.InvokeProgramEntry(vm.NewExecContext(), "mandelbrot", "-main", nil); err != nil {
\t\tfmt.Fprint(os.Stderr, vm.FormatError(err))
\t}
'''
p.write_text(t[:start] + new + t[end:])
print("aot-wasm: patched", p, file=sys.stderr)
