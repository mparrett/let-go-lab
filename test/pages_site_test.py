#!/usr/bin/env python3
"""Smoke-test the assembled Pages site at a project subpath on a header-free server.

Usage: python3 test/pages_site_test.py _site
Needs Playwright + Chromium. Checks the index (images, links, layout, the redirect
for old root start-view links) and that every demo page isolates through its own
service worker and boots without page errors. microgpt has its own deeper test
(test/microgpt_pages_test.py); here it only gets the same boot check.
"""
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import shutil
import sys
import tempfile
import threading
from urllib.request import urlopen

try:
    from playwright.sync_api import sync_playwright
except ImportError:
    print("Playwright is required for the Pages site smoke test")
    sys.exit(77)

DEMOS = ["mandelbrot", "mandelbrot-sixel", "pathtrace", "ptcanvas", "microgpt"]


class QuietHandler(SimpleHTTPRequestHandler):
    def log_message(self, *_args):
        pass

    def copyfile(self, source, outputfile):
        # The isolation bootstrap deliberately reloads the first navigation.
        try:
            super().copyfile(source, outputfile)
        except (BrokenPipeError, ConnectionResetError):
            pass


def check_index(browser, base):
    for label, viewport in [("desktop", {"width": 1280, "height": 900}),
                            ("mobile", {"width": 390, "height": 844})]:
        page = browser.new_page(viewport=viewport)
        errors = []
        page.on("pageerror", lambda error: errors.append(str(error)))
        page.goto(base, wait_until="networkidle")
        broken = page.evaluate("[...document.images].filter(i => !i.naturalWidth).map(i => i.src)")
        assert not broken, f"index images failed to load: {broken}"
        links = page.evaluate("[...document.querySelectorAll('.mosaic a')].map(a => a.href)")
        for href in [h for h in links if h.startswith(base)]:
            with urlopen(href) as response:
                assert response.status == 200, href
        assert page.evaluate("document.documentElement.scrollWidth <= innerWidth"), "horizontal overflow"
        assert not errors, errors
        page.close()
        print(f"PASS: index {label}: {len(links)} tiles, images load, local links resolve, no overflow")

    page = browser.new_page()
    page.goto(base + "?x=-0.745&y=0.11&zoom=50")
    page.wait_for_url(base + "mandelbrot/?x=-0.745&y=0.11&zoom=50")
    page.close()
    print("PASS: index redirects old root start-view links to mandelbrot/")


def check_demo(browser, base, demo):
    url = f"{base}{demo}/"
    context = browser.new_context(viewport={"width": 1280, "height": 900})
    page = context.new_page()
    errors = []
    page.on("pageerror", lambda error: errors.append(str(error)))
    page.goto(url)
    page.wait_for_function("crossOriginIsolated", timeout=60000)
    worker = page.evaluate("navigator.serviceWorker.controller.scriptURL")
    assert worker == f"{url}coi-serviceworker.js", worker
    page.wait_for_timeout(5000)   # let the program boot and draw
    assert not errors, errors
    context.close()
    print(f"PASS: {demo}: isolated by its own service worker, boots without page errors")


def main():
    source = Path(sys.argv[1] if len(sys.argv) > 1 else "_site").resolve()
    with tempfile.TemporaryDirectory(prefix="pages-site-") as directory:
        root = Path(directory)
        shutil.copytree(source, root / "let-go-lab")
        server = ThreadingHTTPServer(("127.0.0.1", 0), partial(QuietHandler, directory=str(root)))
        threading.Thread(target=server.serve_forever, daemon=True).start()
        base = f"http://localhost:{server.server_port}/let-go-lab/"
        try:
            with urlopen(base) as response:
                assert not response.headers.get("Cross-Origin-Opener-Policy")
            with sync_playwright() as playwright:
                browser = playwright.chromium.launch(headless=True)
                try:
                    check_index(browser, base)
                    for demo in DEMOS:
                        check_demo(browser, base, demo)
                finally:
                    browser.close()
        finally:
            server.shutdown()
            server.server_close()


if __name__ == "__main__":
    main()
