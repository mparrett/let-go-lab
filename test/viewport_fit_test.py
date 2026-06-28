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
# Renders are slow (compute + band-streamed emit ≈ 10s under headless/CI load), so
# everything is poll-based with generous budgets rather than fixed sleeps.
SETTLE_TIMEOUT = 40000   # a frame to finish (bottom status written) — boot or post-click
RACE_TIMEOUT = 150000    # the resize race converges through ~3 sequential renders


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


def read_scale(pg):
    """The scale the program last rendered at — ground truth via the OSC 5003
    signal the program emits each render (window.__lgRenderedScale), recorded by
    the shell. Distinct from window.__lgScale (what the shell asked for). Robust
    to terminal scrolling, unlike scraping the relative-positioned bottom status."""
    return pg.evaluate("() => (window.__lgRenderedScale ?? null)")


def wait_settled(pg, timeout=SETTLE_TIMEOUT):
    """Block until a frame has fully rendered (the bottom status row exists), i.e.
    the program is parked at read-key with a free input slot. Returns the rendered
    scale, or None on timeout."""
    waited = 0
    while waited <= timeout:
        s = read_scale(pg)
        if s is not None:
            return s
        pg.wait_for_timeout(500)
        waited += 500
    return None


def new_page(p, w, h):
    """Launch a chromium page at the given viewport with error capture wired and
    the document loaded. Returns (page, errs); errs accumulates pageerror +
    console-error text for the caller to assert on."""
    pg = p.chromium.launch().new_context(viewport={"width": w, "height": h}).new_page()
    errs = []
    pg.on("pageerror", lambda e: errs.append(str(e)))
    pg.on("console", lambda m: errs.append(m.text) if m.type == "error" else None)
    pg.goto(URL, wait_until="networkidle", timeout=30000)
    return pg, errs


def load(p, w, h):
    pg, errs = new_page(p, w, h)
    wait_settled(pg)   # first fitted frame done before any interaction
    return pg, errs


def coords_after(pg, before, timeout=SETTLE_TIMEOUT):
    """Poll the coords row until it settles to a value DIFFERENT from `before`
    (the post-input frame landed), or timeout. Replaces a fixed sleep that waited
    for the transient 'computing…' marker to clear — adapts to render duration
    instead of guessing it."""
    waited = 0
    while waited <= timeout:
        c = coords(pg, timeout=2000)
        if c and c != before:
            return c
        pg.wait_for_timeout(300)
        waited += 300
    return coords(pg)


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
        c = coords_after(pg, home)   # poll for the post-click frame to land
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
    left = coords_after(pg2, base)   # poll for the post-click frame to land
    if base and left and not (left[0] < base[0]):
        fails.append(f"{tag}: left click did not decrease x ({base[0]}->{left[0]})")
    pg2.context.browser.close()

    for f in fails:
        print("FAIL", f)
    if not fails:
        print(f"PASS {tag}: scale={info['scale']} grid={info['grid']} "
              f"term={info['termW']}px<=vp={info['vpW']}px, click mapping correct")
    return not fails


def check_resize_race(p):
    """Regression for the dropped-scale handshake (PR #13 review): a resize whose
    'S<n>' send is dropped (1-slot key ring busy) must still converge — the
    program's rendered scale must eventually match window.__lgScale.

    Reproduce the drop: start a render, queue an unrelated key so the slot is
    occupied, then resize. The resize's S2 is refused; only the retry path makes
    it land. The buggy version (ack-on-send) never retries and stays mismatched.
    """
    fails = []
    pg, errs = new_page(p, 1280, 900)
    start = wait_settled(pg)   # parked at read-key, slot free
    if start != 3:
        fails.append(f"race: expected initial rendered scale 3, got {start}")

    # Observe the shell's own sendInput calls without interfering (don't send S2
    # ourselves — that could deliver it and mask a broken retry).
    pg.evaluate("""() => {
      window.__lgSends = [];
      const orig = window.LetGoHost.sendInput.bind(window.LetGoHost);
      window.LetGoHost.sendInput = (s) => { const r = orig(s); window.__lgSends.push([s, r]); return r; };
    }""")

    # Start a render (program reads '+' immediately since it's parked), then fill
    # the 1-slot key ring with an unrelated key while it's busy — so the upcoming
    # resize send hits an occupied slot and is dropped.
    pg.evaluate("() => window.LetGoHost.sendInput('+')")   # consumed at once → render starts
    pg.wait_for_timeout(800)                               # ensure it's mid-render, not reading
    pg.evaluate("() => window.LetGoHost.sendInput('x')")   # occupies the slot mid-render

    # Resize narrow → shell wants scale 2; its S2 send hits the busy slot.
    pg.set_viewport_size({"width": 390, "height": 780})
    pg.evaluate("() => window.dispatchEvent(new Event('resize'))")
    pg.wait_for_timeout(400)
    want = pg.evaluate("() => window.__lgScale")

    # Converge: poll until the rendered scale matches what the shell asked for.
    # This needs ~3 sequential renders (+ , x, then S2→scale 2), hence the budget.
    converged = False
    waited = 0
    while waited <= RACE_TIMEOUT:
        if read_scale(pg) == want:
            converged = True
            break
        pg.wait_for_timeout(2000)   # poll gently so it doesn't starve the render worker
        waited += 2000

    final = read_scale(pg)
    sends = pg.evaluate("() => window.__lgSends")
    drops = sum(1 for s, r in sends if s == "S2" and r is False)   # shell's S2 refused (slot busy)
    if want != 2:
        fails.append(f"race: shell did not request scale 2 (__lgScale={want})")
    if drops == 0:
        fails.append(f"race: did not reproduce a dropped S2 send (sends={sends}); test inconclusive")
    if not converged:
        fails.append(f"race: rendered scale {final} never matched __lgScale {want} "
                     f"within {RACE_TIMEOUT // 1000}s (drops={drops}, sends={sends})")
    if errs:
        fails.append(f"race: console/page errors {errs[:4]}")
    pg.context.browser.close()

    for f in fails:
        print("FAIL", f)
    if not fails:
        print(f"PASS race: {drops} dropped S2 send(s) recovered — rendered scale "
              f"converged {start}→{final} == __lgScale {want}")
    return not fails


SKIP_EXIT = 77   # conventional "skipped"; ci.sh maps it to skip (lenient) / fail (strict)


def main():
    try:
        from playwright.sync_api import sync_playwright
    except ImportError:
        print("SKIP: playwright not installed")
        return SKIP_EXIT
    ok = True
    with sync_playwright() as p:
        ok &= check_viewport(p, "phone-390", 390, 780, 2)
        ok &= check_viewport(p, "desktop", 1280, 900, 3)
        ok &= check_resize_race(p)
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
