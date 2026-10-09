"""Compare three Adam steps with Python using identical documents and weights.

Run fetch-reference.sh first, then: python3 -I verify.py --lg /path/to/lg
Temporary copies leave the port, fetched reference, and trained weights intact.
"""
import argparse
import os
from pathlib import Path
import subprocess
import sys
import tempfile


def replace_once(text, old, new):
    assert text.count(old) == 1, "source shape changed: " + old
    return text.replace(old, new)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--lg", default="lg")
    args = parser.parse_args()
    here = Path(__file__).resolve().parent
    docs = ["emma", "karpathy", "anria"]
    with tempfile.TemporaryDirectory(prefix="microgpt-parity-") as directory:
        tmp = Path(directory)
        lg_weights, py_weights = tmp / "lg-weights.txt", tmp / "py-weights.txt"
        source = (here / "microgpt.lg").read_text()
        # Keep the full dataset's vocabulary; only replace the training docs.
        source = replace_once(source, "(when-not *compiling-aot* (-main))",
                              '(def docs ["emma" "karpathy" "anria"])\n'
                              '(load-weights! "weights.txt")\n(train! 3)\n'
                              f'(save-weights! "{lg_weights.as_posix()}")')
        lg_script = tmp / "parity.lg"
        lg_script.write_text(source)
        source = (here / "microgpt_tape.py").read_text()
        source = replace_once(source, "# Repeat in sequence",
                              f"docs = {docs!r}\n"
                              "for p, w in zip(params, map(float, open('weights.txt'))):\n"
                              "    p.data = w\n\n# Repeat in sequence")
        source = replace_once(source,
                              "num_steps = int(sys.argv[1]) if len(sys.argv) > 1 else 1000",
                              "num_steps = 3")
        # Training only; write every parameter after the final Adam update.
        source, marker, _ = source.partition("# Inference:")
        assert marker, "reference inference marker changed"
        source += (f"\nwith open({str(py_weights)!r}, 'w') as out:\n"
                   "    out.write('\\n'.join(str(p.data) for p in params))\n")
        py_script = tmp / "parity.py"
        py_script.write_text(source)
        subprocess.run([args.lg, str(lg_script)], cwd=here, check=True)
        subprocess.run([sys.executable, "-I", str(py_script)], cwd=here, check=True,
                       env={**os.environ, "MICROGPT_TAPE": "1"})
        actual = list(map(float, lg_weights.read_text().split()))
        expected = list(map(float, py_weights.read_text().split()))
        assert len(actual) == len(expected) == 4192, "parameter count changed"
        difference = max(abs(a - b) for a, b in zip(actual, expected))
        assert difference <= 1e-12, f"parameter difference: {difference:g}"
        print(f"PASS: 3 Adam steps, {len(actual)} parameters; max difference {difference:.3g}")


if __name__ == "__main__":
    main()
