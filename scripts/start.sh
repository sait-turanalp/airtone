#!/bin/bash
# AirTone engine — START
# Captures system audio with a CoreAudio process tap (no virtual driver, no
# device switching), serves it via snapserver/Snapweb, and plays it back on the
# Mac through a local snapclient. Both the Mac and the phone are snapcast
# clients, so they share one clock — that is what keeps them in sync.
#
# The tap mutes audio at its source and excludes snapclient, or snapclient would
# capture its own playback in a feedback loop.
set -euo pipefail
# shellcheck source=scripts/common.sh disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

mkdir -p "$AIRTONE_HOME"

# Preconditions (full checks live in `doctor`; these are the blocking ones).
if [ ! -d "$SNAPWEB_DIR" ]; then
  echo "error: Snapweb not found at $SNAPWEB_DIR — run setup first" >&2; exit 1
fi
for bin in "$SNAPSERVER" "$SNAPCLIENT"; do
  [ -x "$bin" ] || { echo "error: $bin missing — brew install snapcast" >&2; exit 1; }
done

echo "==> Cleaning up any previous session..."
kill_pipeline
sleep 1

build_tap

# Ask the tap what the output device actually runs at, so snapserver is told the
# truth and nobody has to resample.
SAMPLEFORMAT="$("$TAP_BIN" --probe)"
echo "    Capture format: $SAMPLEFORMAT"

# Generate the runtime snapserver config from the template.
sed -e "s|__FIFO__|$FIFO|g" \
    -e "s|__SAMPLEFORMAT__|$SAMPLEFORMAT|g" \
    -e "s|__BUFFER__|$BUFFER|g" \
    -e "s|__CODEC__|$CODEC|g" \
    -e "s|__CHUNK_MS__|$CHUNK_MS|g" \
    -e "s|__HTTP_PORT__|$HTTP_PORT|g" \
    -e "s|__DOC_ROOT__|$SNAPWEB_DIR|g" \
    "$ASSETS_DIR/snapserver.conf.tmpl" > "$RUNTIME_CONF"

# (Re)create the FIFO.
rm -f "$FIFO"; mkfifo "$FIFO"

echo "==> Starting snapserver (buffer ${BUFFER}ms, codec ${CODEC})..."
"$SNAPSERVER" -c "$RUNTIME_CONF" >"$AIRTONE_HOME/snapserver.log" 2>&1 &
sleep 1

# The Mac's own speakers are fed by this client, not by the system output.
# It plays to whatever the default output device is — untouched, as the user
# left it — so AirPods or an external DAC keep working with no extra config.
echo "==> Starting the local player (snapclient)..."
"$SNAPCLIENT" --hostID airtone-mac tcp://127.0.0.1 >"$AIRTONE_HOME/snapclient.log" 2>&1 &
SNAPCLIENT_PID=$!
sleep 1

if ! kill -0 "$SNAPCLIENT_PID" 2>/dev/null; then
  echo "error: snapclient failed to start:" >&2
  tail -5 "$AIRTONE_HOME/snapclient.log" >&2
  kill_pipeline; exit 1
fi

echo "==> Starting the system tap..."
: > "$AIRTONE_HOME/tap.log"
"$TAP_BIN" --mute --exclude-pid "$SNAPCLIENT_PID" > "$FIFO" 2>"$AIRTONE_HOME/tap.log" &
TAP_PID=$!

# The tap streams silence until snapclient opens its audio device, then swaps in
# the real capture. Wait for that handover before calling the pipeline ready.
for _ in $(seq 1 40); do
  grep -q "capturing" "$AIRTONE_HOME/tap.log" 2>/dev/null && break
  kill -0 "$TAP_PID" 2>/dev/null || break
  sleep 0.5
done

if ! kill -0 "$TAP_PID" 2>/dev/null || ! grep -q "capturing" "$AIRTONE_HOME/tap.log"; then
  echo "error: the system tap failed to start:" >&2
  tail -5 "$AIRTONE_HOME/tap.log" >&2
  kill_pipeline; exit 1
fi

IP="$(lan_ip)"
cat <<EOF

============================================
 READY ✅   AirTone is streaming
 Mac and phone play the same stream in sync.
 On your phone (same Wi-Fi), open:
   http://$IP:$HTTP_PORT
 (or scan the QR code shown by the app)
 Play audio on the Mac. Stop with: stop.sh
============================================
EOF
