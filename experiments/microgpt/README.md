# microgpt in let-go

A port of karpathy's [microgpt](https://gist.github.com/karpathy/8627fe009c40f57531cb18360106ce95)
(rev `14fb0388`): a ~200-line, dependency-free GPT trainer with a scalar autograd
engine, trained on 32k names. The let-go version builds the same model and the
same graph, node for node, and runs on the bytecode VM or, through `lg compile`,
as a native Go binary.

It's an allocation-heavy workload, roughly 50k small graph nodes per training
step, which makes it a good stress test for let-go's VM and AOT lowering. The
mandelbrot AOT spike in [`experiments/aot`](../aot) is the pure-float
counterpart.

## Run

```sh
cd experiments/microgpt
./fetch-reference.sh            # input.txt, microgpt.py, microgpt_tape.py (pinned, checksummed)
lg microgpt.lg 1000             # VM; the argument is the number of training steps
aot/build.sh                    # native binary via lg compile (needs Go)
aot/microgpt-native 1000 weights.txt   # train, save the weights (~5 min; a trained copy is committed)
lg microgpt.lg 0 weights.txt    # load them and only sample (either build reads either's file)
LETGO_USE_TINYGO=1 wasm/build.sh --weights weights.txt   # browser page that samples from them
python3 ../../scripts/serve.py --dir wasm/out --headers ../../harness/serve.json
python3 -I microgpt.py          # the reference, 1000 steps
python3 -I count_tokens.py 1000 # its token count, for ms/token
```

The reference is fetched, not vendored: the gist carries no license.
`microgpt_tape.py` is the reference plus `tape.patch`, the same tape backward
in CPython (`MICROGPT_TAPE=1`), for a like-for-like comparison.

## Results

