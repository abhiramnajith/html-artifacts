#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
make build >/dev/null

TMP="$(mktemp -d)"
trap 'pkill -f "$TMP/.html-artifacts/bin" 2>/dev/null || true; rm -rf "$TMP"' EXIT
export HTML_ARTIFACTS_HOME="$TMP/.html-artifacts"
mkdir -p "$HTML_ARTIFACTS_HOME/bin"
cp bin/html-artifacts "$HTML_ARTIFACTS_HOME/bin/html-artifacts"

url1="$(scripts/ensure-server.sh)"
echo "first:  $url1"
case "$url1" in http://127.0.0.1:*) ;; *) echo "FAIL: bad url $url1"; exit 1;; esac
curl -sf -o /dev/null "$url1/artifacts" || { echo "FAIL: server not reachable"; exit 1; }

# Idempotent: second call reuses the same server/port.
url2="$(scripts/ensure-server.sh)"
[ "$url1" = "$url2" ] || { echo "FAIL: not idempotent ($url1 != $url2)"; exit 1; }

echo "PASS: ensure-server.sh bootstraps and is idempotent"
