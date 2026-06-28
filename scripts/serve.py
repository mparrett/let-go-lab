#!/usr/bin/env python3
"""scripts/serve.py — minimal stdlib static server for a built demo bundle (#9).

Replaces `npx serve@latest` so a clean checkout serves with no npm/node and no
network. Cross-origin isolation (SharedArrayBuffer, needed by the wasm input
ring) requires COOP/COEP; the header values are read from harness/serve.json so
they stay single-sourced. HTTPS (for LAN/phone, a secure context) when --cert
/--key are given; localhost HTTP otherwise.

Usage:
  serve.py --dir DIR [--port N] [--host H] [--cert cert.pem --key key.pem]
           [--headers harness/serve.json]
"""
import argparse
import json
import os
import ssl
import sys
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer


def load_headers(path):
    """Flatten serve.json's headers[].headers[] into a [(key, value)] list."""
    if not path or not os.path.isfile(path):
        return []
    with open(path) as f:
        cfg = json.load(f)
    out = []
    for entry in cfg.get("headers", []):
        for h in entry.get("headers", []):
            out.append((h["key"], h["value"]))
    return out


class Handler(SimpleHTTPRequestHandler):
    extra_headers = []

    def end_headers(self):
        for k, v in self.extra_headers:
            self.send_header(k, v)
        super().end_headers()

    def log_message(self, *args):
        pass  # quiet; the wrapper prints the URLs


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dir", required=True)
    ap.add_argument("--port", type=int, default=8249)
    ap.add_argument("--host", default="localhost")
    ap.add_argument("--cert")
    ap.add_argument("--key")
    ap.add_argument("--headers")
    args = ap.parse_args()

    Handler.extra_headers = load_headers(args.headers)
    handler = partial(Handler, directory=args.dir)
    try:
        httpd = ThreadingHTTPServer((args.host, args.port), handler)
    except OSError as e:
        print(f"serve.py: cannot bind {args.host}:{args.port} — {e.strerror or e}",
              file=sys.stderr)
        return 1

    if args.cert and args.key:
        ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        ctx.load_cert_chain(args.cert, args.key)
        httpd.socket = ctx.wrap_socket(httpd.socket, server_side=True)

    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        return 0
    finally:
        httpd.server_close()


if __name__ == "__main__":
    sys.exit(main())
