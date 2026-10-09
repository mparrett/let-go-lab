"""Smoke-test the sampler at a project subpath on a header-free HTTP server.

Usage: python3 test/microgpt_pages_test.py _site/microgpt
Needs Playwright + Chromium. This exercises Pages' service-worker path.
"""
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import os
import shutil
import sys
import tempfile
import threading
from urllib.request import urlopen

try:
    from playwright.sync_api import sync_playwright
except ImportError:
    print("Playwright is required for microgpt Pages smoke test")
    sys.exit(77)


class QuietHandler(SimpleHTTPRequestHandler):
    def log_message(self, *_args):
        pass

    def copyfile(self, source, outputfile):
        # The isolation bootstrap deliberately reloads the first navigation.
        try:
            super().copyfile(source, outputfile)
        except (BrokenPipeError, ConnectionResetError):
            pass


def main():
    source = Path(sys.argv[1] if len(sys.argv) > 1 else "_site/microgpt").resolve()
    with tempfile.TemporaryDirectory(prefix="microgpt-pages-") as directory:
        root = Path(directory)
        shutil.copytree(source, root / "let-go-lab" / "microgpt")
        server = ThreadingHTTPServer(("127.0.0.1", 0), partial(QuietHandler, directory=str(root)))
        threading.Thread(target=server.serve_forever, daemon=True).start()
        url = f"http://localhost:{server.server_port}/let-go-lab/microgpt/"
        try:
            with urlopen(url) as response:
                assert not response.headers.get("Cross-Origin-Opener-Policy")
                assert not response.headers.get("Cross-Origin-Embedder-Policy")
            with sync_playwright() as playwright:
                browser = playwright.chromium.launch(headless=True)
                try:
                    for label, viewport in [("desktop", {"width": 1280, "height": 900}),
                                            ("mobile", {"width": 390, "height": 844})]:
                        context = browser.new_context(viewport=viewport)
                        page = context.new_page()
                        errors, failed = [], []
                        page.on("pageerror", lambda error: errors.append(str(error)))
                        page.on("requestfailed", lambda request: failed.append(request.url))
                        page.goto(url)
                        page.wait_for_load_state("networkidle")
                        page.wait_for_function("!document.getElementById('generate').disabled", timeout=90000)
                        page.get_by_role("button", name="Generate again", exact=True).wait_for()
                        assert page.evaluate("crossOriginIsolated"), "service worker did not isolate the page"
                        worker = page.evaluate("navigator.serviceWorker.controller.scriptURL")
                        assert worker.endswith("/let-go-lab/microgpt/coi-serviceworker.js"), worker
                        first = page.locator("#names .name").all_text_contents()
                        assert first == ["anria", "aliia", "kirli", "keson", "denan", "amayan", "kashi", "oranag"], first
                        assert page.evaluate("document.documentElement.scrollWidth <= innerWidth"), "horizontal overflow"
                        if os.environ.get("MICROGPT_SCREENSHOT_DIR"):
                            target = Path(os.environ["MICROGPT_SCREENSHOT_DIR"])
                            target.mkdir(parents=True, exist_ok=True)
                            page.screenshot(path=str(target / f"microgpt-{label}.png"), full_page=True)
                        for _ in range(2):
                            page.get_by_role("button", name="Generate again", exact=True).click()
                            assert page.locator("#names").get_attribute("aria-busy") == "true"
                            page.wait_for_function("!document.getElementById('generate').disabled", timeout=90000)
                            current = page.locator("#names .name").all_text_contents()
                            assert len(current) == 8 and current != first, current
                            first = current
                        assert not errors, errors
                        # An initial navigation may be cancelled by the isolation reload.
                        assert not [item for item in failed if not item.startswith(url)], failed
                        # Simulate a failed request to check the retry interface.
                        page.evaluate("() => { window.originalEval = LetGoHost.eval; LetGoHost.eval = () => Promise.reject(new Error('test failure')); }")
                        page.get_by_role("button", name="Generate again", exact=True).click()
                        page.get_by_role("button", name="Try again", exact=True).wait_for()
                        assert "Couldn’t generate" in page.get_by_role("status").inner_text()
                        page.evaluate("() => { LetGoHost.eval = window.originalEval; }")
                        page.get_by_role("button", name="Try again", exact=True).click()
                        page.wait_for_function("!document.getElementById('generate').disabled", timeout=90000)
                        assert len(page.locator("#names .name").all_text_contents()) == 8
                        assert page.get_by_role("button", name="Generate again", exact=True).is_enabled()
                        context.close()
                        print(f"PASS: microgpt {label}: isolation, names, repeat generation, layout, retry")
                finally:
                    browser.close()
        finally:
            server.shutdown()
            server.server_close()


if __name__ == "__main__":
    main()
