# Full native demo builds

`just native mandelbrot`, `just native pathtrace`, and `just native microgpt`
compile the existing `.lg` programs with `lg compile`. Executables and generated
Go live in `dist/native/`. These are complete programs; the separate
`demos/mandelbrot-aot` experiment still uses a lowered kernel and a Go terminal
driver.

The native compiler is pinned by [`config/let-go-native.sha`](../config/let-go-native.sha)
to `49858bd247e5277874d3f5bad80c57c756a074d6`. The first build fetches that SHA
into `.cache/let-go-native/<sha>` and runs upstream's `make build`, including its
smoke gates. It needs Git, Make, Go 1.27.1 or newer, and network access for the
first runtime/module downloads. Microgpt also fetches its hash-checked reference
using its existing fetch script. `just` is optional: run
`scripts/build-native.sh <demo>` directly.

To reuse an existing checkout of the pinned SHA:

```sh
LETGO_NATIVE=/path/to/pinned-let-go just native pathtrace
dist/native/pathtrace
```

Builds verify the checkout's HEAD against the pin. `LETGO_NATIVE` is separate
from the `LETGO` used by VM/browser recipes. CI and Pages retain v1.13.0 for
the browser; a separate native CI job uses the SHA pin.

Mandelbrot needs a sixel terminal for interactive output; pathtrace uses the
iTerm2 inline-image protocol. Piped/headless runs execute each demo's benchmark.
Pathtrace's headless PNG writer needs `openssl` on PATH. Microgpt reads its data
and weights relative to the working directory:

```sh
just native microgpt
cd experiments/microgpt
../../dist/native/microgpt 0 weights.txt
```

## What changed

The newer compiler's trampoline-result unboxing fix
([#1044](https://github.com/nooga/let-go/pull/1044)) lets the full Mandelbrot and
pathtrace programs build without patching generated Go. The `^double` coordinate
hints on **both** `escape` and its caller `compute-grid` keep Mandelbrot's hot
loop native. Hinting only `escape` leaves `compute-grid` on a VM fallback.
Type hints are Clojure metadata; they were already supported before this pin.
The newer working full-program build is what makes them useful here.

On Apple M2 / macOS 26.7.1 / Go 1.27.1, three interleaved native runs of the
20-frame home-view benchmark gave these medians:

| Full native Mandelbrot | Before coordinate hints | With coordinate hints |
|---|---:|---:|
| Whole run | 3.266 s | 1.262 s |
| Compute per frame | 107 ms | 8 ms |
| Sixel encoding per frame | 55 ms | 55 ms |

All frames retained `450584` iterations and `27695` encoded bytes. These are
local workload measurements, not universal performance promises. Run results are
in [native-mandelbrot-results.json](native-mandelbrot-results.json).

The preceding runtime audit found no meaningful VM speed improvement from
v1.13.0 to this tip. Its 4-sample/pixel pathtrace test ran about 2.8× faster
natively than on the tip VM and produced a byte-identical PNG. CI uses the
normal 32-sample workload and compares the actual PNG bytes, not timings.

## Checks and remaining limitations

The native CI job builds all three executables, checks Mandelbrot's frame count,
iteration counts and actual sixel bytes, compares pathtrace PNG bytes, and checks
microgpt's pretrained samples and all 4,192 weights after three training steps.
It also runs the existing microgpt comparison with Python. For a local check:

```sh
scripts/build-native.sh mandelbrot
scripts/build-native.sh pathtrace
scripts/build-native.sh microgpt
PIN=$(cat config/let-go-native.sha)
python3 test/native_demo_test.py --lg ".cache/let-go-native/$PIN/bin/lg"
```

When using `LETGO_NATIVE`, pass that checkout's `bin/lg` to the test instead.

Ptcanvas remains a VM/browser demo: this compiler emits an invalid type assertion
at two native closure call sites. Microgpt's explicit loops, type hints,
plain-number inference branches, text weights, and profiling hook still address
open upstream limitations and remain in place.

The new Go build tag `lg_no_json` drops unused JSON/Transit support. In the audit,
it reduced Mandelbrot's standalone browser HTML by about 5%, but microgpt's by
only 0.36% because its host-eval bridge retains JSON support. It is not enabled
for Pages: the current release does not have it, and there is no measured model
speed gain. New caller-module and `-import` features support custom Go packages;
these demos do not need extra bindings.

To update the native pin, choose a full upstream SHA, match CI's Go version to
its go.mod, rebuild the three executables, and run the parity checks. Keep the
browser/Pages release pin as a separate decision.
