"""Hash the mandelbrot canvas of each page, to compare two builds' output.

Usage: pixels.py URL [URL...]
"""
import sys, hashlib
from playwright.sync_api import sync_playwright
with sync_playwright() as p:
    b = p.chromium.launch()
    for url in sys.argv[1:]:
        pg = b.new_page(viewport={"width":1280,"height":900})
        pg.goto(url, wait_until="networkidle", timeout=60000)
        pg.wait_for_function("() => window.__mbStatus && window.__mbStatus.compute !== undefined", timeout=90000)
        pg.wait_for_timeout(500)
        data = pg.evaluate("""() => { const c = document.querySelector('canvas');
          const x = document.createElement('canvas'); x.width = c.width; x.height = c.height;
          const g = x.getContext('2d'); g.drawImage(c, 0, 0);
          return Array.from(g.getImageData(0,0,x.width,x.height).data).join(','); }""")
        dims = pg.evaluate("() => { const c=document.querySelector('canvas'); return [c.width,c.height]; }")
        print(url, dims, hashlib.sha256(data.encode()).hexdigest()[:16], len(set(data.split(','))), "distinct byte values")
        pg.close()
    b.close()
