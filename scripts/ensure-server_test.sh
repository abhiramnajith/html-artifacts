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

# Concurrency: two simultaneous cold-starts must not spawn duplicate servers.
# Stop the running one so both callers race the cold-start path.
pkill -f "$VELLUM_HOME/bin/vellum serve" 2>/dev/null || true
rm -f "$VELLUM_HOME/port"
sleep 0.3
c1="$(mktemp)"; c2="$(mktemp)"
( scripts/ensure-server.sh >"$c1" 2>/dev/null ) &
( scripts/ensure-server.sh >"$c2" 2>/dev/null ) &
wait
[ "$(cat "$c1")" = "$(cat "$c2")" ] || { echo "FAIL: concurrent cold-starts got different servers ($(cat "$c1") vs $(cat "$c2"))"; exit 1; }
n="$(pgrep -f "$VELLUM_HOME/bin/vellum serve" | wc -l | tr -d ' ')"
[ "$n" = "1" ] || { echo "FAIL: concurrent cold-starts spawned $n servers, want 1"; exit 1; }
rm -f "$c1" "$c2"

echo "PASS: ensure-server.sh bootstraps, is idempotent, and serializes cold-starts"