4-vCPU Linux box, CPython 3.12.3, let-go `main` at `ff1e6da` plus the #562 fix
(nooga/let-go#1044). Compare **ms/token**, not per-step times: the two RNGs
shuffle the documents differently. Every comparison was run interleaved, two
or more reps each.

| ms/token | DFS backward | tape backward |
|---|---:|---:|
| CPython 3.12 | 49.3 | 40.2 |
| let-go, native (`lg compile`) | ~62 | ~41 |
| let-go, VM | ~106 | ~66 |

`microgpt.lg` uses the tape. With it, the native binary is level with CPython
running the same algorithm. Where the native binary started, and what moved it:

| Change | Effect |
|---|---|
| First native build (protocol methods, DFS) | ~94 ms/token, ~2.5× CPython (the VM started at ~123) |
| Node mutators as fns over `set-field!` instead of protocol methods | ~15% faster |
| Typed loops in `backward!` (`peek`, an explicit `loop`) | ~32% faster |
| `^double` hints (`vpow`, `gauss`), softmax via `v+` | no change (cold paths) |
| Tape backward instead of the topo-sort DFS | ~34% faster (VM ~38%) |
| Dot products as `(vsum (map v* a b))` instead of an indexed helper | native the same; VM ~18% slower, accepted to stay close to the Python |

The tape changes the order gradients accumulate in, so results match Python to
rounding, not bit for bit. In CPython the two backward modes first differ in
the last bits at step 12 and still agree to 10 decimals at step 100.

## What the lowering taught us

- **Everything lowered, and that said little.** `lg compile` and
  [lowering-census](https://github.com/nooga/let-go/pull/1025) reported 100%
  lowered at every step, including two where the generated Go didn't build, and
  while the hot loop ran through `clojure.core` var calls.
- **Protocol methods ran as bytecode under AOT**: lowered code can't
  devirtualize a call on a node of unknown type. Plain functions over
  `set-field!` (what the `deftype` macro turns `(set! field v)` into) get a
  direct native call.
- **`dotimes` loses its counter's type**: it expands to `clojure.core/<` and
  `clojure.core/inc`, which miss the arithmetic intrinsics, on the VM too.
- **Typed parameters keep a function off its var's native path**: functions
  with typed params aren't registered as var overrides, and wrong `int64`
  inference (#551) made that bite for `vpow`, `v-` and `gauss`.

## Workarounds in the code, and when they can go

Each exists because of a let-go limitation, not because it's the natural
Clojure.

| Workaround | Because of | Revert when |
|---|---|---|
| `add-grad!`/`adam!` are fns over `set-field!`, not deftype protocol methods | under AOT a protocol call on a node of unknown type runs the method body as bytecode | protocol dispatch lowers natively |
| `backward!`'s inner loop is a hand-written `loop`, not `dotimes` | `dotimes` emits `clojure.core/<`/`inc`, which miss the arithmetic intrinsics | nooga/let-go#1045 |
| `vpow` takes `^double k`; `vdiv` passes `-1.0` | float params inferred `int64` without a hint | nooga/let-go#551 |
| `aot/patch562.py`, `aot/unbox562.go` | `math/*` results don't build into float slots | nooga/let-go#1044 merges (#562) |
| `aot/profile_hook.go` | `lg compile` binaries have no profiling flags | a let-go feature |
| every op checks `number?` so sampling can run on plain doubles; native training pays ~20% | `lg compile` doesn't see a `defn` that isn't a literal top-level form, so the `defn-twins` macro below doesn't build | nooga/let-go#1049 |
| `weights.txt` is one number per line, not EDN | `lg compile` binaries link no reader | nooga/let-go#992 |

Not workarounds: the `-main` entry (`lg compile` needs one), Box-Muller `gauss`
and the weighted `choose` (Python's stdlib has them; let-go doesn't), the ordered
`state-entries` (a Clojure map loses order past 8 entries too), and the
`string`/`io` namespace names. Avoid a local named `args` in `-main`
(nooga/let-go#1046).

## In the browser

`wasm/build.sh` builds a page with `lg -w`, which runs the bytecode VM
compiled to wasm. It comes in two modes:

- `wasm/build.sh --weights weights.txt` bakes in weights trained elsewhere,
  and the page only samples. With weights from a native 1000-step run
  (loss 2.6057), the TinyGo page prints the same 20 names as the native binary
  ("kirli", "keson", "amayan", "jaden", ...), about 9 s after load in headless
  Chromium. Sampling runs on plain doubles, not through the autograd graph (see
  "Sampling without the graph" below); through the graph it took about 20 s.
- `wasm/build.sh [steps]` trains in the page (20 steps by default) and then
  samples. Loss and samples match the VM exactly.

`LETGO_USE_TINYGO=1` (with `tinygo` on PATH; let-go's CI pins 0.42.0) builds
with TinyGo instead of stock Go, and the script then defaults
`LETGO_TINYGO_OPT` to 2. Training in headless Chromium, 20 steps, the same lg:

| build | page | ms/token |
|---|---:|---:|
| stock Go `lg -w` | 8.5 MB | 432–479 |
| TinyGo `-opt=z` (lg's default) | 2.2 MB | ~296 |
| TinyGo `-opt=2` | 2.6 MB | ~245 |
| TinyGo `-opt=2 -gc=leaking` | 2.2 MB | ~118 |
| `lg microgpt.lg`, native VM | | 86 |

`LETGO_TINYGO_GC=precise` is no different from the default collector. The
leaking GC never frees, so it is a measurement (about half of TinyGo's time is
collection), not a build to ship. None of the `lg compile` gains carry over,
because `-w` doesn't use the lowered Go. A TinyGo build takes 3.5–4.5 minutes,
against about one for stock Go.

The page needs cross-origin isolation (COOP/COEP), which `scripts/serve.py`
sets from `harness/serve.json`. `lg -w` embeds no resources and the browser
passes no arguments, so the script writes a copy of `microgpt.lg` to
`wasm/gen/` with `input.txt` inlined as a string and the entry point replaced.

The weights file is the params' data, one number per line, in params order.
`weights.txt` here is from `aot/microgpt-native 1000 weights.txt` (loss 2.6057;
`lg microgpt.lg 0 weights.txt` names "anria", "aliia", "kirli", ...).
It is plain text rather than EDN because a binary from `lg compile` links no
reader: `read-string` and `edn/read-string` are unbound there, and the call
only fails when it runs.

## Profiling the native binary

`PROFILE=1 aot/build.sh` adds a hook to the generated `main`. Then:

```sh
LG_CPUPROFILE=cpu.prof LG_MEMPROFILE=mem.prof aot/microgpt-native 40
go tool pprof -top aot/microgpt-native cpu.prof
go tool pprof -top -sample_index=alloc_space aot/microgpt-native mem.prof
```

## Sampling without the graph: the macro version, for reference

Sampling never calls `backward!`, so it doesn't need the autograd graph. The
shipped code makes every op accept plain numbers. This version writes the
model once and has a macro define a `-plain` twin of each model fn with the
graph ops swapped for arithmetic, so the training ops stay untouched:

```clojure
(defmacro defn-twins [ops & defns]
  (let [names (map second defns)
        subs (merge ops (zipmap names (map #(symbol (str % "-plain")) names)))]
    `(do ~@defns ~@(map #(walk/postwalk-replace subs %) defns))))

(defn sum [xs] (reduce + 0 xs))
(defn div [a b] (* a (math/pow b -1)))
(defn relu [x] (max 0.0 x))

(defn-twins {v+ +, v* *, vpow math/pow, vexp math/exp, vrelu relu, vdiv div, vsum sum,
             .data identity}
  (defn linear ...) (defn softmax ...) (defn rmsnorm ...) (defn gpt [sd ...] ...))
```

On the VM it samples in 0.87 s (1.04 s shipped, 5.7 s through the graph),
trains at the same speed and prints the same samples. `lg compile` rejects it
("Can't resolve gpt"): see the workarounds table.

## The DFS backward (Python's topo sort), for reference

`microgpt.lg` runs backward over a tape. An earlier version used the
faithful port of Python's `backward()`: a topo-sort DFS from the loss, then
the chain rule in reverse topological order. It reproduces Python's gradient
accumulation order exactly (checked against Python: all gradients within 1.7e-16),
which the tape does not (it matches to rounding). Kept here for that kind of
validation. It needs a `^:mutable mark` field on `Value` and these alongside:

```clojure
(defn visit! [n epoch]
  (if (== (.mark n) epoch) false (do (set-field! n 'mark epoch) true)))

(def epoch (volatile! 0))

(defn backward! [root]
  ;; Iterative post-order DFS: the graph is a few hundred levels deep and
  ;; Python's recursive build_topo would lean on the VM stack. The stack is a
  ;; list: persistent-vector conj/pop copy tails, ~52% of allocated bytes.
  (let [ep (vswap! epoch inc)
        topo (loop [stack (list [root false]) topo (transient [])]
               ;; peek, not empty?: entries are never nil, and peek lowers
               ;; to a native call where empty? stays a bytecode fn.
               (if-let [top (peek stack)]
                 (let [[n done?] top stack (pop stack)]
                   (cond
                     done? (recur stack (conj! topo n))
                     (visit! n ep)
                     (recur (reduce (fn [s c] (conj s [c false]))
                                    (conj stack [n true])
                                    (.children n))
                            topo)
                     :else (recur stack topo)))
                 (persistent! topo)))]
    (add-grad! root 1.0)
    (loop [i (dec (count topo))]
      (when (>= i 0)
        (let [n (nth topo i) g (.grad n)]
          ;; An explicit loop: dotimes here lowered its counter as vm.Value,
          ;; so < and inc went through their vars.
          (let [cs (.children n) ls (.lgrads n)
                k (count cs)]
            (loop [j 0]
              (when (< j k)
                (add-grad! (nth cs j) (* (nth ls j) g))
                (recur (inc j))))))
        (recur (dec i))))))
```

Python's recursive `build_topo` becomes an iterative post-order DFS (deep
recursion would lean on the VM stack), and its identity `set` becomes an epoch
number in `mark`. That is why the tape is the simpler code in let-go, where in
Python the DFS is the elegant one.

## An indexed dot product (VM speed), for reference

Every dot product is `(vsum (map v* a b))`, the closest form to Python's
`sum(wi * xi for wi, xi in zip(wo, x))`. On the VM, the lazy `map` costs a seq
per dot product. This helper avoids it and makes the VM ~18% faster; under AOT
it makes no difference. Left out to keep the code close to the
Python. To use it, swap `(vsum (map v* a b))` for `(vsum2 v* a b)`:

```clojure
;; sum(f(a[i], b[i]) ...) over two vectors: the same nodes as (vsum (map f a b)),
;; without a lazy seq per call.
(defn vsum2 [f a b]
  (let [n (count a)]
    (loop [i 0 acc (lift 0)]
      (if (< i n) (recur (inc i) (v+ acc (f (nth a i) (nth b i)))) acc))))
```

