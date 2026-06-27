#!/usr/bin/env python3
"""Regression for #7: loop-keys must exit when term/read-key returns EOF.

Runs the mandelbrot demo with a PTY for stdout (so term/size is non-nil and
-main enters the interactive loop, not the headless bench) while stdin is at EOF
(/dev/null). With the fix the program renders one frame, reads nil, and exits;
without it, nil falls through to the default recurrence and the loop re-renders
forever. We assert the process exits cleanly within a deadline.

Standalone: `python3 test/eof_exit_test.py` (LG=/path/to/lg to override).
"""
import os
import pty
import select
import subprocess
import sys
import time

LAB = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
LG = os.environ.get("LG", os.path.join(LAB, "let-go", "lg"))
LGFILE = os.path.join(LAB, "demos", "mandelbrot", "mandelbrot.lg")
DEADLINE = float(os.environ.get("EOF_DEADLINE", "20"))


def main():
    if not os.access(LG, os.X_OK):
        print(f"SKIP: no lg at {LG} (set LG=<path>)")
        return 0

    master, slave = pty.openpty()
    proc = subprocess.Popen(
        [LG, LGFILE], stdin=subprocess.DEVNULL,
        stdout=slave, stderr=slave, close_fds=True,
    )
    os.close(slave)

    start = time.time()
    deadline = start + DEADLINE
    # Drain the pty so the child never blocks writing the (large) sixel frame.
    while proc.poll() is None:
        if time.time() > deadline:
            proc.kill()
            proc.wait()
            os.close(master)
            print(f"FAIL: still running after {DEADLINE:.0f}s — EOF redraw loop")
            return 1
        r, _, _ = select.select([master], [], [], 0.2)
        if r:
            try:
                os.read(master, 65536)
            except OSError:  # EIO once the slave side closes
                break
    proc.wait()
    os.close(master)

    elapsed = time.time() - start
    if proc.returncode == 0:
        print(f"PASS: exited cleanly on EOF in {elapsed:.1f}s")
        return 0
    print(f"FAIL: exited with rc={proc.returncode}")
    return 1


if __name__ == "__main__":
    sys.exit(main())
