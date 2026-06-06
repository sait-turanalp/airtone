#!/bin/bash
# AirTone engine — START
# Routes system audio to the "AirTone Sync" Multi-Output (speakers + BlackHole),
# captures BlackHole gaplessly with sox, and serves it via snapserver/Snapweb.
set -euo pipefail
# shellcheck source=scripts/common.sh disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

mkdir -p "$AIRTONE_HOME"

# Preconditions (full checks live in `doctor`; these are the blocking ones).
if [ ! -d "$SNAPWEB_DIR" ]; then
  echo "error: Snapweb not found at $SNAPWEB_DIR — run setup first" >&2; exit 1
fi
if ! device_exists "$DEVICE_NAME"; then
  echo "error: '$DEVICE_NAME' device missing — run setup first" >&2; exit 1
fi

echo "==> Cleaning up any previous session..."
pkill -x snapserver 2>/dev/null || true
pkill -x sox 2>/dev/null || true
sleep 1

# Remember the user's current output so stop can restore it.
"$SWITCH" -c > "$PREV_OUTPUT_FILE" 2>/dev/null || true
"$SWITCH" -s "$DEVICE_NAME" >/dev/null
echo "    Output -> $DEVICE_NAME (speakers + BlackHole)"

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

echo "==> Starting sox capture (gapless)..."
"$SOX" -t coreaudio "$CAPTURE_DEVICE" -t raw -b 16 -e signed-integer -c 2 -r "$SAMPLE_RATE" - \
  > "$FIFO" 2>"$AIRTONE_HOME/sox.log" &
sleep 2

if ! pgrep -x sox >/dev/null; then
  echo "error: sox failed to start:" >&2; tail -5 "$AIRTONE_HOME/sox.log" >&2; exit 1
fi

IP="$(lan_ip)"
cat <<EOF

============================================
 READY ✅   AirTone is streaming
 Mac plays on its speakers (instant).
 On your phone (same Wi-Fi), open:
   http://$IP:$HTTP_PORT
 (or scan the QR code shown by the app)
 Play audio on the Mac. Stop with: stop.sh
============================================
EOF
