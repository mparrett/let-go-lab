"""Interleave native, VM, and CPython tape training; report training ms/token.

Run fetch-reference.sh and aot/build.sh first. Sampling is excluded by each
program's training timer. Python's token count replays its seeded shuffle.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import statistics
import subprocess
import sys
import tempfile


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--lg", default="lg")
    parser.add_argument("--native", default="aot/microgpt-native")
    parser.add_argument("--steps", type=int, default=100)
    parser.add_argument("--reps", type=int, default=3)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    if args.steps <= 0 or args.reps < 2:
        parser.error("use positive steps and at least two repetitions")
    here = Path(__file__).resolve().parent
    native = str((here / args.native).resolve())
    tokens = int(subprocess.check_output(
        [sys.executable, "-I", "count_tokens.py", str(args.steps)], cwd=here, text=True))
    results = {name: [] for name in ("native", "vm", "python-tape")}
    with tempfile.TemporaryDirectory(prefix="microgpt-bench-") as directory:
        script = Path(directory) / "microgpt_tape.py"
        source = (here / "microgpt_tape.py").read_text()
        assert source.count("{time.time()-t0:.1f}") == 1, "reference timer changed"
        script.write_text(source.replace("{time.time()-t0:.1f}", "{time.time()-t0:.6f}"))
        commands = {"native": [native, str(args.steps)],
                    "vm": [args.lg, "microgpt.lg", str(args.steps)],
                    "python-tape": [sys.executable, "-I", str(script), str(args.steps)]}
        names = list(commands)
        for repetition in range(args.reps):
            # Rotate order, with only one trainer running at a time.
            order = names[repetition % len(names):] + names[:repetition % len(names)]
            for name in order:
                output = subprocess.check_output(commands[name], cwd=here, text=True,
                                                 env={**os.environ, "MICROGPT_TAPE": "1"})
                if name == "python-tape":
                    seconds = float(re.search(r"train time: ([0-9.]+)s", output)[1])
                    value = seconds * 1000 / tokens
                    count = tokens
                else:
                    match = re.search(r"(\d+) tokens \| ([0-9.]+) ms/token", output)
                    assert match, "training summary missing: " + name
                    count, value = int(match[1]), float(match[2])
                results[name].append({"tokens": count, "ms_per_token": value})
                print(f"rep {repetition + 1}: {name}: {value:.2f} ms/token ({count} tokens)", flush=True)
    report = {"platform": platform.platform(), "python": platform.python_version(),
              "lg": subprocess.check_output([args.lg, "--version"], text=True).strip(),
              "source_sha256": hashlib.sha256((here / "microgpt.lg").read_bytes()).hexdigest(),
              "steps": args.steps, "reps": args.reps, "runs": results}
    for name, runs in results.items():
        values = [run["ms_per_token"] for run in runs]
        print(f"{name}: median {statistics.median(values):.2f}; range {min(values):.2f}–{max(values):.2f} ms/token")
    if args.output:
        args.output.write_text(json.dumps(report, indent=2) + "\n")


if __name__ == "__main__":
    main()
