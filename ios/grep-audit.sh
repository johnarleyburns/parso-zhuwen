#!/usr/bin/env bash
# grep-audit.sh — the network-surface CI gate for the Zhuwen iOS app (handoff §8, invariant I2).
#
# Enforces that the ONLY network surface is the isolated PackClient (anonymous CDN GET), StoreKit 2,
# and the opt-in private CloudKit sync. Fails the build if any of these leak in:
#   1. `URLSession` used anywhere outside Sources/ZhuwenPacks/PackClient.swift
#   2. a known third-party analytics / crash / ads SDK name (NFR-5: zero third-party SDKs)
#   3. a cleartext `http://` URL literal
#   4. a secret-looking value written to UserDefaults
#
# Usage: ios/grep-audit.sh            (run from anywhere; resolves its own Sources dir)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SRC="$ROOT/Sources"
FAIL=0

fail() { echo "  ✗ $1"; FAIL=1; }

echo "grep-audit: network-surface (I2) + no-SDK (NFR-5)"

# 1) URLSession only inside PackClient.swift.
if command -v rg >/dev/null 2>&1; then
  HITS=$(rg -l --no-messages 'URLSession' "$SRC" | grep -v 'ZhuwenPacks/PackClient.swift' || true)
else
  HITS=$(grep -rl 'URLSession' "$SRC" | grep -v 'ZhuwenPacks/PackClient.swift' || true)
fi
if [ -n "$HITS" ]; then
  fail "URLSession used outside ZhuwenPacks/PackClient.swift:"
  echo "$HITS" | sed 's/^/      /'
else
  echo "  ✓ URLSession confined to PackClient.swift"
fi

# 2) No third-party analytics / crash / ads SDKs.
SDK_PATTERN='Firebase|Crashlytics|GoogleAnalytics|Segment|Mixpanel|Amplitude|AppsFlyer|Adjust|Sentry|Bugsnag|Flurry|Braze|OneSignal|FBSDK|FacebookSDK|GADBanner|AdMob'
if grep -rlE "$SDK_PATTERN" "$SRC" >/dev/null 2>&1; then
  fail "third-party analytics/crash/ads SDK reference found:"
  grep -rnE "$SDK_PATTERN" "$SRC" | sed 's/^/      /'
else
  echo "  ✓ no third-party analytics/crash/ads SDKs"
fi

# 3) No cleartext http:// URL literals (ATS; https only).
if grep -rnE '"http://' "$SRC" >/dev/null 2>&1; then
  fail "cleartext http:// URL literal found:"
  grep -rnE '"http://' "$SRC" | sed 's/^/      /'
else
  echo "  ✓ no cleartext http:// literals"
fi

# 4) No secrets written to UserDefaults.
if grep -rniE 'UserDefaults[^\n]*\.set\([^)]*(secret|token|password|apikey|api_key|credential)' "$SRC" >/dev/null 2>&1; then
  fail "secret-looking value written to UserDefaults:"
  grep -rniE 'UserDefaults[^\n]*\.set\([^)]*(secret|token|password|apikey|api_key|credential)' "$SRC" | sed 's/^/      /'
else
  echo "  ✓ no secrets in UserDefaults"
fi

if [ "$FAIL" -ne 0 ]; then
  echo "grep-audit: FAILED"
  exit 1
fi
echo "grep-audit: OK"
