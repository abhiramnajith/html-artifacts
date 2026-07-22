#!/usr/bin/env bash
# Ensure the vellum viewer is running; print its base URL on stdout.
# Bootstraps the binary (PATH -> local -> GitHub release -> go install) and
# starts the server on a free port, recording it for the skill.
set -euo pipefail

REPO="abhiramnajith/vellum"
HA_HOME="${VELLUM_HOME:-$HOME/.vellum}"
BIN_DIR="$HA_HOME/bin"
BIN="$BIN_DIR/vellum"
PORT_FILE="$HA_HOME/port"
LOCK="$HA_HOME/.startlock"
DIR="${VELLUM_DIR:-$HA_HOME/artifacts}"
START_PORT="${VELLUM_PORT:-47600}"
# Auto-started servers reap themselves after this long with no requests, so a
# forgotten background viewer does not linger forever. 0 = never idle-exit.
IDLE="${VELLUM_IDLE_TIMEOUT:-30m}"
mkdir -p "$BIN_DIR" "$DIR"
chmod 700 "$HA_HOME" 2>/dev/null || true  # Finding 4: keep the global store private to this user

server_up() { # $1=port
  curl -sf -o /dev/null "http://127.0.0.1:$1/artifacts" 2>/dev/null
}

port_free() { # $1=port ; 0 if nothing is listening
  ! (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null
}

# print_if_running: if the recorded port answers, print its URL and succeed.
print_if_running() {
  [ -f "$PORT_FILE" ] || return 1
  p="$(cat "$PORT_FILE")"
  [ -n "$p" ] && server_up "$p" || return 1
  echo "http://127.0.0.1:$p"
}

# 1. Fast path (no lock): already running on the recorded port?
if print_if_running; then exit 0; fi

# 2. Serialize cold-start so two concurrent callers can't spawn duplicate
#    servers on adjacent ports. mkdir is atomic; a stale lock (crashed starter)
#    is stolen after ~5s so we never deadlock.
i=0
while ! mkdir "$LOCK" 2>/dev/null; do
  i=$((i + 1))
  if [ "$i" -ge 50 ]; then rm -rf "$LOCK" 2>/dev/null || true; i=0; fi
  sleep 0.1
done
trap 'rm -rf "$LOCK" 2>/dev/null || true' EXIT INT TERM HUP

# Another caller may have started the server while we waited for the lock.
if print_if_running; then exit 0; fi

# 3. Ensure a binary.
resolve_bin() {
  if command -v vellum >/dev/null 2>&1; then echo "$(command -v vellum)"; return; fi
  if [ -x "$BIN" ]; then echo "$BIN"; return; fi
  echo ""
}
BIN_PATH="$(resolve_bin)"
if [ -z "$BIN_PATH" ]; then
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$arch" in x86_64|amd64) arch=amd64 ;; arm64|aarch64) arch=arm64 ;; esac
  asset="vellum-${os}-${arch}"
  base="https://github.com/$REPO/releases/latest/download"
  if curl -fsSL "$base/$asset" -o "$BIN.tmp" 2>/dev/null \
     && curl -fsSL "$base/SHA256SUMS" -o "$HA_HOME/SHA256SUMS.tmp" 2>/dev/null; then
    want="$(grep " $asset\$" "$HA_HOME/SHA256SUMS.tmp" | awk '{print $1}' || true)"
    got="$( (command -v sha256sum >/dev/null && sha256sum "$BIN.tmp" || shasum -a 256 "$BIN.tmp") | awk '{print $1}')"
    if [ -n "$want" ] && [ "$want" = "$got" ]; then
      mv "$BIN.tmp" "$BIN"; chmod +x "$BIN"; BIN_PATH="$BIN"
    else
      rm -f "$BIN.tmp"; echo "checksum mismatch for $asset" >&2
    fi
    rm -f "$HA_HOME/SHA256SUMS.tmp"
  else
    rm -f "$BIN.tmp" "$HA_HOME/SHA256SUMS.tmp"
  fi
fi
if [ -z "$BIN_PATH" ] && command -v go >/dev/null 2>&1; then
  go install "github.com/$REPO/server@latest" || true
  gobin="$(go env GOBIN)"; [ -z "$gobin" ] && gobin="$(go env GOPATH)/bin"
  [ -x "$gobin/server" ] && BIN_PATH="$gobin/server"
fi
if [ -z "$BIN_PATH" ]; then
  echo "vellum: no binary found and no download/go available." >&2
  echo "Install Go, or download a release from https://github.com/$REPO/releases" >&2
  exit 1
fi

# 4. Pick a free port.
port="$START_PORT"
while ! port_free "$port"; do
  port=$((port + 1))
  [ "$port" -gt 65000 ] && { echo "no free port" >&2; exit 1; }
done

# 5. Start in the background, record the port, wait for readiness. The idle
#    timeout lets a forgotten viewer reap itself; the lock (held until we exit)
#    keeps a second caller from racing us here.
nohup "$BIN_PATH" serve --port "$port" --dir "$DIR" --idle-timeout "$IDLE" >"$HA_HOME/server.log" 2>&1 &
echo "$port" > "$PORT_FILE"
for _ in $(seq 1 50); do
  server_up "$port" && { echo "http://127.0.0.1:$port"; exit 0; }
  sleep 0.1
done
echo "vellum: server did not become ready; see $HA_HOME/server.log" >&2
exit 1
