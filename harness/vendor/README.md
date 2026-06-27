# harness/vendor — pinned browser assets

Vendored so a clean checkout builds and runs the browser demo with **no network
and no npm/node** (issue #9). The build inlines these into the single
`index.html`, so the generated bundle is genuinely self-contained.

## xterm@5.5.0

xterm.js and the addons the sixel shell needs, copied verbatim from jsDelivr at
the versions `shell.html` was written against. The `.min.js` files are UMD
builds that expose the `Terminal`, `FitAddon`, and `ImageAddon` globals the shell
uses.

| File | Source | SHA-256 |
|------|--------|---------|
| `xterm.min.css` | `https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/css/xterm.min.css` | `f7f724aea2bb620a6482bfb8e4bdecfae1152b0c7facef55fbda61f3b6cfedb2` |
| `xterm.min.js` | `https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/lib/xterm.min.js` | `4196e242ef1cf4c2adead8d97f4a772a69576076f70b095e004b4abbb049e7bf` |
| `addon-fit.min.js` | `https://cdn.jsdelivr.net/npm/@xterm/addon-fit@0.10.0/lib/addon-fit.min.js` | `a6a7bbb33569f16aa3e18d71425e34d035fc89a0b7e8cba084f8855f91aa38f1` |
| `addon-image.min.js` | `https://cdn.jsdelivr.net/npm/@xterm/addon-image@0.8.0/lib/addon-image.min.js` | `9e9f76912ba3c450b7ca40b356ad1ea9238fe2640c57e72ef089482d56ecc252` |

### Updating

```sh
just vendor-xterm     # re-fetch the pinned versions and reprint checksums
```

To move to a new xterm version, bump the versions in `scripts/vendor-xterm.sh`,
re-run it, update this table, and re-test the browser demo. Verify the
downloaded files against the published checksums before committing.
