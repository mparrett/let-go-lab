# microgpt in let-go

A readable scalar GPT: tokenizer, autograd, attention, Adam, and sampling in
[one let-go file](microgpt.lg). It ports Andrej Karpathy's dependency-free
[microgpt](https://gist.github.com/karpathy/8627fe009c40f57531cb18360106ce95)
(revision `14fb0388`), with the same model and training graph. The same source
runs on let-go's bytecode VM, as a native Go binary through `lg compile`, and
in the browser through `lg -w`.

The committed weights were trained on 32,033 names for 1,000 steps (final
training loss 2.6057). Sampling from them starts:

```text
sample  1: anria
sample  2: aliia
sample  3: kirli
sample  4: keson
sample  5: denan
```

## Try it

With `lg` on PATH:

```sh
cd experiments/microgpt
./fetch-reference.sh
lg microgpt.lg 0 weights.txt
```

The fetch script downloads the pinned names dataset and Python reference,
checks their SHA-256 hashes, and applies `tape.patch` to make a CPython tape
comparison. It is safe to run again. The reference is fetched rather than
vendored because the gist carries no license.

To train on the VM, run `lg microgpt.lg 1000`. To build and train natively:

```sh
aot/build.sh
aot/microgpt-native 1000 /tmp/microgpt-weights.txt
lg microgpt.lg 0 /tmp/microgpt-weights.txt
```

Native builds need Go and a let-go checkout at or after `bb36063`
(nooga/let-go#1044). The lab's `let-go` symlink is the default; use
`LETGO=/path/to/let-go` to select another checkout. VM and native runs read
each other's weights. The file stores parameter values in model order;
its vocabulary comes from `input.txt`, so use the dataset it was trained on.

To sample in the browser:

```sh
wasm/build.sh --weights weights.txt
python3 ../../scripts/serve.py --dir wasm/out --headers ../../harness/serve.json
```

Open the server's printed URL. Set `LETGO_USE_TINYGO=1` to build with TinyGo
when installed. To train in the browser instead,
use `wasm/build.sh 20`. Browser execution uses the VM compiled to WebAssembly.
The local server supplies the cross-origin isolation headers the runtime needs.

## The one algorithm change

Backward walks a tape instead of finding a topological order with a DFS.
Each operation records its result when it is created. Children already exist
before their parent, so creation order is topological; walking the tape
backwards applies the chain rule in the required order. This removes the
visited set and traversal without changing the model or its derivatives.
Gradient accumulation order can differ, so agreement is to rounding.

Sampling uses the same model functions on plain doubles. It needs no gradient
graph. The numeric branches make inference faster but add overhead to training.

## Check the numerical agreement

```sh
python3 -I verify.py --lg /path/to/lg
```

This checks three Adam steps using identical initial weights and the documents
`emma`, `karpathy`, and `anria`, while preserving the full dataset's vocabulary.
It compares all 4,192 final parameter values against the pinned Python
reference with tape backward, with a maximum absolute tolerance of `1e-12`.
Temporary copies leave the source, reference, and committed weights intact.
On the verification run, the maximum absolute difference was `1.11e-16`.
This is a focused parity check, not a claim of identical training trajectories
with the two languages' different RNGs.

## Measure training

```sh
python3 -I benchmark.py --lg /path/to/lg --steps 100 --reps 3 --output /tmp/microgpt-benchmark.json
```

Build the native binary first. The command rotates native, VM, and CPython tape
runs, one trainer at a time. Each program's training timer excludes sampling.
Report ms/token: the two RNGs shuffle documents differently, so per-step times
do not describe the same workload. Token normalization still leaves differences
in document lengths and initialization; this is a workload comparison, not
a controlled comparison of language implementations.

Measured on an Apple M2, macOS, CPython 3.14.7, and let-go source
`bb3606367da36d59efb0803e06bf6ac957150dd2`, built locally. The training
implementation is from `b1b156e`; the port's algorithm is unchanged by the
documentation and profiling fixes. Three interleaved repetitions of 100 steps:

| Training | Median ms/token | Range ms/token | Tokens/run |
|---|---:|---:|---:|
| CPython, tape backward | 7.06 | 7.00–7.14 | 744 |
| let-go, native | 13.49 | 13.42–14.18 | 715 |
| let-go, VM | 16.30 | 15.86–16.39 | 715 |

These short local runs put native training at about 1.9× CPython's time per
token on this machine, which was in normal use.
[Raw runs and tool versions](benchmark-results.json) are included.

The earlier Linux measurements are retained as [historical results](NOTES.md#historical-results-before-plain-number-inference).
They predate the inference branches and do not establish current CPython parity.

Compiler workarounds, profiling commands, browser measurements, and alternative
implementations are collected in [implementation notes](NOTES.md). The
[Mandelbrot experiment](../aot) is the pure-float counterpart to this
allocation-heavy workload.
