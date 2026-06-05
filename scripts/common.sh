#!/bin/bash
# shellcheck disable=SC2034  # these vars are consumed by the scripts that source this file
# Shared helpers and resolved paths for the AirTone engine scripts.
# Sourced by start.sh / stop.sh / setup.sh.

# Data dir (override with AIRTONE_HOME). No spaces -> simpler shell handling.
AIRTONE_HOME="${AIRTONE_HOME:-$HOME/.airtone}"
FIFO="${AIRTONE_FIFO:-$AIRTONE_HOME/fifo}"
RUNTIME_CONF="$AIRTONE_HOME/snapserver.conf"
SNAPWEB_DIR="$AIRTONE_HOME/snapweb"
PREV_OUTPUT_FILE="$AIRTONE_HOME/prev_output"

# Virtual Multi-Output device created by setup.
DEVICE_NAME="${AIRTONE_DEVICE_NAME:-AirTone Sync}"
CAPTURE_DEVICE="${AIRTONE_CAPTURE_DEVICE:-BlackHole 2ch}"

# Tunables.
BUFFER="${AIRTONE_BUFFER:-4000}"
CODEC="${AIRTONE_CODEC:-opus}"
HTTP_PORT="${AIRTONE_HTTP_PORT:-1780}"
SAMPLE_RATE="${AIRTONE_SAMPLE_RATE:-48000}"
SAMPLEFORMAT="${SAMPLE_RATE}:16:2"

# Resolve the repo asset dir relative to the scripts dir.
ENGINE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ASSETS_DIR="${AIRTONE_ASSETS_DIR:-$ENGINE_DIR/../assets}"

# Binaries (prefer PATH, fall back to Homebrew prefix).
_resolve() { command -v "$1" 2>/dev/null || echo "/opt/homebrew/bin/$1"; }
SNAPSERVER="$(_resolve snapserver)"
SOX="$(_resolve sox)"
SWITCH="$(_resolve SwitchAudioSource)"

lan_ip() { ipconfig getifaddr en0 2>/dev/null || ipconfig getifaddr en1 2>/dev/null || echo "127.0.0.1"; }

device_exists() { "$SWITCH" -a 2>/dev/null | grep -qx "$1"; }
