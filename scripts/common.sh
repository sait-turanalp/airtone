#!/bin/bash
# shellcheck disable=SC2034  # these vars are consumed by the scripts that source this file
# Shared helpers and resolved paths for the AirTone engine scripts.
# Sourced by start.sh / stop.sh / setup.sh.

# Data dir (override with AIRTONE_HOME). No spaces -> simpler shell handling.
AIRTONE_HOME="${AIRTONE_HOME:-$HOME/.airtone}"
FIFO="${AIRTONE_FIFO:-$AIRTONE_HOME/fifo}"
RUNTIME_CONF="$AIRTONE_HOME/snapserver.conf"
SNAPWEB_DIR="$AIRTONE_HOME/snapweb"
BIN_DIR="$AIRTONE_HOME/engine/bin"
TAP_BIN="$BIN_DIR/airtone-tap"

# Tunables.
BUFFER="${AIRTONE_BUFFER:-500}"        # snapcast end-to-end delay; every client shares it
CODEC="${AIRTONE_CODEC:-opus}"         # opus: bandwidth-friendly for multiple synced clients
CHUNK_MS="${AIRTONE_CHUNK_MS:-20}"     # snapcast default; latency granularity
HTTP_PORT="${AIRTONE_HTTP_PORT:-1780}"

# Resolve the repo asset dir relative to the scripts dir.
ENGINE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ASSETS_DIR="${AIRTONE_ASSETS_DIR:-$ENGINE_DIR/../assets}"

# Binaries (prefer PATH, fall back to Homebrew prefix).
_resolve() { command -v "$1" 2>/dev/null || echo "/opt/homebrew/bin/$1"; }
SNAPSERVER="$(_resolve snapserver)"
SNAPCLIENT="$(_resolve snapclient)"

lan_ip() { ipconfig getifaddr en0 2>/dev/null || ipconfig getifaddr en1 2>/dev/null || echo "127.0.0.1"; }

# Compile the Swift system tap once into the data dir. Rebuilt only when the
# embedded source is newer, so an upgraded binary always ships its own tap.
build_tap() {
  if [ -x "$TAP_BIN" ] && [ "$TAP_BIN" -nt "$ASSETS_DIR/systemtap.swift" ]; then
    return 0
  fi
  if ! command -v swiftc >/dev/null 2>&1; then
    echo "error: swiftc not found — install the Xcode command line tools:" >&2
    echo "       xcode-select --install" >&2
    return 1
  fi
  mkdir -p "$BIN_DIR"
  echo "    Compiling the system tap (one-time)..."
  swiftc -O "$ASSETS_DIR/systemtap.swift" -o "$TAP_BIN"
}

# Kill everything AirTone starts. Killing the tap is what un-mutes the Mac, so
# this runs on every teardown path, successful or not.
kill_pipeline() {
  pkill -x snapserver 2>/dev/null || true
  pkill -x snapclient 2>/dev/null || true
  pkill -f "$TAP_BIN" 2>/dev/null || true
}
