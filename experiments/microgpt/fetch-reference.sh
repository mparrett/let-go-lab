#!/usr/bin/env bash
# Fetch what the port is measured against, pinned and checksummed:
#   input.txt         karpathy/makemore names.txt @988aa59 (the training data)
#   microgpt.py       karpathy's microgpt gist @14fb0388 (the reference)
#   microgpt_tape.py  microgpt.py + tape.patch (the same tape backward, for CPython)
# The reference is fetched rather than vendored: the gist carries no license.
#
#   experiments/microgpt/fetch-reference.sh
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
cd "$here"

# sha256sum on Linux, shasum on macOS.
if command -v sha256sum >/dev/null; then sha256() { sha256sum "$@"; }; else sha256() { shasum -a 256 "$@"; }; fi

fetch() { # url out sha256
  if [ ! -f "$2" ]; then
    curl -fsSL "$1" -o "$2.tmp"
    mv "$2.tmp" "$2"
  fi
  echo "$3  $2" | sha256 -c --quiet - >/dev/null || { echo "checksum mismatch: $2" >&2; exit 1; }
}

fetch https://raw.githubusercontent.com/karpathy/makemore/988aa59/names.txt \
  input.txt 0a30b5557f192f32ab962680889aac5f6fda0f4cecf40a6d0b5694f58ea8cc4d
fetch https://gist.githubusercontent.com/karpathy/8627fe009c40f57531cb18360106ce95/raw/14fb038816c7aae0bb9342c2dbf1a51dd134a5ff/microgpt.py \
  microgpt.py d47d88c2fd432c8ebdc1048beab7f7eb64ea7e0e664e11b812d72a6d95ebccee

patch --quiet -o microgpt_tape.py microgpt.py tape.patch
echo "fetched input.txt, microgpt.py; built microgpt_tape.py"
