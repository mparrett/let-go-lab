# shellcheck shell=sh
# Resolve the lg binary inside a let-go checkout. Sourced by the scripts that
# shell out to lg; `just env` sources it too -- and just runs recipes under sh,
# which is dash on Linux CI, so this file stays POSIX: no [[ ]], no local.
#
# Upstream's Makefile builds the candidate to build/lg and installs the
# smoke-gated copy to bin/lg ("Promotion is part of the build rule rather than a
# separate step, so bin/lg cannot silently lag build/lg"). Older checkouts left
# the binary at the checkout root, and that stale root copy OUTLIVES the layout
# change -- it is not rebuilt and not removed, so a lab that looks there pins
# itself to whatever lg was current the day that file was last written, while
# reporting no error at all. Prefer the promoted copy; fall back to the root one
# so a checkout that still builds that way keeps working.
lg_path() {
  if [ -x "$1/bin/lg" ]; then
    printf '%s\n' "$1/bin/lg"
  else
    printf '%s\n' "$1/lg"
  fi
}
