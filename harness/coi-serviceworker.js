// coi-serviceworker.js — re-add COOP/COEP to same-origin responses so the page
// is cross-origin isolated where the server can't set headers (GitHub Pages).
//
// let-go's wasm input ring (read-key / term/size) is SharedArrayBuffer-backed,
// which requires crossOriginIsolated === true. Locally, serve.py / serve.json
// set the headers directly and this shim isn't needed; on Pages it is. The page
// registers this worker and reloads (see the COI bootstrap in index.html's head);
// on the controlled reload these headers make the context isolated.
//
// Pass server-set isolation headers through untouched — only add them when
// absent — so a properly-headered origin keeps require-corp rather than being
// downgraded to the Chrome-only credentialless variant Safari rejects. Adapted
// from the sibling xsofy/joint-xsofy shim.

addEventListener('install', () => skipWaiting());
addEventListener('activate', (e) => e.waitUntil(clients.claim()));
addEventListener('fetch', (e) => {
  // Bail on a cache-only cross-origin probe (Chrome dev-tools / extensions).
  if (e.request.cache === 'only-if-cached' && e.request.mode !== 'same-origin') return;
  e.respondWith(
    fetch(e.request)
      .then((r) => {
        if (r.status === 0) return r; // opaque response — leave it alone
        const h = new Headers(r.headers);
        // require-corp is the broadest-compatible COEP (Safari/Firefox/Chrome).
        if (!h.has('Cross-Origin-Embedder-Policy')) h.set('Cross-Origin-Embedder-Policy', 'require-corp');
        if (!h.has('Cross-Origin-Opener-Policy')) h.set('Cross-Origin-Opener-Policy', 'same-origin');
        return new Response(r.body, { status: r.status, statusText: r.statusText, headers: h });
      })
      .catch(() => new Response(null, { status: 500 }))
  );
});
