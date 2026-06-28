#!/usr/bin/env bash
# Regression for #8: harness/inject-shell.sh must be transactional — first
# injection and idempotent replacement work, and any malformed input (lone or
# duplicate sentinel, missing </body>, missing LetGoHost) fails nonzero and
# leaves the file byte-for-byte unchanged.
#
# Standalone: `bash test/inject_shell_test.sh`. No deps beyond coreutils + awk.

set -uo pipefail

LAB="$(cd "$(dirname "$0")/.." && pwd)"
INJECT="$LAB/harness/inject-shell.sh"
START="<!--LETGO-LAB-SHELL-START-->"
END="<!--LETGO-LAB-SHELL-END-->"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

fails=0
pass() { echo "ok   - $1"; }
fail() { echo "FAIL - $1"; fails=$((fails + 1)); }

# A minimal shell-less bundle (has window.LetGoHost + </body>) and a shell file.
make_index() { printf '<html><body>\n<script>window.LetGoHost={}</script>\n</body></html>\n' > "$1"; }
make_shell() { printf '<style>#x{}</style>\n<script>/*shell %s*/</script>\n' "${1:-A}" > "$2"; }

count() { grep -cF "$1" "$2" 2>/dev/null || true; }
mode() { stat -f '%Lp' "$1" 2>/dev/null || stat -c '%a' "$1" 2>/dev/null; }

# 1. First injection succeeds with exactly one matched pair + surviving </body>.
idx="$tmp/i1.html"; sh="$tmp/s1.html"; make_index "$idx"; make_shell "A" "$sh"
if "$INJECT" "$idx" "$sh" >/dev/null 2>&1 \
   && [[ "$(count "$START" "$idx")" -eq 1 && "$(count "$END" "$idx")" -eq 1 ]] \
   && grep -q "</body>" "$idx" && grep -q "shell A" "$idx"; then
  pass "first injection"
else
  fail "first injection"
fi

# 2. Idempotent replacement: re-injecting an edited shell swaps in place (one
#    pair, no stacking) and a same-shell re-run is a no-op (identical bytes).
make_shell "B" "$sh"
"$INJECT" "$idx" "$sh" >/dev/null 2>&1
before="$(shasum "$idx" | cut -d' ' -f1)"
"$INJECT" "$idx" "$sh" >/dev/null 2>&1
after="$(shasum "$idx" | cut -d' ' -f1)"
if [[ "$(count "$START" "$idx")" -eq 1 && "$(count "$END" "$idx")" -eq 1 ]] \
   && grep -q "shell B" "$idx" && ! grep -q "shell A" "$idx" \
   && [[ "$before" == "$after" ]]; then
  pass "idempotent replacement"
else
  fail "idempotent replacement (start=$(count "$START" "$idx") end=$(count "$END" "$idx") stable=$([[ "$before" == "$after" ]] && echo y || echo n))"
fi

# 2b. Mode preservation: a 0644 bundle must stay 0644 after injection (the temp
#     file mktemp creates is 0600, so a bare mv would leak that mode).
idx="$tmp/imode.html"; make_index "$idx"; chmod 644 "$idx"
"$INJECT" "$idx" "$sh" >/dev/null 2>&1
if [[ "$(mode "$idx")" == "644" ]]; then
  pass "preserves file mode (0644)"
else
  fail "preserves file mode (got $(mode "$idx"), want 644)"
fi

# Helper: assert the injector fails nonzero AND leaves the file unchanged.
assert_reject() {
  local name="$1" file="$2" shell="$3"
  local pre rc post
  pre="$(shasum "$file" | cut -d' ' -f1)"
  "$INJECT" "$file" "$shell" >/dev/null 2>&1; rc=$?
  post="$(shasum "$file" | cut -d' ' -f1)"
  if [[ "$rc" -ne 0 && "$pre" == "$post" ]]; then
    pass "$name"
  else
    fail "$name (rc=$rc unchanged=$([[ "$pre" == "$post" ]] && echo y || echo n))"
  fi
}

# 3. Lone start sentinel (the original silent-truncation bug): reject, unchanged.
idx="$tmp/i3.html"; printf '<html><body>\n%s\nwindow.LetGoHost\nold-content\n</body></html>\n' "$START" > "$idx"
assert_reject "reject lone start sentinel" "$idx" "$sh"

# 4. Duplicate sentinels: reject, unchanged.
idx="$tmp/i4.html"; printf '<html><body>\n%s\n%s\n%s\n%s\nwindow.LetGoHost\n</body></html>\n' "$START" "$END" "$START" "$END" > "$idx"
assert_reject "reject duplicate sentinels" "$idx" "$sh"

# 5. Missing </body>: reject, unchanged.
idx="$tmp/i5.html"; printf '<html><body>\nwindow.LetGoHost\n</html>\n' > "$idx"
assert_reject "reject missing </body>" "$idx" "$sh"

# 6. Missing window.LetGoHost (wrong build): reject, unchanged.
idx="$tmp/i6.html"; printf '<html><body>\nno host here\n</body></html>\n' > "$idx"
assert_reject "reject missing LetGoHost" "$idx" "$sh"

echo
if [[ "$fails" -eq 0 ]]; then
  echo "PASS: all inject-shell cases"
  exit 0
fi
echo "FAIL: $fails case(s)"
exit 1
