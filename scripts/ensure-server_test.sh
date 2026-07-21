#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
make build >/dev/null

TMP="$(mktemp -d)"
trap 'pkill -f "$TMP/.vellum/bin" 2>/dev/null || true; rm -rf "$TMP"' EXIT
export VELLUM_HOME="$TMP/.vellum"
mkdir -p "$VELLUM_HOME/bin"
cp bin/vellum "$VELLUM_HOME/bin/vellum"

url1="$(scripts/ensure-server.sh)"
echo "first:  $url1"
case "$url1" in http://127.0.0.1:*) ;; *) echo "FAIL: bad url $url1"; exit 1;; esac
curl -sf -o /dev/null "$url1/artifacts" || { echo "FAIL: server not reachable"; exit 1; }

# Idempotent: second call reuses the same server/port.
url2="$(scripts/ensure-server.sh)"
[ "$url1" = "$url2" ] || { echo "FAIL: not idempotent ($url1 != $url2)"; exit 1; }

echo "PASS: ensure-server.sh bootstraps and is idempotent"
