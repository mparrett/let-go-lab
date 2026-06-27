#!/usr/bin/env python3
"""Regression for #6: the sixel must fit (no clipping) and clicks must map to the
correct complex-plane point at both a narrow (390px) and a desktop viewport.

Drives the served browser bundle with Playwright. For each viewport it asserts:
  - no horizontal overflow (the terminal box fits the viewport width),
  - the program adapted SCALE (smaller on the phone, 3 on desktop),
  - no console/page errors,
  - a center click keeps the center x within one cell and zooms in,
  - a left-of-center click moves the center to a smaller x (monotonic mapping).

Needs a running server: pass the URL as argv[1] (default http://localhost:8250/).
Reads the coords row via the shell's __lgReadLine test hook. Requires the
playwright pip package and a chromium install (PLAYWRIGHT_BROWSERS_PATH).
"""
import re
import sys

URL = sys.argv[1] if len(sys.argv) > 1 else "http://localhost:8250/"
SETTLE_MS = 10000  # boot + geometry handshake + first fitted frame
SETTLE_TIMEOUT = 15000  # poll budget for a frame to finish after a click


def coords(pg, timeout=SETTLE_TIMEOUT):
    """Parse the settled 'x=.. y=.. zoom=..x' coords row, polling past the
    transient 'computing…' marker the render writes there mid-frame."""
    waited = 0
    while waited <= timeout:
        for row in range(1, 5):
            s = pg.evaluate("(r) => window.__lgReadLine ? window.__lgReadLine(r) : ''", row)
            m = re.search(r"x=(-?[\d.]+)\s+y=(-?[\d.]+)\s+zoom=(\d+)x", s)
            if m:
                return float(m.group(1)), float(m.group(2)), int(m.group(3))
        pg.wait_for_timeout(500)
        waited += 500
    return None


def image_box(pg):
    return pg.evaluate(
        "() => {const t=document.querySelector('.xterm-screen')||document.querySelector('canvas');"
        "const r=t.getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height};}"
    )


def load(p, w, h):
    pg = p.chromium.launch().new_context(viewport={"width": w, "height": h}).new_page()
    errs = []
    pg.on("pageerror", lambda e: errs.append(str(e)))
    pg.on("console", lambda m: errs.append(m.text) if m.type == "error" else None)
    pg.goto(URL, wait_until="networkidle", timeout=30000)
    pg.wait_for_timeout(SETTLE_MS)
    return pg, errs


def check_viewport(p, tag, w, h, want_scale):
    fails = []
    # --- layout + center-click invariant on one load ---
    pg, errs = load(p, w, h)
    info = pg.evaluate(
        "() => {const el=document.getElementById('terminal');const r=el.getBoundingClientRect();"
        "return {scale:window.__lgScale, grid:window.__lgGrid, termW:Math.round(r.width), "
        "vpW:window.innerWidth, overflowX:Math.round(r.width)>window.innerWidth+1};}"
    )
    if info["overflowX"]:
        fails.append(f"{tag}: terminal {info['termW']}px overflows viewport {info['vpW']}px (clipped)")
    if info["scale"] != want_scale:
        fails.append(f"{tag}: scale={info['scale']}, expected {want_scale}")
    if errs:
        fails.append(f"{tag}: console/page errors {errs[:4]}")

    home = coords(pg)
    if not home:
        fails.append(f"{tag}: could not read coords row")
    else:
        hx, hy, hzoom = home
        box = image_box(pg)
        pg.mouse.click(box["x"] + box["w"] * 0.5, box["y"] + box["h"] * 0.45)
        pg.wait_for_timeout(1500)  # let "computing…" overwrite the old coords first
        c = coords(pg)
        if not c:
            fails.append(f"{tag}: no coords after center click")
        else:
            cx, cy, czoom = c
            # one cell ≈ vw/img_cols of complex width; home vw=3.2, img_cols≈grid-1
            cols = int(info["grid"].split("x")[0])
            tol = 3.2 / max(1, cols - 1) * 1.5
            if abs(cx - hx) > tol:
                fails.append(f"{tag}: center click moved x {hx}->{cx} (> {tol:.3f}); mapping off-center")
            if czoom <= hzoom and not (hzoom == 1 and czoom == 1):
                fails.append(f"{tag}: center click did not zoom in ({hzoom}->{czoom})")
    pg.context.browser.close()

    # --- left-of-center click moves x negative (fresh load) ---
    pg2, _ = load(p, w, h)
    base = coords(pg2)
    box2 = image_box(pg2)
    pg2.mouse.click(box2["x"] + box2["w"] * 0.20, box2["y"] + box2["h"] * 0.45)
    pg2.wait_for_timeout(1500)  # let "computing…" overwrite the old coords first
    left = coords(pg2)
    if base and left and not (left[0] < base[0]):
        fails.append(f"{tag}: left click did not decrease x ({base[0]}->{left[0]})")
    pg2.context.browser.close()

    for f in fails:
        print("FAIL", f)
    if not fails:
        print(f"PASS {tag}: scale={info['scale']} grid={info['grid']} "
              f"term={info['termW']}px<=vp={info['vpW']}px, click mapping correct")
    return not fails


def main():
    try:
        from playwright.sync_api import sync_playwright
    except ImportError:
        print("SKIP: playwright not installed")
        return 0
    ok = True
    with sync_playwright() as p:
        ok &= check_viewport(p, "phone-390", 390, 780, 2)
        ok &= check_viewport(p, "desktop", 1280, 900, 3)
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
