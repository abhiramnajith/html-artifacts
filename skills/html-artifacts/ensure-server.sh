#!/usr/bin/env bash
# Ensure the html-artifacts viewer is running; print its base URL on stdout.
# Bootstraps the binary (PATH -> local -> GitHub release -> go install) and
# starts the server on a free port, recording it for the skill.
set -euo pipefail

REPO="abhiramnajith/html-artifacts"
HA_HOME="${HTML_ARTIFACTS_HOME:-$HOME/.html-artifacts}"
BIN_DIR="$HA_HOME/bin"
BIN="$BIN_DIR/html-artifacts"
PORT_FILE="$HA_HOME/port"
DIR="${HTML_ARTIFACTS_DIR:-$HA_HOME/artifacts}"
START_PORT="${HTML_ARTIFACTS_PORT:-47600}"
mkdir -p "$BIN_DIR" "$DIR"
chmod 700 "$HA_HOME" 2>/dev/null || true  # Finding 4: keep the global store private to this user

server_up() { # $1=port
  curl -sf -o /dev/null "http://127.0.0.1:$1/artifacts" 2>/dev/null
}

port_free() { # $1=port ; 0 if nothing is listening
  ! (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null
}

# 1. Already running on the recorded port?
if [ -f "$PORT_FILE" ]; then
  p="$(cat "$PORT_FILE")"
  if [ -n "$p" ] && server_up "$p"; then
    echo "http://127.0.0.1:$p"; exit 0
  fi
fi

# 2. Ensure a binary.
resolve_bin() {
  if command -v html-artifacts >/dev/null 2>&1; then echo "$(command -v html-artifacts)"; return; fi
  if [ -x "$BIN" ]; then echo "$BIN"; return; fi
  echo ""
}
BIN_PATH="$(resolve_bin)"
if [ -z "$BIN_PATH" ]; then
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$arch" in x86_64|amd64) arch=amd64 ;; arm64|aarch64) arch=arm64 ;; esac
  asset="html-artifacts-${os}-${arch}"
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
  echo "html-artifacts: no binary found and no download/go available." >&2
  echo "Install Go, or download a release from https://github.com/$REPO/releases" >&2
  exit 1
fi

# 3. Pick a free port.
port="$START_PORT"
while ! port_free "$port"; do
  port=$((port + 1))
  [ "$port" -gt 65000 ] && { echo "no free port" >&2; exit 1; }
done

# 4. Start in the background, record the port, wait for readiness.
nohup "$BIN_PATH" serve --port "$port" --dir "$DIR" >"$HA_HOME/server.log" 2>&1 &
echo "$port" > "$PORT_FILE"
for _ in $(seq 1 50); do
  server_up "$port" && { echo "http://127.0.0.1:$port"; exit 0; }
  sleep 0.1
done
echo "html-artifacts: server did not become ready; see $HA_HOME/server.log" >&2
exit 1
