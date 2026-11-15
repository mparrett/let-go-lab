"""Median per-frame compute of a mandelbrot canvas page over N re-renders.

Usage: bench.py URL N. Reads window.__mbStatus, which the page's shell sets
from each status the program emits; 'r' re-renders the home view.
"""
import sys, time, statistics
from playwright.sync_api import sync_playwright
url, n = sys.argv[1], int(sys.argv[2])
with sync_playwright() as p:
    b = p.chromium.launch(); pg = b.new_page(viewport={"width":1280,"height":900})
    errs=[]; pg.on("pageerror", lambda e: errs.append(str(e)))
    pg.goto(url, wait_until="networkidle", timeout=60000)
    pg.wait_for_function("() => window.__mbStatus && window.__mbStatus.compute !== undefined", timeout=90000)
    first = pg.evaluate("() => window.__mbStatus")
    seen=[]
    for i in range(n):
        pg.evaluate("() => { window.__mbStatus = null; window.LetGoHost.sendInput('r'); }")
        pg.wait_for_function("() => window.__mbStatus && window.__mbStatus.compute !== undefined", timeout=60000)
        seen.append(pg.evaluate("() => window.__mbStatus.compute"))
    st = pg.evaluate("() => window.__mbStatus")
    print(f"first compute={first['compute']}ms iters={first['iters']}; {n} re-renders: median compute={statistics.median(seen)}ms min={min(seen)} max={max(seen)}; iters={st['iters']}; errors={errs[:2]}")
    b.close()
