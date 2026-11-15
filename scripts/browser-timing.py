#!/usr/bin/env python3
"""Collect a demo's own per-frame timings from its browser page.

Every demo prints a timing line after each frame or pass:
  mandelbrot  "compute N ms · emit M ms"
  pathtrace   "compute N ms · encode M ms /pass"
  ptcanvas    "compute N ms · present M ms /pass"
This loads a served page, polls document.body.innerText (xterm's DOM renderer
puts the terminal rows there; ptcanvas has a DOM footer), and records each new
timing line until the time is up. mandelbrot draws one frame per key, so
--rekey presses "r" after every frame to keep it drawing.

  python3 scripts/browser-timing.py URL [--seconds 60] [--rekey]

Prints one JSON object: the URL, whether the page was cross-origin isolated,
page errors, and the frames with their timings and medians. Needs a server
with COOP/COEP (scripts/serve.py) and the playwright pip package; exits 77
when playwright isn't importable.
"""
import argparse
import json
import re
import statistics
import sys
import time

try:
    from playwright.sync_api import sync_playwright
except ImportError:
    print("browser-timing: playwright not installed", file=sys.stderr)
    sys.exit(77)

TIMING = re.compile(r"compute (\d+)ms · (emit|encode|present) (\d+)ms")


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("url")
    ap.add_argument("--seconds", type=float, default=60)
    ap.add_argument("--rekey", action="store_true", help='press "r" after each frame')
    a = ap.parse_args()

    frames, errors, last = [], [], None
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page(viewport={"width": 1280, "height": 900})
        page.on("pageerror", lambda e: errors.append(str(e)))
        t0 = time.monotonic()
        page.goto(a.url, wait_until="domcontentloaded")
        while time.monotonic() - t0 < a.seconds:
            page.wait_for_timeout(500)
            m = TIMING.search(page.evaluate("document.body.innerText"))
            if m and m.group(0) != last:
                last = m.group(0)
                frames.append({"t": round(time.monotonic() - t0, 1),
                               "compute": int(m.group(1)), m.group(2): int(m.group(3))})
                if a.rekey:
                    page.evaluate("document.querySelector('.xterm-helper-textarea')?.focus()")
                    page.keyboard.press("r")
        isolated = page.evaluate("self.crossOriginIsolated")
        browser.close()

    medians = {}
    if frames:
        for k in frames[0]:
            if k != "t":
                medians[k] = statistics.median(f[k] for f in frames)
    print(json.dumps({"url": a.url, "isolated": isolated, "errors": errors[:3],
                      "frames": frames, "median_ms": medians}))
    return 0 if frames and not errors else 1


if __name__ == "__main__":
    sys.exit(main())
