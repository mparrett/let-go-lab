#!/usr/bin/env python3
"""Check full native demos against the VM: frame counts, PNG bytes, GPT weights.

Usage: python3 test/native_demo_test.py --lg /path/to/pinned-let-go/bin/lg
       --letgo-src /path/to/pinned-let-go
Build mandelbrot, pathtrace, and microgpt with scripts/build-native.sh first.
"""
import argparse
import os
from pathlib import Path
import re
import subprocess
import tempfile

LAB = Path(__file__).resolve().parent.parent


def run(command, cwd):
    return subprocess.check_output(list(map(str, command)), cwd=cwd, text=True,
                                   stdin=subprocess.DEVNULL, timeout=300)


def frames(output):
    # Golden home view: 160×120 grid, scale 3, maxiter 96; normal 20-frame bench.
    rows = re.findall(r"frame (\d+)\s+compute=\d+ms\s+encode=\d+ms\s+"
                      r"iters=(\d+).*bytes=(\d+)", output)
    assert rows == [(str(i), "450584", "27695") for i in range(20)], output
    return rows


def samples(output):
    # First eight samples from committed weights.txt with the demo's default seed.
    names = re.findall(r"sample\s+\d+: (\w*)", output)
    assert len(names) == 20, output
    assert names[:8] == ["anria", "aliia", "kirli", "keson", "denan", "amayan",
                         "kashi", "oranag"], names
    return names


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--lg", required=True, type=Path)
    parser.add_argument("--letgo-src", required=True, type=Path)
    args = parser.parse_args()
    if not __debug__:
        parser.error("run without -O/PYTHONOPTIMIZE so parity assertions execute")
    lg = args.lg.resolve()
    source_checkout = args.letgo_src.resolve()
    native = LAB / "dist/native"
    with tempfile.TemporaryDirectory(prefix="native-demo-parity-") as directory:
        temporary = Path(directory)
        vm_frames = run([lg, LAB / "demos/mandelbrot/mandelbrot.lg"], temporary)
        native_frames = run([native / "mandelbrot"], temporary)
        assert frames(vm_frames) == frames(native_frames)
        print("PASS: Mandelbrot: 20 frames, matching iterations and encoded lengths", flush=True)

        # Compare actual sixel bytes too: equal iteration sums can hide pixel changes.
        # Keep the demo functions intact, replacing only its interactive entry point.
        source = (LAB / "demos/mandelbrot/mandelbrot.lg").read_text()
        assert source.count("(defn -main []") == 1
        source = source.partition("(defn -main []")[0] + """
(defn -main []
  (term/write (build-frame
    (first (compute-grid home-x home-y home-w home-maxiter)) home-scale)))
(when-not *compiling-aot* (-main))
"""
        frame_script = temporary / "frame.lg"
        frame_script.write_text(source)
        frame_binary = temporary / "frame-native"
        subprocess.run([lg, "compile", "-o", frame_binary, frame_script], cwd=LAB,
                       env={**os.environ, "LETGO_SRC": str(source_checkout)},
                       stdout=subprocess.PIPE, check=True, timeout=300)
        vm_sixel = subprocess.check_output([lg, frame_script], cwd=temporary, timeout=300)
        native_sixel = subprocess.check_output([frame_binary], cwd=temporary, timeout=300)
        assert vm_sixel.startswith(b"\x1bPq") and len(vm_sixel) == 27695
        assert vm_sixel == native_sixel, "native Mandelbrot sixel differs from VM"
        print("PASS: Mandelbrot: byte-identical sixel frame", flush=True)

        pngs = []
        for label, command in [("vm", [lg, LAB / "demos/pathtrace/pathtrace.lg"]),
                               ("native", [native / "pathtrace"])]:
            cwd = temporary / label
            cwd.mkdir()
            output = run(command, cwd)
            assert "wrote out.png" in output, output
            assert len(re.findall(r"^pass \d+", output, re.MULTILINE)) == 32, output
            pngs.append((cwd / "out.png").read_bytes())
        assert pngs[0].startswith(b"\x89PNG\r\n\x1a\n")
        assert pngs[0] == pngs[1], "native pathtrace PNG differs from VM"
        print("PASS: pathtrace: 32 samples/pixel, byte-identical PNG", flush=True)

        microgpt = LAB / "experiments/microgpt"
        vm = [lg, microgpt / "microgpt.lg"]
        compiled = [native / "microgpt"]
        assert samples(run(vm + ["0", "weights.txt"], microgpt)) == samples(
            run(compiled + ["0", "weights.txt"], microgpt))
        weights = []
        for label, command in [("vm", vm), ("native", compiled)]:
            saved = temporary / f"{label}-weights.txt"
            run(command + ["3", saved], microgpt)
            weights.append(list(map(float, saved.read_text().split())))
        assert len(weights[0]) == len(weights[1]) == 4192
        difference = max(abs(a - b) for a, b in zip(*weights))
        assert difference <= 1e-12, difference
        print(f"PASS: microgpt: matching samples; 3 training steps, 4192 weights; "
              f"max difference {difference:.3g}", flush=True)


if __name__ == "__main__":
    main()
