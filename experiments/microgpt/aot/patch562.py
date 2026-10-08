"""Wrap each float64-typed math/* ec.Invoke that `go build` rejects (let-go#562).

    python3 patch562.py <generated-module-dir>
Loops go build -> patch the reported lines -> rebuild, until it builds.
"""
import re, subprocess, sys, pathlib
mod = pathlib.Path(sys.argv[1])
pat = re.compile(r'(=\s*)(ec\.Invoke\(rt\.CachedVarFn\(&__v_math_\w+, "math", "\w+"\), \[\]vm\.Value\{.*\}\))\s*$')
total = 0
for _ in range(20):
    r = subprocess.run(['go', 'build', '-gcflags=-e', '-o', '/dev/null', '.'], cwd=mod, capture_output=True, text=True)
    if r.returncode == 0:
        print(f'builds; patched {total} sites'); sys.exit(0)
    errs = re.findall(r'(\S+\.go):(\d+):\d+: cannot use 1st function result \(value of interface type vm\.Value\) as float64', r.stderr)
    if not errs:
        print(r.stderr[-3000:]); sys.exit(1)
    by_file = {}
    for f, l in errs: by_file.setdefault(f, set()).add(int(l))
    for f, lines in by_file.items():
        p = mod / f.removeprefix('./')
        src = p.read_text().split('\n')
        for l in lines:
            new, n = pat.subn(r'\1unbox562(\2)', src[l-1])
            if n != 1: print(f'cannot patch {f}:{l}: {src[l-1].strip()}'); sys.exit(1)
            src[l-1] = new; total += 1
        p.write_text('\n'.join(src))
print('gave up'); sys.exit(1)
