#!/usr/bin/env python3
"""Regression for #6, on the canvas page: the frame must fit the viewport (no
clipping, no horizontal scroll) and taps must map to the right point of the
complex plane, at a narrow (390px) and a desktop viewport.

In the browser the mandelbrot demo draws to a <canvas> (surface/present) and
reports each frame's view through js/emit; the page keeps the latest report (centre x/y and view width vw, among others) in
window.__mbStatus and counts frames in window.__mbFrames. For each viewport this
asserts:
  - no horizontal overflow, and the whole canvas is on screen,
  - no console/page errors,
  - a centre tap zooms in and keeps x within about one grid cell,
  - an off-centre tap pans (x decreases for a tap left of centre) without zooming,
  - shift-click zooms out, the wheel zooms in, and "+" and the Zoom out button work.

Needs a running server: pass the URL as argv[1] (default http://localhost:8250/).
Requires the playwright pip package and a chromium install
(PLAYWRIGHT_BROWSERS_PATH); exits 77 when playwright isn't importable.
"""
import sys

URL = sys.argv[1] if len(sys.argv) > 1 else "http://localhost:8250/"
# A frame takes seconds under headless/CI load, so everything polls with generous
# budgets rather than sleeping.
FRAME_TIMEOUT = 60000
SKIP_EXIT = 77   # conventional "skipped"; ci.sh maps it to skip (lenient) / fail (strict)
HOME_W = 3.2     # home-w in mandelbrot.lg
CW = 160         # grid width in mandelbrot.lg


def frames(pg):
    return pg.evaluate("() => window.__mbFrames || 0")


def status(pg):
    return pg.evaluate("() => window.__mbStatus || null")


def wait_frame(pg, after, timeout=FRAME_TIMEOUT):
    """Block until a frame newer than `after` has been presented; return its status."""
    waited = 0
    while waited <= timeout:
        if frames(pg) > after:
            return status(pg)
        pg.wait_for_timeout(250)
        waited += 250
    return None


def canvas_box(pg):
    return pg.evaluate(
        "() => {const r=document.getElementById('mb-canvas').getBoundingClientRect();"
        "return {x:r.x, y:r.y, w:r.width, h:r.height, right:r.right, bottom:r.bottom,"
        " vpW:window.innerWidth, vpH:window.innerHeight,"
        " scrollW:document.documentElement.scrollWidth};}")


def tap(pg, fx, fy, shift=False):
    """Tap the canvas at fractions (fx, fy) of its box; return the next frame's status."""
    b = canvas_box(pg)
    n = frames(pg)
    if shift:
        pg.keyboard.down("Shift")
    pg.mouse.click(b["x"] + b["w"] * fx, b["y"] + b["h"] * fy)
    if shift:
        pg.keyboard.up("Shift")
    return wait_frame(pg, n)


def check_viewport(p, tag, w, h):
    fails = []
    browser = p.chromium.launch()
    pg = browser.new_context(viewport={"width": w, "height": h}).new_page()
    errs = []
    pg.on("pageerror", lambda e: errs.append(str(e)))
    pg.on("console", lambda m: errs.append(m.text) if m.type == "error" else None)
    pg.goto(URL, wait_until="domcontentloaded", timeout=30000)
    home = wait_frame(pg, 0)
    if not home:
        print(f"FAIL {tag}: no frame within {FRAME_TIMEOUT // 1000}s")
        browser.close()
        return False

    b = canvas_box(pg)
    if b["scrollW"] > b["vpW"] + 1:
        fails.append(f"{tag}: page is {b['scrollW']}px wide in a {b['vpW']}px viewport (horizontal scroll)")
    if b["right"] > b["vpW"] + 1 or b["bottom"] > b["vpH"] + 1 or b["x"] < -1 or b["y"] < -1:
        fails.append(f"{tag}: canvas {b['w']:.0f}x{b['h']:.0f} at ({b['x']:.0f},{b['y']:.0f}) "
                     f"is not fully inside the {b['vpW']}x{b['vpH']} viewport")

    tol = HOME_W / CW * 1.5   # about one grid cell of complex width at the home view
    c = tap(pg, 0.5, 0.5)
    if not c:
        fails.append(f"{tag}: no frame after the centre tap")
    else:
        if abs(c["x"] - home["x"]) > tol:
            fails.append(f"{tag}: centre tap moved x {home['x']}->{c['x']} (> {tol:.3f})")
        if not c["vw"] < home["vw"]:
            fails.append(f"{tag}: centre tap did not zoom in (view width {home['vw']}->{c['vw']})")

    n = frames(pg)
    pg.keyboard.press("r")
    reset = wait_frame(pg, n)
    left = tap(pg, 0.2, 0.5)
    if reset and left:
        if not left["x"] < reset["x"]:
            fails.append(f"{tag}: tap left of centre did not decrease x ({reset['x']}->{left['x']})")
        if left["vw"] != reset["vw"]:
            fails.append(f"{tag}: off-centre tap zoomed (view width {reset['vw']}->{left['vw']}); it should only pan")
    else:
        fails.append(f"{tag}: no frame after reset / left tap")

    before = status(pg)
    out = tap(pg, 0.5, 0.5, shift=True)
    if not out or not out["vw"] > before["vw"]:
        fails.append(f"{tag}: shift-click did not zoom out")

    before = status(pg)
    n = frames(pg)
    bx = canvas_box(pg)
    pg.mouse.move(bx["x"] + bx["w"] / 2, bx["y"] + bx["h"] / 2)
    pg.mouse.wheel(0, -120)
    wheel = wait_frame(pg, n)
    if not wheel or not wheel["vw"] < before["vw"]:
        fails.append(f"{tag}: wheel up did not zoom in")

    before = status(pg)
    n = frames(pg)
    pg.keyboard.press("+")
    plus = wait_frame(pg, n)
    if not plus or not plus["vw"] < before["vw"]:
        fails.append(f"{tag}: '+' did not zoom in")

    before = status(pg)
    n = frames(pg)
    pg.get_by_role("button", name="Zoom out").click()
    minus = wait_frame(pg, n)
    if not minus or not minus["vw"] > before["vw"]:
        fails.append(f"{tag}: the Zoom out button did not zoom out")

    if errs:
        fails.append(f"{tag}: console/page errors {errs[:4]}")
    browser.close()
    for f in fails:
        print("FAIL", f)
    if not fails:
        print(f"PASS {tag}: canvas {b['w']:.0f}x{b['h']:.0f} inside {b['vpW']}x{b['vpH']}, "
              f"taps, shift-click, wheel, key and button all map correctly")
    return not fails


def main():
    try:
        from playwright.sync_api import sync_playwright
    except ImportError:
        print("SKIP: playwright not installed")
        return SKIP_EXIT
    ok = True
    with sync_playwright() as p:
        ok &= check_viewport(p, "phone-390", 390, 780)
        ok &= check_viewport(p, "desktop", 1280, 900)
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
